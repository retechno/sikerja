package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBehaviorScore(t *testing.T) {
	svc := AppraisalService{}
	tests := []struct {
		name   string
		scores []int
		want   string
	}{
		{"mockup H-07: 4,5,4,4,5 → rata-rata 4,4", []int{4, 5, 4, 4, 5}, "88"},
		{"semua 5", []int{5, 5, 5, 5, 5}, "100"},
		{"semua 1", []int{1, 1, 1, 1, 1}, "20"},
		{"campuran", []int{3, 4, 5, 2, 3}, "68"},
		{"rata-rata tidak bulat", []int{4, 4, 4, 4, 5}, "84"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := svc.BehaviorScore(tc.scores)
			require.NoError(t, err)
			assert.True(t, d(tc.want).Equal(got), "want %s got %s", tc.want, got)
		})
	}
}

func TestBehaviorScoreInvalid(t *testing.T) {
	svc := AppraisalService{}
	_, err := svc.BehaviorScore(nil)
	assert.ErrorIs(t, err, ErrNoBehaviorScores)
	_, err = svc.BehaviorScore([]int{4, 6})
	assert.ErrorIs(t, err, ErrScoreOutOfRange)
	_, err = svc.BehaviorScore([]int{0, 3})
	assert.ErrorIs(t, err, ErrScoreOutOfRange)
}

func TestFinalScore(t *testing.T) {
	svc := AppraisalService{}

	// Contoh H-07 di SPEC: KPI 95,7 + perilaku 88,0 → 0,7×95,7 + 0,3×88,0 = 93,4 (Istimewa).
	got, err := svc.FinalScore(d("95.7"), d("88"), d("70"), d("30"))
	require.NoError(t, err)
	assert.True(t, d("93.39").Equal(got), "got %s", got)
	assert.Equal(t, "93,4", FormatScore(got))
	assert.Equal(t, GradeIstimewa, svc.Grade(got))

	// Bobot periode bisa diatur HR.
	got, err = svc.FinalScore(d("80"), d("100"), d("50"), d("50"))
	require.NoError(t, err)
	assert.True(t, d("90").Equal(got))

	got, err = svc.FinalScore(d("100"), d("0"), d("100"), d("0"))
	require.NoError(t, err)
	assert.True(t, d("100").Equal(got))

	_, err = svc.FinalScore(d("90"), d("90"), d("70"), d("20"))
	assert.ErrorIs(t, err, ErrPeriodWeights)
}

func TestFinalScoreMockupH07EndToEnd(t *testing.T) {
	kpi, err := KpiScoreService{}.Score(itSupportInputs(dp("3")))
	require.NoError(t, err)
	beh, err := AppraisalService{}.BehaviorScore([]int{4, 5, 4, 4, 5})
	require.NoError(t, err)
	final, err := AppraisalService{}.FinalScore(kpi.Score, beh, d("70"), d("30"))
	require.NoError(t, err)
	assert.Equal(t, "95,7", FormatScore(kpi.Score))
	assert.Equal(t, "88,0", FormatScore(beh))
	assert.Equal(t, "93,4", FormatScore(final))
	assert.Equal(t, GradeIstimewa, AppraisalService{}.Grade(final))
}

func TestGrade(t *testing.T) {
	svc := AppraisalService{}
	tests := []struct {
		score string
		want  string
	}{
		{"114", GradeIstimewa},
		{"100", GradeIstimewa},
		{"90", GradeIstimewa},
		{"90.00", GradeIstimewa},
		{"89.99", GradeBaik},
		{"80", GradeBaik},
		{"79.99", GradeCukup},
		{"70", GradeCukup},
		{"69.99", GradePerluPerbaikan},
		{"60", GradePerluPerbaikan},
		{"59.99", GradeKurang},
		{"0", GradeKurang},
	}
	for _, tc := range tests {
		t.Run(tc.score, func(t *testing.T) {
			assert.Equal(t, tc.want, svc.Grade(d(tc.score)))
		})
	}
}

func TestGradeRounding(t *testing.T) {
	svc := AppraisalService{}
	// Skor dibulatkan ke 2 desimal (half-up) sebelum menentukan predikat.
	assert.Equal(t, GradeBaik, svc.Grade(d("89.994")))
	assert.Equal(t, GradeIstimewa, svc.Grade(d("89.995")))
	assert.Equal(t, GradeCukup, svc.Grade(d("79.994")))
	assert.Equal(t, GradeBaik, svc.Grade(d("79.995")))
}
