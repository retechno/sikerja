package service

import (
	"context"
	"net/http"

	"github.com/retechno/sikerja/backend/internal/apperr"
	"github.com/retechno/sikerja/backend/internal/model"
	"github.com/retechno/sikerja/backend/internal/repository"
)

const maxImportItems = 100

// SetSource memasang sumber impor eksternal (nil mematikannya).
func (s *HolidayService) SetSource(src HolidaySource) { s.src = src }

// HolidayCandidate satu baris pratinjau impor.
type HolidayCandidate struct {
	Date         string `json:"date"`
	Name         string `json:"name"`
	Type         string `json:"type"`
	Tentative    bool   `json:"tentative"`
	Exists       bool   `json:"exists"`
	ExistingName string `json:"existing_name,omitempty"`
}

func checkImportYear(year int) error {
	if year < 2000 || year > 2100 {
		return apperr.Validation("Tahun tidak valid", map[string]string{"year": "Tahun harus antara 2000 dan 2100"})
	}
	return nil
}

// PreviewImport mengambil daftar libur setahun dari sumber eksternal dan menandai tanggal yang sudah ada.
// Tidak menyimpan apa pun: HR memeriksa dulu terhadap SKB, baru mengonfirmasi (ImportSelected).
func (s *HolidayService) PreviewImport(ctx context.Context, a Actor, year int) ([]HolidayCandidate, error) {
	if err := requireHR(a); err != nil {
		return nil, err
	}
	if err := checkImportYear(year); err != nil {
		return nil, err
	}
	if s.src == nil {
		return nil, apperr.New(http.StatusNotFound, "holiday_source_disabled",
			"Impor dari sumber eksternal tidak diaktifkan; tambahkan hari libur secara manual")
	}
	list, err := s.src.Fetch(ctx, year)
	if err != nil {
		return nil, err
	}
	existing, err := s.store.R().Holidays.ByYear(year)
	if err != nil {
		return nil, err
	}
	have := make(map[string]string, len(existing))
	for _, h := range existing {
		have[h.Date.Format("2006-01-02")] = h.Name
	}
	out := make([]HolidayCandidate, 0, len(list))
	for _, h := range list {
		c := HolidayCandidate{Date: h.Date.Format("2006-01-02"), Name: h.Name, Type: h.Type, Tentative: h.Tentative}
		if name, ok := have[c.Date]; ok {
			c.Exists, c.ExistingName = true, name
		}
		out = append(out, c)
	}
	return out, nil
}

// ImportSelected menyimpan baris pratinjau yang dipilih HR. Tanggal yang sudah ada dilewati (tidak ditimpa).
func (s *HolidayService) ImportSelected(a Actor, items []HolidayInput) (added, skipped int, err error) {
	if err := requireHR(a); err != nil {
		return 0, 0, err
	}
	if len(items) == 0 {
		return 0, 0, apperr.Validation("Pilih minimal satu hari libur", map[string]string{"items": "Belum ada yang dipilih"})
	}
	if len(items) > maxImportItems {
		return 0, 0, apperr.BadRequest("Terlalu banyak baris sekaligus")
	}
	for i := range items {
		if err := items[i].validate(); err != nil {
			return 0, 0, err
		}
	}
	err = s.store.Tx(func(r repository.Repos) error {
		seen := map[string]bool{}
		for _, in := range items {
			key := in.Date.Format("2006-01-02")
			taken, err := r.Holidays.DateTaken(in.Date, 0)
			if err != nil {
				return err
			}
			if taken || seen[key] {
				skipped++
				continue
			}
			seen[key] = true
			row := modelHoliday(in)
			if err := r.Holidays.Create(&row); err != nil {
				return err
			}
			added++
		}
		return logActivity(r, a, AuditHolidayChange, "holiday", 0, nil, map[string]any{"import": "external", "added": added, "skipped": skipped})
	})
	return added, skipped, err
}

func modelHoliday(in HolidayInput) model.Holiday {
	return model.Holiday{Date: in.Date, Name: in.Name, Type: in.Type}
}
