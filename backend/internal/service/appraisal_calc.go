package service

import (
	"errors"

	"github.com/shopspring/decimal"
)

const (
	GradeIstimewa       = "Istimewa"
	GradeBaik           = "Baik"
	GradeCukup          = "Cukup"
	GradePerluPerbaikan = "Perlu Perbaikan"
	GradeKurang         = "Kurang"
)

var (
	ErrNoBehaviorScores = errors.New("nilai perilaku dari atasan belum lengkap")
	ErrScoreOutOfRange  = errors.New("nilai perilaku harus 1 sampai 5")
	ErrPeriodWeights    = errors.New("bobot KPI + bobot perilaku periode harus 100")
)

// AppraisalService: rumus penilaian (file ini) dan alur penilaian (appraisal.go).
type AppraisalService struct{}

var gradeBands = []struct {
	min   decimal.Decimal
	grade string
}{
	{decimal.NewFromInt(90), GradeIstimewa},
	{decimal.NewFromInt(80), GradeBaik},
	{decimal.NewFromInt(70), GradeCukup},
	{decimal.NewFromInt(60), GradePerluPerbaikan},
}

// BehaviorScore = rata-rata nilai atasan (1–5) × 20. Self-assessment tidak ikut dihitung.
func (AppraisalService) BehaviorScore(managerScores []int) (decimal.Decimal, error) {
	if len(managerScores) == 0 {
		return decZero, ErrNoBehaviorScores
	}
	sum := 0
	for _, s := range managerScores {
		if s < 1 || s > 5 {
			return decZero, ErrScoreOutOfRange
		}
		sum += s
	}
	avg := decimal.NewFromInt(int64(sum)).Div(decimal.NewFromInt(int64(len(managerScores))))
	return avg.Mul(decimal.NewFromInt(20)).Round(scoreRoundBase), nil
}

// FinalScore = skor KPI × bobot KPI periode + skor perilaku × bobot perilaku periode.
// Bobot dalam persen (70 dan 30) dan harus berjumlah 100.
func (AppraisalService) FinalScore(kpi, behavior, kpiWeight, behaviorWeight decimal.Decimal) (decimal.Decimal, error) {
	if kpiWeight.IsNegative() || behaviorWeight.IsNegative() ||
		kpiWeight.Add(behaviorWeight).Sub(dec100).Abs().GreaterThan(weightEpsilon) {
		return decZero, ErrPeriodWeights
	}
	score := kpi.Mul(kpiWeight).Add(behavior.Mul(behaviorWeight)).Div(dec100)
	return score.Round(scoreRoundBase), nil
}

// Grade menentukan predikat dari skor akhir yang dibulatkan ke 2 desimal.
func (AppraisalService) Grade(score decimal.Decimal) string {
	score = score.Round(scoreRoundBase)
	for _, b := range gradeBands {
		if score.GreaterThanOrEqual(b.min) {
			return b.grade
		}
	}
	return GradeKurang
}
