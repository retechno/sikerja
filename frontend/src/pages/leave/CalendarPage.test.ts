import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import CalendarPage from './CalendarPage.vue'
import type { CalendarItem, LeaveCalendar } from '@/api'
import { useAuthStore } from '@/stores/auth'
import { rina } from '@/test/fixtures'

const api = vi.hoisted(() => ({ calendar: vi.fn(), departments: vi.fn() }))

vi.mock('@/api', async (orig) => {
  const real = await orig<typeof import('@/api')>()
  return {
    ...real,
    leave: { ...real.leave, calendar: api.calendar },
    master: { ...real.master, departments: api.departments },
  }
})

const item = (over: Partial<CalendarItem>): CalendarItem => ({
  id: 1,
  employee_id: 4,
  employee_name: 'Rina Wulandari',
  department_id: 1,
  department_name: 'IT Support',
  type_code: 'CT',
  type_name: 'Cuti tahunan',
  start_date: '2026-11-23',
  end_date: '2026-11-27',
  half_day: false,
  status: 'approved',
  days: '5',
  ...over,
})

const cal = (items: CalendarItem[]): LeaveCalendar => ({
  month: '2026-11',
  from: '2026-11-01',
  to: '2026-11-30',
  items,
  holidays: [{ id: 1, date: '2026-11-11T00:00:00Z', name: 'Libur contoh', type: 'nasional' }],
})

async function mountPage() {
  const w = mount(CalendarPage, { global: { stubs: { Icon: true } } })
  await flushPromises()
  return w
}

describe('CalendarPage (H-11)', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    useAuthStore().setUser(rina)
    vi.clearAllMocks()
    // tanggal diset ke November 2026 agar kalender membuka bulan yang sama dengan data uji
    vi.useFakeTimers({ toFake: ['Date'] })
    vi.setSystemTime(new Date(2026, 10, 16, 10))
    api.departments.mockResolvedValue([
      { id: 1, name: 'IT Support', code: 'ITS', head_id: 3 },
      { id: 2, name: 'Customer Service', code: 'CS', head_id: 9 },
    ])
    api.calendar.mockResolvedValue(
      cal([
        item({}),
        item({
          id: 2,
          employee_id: 5,
          employee_name: 'Putri Maharani',
          start_date: '2026-11-04',
          end_date: '2026-11-06',
          status: 'pending_manager',
          days: '3',
        }),
        item({
          id: 3,
          employee_id: 6,
          employee_name: 'Agus Setiawan',
          type_code: 'CS',
          type_name: 'Cuti sakit',
          start_date: '2026-11-17',
          end_date: '2026-11-17',
          days: '1',
        }),
        item({
          id: 4,
          employee_id: 7,
          employee_name: 'Dimas Saputra',
          type_code: 'DL',
          type_name: 'Dinas luar',
          start_date: '2026-11-10',
          end_date: '2026-11-12',
          days: '3',
        }),
      ]),
    )
  })

  afterEach(() => vi.useRealTimers())

  it('memuat departemen sendiri secara default, termasuk yang menunggu', async () => {
    await mountPage()
    expect(api.calendar.mock.calls[0]![0]).toMatchObject({ department_id: 1, include_pending: true })
  })

  it('menampilkan nama dan jenis pada setiap hari yang dicakup', async () => {
    const w = await mountPage()
    const day = (d: string) => w.get(`[data-date="${d}"]`).text()
    expect(day('2026-11-23')).toContain('Rina · cuti')
    expect(day('2026-11-25')).toContain('Rina · cuti')
    expect(day('2026-11-27')).toContain('Rina · cuti')
    expect(day('2026-11-28')).not.toContain('Rina')
    expect(day('2026-11-17')).toContain('Agus · sakit')
    expect(day('2026-11-11')).toContain('Dimas · dinas luar')
    expect(day('2026-11-05')).toContain('Putri · cuti')
  })

  it('hari libur diberi nama', async () => {
    const w = await mountPage()
    expect(w.get('[data-date="2026-11-11"]').text()).toContain('Libur contoh')
  })

  it('daftar "tidak di kantor" memuat tiap orang dan menandai yang menunggu', async () => {
    const w = await mountPage()
    const list = w.get('[data-testid="out-list"]').text()
    expect(list).toContain('Rina Wulandari')
    expect(list).toContain('23 – 27 Nov 2026')
    expect(list).toContain('Putri Maharani')
    expect(list).toContain('menunggu')
  })

  it('pindah bulan dan ganti departemen memuat ulang dari API', async () => {
    const w = await mountPage()
    api.calendar.mockClear()
    await w.get('[data-testid="next"]').trigger('click')
    await flushPromises()
    expect(api.calendar.mock.calls[0]![0]).toMatchObject({ month: expect.stringMatching(/^\d{4}-\d{2}$/) })
    const first = api.calendar.mock.calls[0]![0].month as string

    api.calendar.mockClear()
    await w.get('[data-testid="dept"]').setValue('0')
    await flushPromises()
    expect(api.calendar.mock.calls[0]![0]).toMatchObject({ month: first, department_id: undefined })

    api.calendar.mockClear()
    await w.get('[data-testid="pending"]').setValue(false)
    await flushPromises()
    expect(api.calendar.mock.calls[0]![0]).toMatchObject({ include_pending: false })
  })

  it('bulan tanpa izin/cuti menampilkan pesan kosong', async () => {
    api.calendar.mockResolvedValue(cal([]))
    const w = await mountPage()
    expect(w.get('[data-testid="out-list"]').text()).toContain('Tidak ada izin atau cuti pada bulan ini')
  })
})
