import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import WorkSchedulesTab from './WorkSchedulesTab.vue'
import { ApiError, type WorkSchedule } from '@/api'

const api = vi.hoisted(() => ({ list: vi.fn(), save: vi.fn(), remove: vi.fn() }))

vi.mock('@/api', async (orig) => {
  const real = await orig<typeof import('@/api')>()
  return { ...real, schedules: api }
})

const wk = (start: string, end: string) =>
  [1, 2, 3, 4, 5].map((weekday) => ({ weekday, start_time: start, end_time: end }))
const A: WorkSchedule = { id: 1, name: 'Jam kantor', is_default: true, days: wk('08:00', '17:00') }
const B: WorkSchedule = {
  id: 2,
  name: 'Shift Sabtu',
  is_default: false,
  days: [...wk('09:00', '17:00'), { weekday: 6, start_time: '09:00', end_time: '15:00' }],
}

const doc = () => document.body
const click = async (sel: string) => {
  doc().querySelector<HTMLElement>(sel)!.click()
  await flushPromises()
}
const fill = async (sel: string, v: string) => {
  const el = doc().querySelector<HTMLInputElement>(sel)!
  el.value = v
  el.dispatchEvent(new Event('input'))
  await flushPromises()
}

async function mountTab() {
  const w = mount(WorkSchedulesTab, { attachTo: document.body, global: { stubs: { Icon: true } } })
  await flushPromises()
  return w
}

describe('Pengaturan → Jam kerja (H-16)', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    document.body.innerHTML = ''
    api.list.mockResolvedValue([A, B])
    api.save.mockResolvedValue(B)
    api.remove.mockResolvedValue(undefined)
  })

  it('menampilkan ringkasan hari & jam kerja tiap jadwal, default diberi label', async () => {
    const w = await mountTab()
    const rows = w.findAll('[data-testid="summary"]').map((r) => r.text())
    expect(rows[0]).toBe('Sen–Jum 08:00–17:00')
    expect(rows[1]).toBe('Sen–Jum 09:00–17:00 · Sab 09:00–15:00')
    expect(w.text()).toContain('default')
    // jadwal default tidak bisa dihapus dari antarmuka
    expect(w.get('button[aria-label="Hapus Jam kantor"]').attributes('disabled')).toBeDefined()
    w.unmount()
  })

  it('jadwal baru: Senin–Jumat aktif secara bawaan; menambah Sabtu 09–15 dikirim sebagai weekday 6', async () => {
    const w = await mountTab()
    await w.get('[data-testid="add-schedule"]').trigger('click')
    await flushPromises()
    await fill('[data-testid="s-name"]', 'Shift CS')
    await click('[data-testid="on-6"]')
    await fill('[data-testid="start-6"]', '09:00')
    await fill('[data-testid="end-6"]', '15:00')
    await click('[data-testid="s-save"]')
    expect(api.save).toHaveBeenCalledTimes(1)
    const [id, body] = api.save.mock.calls[0]!
    expect(id).toBeNull()
    expect(body.name).toBe('Shift CS')
    expect(body.is_default).toBe(false)
    expect(body.days.map((d: { weekday: number }) => d.weekday).sort()).toEqual([1, 2, 3, 4, 5, 6])
    expect(body.days.find((d: { weekday: number }) => d.weekday === 6)).toEqual({
      weekday: 6,
      start_time: '09:00',
      end_time: '15:00',
    })
    expect(w.emitted('changed')).toBeTruthy()
    w.unmount()
  })

  it('tanpa hari kerja atau jam terbalik ditolak di klien', async () => {
    const w = await mountTab()
    await w.get('[data-testid="add-schedule"]').trigger('click')
    await flushPromises()
    await fill('[data-testid="s-name"]', 'X')
    for (const wd of [1, 2, 3, 4, 5]) await click(`[data-testid="on-${wd}"]`)
    await click('[data-testid="s-save"]')
    expect(doc().textContent).toContain('Pilih minimal satu hari kerja')

    await click('[data-testid="on-1"]')
    await fill('[data-testid="start-1"]', '17:00')
    await fill('[data-testid="end-1"]', '08:00')
    await click('[data-testid="s-save"]')
    expect(doc().textContent).toContain('Jam selesai Senin harus setelah jam mulai')
    expect(api.save).not.toHaveBeenCalled()
    w.unmount()
  })

  it('mengubah jadwal memuat harinya; kesalahan server ditampilkan', async () => {
    api.save.mockRejectedValue(
      new ApiError(422, 'validation_error', 'Jadwal kerja tidak valid', {
        name: 'Nama jadwal sudah dipakai',
      }),
    )
    const w = await mountTab()
    await w.get('button[aria-label="Ubah Shift Sabtu"]').trigger('click')
    await flushPromises()
    expect(doc().querySelector<HTMLInputElement>('[data-testid="on-6"]')!.checked).toBe(true)
    expect(doc().querySelector<HTMLInputElement>('[data-testid="end-6"]')!.value).toBe('15:00')
    expect(doc().querySelector<HTMLInputElement>('[data-testid="on-0"]')!.checked).toBe(false)
    await click('[data-testid="s-save"]')
    expect(doc().textContent).toContain('Nama jadwal sudah dipakai')
    expect(api.save.mock.calls[0]![0]).toBe(2)
    w.unmount()
  })

  it('menghapus jadwal lewat konfirmasi', async () => {
    const w = await mountTab()
    await w.get('button[aria-label="Hapus Shift Sabtu"]').trigger('click')
    await flushPromises()
    await click('[data-testid="s-confirm-remove"]')
    expect(api.remove).toHaveBeenCalledWith(2)
    w.unmount()
  })
})
