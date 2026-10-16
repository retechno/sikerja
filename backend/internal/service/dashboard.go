package service

import (
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/shopspring/decimal"

	"github.com/retechno/sikerja/backend/internal/apperr"
	"github.com/retechno/sikerja/backend/internal/model"
	"github.com/retechno/sikerja/backend/internal/repository"
)

// DashboardService menyusun data tiga dasbor. Jumlah query tetap (tidak tergantung jumlah karyawan)
// dan hasilnya di-cache 5 menit di Redis (dibuang saat ada perubahan data).
type DashboardService struct {
	store repository.Store
	cache *Cache
}

func NewDashboardService(store repository.Store, cache *Cache) *DashboardService {
	return &DashboardService{store: store, cache: cache}
}

var gradeOrder = []string{GradeIstimewa, GradeBaik, GradeCukup, GradePerluPerbaikan, GradeKurang}

// resolvePeriod: periodID 0 → periode aktif terbaru, kalau tidak ada → periode terbaru yang bukan draft.
func resolvePeriod(r repository.Repos, id uint64) (*model.AppraisalPeriod, error) {
	if id != 0 {
		p, err := loadPeriod(r, id)
		if err != nil {
			return nil, err
		}
		if p.Status == model.PeriodDraft {
			return nil, apperr.NotFound("Periode tidak ditemukan")
		}
		return p, nil
	}
	ps, err := r.Periods.Latest(12)
	if err != nil {
		return nil, err
	}
	for i := len(ps) - 1; i >= 0; i-- {
		if ps[i].Status == model.PeriodActive {
			return &ps[i], nil
		}
	}
	if len(ps) > 0 {
		return &ps[len(ps)-1], nil
	}
	return nil, apperr.NotFound("Belum ada periode penilaian")
}

// window = hingga 6 periode non-draft berakhir pada p (urut lama → baru).
func window(r repository.Repos, p *model.AppraisalPeriod) ([]model.AppraisalPeriod, error) {
	all, err := r.Periods.Latest(60)
	if err != nil {
		return nil, err
	}
	var upto []model.AppraisalPeriod
	for _, x := range all {
		if !x.StartDate.After(p.StartDate) {
			upto = append(upto, x)
		}
	}
	if len(upto) > 6 {
		upto = upto[len(upto)-6:]
	}
	return upto, nil
}

func round2(f float64) decimal.Decimal { return decimal.NewFromFloat(f).Round(2) }

func nextGrade(score decimal.Decimal) (grade string, gap decimal.Decimal, ok bool) {
	for i := len(gradeBands) - 1; i >= 0; i-- {
		if score.LessThan(gradeBands[i].min) {
			return gradeBands[i].grade, gradeBands[i].min.Sub(score), true
		}
	}
	return "", decimal.Zero, false
}

// ---------- karyawan (H-02) ----------

type HistoryPoint struct {
	PeriodID uint64           `json:"period_id"`
	Name     string           `json:"name"`
	Score    *decimal.Decimal `json:"score"`
	Grade    string           `json:"grade"`
	Current  bool             `json:"current"`
}

type NextGrade struct {
	Grade string          `json:"grade"`
	Gap   decimal.Decimal `json:"gap"`
}

type LatestFinal struct {
	PeriodName string          `json:"period_name"`
	Score      decimal.Decimal `json:"score"`
	Grade      string          `json:"grade"`
	Next       *NextGrade      `json:"next"`
}

type EmployeeDash struct {
	Period            *model.AppraisalPeriod `json:"period"`
	Employee          *model.Employee        `json:"employee"`
	Sheet             *KpiSheet              `json:"sheet"`
	KpiScore          decimal.Decimal        `json:"kpi_score"`
	KpiDelta          *decimal.Decimal       `json:"kpi_delta"` // vs skor KPI periode final sebelumnya
	PrevPeriodName    string                 `json:"prev_period_name"`
	AppraisalStatus   string                 `json:"appraisal_status"`
	FinalScore        *decimal.Decimal       `json:"final_score"` // hanya bila final
	Grade             string                 `json:"grade"`
	Latest            *LatestFinal           `json:"latest_final"`
	Deadline          time.Time              `json:"deadline"`
	DaysLeft          int                    `json:"days_left"`
	PendingValidation int                    `json:"pending_validation"`
	History           []HistoryPoint         `json:"history"`
}

func (s *DashboardService) Employee(a Actor, periodID uint64) (*EmployeeDash, error) {
	r := s.store.R()
	p, err := resolvePeriod(r, periodID)
	if err != nil {
		return nil, err
	}
	return Remember(bgCtx(), s.cache, fmt.Sprintf("emp:%d:%d", a.EmployeeID, p.ID), func() (*EmployeeDash, error) {
		emp, err := loadEmployee(r, a.EmployeeID)
		if err != nil {
			return nil, err
		}
		if emp.DepartmentID == nil {
			return nil, apperr.BadRequest("Akun ini tidak memiliki KPI (tanpa departemen)")
		}
		sh, err := loadSheet(r, KpiScoreService{}, emp, p)
		if err != nil {
			return nil, err
		}
		win, err := window(r, p)
		if err != nil {
			return nil, err
		}
		ids := make([]uint64, len(win))
		for i, x := range win {
			ids[i] = x.ID
		}
		aps, err := r.Appraisals.ByEmployee(emp.ID, ids)
		if err != nil {
			return nil, err
		}
		byP := map[uint64]model.Appraisal{}
		for _, x := range aps {
			byP[x.PeriodID] = x
		}

		d := &EmployeeDash{Period: p, Employee: emp, Sheet: sh, KpiScore: sh.Score, AppraisalStatus: model.AppraisalDraft,
			Deadline: p.EndDate, PendingValidation: sh.Pending}
		d.DaysLeft = int(math.Ceil(time.Until(p.EndDate.Add(24*time.Hour)).Hours() / 24))
		if p.Status != model.PeriodActive || d.DaysLeft < 0 {
			d.DaysLeft = 0
		}
		if cur, ok := byP[p.ID]; ok {
			d.AppraisalStatus = cur.Status
			if cur.Status == model.AppraisalFinal && cur.FinalScore.Valid {
				f := cur.FinalScore.Decimal
				d.FinalScore, d.Grade = &f, cur.Grade
			}
		}
		for _, x := range win {
			hp := HistoryPoint{PeriodID: x.ID, Name: x.Name, Current: x.ID == p.ID}
			if ap, ok := byP[x.ID]; ok && ap.Status == model.AppraisalFinal && ap.FinalScore.Valid {
				v := ap.FinalScore.Decimal
				hp.Score, hp.Grade = &v, ap.Grade
			}
			d.History = append(d.History, hp)
		}
		// periode final terakhir sebelum periode ini
		for i := len(win) - 1; i >= 0; i-- {
			x := win[i]
			ap, ok := byP[x.ID]
			if x.ID == p.ID || !ok || ap.Status != model.AppraisalFinal || !ap.FinalScore.Valid {
				continue
			}
			lf := &LatestFinal{PeriodName: x.Name, Score: ap.FinalScore.Decimal, Grade: ap.Grade}
			if g, gap, ok := nextGrade(ap.FinalScore.Decimal); ok {
				lf.Next = &NextGrade{Grade: g, Gap: gap}
			}
			d.Latest = lf
			if ap.KpiScore.Valid {
				delta := sh.Score.Sub(ap.KpiScore.Decimal)
				d.KpiDelta, d.PrevPeriodName = &delta, x.Name
			}
			break
		}
		return d, nil
	})
}

// ---------- manajer (H-03) ----------

type RankRow struct {
	Employee   *model.Employee  `json:"employee"`
	PrevScore  *decimal.Decimal `json:"prev_score"`
	PrevGrade  string           `json:"prev_grade"`
	KpiScore   decimal.Decimal  `json:"kpi_score"` // sementara, periode terpilih
	FinalScore *decimal.Decimal `json:"final_score"`
	Status     string           `json:"status"`
}

type BelowTargetRow struct {
	Employee    *model.Employee `json:"employee"`
	Indicator   string          `json:"indicator"`
	Unit        string          `json:"unit"`
	Direction   string          `json:"direction"`
	Actual      decimal.Decimal `json:"actual"`
	Target      decimal.Decimal `json:"target"`
	Achievement decimal.Decimal `json:"achievement"`
}

type LowMember struct {
	Employee *model.Employee `json:"employee"`
	Score    decimal.Decimal `json:"score"`
}

type ManagerDash struct {
	Period             *model.AppraisalPeriod `json:"period"`
	PrevPeriodName     string                 `json:"prev_period_name"`
	PrevAvg            *decimal.Decimal       `json:"prev_avg"`
	PrevAvgDelta       *decimal.Decimal       `json:"prev_avg_delta"`
	PendingValidations int                    `json:"pending_validations"`
	MembersPending     int                    `json:"members_pending"`
	MembersTotal       int                    `json:"members_total"`
	NotAppraised       int                    `json:"not_appraised"` // belum dikirim ke HR/Direktur
	LowMembers         []LowMember            `json:"low_members"`   // skor final < 70 periode sebelumnya
	Ranking            []RankRow              `json:"ranking"`
	BelowTarget        []BelowTargetRow       `json:"below_target"`
}

func avgScores(aps map[uint64]model.Appraisal, onlyFinal bool) *decimal.Decimal {
	sum, n := decimal.Zero, 0
	for _, x := range aps {
		if x.FinalScore.Valid && (!onlyFinal || x.Status == model.AppraisalFinal) {
			sum = sum.Add(x.FinalScore.Decimal)
			n++
		}
	}
	if n == 0 {
		return nil
	}
	v := sum.Div(decimal.NewFromInt(int64(n))).Round(2)
	return &v
}

func (s *DashboardService) Manager(a Actor, periodID uint64) (*ManagerDash, error) {
	if !a.HasAny(model.RoleManager, model.RoleDirector) {
		return nil, apperr.Forbidden("Dasbor ini hanya untuk manajer")
	}
	r := s.store.R()
	p, err := resolvePeriod(r, periodID)
	if err != nil {
		return nil, err
	}
	return Remember(bgCtx(), s.cache, fmt.Sprintf("mgr:%d:%d", a.EmployeeID, p.ID), func() (*ManagerDash, error) {
		td, err := loadTeamData(r, a, p)
		if err != nil {
			return nil, err
		}
		ids := make([]uint64, len(td.Members))
		for i, m := range td.Members {
			ids[i] = m.ID
		}
		// dua periode final terakhir sebelum periode terpilih → skor sebelumnya + delta
		win, err := window(r, p)
		if err != nil {
			return nil, err
		}
		var prevPeriods []model.AppraisalPeriod
		for i := len(win) - 1; i >= 0 && len(prevPeriods) < 2; i-- {
			if win[i].ID != p.ID {
				prevPeriods = append(prevPeriods, win[i])
			}
		}
		var prev, prev2 map[uint64]model.Appraisal
		d := &ManagerDash{Period: p, MembersTotal: len(td.Members), LowMembers: []LowMember{}, Ranking: []RankRow{}, BelowTarget: []BelowTargetRow{}}
		if len(prevPeriods) > 0 {
			prev, err = apprMap(r, prevPeriods[0].ID, ids)
			if err != nil {
				return nil, err
			}
			d.PrevPeriodName = prevPeriods[0].Name
			d.PrevAvg = avgScores(prev, true)
		}
		if len(prevPeriods) > 1 {
			prev2, err = apprMap(r, prevPeriods[1].ID, ids)
			if err != nil {
				return nil, err
			}
			if a2, a1 := avgScores(prev2, true), d.PrevAvg; a1 != nil && a2 != nil {
				delta := a1.Sub(*a2)
				d.PrevAvgDelta = &delta
			}
		}

		for i := range td.Members {
			m := &td.Members[i]
			sh := td.Sheets[m.ID]
			if sh.Pending > 0 {
				d.MembersPending++
				d.PendingValidations += sh.Pending
			}
			row := RankRow{Employee: m, KpiScore: sh.Score, Status: model.AppraisalDraft}
			if ap, ok := td.Apprs[m.ID]; ok {
				row.Status = ap.Status
				if ap.FinalScore.Valid {
					v := ap.FinalScore.Decimal
					row.FinalScore = &v
				}
			}
			if row.Status != model.AppraisalManagerDone && row.Status != model.AppraisalFinal {
				d.NotAppraised++
			}
			if pa, ok := prev[m.ID]; ok && pa.Status == model.AppraisalFinal && pa.FinalScore.Valid {
				v := pa.FinalScore.Decimal
				row.PrevScore, row.PrevGrade = &v, pa.Grade
				if v.LessThan(decimal.NewFromInt(70)) {
					d.LowMembers = append(d.LowMembers, LowMember{Employee: m, Score: v})
				}
			}
			d.Ranking = append(d.Ranking, row)

			for _, kr := range sh.Rows {
				if kr.Result == nil || kr.Status == model.ResultRejected || kr.Achievement.GreaterThanOrEqual(dec100) {
					continue
				}
				d.BelowTarget = append(d.BelowTarget, BelowTargetRow{Employee: m, Indicator: kr.Indicator.Name, Unit: kr.Indicator.Unit,
					Direction: kr.Indicator.Direction, Actual: kr.Result.Actual, Target: kr.Indicator.Target, Achievement: kr.Achievement})
			}
		}
		sort.SliceStable(d.Ranking, func(i, j int) bool { return rankKey(d.Ranking[i]).GreaterThan(rankKey(d.Ranking[j])) })
		sort.SliceStable(d.BelowTarget, func(i, j int) bool { return d.BelowTarget[i].Achievement.LessThan(d.BelowTarget[j].Achievement) })
		if len(d.BelowTarget) > 10 {
			d.BelowTarget = d.BelowTarget[:10]
		}
		sort.SliceStable(d.LowMembers, func(i, j int) bool { return d.LowMembers[i].Score.LessThan(d.LowMembers[j].Score) })
		return d, nil
	})
}

func rankKey(r RankRow) decimal.Decimal {
	if r.PrevScore != nil {
		return *r.PrevScore
	}
	return r.KpiScore
}

func apprMap(r repository.Repos, periodID uint64, ids []uint64) (map[uint64]model.Appraisal, error) {
	aps, err := r.Appraisals.ForEmployees(periodID, ids)
	if err != nil {
		return nil, err
	}
	m := make(map[uint64]model.Appraisal, len(aps))
	for _, x := range aps {
		m[x.EmployeeID] = x
	}
	return m, nil
}

// ---------- direktur (H-04) ----------

type DeptAvg struct {
	DepartmentID uint64          `json:"department_id"`
	Name         string          `json:"name"`
	Avg          decimal.Decimal `json:"avg"`
	Count        int64           `json:"count"`
}

type TrendPoint struct {
	PeriodID uint64           `json:"period_id"`
	Name     string           `json:"name"`
	Avg      *decimal.Decimal `json:"avg"`
}

type GradeCount struct {
	Grade string `json:"grade"`
	Count int64  `json:"count"`
}

type ScoreItem struct {
	EmployeeID uint64          `json:"employee_id"`
	Name       string          `json:"name"`
	Department string          `json:"department"`
	Score      decimal.Decimal `json:"score"`
	Grade      string          `json:"grade"`
}

type DirectorDash struct {
	Period         *model.AppraisalPeriod `json:"period"`
	Employees      int                    `json:"employees"` // karyawan aktif yang dinilai
	Scored         int                    `json:"scored"`    // sudah punya skor akhir
	CompanyAvg     *decimal.Decimal       `json:"company_avg"`
	CompanyDelta   *decimal.Decimal       `json:"company_delta"`
	CompanyGrade   string                 `json:"company_grade"`
	Departments    []DeptAvg              `json:"departments"`
	BestDepartment *DeptAvg               `json:"best_department"`
	Trend          []TrendPoint           `json:"trend"`
	Grades         []GradeCount           `json:"grades"`
	Attention      int                    `json:"attention"` // skor < 70
	Top            []ScoreItem            `json:"top"`
	Bottom         []ScoreItem            `json:"bottom"`
}

func (s *DashboardService) Director(a Actor, periodID uint64) (*DirectorDash, error) {
	if !a.HasAny(model.RoleDirector, model.RoleHR) {
		return nil, apperr.Forbidden("Dasbor ini hanya untuk Direktur")
	}
	r := s.store.R()
	p, err := resolvePeriod(r, periodID)
	if err != nil {
		return nil, err
	}
	return Remember(bgCtx(), s.cache, fmt.Sprintf("dir:%d", p.ID), func() (*DirectorDash, error) {
		win, err := window(r, p)
		if err != nil {
			return nil, err
		}
		ids := make([]uint64, len(win))
		for i, x := range win {
			ids[i] = x.ID
		}
		avgs, err := r.Reports.DeptAverages(ids)
		if err != nil {
			return nil, err
		}
		rows, err := r.Reports.ScoresForPeriod(p.ID, 0, nil)
		if err != nil {
			return nil, err
		}

		d := &DirectorDash{Period: p, Employees: len(rows), Departments: []DeptAvg{}, Trend: []TrendPoint{}, Top: []ScoreItem{}, Bottom: []ScoreItem{}}
		sumBy, cntBy := map[uint64]float64{}, map[uint64]int64{}
		for _, x := range avgs {
			sumBy[x.PeriodID] += x.Avg * float64(x.Count)
			cntBy[x.PeriodID] += x.Count
			if x.PeriodID == p.ID {
				d.Departments = append(d.Departments, DeptAvg{DepartmentID: x.DepartmentID, Name: x.DepartmentName, Avg: round2(x.Avg), Count: x.Count})
			}
		}
		for _, x := range win {
			tp := TrendPoint{PeriodID: x.ID, Name: x.Name}
			if c := cntBy[x.ID]; c > 0 {
				v := round2(sumBy[x.ID] / float64(c))
				tp.Avg = &v
			}
			d.Trend = append(d.Trend, tp)
		}
		for i, tp := range d.Trend {
			if tp.PeriodID == p.ID && tp.Avg != nil {
				d.CompanyAvg = tp.Avg
				d.CompanyGrade = AppraisalService{}.Grade(*tp.Avg)
				if i > 0 && d.Trend[i-1].Avg != nil {
					delta := tp.Avg.Sub(*d.Trend[i-1].Avg)
					d.CompanyDelta = &delta
				}
			}
		}
		sort.SliceStable(d.Departments, func(i, j int) bool { return d.Departments[i].Avg.GreaterThan(d.Departments[j].Avg) })
		if len(d.Departments) > 0 {
			best := d.Departments[0]
			d.BestDepartment = &best
		}

		var scored []ScoreItem
		counts := map[string]int64{}
		for _, row := range rows {
			if !row.FinalScore.Valid {
				continue
			}
			g := ""
			if row.Grade != nil {
				g = *row.Grade
			}
			scored = append(scored, ScoreItem{EmployeeID: row.EmployeeID, Name: row.EmployeeName, Department: row.DepartmentName, Score: row.FinalScore.Decimal, Grade: g})
			counts[g]++
			if row.FinalScore.Decimal.LessThan(decimal.NewFromInt(70)) {
				d.Attention++
			}
		}
		d.Scored = len(scored)
		for _, g := range gradeOrder {
			d.Grades = append(d.Grades, GradeCount{Grade: g, Count: counts[g]})
		}
		sort.SliceStable(scored, func(i, j int) bool { return scored[i].Score.GreaterThan(scored[j].Score) })
		for i := 0; i < len(scored) && i < 5; i++ {
			d.Top = append(d.Top, scored[i])
		}
		for i := len(scored) - 1; i >= 0 && len(d.Bottom) < 5; i-- {
			d.Bottom = append(d.Bottom, scored[i])
		}
		return d, nil
	})
}
