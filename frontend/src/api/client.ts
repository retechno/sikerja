import axios, { AxiosError, type AxiosRequestConfig } from 'axios'

/** Error API yang sudah dinormalisasi dari { error: { code, message, fields } }. */
export class ApiError extends Error {
  status: number
  code: string
  fields: Record<string, string>
  retryAfter: number
  data?: unknown

  constructor(
    status: number,
    code: string,
    message: string,
    fields: Record<string, string> = {},
    retryAfter = 0,
    data?: unknown,
  ) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.code = code
    this.fields = fields
    this.retryAfter = retryAfter
    this.data = data
  }
}

export const http = axios.create({
  baseURL: '/api/v1',
  withCredentials: true, // token ada di cookie httpOnly; tidak pernah disimpan di JS
  headers: { 'X-Requested-With': 'XMLHttpRequest' }, // proteksi CSRF
  timeout: 30000,
})

let onExpired: (() => void) | null = null
/** Didaftarkan store auth: dipanggil saat sesi benar-benar berakhir (refresh gagal). */
export function onSessionExpired(fn: () => void) {
  onExpired = fn
}

interface ErrorBody {
  error?: { code?: string; message?: string; fields?: Record<string, string> | null }
  data?: unknown
}

function toApiError(e: unknown): ApiError {
  if (e instanceof ApiError) return e
  const ax = e as AxiosError<ErrorBody>
  if (ax.response) {
    const body = ax.response.data
    const err = body?.error
    const retry = Number(ax.response.headers?.['retry-after'] ?? 0) || 0
    return new ApiError(
      ax.response.status,
      err?.code ?? 'error',
      err?.message ?? 'Terjadi kesalahan',
      err?.fields ?? {},
      retry,
      body?.data ?? body,
    )
  }
  if (ax.code === 'ECONNABORTED') return new ApiError(0, 'timeout', 'Server tidak merespons. Coba lagi.')
  return new ApiError(0, 'network', 'Tidak dapat terhubung ke server. Periksa koneksi Anda.')
}

// Satu refresh pada satu waktu (single-flight); request lain menunggu hasilnya.
let refreshing: Promise<void> | null = null

function refreshSession(): Promise<void> {
  if (!refreshing) {
    refreshing = http
      .post('/auth/refresh', null, { _skipRefresh: true } as AxiosRequestConfig)
      .then(() => undefined)
      .finally(() => {
        refreshing = null
      })
  }
  return refreshing
}

declare module 'axios' {
  interface AxiosRequestConfig {
    _skipRefresh?: boolean
    _retried?: boolean
  }
}

http.interceptors.response.use(
  (r) => r,
  async (error: AxiosError) => {
    const cfg = error.config
    const isAuthCall = cfg?.url?.startsWith('/auth/')
    if (error.response?.status === 401 && cfg && !cfg._skipRefresh && !cfg._retried && !isAuthCall) {
      try {
        await refreshSession()
        cfg._retried = true
        return await http.request(cfg)
      } catch {
        onExpired?.()
      }
    }
    throw toApiError(error)
  },
)

/** Ambil `data` dari amplop { data, meta }. */
export async function get<T>(url: string, params?: Record<string, unknown>): Promise<T> {
  return (await http.get(url, { params })).data.data as T
}

export async function getWithMeta<T, M>(
  url: string,
  params?: Record<string, unknown>,
): Promise<{ data: T; meta: M }> {
  const r = (await http.get(url, { params })).data
  return { data: r.data as T, meta: r.meta as M }
}

export async function send<T>(method: 'post' | 'put' | 'delete', url: string, body?: unknown): Promise<T> {
  const r = await http.request({ method, url, data: body })
  return r.data?.data as T
}

/** Unduh berkas (Excel/PDF) lewat blob supaya error JSON dari server tetap bisa ditampilkan. */
export async function download(
  url: string,
  params: Record<string, unknown>,
  fallbackName: string,
): Promise<void> {
  let res
  try {
    res = await http.get(url, { params, responseType: 'blob' })
  } catch (e) {
    const err = e as ApiError
    const blob = (err.data as unknown) instanceof Blob ? (err.data as Blob) : null
    if (blob) {
      let parsed: ErrorBody | null = null
      try {
        parsed = JSON.parse(await blob.text()) as ErrorBody
      } catch {
        parsed = null
      }
      if (parsed?.error)
        throw new ApiError(err.status, parsed.error.code ?? 'error', parsed.error.message ?? err.message)
    }
    throw err
  }
  const dispo: string = res.headers['content-disposition'] ?? ''
  const m = /filename\*?=(?:UTF-8'')?"?([^";]+)"?/i.exec(dispo)
  const name = m?.[1] ? decodeURIComponent(m[1]) : fallbackName
  const href = URL.createObjectURL(res.data as Blob)
  const a = document.createElement('a')
  a.href = href
  a.download = name
  document.body.appendChild(a)
  a.click()
  a.remove()
  setTimeout(() => URL.revokeObjectURL(href), 1000)
}
