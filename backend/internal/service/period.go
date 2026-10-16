package service

import (
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"github.com/retechno/sikerja/backend/internal/apperr"
	"github.com/retechno/sikerja/backend/internal/model"
	"github.com/retechno/sikerja/backend/internal/repository"
)

type PeriodService struct {
	store repository.Store
	cache *Cache
}

func NewPeriodService(store repository.Store, cache *Cache) *PeriodService {
	return &PeriodService{store: store, cache: cache}
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
	return s.transition(a, id, []string{model.PeriodDraft}, model.PeriodActive, AuditPeriodOpen)
}

// Close: Aktif → Ditutup (data terkunci).
func (s *PeriodService) Close(a Actor, id uint64) (*model.AppraisalPeriod, error) {
	return s.transition(a, id, []string{model.PeriodActive}, model.PeriodClosed, AuditPeriodClose)
}

// Reopen: Ditutup → Aktif; hanya HR dan tercatat di audit log.
func (s *PeriodService) Reopen(a Actor, id uint64) (*model.AppraisalPeriod, error) {
	return s.transition(a, id, []string{model.PeriodClosed}, model.PeriodActive, AuditPeriodReopen)
}
