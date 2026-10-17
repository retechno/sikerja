// Tipe respons API (lihat docs/api.yaml). Angka desimal dikirim sebagai string.

export type Dec = string
export type Role = 'employee' | 'manager' | 'hr' | 'director'

export interface Department {
  id: number
  name: string
  code: string
  head_id: number | null
}

export interface Position {
  id: number
  name: string
  department_id: number | null
}

export interface Employee {
  id: number
  user_id: number
  nik: string
  name: string
  email: string
  phone: string
  department_id: number | null
  position_id: number | null
  manager_id: number | null
  join_date: string
  status: 'active' | 'inactive'
  roles: Role[]
  has_photo: boolean
  department?: Department | null
  position?: Position | null
}

export interface Period {
  id: number
  name: string
  type: 'monthly' | 'quarterly'
  start_date: string
  end_date: string
  status: 'draft' | 'active' | 'closed'
  kpi_weight: Dec
  behavior_weight: Dec
}

export interface Indicator {
  id: number
  kpi_template_id?: number
  name: string
  unit: string
  target: Dec
  weight: Dec
  direction: 'higher' | 'lower'
  source: 'manual' | 'integration'
  sort: number
}

export interface Template {
  id: number
  department_id: number
  name: string
  is_active: boolean
  total_weight: Dec
  indicators: Indicator[]
  department?: Department
}

export type ResultStatus = 'missing' | 'submitted' | 'validated' | 'rejected'

export interface KpiResult {
  id: number
  period_id: number
  employee_id: number
  indicator_id: number
  actual: Dec
  achievement: Dec
  note: string
  evidence_name: string
  status: Exclude<ResultStatus, 'missing'>
  validation_note: string
}

export interface KpiRow {
  indicator: Indicator
  result: KpiResult | null
  achievement: Dec
  points: Dec
  status: ResultStatus
  has_evidence: boolean
}

export interface KpiSheet {
  employee: Employee
  period: Period
  template_id: number
  rows: KpiRow[] | null
  score: Dec
  total: number
  validated: number
  pending: number
  rejected: number
  missing: number
  all_validated: boolean
  can_input: boolean
  can_validate: boolean
}

export type AppraisalStatus = 'draft' | 'self_done' | 'manager_done' | 'final'

export interface AspectRow {
  aspect: { id: number; name: string; description: string }
  self_score: number | null
  manager_score: number | null
}

export interface AppraisalView {
  appraisal_id: number
  status: AppraisalStatus
  employee: Employee
  period: Period
  aspects: AspectRow[] | null
  employee_comment: string
  manager_comment: string
  development_plan: string
  kpi: KpiSheet
  kpi_score: Dec
  behavior_score: Dec | null
  final_score: Dec | null
  grade: string
  previous: { period_name: string; final_score: Dec; grade: string } | null
  self_submitted_at: string | null
  manager_submitted_at: string | null
  finalized_at: string | null
  can_self: boolean
  can_manager: boolean
  can_finalize: boolean
  hidden_from_employee: boolean
}

export interface SelfPayload {
  scores: Record<number, number>
  comment: string
  submit: boolean
}

export interface ManagerPayload {
  scores: Record<number, number>
  manager_comment: string
  development_plan: string
  submit: boolean
}

export interface TeamRow {
  employee: Employee
  status: AppraisalStatus
  self_done: boolean
  kpi_score: Dec
  kpi_total: number
  kpi_validated: number
  kpi_pending: number
  final_score: Dec | null
  grade: string
}

export interface TeamView {
  period: Period
  rows: TeamRow[]
  summary: {
    members: number
    appraised: number
    finalized: number
    pending_kpi: number
    not_appraised: number
  }
}

export interface EmployeeDash {
  period: Period
  employee: Employee
  sheet: KpiSheet
  kpi_score: Dec
  kpi_delta: Dec | null
  prev_period_name: string
  appraisal_status: AppraisalStatus
  final_score: Dec | null
  grade: string
  latest_final: {
    period_name: string
    score: Dec
    grade: string
    next: { grade: string; gap: Dec } | null
  } | null
  deadline: string
  days_left: number
  pending_validation: number
  history: { period_id: number; name: string; score: Dec | null; grade: string; current: boolean }[]
}

export interface ManagerDash {
  period: Period
  prev_period_name: string
  prev_avg: Dec | null
  prev_avg_delta: Dec | null
  pending_validations: number
  members_pending: number
  members_total: number
  not_appraised: number
  low_members: { employee: Employee; score: Dec }[]
  ranking: {
    employee: Employee
    prev_score: Dec | null
    prev_grade: string
    kpi_score: Dec
    final_score: Dec | null
    status: AppraisalStatus
  }[]
  below_target: {
    employee: Employee
    indicator: string
    unit: string
    direction: 'higher' | 'lower'
    actual: Dec
    target: Dec
    achievement: Dec
  }[]
}

export interface ScoreItem {
  employee_id: number
  name: string
  department: string
  score: Dec
  grade: string
}

export interface DirectorDash {
  period: Period
  employees: number
  scored: number
  company_avg: Dec | null
  company_delta: Dec | null
  company_grade: string
  departments: { department_id: number; name: string; avg: Dec; count: number }[]
  best_department: { name: string; avg: Dec } | null
  trend: { period_id: number; name: string; avg: Dec | null }[]
  grades: { grade: string; count: number }[]
  attention: number
  top: ScoreItem[]
  bottom: ScoreItem[]
}

export interface ReportRow {
  employee_id: number
  nik: string
  name: string
  department: string
  position: string
  kpi_score: Dec | null
  behavior_score: Dec | null
  final_score: Dec | null
  grade: string
  status: string
}

export interface ReportView {
  period: Period
  department_id: number
  department_name: string
  rows: ReportRow[]
  summary: { total: number; scored: number; avg: Dec | null }
}

export interface ImportResult {
  created: number
  positions_created: number
  errors: { row: number; message: string }[]
  credentials: { nik: string; email: string; temp_password: string }[]
}

export interface NotificationItem {
  id: number
  type: string
  data: string
  read_at: string | null
  created_at: string
}

export interface PageMeta {
  page: number
  per_page: number
  total: number
}

export interface PeriodProgress {
  period_id: number
  employees: number
  expected_results: number
  results: number
  validated: number
  self_done: number
  manager_done: number
  finalized: number
  avg_final: Dec | null
}
