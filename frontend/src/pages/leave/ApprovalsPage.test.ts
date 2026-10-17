import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'
import ApprovalsPage from './ApprovalsPage.vue'
import { ApiError } from '@/api'
import { useAuthStore } from '@/stores/auth'
import { rina } from '@/test/fixtures'
import { makeLeave } from '@/test/leaveFixtures'

const api = vi.hoisted(() => ({
  approvals: vi.fn(),
  get: vi.fn(),
  approve: vi.fn(),
  reject: vi.fn(),
  approvalCount: vi.fn(),
}))

vi.mock('@/api', async (orig) => {
  const real = await orig<typeof import('@/api')>()
  return { ...real, leave: { ...real.leave, ...api } }
})

const router = createRouter({
  history: createMemoryHistory(),
  routes: [{ path: '/:p(.*)*', component: { template: '<div />' } }],
})

const putri = { ...rina, id: 9, name: 'Putri Maharani' }
const q1 = makeLeave({ id: 142, can_act: true })
const q2 = makeLeave({
  id: 139,
  number: 'CT-2026-0139',
  employee: putri,
  employee_id: 9,
  days: '3',
  start_date: '2026-11-04T00:00:00Z',
  end_date: '2026-11-06T00:00:00Z',
  can_act: true,
  can_cancel: false,
  reason: 'Mengurus pindahan rumah',
  conflicts: [],
  balance: { quota: '12', used: '5', pending: '0', available: '7', after: '4' },
})

async function mountPage() {
  const w = mount(ApprovalsPage, { global: { plugins: [router], stubs: { Icon: true } } })
  await flushPromises()
  return w
}

describe('ApprovalsPage (H-10)', () => {
  beforeEach(async () => {
    setActivePinia(createPinia())
    useAuthStore().setUser({ ...rina, id: 3, name: 'Bayu Pratama', roles: ['manager', 'employee'] })
    vi.clearAllMocks()
    await router.push('/persetujuan')
    api.approvals.mockResolvedValue([q1, q2])
    api.get.mockImplementation(async (id: number) => (id === 139 ? q2 : q1))
    api.approve.mockResolvedValue(q1)
    api.reject.mockResolvedValue(q1)
    api.approvalCount.mockResolvedValue({ pending: 0 })
    window.matchMedia = vi.fn().mockReturnValue({ matches: false }) as unknown as typeof window.matchMedia
  })

  it('menampilkan antrean dengan jenis, rentang tanggal, dan jumlah hari', async () => {
    const w = await mountPage()
    const item = w.get('[data-testid="q-139"]').text()
    expect(item).toContain('Putri Maharani')
    expect(item).toContain('Cuti tahunan')
    expect(item).toContain('4 – 6 Nov 2026')
    expect(item).toContain('3 hari')
    expect(w.get('[data-testid="tab-pending"]').text()).toContain('2')
  })

  it('memilih pengajuan menampilkan detail, saldo, dan pemeriksaan bentrok', async () => {
    const w = await mountPage()
    await w.get('[data-testid="q-139"]').trigger('click')
    await flushPromises()
    const d = w.get('[data-testid="detail"]').text()
    expect(d).toContain('Mengurus pindahan rumah')
    expect(w.get('[data-testid="balance"]').text()).toContain('7 hari → sisa 4 setelah disetujui')
    expect(w.get('[data-testid="conflict"]').text()).toContain('Tidak bentrok')
  })

  it('menandai bentrok dengan rekan satu departemen', async () => {
    api.get.mockResolvedValue({
      ...q2,
      conflicts: [
        {
          employee_name: 'Agus Setiawan',
          type_name: 'Cuti tahunan',
          range: '5 – 6 Nov 2026',
          status: 'Disetujui',
        },
      ],
    })
    const w = await mountPage()
    await w.get('[data-testid="q-139"]').trigger('click')
    await flushPromises()
    expect(w.get('[data-testid="conflict"]').text()).toContain('Ada yang bentrok')
    expect(w.get('[data-testid="conflict"]').text()).toContain('Agus Setiawan')
  })

  it('menolak tanpa catatan ditahan di klien; dengan catatan memanggil API', async () => {
    const w = await mountPage()
    await w.get('[data-testid="q-142"]').trigger('click')
    await flushPromises()
    await w.get('[data-testid="reject"]').trigger('click')
    await flushPromises()
    expect(w.text()).toContain('Catatan wajib diisi jika menolak')
    expect(api.reject).not.toHaveBeenCalled()

    await w.get('[data-testid="note"]').setValue('Sedang ada rilis besar')
    await w.get('[data-testid="reject"]').trigger('click')
    await flushPromises()
    expect(api.reject).toHaveBeenCalledWith(142, 'Sedang ada rilis besar')
  })

  it('tombol setuju: "teruskan ke HR" di tahap 1, "Setujui" di tahap akhir', async () => {
    const w = await mountPage()
    await w.get('[data-testid="q-142"]').trigger('click')
    await flushPromises()
    expect(w.get('[data-testid="approve"]').text()).toBe('Setujui & teruskan ke HR')

    api.get.mockResolvedValue({ ...q1, is_last_stage: true })
    await w.get('[data-testid="q-139"]').trigger('click')
    await flushPromises()
    expect(w.get('[data-testid="approve"]').text()).toBe('Setujui')

    await w.get('[data-testid="note"]').setValue('Silakan')
    await w.get('[data-testid="approve"]').trigger('click')
    await flushPromises()
    expect(api.approve).toHaveBeenCalledTimes(1)
    expect(api.approve.mock.calls[0]![1]).toBe('Silakan')
  })

  it('pengajuan yang sudah diproses orang lain (409) memuat ulang antrean dan memberi tahu', async () => {
    api.approve.mockRejectedValue(new ApiError(409, 'conflict', 'Pengajuan sudah diproses (Disetujui)'))
    const w = await mountPage()
    await w.get('[data-testid="q-142"]').trigger('click')
    await flushPromises()
    api.approvals.mockClear()
    await w.get('[data-testid="approve"]').trigger('click')
    await flushPromises()
    expect(api.approvals).toHaveBeenCalled()
  })

  it('antrean kosong menampilkan pesan, bukan daftar', async () => {
    api.approvals.mockResolvedValue([])
    const w = await mountPage()
    expect(w.text()).toContain('Tidak ada pengajuan yang menunggu keputusan Anda')
    expect(w.find('[data-testid="queue"]').exists()).toBe(false)
  })

  it('tab "Sudah diproses" memuat riwayat keputusan', async () => {
    const w = await mountPage()
    api.approvals.mockClear()
    await w.get('[data-testid="tab-history"]').trigger('click')
    await flushPromises()
    expect(api.approvals).toHaveBeenCalledWith('history')
  })
})
