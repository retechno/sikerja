package service

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"github.com/retechno/sikerja/backend/internal/apperr"
	"github.com/retechno/sikerja/backend/internal/model"
	"github.com/retechno/sikerja/backend/internal/repository"
)

// ---------- jenis cuti ----------

type LeaveTypeService struct{ store repository.Store }

func NewLeaveTypeService(store repository.Store) *LeaveTypeService {
	return &LeaveTypeService{store: store}
}

type LeaveTypeInput struct {
	Name              string           `json:"name"`
	Code              string           `json:"code"`
	AnnualQuota       *decimal.Decimal `json:"annual_quota"`
	DeductsBalance    bool             `json:"deducts_balance"`
	RequiresAttach    bool             `json:"requires_attachment"`
	AttachMinDays     decimal.Decimal  `json:"attachment_min_days"`
	AllowHalfDay      bool             `json:"allow_half_day"`
	MinTenureMonths   int              `json:"min_tenure_months"`
	MaxDaysPerRequest *decimal.Decimal `json:"max_days_per_request"`
	Sort              int              `json:"sort"`
	IsActive          bool             `json:"is_active"`
}

var codePattern = regexp.MustCompile(`^[A-Z0-9][A-Z0-9-]{0,19}$`)

func (in *LeaveTypeInput) validate() error {
	f := map[string]string{}
	in.Name = strings.TrimSpace(in.Name)
	in.Code = strings.ToUpper(strings.TrimSpace(in.Code))
	if in.Name == "" {
		f["name"] = "Nama wajib diisi"
	}
	if !codePattern.MatchString(in.Code) {
		f["code"] = "Kode 1–20 karakter: huruf besar, angka, atau tanda hubung"
	}
	if in.AnnualQuota != nil && in.AnnualQuota.IsNegative() {
		f["annual_quota"] = "Kuota tidak boleh negatif"
	}
	if in.DeductsBalance && in.AnnualQuota == nil {
		f["annual_quota"] = "Isi kuota per tahun bila saldo dipotong"
	}
	if in.MaxDaysPerRequest != nil && !in.MaxDaysPerRequest.IsPositive() {
		f["max_days_per_request"] = "Batas hari harus lebih dari 0"
	}
	if in.AttachMinDays.IsNegative() {
		f["attachment_min_days"] = "Tidak boleh negatif"
	}
	if in.MinTenureMonths < 0 || in.MinTenureMonths > 120 {
		f["min_tenure_months"] = "Masa kerja 0–120 bulan"
	}
	if len(f) > 0 {
		return apperr.Validation("Data jenis cuti tidak valid", f)
	}
	return nil
}

func (in LeaveTypeInput) apply(t *model.LeaveType) {
	t.Name, t.Code, t.AnnualQuota, t.DeductsBalance = in.Name, in.Code, in.AnnualQuota, in.DeductsBalance
	t.RequiresAttach, t.AttachMinDays, t.AllowHalfDay = in.RequiresAttach, in.AttachMinDays, in.AllowHalfDay
	t.MinTenureMonths, t.MaxDaysPerRequest, t.Sort, t.IsActive = in.MinTenureMonths, in.MaxDaysPerRequest, in.Sort, in.IsActive
}

// List: HR melihat semua jenis; yang lain hanya yang aktif.
func (s *LeaveTypeService) List(a Actor) ([]model.LeaveType, error) {
	return s.store.R().LeaveTypes.List(!a.IsHR())
}

func (s *LeaveTypeService) Create(a Actor, in LeaveTypeInput) (*model.LeaveType, error) {
	if err := requireHR(a); err != nil {
		return nil, err
	}
	if err := in.validate(); err != nil {
		return nil, err
	}
	t := &model.LeaveType{}
	in.apply(t)
	err := s.store.Tx(func(r repository.Repos) error {
		if taken, err := r.LeaveTypes.CodeTaken(t.Code, 0); err != nil {
			return err
		} else if taken {
			return fieldErr("Kode sudah dipakai", "code")
		}
		if err := r.LeaveTypes.Create(t); err != nil {
			return err
		}
		return logActivity(r, a, AuditLeaveTypeSave, "leave_type", t.ID, nil, t)
	})
	return t, err
}

func (s *LeaveTypeService) Update(a Actor, id uint64, in LeaveTypeInput) (*model.LeaveType, error) {
	if err := requireHR(a); err != nil {
		return nil, err
	}
	if err := in.validate(); err != nil {
		return nil, err
	}
	var out *model.LeaveType
	err := s.store.Tx(func(r repository.Repos) error {
		t, err := r.LeaveTypes.ByID(id)
		if err != nil {
			if err == repository.ErrNotFound {
				return apperr.NotFound("Jenis cuti tidak ditemukan")
			}
			return err
		}
		if taken, err := r.LeaveTypes.CodeTaken(in.Code, id); err != nil {
			return err
		} else if taken {
			return fieldErr("Kode sudah dipakai", "code")
		}
		old := *t
		in.apply(t)
		if err := r.LeaveTypes.Update(t); err != nil {
			return err
		}
		// kuota baru berlaku untuk saldo tahun berjalan yang belum diubah manual
		if trackedType(&old) && trackedType(t) && !old.AnnualQuota.Equal(*t.AnnualQuota) {
			if _, err := r.Balances.ApplyQuota(t.ID, time.Now().Year(), *old.AnnualQuota, *t.AnnualQuota); err != nil {
				return err
			}
		}
		out = t
		return logActivity(r, a, AuditLeaveTypeSave, "leave_type", t.ID, old, t)
	})
	return out, err
}

func (s *LeaveTypeService) Delete(a Actor, id uint64) error {
	if err := requireHR(a); err != nil {
		return err
	}
	return s.store.Tx(func(r repository.Repos) error {
		t, err := r.LeaveTypes.ByID(id)
		if err != nil {
			if err == repository.ErrNotFound {
				return apperr.NotFound("Jenis cuti tidak ditemukan")
			}
			return err
		}
		if used, err := r.LeaveTypes.InUse(id); err != nil {
			return err
		} else if used {
			return apperr.Conflict("Jenis ini sudah dipakai pengajuan; nonaktifkan saja agar riwayat tetap utuh")
		}
		if err := r.LeaveTypes.Delete(id); err != nil {
			return err
		}
		return logActivity(r, a, AuditLeaveTypeSave, "leave_type", id, t, nil)
	})
}

// ---------- hari libur ----------

type HolidayService struct{ store repository.Store }

func NewHolidayService(store repository.Store) *HolidayService { return &HolidayService{store: store} }

type HolidayInput struct {
	Date time.Time `json:"-"`
	Name string    `json:"name"`
	Type string    `json:"type"`
}

func (in *HolidayInput) validate() error {
	f := map[string]string{}
	in.Name = strings.TrimSpace(in.Name)
	if in.Date.IsZero() {
		f["date"] = "Tanggal wajib diisi"
	}
	if in.Name == "" {
		f["name"] = "Nama wajib diisi"
	}
	if in.Type == "" {
		in.Type = model.HolidayNational
	}
	if in.Type != model.HolidayNational && in.Type != model.HolidayJoint {
		f["type"] = "Jenis harus nasional atau cuti bersama"
	}
	if len(f) > 0 {
		return apperr.Validation("Data hari libur tidak valid", f)
	}
	in.Date = DateOnly(in.Date)
	return nil
}

func (s *HolidayService) List(year int) ([]model.Holiday, error) {
	if year == 0 {
		year = time.Now().Year()
	}
	hs, err := s.store.R().Holidays.ByYear(year)
	if hs == nil {
		hs = []model.Holiday{} // JSON [] bukan null
	}
	return hs, err
}

func (s *HolidayService) Create(a Actor, in HolidayInput) (*model.Holiday, error) {
	if err := requireHR(a); err != nil {
		return nil, err
	}
	if err := in.validate(); err != nil {
		return nil, err
	}
	h := &model.Holiday{Date: in.Date, Name: in.Name, Type: in.Type}
	err := s.store.Tx(func(r repository.Repos) error {
		if taken, err := r.Holidays.DateTaken(h.Date, 0); err != nil {
			return err
		} else if taken {
			return fieldErr("Tanggal ini sudah terdaftar sebagai hari libur", "date")
		}
		if err := r.Holidays.Create(h); err != nil {
			return err
		}
		return logActivity(r, a, AuditHolidayChange, "holiday", h.ID, nil, h)
	})
	return h, err
}

func (s *HolidayService) Update(a Actor, id uint64, in HolidayInput) (*model.Holiday, error) {
	if err := requireHR(a); err != nil {
		return nil, err
	}
	if err := in.validate(); err != nil {
		return nil, err
	}
	var out *model.Holiday
	err := s.store.Tx(func(r repository.Repos) error {
		h, err := r.Holidays.ByID(id)
		if err != nil {
			if err == repository.ErrNotFound {
				return apperr.NotFound("Hari libur tidak ditemukan")
			}
			return err
		}
		if taken, err := r.Holidays.DateTaken(in.Date, id); err != nil {
			return err
		} else if taken {
			return fieldErr("Tanggal ini sudah terdaftar sebagai hari libur", "date")
		}
		old := *h
		h.Date, h.Name, h.Type = in.Date, in.Name, in.Type
		if err := r.Holidays.Update(h); err != nil {
			return err
		}
		out = h
		return logActivity(r, a, AuditHolidayChange, "holiday", h.ID, old, h)
	})
	return out, err
}

func (s *HolidayService) Delete(a Actor, id uint64) error {
	if err := requireHR(a); err != nil {
		return err
	}
	return s.store.Tx(func(r repository.Repos) error {
		h, err := r.Holidays.ByID(id)
		if err != nil {
			if err == repository.ErrNotFound {
				return apperr.NotFound("Hari libur tidak ditemukan")
			}
			return err
		}
		if err := r.Holidays.Delete(id); err != nil {
			return err
		}
		return logActivity(r, a, AuditHolidayChange, "holiday", id, h, nil)
	})
}

// ImportOfficial menambahkan daftar libur resmi bawaan untuk tahun tertentu (tanggal yang sudah ada dilewati).
// Daftar bawaan harus dicek ulang HR terhadap SKB terbaru; lihat docs/DECISIONS.md.
func (s *HolidayService) ImportOfficial(a Actor, year int) (added int, err error) {
	if err := requireHR(a); err != nil {
		return 0, err
	}
	list, ok := OfficialHolidays[year]
	if !ok {
		return 0, apperr.NotFound("Daftar libur resmi untuk tahun ini belum tersedia; tambahkan manual setelah SKB terbit")
	}
	err = s.store.Tx(func(r repository.Repos) error {
		for _, h := range list {
			taken, err := r.Holidays.DateTaken(h.Date, 0)
			if err != nil {
				return err
			}
			if taken {
				continue
			}
			row := h
			if err := r.Holidays.Create(&row); err != nil {
				return err
			}
			added++
		}
		return logActivity(r, a, AuditHolidayChange, "holiday", 0, nil, map[string]any{"import_year": year, "added": added})
	})
	return added, err
}

func hol(m, d int, name, typ string) model.Holiday {
	return model.Holiday{Date: time.Date(2026, time.Month(m), d, 0, 0, 0, 0, time.UTC), Name: name, Type: typ}
}

// OfficialHolidays: libur nasional & cuti bersama menurut SKB Tiga Menteri. WAJIB dicek ulang HR.
var OfficialHolidays = map[int][]model.Holiday{
	2026: {
		hol(1, 1, "Tahun Baru 2026 Masehi", model.HolidayNational),
		hol(1, 16, "Isra Mi'raj Nabi Muhammad SAW", model.HolidayNational),
		hol(2, 16, "Cuti Bersama Tahun Baru Imlek", model.HolidayJoint),
		hol(2, 17, "Tahun Baru Imlek 2577 Kongzili", model.HolidayNational),
		hol(3, 18, "Cuti Bersama Hari Suci Nyepi", model.HolidayJoint),
		hol(3, 19, "Hari Suci Nyepi (Tahun Baru Saka 1948)", model.HolidayNational),
		hol(3, 20, "Cuti Bersama Idul Fitri", model.HolidayJoint),
		hol(3, 21, "Idul Fitri 1447 H", model.HolidayNational),
		hol(3, 22, "Idul Fitri 1447 H", model.HolidayNational),
		hol(3, 23, "Cuti Bersama Idul Fitri", model.HolidayJoint),
		hol(3, 24, "Cuti Bersama Idul Fitri", model.HolidayJoint),
		hol(4, 3, "Wafat Yesus Kristus", model.HolidayNational),
		hol(4, 5, "Hari Paskah", model.HolidayNational),
		hol(5, 1, "Hari Buruh Internasional", model.HolidayNational),
		hol(5, 14, "Kenaikan Yesus Kristus", model.HolidayNational),
		hol(5, 15, "Cuti Bersama Kenaikan Yesus Kristus", model.HolidayJoint),
		hol(5, 27, "Idul Adha 1447 H", model.HolidayNational),
		hol(5, 28, "Cuti Bersama Idul Adha", model.HolidayJoint),
		hol(5, 31, "Hari Raya Waisak 2570 BE", model.HolidayNational),
		hol(6, 1, "Hari Lahir Pancasila", model.HolidayNational),
		hol(6, 16, "Tahun Baru Islam 1448 H", model.HolidayNational),
		hol(8, 17, "Hari Kemerdekaan Republik Indonesia", model.HolidayNational),
		hol(8, 25, "Maulid Nabi Muhammad SAW", model.HolidayNational),
		hol(12, 24, "Cuti Bersama Natal", model.HolidayJoint),
		hol(12, 25, "Hari Raya Natal", model.HolidayNational),
	},
}

// ---------- sisa saldo akhir tahun ----------

// SettingCarryOver: bila "true", sisa saldo tahun lalu ditambahkan ke kuota tahun berikutnya (default: tidak).
const SettingCarryOver = "leave.carry_over"

// ensureBalance membuat saldo tahun ini bila belum ada. Kuota = kuota tahunan jenis cuti, ditambah sisa tahun
// lalu (kuota − terpakai, tidak negatif) bila pengaturan SettingCarryOver aktif.
func ensureBalance(r repository.Repos, empID uint64, t *model.LeaveType, year int) (*model.LeaveBalance, error) {
	if b, err := r.Balances.Find(empID, t.ID, year); err == nil {
		return b, nil
	} else if !errors.Is(err, repository.ErrNotFound) {
		return nil, err
	}
	quota := *t.AnnualQuota
	m, err := r.Settings.All()
	if err != nil {
		return nil, err
	}
	if m[SettingCarryOver] == "true" {
		prev, err := r.Balances.Find(empID, t.ID, year-1)
		switch {
		case err == nil:
			if left := prev.Quota.Sub(prev.Used); left.IsPositive() {
				quota = quota.Add(left)
			}
		case !errors.Is(err, repository.ErrNotFound):
			return nil, err
		}
	}
	return r.Balances.Ensure(empID, t.ID, year, quota)
}

type LeavePolicy struct {
	CarryOver bool `json:"carry_over"`
}

func (s *LeaveTypeService) Policy(a Actor) (*LeavePolicy, error) {
	if !a.IsHR() {
		return nil, errForbiddenData
	}
	m, err := s.store.R().Settings.All()
	if err != nil {
		return nil, err
	}
	return &LeavePolicy{CarryOver: m[SettingCarryOver] == "true"}, nil
}

// SetPolicy hanya memengaruhi saldo yang dibuat setelahnya (saldo tahun berjalan yang sudah ada tidak diubah).
func (s *LeaveTypeService) SetPolicy(a Actor, in LeavePolicy) (*LeavePolicy, error) {
	if !a.IsHR() {
		return nil, errForbiddenData
	}
	err := s.store.Tx(func(r repository.Repos) error {
		old, _ := r.Settings.All()
		if err := r.Settings.Set(SettingCarryOver, strconv.FormatBool(in.CarryOver)); err != nil {
			return err
		}
		return logActivity(r, a, AuditSettingChange, "setting", 0,
			map[string]any{SettingCarryOver: old[SettingCarryOver] == "true"}, map[string]any{SettingCarryOver: in.CarryOver})
	})
	if err != nil {
		return nil, err
	}
	return &in, nil
}
