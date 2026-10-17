import type { LeaveBalance, LeavePreview, LeaveRequest, LeaveType, LeaveStatus, WorkingDays } from '@/api'
import { rina } from './fixtures'

const base = {
  attachment_min_days: '0',
  allow_half_day: false,
  min_tenure_months: 0,
  max_days_per_request: null,
  sort: 0,
  is_active: true,
  requires_attachment: false,
}

export const CT: LeaveType = {
  ...base,
  id: 1,
  name: 'Cuti tahunan',
  code: 'CT',
  annual_quota: '12',
  deducts_balance: true,
  min_tenure_months: 12,
}
export const CS: LeaveType = {
  ...base,
  id: 2,
  name: 'Cuti sakit',
  code: 'CS',
  annual_quota: null,
  deducts_balance: false,
  requires_attachment: true,
  attachment_min_days: '1',
}
export const IZ: LeaveType = {
  ...base,
  id: 3,
  name: 'Izin',
  code: 'IZ',
  annual_quota: '6',
  deducts_balance: true,
  allow_half_day: true,
}
export const DL: LeaveType = {
  ...base,
  id: 4,
  name: 'Dinas luar',
  code: 'DL',
  annual_quota: null,
  deducts_balance: false,
  requires_attachment: true,
}
export const types: LeaveType[] = [CT, CS, IZ, DL]

/** Saldo mockup H-08: cuti tahunan 12, terpakai 2, menunggu 2 → tersedia 8; izin terpakai 1,5. */
export const balances: LeaveBalance[] = [
  { leave_type: CT, year: 2026, tracked: true, quota: '12', used: '2', pending: '2', available: '8' },
  { leave_type: CS, year: 2026, tracked: false, quota: null, used: '2', pending: '0', available: null },
  { leave_type: IZ, year: 2026, tracked: true, quota: '6', used: '1.5', pending: '0', available: '4.5' },
  { leave_type: DL, year: 2026, tracked: false, quota: null, used: '0', pending: '0', available: null },
]

export function workingDays(
  days: string,
  dates: string[],
  skipped: WorkingDays['skipped'] = [],
): WorkingDays {
  return {
    days,
    half_day: false,
    schedule: 'Jam kantor (Sen-Jum 08.00-17.00)',
    calendar_days: dates.length,
    breakdown: dates.map((date) => ({
      date,
      weekday: 'Sen',
      counted: !skipped.some((s) => s.date === date),
      hours: skipped.some((s) => s.date === date) ? undefined : '08:00–17:00',
    })),
    skipped,
  }
}

export function makePreview(over: Partial<LeavePreview> = {}): LeavePreview {
  return {
    working_days: workingDays('5', ['2026-11-23', '2026-11-24', '2026-11-25', '2026-11-26', '2026-11-27']),
    flow: [
      { level: 1, role: 'Atasan langsung', names: ['Bayu Pratama'] },
      { level: 2, role: 'HR', names: ['Sinta Maharani'] },
    ],
    balance: null,
    warnings: [],
    ...over,
  }
}

export function makeLeave(over: Partial<LeaveRequest> = {}): LeaveRequest {
  return {
    id: 142,
    number: 'CT-2026-0142',
    employee_id: rina.id,
    employee: rina,
    leave_type_id: CT.id,
    leave_type: CT,
    start_date: '2026-10-30T00:00:00Z',
    end_date: '2026-11-02T00:00:00Z',
    half_day: false,
    half_session: null,
    start_time: null,
    end_time: null,
    days: '2',
    reason: 'Menghadiri wisuda adik di Malang',
    substitute: null,
    status: 'pending_manager' as LeaveStatus,
    status_label: 'Menunggu Atasan',
    deducted: false,
    has_attachment: false,
    attachment_name: '',
    can_cancel: true,
    cancel_until: '2026-10-29',
    can_act: false,
    is_last_stage: false,
    created_at: '2026-10-16T09:12:00+08:00',
    timeline: [
      { title: 'Diajukan', actor: 'Rina Wulandari', at: '2026-10-16T09:12:00+08:00', state: 'done' },
      { title: 'Menunggu atasan langsung', actor: 'Bayu Pratama', at: null, state: 'current' },
      { title: 'Persetujuan hr', actor: 'Sinta Maharani', at: null, state: 'todo' },
    ],
    skipped: [],
    conflicts: [],
    ...over,
  }
}
