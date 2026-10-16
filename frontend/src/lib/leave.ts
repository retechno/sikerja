// Pembantu tampilan izin & cuti: label status, hitung kalender, format hari.
import type { CalendarItem, LeaveRequest, LeaveStatus, LeaveType } from '@/api'

export const LEAVE_STATUS: Record<LeaveStatus, { label: string; cls: string }> = {
  pending_manager: { label: 'Menunggu Atasan', cls: 'b-warn' },
  pending_hr: { label: 'Menunggu HR', cls: 'b-warn' },
  approved: { label: 'Disetujui', cls: 'b-good' },
  rejected: { label: 'Ditolak', cls: 'b-bad' },
  cancelled: { label: 'Dibatalkan', cls: 'b-mute' },
}

/** Hari dengan koma desimal: "2", "0,5". Menerima string desimal dari API. */
export function fmtDays(v: unknown): string {
  const n = typeof v === 'number' ? v : Number(v)
  if (!Number.isFinite(n)) return '—'
  return new Intl.NumberFormat('id-ID', { maximumFractionDigits: 1 }).format(n)
}

const MONTHS = ['Jan', 'Feb', 'Mar', 'Apr', 'Mei', 'Jun', 'Jul', 'Agu', 'Sep', 'Okt', 'Nov', 'Des']
const MONTHS_LONG = [
  'Januari',
  'Februari',
  'Maret',
  'April',
  'Mei',
  'Juni',
  'Juli',
  'Agustus',
  'September',
  'Oktober',
  'November',
  'Desember',
]
const WEEKDAYS = ['Min', 'Sen', 'Sel', 'Rab', 'Kam', 'Jum', 'Sab']
const WEEKDAYS_LONG = ['Minggu', 'Senin', 'Selasa', 'Rabu', 'Kamis', 'Jumat', 'Sabtu']

function ymd(iso: string): [number, number, number] {
  const m = /^(\d{4})-(\d{2})-(\d{2})/.exec(iso)
  return m ? [Number(m[1]), Number(m[2]), Number(m[3])] : [0, 0, 0]
}

/** Hari dalam minggu dari teks tanggal, tanpa pergeseran zona waktu (0 = Minggu). */
export function weekdayOf(iso: string): number {
  const [y, m, d] = ymd(iso)
  return new Date(Date.UTC(y, m - 1, d)).getUTCDay()
}

/** "Sen 23 Nov" */
export function fmtShortDay(iso: string): string {
  const [, m, d] = ymd(iso)
  return `${WEEKDAYS[weekdayOf(iso)]} ${d} ${MONTHS[m - 1]}`
}

/** "Senin, 23 Nov 2026" */
export function fmtWeekdayDate(iso: string): string {
  if (!iso) return '—'
  const [y, m, d] = ymd(iso)
  return `${WEEKDAYS_LONG[weekdayOf(iso)]}, ${d} ${MONTHS[m - 1]} ${y}`
}

/** "30 Okt – 2 Nov 2026" / "4 – 6 Nov 2026" / "9 Okt 2026" */
export function fmtRange(startIso: string, endIso: string): string {
  const [y1, m1, d1] = ymd(startIso)
  const [y2, m2, d2] = ymd(endIso)
  if (!y1 || !y2) return '—'
  if (y1 === y2 && m1 === m2 && d1 === d2) return `${d1} ${MONTHS[m1 - 1]} ${y1}`
  if (y1 === y2 && m1 === m2) return `${d1} – ${d2} ${MONTHS[m2 - 1]} ${y2}`
  if (y1 === y2) return `${d1} ${MONTHS[m1 - 1]} – ${d2} ${MONTHS[m2 - 1]} ${y2}`
  return `${d1} ${MONTHS[m1 - 1]} ${y1} – ${d2} ${MONTHS[m2 - 1]} ${y2}`
}

/** Rentang untuk satu pengajuan, menyebut sesi bila setengah hari: "9 Okt 2026, pagi". */
export function leaveRangeText(
  l: Pick<LeaveRequest, 'start_date' | 'end_date' | 'half_day' | 'half_session'>,
): string {
  const base = fmtRange(l.start_date, l.end_date)
  return l.half_day && l.half_session ? `${base}, ${l.half_session}` : base
}

/** Nama jenis untuk daftar: izin setengah hari dibedakan. */
export function leaveTitle(l: Pick<LeaveRequest, 'half_day' | 'leave_type'>): string {
  const n = l.leave_type?.name ?? 'Pengajuan'
  return l.half_day ? `${n} setengah hari` : n
}

/** "hari ini", "kemarin", "3 hari lalu" dari tanggal ISO relatif ke sekarang. */
export function relativeDay(iso: string, now = new Date()): string {
  const t = new Date(iso)
  if (Number.isNaN(t.getTime())) return ''
  const a = new Date(t.getFullYear(), t.getMonth(), t.getDate()).getTime()
  const b = new Date(now.getFullYear(), now.getMonth(), now.getDate()).getTime()
  const diff = Math.round((b - a) / 86400000)
  if (diff <= 0) return 'hari ini'
  if (diff === 1) return 'kemarin'
  return `${diff} hari lalu`
}

// ---------- kalender ----------

export type CalendarKind = 'cuti' | 'sakit' | 'dinas' | 'izin'

/** Kelompok warna kalender menurut kode jenis (CS = sakit, DL = dinas luar, IZ = izin). */
export function calendarKind(code: string): CalendarKind {
  const c = code.toUpperCase()
  if (c === 'CS') return 'sakit'
  if (c === 'DL') return 'dinas'
  if (c === 'IZ') return 'izin'
  return 'cuti'
}

export const KIND_LABEL: Record<CalendarKind, string> = {
  cuti: 'cuti',
  sakit: 'sakit',
  dinas: 'dinas luar',
  izin: 'izin',
}

export interface GridDay {
  date: string
  day: number
  inMonth: boolean
  weekend: boolean
}

function pad(n: number): string {
  return String(n).padStart(2, '0')
}

/** Kisi bulan Senin–Minggu: minggu penuh yang mencakup seluruh bulan. month = "YYYY-MM". */
export function buildMonthGrid(month: string): GridDay[][] {
  const [y, m] = month.split('-').map(Number) as [number, number]
  const first = new Date(Date.UTC(y, m - 1, 1))
  const lead = (first.getUTCDay() + 6) % 7 // Senin = 0
  const start = new Date(Date.UTC(y, m - 1, 1 - lead))
  const last = new Date(Date.UTC(y, m, 0))
  const total = Math.ceil((lead + last.getUTCDate()) / 7) * 7
  const weeks: GridDay[][] = []
  for (let i = 0; i < total; i++) {
    const d = new Date(start.getTime() + i * 86400000)
    const dow = d.getUTCDay()
    const cell: GridDay = {
      date: `${d.getUTCFullYear()}-${pad(d.getUTCMonth() + 1)}-${pad(d.getUTCDate())}`,
      day: d.getUTCDate(),
      inMonth: d.getUTCMonth() === m - 1,
      weekend: dow === 0 || dow === 6,
    }
    if (i % 7 === 0) weeks.push([])
    weeks[weeks.length - 1]!.push(cell)
  }
  return weeks
}

/** Pengajuan yang mencakup tanggal tertentu. */
export function itemsOn(items: CalendarItem[], date: string): CalendarItem[] {
  return items.filter((i) => i.start_date.slice(0, 10) <= date && i.end_date.slice(0, 10) >= date)
}

export function shiftMonth(month: string, delta: number): string {
  const [y, m] = month.split('-').map(Number) as [number, number]
  const d = new Date(Date.UTC(y, m - 1 + delta, 1))
  return `${d.getUTCFullYear()}-${pad(d.getUTCMonth() + 1)}`
}

export function monthTitle(month: string): string {
  const [y, m] = month.split('-').map(Number) as [number, number]
  return `${MONTHS_LONG[m - 1]} ${y}`
}

export function currentMonth(now = new Date()): string {
  return `${now.getFullYear()}-${pad(now.getMonth() + 1)}`
}

export function todayYmd(now = new Date()): string {
  return `${now.getFullYear()}-${pad(now.getMonth() + 1)}-${pad(now.getDate())}`
}

/** Teks persetujuan untuk tabel jenis cuti (H-16). */
export function approvalText(t: Pick<LeaveType, 'allow_half_day'>): string {
  return t.allow_half_day ? 'Atasan (½ hari) · Atasan → HR' : 'Atasan → HR'
}

/** Ringkasan kuota per jenis untuk tabel H-16. */
export function quotaText(t: LeaveType): string {
  const parts: string[] = []
  if (t.annual_quota != null) parts.push(`${fmtDays(t.annual_quota)} hari`)
  else if (t.max_days_per_request != null)
    parts.push(`maks ${fmtDays(t.max_days_per_request)} hari / pengajuan`)
  else parts.push('Tidak dibatasi')
  if (t.min_tenure_months > 0) parts.push(`masa kerja ≥ ${t.min_tenure_months} bln`)
  return parts.join(' · ')
}

export function attachmentText(t: LeaveType): string {
  if (!t.requires_attachment) return 'Tidak'
  return Number(t.attachment_min_days) > 0 ? `Ya, jika > ${fmtDays(t.attachment_min_days)} hari` : 'Ya'
}
