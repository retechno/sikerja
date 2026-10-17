package service_test

import (
	"bytes"
	"fmt"
	"net/http"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"

	"github.com/retechno/sikerja/backend/internal/model"
	"github.com/retechno/sikerja/backend/internal/service"
	"github.com/retechno/sikerja/backend/internal/testutil"
)

type dash struct {
	*svcs
	dash   *service.DashboardService
	report *service.ReportService
}

func newDash(t *testing.T) *dash {
	s := newSvcs(t)
	return &dash{svcs: s, dash: service.NewDashboardService(s.w.Store, service.NewCache(s.w.Rdb)), report: service.NewReportService(s.w.Store)}
}

// finalize menjalankan alur lengkap sampai final untuk satu karyawan IT.
func (d *dash) finalize(t *testing.T, emp *model.Employee, behavior int) {
	t.Helper()
	w := d.w
	rs := d.fillAll(t, emp, emp, w.Active.ID)
	d.validateAll(t, w.MgrIT, rs)
	_, err := d.app.SaveManager(w.Actor(w.MgrIT), emp.ID, w.Active.ID, service.ManagerInput{Scores: d.managerScores(behavior, behavior, behavior, behavior, behavior), Submit: true})
	require.NoError(t, err)
	_, err = d.app.Finalize(w.Actor(w.HR), emp.ID, w.Active.ID)
	require.NoError(t, err)
}

// addIT menambah n karyawan IT (langsung lewat repository) untuk uji skala.
func addIT(t *testing.T, w *testutil.World, from, n int) {
	t.Helper()
	r := w.Store.R()
	for i := from; i < from+n; i++ {
		u := &model.User{Email: fmt.Sprintf("extra%d@test.id", i), Password: "x", IsActive: true}
		require.NoError(t, r.Users.Create(u))
		require.NoError(t, r.Users.SetRoles(u.ID, []string{"employee"}))
		e := &model.Employee{UserID: u.ID, NIK: fmt.Sprintf("EX-%03d", i), Name: fmt.Sprintf("Extra %03d", i), DepartmentID: &w.ITS.ID,
			ManagerID: &w.MgrIT.ID, JoinDate: w.MgrIT.JoinDate, Status: model.StatusActive}
		require.NoError(t, r.Employees.Create(e))
		for j, ind := range w.TplIT.Indicators {
			_ = j
			res := &model.KpiResult{PeriodID: w.Active.ID, EmployeeID: e.ID, IndicatorID: ind.ID, Actual: ind.Target, Achievement: dd("100"), Status: model.ResultSubmitted}
			require.NoError(t, r.Results.Upsert(res))
		}
		ap := &model.Appraisal{PeriodID: w.Active.ID, EmployeeID: e.ID, Status: model.AppraisalDraft}
		require.NoError(t, r.Appraisals.Create(ap))
	}
}

func countQueries(db *gorm.DB) (get func() int64) {
	var n atomic.Int64
	_ = db.Callback().Query().Register("test:count", func(*gorm.DB) { n.Add(1) })
	_ = db.Callback().Row().Register("test:count_row", func(*gorm.DB) { n.Add(1) })
	return n.Load
}

func TestDashboardsHaveConstantQueryCount(t *testing.T) {
	d := newDash(t)
	w := d.w
	queries := countQueries(w.DB)
	// cache dimatikan agar yang diukur benar-benar query DB
	noCache := service.NewDashboardService(w.Store, nil)
	team := service.NewAppraisalService(w.Store, nil)

	measure := func() map[string]int64 {
		out := map[string]int64{}
		for name, fn := range map[string]func() error{
			"manager":  func() error { _, err := noCache.Manager(w.Actor(w.MgrIT), w.Active.ID); return err },
			"team":     func() error { _, err := team.Team(w.Actor(w.MgrIT), w.Active.ID); return err },
			"director": func() error { _, err := noCache.Director(w.Actor(w.Director), w.Active.ID); return err },
			"report":   func() error { _, err := d.report.Build(w.Actor(w.HR), w.Active.ID, 0); return err },
		} {
			before := queries()
			require.NoError(t, fn(), name)
			out[name] = queries() - before
		}
		return out
	}
	// 1 karyawan tambahan lebih dulu agar jalur data-dependen (mis. preload Scores) ikut terukur
	addIT(t, w, 0, 1)
	small := measure()
	addIT(t, w, 1, 40)
	big := measure()
	for k, v := range small {
		assert.Equal(t, v, big[k], "jumlah query %q tidak boleh bertambah dengan jumlah karyawan (tanpa N+1)", k)
	}
	assert.Less(t, big["manager"], int64(30))
}

func TestManagerDashboardAndCacheInvalidation(t *testing.T) {
	d := newDash(t)
	w := d.w
	d.fillAll(t, w.EmpIT1, w.EmpIT1, w.Active.ID) // 5 menunggu validasi
	bayu := w.Actor(w.MgrIT)

	md, err := d.dash.Manager(bayu, w.Active.ID)
	require.NoError(t, err)
	assert.Equal(t, 5, md.PendingValidations)
	assert.Equal(t, 1, md.MembersPending)
	assert.Equal(t, 2, md.MembersTotal)
	assert.Len(t, md.Ranking, 2)
	require.NotEmpty(t, md.BelowTarget)
	assert.Equal(t, "Instalasi / training PMS di hotel klien", md.BelowTarget[0].Indicator, "terburuk dulu (75,0)")

	// cache menyimpan hasil; setelah validasi (yang menaikkan versi cache) hasil harus segar
	rs, err := w.Store.R().Results.ForEmployee(w.Active.ID, w.EmpIT1.ID)
	require.NoError(t, err)
	_, err = d.kpi.Validate(bayu, rs[0].ID, service.ActionValidate, "")
	require.NoError(t, err)
	md, err = d.dash.Manager(bayu, w.Active.ID)
	require.NoError(t, err)
	assert.Equal(t, 4, md.PendingValidations, "cache dibuang setelah data berubah")

	// hanya manajer
	_, err = d.dash.Manager(w.Actor(w.EmpIT1), w.Active.ID)
	assert.Equal(t, http.StatusForbidden, status(t, err))
}

func TestEmployeeDashboardHidesScoreUntilFinal(t *testing.T) {
	d := newDash(t)
	w := d.w
	rs := d.fillAll(t, w.EmpIT1, w.EmpIT1, w.Active.ID)
	d.validateAll(t, w.MgrIT, rs)
	_, err := d.app.SaveManager(w.Actor(w.MgrIT), w.EmpIT1.ID, w.Active.ID, service.ManagerInput{Scores: d.managerScores(5, 5, 5, 5, 5), Submit: true})
	require.NoError(t, err)

	rina := w.Actor(w.EmpIT1)
	ed, err := d.dash.Employee(rina, w.Active.ID)
	require.NoError(t, err)
	assert.Equal(t, "95,7", service.FormatScore(ed.KpiScore))
	assert.Nil(t, ed.FinalScore, "skor akhir belum final → tidak ditampilkan ke karyawan")
	assert.Empty(t, ed.Grade)
	assert.Len(t, ed.Sheet.Rows, 5)

	_, err = d.app.Finalize(w.Actor(w.Director), w.EmpIT1.ID, w.Active.ID)
	require.NoError(t, err)
	ed, err = d.dash.Employee(rina, w.Active.ID)
	require.NoError(t, err)
	require.NotNil(t, ed.FinalScore)
	assert.Equal(t, "97,0", service.FormatScore(*ed.FinalScore)) // 0,7×95,65 + 0,3×100 = 96,96
	assert.Equal(t, service.GradeIstimewa, ed.Grade)
}

func TestDirectorDashboard(t *testing.T) {
	d := newDash(t)
	w := d.w
	d.finalize(t, w.EmpIT1, 4)
	d.finalize(t, w.EmpIT2, 2)

	dd1, err := d.dash.Director(w.Actor(w.Director), w.Active.ID)
	require.NoError(t, err)
	assert.Equal(t, 2, dd1.Scored)
	assert.Equal(t, 5, dd1.Employees) // 2 IT + MgrIT + MgrCS + EmpCS1 = karyawan ber-departemen
	require.Len(t, dd1.Departments, 1)
	assert.Equal(t, "IT Support", dd1.Departments[0].Name)
	require.Len(t, dd1.Top, 2)
	assert.Equal(t, "Rina Karyawan", dd1.Top[0].Name)
	assert.Equal(t, "Fajar Karyawan", dd1.Bottom[0].Name)
	assert.Len(t, dd1.Grades, 5)
	require.NotNil(t, dd1.CompanyAvg)
	assert.Len(t, dd1.Trend, 2)

	for _, e := range []*model.Employee{w.EmpIT1, w.MgrIT} {
		_, err := d.dash.Director(w.Actor(e), w.Active.ID)
		assert.Equal(t, http.StatusForbidden, status(t, err), e.Name)
	}
}

func TestReportAccessAndExports(t *testing.T) {
	d := newDash(t)
	w := d.w
	d.finalize(t, w.EmpIT1, 4)

	for _, e := range []*model.Employee{w.EmpIT1, w.MgrIT} {
		_, err := d.report.Build(w.Actor(e), w.Active.ID, 0)
		assert.Equal(t, http.StatusForbidden, status(t, err), e.Name)
		_, _, err = d.report.ExportXLSX(w.Actor(e), w.Active.ID, 0)
		assert.Equal(t, http.StatusForbidden, status(t, err), e.Name)
		_, _, err = d.report.ExportPDF(w.Actor(e), w.Active.ID, 0)
		assert.Equal(t, http.StatusForbidden, status(t, err), e.Name)
	}
	for _, e := range []*model.Employee{w.HR, w.Director} {
		v, err := d.report.Build(w.Actor(e), w.Active.ID, 0)
		require.NoError(t, err)
		assert.Equal(t, 5, v.Summary.Total)
		assert.Equal(t, 1, v.Summary.Scored)
	}

	v, err := d.report.Build(w.Actor(w.HR), w.Active.ID, w.CS.ID)
	require.NoError(t, err)
	assert.Equal(t, 2, v.Summary.Total, "filter departemen")
	assert.Equal(t, "Customer Service", v.DepartmentName)

	b, name, err := d.report.ExportXLSX(w.Actor(w.HR), w.Active.ID, 0)
	require.NoError(t, err)
	assert.Contains(t, name, ".xlsx")
	f, err := excelize.OpenReader(bytes.NewReader(b))
	require.NoError(t, err)
	rows, err := f.GetRows("Laporan")
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(rows), 5)
	assert.Equal(t, "Skor Akhir", rows[3][7])
	found := false
	for _, r := range rows[4:] {
		if len(r) > 2 && r[2] == "Rina Karyawan" {
			found = true
			assert.Equal(t, w.MgrIT.Name, r[9], "kolom Penilai")
			assert.Equal(t, "Final", r[10])
		}
	}
	assert.True(t, found)

	pdf, name, err := d.report.ExportPDF(w.Actor(w.HR), w.Active.ID, 0)
	require.NoError(t, err)
	assert.Contains(t, name, ".pdf")
	assert.True(t, bytes.HasPrefix(pdf, []byte("%PDF-")), "bukan PDF")
	assert.Greater(t, len(pdf), 1000)
}

func TestPeriodProgress(t *testing.T) {
	d := newDash(t)
	w := d.w
	rs := d.fillAll(t, w.EmpIT1, w.EmpIT1, w.Active.ID)
	d.validateAll(t, w.MgrIT, rs[:3])
	d.finalize(t, w.EmpIT2, 4)

	ps := service.NewPeriodService(w.Store, nil)
	_, err := ps.Progress(w.Actor(w.MgrIT))
	assert.Equal(t, http.StatusForbidden, status(t, err))

	list, err := ps.Progress(w.Actor(w.HR))
	require.NoError(t, err)
	require.Len(t, list, 2)
	var act, closed service.PeriodProgress
	for _, p := range list {
		if p.PeriodID == w.Active.ID {
			act = p
		} else {
			closed = p
		}
	}
	assert.Equal(t, 5, act.Employees, "2 IT + MgrIT + MgrCS + EmpCS1")
	// IT: Bayu + Rina + Fajar → 3 × 5 indikator; CS: Citra + Dimas → 2 × 4 indikator
	assert.Equal(t, 3*5+2*4, act.ExpectedResults)
	assert.Equal(t, int64(10), act.Results, "5 milik Rina + 5 milik Fajar")
	assert.Equal(t, int64(8), act.Validated, "3 divalidasi untuk Rina + 5 untuk Fajar")
	assert.Equal(t, int64(1), act.ManagerDone)
	assert.Equal(t, int64(1), act.Finalized)
	require.NotNil(t, act.AvgFinal)
	assert.Zero(t, closed.Results)
	assert.Nil(t, closed.AvgFinal)
}

func TestReportShowsAppraiserOnlyAfterAppraisalIsSubmitted(t *testing.T) {
	d := newDash(t)
	w := d.w
	d.finalize(t, w.EmpIT1, 4)
	v, err := d.report.Build(w.Actor(w.HR), w.Active.ID, 0)
	require.NoError(t, err)
	by := map[uint64]service.ReportRow{}
	for _, r := range v.Rows {
		by[r.EmployeeID] = r
	}
	assert.Equal(t, w.MgrIT.Name, by[w.EmpIT1.ID].Appraiser)
	assert.Empty(t, by[w.EmpIT2.ID].Appraiser, "belum dinilai")
}
