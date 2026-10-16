// Pratinjau skor di sisi klien. Sumber kebenaran tetap backend (KpiScoreService / AppraisalService);
// rumus di sini sama persis supaya form menampilkan angka yang sama sebelum disimpan.

export const MAX_ACHIEVEMENT = 120

export type Direction = 'higher' | 'lower'
export type Grade = 'Istimewa' | 'Baik' | 'Cukup' | 'Perlu Perbaikan' | 'Kurang'

/** Pembulatan half-up ke 2 desimal (nilai yang disimpan backend). */
export function round2(x: number): number {
  return Math.round((x + Number.EPSILON) * 100) / 100
}

/** Capaian per KPI, dibatasi 120. Lebih rendah lebih baik: realisasi 0 → 120. */
export function achievement(actual: number, target: number, direction: Direction): number {
  if (!(target > 0) || actual < 0 || Number.isNaN(actual)) return 0
  let raw: number
  if (direction === 'lower') {
    if (actual === 0) return MAX_ACHIEVEMENT
    raw = (target / actual) * 100
  } else {
    raw = (actual / target) * 100
  }
  return round2(Math.min(raw, MAX_ACHIEVEMENT))
}

/** Skor perilaku = rata-rata nilai atasan (1–5) × 20. Null bila belum ada nilai. */
export function behaviorScore(scores: number[]): number | null {
  if (scores.length === 0) return null
  const avg = scores.reduce((a, b) => a + b, 0) / scores.length
  return round2(avg * 20)
}

/** Skor akhir = KPI × bobot KPI periode + perilaku × bobot perilaku periode (bobot dalam persen). */
export function finalScore(kpi: number, behavior: number, kpiWeight: number, behaviorWeight: number): number {
  return round2((kpi * kpiWeight + behavior * behaviorWeight) / 100)
}

/** Predikat dari skor yang dibulatkan 2 desimal: 89,99 → Baik, 90 → Istimewa. */
export function gradeOf(score: number): Grade {
  const s = round2(score)
  if (s >= 90) return 'Istimewa'
  if (s >= 80) return 'Baik'
  if (s >= 70) return 'Cukup'
  if (s >= 60) return 'Perlu Perbaikan'
  return 'Kurang'
}
