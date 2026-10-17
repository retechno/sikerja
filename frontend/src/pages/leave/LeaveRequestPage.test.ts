import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'
import LeaveRequestPage from './LeaveRequestPage.vue'
import { ApiError } from '@/api'
import { balances, makeLeave, makePreview, types, workingDays } from '@/test/leaveFixtures'

const api = vi.hoisted(() => ({
  types: vi.fn(),
  balances: vi.fn(),
  substitutes: vi.fn(),
  preview: vi.fn(),
  submit: vi.fn(),
}))

vi.mock('@/api', async (orig) => {
  const real = await orig<typeof import('@/api')>()
  return { ...real, leave: { ...real.leave, ...api } }
})

const router = createRouter({
  history: createMemoryHistory(),
  routes: [{ path: '/:p(.*)*', component: { template: '<div />' } }],
})

async function mountPage() {
  const w = mount(LeaveRequestPage, { global: { plugins: [router], stubs: { Icon: true } } })
  await flushPromises()
  return w
}
const tick = async (ms = 300) => {
  await vi.advanceTimersByTimeAsync(ms)
  await flushPromises()
}

describe('LeaveRequestPage (H-08)', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] })
    api.types.mockResolvedValue(types)
    api.balances.mockResolvedValue(balances)
    api.substitutes.mockResolvedValue([{ id: 5, name: 'Dimas Saputra' }])
    api.preview.mockResolvedValue(makePreview())
    api.submit.mockResolvedValue(makeLeave({ id: 77, number: 'CT-2026-0077' }))
    vi.spyOn(router, 'push').mockResolvedValue(undefined)
  })
  afterEach(() => vi.useRealTimers())

  it('menampilkan jumlah hari kerja dari API, alur persetujuan, dan kartu saldo (mockup H-08)', async () => {
    const w = await mountPage()
    expect(api.preview).toHaveBeenCalled()
    expect(w.get('[data-testid="days"]').text()).toBe('5')
    expect(w.get('[data-testid="flow"]').text()).toContain('Bayu Pratama')
    expect(w.get('[data-testid="flow"]').text()).toContain('Sinta Maharani')
    const ct = w.get('[data-testid="bal-CT"]').text()
    expect(ct).toContain('8')
    expect(ct).toContain('/ 12 hari')
    expect(ct).toContain('2 terpakai')
    expect(ct).toContain('2 menunggu')
    // pengajuan ini 5 hari → sisa 3 (persis mockup)
    expect(ct).toContain('pengajuan ini 5 → sisa 3')
    expect(w.get('[data-testid="bal-IZ"]').text()).toContain('4,5')
    expect(w.text()).toContain('tidak memotong saldo') // cuti sakit hanya dicatat
  })

  it('menghitung ulang secara live saat tanggal diubah (dengan jeda) dan memakai tanggal terbaru', async () => {
    const w = await mountPage()
    api.preview.mockClear()
    api.preview.mockResolvedValue(
      makePreview({ working_days: workingDays('3', ['2026-11-25', '2026-11-26', '2026-11-27']) }),
    )
    await w.get('[data-testid="start"]').setValue('2026-11-25')
    await w.get('[data-testid="end"]').setValue('2026-11-27')
    expect(api.preview).not.toHaveBeenCalled() // belum lewat jeda 250 ms
    await tick()
    expect(api.preview).toHaveBeenCalledTimes(1)
    expect(api.preview.mock.calls[0]![0]).toMatchObject({
      type_id: 1,
      start_date: '2026-11-25',
      end_date: '2026-11-27',
      half_day: false,
    })
    expect(w.get('[data-testid="days"]').text()).toBe('3')
  })

  it('tanggal selesai sebelum mulai: pesan kesalahan, tanpa memanggil API', async () => {
    const w = await mountPage()
    await w.get('[data-testid="start"]').setValue('2026-11-25')
    await tick()
    api.preview.mockClear()
    // pilih tanggal selesai lebih awal dari mulai
    await w.get('[data-testid="end"]').setValue('2026-11-20')
    await tick()
    expect(api.preview).not.toHaveBeenCalled()
    expect(w.get('[data-testid="calc"]').text()).toContain('tidak boleh sebelum')
  })

  it('izin sebagian hari: input jam muncul, diisi paruh pagi dari jam kerja API, tanggal selesai dikunci', async () => {
    const w = await mountPage()
    expect(w.find('[data-testid="mode-partial"]').exists()).toBe(false) // cuti tahunan tidak boleh sebagian hari
    await w.get('[data-testid="type-IZ"]').trigger('click')
    await w.get('[data-testid="mode-partial"]').trigger('click')
    await tick()
    // paruh pagi dari API (jam efektif, istirahat 12–13 tidak dihitung)
    expect(w.get<HTMLInputElement>('[data-testid="from"]').element.value).toBe('08:00')
    expect(w.get<HTMLInputElement>('[data-testid="to"]').element.value).toBe('12:00')
    expect(w.get('[data-testid="end"]').attributes('disabled')).toBeDefined()
    expect(w.get('[data-testid="partial-box"]').text()).toContain('Jam kerja hari itu 08:00–17:00')
    expect(w.get('[data-testid="partial-box"]').text()).toContain('istirahat 12:00–13:00')
    await tick()
    expect(api.preview.mock.calls.at(-1)![0]).toMatchObject({
      type_id: 3,
      half_day: true,
      start_time: '08:00',
      end_time: '12:00',
    })

    await w.get('[data-testid="quick-siang"]').trigger('click')
    expect((w.get('[data-testid="from"]').element as HTMLInputElement).value).toBe('13:00')
    expect((w.get('[data-testid="to"]').element as HTMLInputElement).value).toBe('17:00')
  })

  it('izin 2 jam: jam dikirim ke API dan ringkasan jam dari hari kerja ditampilkan', async () => {
    api.preview.mockResolvedValue(
      makePreview({
        working_days: {
          ...workingDays('0.25', ['2026-11-23']),
          half_day: true,
          partial_minutes: 120,
          day_minutes: 480,
        },
      }),
    )
    const w = await mountPage()
    await w.get('[data-testid="type-IZ"]').trigger('click')
    await w.get('[data-testid="mode-partial"]').trigger('click')
    await tick()
    await w.get('[data-testid="from"]').setValue('08:00')
    await w.get('[data-testid="to"]').setValue('10:00')
    await tick()
    expect(api.preview.mock.calls.at(-1)![0]).toMatchObject({ start_time: '08:00', end_time: '10:00' })
    expect(w.get('[data-testid="days"]').text()).toBe('0,25')
    expect(w.get('[data-testid="calc"]').text()).toContain('2 jam dari 8 jam kerja efektif hari itu')
    expect(w.get('[data-testid="hours"]').text()).toContain('08:00–17:00')

    await w.get('[data-testid="reason"]').setValue('Kontrol ke dokter')
    await w.get('form').trigger('submit')
    await flushPromises()
    expect(api.submit.mock.calls[0]![0]).toMatchObject({
      leave_type_id: 3,
      half_day: true,
      start_time: '08:00',
      end_time: '10:00',
      reason: 'Kontrol ke dokter',
    })
  })

  it('jam selesai tidak setelah jam mulai ditolak di sisi klien', async () => {
    const w = await mountPage()
    await w.get('[data-testid="type-IZ"]').trigger('click')
    await w.get('[data-testid="mode-partial"]').trigger('click')
    await tick()
    await w.get('[data-testid="from"]').setValue('10:00')
    await w.get('[data-testid="to"]').setValue('09:00')
    await w.get('[data-testid="reason"]').setValue('x')
    await w.get('form').trigger('submit')
    await flushPromises()
    expect(w.text()).toContain('Jam selesai harus setelah jam mulai')
    expect(api.submit).not.toHaveBeenCalled()
  })

  it('jadwal dengan Sabtu: ringkasan jam kerja menyebut Sabtu 09:00–15:00', async () => {
    api.preview.mockResolvedValue(
      makePreview({
        working_days: {
          ...workingDays('2', ['2026-11-27', '2026-11-28']),
          schedule: 'Shift Sabtu',
          breakdown: [
            { date: '2026-11-27', weekday: 'Jum', counted: true, hours: '09:00–17:00', minutes: 480 },
            { date: '2026-11-28', weekday: 'Sab', counted: true, hours: '09:00–15:00', minutes: 360 },
          ],
        },
      }),
    )
    const w = await mountPage()
    const t = w.get('[data-testid="hours"]').text()
    expect(t).toContain('Shift Sabtu')
    expect(t).toContain('Jum 09:00–17:00')
    expect(t).toContain('Sab 09:00–15:00')
    expect(w.get('[data-testid="days"]').text()).toBe('2')
  })

  it('alasan kosong ditolak di sisi klien', async () => {
    const w = await mountPage()
    await w.get('form').trigger('submit')
    await flushPromises()
    expect(w.text()).toContain('Alasan wajib diisi')
    expect(api.submit).not.toHaveBeenCalled()
  })

  it('cuti sakit lebih dari 1 hari wajib lampiran; 1 hari tidak', async () => {
    const w = await mountPage()
    await w.get('[data-testid="type-CS"]').trigger('click')
    await w.get('[data-testid="reason"]').setValue('Demam tinggi')
    await tick() // pratinjau: 5 hari kerja
    await w.get('form').trigger('submit')
    await flushPromises()
    expect(w.text()).toContain('Lampiran wajib diunggah untuk Cuti sakit')
    expect(api.submit).not.toHaveBeenCalled()

    api.preview.mockResolvedValue(makePreview({ working_days: workingDays('1', ['2026-11-23']) }))
    await w
      .get('[data-testid="end"]')
      .setValue(w.get<HTMLInputElement>('[data-testid="start"]').element.value)
    await w.get('[data-testid="start"]').setValue('2026-11-23')
    await tick()
    expect(w.get('[data-testid="days"]').text()).toBe('1')
    await w.get('form').trigger('submit')
    await flushPromises()
    expect(api.submit).toHaveBeenCalledTimes(1)
  })

  it('mengirim pengajuan lalu membuka riwayat dengan pengajuan baru terpilih', async () => {
    const w = await mountPage()
    await w.get('[data-testid="reason"]').setValue('Acara keluarga di Yogyakarta.')
    await w.get('form').trigger('submit')
    await flushPromises()
    expect(api.submit).toHaveBeenCalledTimes(1)
    expect(api.submit.mock.calls[0]![0]).toMatchObject({
      leave_type_id: 1,
      half_day: false,
      reason: 'Acara keluarga di Yogyakarta.',
      substitute_id: null,
    })
    expect(router.push).toHaveBeenCalledWith({ path: '/izin-cuti/riwayat', query: { id: '77' } })
  })

  it('menampilkan kesalahan dari server (saldo kurang) pada kolom terkait', async () => {
    api.submit.mockRejectedValue(
      new ApiError(422, 'validation_error', 'Saldo Cuti tahunan tidak cukup', {
        end_date: 'Saldo Cuti tahunan tidak cukup: tersisa 3 hari, diminta 5 hari',
      }),
    )
    const w = await mountPage()
    await w.get('[data-testid="reason"]').setValue('Liburan')
    await w.get('form').trigger('submit')
    await flushPromises()
    expect(w.text()).toContain('tersisa 3 hari, diminta 5 hari')
    expect(router.push).not.toHaveBeenCalled()
  })

  it('peringatan dari pratinjau (tumpang tindih) tampil sebelum dikirim', async () => {
    api.preview.mockResolvedValue(
      makePreview({ warnings: ['Tanggal tumpang tindih dengan pengajuan CT-2026-0142'] }),
    )
    const w = await mountPage()
    expect(w.get('[data-testid="calc"]').text()).toContain('tumpang tindih')
  })
})
