import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import LeaveTypesTab from './LeaveTypesTab.vue'
import HolidaysTab from './HolidaysTab.vue'
import NotificationsTab from './NotificationsTab.vue'
import { ApiError } from '@/api'
import { CS, CT, IZ, types } from '@/test/leaveFixtures'

const api = vi.hoisted(() => ({
  types: vi.fn(),
  saveType: vi.fn(),
  deleteType: vi.fn(),
  holidays: vi.fn(),
  saveHoliday: vi.fn(),
  deleteHoliday: vi.fn(),
  importHolidays: vi.fn(),
  notifySettings: vi.fn(),
  setNotifySetting: vi.fn(),
  policy: vi.fn(),
  setPolicy: vi.fn(),
}))

vi.mock('@/api', async (orig) => {
  const real = await orig<typeof import('@/api')>()
  return { ...real, leave: { ...real.leave, ...api } }
})

const body = () => document.body
const click = async (sel: string) => {
  body().querySelector<HTMLElement>(sel)!.click()
  await flushPromises()
}
const fill = async (sel: string, v: string) => {
  const el = body().querySelector<HTMLInputElement>(sel)!
  el.value = v
  el.dispatchEvent(new Event('input'))
  await flushPromises()
}

beforeEach(() => {
  setActivePinia(createPinia())
  vi.clearAllMocks()
  document.body.innerHTML = ''
  api.types.mockResolvedValue(types)
  api.saveType.mockResolvedValue(CT)
  api.policy.mockResolvedValue({ carry_over: false })
  api.setPolicy.mockResolvedValue({ carry_over: true })
  api.holidays.mockResolvedValue([
    { id: 1, date: '2026-12-25T00:00:00Z', name: 'Hari Raya Natal', type: 'nasional' },
    { id: 2, date: '2026-12-24T00:00:00Z', name: 'Cuti Bersama Natal', type: 'cuti_bersama' },
  ])
  api.saveHoliday.mockResolvedValue({})
  api.importHolidays.mockResolvedValue({ added: 25 })
  api.notifySettings.mockResolvedValue([
    {
      key: 'notify.leave_submitted',
      label: 'Pengajuan izin/cuti baru',
      hint: 'ke penyetuju berikutnya',
      enabled: true,
    },
    {
      key: 'notify.leave_result',
      label: 'Hasil persetujuan',
      hint: 'ke karyawan yang mengajukan',
      enabled: false,
    },
  ])
})

describe('Pengaturan → Jenis cuti (H-16)', () => {
  async function mountTab() {
    const w = mount(LeaveTypesTab, { attachTo: document.body, global: { stubs: { Icon: true } } })
    await flushPromises()
    return w
  }

  it('saklar bawa sisa saldo: bawaan nonaktif, klik menyimpan ke API', async () => {
    const w = await mountTab()
    const sw = w.get('[data-testid="sw-carry-over"]')
    expect(sw.attributes('aria-checked')).toBe('false')
    api.policy.mockResolvedValue({ carry_over: true })
    await sw.trigger('click')
    await flushPromises()
    expect(api.setPolicy).toHaveBeenCalledWith(true)
    expect(w.get('[data-testid="sw-carry-over"]').attributes('aria-checked')).toBe('true')
  })

  it('menampilkan kuota, potong saldo, lampiran, setengah hari, dan alur persetujuan', async () => {
    const w = await mountTab()
    const t = w.get('[data-testid="leave-types"]').text()
    expect(t).toContain('12 hari · masa kerja ≥ 12 bln')
    expect(t).toContain('Ya, jika > 1 hari')
    expect(t).toContain('Atasan (½ hari) · Atasan → HR')
    expect(t).toContain('Tidak dibatasi')
    w.unmount()
  })

  it('potong saldo tanpa kuota ditolak di klien', async () => {
    const w = await mountTab()
    await w.get('[data-testid="add-type"]').trigger('click')
    await flushPromises()
    await fill('[data-testid="t-name"]', 'Cuti besar')
    await fill('[data-testid="t-code"]', 'cb')
    await click('[data-testid="t-deduct"]')
    await click('[data-testid="t-save"]')
    expect(body().textContent).toContain('Isi kuota per tahun bila saldo dipotong')
    expect(api.saveType).not.toHaveBeenCalled()
    w.unmount()
  })

  it('menyimpan jenis baru dengan kode kapital dan kuota angka', async () => {
    const w = await mountTab()
    await w.get('[data-testid="add-type"]').trigger('click')
    await flushPromises()
    await fill('[data-testid="t-name"]', 'Cuti besar')
    await fill('[data-testid="t-code"]', 'cb')
    await fill('[data-testid="t-quota"]', '10')
    await click('[data-testid="t-deduct"]')
    await click('[data-testid="t-save"]')
    expect(api.saveType).toHaveBeenCalledTimes(1)
    expect(api.saveType.mock.calls[0]![0]).toBeNull()
    expect(api.saveType.mock.calls[0]![1]).toMatchObject({
      name: 'Cuti besar',
      code: 'CB',
      annual_quota: 10,
      deducts_balance: true,
    })
    w.unmount()
  })

  it('kesalahan server (kode kembar) ditampilkan pada kolomnya', async () => {
    api.saveType.mockRejectedValue(
      new ApiError(422, 'validation_error', 'Data tidak valid', { code: 'Kode sudah dipakai' }),
    )
    const w = await mountTab()
    await w.get('[data-testid="add-type"]').trigger('click')
    await flushPromises()
    await fill('[data-testid="t-name"]', 'X')
    await fill('[data-testid="t-code"]', 'CT')
    await click('[data-testid="t-save"]')
    expect(body().textContent).toContain('Kode sudah dipakai')
    w.unmount()
  })

  it('menghapus jenis lewat konfirmasi', async () => {
    api.deleteType.mockResolvedValue(undefined)
    const w = await mountTab()
    await w.get(`button[aria-label="Hapus ${CS.name}"]`).trigger('click')
    await flushPromises()
    await click('[data-testid="t-confirm-remove"]')
    expect(api.deleteType).toHaveBeenCalledWith(CS.id)
    w.unmount()
  })

  it('jenis nonaktif diberi label', async () => {
    api.types.mockResolvedValue([{ ...IZ, is_active: false }])
    const w = await mountTab()
    expect(w.text()).toContain('nonaktif')
    w.unmount()
  })
})

describe('Pengaturan → Hari libur (H-16)', () => {
  async function mountTab() {
    const w = mount(HolidaysTab, { attachTo: document.body, global: { stubs: { Icon: true } } })
    await flushPromises()
    return w
  }

  it('daftar libur menampilkan tanggal, nama, hari, dan jenisnya', async () => {
    const w = await mountTab()
    const natal = w.get('[data-testid="holiday-1"]').text()
    expect(natal).toContain('25')
    expect(natal).toContain('Des')
    expect(natal).toContain('Hari Raya Natal')
    expect(natal).toContain('Jumat')
    expect(natal).toContain('Nasional')
    expect(w.get('[data-testid="holiday-2"]').text()).toContain('Cuti bersama')
    w.unmount()
  })

  it('tambah hari libur mengirim tanggal YYYY-MM-DD', async () => {
    const w = await mountTab()
    await w.get('[data-testid="add-holiday"]').trigger('click')
    await flushPromises()
    await fill('[data-testid="h-date"]', '2026-12-31')
    await fill('[data-testid="h-name"]', 'Libur perusahaan')
    await click('[data-testid="h-save"]')
    expect(api.saveHoliday).toHaveBeenCalledWith(null, {
      date: '2026-12-31',
      name: 'Libur perusahaan',
      type: 'nasional',
    })
    w.unmount()
  })

  it('nama kosong ditolak; impor daftar resmi memanggil API untuk tahun terpilih', async () => {
    const w = await mountTab()
    await w.get('[data-testid="add-holiday"]').trigger('click')
    await flushPromises()
    await click('[data-testid="h-save"]')
    expect(body().textContent).toContain('Nama wajib diisi')
    expect(api.saveHoliday).not.toHaveBeenCalled()
    await click('[data-testid="h-save"]') // tetap terbuka
    document.body.querySelector<HTMLElement>('button[aria-label="Tutup"]')!.click()
    await flushPromises()

    await w.get('[data-testid="import"]').trigger('click')
    await flushPromises()
    expect(api.importHolidays).toHaveBeenCalledWith(new Date().getFullYear())
    w.unmount()
  })
})

describe('Pengaturan → Notifikasi (H-16)', () => {
  it('menampilkan saklar dan menyimpan perubahan; WhatsApp terkunci (rilis 3)', async () => {
    api.setNotifySetting.mockResolvedValue([
      { key: 'notify.leave_submitted', label: 'Pengajuan izin/cuti baru', hint: 'x', enabled: false },
      { key: 'notify.leave_result', label: 'Hasil persetujuan', hint: 'y', enabled: false },
    ])
    const w = mount(NotificationsTab, { global: { stubs: { Icon: true } } })
    await flushPromises()
    const sw = w.get('[data-testid="sw-notify.leave_submitted"]')
    expect(sw.attributes('aria-checked')).toBe('true')
    expect(w.get('[data-testid="sw-notify.leave_result"]').attributes('aria-checked')).toBe('false')

    await sw.trigger('click')
    await flushPromises()
    expect(api.setNotifySetting).toHaveBeenCalledWith('notify.leave_submitted', false)
    expect(w.get('[data-testid="sw-notify.leave_submitted"]').attributes('aria-checked')).toBe('false')

    const wa = w.findAll('button[role="switch"]').at(-1)!
    expect(wa.attributes('disabled')).toBeDefined()
    expect(w.text()).toContain('tersedia di rilis 3')
  })
})
