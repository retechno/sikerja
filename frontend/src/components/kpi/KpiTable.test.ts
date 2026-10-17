import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import KpiTable from './KpiTable.vue'
import { makeRows } from '@/test/fixtures'

const base = { rows: makeRows(), score: '95.65', canInput: true }

describe('KpiTable (H-05)', () => {
  it('menampilkan target, realisasi, capaian, bobot, dan nilai dengan format Indonesia', () => {
    const w = mount(KpiTable, { props: base })
    const row1 = w.get('[data-testid="kpi-row-1"]')
    expect(row1.text()).toContain('Tiket terselesaikan')
    expect(row1.text()).toContain('lebih tinggi lebih baik')
    expect(row1.text()).toContain('120 tiket') // target + satuan
    expect(row1.get('[data-col="actual"]').text()).toBe('104')
    expect(row1.get('[data-col="achievement"]').text()).toBe('86,7') // 86,67 → 1 desimal koma
    expect(row1.text()).toContain('25%')
    expect(row1.get('[data-col="points"]').text()).toBe('21,7')

    const sla = w.get('[data-testid="kpi-row-2"]')
    expect(sla.text()).toContain('95%') // satuan persen menempel
    expect(sla.get('[data-col="actual"]').text()).toBe('96,2%')
    expect(sla.get('[data-col="achievement"]').text()).toBe('101,3')

    const resp = w.get('[data-testid="kpi-row-3"]')
    expect(resp.text()).toContain('lebih rendah lebih baik')
    expect(resp.get('[data-col="achievement"]').text()).toBe('120,0') // dibatasi 120

    const rating = w.get('[data-testid="kpi-row-5"]')
    expect(rating.text()).toContain('4,5') // satuan "dari 5" tidak ditulis
    expect(rating.get('[data-col="achievement"]').text()).toBe('104,4')
  })

  it('footer: total bobot 100% dan skor KPI 95,7 (mockup H-05)', () => {
    const w = mount(KpiTable, { props: base })
    expect(w.get('[data-testid="kpi-total-weight"]').text()).toBe('100%')
    expect(w.get('[data-testid="kpi-total-score"]').text()).toBe('95,7')
  })

  it('status per baris: tervalidasi vs menunggu validasi', () => {
    const w = mount(KpiTable, { props: base })
    expect(w.get('[data-testid="kpi-row-1"] [data-col="status"]').text()).toBe('Tervalidasi')
    expect(w.get('[data-testid="kpi-row-4"] [data-col="status"]').text()).toBe('Menunggu validasi')
    expect(w.findAll('[data-col="status"]').filter((s) => s.text() === 'Tervalidasi')).toHaveLength(4)
  })

  it('tombol aksi: "Ubah" (utama) untuk yang menunggu validasi, "Ubah" biasa untuk tervalidasi', () => {
    const w = mount(KpiTable, { props: base })
    const pending = w.get('[data-testid="kpi-action-4"]')
    expect(pending.text()).toBe('Ubah')
    expect(pending.classes()).toContain('pri')
    const done = w.get('[data-testid="kpi-action-1"]')
    expect(done.text()).toBe('Ubah')
    expect(done.classes()).not.toContain('pri')
  })

  it('periode terkunci (canInput=false) → semua tombol "Lihat"', () => {
    const w = mount(KpiTable, { props: { ...base, canInput: false } })
    const labels = w.findAll('button').map((b) => b.text())
    expect(labels).toHaveLength(5)
    expect(new Set(labels)).toEqual(new Set(['Lihat']))
  })

  it('indikator dari integrasi tidak bisa diisi manual', () => {
    const rows = makeRows()
    rows[0]!.indicator.source = 'integration'
    const w = mount(KpiTable, { props: { ...base, rows } })
    expect(w.get('[data-testid="kpi-action-1"]').text()).toBe('Lihat')
  })

  it('belum diisi: tampil "—" dan tombol "Isi"; ditolak: alasan terlihat', () => {
    const w = mount(KpiTable, { props: { ...base, rows: makeRows({ 2: 'missing', 5: 'rejected' }) } })
    const miss = w.get('[data-testid="kpi-row-2"]')
    expect(miss.get('[data-col="actual"]').text()).toBe('—')
    expect(miss.get('[data-col="achievement"]').text()).toBe('—')
    expect(miss.get('[data-col="status"]').text()).toBe('Belum diisi')
    expect(w.get('[data-testid="kpi-action-2"]').text()).toBe('Isi')
    const rej = w.get('[data-testid="kpi-row-5"]')
    expect(rej.get('[data-col="status"]').text()).toBe('Ditolak')
    expect(rej.text()).toContain('Bukti kurang lengkap')
  })

  it('menyorot baris terpilih dan mengirim event select', async () => {
    const w = mount(KpiTable, { props: { ...base, selectedId: 4 } })
    expect(w.get('[data-testid="kpi-row-4"]').classes()).toContain('sel')
    expect(w.get('[data-testid="kpi-row-1"]').classes()).not.toContain('sel')
    await w.get('[data-testid="kpi-action-4"]').trigger('click')
    const ev = w.emitted('select')
    expect(ev).toHaveLength(1)
    expect((ev![0]![0] as { indicator: { id: number } }).indicator.id).toBe(4)
  })
})
