import { describe, expect, it } from 'vitest'
import { achievement, behaviorScore, finalScore, gradeOf, round2 } from './scoring'

describe('achievement', () => {
  it.each([
    [104, 120, 'higher', 86.67],
    [95, 95, 'higher', 100],
    [300, 100, 'higher', 120], // dibatasi 120
    [0, 10, 'higher', 0],
    [4.7, 4.5, 'higher', 104.44],
    [12, 15, 'lower', 120], // 15/12 = 125 → 120
    [20, 15, 'lower', 75],
    [3, 3, 'lower', 100],
  ] as const)('realisasi %s target %s (%s) → %s', (a, t, d, want) => {
    expect(achievement(a, t, d)).toBe(want)
  })

  it('realisasi 0 pada arah lebih rendah = 120', () => {
    expect(achievement(0, 3, 'lower')).toBe(120)
  })

  it('input tidak valid → 0', () => {
    expect(achievement(5, 0, 'higher')).toBe(0)
    expect(achievement(-1, 5, 'higher')).toBe(0)
    expect(achievement(Number.NaN, 5, 'higher')).toBe(0)
  })
})

describe('skor perilaku & akhir', () => {
  it('contoh H-07: nilai atasan 4,5,4,4,5 → perilaku 88,0', () => {
    expect(behaviorScore([4, 5, 4, 4, 5])).toBe(88)
  })

  it('contoh H-07: KPI 95,7 + perilaku 88,0 → 93,4 (Istimewa)', () => {
    const f = finalScore(95.7, 88, 70, 30)
    expect(f).toBe(93.39)
    expect(Math.round(f * 10) / 10).toBe(93.4)
    expect(gradeOf(f)).toBe('Istimewa')
  })

  it('bobot periode dapat diatur', () => {
    expect(finalScore(80, 100, 50, 50)).toBe(90)
  })

  it('tanpa nilai atasan → null', () => {
    expect(behaviorScore([])).toBeNull()
  })
})

describe('predikat — batas', () => {
  it.each([
    [114, 'Istimewa'],
    [90, 'Istimewa'],
    [89.99, 'Baik'],
    [80, 'Baik'],
    [79.99, 'Cukup'],
    [70, 'Cukup'],
    [69.99, 'Perlu Perbaikan'],
    [60, 'Perlu Perbaikan'],
    [59.99, 'Kurang'],
    [0, 'Kurang'],
  ] as const)('%s → %s', (score, grade) => {
    expect(gradeOf(score)).toBe(grade)
  })

  it('dibulatkan 2 desimal dulu: 89,994 → Baik, 89,995 → Istimewa', () => {
    expect(gradeOf(89.994)).toBe('Baik')
    expect(gradeOf(89.995)).toBe('Istimewa')
  })
})

describe('round2', () => {
  it('half-up', () => {
    expect(round2(1.005)).toBe(1.01)
    expect(round2(95.6485)).toBe(95.65)
  })
})
