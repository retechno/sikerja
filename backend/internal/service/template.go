package service

import (
	"context"
	"strings"

	"github.com/shopspring/decimal"

	"github.com/retechno/sikerja/backend/internal/apperr"
	"github.com/retechno/sikerja/backend/internal/model"
	"github.com/retechno/sikerja/backend/internal/repository"
)

func bgCtx() context.Context { return context.Background() }

type TemplateService struct {
	store repository.Store
	kpi   KpiScoreService
	cache *Cache
}

func NewTemplateService(store repository.Store, cache *Cache) *TemplateService {
	return &TemplateService{store: store, cache: cache}
}

type IndicatorIn struct {
	ID        uint64          `json:"id"` // 0 = indikator baru
	Name      string          `json:"name"`
	Unit      string          `json:"unit"`
	Target    decimal.Decimal `json:"target"`
	Weight    decimal.Decimal `json:"weight"` // persen
	Direction string          `json:"direction"`
	Source    string          `json:"source"`
}

type TemplateIn struct {
	Name       string        `json:"name"`
	IsActive   bool          `json:"is_active"`
	Indicators []IndicatorIn `json:"indicators"`
}

// TemplateTotals ringkasan bobot untuk UI.
type TemplateView struct {
	model.KpiTemplate
	TotalWeight decimal.Decimal `json:"total_weight"`
}

func withTotal(t model.KpiTemplate) TemplateView {
	sum := decimal.Zero
	for _, i := range t.Indicators {
		sum = sum.Add(i.Weight)
	}
	return TemplateView{KpiTemplate: t, TotalWeight: sum}
}

func canReadTemplates(a Actor) bool { return a.IsHR() || a.IsDirector() }

func (s *TemplateService) List(a Actor) ([]TemplateView, error) {
	if !canReadTemplates(a) {
		return nil, apperr.Forbidden("Anda tidak berhak melihat template KPI")
	}
	ts, err := s.store.R().Templates.List()
	if err != nil {
		return nil, err
	}
	out := make([]TemplateView, len(ts))
	for i, t := range ts {
		out[i] = withTotal(t)
	}
	return out, nil
}

func (s *TemplateService) Get(a Actor, id uint64) (*TemplateView, error) {
	if !canReadTemplates(a) {
		return nil, apperr.Forbidden("Anda tidak berhak melihat template KPI")
	}
	t, err := s.store.R().Templates.ByID(id)
	if err != nil {
		if err == repository.ErrNotFound {
			return nil, apperr.NotFound("Template tidak ditemukan")
		}
		return nil, err
	}
	v := withTotal(*t)
	return &v, nil
}

// Create membuat template kosong untuk satu departemen (isi indikator lewat Save).
func (s *TemplateService) Create(a Actor, deptID uint64, name string) (*model.KpiTemplate, error) {
	if !a.IsHR() {
		return nil, apperr.Forbidden("Hanya HR yang dapat mengelola template KPI")
	}
	if strings.TrimSpace(name) == "" {
		return nil, apperr.Validation("Nama template wajib diisi", map[string]string{"name": "Wajib diisi"})
	}
	r := s.store.R()
	if _, err := r.Departments.ByID(deptID); err != nil {
		return nil, apperr.Validation("Departemen tidak ditemukan", map[string]string{"department_id": "Tidak valid"})
	}
	t := &model.KpiTemplate{DepartmentID: deptID, Name: strings.TrimSpace(name), IsActive: false}
	if err := r.Templates.Create(t); err != nil {
		return nil, err
	}
	return t, nil
}

func (in TemplateIn) validate(k KpiScoreService) error {
	f := map[string]string{}
	if strings.TrimSpace(in.Name) == "" {
		f["name"] = "Nama template wajib diisi"
	}
	weights := make([]decimal.Decimal, 0, len(in.Indicators))
	for i, ind := range in.Indicators {
		key := func(field string) string { return "indicators." + itoa(i) + "." + field }
		if strings.TrimSpace(ind.Name) == "" {
			f[key("name")] = "Nama indikator wajib diisi"
		}
		if !ind.Target.IsPositive() {
			f[key("target")] = "Target harus lebih besar dari 0"
		}
		if ind.Weight.IsNegative() || ind.Weight.IsZero() {
			f[key("weight")] = "Bobot harus lebih besar dari 0"
		}
		if ind.Direction != DirectionHigher && ind.Direction != DirectionLower {
			f[key("direction")] = "Arah harus higher atau lower"
		}
		if ind.Source != "" && ind.Source != model.SourceManual && ind.Source != model.SourceIntegration {
			f[key("source")] = "Sumber data harus manual atau integration"
		}
		weights = append(weights, ind.Weight)
	}
	if len(in.Indicators) == 0 {
		f["indicators"] = "Minimal satu indikator"
	} else if err := k.ValidateWeights(weights); err != nil {
		f["indicators"] = ErrWeightNot100.Error()
	}
	if len(f) > 0 {
		return apperr.Validation("Template tidak valid", f)
	}
	return nil
}

func itoa(i int) string { return decimal.NewFromInt(int64(i)).String() }

// Save mengganti isi template. Total bobot wajib 100%. Indikator yang dihapus di-soft-delete
// agar realisasi lama tetap terbaca. Mengaktifkan template menonaktifkan template lain di departemen yang sama.
func (s *TemplateService) Save(a Actor, id uint64, in TemplateIn) (*TemplateView, error) {
	if !a.IsHR() {
		return nil, apperr.Forbidden("Hanya HR yang dapat mengelola template KPI")
	}
	if err := in.validate(s.kpi); err != nil {
		return nil, err
	}
	var saved uint64
	err := s.store.Tx(func(r repository.Repos) error {
		t, err := r.Templates.ByID(id)
		if err != nil {
			if err == repository.ErrNotFound {
				return apperr.NotFound("Template tidak ditemukan")
			}
			return err
		}
		before := summarize(*t)

		existing := map[uint64]model.KpiIndicator{}
		for _, i := range t.Indicators {
			existing[i.ID] = i
		}
		keep := map[uint64]bool{}
		for idx, ind := range in.Indicators {
			src := ind.Source
			if src == "" {
				src = model.SourceManual
			}
			row := model.KpiIndicator{KpiTemplateID: t.ID, Name: strings.TrimSpace(ind.Name), Unit: strings.TrimSpace(ind.Unit),
				Target: ind.Target, Weight: ind.Weight, Direction: ind.Direction, Source: src, Sort: idx + 1}
			if old, ok := existing[ind.ID]; ok && ind.ID != 0 {
				row.Base = old.Base
				keep[old.ID] = true
				if err := r.Templates.SaveIndicator(&row); err != nil {
					return err
				}
			} else if err := r.Templates.CreateIndicator(&row); err != nil {
				return err
			}
		}
		for oid := range existing {
			if !keep[oid] {
				if err := r.Templates.DeleteIndicator(oid); err != nil {
					return err
				}
			}
		}

		t.Name, t.IsActive = strings.TrimSpace(in.Name), in.IsActive
		if err := r.Templates.Save(t); err != nil {
			return err
		}
		if t.IsActive {
			all, err := r.Templates.List()
			if err != nil {
				return err
			}
			for _, o := range all {
				if o.DepartmentID == t.DepartmentID && o.ID != t.ID && o.IsActive {
					o.IsActive = false
					if err := r.Templates.Save(&o); err != nil {
						return err
					}
				}
			}
		}
		saved = t.ID
		after, err := r.Templates.ByID(t.ID)
		if err != nil {
			return err
		}
		return logActivity(r, a, AuditTemplateChange, "kpi_template", t.ID, before, summarize(*after))
	})
	if err != nil {
		return nil, err
	}
	s.cache.Bump(bgCtx())
	return s.Get(a, saved)
}

func summarize(t model.KpiTemplate) map[string]any {
	items := make([]map[string]any, 0, len(t.Indicators))
	for _, i := range t.Indicators {
		items = append(items, map[string]any{"id": i.ID, "name": i.Name, "target": i.Target, "weight": i.Weight, "direction": i.Direction})
	}
	return map[string]any{"name": t.Name, "is_active": t.IsActive, "indicators": items}
}
