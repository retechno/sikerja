import { download, get, getWithMeta, http, send } from './client'
import type {
  AppraisalView,
  Department,
  DirectorDash,
  Employee,
  EmployeeDash,
  ImportResult,
  KpiResult,
  KpiSheet,
  ManagerDash,
  ManagerPayload,
  NotificationItem,
  PageMeta,
  Period,
  Position,
  ReportView,
  SelfPayload,
  TeamView,
  Template,
} from './types'

export * from './types'
export { ApiError } from './client'

// ---------- auth & profil ----------
export const auth = {
  login: (email: string, password: string, remember: boolean) =>
    send<Employee>('post', '/auth/login', { email, password, remember }),
  logout: () => send<void>('post', '/auth/logout'),
  forgot: (email: string) => send<{ message: string }>('post', '/auth/forgot-password', { email }),
  reset: (token: string, password: string) =>
    send<{ message: string }>('post', '/auth/reset-password', { token, password }),
  changePassword: (current_password: string, new_password: string) =>
    send<{ message: string }>('post', '/auth/change-password', { current_password, new_password }),
  me: () => get<Employee>('/me'),
  updateMe: (name: string, phone: string) => send<Employee>('put', '/me', { name, phone }),
  uploadPhoto: async (file: File) => {
    const fd = new FormData()
    fd.append('file', file)
    return (await http.post('/me/photo', fd)).data.data as Employee
  },
}

export const photoUrl = (employeeId: number) => `/api/v1/employees/${employeeId}/photo`

// ---------- master ----------
export const master = {
  departments: () => get<Department[]>('/departments'),
  saveDepartment: (id: number | null, body: { name: string; code: string; head_id?: number | null }) =>
    id ? send<Department>('put', `/departments/${id}`, body) : send<Department>('post', '/departments', body),
  deleteDepartment: (id: number) => send<void>('delete', `/departments/${id}`),
  positions: (departmentId?: number) =>
    get<Position[]>('/positions', departmentId ? { department_id: departmentId } : undefined),
  savePosition: (id: number | null, body: { name: string; department_id: number | null }) =>
    id ? send<Position>('put', `/positions/${id}`, body) : send<Position>('post', '/positions', body),
  deletePosition: (id: number) => send<void>('delete', `/positions/${id}`),
}

export interface EmployeeBody {
  nik: string
  name: string
  email: string
  phone: string
  department_id: number | null
  position_id: number | null
  manager_id: number | null
  join_date: string
  status: 'active' | 'inactive'
  roles: string[]
  password?: string
}

export const employees = {
  list: (params: { q?: string; department_id?: number; status?: string; page?: number; per_page?: number }) =>
    getWithMeta<Employee[], PageMeta>('/employees', params),
  get: (id: number) => get<Employee>(`/employees/${id}`),
  create: async (body: EmployeeBody) => {
    const r = (await http.post('/employees', body)).data
    return { employee: r.data as Employee, tempPassword: (r.meta?.temp_password ?? '') as string }
  },
  update: (id: number, body: EmployeeBody) => send<Employee>('put', `/employees/${id}`, body),
  remove: (id: number) => send<void>('delete', `/employees/${id}`),
  /** Import Excel. Bila ada baris salah, server membalas 422 dan hasilnya (daftar error) dikembalikan apa adanya. */
  importFile: async (file: File): Promise<ImportResult> => {
    const fd = new FormData()
    fd.append('file', file)
    try {
      return (await http.post('/employees/import', fd)).data.data as ImportResult
    } catch (e) {
      const err = e as { code?: string; data?: ImportResult }
      if (err.code === 'import_invalid' && err.data) return err.data
      throw e
    }
  },
  downloadTemplate: () => download('/employees-import-template', {}, 'template-import-karyawan.xlsx'),
}

// ---------- periode & template ----------
export interface PeriodBody {
  name: string
  type: 'monthly' | 'quarterly'
  start_date: string
  end_date: string
  kpi_weight: number
  behavior_weight: number
}

export const periods = {
  list: () => get<Period[]>('/periods'),
  create: (b: PeriodBody) => send<Period>('post', '/periods', b),
  update: (id: number, b: PeriodBody) => send<Period>('put', `/periods/${id}`, b),
  open: (id: number) => send<Period>('post', `/periods/${id}/open`),
  close: (id: number) => send<Period>('post', `/periods/${id}/close`),
  reopen: (id: number) => send<Period>('post', `/periods/${id}/reopen`),
}

export interface TemplateBody {
  name: string
  is_active: boolean
  indicators: {
    id?: number
    name: string
    unit: string
    target: number
    weight: number
    direction: 'higher' | 'lower'
    source: 'manual' | 'integration'
  }[]
}

export const templates = {
  list: () => get<Template[]>('/kpi-templates'),
  create: (department_id: number, name: string) => send<Template>('post', '/kpi-templates', { department_id, name }),
  save: (id: number, b: TemplateBody) => send<Template>('put', `/kpi-templates/${id}`, b),
}

// ---------- KPI ----------
export const kpi = {
  sheet: (params: { employee_id?: number; period_id?: number }) => get<KpiSheet>('/kpi/sheet', params),
  saveResult: (b: { period_id: number; employee_id: number; indicator_id: number; actual: number; note: string }) =>
    send<KpiResult>('put', '/kpi/results', b),
  validate: (id: number, action: 'validate' | 'reject', note = '') =>
    send<KpiResult>('post', `/kpi/results/${id}/validate`, { action, note }),
  uploadEvidence: async (id: number, file: File) => {
    const fd = new FormData()
    fd.append('file', file)
    return (await http.post(`/kpi/results/${id}/evidence`, fd)).data.data as KpiResult
  },
  evidenceUrl: (id: number, dl = false) => `/api/v1/kpi/results/${id}/evidence${dl ? '?download=1' : ''}`,
}

// ---------- penilaian ----------
export const appraisals = {
  get: (periodId: number, employeeId: number) => get<AppraisalView>(`/periods/${periodId}/appraisals/${employeeId}`),
  saveSelf: (periodId: number, employeeId: number, b: SelfPayload) =>
    send<AppraisalView>('put', `/periods/${periodId}/appraisals/${employeeId}/self`, b),
  saveManager: (periodId: number, employeeId: number, b: ManagerPayload) =>
    send<AppraisalView>('put', `/periods/${periodId}/appraisals/${employeeId}/manager`, b),
  finalize: (periodId: number, employeeId: number) =>
    send<AppraisalView>('post', `/periods/${periodId}/appraisals/${employeeId}/finalize`),
  team: (periodId: number) => get<TeamView>(`/periods/${periodId}/team`),
}

// ---------- dasbor & laporan ----------
export const dashboard = {
  employee: (period_id?: number) => get<EmployeeDash>('/dashboard/employee', { period_id }),
  manager: (period_id?: number) => get<ManagerDash>('/dashboard/manager', { period_id }),
  director: (period_id?: number) => get<DirectorDash>('/dashboard/director', { period_id }),
}

export const reports = {
  scores: (params: { period_id?: number; department_id?: number }) => get<ReportView>('/reports/scores', params),
  export: (format: 'xlsx' | 'pdf', params: { period_id?: number; department_id?: number }) =>
    download('/reports/scores/export', { ...params, format }, `laporan-kinerja.${format}`),
}

export const notifications = {
  list: () => get<{ items: NotificationItem[]; unread: number }>('/notifications'),
  readAll: () => send<void>('post', '/notifications/read-all'),
  read: (id: number) => send<void>('post', `/notifications/${id}/read`),
}
