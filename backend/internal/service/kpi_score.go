package service

import (
	"errors"
	"strings"

	"github.com/shopspring/decimal"
)

const (
	DirectionHigher = "higher"
	DirectionLower  = "lower"

	// MaxAchievement membatasi capaian satu KPI agar tidak menutupi KPI lain.
	MaxAchievement = 120
)

var (
	ErrInvalidTarget    = errors.New("target harus lebih besar dari 0")
	ErrInvalidActual    = errors.New("realisasi tidak boleh negatif")
	ErrInvalidDirection = errors.New("arah KPI harus higher atau lower")
	ErrWeightNot100     = errors.New("total bobot indikator harus 100%")
)

var (
	dec100         = decimal.NewFromInt(100)
	decMaxAchieve  = decimal.NewFromInt(MaxAchievement)
	weightEpsilon  = decimal.RequireFromString("0.001")
	decZero        = decimal.Zero
	scoreRoundBase = int32(2)
)

// KpiScoreService menghitung capaian dan skor KPI. Tanpa state sehingga mudah diuji.
// Semua nilai antara dibulatkan ke 2 desimal (half-up) karena itulah nilai yang disimpan.
type KpiScoreService struct{}

// IndicatorInput adalah satu indikator beserta realisasinya.
// Actual nil artinya belum ada realisasi tervalidasi.
type IndicatorInput struct {
	IndicatorID uint64
	Target      decimal.Decimal
	Weight      decimal.Decimal // persen, mis. 25 untuk 25%
	Direction   string
	Actual      *decimal.Decimal
}

// IndicatorLine adalah hasil hitung satu indikator.
type IndicatorLine struct {
	IndicatorID uint64
	Achievement decimal.Decimal // 0–120
	Weighted    decimal.Decimal // capaian × bobot (poin ke skor KPI)
	Missing     bool            // tidak ada realisasi tervalidasi
}

// KpiScoreResult adalah skor KPI satu karyawan pada satu periode.
type KpiScoreResult struct {
	Lines    []IndicatorLine
	Score    decimal.Decimal
	Complete bool // semua indikator punya realisasi tervalidasi
}

// Achievement menghitung capaian satu KPI, dibatasi maksimum 120.
//
//	higher: min(realisasi/target × 100, 120)
//	lower : min(target/realisasi × 100, 120); realisasi 0 → 120
func (KpiScoreService) Achievement(actual, target decimal.Decimal, direction string) (decimal.Decimal, error) {
	if target.LessThanOrEqual(decZero) {
		return decZero, ErrInvalidTarget
	}
	if actual.IsNegative() {
		return decZero, ErrInvalidActual
	}
	var raw decimal.Decimal
	switch direction {
	case DirectionHigher:
		raw = actual.Mul(dec100).Div(target)
	case DirectionLower:
		if actual.IsZero() {
			return decMaxAchieve, nil
		}
		raw = target.Mul(dec100).Div(actual)
	default:
		return decZero, ErrInvalidDirection
	}
	if raw.GreaterThan(decMaxAchieve) {
		raw = decMaxAchieve
	}
	return raw.Round(scoreRoundBase), nil
}

// ValidateWeights memastikan total bobot (dalam persen) = 100 dan tidak ada bobot negatif.
func (KpiScoreService) ValidateWeights(weights []decimal.Decimal) error {
	sum := decZero
	for _, w := range weights {
		if w.IsNegative() {
			return ErrWeightNot100
		}
		sum = sum.Add(w)
	}
	if sum.Sub(dec100).Abs().GreaterThan(weightEpsilon) {
		return ErrWeightNot100
	}
	return nil
}

// Score menghitung skor KPI berbobot: Σ capaian × bobot/100.
// Indikator tanpa realisasi tervalidasi dihitung 0 (bukan 120 walau arahnya "lower").
func (s KpiScoreService) Score(items []IndicatorInput) (KpiScoreResult, error) {
	weights := make([]decimal.Decimal, len(items))
	for i, it := range items {
		weights[i] = it.Weight
	}
	if err := s.ValidateWeights(weights); err != nil {
		return KpiScoreResult{}, err
	}

	res := KpiScoreResult{Lines: make([]IndicatorLine, 0, len(items)), Complete: true}
	total := decZero
	for _, it := range items {
		line := IndicatorLine{IndicatorID: it.IndicatorID}
		if it.Actual == nil {
			line.Missing = true
			res.Complete = false
		} else {
			ach, err := s.Achievement(*it.Actual, it.Target, it.Direction)
			if err != nil {
				return KpiScoreResult{}, err
			}
			line.Achievement = ach
			line.Weighted = ach.Mul(it.Weight).Div(dec100)
			total = total.Add(line.Weighted)
		}
		res.Lines = append(res.Lines, line)
	}
	res.Score = total.Round(scoreRoundBase)
	return res, nil
}

// FormatScore menampilkan skor 1 desimal dengan koma, mis. 95,7.
func FormatScore(v decimal.Decimal) string {
	return strings.Replace(v.Round(1).StringFixed(1), ".", ",", 1)
}
