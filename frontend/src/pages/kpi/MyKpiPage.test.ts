import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import MyKpiPage from './MyKpiPage.vue'
import { makeSheet, period, rina } from '@/test/fixtures'
import { useAuthStore } from '@/stores/auth'

const api = vi.hoisted(() => ({
  sheet: vi.fn(),
  saveResult: vi.fn(),
  uploadEvidence: vi.fn(),
  periods: vi.fn(),
}))

vi.mock('@/api', async (orig) => {
  const real = await orig<typeof import('@/api')>()
  return {
    ...real,
    kpi: {
      sheet: api.sheet,
      saveResult: api.saveResult,
      uploadEvidence: api.uploadEvidence,
      evidenceUrl: (id: number) => `/e/${id}`,
    },
    periods: { ...real.periods, list: api.periods },
  }
})

async function mountPage() {
  const w = mount(MyKpiPage, { global: { stubs: { Icon: true } } })
  await flushPromises()
  return w
}

describe('MyKpiPage (H-05) — input realisasi', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    useAuthStore().setUser(rina)
    vi.clearAllMocks()
    api.periods.mockResolvedValue([period])
    api.sheet.mockResolvedValue(makeSheet())
    api.saveResult.mockResolvedValue({ id: 103 })
  })

  it('mengirim realisasi bertipe angka (regresi: v-model input number bukan string)', async () => {
    const w = await mountPage()
    await w.get('[data-testid="kpi-action-4"]').trigger('click')
    const input = w.get('[data-testid="actual-input"]')
    await input.setValue('4') // jsdom + v-model pada type=number → number
    expect(w.get('[data-testid="preview"]').text()).toBe('100,0') // 4 dari target 4

    await w.get('form').trigger('submit')
    await flushPromises()

    expect(api.saveResult).toHaveBeenCalledTimes(1)
    expect(api.saveResult.mock.calls[0]![0]).toMatchObject({
      period_id: 6,
      employee_id: 4,
      indicator_id: 4,
      actual: 4,
    })
    expect(api.uploadEvidence).not.toHaveBeenCalled()
  })

  it('menolak realisasi kosong atau negatif tanpa memanggil API', async () => {
    const w = await mountPage()
    await w.get('[data-testid="kpi-action-4"]').trigger('click')
    await w.get('[data-testid="actual-input"]').setValue('')
    await w.get('form').trigger('submit')
    await flushPromises()
    expect(w.text()).toContain('Isi angka realisasi')

    await w.get('[data-testid="actual-input"]').setValue('-2')
    await w.get('form').trigger('submit')
    await flushPromises()
    expect(api.saveResult).not.toHaveBeenCalled()
  })

  it('periode terkunci: form tidak bisa disimpan dan ada penjelasan', async () => {
    api.sheet.mockResolvedValue(makeSheet({ can_input: false, period: { ...period, status: 'closed' } }))
    const w = await mountPage()
    expect(w.text()).toContain('sudah ditutup')
    expect(w.find('[data-testid="save"]').exists()).toBe(false)
    expect(w.get('[data-testid="actual-input"]').attributes('disabled')).toBeDefined()
  })

  it('menampilkan ringkasan: skor sementara 95,7 dan 4 dari 5 tervalidasi (mockup H-05)', async () => {
    const w = await mountPage()
    expect(w.get('[data-testid="score-card"]').text()).toContain('95,7')
    expect(w.get('[data-testid="validated-card"]').text()).toContain('4')
    expect(w.get('[data-testid="validated-card"]').text()).toContain('1 menunggu validasi')
  })
})
