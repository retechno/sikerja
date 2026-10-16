package service

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func d(s string) decimal.Decimal { return decimal.RequireFromString(s) }

func dp(s string) *decimal.Decimal { v := d(s); return &v }

func TestAchievement(t *testing.T) {
	svc := KpiScoreService{}
	tests := []struct {
		name      string
		actual    string
		target    string
		direction string
		want      string
	}{
		{"higher di bawah target", "104", "120", DirectionHigher, "86.67"},
		{"higher tepat target", "95", "95", DirectionHigher, "100"},
		{"higher melebihi target dibatasi 120", "300", "100", DirectionHigher, "120"},
		{"higher tepat 120 persen", "6", "5", DirectionHigher, "120"},
		{"higher realisasi 0", "0", "10", DirectionHigher, "0"},
		{"higher desimal", "4.7", "4.5", DirectionHigher, "104.44"},
		{"lower lebih baik dari target", "12", "15", DirectionLower, "120"},
		{"lower sedikit lebih baik dibatasi 120", "10", "15", DirectionLower, "120"},
		{"lower di atas target", "20", "15", DirectionLower, "75"},
		{"lower tepat target", "3", "3", DirectionLower, "100"},
		{"lower realisasi 0 = 120", "0", "3", DirectionLower, "120"},
		{"lower sangat buruk", "300", "3", DirectionLower, "1"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := svc.Achievement(d(tc.actual), d(tc.target), tc.direction)
			require.NoError(t, err)
			assert.True(t, d(tc.want).Equal(got), "want %s got %s", tc.want, got)
		})
	}
}

func TestAchievementInvalid(t *testing.T) {
	svc := KpiScoreService{}
	_, err := svc.Achievement(d("1"), d("0"), DirectionHigher)
	assert.ErrorIs(t, err, ErrInvalidTarget)
	_, err = svc.Achievement(d("1"), d("-5"), DirectionLower)
	assert.ErrorIs(t, err, ErrInvalidTarget)
	_, err = svc.Achievement(d("-1"), d("5"), DirectionHigher)
	assert.ErrorIs(t, err, ErrInvalidActual)
	_, err = svc.Achievement(d("1"), d("5"), "sideways")
	assert.ErrorIs(t, err, ErrInvalidDirection)
}

// Lima indikator IT Support dari mockup H-05 (Rina Wulandari, Okt 2026).
func itSupportInputs(instalasiActual *decimal.Decimal) []IndicatorInput {
	return []IndicatorInput{
		{IndicatorID: 1, Target: d("120"), Weight: d("25"), Direction: DirectionHigher, Actual: dp("104")},
		{IndicatorID: 2, Target: d("95"), Weight: d("25"), Direction: DirectionHigher, Actual: dp("96.2")},
		{IndicatorID: 3, Target: d("15"), Weight: d("15"), Direction: DirectionLower, Actual: dp("12")},
		{IndicatorID: 4, Target: d("4"), Weight: d("20"), Direction: DirectionHigher, Actual: instalasiActual},
		{IndicatorID: 5, Target: d("4.5"), Weight: d("15"), Direction: DirectionHigher, Actual: dp("4.7")},
	}
}

func TestScoreMockupH05(t *testing.T) {
	res, err := KpiScoreService{}.Score(itSupportInputs(dp("3")))
	require.NoError(t, err)
	require.Len(t, res.Lines, 5)
	assert.True(t, res.Complete)

	wantAch := []string{"86.67", "101.26", "120", "75", "104.44"}
	for i, w := range wantAch {
		assert.True(t, d(w).Equal(res.Lines[i].Achievement), "baris %d: want %s got %s", i, w, res.Lines[i].Achievement)
	}
	// 21,6675 + 25,315 + 18 + 15 + 15,666 = 95,6485 -> 95,65 (tampil 95,7 seperti mockup)
	assert.True(t, d("95.65").Equal(res.Score), "got %s", res.Score)
	assert.Equal(t, "95,7", FormatScore(res.Score))
}

func TestScoreMissingResultCountsZero(t *testing.T) {
	res, err := KpiScoreService{}.Score(itSupportInputs(nil))
	require.NoError(t, err)
	assert.False(t, res.Complete)
	assert.True(t, res.Lines[3].Achievement.IsZero())
	assert.True(t, res.Lines[3].Missing)
	// 95,65 - 15 = 80,65
	assert.True(t, d("80.65").Equal(res.Score), "got %s", res.Score)
}

func TestScoreMissingLowerDirectionIsNotBonus(t *testing.T) {
	in := []IndicatorInput{
		{Target: d("3"), Weight: d("100"), Direction: DirectionLower, Actual: nil},
	}
	res, err := KpiScoreService{}.Score(in)
	require.NoError(t, err)
	assert.True(t, res.Score.IsZero(), "belum ada realisasi tidak boleh dianggap 0 bug (=120)")
}

func TestScoreActualZeroLowerIsMax(t *testing.T) {
	in := []IndicatorInput{
		{Target: d("3"), Weight: d("100"), Direction: DirectionLower, Actual: dp("0")},
	}
	res, err := KpiScoreService{}.Score(in)
	require.NoError(t, err)
	assert.True(t, d("120").Equal(res.Score))
}

func TestScoreWeightMustBe100(t *testing.T) {
	in := []IndicatorInput{
		{Target: d("10"), Weight: d("60"), Direction: DirectionHigher, Actual: dp("10")},
		{Target: d("10"), Weight: d("30"), Direction: DirectionHigher, Actual: dp("10")},
	}
	_, err := KpiScoreService{}.Score(in)
	assert.ErrorIs(t, err, ErrWeightNot100)
	_, err = KpiScoreService{}.Score(nil)
	assert.ErrorIs(t, err, ErrWeightNot100)
}

func TestValidateWeights(t *testing.T) {
	svc := KpiScoreService{}
	assert.NoError(t, svc.ValidateWeights([]decimal.Decimal{d("25"), d("25"), d("15"), d("20"), d("15")}))
	assert.NoError(t, svc.ValidateWeights([]decimal.Decimal{d("33.333"), d("33.333"), d("33.334")}))
	assert.ErrorIs(t, svc.ValidateWeights([]decimal.Decimal{d("50"), d("49")}), ErrWeightNot100)
	assert.ErrorIs(t, svc.ValidateWeights([]decimal.Decimal{d("50"), d("-10"), d("60")}), ErrWeightNot100)
}

func TestSeedTemplatesWeightsAre100(t *testing.T) {
	for dept, items := range DefaultKpiTemplates {
		ws := make([]decimal.Decimal, 0, len(items))
		for _, it := range items {
			ws = append(ws, d(it.Weight))
		}
		assert.NoError(t, KpiScoreService{}.ValidateWeights(ws), dept)
	}
}
