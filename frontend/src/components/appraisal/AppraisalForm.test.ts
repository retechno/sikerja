import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import AppraisalForm from './AppraisalForm.vue'
import { aspects, makeView } from '@/test/fixtures'

const scale = (w: ReturnType<typeof mount>, aspectId: number, n: number) =>
  w.get(`[data-testid="aspect-${aspectId}"]`).findAll('button[role="radio"]')[n - 1]!

describe('AppraisalForm (H-07) — mode atasan', () => {
  it('menampilkan nilai karyawan sebagai pembanding dan catatan pencapaian', () => {
    const w = mount(AppraisalForm, { props: { view: makeView() } })
    const pills = w.findAll('[data-testid="self-pill"]').map((p) => p.text())
    expect(pills).toEqual(['Karyawan: 5', 'Karyawan: 4', 'Karyawan: 4', 'Karyawan: 4', 'Karyawan: 5'])
    expect(w.get('[data-testid="employee-comment"]').text()).toContain('Menangani migrasi data PMS')
    expect(w.findAll('[data-testid^="aspect-"]')).toHaveLength(5)
  })

  it('skor akhir dihitung langsung saat semua aspek dinilai: 93,4 Istimewa (contoh SPEC)', async () => {
    const w = mount(AppraisalForm, { props: { view: makeView() } })
    expect(w.get('[data-testid="final-score"]').text()).toBe('—')
    expect(w.get('[data-testid="submit"]').attributes('disabled')).toBeDefined()

    for (const [id, n] of [[1, 4], [2, 5], [3, 4], [4, 4], [5, 5]] as const) {
      await scale(w, id, n).trigger('click')
    }
    expect(w.get('[data-testid="behavior-part"]').text()).toBe('26,4') // 88,0 × 30%
    expect(w.get('[data-testid="kpi-part"]').text()).toBe('67,0') // 95,7 × 70%
    expect(w.get('[data-testid="average"]').text()).toBe('rata-rata nilai atasan 4,4 × 20') // sama dengan mockup H-07
    expect(w.get('[data-testid="final-score"]').text()).toBe('93,4')
    expect(w.get('[data-testid="grade"]').text()).toBe('Istimewa')
    expect(w.get('[data-testid="submit"]').attributes('disabled')).toBeUndefined()
  })

  it('predikat berubah mengikuti nilai (semua 3 → perilaku 60 → skor 85,0 Baik)', async () => {
    const w = mount(AppraisalForm, { props: { view: makeView() } })
    for (const a of aspects) await scale(w, a.id, 3).trigger('click')
    expect(w.get('[data-testid="final-score"]').text()).toBe('85,0') // 0,7×95,65 + 0,3×60 = 84,96
    expect(w.get('[data-testid="grade"]').text()).toBe('Baik')
  })

  it('tombol skala menandai pilihan (aria-checked) dan hanya satu per aspek', async () => {
    const w = mount(AppraisalForm, { props: { view: makeView() } })
    await scale(w, 1, 4).trigger('click')
    await scale(w, 1, 2).trigger('click')
    const checked = w.get('[data-testid="aspect-1"]').findAll('button[aria-checked="true"]')
    expect(checked).toHaveLength(1)
    expect(checked[0]!.text()).toBe('2')
  })

  it('kirim: emit payload lengkap dengan submit=true; draf: submit=false', async () => {
    const w = mount(AppraisalForm, { props: { view: makeView() } })
    for (const [id, n] of [[1, 4], [2, 5], [3, 4], [4, 4], [5, 5]] as const) await scale(w, id, n).trigger('click')
    await w.get('[data-testid="manager-comment"]').setValue('Respons cepat')
    await w.get('[data-testid="development-plan"]').setValue('Sertifikasi DBA')

    await w.get('[data-testid="submit"]').trigger('click')
    await w.get('[data-testid="draft"]').trigger('click')
    const ev = w.emitted('save')!
    expect(ev).toHaveLength(2)
    const [payload, submit] = ev[0] as [{ scores: Record<number, number>; managerComment: string; developmentPlan: string }, boolean]
    expect(payload.scores).toEqual({ 1: 4, 2: 5, 3: 4, 4: 4, 5: 5 })
    expect(payload.managerComment).toBe('Respons cepat')
    expect(payload.developmentPlan).toBe('Sertifikasi DBA')
    expect(submit).toBe(true)
    expect(ev[1]![1]).toBe(false)
  })

  it('draf boleh sebagian; petunjuk jumlah aspek terisi', async () => {
    const w = mount(AppraisalForm, { props: { view: makeView() } })
    await scale(w, 1, 4).trigger('click')
    expect(w.get('[data-testid="incomplete"]').text()).toContain('1/5')
    await w.get('[data-testid="draft"]').trigger('click')
    expect((w.emitted('save')![0]![0] as { scores: object }).scores).toEqual({ 1: 4 })
  })

  it('membawa nilai atasan yang sudah tersimpan (mengedit ulang)', () => {
    const w = mount(AppraisalForm, { props: { view: makeView({ status: 'manager_done' }, [4, 5, 4, 4, 5]) } })
    expect(w.get('[data-testid="final-score"]').text()).toBe('93,4')
    expect(w.get('[data-testid="aspect-2"]').get('button[aria-checked="true"]').text()).toBe('5')
  })

  it('tombol finalisasi hanya muncul bila can_finalize', async () => {
    const without = mount(AppraisalForm, { props: { view: makeView() } })
    expect(without.find('[data-testid="finalize"]').exists()).toBe(false)
    const w = mount(AppraisalForm, { props: { view: makeView({ can_finalize: true, status: 'manager_done' }, [4, 5, 4, 4, 5]) } })
    await w.get('[data-testid="finalize"]').trigger('click')
    expect(w.emitted('finalize')).toHaveLength(1)
  })
})

describe('AppraisalForm — mode karyawan (self-assessment)', () => {
  const selfView = () =>
    makeView({ can_manager: false, can_self: true, status: 'draft', hidden_from_employee: true, kpi_score: '95.65' }, [null, null, null, null, null])

  it('mengisi nilai sendiri; nilai atasan & skor akhir disembunyikan', () => {
    const w = mount(AppraisalForm, { props: { view: selfView() } })
    expect(w.find('[data-testid="self-pill"]').exists()).toBe(false)
    expect(w.find('[data-testid="manager-comment"]').exists()).toBe(false)
    expect(w.get('[data-testid="hidden-note"]').text()).toContain('setelah penilaian difinalisasi')
    expect(w.get('[data-testid="final-score"]').text()).toBe('—')
    expect(w.find('[data-testid="grade"]').exists()).toBe(false)
    expect(w.get('[data-testid="self-comment"]').element).toBeTruthy()
  })

  it('kirim self-assessment membawa nilai sendiri dan catatan', async () => {
    const view = makeView({ can_manager: false, can_self: true, status: 'draft', hidden_from_employee: true })
    for (const a of view.aspects!) a.self_score = null
    const w = mount(AppraisalForm, { props: { view } })
    for (const a of aspects) await scale(w, a.id, 4).trigger('click')
    await w.get('[data-testid="self-comment"]').setValue('Target tercapai')
    await w.get('[data-testid="submit"]').trigger('click')
    const [payload, submit] = w.emitted('save')![0] as [{ scores: Record<number, number>; comment: string }, boolean]
    expect(payload.scores).toEqual({ 1: 4, 2: 4, 3: 4, 4: 4, 5: 4 })
    expect(payload.comment).toBe('Target tercapai')
    expect(submit).toBe(true)
    expect(w.get('[data-testid="submit"]').text()).toBe('Kirim self-assessment')
  })
})

describe('AppraisalForm — hanya baca', () => {
  it('periode terkunci/final: skala nonaktif dan tanpa tombol kirim', () => {
    const w = mount(AppraisalForm, {
      props: {
        view: makeView(
          { can_manager: false, can_self: false, status: 'final', final_score: '93.36', behavior_score: '88', grade: 'Istimewa', manager_comment: 'Bagus', development_plan: 'Sertifikasi' },
          [4, 5, 4, 4, 5],
        ),
      },
    })
    expect(w.findAll('button[role="radio"]').every((b) => b.attributes('disabled') !== undefined)).toBe(true)
    expect(w.find('[data-testid="submit"]').exists()).toBe(false)
    expect(w.get('[data-testid="final-score"]').text()).toBe('93,4')
    expect(w.text()).toContain('Bagus')
    expect(w.text()).toContain('Sertifikasi')
  })
})
