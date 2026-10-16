// Format tampilan: tanggal "16 Okt 2026", desimal pakai koma, skor 1 desimal.

const MONTHS = ['Jan', 'Feb', 'Mar', 'Apr', 'Mei', 'Jun', 'Jul', 'Agu', 'Sep', 'Okt', 'Nov', 'Des']
const DAYS = ['Minggu', 'Senin', 'Selasa', 'Rabu', 'Kamis', 'Jumat', 'Sabtu']

export function toNum(v: unknown): number | null {
  if (v === null || v === undefined || v === '') return null
  const n = typeof v === 'number' ? v : Number(v)
  return Number.isFinite(n) ? n : null
}

/** Skor 1 desimal dengan koma (mis. 95,7). Kosong → "—". */
export function fmtScore(v: unknown): string {
  const n = toNum(v)
  if (n === null) return '—'
  return (Math.round((n + Number.EPSILON) * 10) / 10).toFixed(1).replace('.', ',')
}

/** Angka bebas (target/realisasi): maks. 2 desimal tanpa nol di belakang; ribuan dipisah titik. */
export function fmtNum(v: unknown, maxFrac = 2): string {
  const n = toNum(v)
  if (n === null) return '—'
  return new Intl.NumberFormat('id-ID', { maximumFractionDigits: maxFrac }).format(n)
}

function parseYmd(iso: string): [number, number, number] | null {
  const m = /^(\d{4})-(\d{2})-(\d{2})/.exec(iso)
  return m ? [Number(m[1]), Number(m[2]), Number(m[3])] : null
}

/** "2026-10-16T00:00:00Z" → "16 Okt 2026" (diambil dari teksnya, tanpa pergeseran zona waktu). */
export function fmtDate(iso?: string | null): string {
  const p = iso ? parseYmd(iso) : null
  if (!p) return '—'
  return `${p[2]} ${MONTHS[p[1] - 1]} ${p[0]}`
}

/** "2026-10-16T..." → "16 Okt" */
export function fmtDayMonth(iso?: string | null): string {
  const p = iso ? parseYmd(iso) : null
  return p ? `${p[2]} ${MONTHS[p[1] - 1]}` : '—'
}

/** Contoh: "Jumat, 16 Okt 2026" */
export function fmtLongDate(d: Date): string {
  return `${DAYS[d.getDay()]}, ${d.getDate()} ${MONTHS[d.getMonth()]} ${d.getFullYear()}`
}

/** "2026-10-16T14:20:00+08:00" → "14.20" (jam lokal peramban). */
export function fmtTime(iso?: string | null): string {
  if (!iso) return '—'
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return '—'
  return `${String(d.getHours()).padStart(2, '0')}.${String(d.getMinutes()).padStart(2, '0')}`
}

/** Ke format YYYY-MM-DD untuk <input type="date"> dari ISO API. */
export function toDateInput(iso?: string | null): string {
  const p = iso ? parseYmd(iso) : null
  return p ? `${p[0]}-${String(p[1]).padStart(2, '0')}-${String(p[2]).padStart(2, '0')}` : ''
}

export function initials(name?: string | null): string {
  if (!name) return '?'
  const parts = name.trim().split(/\s+/)
  return (
    (parts[0]?.[0] ?? '') + (parts.length > 1 ? (parts[parts.length - 1]?.[0] ?? '') : '')
  ).toUpperCase()
}

const GRADE_BADGE: Record<string, string> = {
  Istimewa: 'b-ist',
  Baik: 'b-teal',
  Cukup: 'b-info',
  'Perlu Perbaikan': 'b-warn',
  Kurang: 'b-bad',
}

export function gradeClass(grade?: string | null): string {
  return (grade && GRADE_BADGE[grade]) || 'b-mute'
}

export const ROLE_LABEL: Record<string, string> = {
  employee: 'Karyawan',
  manager: 'Manajer',
  hr: 'HR / Admin',
  director: 'Direktur',
}

export const APPRAISAL_STATUS: Record<string, { label: string; cls: string }> = {
  draft: { label: 'Belum dinilai', cls: 'b-mute' },
  self_done: { label: 'Self-assessment selesai', cls: 'b-info' },
  manager_done: { label: 'Dinilai atasan', cls: 'b-warn' },
  final: { label: 'Final', cls: 'b-good' },
}

export const RESULT_STATUS: Record<string, { label: string; cls: string }> = {
  missing: { label: 'Belum diisi', cls: 'b-mute' },
  submitted: { label: 'Menunggu validasi', cls: 'b-warn' },
  validated: { label: 'Tervalidasi', cls: 'b-good' },
  rejected: { label: 'Ditolak', cls: 'b-bad' },
}

export const PERIOD_STATUS: Record<string, { label: string; cls: string }> = {
  draft: { label: 'Draf', cls: 'b-mute' },
  active: { label: 'Aktif', cls: 'b-good' },
  closed: { label: 'Ditutup', cls: 'b-info' },
}

export const DIRECTION_LABEL: Record<string, string> = {
  higher: 'lebih tinggi lebih baik',
  lower: 'lebih rendah lebih baik',
}
