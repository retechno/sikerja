import type {
  AppraisalView,
  Employee,
  Indicator,
  KpiResult,
  KpiRow,
  KpiSheet,
  Period,
  ResultStatus,
} from '@/api'
import { achievement } from '@/lib/scoring'

export const period: Period = {
  id: 6,
  name: 'Oktober 2026',
  type: 'monthly',
  start_date: '2026-10-01T00:00:00Z',
  end_date: '2026-10-31T00:00:00Z',
  status: 'active',
  kpi_weight: '70',
  behavior_weight: '30',
}

export const rina: Employee = {
  id: 4,
  user_id: 4,
  nik: '2021-0047',
  name: 'Rina Wulandari',
  email: 'rina.wulandari@sikerja.test',
  phone: '',
  department_id: 1,
  position_id: 1,
  manager_id: 3,
  join_date: '2021-05-03T00:00:00Z',
  status: 'active',
  roles: ['employee'],
  has_photo: false,
}

// Lima indikator IT Support + realisasi dari mockup H-05 (instalasi 3 dari 4 = menunggu validasi).
const defs: {
  name: string
  unit: string
  target: number
  weight: number
  dir: 'higher' | 'lower'
  actual: number
  status: ResultStatus
}[] = [
  {
    name: 'Tiket terselesaikan',
    unit: 'tiket',
    target: 120,
    weight: 25,
    dir: 'higher',
    actual: 104,
    status: 'validated',
  },
  {
    name: 'Tiket selesai dalam SLA',
    unit: '%',
    target: 95,
    weight: 25,
    dir: 'higher',
    actual: 96.2,
    status: 'validated',
  },
  {
    name: 'Waktu respons pertama',
    unit: 'menit',
    target: 15,
    weight: 15,
    dir: 'lower',
    actual: 12,
    status: 'validated',
  },
  {
    name: 'Instalasi / training PMS di hotel klien',
    unit: 'kegiatan',
    target: 4,
    weight: 20,
    dir: 'higher',
    actual: 3,
    status: 'submitted',
  },
  {
    name: 'Rating kepuasan klien',
    unit: 'dari 5',
    target: 4.5,
    weight: 15,
    dir: 'higher',
    actual: 4.7,
    status: 'validated',
  },
]

export function makeRows(statusOverride: Partial<Record<number, ResultStatus>> = {}): KpiRow[] {
  return defs.map((d, i) => {
    const indicator: Indicator = {
      id: i + 1,
      name: d.name,
      unit: d.unit,
      target: String(d.target),
      weight: String(d.weight),
      direction: d.dir,
      source: 'manual',
      sort: i + 1,
    }
    const status = statusOverride[i + 1] ?? d.status
    const ach = achievement(d.actual, d.target, d.dir)
    const result: KpiResult | null =
      status === 'missing'
        ? null
        : {
            id: 100 + i,
            period_id: 6,
            employee_id: 4,
            indicator_id: i + 1,
            actual: String(d.actual),
            achievement: String(ach),
            note: '',
            evidence_name: '',
            status,
            validation_note: status === 'rejected' ? 'Bukti kurang lengkap' : '',
          }
    return {
      indicator,
      result,
      achievement: status === 'missing' ? '0' : String(ach),
      points: status === 'missing' ? '0' : String((ach * d.weight) / 100),
      status,
      has_evidence: false,
    }
  })
}

export function makeSheet(over: Partial<KpiSheet> = {}): KpiSheet {
  const rows = over.rows ?? makeRows()
  return {
    employee: rina,
    period,
    template_id: 1,
    rows,
    score: '95.65',
    total: rows.length,
    validated: rows.filter((r) => r.status === 'validated').length,
    pending: rows.filter((r) => r.status === 'submitted').length,
    rejected: 0,
    missing: 0,
    all_validated: false,
    can_input: true,
    can_validate: false,
    ...over,
  }
}

export const aspects = ['Disiplin', 'Kerja sama tim', 'Inisiatif', 'Komunikasi', 'Tanggung jawab'].map(
  (name, i) => ({
    id: i + 1,
    name,
    description: `Deskripsi ${name}`,
  }),
)

export function makeView(
  over: Partial<AppraisalView> = {},
  manager: (number | null)[] = [null, null, null, null, null],
): AppraisalView {
  const self = [5, 4, 4, 4, 5]
  return {
    appraisal_id: 1,
    status: 'self_done',
    employee: rina,
    period,
    aspects: aspects.map((a, i) => ({
      aspect: a,
      self_score: self[i] ?? null,
      manager_score: manager[i] ?? null,
    })),
    employee_comment: 'Menangani migrasi data PMS lebih cepat dari jadwal.',
    manager_comment: '',
    development_plan: '',
    kpi: makeSheet(),
    kpi_score: '95.65',
    behavior_score: null,
    final_score: null,
    grade: '',
    previous: null,
    self_submitted_at: null,
    manager_submitted_at: null,
    finalized_at: null,
    can_self: false,
    can_manager: true,
    can_finalize: false,
    hidden_from_employee: false,
    ...over,
  }
}
