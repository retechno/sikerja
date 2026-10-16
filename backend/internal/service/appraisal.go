package service

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/shopspring/decimal"

	"github.com/retechno/sikerja/backend/internal/apperr"
	"github.com/retechno/sikerja/backend/internal/model"
	"github.com/retechno/sikerja/backend/internal/repository"
)

// AppraisalService: rumus penilaian (appraisal_calc.go) dan alur penilaian (file ini).
// Nilai nol (AppraisalService{}) cukup untuk memakai rumus tanpa DB.
type AppraisalService struct {
	store repository.Store
	cache *Cache
}

func NewAppraisalService(store repository.Store, cache *Cache) AppraisalService {
	return AppraisalService{store: store, cache: cache}
}

const maxCommentLen = 5000

// ---------- tampilan ----------

type AspectRow struct {
	Aspect       model.BehaviorAspect `json:"aspect"`
	SelfScore    *int                 `json:"self_score"`
	ManagerScore *int                 `json:"manager_score"`
}

type PreviousScore struct {
	PeriodName string          `json:"period_name"`
	FinalScore decimal.Decimal `json:"final_score"`
	Grade      string          `json:"grade"`
}

type AppraisalView struct {
	AppraisalID     uint64                 `json:"appraisal_id"`
	Status          string                 `json:"status"`
	Employee        *model.Employee        `json:"employee"`
	Period          *model.AppraisalPeriod `json:"period"`
	Aspects         []AspectRow            `json:"aspects"`
	EmployeeComment string                 `json:"employee_comment"`
	ManagerComment  string                 `json:"manager_comment"`
	DevelopmentPlan string                 `json:"development_plan"`
	Kpi             *KpiSheet              `json:"kpi"`
	KpiScore        decimal.Decimal        `json:"kpi_score"`
	BehaviorScore   *decimal.Decimal       `json:"behavior_score"`
	FinalScore      *decimal.Decimal       `json:"final_score"`
	Grade           string                 `json:"grade"`
	Previous        *PreviousScore         `json:"previous"`
	SelfSubmittedAt *time.Time             `json:"self_submitted_at"`
	MgrSubmittedAt  *time.Time             `json:"manager_submitted_at"`
	FinalizedAt     *time.Time             `json:"finalized_at"`
	CanSelf         bool                   `json:"can_self"`
	CanManager      bool                   `json:"can_manager"`
	CanFinalize     bool                   `json:"can_finalize"`
	// HiddenFromEmployee true bila nilai atasan disembunyikan karena penilaian belum final.
	HiddenFromEmployee bool `json:"hidden_from_employee"`
}

func (s AppraisalService) Get(a Actor, employeeID, periodID uint64) (*AppraisalView, error) {
	r := s.store.R()
	emp, err := loadEmployee(r, employeeID)
	if err != nil {
		return nil, err
	}
	if err := requireView(a, emp); err != nil {
		return nil, err
	}
	p, err := loadPeriod(r, periodID)
	if err != nil {
		return nil, err
	}
	if p.Status == model.PeriodDraft && !a.IsHR() {
		return nil, apperr.NotFound("Periode tidak ditemukan")
	}
	if emp.DepartmentID == nil {
		return nil, apperr.BadRequest("Karyawan ini tidak dinilai (tanpa departemen)")
	}
	return s.view(r, a, emp, p)
}

func (s AppraisalService) view(r repository.Repos, a Actor, emp *model.Employee, p *model.AppraisalPeriod) (*AppraisalView, error) {
	aspects, err := r.Aspects.List()
	if err != nil {
		return nil, err
	}
	appr, err := r.Appraisals.Find(p.ID, emp.ID)
	if err != nil && err != repository.ErrNotFound {
		return nil, err
	}
	if appr == nil {
		appr = &model.Appraisal{PeriodID: p.ID, EmployeeID: emp.ID, Status: model.AppraisalDraft}
	}
	sheet, err := loadSheet(r, KpiScoreService{}, emp, p)
	if err != nil {
		return nil, err
	}

	v := &AppraisalView{AppraisalID: appr.ID, Status: appr.Status, Employee: emp, Period: p, Kpi: sheet,
		EmployeeComment: appr.EmployeeComment, ManagerComment: appr.ManagerComment, DevelopmentPlan: appr.DevelopmentPlan,
		SelfSubmittedAt: appr.SelfSubmittedAt, MgrSubmittedAt: appr.MgrSubmittedAt, FinalizedAt: appr.FinalizedAt}

	byAspect := map[uint64]model.AppraisalScore{}
	for _, sc := range appr.Scores {
		byAspect[sc.AspectID] = sc
	}
	var mgr []int
	for _, as := range aspects {
		sc := byAspect[as.ID]
		v.Aspects = append(v.Aspects, AspectRow{Aspect: as, SelfScore: sc.SelfScore, ManagerScore: sc.ManagerScore})
		if sc.ManagerScore != nil {
			mgr = append(mgr, *sc.ManagerScore)
		}
	}

	v.KpiScore = sheet.Score
	final := appr.Status == model.AppraisalFinal
	if final && appr.KpiScore.Valid { // skor yang dibekukan saat finalisasi
		v.KpiScore = appr.KpiScore.Decimal
	}
	if len(aspects) > 0 && len(mgr) == len(aspects) {
		if b, err := s.BehaviorScore(mgr); err == nil {
			v.BehaviorScore = &b
			if f, err := s.FinalScore(v.KpiScore, b, p.KpiWeight, p.BehaviorWeight); err == nil {
				v.FinalScore = &f
				v.Grade = s.Grade(f)
			}
		}
	}
	if final && appr.FinalScore.Valid {
		b, f := appr.BehaviorScore.Decimal, appr.FinalScore.Decimal
		v.BehaviorScore, v.FinalScore, v.Grade = &b, &f, appr.Grade
	}

	open := p.Status == model.PeriodActive && !final
	v.CanSelf = open && emp.ID == a.EmployeeID && (appr.Status == model.AppraisalDraft || appr.Status == model.AppraisalSelfDone)
	v.CanManager = open && a.CanAppraise(emp)
	v.CanFinalize = open && a.CanFinalize() && appr.Status == model.AppraisalManagerDone
	v.Previous = s.previous(r, emp.ID, p)

	// Karyawan baru boleh melihat nilai atasan setelah final.
	if emp.ID == a.EmployeeID && !final {
		v.HiddenFromEmployee = true
		v.ManagerComment, v.DevelopmentPlan = "", ""
		v.BehaviorScore, v.FinalScore, v.Grade = nil, nil, ""
		for i := range v.Aspects {
			v.Aspects[i].ManagerScore = nil
		}
	}
	return v, nil
}

// previous = skor akhir final pada periode sebelumnya (maks. 6 periode ke belakang).
func (s AppraisalService) previous(r repository.Repos, empID uint64, cur *model.AppraisalPeriod) *PreviousScore {
	periods, err := r.Periods.List() // urut start_date turun
	if err != nil {
		return nil
	}
	checked := 0
	for _, p := range periods {
		if !p.StartDate.Before(cur.StartDate) {
			continue
		}
		if checked++; checked > 6 {
			break
		}
		if ap, err := r.Appraisals.Find(p.ID, empID); err == nil && ap.Status == model.AppraisalFinal && ap.FinalScore.Valid {
			return &PreviousScore{PeriodName: p.Name, FinalScore: ap.FinalScore.Decimal, Grade: ap.Grade}
		}
	}
	return nil
}

// ---------- menyimpan ----------

type SelfInput struct {
	Scores  map[uint64]int `json:"scores"` // aspect_id → 1..5
	Comment string         `json:"comment"`
	Submit  bool           `json:"submit"`
}

type ManagerInput struct {
	Scores          map[uint64]int `json:"scores"`
	ManagerComment  string         `json:"manager_comment"`
	DevelopmentPlan string         `json:"development_plan"`
	Submit          bool           `json:"submit"`
}

func validateScores(scores map[uint64]int, aspects []model.BehaviorAspect) error {
	known := map[uint64]bool{}
	for _, a := range aspects {
		known[a.ID] = true
	}
	for id, v := range scores {
		if !known[id] {
			return apperr.Validation("Aspek tidak dikenal", map[string]string{"scores": "Aspek tidak valid"})
		}
		if v < 1 || v > 5 {
			return apperr.Validation("Nilai harus 1 sampai 5", map[string]string{"scores": "Nilai harus 1 sampai 5"})
		}
	}
	return nil
}

func validateText(fields map[string]string) error {
	f := map[string]string{}
	for k, v := range fields {
		if utf8.RuneCountInString(v) > maxCommentLen {
			f[k] = "Maksimal 5000 karakter"
		}
	}
	if len(f) > 0 {
		return apperr.Validation("Teks terlalu panjang", f)
	}
	return nil
}

// ensureAppraisal mengambil atau membuat baris penilaian (unik per periode+karyawan).
func ensureAppraisal(r repository.Repos, empID, periodID uint64) (*model.Appraisal, error) {
	ap, err := r.Appraisals.Find(periodID, empID)
	if err == nil {
		return ap, nil
	}
	if err != repository.ErrNotFound {
		return nil, err
	}
	ap = &model.Appraisal{PeriodID: periodID, EmployeeID: empID, Status: model.AppraisalDraft}
	if err := r.Appraisals.Create(ap); err != nil {
		return nil, err
	}
	return ap, nil
}

func scoreMap(ap *model.Appraisal, manager bool) map[uint64]*int {
	out := map[uint64]*int{}
	for _, sc := range ap.Scores {
		if manager {
			out[sc.AspectID] = sc.ManagerScore
		} else {
			out[sc.AspectID] = sc.SelfScore
		}
	}
	return out
}

func derefMap(m map[uint64]*int) map[uint64]int {
	out := map[uint64]int{}
	for k, v := range m {
		if v != nil {
			out[k] = *v
		}
	}
	return out
}

// SaveSelf: karyawan mengisi self-assessment (draf atau kirim). Tidak masuk perhitungan skor.
func (s AppraisalService) SaveSelf(a Actor, employeeID, periodID uint64, in SelfInput) (*AppraisalView, error) {
	if employeeID != a.EmployeeID {
		return nil, errForbiddenData
	}
	if err := validateText(map[string]string{"comment": in.Comment}); err != nil {
		return nil, err
	}
	var view *AppraisalView
	err := s.store.Tx(func(r repository.Repos) error {
		emp, err := loadEmployee(r, employeeID)
		if err != nil {
			return err
		}
		if emp.DepartmentID == nil {
			return apperr.BadRequest("Karyawan ini tidak dinilai (tanpa departemen)")
		}
		p, err := loadPeriod(r, periodID)
		if err != nil {
			return err
		}
		if err := requireOpen(p); err != nil {
			return err
		}
		aspects, err := r.Aspects.List()
		if err != nil {
			return err
		}
		if err := validateScores(in.Scores, aspects); err != nil {
			return err
		}
		ap, err := ensureAppraisal(r, emp.ID, p.ID)
		if err != nil {
			return err
		}
		if ap.Status != model.AppraisalDraft && ap.Status != model.AppraisalSelfDone {
			return apperr.Locked("Penilaian sudah dinilai atasan; self-assessment tidak dapat diubah")
		}
		before := derefMap(scoreMap(ap, false))
		for id, v := range in.Scores {
			v := v
			sc := model.AppraisalScore{AppraisalID: ap.ID, AspectID: id, SelfScore: &v}
			if err := r.Appraisals.UpsertScore(&sc, "self_score"); err != nil {
				return err
			}
		}
		ap.EmployeeComment = strings.TrimSpace(in.Comment)
		if in.Submit {
			fresh, err := r.Appraisals.ByID(ap.ID)
			if err != nil {
				return err
			}
			have := derefMap(scoreMap(fresh, false))
			if len(have) < len(aspects) {
				return apperr.Validation("Semua aspek perilaku harus dinilai", map[string]string{"scores": "Lengkapi semua aspek"})
			}
			now := time.Now()
			ap.Status, ap.SelfSubmittedAt = model.AppraisalSelfDone, &now
		}
		if err := r.Appraisals.Save(ap); err != nil {
			return err
		}
		if err := logActivity(r, a, AuditScoreChange, "appraisal", ap.ID,
			map[string]any{"kind": "self", "scores": before}, map[string]any{"kind": "self", "scores": in.Scores, "submit": in.Submit}); err != nil {
			return err
		}
		view, err = s.view(r, a, emp, p)
		return err
	})
	if err == nil {
		s.cache.Bump(bgCtx())
	}
	return view, err
}

// compute menghitung skor KPI, perilaku, akhir, dan predikat dari data saat ini.
func (s AppraisalService) compute(r repository.Repos, emp *model.Employee, p *model.AppraisalPeriod, ap *model.Appraisal) (kpi, beh, final decimal.Decimal, grade string, allValidated bool, err error) {
	sheet, err := loadSheet(r, KpiScoreService{}, emp, p)
	if err != nil {
		return
	}
	aspects, err := r.Aspects.List()
	if err != nil {
		return
	}
	mgr := derefMap(scoreMap(ap, true))
	var list []int
	for _, as := range aspects {
		v, ok := mgr[as.ID]
		if !ok {
			err = apperr.Validation("Semua aspek perilaku harus dinilai", map[string]string{"scores": "Lengkapi semua aspek"})
			return
		}
		list = append(list, v)
	}
	beh, err = s.BehaviorScore(list)
	if err != nil {
		err = apperr.Validation("Nilai perilaku belum lengkap", map[string]string{"scores": err.Error()})
		return
	}
	kpi = sheet.Score
	final, err = s.FinalScore(kpi, beh, p.KpiWeight, p.BehaviorWeight)
	if err != nil {
		return
	}
	return kpi, beh, final, s.Grade(final), sheet.AllValidated, nil
}

// SaveManager: atasan menilai aspek perilaku + komentar + rencana pengembangan (draf atau kirim).
func (s AppraisalService) SaveManager(a Actor, employeeID, periodID uint64, in ManagerInput) (*AppraisalView, error) {
	if err := validateText(map[string]string{"manager_comment": in.ManagerComment, "development_plan": in.DevelopmentPlan}); err != nil {
		return nil, err
	}
	var view *AppraisalView
	err := s.store.Tx(func(r repository.Repos) error {
		emp, err := loadEmployee(r, employeeID)
		if err != nil {
			return err
		}
		if err := requireAppraise(a, emp); err != nil {
			return err
		}
		p, err := loadPeriod(r, periodID)
		if err != nil {
			return err
		}
		if err := requireOpen(p); err != nil {
			return err
		}
		aspects, err := r.Aspects.List()
		if err != nil {
			return err
		}
		if err := validateScores(in.Scores, aspects); err != nil {
			return err
		}
		ap, err := ensureAppraisal(r, emp.ID, p.ID)
		if err != nil {
			return err
		}
		if ap.Status == model.AppraisalFinal {
			return apperr.Locked("Penilaian sudah final dan tidak dapat diubah")
		}
		before := derefMap(scoreMap(ap, true))
		for id, v := range in.Scores {
			v := v
			sc := model.AppraisalScore{AppraisalID: ap.ID, AspectID: id, ManagerScore: &v}
			if err := r.Appraisals.UpsertScore(&sc, "manager_score"); err != nil {
				return err
			}
		}
		fresh, err := r.Appraisals.ByID(ap.ID)
		if err != nil {
			return err
		}
		fresh.ManagerComment, fresh.DevelopmentPlan = strings.TrimSpace(in.ManagerComment), strings.TrimSpace(in.DevelopmentPlan)
		uid := a.EmployeeID
		fresh.AppraiserID = &uid
		oldStatus := fresh.Status

		if in.Submit || fresh.Status == model.AppraisalManagerDone {
			k, b, f, g, _, err := s.compute(r, emp, p, fresh)
			if err != nil {
				return err
			}
			fresh.KpiScore = decimal.NullDecimal{Decimal: k, Valid: true}
			fresh.BehaviorScore = decimal.NullDecimal{Decimal: b, Valid: true}
			fresh.FinalScore = decimal.NullDecimal{Decimal: f, Valid: true}
			fresh.Grade = g
		}
		if in.Submit {
			now := time.Now()
			fresh.Status, fresh.MgrSubmittedAt = model.AppraisalManagerDone, &now
		}
		if err := r.Appraisals.Save(fresh); err != nil {
			return err
		}
		if err := logActivity(r, a, AuditScoreChange, "appraisal", fresh.ID,
			map[string]any{"kind": "manager", "scores": before, "status": oldStatus},
			map[string]any{"kind": "manager", "scores": in.Scores, "status": fresh.Status, "final_score": fresh.FinalScore.Decimal}); err != nil {
			return err
		}
		view, err = s.view(r, a, emp, p)
		return err
	})
	if err == nil {
		s.cache.Bump(bgCtx())
	}
	return view, err
}

// Finalize: HR atau Direktur mengesahkan hasil. Syarat: sudah dinilai atasan dan semua realisasi KPI tervalidasi.
func (s AppraisalService) Finalize(a Actor, employeeID, periodID uint64) (*AppraisalView, error) {
	if !a.CanFinalize() {
		return nil, apperr.Forbidden("Hanya HR atau Direktur yang dapat memfinalisasi penilaian")
	}
	var view *AppraisalView
	err := s.store.Tx(func(r repository.Repos) error {
		emp, err := loadEmployee(r, employeeID)
		if err != nil {
			return err
		}
		p, err := loadPeriod(r, periodID)
		if err != nil {
			return err
		}
		if err := requireOpen(p); err != nil {
			return err
		}
		ap, err := r.Appraisals.Find(p.ID, emp.ID)
		if err != nil {
			return apperr.Conflict("Penilaian belum dikirim oleh atasan")
		}
		if ap.Status == model.AppraisalFinal {
			return apperr.Conflict("Penilaian sudah final")
		}
		if ap.Status != model.AppraisalManagerDone {
			return apperr.Conflict("Penilaian belum dikirim oleh atasan")
		}
		k, b, f, g, allValid, err := s.compute(r, emp, p, ap)
		if err != nil {
			return err
		}
		if !allValid {
			return apperr.Conflict("Masih ada realisasi KPI yang belum divalidasi atau belum diisi")
		}
		old := map[string]any{"status": ap.Status, "final_score": ap.FinalScore.Decimal, "grade": ap.Grade}
		now := time.Now()
		uid := a.EmployeeID
		ap.KpiScore = decimal.NullDecimal{Decimal: k, Valid: true}
		ap.BehaviorScore = decimal.NullDecimal{Decimal: b, Valid: true}
		ap.FinalScore = decimal.NullDecimal{Decimal: f, Valid: true}
		ap.Grade, ap.Status, ap.FinalizedAt, ap.FinalizedBy = g, model.AppraisalFinal, &now, &uid
		if err := r.Appraisals.Save(ap); err != nil {
			return err
		}
		if err := logActivity(r, a, AuditFinalize, "appraisal", ap.ID, old,
			map[string]any{"status": ap.Status, "final_score": f, "grade": g, "kpi_score": k, "behavior_score": b}); err != nil {
			return err
		}
		view, err = s.view(r, a, emp, p)
		return err
	})
	if err == nil {
		s.cache.Bump(bgCtx())
	}
	return view, err
}

// ---------- daftar tim (H-06) ----------

type TeamRow struct {
	Employee     *model.Employee  `json:"employee"`
	Status       string           `json:"status"` // status penilaian
	SelfDone     bool             `json:"self_done"`
	KpiScore     decimal.Decimal  `json:"kpi_score"`
	KpiTotal     int              `json:"kpi_total"`
	KpiValidated int              `json:"kpi_validated"`
	KpiPending   int              `json:"kpi_pending"`
	FinalScore   *decimal.Decimal `json:"final_score"`
	Grade        string           `json:"grade"`
}

type TeamSummary struct {
	Members      int `json:"members"`
	Appraised    int `json:"appraised"` // manager_done + final
	Finalized    int `json:"finalized"`
	PendingKpi   int `json:"pending_kpi"` // realisasi menunggu validasi
	NotAppraised int `json:"not_appraised"`
}

type TeamView struct {
	Period  *model.AppraisalPeriod `json:"period"`
	Rows    []TeamRow              `json:"rows"`
	Summary TeamSummary            `json:"summary"`
}

// teamMembers: anggota departemen (manajer) atau bawahan langsung (Direktur → Manajer).
func teamMembers(r repository.Repos, a Actor) ([]model.Employee, error) {
	seen := map[uint64]bool{}
	var out []model.Employee
	if a.IsManager() && a.DepartmentID != nil {
		ms, err := r.Employees.ByDepartment(*a.DepartmentID, true)
		if err != nil {
			return nil, err
		}
		for _, m := range ms {
			if m.ID != a.EmployeeID {
				seen[m.ID] = true
				out = append(out, m)
			}
		}
	}
	if a.HasAny(model.RoleManager, model.RoleDirector) {
		all, err := r.Employees.AllActive()
		if err != nil {
			return nil, err
		}
		for _, e := range all {
			if e.ManagerID != nil && *e.ManagerID == a.EmployeeID && e.DepartmentID != nil && !seen[e.ID] {
				out = append(out, e)
			}
		}
	}
	return out, nil
}

// teamData adalah muatan data tim (jumlah query tetap, tidak bergantung jumlah anggota).
type teamData struct {
	Period  *model.AppraisalPeriod
	Members []model.Employee
	Sheets  map[uint64]*KpiSheet
	Apprs   map[uint64]model.Appraisal
}

func loadTeamData(r repository.Repos, a Actor, p *model.AppraisalPeriod) (*teamData, error) {
	members, err := teamMembers(r, a)
	if err != nil {
		return nil, err
	}
	ids := make([]uint64, len(members))
	deptSet := map[uint64]bool{}
	var depts []uint64
	for i, m := range members {
		ids[i] = m.ID
		if !deptSet[*m.DepartmentID] {
			deptSet[*m.DepartmentID] = true
			depts = append(depts, *m.DepartmentID)
		}
	}
	tpls, err := r.Templates.ActiveByDepartments(depts)
	if err != nil {
		return nil, err
	}
	results, err := r.Results.ForEmployees(p.ID, ids)
	if err != nil {
		return nil, err
	}
	byEmp := map[uint64][]model.KpiResult{}
	for _, x := range results {
		byEmp[x.EmployeeID] = append(byEmp[x.EmployeeID], x)
	}
	appraisals, err := r.Appraisals.ForEmployees(p.ID, ids)
	if err != nil {
		return nil, err
	}
	td := &teamData{Period: p, Members: members, Sheets: map[uint64]*KpiSheet{}, Apprs: map[uint64]model.Appraisal{}}
	for _, x := range appraisals {
		td.Apprs[x.EmployeeID] = x
	}
	for i := range members {
		m := &members[i]
		sh, err := assembleSheet(KpiScoreService{}, m, p, tpls[*m.DepartmentID], byEmp[m.ID])
		if err != nil {
			return nil, err
		}
		td.Sheets[m.ID] = sh
	}
	return td, nil
}

// Team menampilkan status penilaian anggota tim dengan jumlah query tetap (tanpa N+1).
func (s AppraisalService) Team(a Actor, periodID uint64) (*TeamView, error) {
	if !a.HasAny(model.RoleManager, model.RoleDirector) {
		return nil, apperr.Forbidden("Hanya manajer yang dapat membuka penilaian tim")
	}
	r := s.store.R()
	p, err := loadPeriod(r, periodID)
	if err != nil {
		return nil, err
	}
	td, err := loadTeamData(r, a, p)
	if err != nil {
		return nil, err
	}
	view := &TeamView{Period: p, Rows: []TeamRow{}}
	for i := range td.Members {
		m := &td.Members[i]
		sh := td.Sheets[m.ID]
		row := TeamRow{Employee: m, Status: model.AppraisalDraft, KpiScore: sh.Score, KpiTotal: sh.Total,
			KpiValidated: sh.Validated, KpiPending: sh.Pending}
		if ap, ok := td.Apprs[m.ID]; ok {
			row.Status = ap.Status
			row.SelfDone = ap.Status != model.AppraisalDraft || ap.SelfSubmittedAt != nil
			if ap.FinalScore.Valid && (ap.Status == model.AppraisalManagerDone || ap.Status == model.AppraisalFinal) {
				f := ap.FinalScore.Decimal
				row.FinalScore, row.Grade = &f, ap.Grade
			}
		}
		view.Summary.PendingKpi += sh.Pending
		switch row.Status {
		case model.AppraisalFinal:
			view.Summary.Finalized++
			view.Summary.Appraised++
		case model.AppraisalManagerDone:
			view.Summary.Appraised++
		default:
			view.Summary.NotAppraised++
		}
		view.Rows = append(view.Rows, row)
	}
	view.Summary.Members = len(view.Rows)
	return view, nil
}
