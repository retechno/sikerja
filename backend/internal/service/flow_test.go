package service_test

import (
	"bytes"
	"net/http"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/retechno/sikerja/backend/internal/apperr"
	"github.com/retechno/sikerja/backend/internal/model"
	"github.com/retechno/sikerja/backend/internal/service"
	"github.com/retechno/sikerja/backend/internal/storage"
	"github.com/retechno/sikerja/backend/internal/testutil"
)

type svcs struct {
	w      *testutil.World
	kpi    *service.KpiService
	app    service.AppraisalService
	period *service.PeriodService
	tpl    *service.TemplateService
}

func newSvcs(t *testing.T) *svcs {
	w := testutil.NewWorld(t)
	files, err := storage.NewLocal(w.Cfg.StorageDir)
	require.NoError(t, err)
	cache := service.NewCache(w.Rdb)
	return &svcs{w: w,
		kpi:    service.NewKpiService(w.Store, files, cache),
		app:    service.NewAppraisalService(w.Store, cache),
		period: service.NewPeriodService(w.Store, cache),
		tpl:    service.NewTemplateService(w.Store, cache)}
}

func dd(s string) decimal.Decimal { return decimal.RequireFromString(s) }

func status(t *testing.T, err error) int {
	t.Helper()
	ae, ok := apperr.As(err)
	require.True(t, ok, "bukan apperr: %v", err)
	return ae.Status
}

// realisasi mockup H-05 untuk karyawan IT: 104, 96,2, 12, 3, 4,7
var mockupActuals = []string{"104", "96.2", "12", "3", "4.7"}

func (s *svcs) fillAll(t *testing.T, filler, target *model.Employee, periodID uint64) []*model.KpiResult {
	t.Helper()
	var out []*model.KpiResult
	for i, ind := range s.w.TplIT.Indicators {
		r, err := s.kpi.SaveResult(s.w.Actor(filler), service.SaveResultInput{
			PeriodID: periodID, EmployeeID: target.ID, IndicatorID: ind.ID, Actual: dd(mockupActuals[i])})
		require.NoError(t, err)
		out = append(out, r)
	}
	return out
}

func (s *svcs) validateAll(t *testing.T, mgr *model.Employee, rs []*model.KpiResult) {
	t.Helper()
	for _, r := range rs {
		_, err := s.kpi.Validate(s.w.Actor(mgr), r.ID, service.ActionValidate, "")
		require.NoError(t, err)
	}
}

func (s *svcs) managerScores(vals ...int) map[uint64]int {
	m := map[uint64]int{}
	for i, a := range s.w.Aspects {
		m[a.ID] = vals[i]
	}
	return m
}

// ---------- realisasi & validasi ----------

func TestEmployeeSubmitsManagerValidates(t *testing.T) {
	s := newSvcs(t)
	w := s.w
	ind := w.TplIT.Indicators[0]

	res, err := s.kpi.SaveResult(w.Actor(w.EmpIT1), service.SaveResultInput{
		PeriodID: w.Active.ID, EmployeeID: w.EmpIT1.ID, IndicatorID: ind.ID, Actual: dd("104"), Note: "ok"})
	require.NoError(t, err)
	assert.Equal(t, model.ResultSubmitted, res.Status)
	assert.True(t, dd("86.67").Equal(res.Achievement), res.Achievement.String())

	v, err := s.kpi.Validate(w.Actor(w.MgrIT), res.ID, service.ActionValidate, "")
	require.NoError(t, err)
	assert.Equal(t, model.ResultValidated, v.Status)

	var logs []model.ActivityLog
	require.NoError(t, w.DB.Where("action = ?", service.AuditKpiValidate).Find(&logs).Error)
	require.Len(t, logs, 1)
	assert.Equal(t, res.ID, logs[0].SubjectID)
	assert.Contains(t, logs[0].OldValues, "submitted")
	assert.Contains(t, logs[0].NewValues, "validated")
}

func TestRejectNeedsReasonAndExcludesFromScore(t *testing.T) {
	s := newSvcs(t)
	w := s.w
	rs := s.fillAll(t, w.EmpIT1, w.EmpIT1, w.Active.ID)

	_, err := s.kpi.Validate(w.Actor(w.MgrIT), rs[3].ID, service.ActionReject, "")
	assert.Equal(t, http.StatusUnprocessableEntity, status(t, err))
	_, err = s.kpi.Validate(w.Actor(w.MgrIT), rs[3].ID, service.ActionReject, "bukti tidak lengkap")
	require.NoError(t, err)

	sh, err := s.kpi.Sheet(w.Actor(w.EmpIT1), w.EmpIT1.ID, w.Active.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, sh.Rejected)
	assert.True(t, dd("80.65").Equal(sh.Score), "ditolak = 0 poin: %s", sh.Score)
}

func TestManagerEnteredResultIsAutoValidated(t *testing.T) {
	s := newSvcs(t)
	w := s.w
	r, err := s.kpi.SaveResult(w.Actor(w.MgrIT), service.SaveResultInput{
		PeriodID: w.Active.ID, EmployeeID: w.EmpIT1.ID, IndicatorID: w.TplIT.Indicators[1].ID, Actual: dd("90")})
	require.NoError(t, err)
	assert.Equal(t, model.ResultValidated, r.Status)
	require.NotNil(t, r.ValidatedBy)
}

func TestLowerDirectionZeroActualIs120(t *testing.T) {
	s := newSvcs(t)
	w := s.w
	r, err := s.kpi.SaveResult(w.Actor(w.EmpIT1), service.SaveResultInput{
		PeriodID: w.Active.ID, EmployeeID: w.EmpIT1.ID, IndicatorID: w.TplIT.Indicators[2].ID, Actual: dd("0")})
	require.NoError(t, err)
	assert.True(t, dd("120").Equal(r.Achievement))
}

func TestInvalidActualRejected(t *testing.T) {
	s := newSvcs(t)
	w := s.w
	_, err := s.kpi.SaveResult(w.Actor(w.EmpIT1), service.SaveResultInput{
		PeriodID: w.Active.ID, EmployeeID: w.EmpIT1.ID, IndicatorID: w.TplIT.Indicators[0].ID, Actual: dd("-1")})
	assert.Equal(t, http.StatusUnprocessableEntity, status(t, err))
}

func TestIndicatorOfAnotherDepartmentRejected(t *testing.T) {
	s := newSvcs(t)
	w := s.w
	_, err := s.kpi.SaveResult(w.Actor(w.EmpIT1), service.SaveResultInput{
		PeriodID: w.Active.ID, EmployeeID: w.EmpIT1.ID, IndicatorID: w.TplCS.Indicators[0].ID, Actual: dd("1")})
	assert.Equal(t, http.StatusNotFound, status(t, err))
}

// ---------- otorisasi ----------

func TestEmployeeCannotOpenOrChangeOthersData(t *testing.T) {
	s := newSvcs(t)
	w := s.w
	rs := s.fillAll(t, w.EmpIT2, w.EmpIT2, w.Active.ID)

	rina := w.Actor(w.EmpIT1)
	_, err := s.kpi.Sheet(rina, w.EmpIT2.ID, w.Active.ID)
	assert.Equal(t, http.StatusForbidden, status(t, err), "lihat KPI rekan")
	_, err = s.app.Get(rina, w.EmpIT2.ID, w.Active.ID)
	assert.Equal(t, http.StatusForbidden, status(t, err), "lihat penilaian rekan")
	_, err = s.kpi.SaveResult(rina, service.SaveResultInput{PeriodID: w.Active.ID, EmployeeID: w.EmpIT2.ID,
		IndicatorID: w.TplIT.Indicators[0].ID, Actual: dd("1")})
	assert.Equal(t, http.StatusForbidden, status(t, err), "mengisi realisasi rekan")
	_, err = s.kpi.Validate(rina, rs[0].ID, service.ActionValidate, "")
	assert.Equal(t, http.StatusForbidden, status(t, err), "memvalidasi")
	_, err = s.app.SaveManager(rina, w.EmpIT2.ID, w.Active.ID, service.ManagerInput{Scores: s.managerScores(5, 5, 5, 5, 5)})
	assert.Equal(t, http.StatusForbidden, status(t, err), "menilai rekan")
	_, _, err = s.kpi.OpenEvidence(rina, rs[0].ID)
	assert.Equal(t, http.StatusForbidden, status(t, err), "unduh bukti rekan")

	// karyawan tidak bisa menilai dirinya sendiri sebagai atasan
	_, err = s.app.SaveManager(rina, w.EmpIT1.ID, w.Active.ID, service.ManagerInput{Scores: s.managerScores(5, 5, 5, 5, 5)})
	assert.Equal(t, http.StatusForbidden, status(t, err))
}

func TestManagerCannotAccessOtherTeam(t *testing.T) {
	s := newSvcs(t)
	w := s.w
	rsCS, err := s.kpi.SaveResult(w.Actor(w.EmpCS1), service.SaveResultInput{PeriodID: w.Active.ID, EmployeeID: w.EmpCS1.ID,
		IndicatorID: w.TplCS.Indicators[0].ID, Actual: dd("4")})
	require.NoError(t, err)

	mgrIT := w.Actor(w.MgrIT)
	_, err = s.kpi.Sheet(mgrIT, w.EmpCS1.ID, w.Active.ID)
	assert.Equal(t, http.StatusForbidden, status(t, err))
	_, err = s.app.Get(mgrIT, w.EmpCS1.ID, w.Active.ID)
	assert.Equal(t, http.StatusForbidden, status(t, err))
	_, err = s.kpi.Validate(mgrIT, rsCS.ID, service.ActionValidate, "")
	assert.Equal(t, http.StatusForbidden, status(t, err))
	_, err = s.kpi.SaveResult(mgrIT, service.SaveResultInput{PeriodID: w.Active.ID, EmployeeID: w.EmpCS1.ID,
		IndicatorID: w.TplCS.Indicators[0].ID, Actual: dd("1")})
	assert.Equal(t, http.StatusForbidden, status(t, err))
	_, err = s.app.SaveManager(mgrIT, w.EmpCS1.ID, w.Active.ID, service.ManagerInput{Scores: s.managerScores(5, 5, 5, 5, 5)})
	assert.Equal(t, http.StatusForbidden, status(t, err))

	// ... tetapi tim sendiri boleh
	_, err = s.kpi.Sheet(mgrIT, w.EmpIT1.ID, w.Active.ID)
	assert.NoError(t, err)
	_, err = s.kpi.Sheet(w.Actor(w.MgrCS), w.EmpCS1.ID, w.Active.ID)
	assert.NoError(t, err)
}

func TestTeamListContainsOnlyOwnTeam(t *testing.T) {
	s := newSvcs(t)
	w := s.w
	tv, err := s.app.Team(w.Actor(w.MgrIT), w.Active.ID)
	require.NoError(t, err)
	var names []string
	for _, r := range tv.Rows {
		names = append(names, r.Employee.Name)
	}
	assert.ElementsMatch(t, []string{"Rina Karyawan", "Fajar Karyawan"}, names)

	_, err = s.app.Team(w.Actor(w.EmpIT1), w.Active.ID)
	assert.Equal(t, http.StatusForbidden, status(t, err))

	// Direktur melihat para manajer (atasan langsung); HR tanpa departemen tidak dinilai
	dv, err := s.app.Team(w.Actor(w.Director), w.Active.ID)
	require.NoError(t, err)
	names = nil
	for _, r := range dv.Rows {
		names = append(names, r.Employee.Name)
	}
	assert.ElementsMatch(t, []string{"Bayu Manajer", "Citra Manajer"}, names)
}

func TestHRAndDirectorCanViewButHRCannotValidate(t *testing.T) {
	s := newSvcs(t)
	w := s.w
	rs := s.fillAll(t, w.EmpIT1, w.EmpIT1, w.Active.ID)
	for _, e := range []*model.Employee{w.HR, w.Director} {
		_, err := s.kpi.Sheet(w.Actor(e), w.EmpIT1.ID, w.Active.ID)
		assert.NoError(t, err, e.Name)
	}
	_, err := s.kpi.Validate(w.Actor(w.HR), rs[0].ID, service.ActionValidate, "")
	assert.Equal(t, http.StatusForbidden, status(t, err))
}

func TestDirectorAppraisesManagerButManagerCannotAppraiseSelf(t *testing.T) {
	s := newSvcs(t)
	w := s.w
	_, err := s.app.SaveManager(w.Actor(w.MgrIT), w.MgrIT.ID, w.Active.ID, service.ManagerInput{Scores: s.managerScores(5, 5, 5, 5, 5)})
	assert.Equal(t, http.StatusForbidden, status(t, err))
	_, err = s.app.SaveManager(w.Actor(w.Director), w.MgrIT.ID, w.Active.ID, service.ManagerInput{Scores: s.managerScores(4, 4, 4, 4, 4)})
	assert.NoError(t, err)
}

// ---------- periode ----------

func TestClosedPeriodIsLocked(t *testing.T) {
	s := newSvcs(t)
	w := s.w
	rs := s.fillAll(t, w.EmpIT1, w.EmpIT1, w.Active.ID)
	_, err := s.app.SaveSelf(w.Actor(w.EmpIT1), w.EmpIT1.ID, w.Active.ID, service.SelfInput{Comment: "x"})
	require.NoError(t, err)

	_, err = s.period.Close(w.Actor(w.HR), w.Active.ID)
	require.NoError(t, err)

	in := service.SaveResultInput{PeriodID: w.Active.ID, EmployeeID: w.EmpIT1.ID, IndicatorID: w.TplIT.Indicators[0].ID, Actual: dd("1")}
	_, err = s.kpi.SaveResult(w.Actor(w.EmpIT1), in)
	assert.Equal(t, http.StatusLocked, status(t, err), "karyawan input realisasi")
	_, err = s.kpi.SaveResult(w.Actor(w.MgrIT), in)
	assert.Equal(t, http.StatusLocked, status(t, err), "manajer input realisasi")
	_, err = s.kpi.Validate(w.Actor(w.MgrIT), rs[0].ID, service.ActionReject, "x")
	assert.Equal(t, http.StatusLocked, status(t, err), "validasi")
	_, err = s.kpi.SaveEvidence(w.Actor(w.EmpIT1), rs[0].ID, "a.png", bytes.NewReader(pngBytes()))
	assert.Equal(t, http.StatusLocked, status(t, err), "bukti")
	_, err = s.app.SaveSelf(w.Actor(w.EmpIT1), w.EmpIT1.ID, w.Active.ID, service.SelfInput{Comment: "y"})
	assert.Equal(t, http.StatusLocked, status(t, err), "self-assessment")
	_, err = s.app.SaveManager(w.Actor(w.MgrIT), w.EmpIT1.ID, w.Active.ID, service.ManagerInput{Scores: s.managerScores(3, 3, 3, 3, 3)})
	assert.Equal(t, http.StatusLocked, status(t, err), "penilaian atasan")
	_, err = s.app.Finalize(w.Actor(w.HR), w.EmpIT1.ID, w.Active.ID)
	assert.Equal(t, http.StatusLocked, status(t, err), "finalisasi")
	_, err = s.period.Update(w.Actor(w.HR), w.Active.ID, service.PeriodInput{Name: "x", Type: "monthly",
		StartDate: w.Active.StartDate, EndDate: w.Active.EndDate, KpiWeight: dd("60"), BehaviorWeight: dd("40")})
	assert.Equal(t, http.StatusLocked, status(t, err), "ubah pengaturan periode")

	// periode Ditutup yang sudah ada sejak awal (September) juga terkunci
	in.PeriodID = w.Closed.ID
	_, err = s.kpi.SaveResult(w.Actor(w.EmpIT1), in)
	assert.Equal(t, http.StatusLocked, status(t, err))
}

func TestOnlyHRCanReopenAndItIsAudited(t *testing.T) {
	s := newSvcs(t)
	w := s.w
	_, err := s.period.Reopen(w.Actor(w.MgrIT), w.Closed.ID)
	assert.Equal(t, http.StatusForbidden, status(t, err))
	_, err = s.period.Reopen(w.Actor(w.Director), w.Closed.ID)
	assert.Equal(t, http.StatusForbidden, status(t, err))
	_, err = s.period.Close(w.Actor(w.MgrIT), w.Active.ID)
	assert.Equal(t, http.StatusForbidden, status(t, err))

	p, err := s.period.Reopen(w.Actor(w.HR), w.Closed.ID)
	require.NoError(t, err)
	assert.Equal(t, model.PeriodActive, p.Status)

	// setelah dibuka kembali, data bisa diubah lagi
	_, err = s.kpi.SaveResult(w.Actor(w.EmpIT1), service.SaveResultInput{PeriodID: w.Closed.ID, EmployeeID: w.EmpIT1.ID,
		IndicatorID: w.TplIT.Indicators[0].ID, Actual: dd("5")})
	assert.NoError(t, err)

	var logs []model.ActivityLog
	require.NoError(t, w.DB.Where("action = ?", service.AuditPeriodReopen).Find(&logs).Error)
	require.Len(t, logs, 1)
	assert.Contains(t, logs[0].OldValues, "closed")
	assert.Contains(t, logs[0].NewValues, "active")
	require.NotNil(t, logs[0].UserID)
	assert.Equal(t, w.HR.UserID, *logs[0].UserID)
}

func TestPeriodTransitionsAndValidation(t *testing.T) {
	s := newSvcs(t)
	w := s.w
	hr := w.Actor(w.HR)
	in := service.PeriodInput{Name: "November 2026", Type: "monthly", StartDate: w.Active.EndDate.AddDate(0, 0, 1),
		EndDate: w.Active.EndDate.AddDate(0, 0, 30), KpiWeight: dd("70"), BehaviorWeight: dd("30")}

	bad := in
	bad.BehaviorWeight = dd("20")
	_, err := s.period.Create(hr, bad)
	assert.Equal(t, http.StatusUnprocessableEntity, status(t, err))
	_, err = s.period.Create(w.Actor(w.MgrIT), in)
	assert.Equal(t, http.StatusForbidden, status(t, err))

	p, err := s.period.Create(hr, in)
	require.NoError(t, err)
	assert.Equal(t, model.PeriodDraft, p.Status)

	// draft: belum bisa diisi dan tidak tampil untuk non-HR
	_, err = s.kpi.SaveResult(w.Actor(w.EmpIT1), service.SaveResultInput{PeriodID: p.ID, EmployeeID: w.EmpIT1.ID,
		IndicatorID: w.TplIT.Indicators[0].ID, Actual: dd("1")})
	assert.Equal(t, http.StatusBadRequest, status(t, err))
	list, err := s.period.List(w.Actor(w.EmpIT1))
	require.NoError(t, err)
	for _, x := range list {
		assert.NotEqual(t, p.ID, x.ID)
	}

	_, err = s.period.Close(hr, p.ID)
	assert.Equal(t, http.StatusConflict, status(t, err), "draft tidak bisa langsung ditutup")
	_, err = s.period.Open(hr, p.ID)
	require.NoError(t, err)
	_, err = s.period.Open(hr, p.ID)
	assert.Equal(t, http.StatusConflict, status(t, err))
}

// ---------- bukti ----------

func pngBytes() []byte {
	return []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01\x08\x06\x00\x00\x00\x1f\x15\xc4\x89\x00\x00\x00\rIDATx\x9cc\xf8\xff\xff?\x00\x05\xfe\x02\xfe\xa7\x9a\x9c\x88\x00\x00\x00\x00IEND\xaeB`\x82")
}

func TestEvidenceUploadAndAccessControl(t *testing.T) {
	s := newSvcs(t)
	w := s.w
	rs := s.fillAll(t, w.EmpIT1, w.EmpIT1, w.Active.ID)
	rina, bayu := w.Actor(w.EmpIT1), w.Actor(w.MgrIT)

	_, err := s.kpi.SaveEvidence(rina, rs[0].ID, "../../etc/passwd.png", bytes.NewReader(pngBytes()))
	require.NoError(t, err)

	f, name, err := s.kpi.OpenEvidence(rina, rs[0].ID)
	require.NoError(t, err)
	f.Close()
	assert.Equal(t, "passwd.png", name, "nama berkas dibersihkan dari path")

	for _, a := range []service.Actor{bayu, w.Actor(w.HR), w.Actor(w.Director)} {
		f, _, err := s.kpi.OpenEvidence(a, rs[0].ID)
		require.NoError(t, err)
		f.Close()
	}
	for _, e := range []*model.Employee{w.EmpIT2, w.MgrCS, w.EmpCS1} {
		_, _, err := s.kpi.OpenEvidence(w.Actor(e), rs[0].ID)
		assert.Equal(t, http.StatusForbidden, status(t, err), e.Name)
	}
	// karyawan lain tidak boleh mengunggah ke realisasi rekan
	_, err = s.kpi.SaveEvidence(w.Actor(w.EmpIT2), rs[1].ID, "a.png", bytes.NewReader(pngBytes()))
	assert.Equal(t, http.StatusForbidden, status(t, err))
}

func TestEvidenceTypeAndSizeChecks(t *testing.T) {
	s := newSvcs(t)
	w := s.w
	rs := s.fillAll(t, w.EmpIT1, w.EmpIT1, w.Active.ID)
	rina := w.Actor(w.EmpIT1)

	_, err := s.kpi.SaveEvidence(rina, rs[0].ID, "x.pdf", bytes.NewReader([]byte("<html><script>alert(1)</script></html>")))
	assert.Equal(t, http.StatusUnprocessableEntity, status(t, err), "isi HTML menyamar jadi pdf")
	_, err = s.kpi.SaveEvidence(rina, rs[0].ID, "x.png", bytes.NewReader(nil))
	assert.Equal(t, http.StatusUnprocessableEntity, status(t, err), "kosong")
	big := append([]byte("%PDF-1.4\n"), make([]byte, service.MaxEvidenceBytes)...)
	_, err = s.kpi.SaveEvidence(rina, rs[0].ID, "big.pdf", bytes.NewReader(big))
	assert.Equal(t, http.StatusUnprocessableEntity, status(t, err), "> 5 MB")
	_, err = s.kpi.SaveEvidence(rina, rs[0].ID, "ok.pdf", bytes.NewReader([]byte("%PDF-1.4\n%test")))
	assert.NoError(t, err)
}

func TestReplacingEvidenceOfValidatedResultNeedsRevalidation(t *testing.T) {
	s := newSvcs(t)
	w := s.w
	rs := s.fillAll(t, w.EmpIT1, w.EmpIT1, w.Active.ID)
	s.validateAll(t, w.MgrIT, rs)
	r, err := s.kpi.SaveEvidence(w.Actor(w.EmpIT1), rs[0].ID, "b.png", bytes.NewReader(pngBytes()))
	require.NoError(t, err)
	assert.Equal(t, model.ResultSubmitted, r.Status)
}

// ---------- template ----------

func TestTemplateWeightMustBe100AndSoftDeletesIndicators(t *testing.T) {
	s := newSvcs(t)
	w := s.w
	hr := w.Actor(w.HR)

	toIn := func(inds []model.KpiIndicator) service.TemplateIn {
		in := service.TemplateIn{Name: w.TplIT.Name, IsActive: true}
		for _, i := range inds {
			in.Indicators = append(in.Indicators, service.IndicatorIn{ID: i.ID, Name: i.Name, Unit: i.Unit,
				Target: i.Target, Weight: i.Weight, Direction: i.Direction, Source: i.Source})
		}
		return in
	}
	in := toIn(w.TplIT.Indicators)
	in.Indicators[0].Weight = dd("30") // total 105
	_, err := s.tpl.Save(hr, w.TplIT.ID, in)
	require.Error(t, err)
	assert.Equal(t, http.StatusUnprocessableEntity, status(t, err))
	ae, _ := apperr.As(err)
	assert.Contains(t, ae.Fields["indicators"], "100%")

	_, err = s.tpl.Save(w.Actor(w.MgrIT), w.TplIT.ID, toIn(w.TplIT.Indicators))
	assert.Equal(t, http.StatusForbidden, status(t, err))

	// hapus indikator terakhir (15%) dan bagi ulang bobotnya → total 100, indikator lama di-soft-delete
	in = toIn(w.TplIT.Indicators[:4])
	in.Indicators[0].Weight = dd("40")
	got, err := s.tpl.Save(hr, w.TplIT.ID, in)
	require.NoError(t, err)
	assert.Len(t, got.Indicators, 4)
	assert.True(t, dd("100").Equal(got.TotalWeight), got.TotalWeight.String())
	var n int64
	w.DB.Unscoped().Model(&model.KpiIndicator{}).Where("kpi_template_id = ?", w.TplIT.ID).Count(&n)
	assert.EqualValues(t, 5, n, "indikator yang dihapus tetap ada (soft delete)")

	_, err = s.tpl.List(w.Actor(w.EmpIT1))
	assert.Equal(t, http.StatusForbidden, status(t, err))
}

// ---------- alur penilaian ----------

func TestAppraisalFlowMockupH07(t *testing.T) {
	s := newSvcs(t)
	w := s.w
	rina, bayu, hr := w.Actor(w.EmpIT1), w.Actor(w.MgrIT), w.Actor(w.HR)
	rs := s.fillAll(t, w.EmpIT1, w.EmpIT1, w.Active.ID)
	s.validateAll(t, w.MgrIT, rs[:4]) // 4 dari 5 tervalidasi seperti mockup H-05

	// self-assessment
	self := map[uint64]int{}
	for i, a := range w.Aspects {
		self[a.ID] = []int{5, 4, 4, 4, 5}[i]
	}
	v, err := s.app.SaveSelf(rina, w.EmpIT1.ID, w.Active.ID, service.SelfInput{Scores: self, Comment: "Migrasi cepat", Submit: true})
	require.NoError(t, err)
	assert.Equal(t, model.AppraisalSelfDone, v.Status)

	// atasan: 4,5,4,4,5 → rata-rata 4,4 → perilaku 88,0; KPI 95,65 → akhir 93,36
	v, err = s.app.SaveManager(bayu, w.EmpIT1.ID, w.Active.ID, service.ManagerInput{
		Scores: s.managerScores(4, 5, 4, 4, 5), ManagerComment: "Bagus", DevelopmentPlan: "Sertifikasi", Submit: true})
	require.NoError(t, err)
	assert.Equal(t, model.AppraisalManagerDone, v.Status)
	require.NotNil(t, v.FinalScore)
	assert.Equal(t, "95,7", service.FormatScore(v.KpiScore))
	assert.Equal(t, "88,0", service.FormatScore(*v.BehaviorScore))
	assert.Equal(t, "93,4", service.FormatScore(*v.FinalScore))
	assert.Equal(t, service.GradeIstimewa, v.Grade)

	// karyawan belum boleh melihat nilai atasan sebelum final
	mine, err := s.app.Get(rina, w.EmpIT1.ID, w.Active.ID)
	require.NoError(t, err)
	assert.True(t, mine.HiddenFromEmployee)
	assert.Nil(t, mine.FinalScore)
	assert.Empty(t, mine.ManagerComment)
	assert.Empty(t, mine.Grade)
	for _, a := range mine.Aspects {
		assert.Nil(t, a.ManagerScore)
		assert.NotNil(t, a.SelfScore)
	}
	// ...dan self-assessment terkunci setelah atasan mengirim
	_, err = s.app.SaveSelf(rina, w.EmpIT1.ID, w.Active.ID, service.SelfInput{Comment: "ubah"})
	assert.Equal(t, http.StatusLocked, status(t, err))

	// finalisasi: manajer tidak boleh; HR boleh tapi hanya bila semua KPI tervalidasi
	_, err = s.app.Finalize(bayu, w.EmpIT1.ID, w.Active.ID)
	assert.Equal(t, http.StatusForbidden, status(t, err))
	_, err = s.app.Finalize(hr, w.EmpIT1.ID, w.Active.ID)
	assert.Equal(t, http.StatusConflict, status(t, err), "masih ada KPI belum valid")

	s.validateAll(t, w.MgrIT, rs[4:])
	fin, err := s.app.Finalize(hr, w.EmpIT1.ID, w.Active.ID)
	require.NoError(t, err)
	assert.Equal(t, model.AppraisalFinal, fin.Status)
	assert.Equal(t, "93,4", service.FormatScore(*fin.FinalScore))

	mine, err = s.app.Get(rina, w.EmpIT1.ID, w.Active.ID)
	require.NoError(t, err)
	assert.False(t, mine.HiddenFromEmployee)
	assert.Equal(t, "93,4", service.FormatScore(*mine.FinalScore))
	assert.Equal(t, "Bagus", mine.ManagerComment)

	// setelah final semua terkunci walau periode masih aktif
	_, err = s.app.SaveManager(bayu, w.EmpIT1.ID, w.Active.ID, service.ManagerInput{Scores: s.managerScores(1, 1, 1, 1, 1)})
	assert.Equal(t, http.StatusLocked, status(t, err))
	_, err = s.kpi.SaveResult(bayu, service.SaveResultInput{PeriodID: w.Active.ID, EmployeeID: w.EmpIT1.ID,
		IndicatorID: w.TplIT.Indicators[0].ID, Actual: dd("999")})
	assert.Equal(t, http.StatusLocked, status(t, err))
	_, err = s.app.Finalize(hr, w.EmpIT1.ID, w.Active.ID)
	assert.Equal(t, http.StatusConflict, status(t, err))

	var logs []model.ActivityLog
	w.DB.Where("action IN ?", []string{service.AuditFinalize, service.AuditScoreChange}).Find(&logs)
	actions := map[string]int{}
	for _, l := range logs {
		actions[l.Action]++
	}
	assert.Equal(t, 1, actions[service.AuditFinalize])
	assert.GreaterOrEqual(t, actions[service.AuditScoreChange], 2)
}

func TestManagerSubmitNeedsAllAspects(t *testing.T) {
	s := newSvcs(t)
	w := s.w
	bayu := w.Actor(w.MgrIT)
	part := map[uint64]int{w.Aspects[0].ID: 4, w.Aspects[1].ID: 4}
	_, err := s.app.SaveManager(bayu, w.EmpIT1.ID, w.Active.ID, service.ManagerInput{Scores: part, Submit: true})
	assert.Equal(t, http.StatusUnprocessableEntity, status(t, err))

	// draf boleh sebagian, status tetap draft
	v, err := s.app.SaveManager(bayu, w.EmpIT1.ID, w.Active.ID, service.ManagerInput{Scores: part, ManagerComment: "draf"})
	require.NoError(t, err)
	assert.Equal(t, model.AppraisalDraft, v.Status)
	assert.Nil(t, v.FinalScore)
}

func TestScoreValidation(t *testing.T) {
	s := newSvcs(t)
	w := s.w
	bayu := w.Actor(w.MgrIT)
	for _, bad := range []int{0, 6, -1} {
		_, err := s.app.SaveManager(bayu, w.EmpIT1.ID, w.Active.ID, service.ManagerInput{Scores: map[uint64]int{w.Aspects[0].ID: bad}})
		assert.Equal(t, http.StatusUnprocessableEntity, status(t, err), "nilai %d", bad)
	}
	_, err := s.app.SaveManager(bayu, w.EmpIT1.ID, w.Active.ID, service.ManagerInput{Scores: map[uint64]int{9999: 3}})
	assert.Equal(t, http.StatusUnprocessableEntity, status(t, err))
}

func TestSelfAssessmentDoesNotAffectScore(t *testing.T) {
	s := newSvcs(t)
	w := s.w
	rs := s.fillAll(t, w.EmpIT1, w.EmpIT1, w.Active.ID)
	s.validateAll(t, w.MgrIT, rs)
	high := map[uint64]int{}
	for _, a := range w.Aspects {
		high[a.ID] = 5
	}
	_, err := s.app.SaveSelf(w.Actor(w.EmpIT1), w.EmpIT1.ID, w.Active.ID, service.SelfInput{Scores: high, Submit: true})
	require.NoError(t, err)
	v, err := s.app.SaveManager(w.Actor(w.MgrIT), w.EmpIT1.ID, w.Active.ID, service.ManagerInput{Scores: s.managerScores(2, 2, 2, 2, 2), Submit: true})
	require.NoError(t, err)
	assert.Equal(t, "40,0", service.FormatScore(*v.BehaviorScore), "hanya nilai atasan yang dihitung")
}

func TestOnlyOwnerCanSaveSelfAssessment(t *testing.T) {
	s := newSvcs(t)
	w := s.w
	_, err := s.app.SaveSelf(w.Actor(w.MgrIT), w.EmpIT1.ID, w.Active.ID, service.SelfInput{Comment: "x"})
	assert.Equal(t, http.StatusForbidden, status(t, err), "atasan tidak boleh mengisi self-assessment bawahan")
}
