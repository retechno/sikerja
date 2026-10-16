package service

import (
	"fmt"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"github.com/retechno/sikerja/backend/internal/apperr"
	"github.com/retechno/sikerja/backend/internal/model"
	"github.com/retechno/sikerja/backend/internal/repository"
)

type PeriodService struct {
	store    repository.Store
	cache    *Cache
	notifier Notifier
}

func NewPeriodService(store repository.Store, cache *Cache) *PeriodService {
	return &PeriodService{store: store, cache: cache, notifier: NoopNotifier{}}
}

// SetNotifier mengaktifkan notifikasi "periode dibuka".
func (s *PeriodService) SetNotifier(n Notifier) { s.notifier = n }

// notifyOpened memberi tahu semua karyawan aktif bahwa periode dibuka.
func (s *PeriodService) notifyOpened(p *model.AppraisalPeriod) {
	emps, err := s.store.R().Employees.AllActive()
	if err != nil {
		return
	}
	s.notifier.Notify(bgCtx(), Notice{
		Type:    EvPeriodOpened,
		Title:   "Periode penilaian " + p.Name + " dibuka",
		Message: fmt.Sprintf("Periode penilaian %s sudah dibuka (sampai %s). Silakan isi realisasi KPI dan self-assessment Anda.", p.Name, fmtDate(DateOnly(p.EndDate))),
		Link:    "/kpi",
		Data:    map[string]any{"period_id": p.ID},
	}, emps...)
}

type PeriodInput struct {
	Name           string          `json:"name"`
	Type           string          `json:"type"`
	StartDate      time.Time       `json:"-"`
	EndDate        time.Time       `json:"-"`
	KpiWeight      decimal.Decimal `json:"kpi_weight"`
	BehaviorWeight decimal.Decimal `json:"behavior_weight"`
}

func (in PeriodInput) validate() error {
	f := map[string]string{}
	if strings.TrimSpace(in.Name) == "" {
		f["name"] = "Nama periode wajib diisi"
	}
	if in.Type != model.PeriodMonthly && in.Type != model.PeriodQuarterly {
		f["type"] = "Jenis harus bulanan atau kuartalan"
	}
	if in.StartDate.IsZero() {
		f["start_date"] = "Tanggal mulai wajib diisi"
	}
	if in.EndDate.IsZero() {
		f["end_date"] = "Tanggal selesai wajib diisi"
	} else if !in.StartDate.IsZero() && in.EndDate.Before(in.StartDate) {
		f["end_date"] = "Tanggal selesai tidak boleh sebelum tanggal mulai"
	}
	if in.KpiWeight.IsNegative() || in.BehaviorWeight.IsNegative() ||
		in.KpiWeight.Add(in.BehaviorWeight).Sub(dec100).Abs().GreaterThan(weightEpsilon) {
		f["kpi_weight"] = "Bobot KPI + bobot perilaku harus 100%"
	}
	if len(f) > 0 {
		return apperr.Validation("Data periode tidak valid", f)
	}
	return nil
}

// List: HR melihat semua; yang lain hanya periode non-draft.
func (s *PeriodService) List(a Actor) ([]model.AppraisalPeriod, error) {
	all, err := s.store.R().Periods.List()
	if err != nil || a.IsHR() {
		return all, err
	}
	out := all[:0:0]
	for _, p := range all {
		if p.Status != model.PeriodDraft {
			out = append(out, p)
		}
	}
	return out, nil
}

func (s *PeriodService) Get(a Actor, id uint64) (*model.AppraisalPeriod, error) {
	p, err := loadPeriod(s.store.R(), id)
	if err != nil {
		return nil, err
	}
	if p.Status == model.PeriodDraft && !a.IsHR() {
		return nil, apperr.NotFound("Periode tidak ditemukan")
	}
	return p, nil
}

func loadPeriod(r repository.Repos, id uint64) (*model.AppraisalPeriod, error) {
	p, err := r.Periods.ByID(id)
	if err != nil {
		if err == repository.ErrNotFound {
			return nil, apperr.NotFound("Periode tidak ditemukan")
		}
		return nil, err
	}
	return p, nil
}

// requireOpen memastikan periode Aktif: realisasi & penilaian hanya bisa diubah saat itu.
func requireOpen(p *model.AppraisalPeriod) error {
	switch p.Status {
	case model.PeriodActive:
		return nil
	case model.PeriodClosed:
		return apperr.Locked("Periode " + p.Name + " sudah ditutup; data terkunci")
	default:
		return apperr.BadRequest("Periode " + p.Name + " belum dibuka")
	}
}

func (s *PeriodService) Create(a Actor, in PeriodInput) (*model.AppraisalPeriod, error) {
	if !a.IsHR() {
		return nil, apperr.Forbidden("Hanya HR yang dapat membuat periode")
	}
	if err := in.validate(); err != nil {
		return nil, err
	}
	p := &model.AppraisalPeriod{Name: strings.TrimSpace(in.Name), Type: in.Type, StartDate: in.StartDate,
		EndDate: in.EndDate, Status: model.PeriodDraft, KpiWeight: in.KpiWeight, BehaviorWeight: in.BehaviorWeight}
	if err := s.store.R().Periods.Create(p); err != nil {
		return nil, err
	}
	return p, nil
}

// Update: periode Ditutup tidak bisa diubah (buka kembali dulu).
func (s *PeriodService) Update(a Actor, id uint64, in PeriodInput) (*model.AppraisalPeriod, error) {
	if !a.IsHR() {
		return nil, apperr.Forbidden("Hanya HR yang dapat mengubah periode")
	}
	if err := in.validate(); err != nil {
		return nil, err
	}
	var out *model.AppraisalPeriod
	err := s.store.Tx(func(r repository.Repos) error {
		p, err := loadPeriod(r, id)
		if err != nil {
			return err
		}
		if p.Status == model.PeriodClosed {
			return apperr.Locked("Periode yang sudah ditutup tidak dapat diubah")
		}
		oldW := map[string]any{"kpi_weight": p.KpiWeight, "behavior_weight": p.BehaviorWeight}
		p.Name, p.Type, p.StartDate, p.EndDate = strings.TrimSpace(in.Name), in.Type, in.StartDate, in.EndDate
		p.KpiWeight, p.BehaviorWeight = in.KpiWeight, in.BehaviorWeight
		if err := r.Periods.Save(p); err != nil {
			return err
		}
		out = p
		return logActivity(r, a, "period.update", "appraisal_period", p.ID, oldW,
			map[string]any{"kpi_weight": p.KpiWeight, "behavior_weight": p.BehaviorWeight})
	})
	if err == nil {
		s.cache.Bump(bgCtx())
	}
	return out, err
}

func (s *PeriodService) transition(a Actor, id uint64, from []string, to, action string) (*model.AppraisalPeriod, error) {
	if !a.IsHR() {
		return nil, apperr.Forbidden("Hanya HR yang dapat mengubah status periode")
	}
	var out *model.AppraisalPeriod
	err := s.store.Tx(func(r repository.Repos) error {
		p, err := loadPeriod(r, id)
		if err != nil {
			return err
		}
		ok := false
		for _, f := range from {
			ok = ok || p.Status == f
		}
		if !ok {
			return apperr.Conflict("Status periode saat ini (" + p.Status + ") tidak dapat diubah menjadi " + to)
		}
		old := p.Status
		p.Status = to
		if err := r.Periods.Save(p); err != nil {
			return err
		}
		out = p
		return logActivity(r, a, action, "appraisal_period", p.ID,
			map[string]any{"status": old}, map[string]any{"status": to})
	})
	if err == nil {
		s.cache.Bump(bgCtx())
	}
	return out, err
}

// Open: Draft → Aktif.
func (s *PeriodService) Open(a Actor, id uint64) (*model.AppraisalPeriod, error) {
	p, err := s.transition(a, id, []string{model.PeriodDraft}, model.PeriodActive, AuditPeriodOpen)
	if err == nil {
		s.notifyOpened(p)
	}
	return p, err
}

// Close: Aktif → Ditutup (data terkunci).
func (s *PeriodService) Close(a Actor, id uint64) (*model.AppraisalPeriod, error) {
	return s.transition(a, id, []string{model.PeriodActive}, model.PeriodClosed, AuditPeriodClose)
}

// Reopen: Ditutup → Aktif; hanya HR dan tercatat di audit log.
func (s *PeriodService) Reopen(a Actor, id uint64) (*model.AppraisalPeriod, error) {
	return s.transition(a, id, []string{model.PeriodClosed}, model.PeriodActive, AuditPeriodReopen)
}

// Resolve mengembalikan id periode: id yang diminta, atau (0) periode aktif terbaru / terbaru yang bukan draft.
func (s *PeriodService) Resolve(a Actor, id uint64) (uint64, error) {
	p, err := resolvePeriod(s.store.R(), id)
	if err != nil {
		return 0, err
	}
	return p.ID, nil
}

// PeriodProgress: progres pengisian satu periode (untuk H-14).
type PeriodProgress struct {
	PeriodID        uint64           `json:"period_id"`
	Employees       int              `json:"employees"`        // karyawan aktif yang dinilai
	ExpectedResults int              `json:"expected_results"` // karyawan × indikator
	Results         int64            `json:"results"`
	Validated       int64            `json:"validated"`
	SelfDone        int64            `json:"self_done"`
	ManagerDone     int64            `json:"manager_done"`
	Finalized       int64            `json:"finalized"`
	AvgFinal        *decimal.Decimal `json:"avg_final"`
}

// Progress mengembalikan progres semua periode (HR).
func (s *PeriodService) Progress(a Actor) ([]PeriodProgress, error) {
	if !a.IsHR() {
		return nil, apperr.Forbidden("Hanya HR yang dapat melihat progres periode")
	}
	r := s.store.R()
	ps, err := r.Periods.List()
	if err != nil {
		return nil, err
	}
	emps, err := r.Employees.AllActive()
	if err != nil {
		return nil, err
	}
	perDept := map[uint64]int{}
	for _, e := range emps {
		if e.DepartmentID != nil {
			perDept[*e.DepartmentID]++
		}
	}
	depts := make([]uint64, 0, len(perDept))
	for d := range perDept {
		depts = append(depts, d)
	}
	tpls, err := r.Templates.ActiveByDepartments(depts)
	if err != nil {
		return nil, err
	}
	employees, expected := 0, 0
	for d, n := range perDept {
		employees += n
		if t := tpls[d]; t != nil {
			expected += n * len(t.Indicators)
		}
	}
	rows, err := r.Reports.PeriodProgress()
	if err != nil {
		return nil, err
	}
	out := make([]PeriodProgress, 0, len(ps))
	for _, p := range ps {
		pp := PeriodProgress{PeriodID: p.ID, Employees: employees, ExpectedResults: expected}
		if x := rows[p.ID]; x != nil {
			pp.Results, pp.Validated, pp.SelfDone, pp.ManagerDone, pp.Finalized = x.Results, x.Validated, x.SelfDone, x.MgrDone, x.Final
			if x.Avg != nil {
				v := round2(*x.Avg)
				pp.AvgFinal = &v
			}
		}
		out = append(out, pp)
	}
	return out, nil
}
