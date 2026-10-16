import { describe, expect, it } from 'vitest'
import {
  approvalText,
  attachmentText,
  buildMonthGrid,
  calendarKind,
  fmtDays,
  fmtRange,
  fmtShortDay,
  fmtWeekdayDate,
  itemsOn,
  leaveRangeText,
  monthTitle,
  quotaText,
  relativeDay,
  shiftMonth,
  weekdayOf,
} from './leave'
import type { CalendarItem } from '@/api'
import { CS, CT, IZ } from '@/test/leaveFixtures'

describe('format hari & tanggal', () => {
  it.each([
    ['2', '2'],
    ['0.5', '0,5'],
    [4.5, '4,5'],
    ['12', '12'],
    ['x', '—'],
  ])('fmtDays(%s) = %s', (v, want) => {
    expect(fmtDays(v)).toBe(want)
  })

  it.each([
    ['2026-10-30T00:00:00Z', '2026-11-02T00:00:00Z', '30 Okt – 2 Nov 2026'],
    ['2026-11-04', '2026-11-06', '4 – 6 Nov 2026'],
    ['2026-10-09', '2026-10-09', '9 Okt 2026'],
    ['2026-12-30', '2027-01-04', '30 Des 2026 – 4 Jan 2027'],
  ])('fmtRange(%s, %s)', (a, b, want) => {
    expect(fmtRange(a, b)).toBe(want)
  })

  it('menyebut sesi untuk setengah hari', () => {
    expect(
      leaveRangeText({
        start_date: '2026-10-09',
        end_date: '2026-10-09',
        half_day: true,
        half_session: 'pagi',
      }),
    ).toBe('9 Okt 2026, pagi')
  })

  it('hari dalam minggu dihitung dari teks tanggal (tanpa pergeseran zona waktu)', () => {
    expect(weekdayOf('2026-11-23')).toBe(1) // Senin
    expect(weekdayOf('2026-11-01T00:00:00Z')).toBe(0) // Minggu
    expect(fmtShortDay('2026-11-23')).toBe('Sen 23 Nov')
    expect(fmtWeekdayDate('2026-11-27')).toBe('Jumat, 27 Nov 2026')
  })

  it('relativeDay', () => {
    const now = new Date(2026, 9, 16, 12)
    expect(relativeDay(new Date(2026, 9, 16, 8).toISOString(), now)).toBe('hari ini')
    expect(relativeDay(new Date(2026, 9, 15, 20).toISOString(), now)).toBe('kemarin')
    expect(relativeDay(new Date(2026, 9, 14, 9).toISOString(), now)).toBe('2 hari lalu')
  })
})

describe('kalender bulanan', () => {
  it('November 2026 mulai hari Minggu → 6 minggu Senin–Minggu, diawali 26 Okt', () => {
    const weeks = buildMonthGrid('2026-11')
    expect(weeks).toHaveLength(6)
    expect(weeks.every((w) => w.length === 7)).toBe(true)
    expect(weeks[0]![0]).toMatchObject({ date: '2026-10-26', day: 26, inMonth: false, weekend: false })
    expect(weeks[0]![6]).toMatchObject({ date: '2026-11-01', inMonth: true, weekend: true })
    expect(weeks[5]![6]!.date).toBe('2026-12-06')
  })

  it('Februari 2027 tepat 4 minggu (mulai Senin, 28 hari)', () => {
    const weeks = buildMonthGrid('2027-02')
    expect(weeks).toHaveLength(4)
    expect(weeks[0]![0]).toMatchObject({ date: '2027-02-01', inMonth: true })
    expect(weeks[3]![6]).toMatchObject({ date: '2027-02-28', inMonth: true })
  })

  it('akhir pekan ditandai di kolom Sabtu dan Minggu saja', () => {
    const weeks = buildMonthGrid('2026-10')
    for (const w of weeks) w.forEach((d, i) => expect(d.weekend).toBe(i >= 5))
  })

  it('shiftMonth melewati pergantian tahun', () => {
    expect(shiftMonth('2026-12', 1)).toBe('2027-01')
    expect(shiftMonth('2026-01', -1)).toBe('2025-12')
    expect(monthTitle('2026-11')).toBe('November 2026')
  })

  it('itemsOn: mencakup tanggal awal, tengah, dan akhir', () => {
    const it = (id: number, a: string, b: string): CalendarItem => ({
      id,
      employee_id: id,
      employee_name: 'X',
      department_id: 1,
      department_name: 'IT',
      type_code: 'CT',
      type_name: 'Cuti',
      start_date: a,
      end_date: b,
      half_day: false,
      status: 'approved',
      days: '3',
    })
    const items = [it(1, '2026-11-04', '2026-11-06'), it(2, '2026-11-06', '2026-11-06')]
    expect(itemsOn(items, '2026-11-03')).toHaveLength(0)
    expect(itemsOn(items, '2026-11-04').map((i) => i.id)).toEqual([1])
    expect(itemsOn(items, '2026-11-05').map((i) => i.id)).toEqual([1])
    expect(itemsOn(items, '2026-11-06').map((i) => i.id)).toEqual([1, 2])
    expect(itemsOn(items, '2026-11-07')).toHaveLength(0)
  })

  it.each([
    ['CT', 'cuti'],
    ['CK-N', 'cuti'],
    ['CS', 'sakit'],
    ['DL', 'dinas'],
    ['iz', 'izin'],
  ])('calendarKind(%s) = %s', (code, want) => {
    expect(calendarKind(code)).toBe(want)
  })
})

describe('ringkasan jenis cuti (H-16)', () => {
  it('kuota, lampiran, dan alur persetujuan', () => {
    expect(quotaText(CT)).toBe('12 hari · masa kerja ≥ 12 bln')
    expect(quotaText(CS)).toBe('Tidak dibatasi')
    expect(quotaText({ ...CS, max_days_per_request: '3' })).toBe('maks 3 hari / pengajuan')
    expect(attachmentText(CS)).toBe('Ya, jika > 1 hari')
    expect(attachmentText({ ...CS, attachment_min_days: '0' })).toBe('Ya')
    expect(attachmentText(CT)).toBe('Tidak')
    expect(approvalText(IZ)).toBe('Atasan (½ hari) · Atasan → HR')
    expect(approvalText(CT)).toBe('Atasan → HR')
  })
})
