import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'
import LeaveHistoryPage from './LeaveHistoryPage.vue'
import { makeLeave } from '@/test/leaveFixtures'

const api = vi.hoisted(() => ({ list: vi.fn(), get: vi.fn(), cancel: vi.fn() }))

vi.mock('@/api', async (orig) => {
  const real = await orig<typeof import('@/api')>()
  return { ...real, leave: { ...real.leave, ...api } }
})

const router = createRouter({
  history: createMemoryHistory(),
  routes: [{ path: '/:p(.*)*', component: { template: '<div />' } }],
})

const meta = { page: 1, per_page: 100, total: 3 }
const pending = makeLeave()
const approvedHalf = makeLeave({
  id: 118,
  number: 'IZ-2026-0118',
  leave_type: { ...makeLeave().leave_type!, id: 3, name: 'Izin', code: 'IZ' },
  start_date: '2026-10-09T00:00:00Z',
  end_date: '2026-10-09T00:00:00Z',
  half_day: true,
  half_session: 'pagi',
  days: '0.5',
  status: 'approved',
  can_cancel: false,
})
const rejected = makeLeave({ id: 48, number: 'CT-2026-0048', status: 'rejected', can_cancel: false })

async function mountPage(query = '') {
  await router.push('/izin-cuti/riwayat' + query)
  const w = mount(LeaveHistoryPage, { global: { plugins: [router], stubs: { Icon: true } } })
  await flushPromises()
  return w
}

describe('LeaveHistoryPage (H-09)', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    api.list.mockResolvedValue({ data: [pending, approvedHalf, rejected], meta })
    api.get.mockImplementation(async (id: number) =>
      [pending, approvedHalf, rejected].find((l) => l.id === id),
    )
    api.cancel.mockResolvedValue({ ...pending, status: 'cancelled', can_cancel: false })
    window.matchMedia = vi.fn().mockReturnValue({ matches: false }) as unknown as typeof window.matchMedia
  })

  it('daftar menampilkan nomor, jenis, tanggal, hari, dan status (mockup H-09)', async () => {
    const w = await mountPage()
    const t = w.get('[data-testid="history"]').text()
    expect(t).toContain('CT-2026-0142')
    expect(t).toContain('30 Okt – 2 Nov 2026')
    expect(t).toContain('Menunggu Atasan')
    expect(t).toContain('Izin setengah hari')
    expect(t).toContain('9 Okt 2026, pagi')
    expect(t).toContain('0,5')
    expect(t).toContain('Disetujui')
    expect(t).toContain('Ditolak')
    expect(w.get('[data-testid="count"]').text()).toContain('3 pengajuan')
  })

  it('filter status & tahun dikirim ke API', async () => {
    const w = await mountPage()
    api.list.mockClear()
    await w.get('select[aria-label="Status"]').setValue('approved')
    await flushPromises()
    expect(api.list.mock.calls[0]![0]).toMatchObject({ status: 'approved' })
  })

  it('memilih baris menampilkan linimasa dan tombol batalkan beserta batas tanggalnya', async () => {
    const w = await mountPage('?id=142')
    const d = w.get('[data-testid="detail"]').text()
    expect(d).toContain('Menghadiri wisuda adik di Malang')
    expect(d).toContain('Diajukan')
    expect(d).toContain('Menunggu atasan langsung')
    expect(d).toContain('Bisa dibatalkan sampai 29 Okt 2026')
    expect(w.find('[data-testid="cancel"]').exists()).toBe(true)
  })

  it('pengajuan yang tidak bisa dibatalkan tidak menampilkan tombol', async () => {
    const w = await mountPage('?id=48')
    expect(w.find('[data-testid="cancel"]').exists()).toBe(false)
  })

  it('membatalkan: konfirmasi dulu, baru memanggil API', async () => {
    const w = await mountPage('?id=142')
    await w.get('[data-testid="cancel"]').trigger('click')
    expect(api.cancel).not.toHaveBeenCalled()
    await document.body.querySelector<HTMLButtonElement>('[data-testid="confirm-cancel"]')!.click()
    await flushPromises()
    expect(api.cancel).toHaveBeenCalledWith(142)
    expect(w.get('[data-testid="detail"]').text()).toContain('Dibatalkan')
  })
})
