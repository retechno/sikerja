import { describe, expect, it } from 'vitest'
import { fmtDate, fmtNum, fmtScore, gradeClass, initials, toDateInput } from './format'

describe('format', () => {
  it('skor 1 desimal dengan koma', () => {
    expect(fmtScore('95.65')).toBe('95,7')
    expect(fmtScore(88)).toBe('88,0')
    expect(fmtScore('93.39')).toBe('93,4')
    expect(fmtScore(null)).toBe('—')
    expect(fmtScore('abc')).toBe('—')
  })

  it('angka bebas dengan koma dan tanpa nol di belakang', () => {
    expect(fmtNum('4.5')).toBe('4,5')
    expect(fmtNum('120.00')).toBe('120')
    expect(fmtNum(1234.5)).toBe('1.234,5')
    expect(fmtNum(undefined)).toBe('—')
  })

  it('tanggal 16 Okt 2026 tanpa pergeseran zona waktu', () => {
    expect(fmtDate('2026-10-16T00:00:00Z')).toBe('16 Okt 2026')
    expect(fmtDate('2026-01-05')).toBe('5 Jan 2026')
    expect(fmtDate('2026-12-31T23:59:59-08:00')).toBe('31 Des 2026')
    expect(fmtDate(null)).toBe('—')
    expect(fmtDate('bukan tanggal')).toBe('—')
  })

  it('nilai input tanggal', () => {
    expect(toDateInput('2021-05-03T00:00:00Z')).toBe('2021-05-03')
  })

  it('inisial dan kelas predikat', () => {
    expect(initials('Rina Wulandari')).toBe('RW')
    expect(initials('Budi')).toBe('B')
    expect(initials('')).toBe('?')
    expect(gradeClass('Istimewa')).toBe('b-ist')
    expect(gradeClass('Perlu Perbaikan')).toBe('b-warn')
    expect(gradeClass('')).toBe('b-mute')
  })
})
