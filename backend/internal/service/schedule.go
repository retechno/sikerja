package service

import (
	"fmt"
	"strings"
	"time"

	"github.com/retechno/sikerja/backend/internal/apperr"
	"github.com/retechno/sikerja/backend/internal/model"
	"github.com/retechno/sikerja/backend/internal/repository"
)

const AuditScheduleChange = "work_schedule.change"

// DayHours adalah jam kerja satu hari dalam menit sejak 00:00.
type DayHours struct{ Start, End int }

func (d DayHours) Minutes() int { return d.End - d.Start }

func (d DayHours) String() string { return fmtHM(d.Start) + "–" + fmtHM(d.End) }

// Break adalah jam istirahat (menit sejak 00:00); tidak dihitung sebagai jam kerja.
type Break struct{ Start, End int }

// Schedule adalah jadwal kerja mingguan; hari yang tidak ada di Days = libur mingguan.
type Schedule struct {
	ID    uint64
	Name  string
	Days  map[time.Weekday]DayHours
	Break *Break
}

func overlapMinutes(a1, a2, b1, b2 int) int {
	lo, hi := max(a1, b1), min(a2, b2)
	if hi > lo {
		return hi - lo
	}
	return 0
}

// Net: menit kerja efektif antara a dan b (menit sejak 00:00), tanpa jam istirahat.
func (s Schedule) Net(a, b int) int {
	if b <= a {
		return 0
	}
	n := b - a
	if s.Break != nil {
		n -= overlapMinutes(a, b, s.Break.Start, s.Break.End)
	}
	return n
}

// NetDay: menit kerja efektif satu hari penuh (jam kerja dikurangi istirahat).
func (s Schedule) NetDay(h DayHours) int { return s.Net(h.Start, h.End) }

// Halves membagi hari kerja efektif menjadi dua sama panjang: paruh pertama berakhir di morningEnd,
// paruh kedua mulai di afternoonStart (setelah istirahat bila batas jatuh tepat di awal istirahat).
func (s Schedule) Halves(h DayHours) (morningEnd, afternoonStart int) {
	half := s.NetDay(h) / 2
	t := h.Start
	for t < h.End && s.Net(h.Start, t) < half {
		t++
	}
	if s.Break != nil && t == s.Break.Start {
		return t, s.Break.End
	}
	return t, t
}

func (s Schedule) Hours(w time.Weekday) (DayHours, bool) {
	h, ok := s.Days[w]
	return h, ok
}

// DefaultSchedule dipakai bila tabel jadwal belum berisi default (Senin–Jumat 08.00–17.00).
var DefaultSchedule = func() Schedule {
	s := Schedule{Name: "Jam kantor (Sen-Jum 08.00-17.00)", Days: map[time.Weekday]DayHours{}, Break: &Break{Start: 12 * 60, End: 13 * 60}}
	for d := time.Monday; d <= time.Friday; d++ {
		s.Days[d] = DayHours{Start: 8 * 60, End: 17 * 60}
	}
	return s
}()

func ScheduleFromModel(m *model.WorkSchedule) Schedule {
	s := Schedule{ID: m.ID, Name: m.Name, Days: map[time.Weekday]DayHours{}}
	for _, d := range m.Days {
		a, err1 := parseHM(d.StartTime)
		b, err2 := parseHM(d.EndTime)
		if err1 == nil && err2 == nil && b > a && d.Weekday >= 0 && d.Weekday <= 6 {
			s.Days[time.Weekday(d.Weekday)] = DayHours{Start: a, End: b}
		}
	}
	bs, e1 := parseHM(m.BreakStart)
	be, e2 := parseHM(m.BreakEnd)
	if e1 == nil && e2 == nil && be > bs {
		s.Break = &Break{Start: bs, End: be}
	}
	return s
}

// parseHM membaca "HH:MM" (juga "H:MM") menjadi menit sejak tengah malam.
func parseHM(s string) (int, error) {
	t, err := time.Parse("15:04", strings.TrimSpace(s))
	if err != nil {
		return 0, fmt.Errorf("jam tidak valid: %q", s)
	}
	return t.Hour()*60 + t.Minute(), nil
}

func fmtHM(min int) string { return fmt.Sprintf("%02d:%02d", min/60, min%60) }

// scheduleFor memilih jadwal kerja karyawan: jadwal departemennya, atau default perusahaan.
// (Jadwal per karyawan/shift dapat ditambahkan di sini tanpa mengubah pemanggil.)
func scheduleFor(r repository.Repos, e *model.Employee) (Schedule, error) {
	if e != nil && e.Department != nil && e.Department.ScheduleID != nil {
		m, err := r.Schedules.ByID(*e.Department.ScheduleID)
		if err == nil {
			return ScheduleFromModel(m), nil
		}
		if err != repository.ErrNotFound {
			return Schedule{}, err
		}
	}
	m, err := r.Schedules.Default()
	if err != nil {
		if err == repository.ErrNotFound {
			return DefaultSchedule, nil
		}
		return Schedule{}, err
	}
	return ScheduleFromModel(m), nil
}

// ---------- administrasi jadwal (H-16) ----------

type ScheduleService struct{ store repository.Store }

func NewScheduleService(store repository.Store) *ScheduleService {
	return &ScheduleService{store: store}
}

type ScheduleDayInput struct {
	Weekday   int    `json:"weekday"`
	StartTime string `json:"start_time"`
	EndTime   string `json:"end_time"`
}

type ScheduleInput struct {
	Name       string             `json:"name"`
	IsDefault  bool               `json:"is_default"`
	BreakStart string             `json:"break_start"` // jam istirahat "HH:MM"; kosong = tanpa istirahat
	BreakEnd   string             `json:"break_end"`
	Days       []ScheduleDayInput `json:"days"`
}

func (in *ScheduleInput) validate() ([]model.WorkScheduleDay, error) {
	f := map[string]string{}
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		f["name"] = "Nama jadwal wajib diisi"
	}
	if len(in.Days) == 0 {
		f["days"] = "Pilih minimal satu hari kerja"
	}
	in.BreakStart, in.BreakEnd = strings.TrimSpace(in.BreakStart), strings.TrimSpace(in.BreakEnd)
	if in.BreakStart != "" || in.BreakEnd != "" {
		bs, e1 := parseHM(in.BreakStart)
		be, e2 := parseHM(in.BreakEnd)
		switch {
		case e1 != nil || e2 != nil:
			f["break_start"] = "Isi jam mulai dan selesai istirahat (HH:MM) atau kosongkan keduanya"
		case be <= bs:
			f["break_start"] = "Jam selesai istirahat harus setelah jam mulai"
		default:
			in.BreakStart, in.BreakEnd = fmtHM(bs), fmtHM(be)
		}
	}
	seen := map[int]bool{}
	var days []model.WorkScheduleDay
	for _, d := range in.Days {
		a, e1 := parseHM(d.StartTime)
		b, e2 := parseHM(d.EndTime)
		switch {
		case d.Weekday < 0 || d.Weekday > 6:
			f["days"] = "Hari tidak valid"
		case seen[d.Weekday]:
			f["days"] = "Satu hari hanya boleh terdaftar sekali"
		case e1 != nil || e2 != nil:
			f["days"] = "Jam harus berformat HH:MM"
		case b <= a:
			f["days"] = "Jam selesai harus setelah jam mulai"
		case b-a < 60:
			f["days"] = "Jam kerja minimal 1 jam per hari"
		default:
			seen[d.Weekday] = true
			days = append(days, model.WorkScheduleDay{Weekday: d.Weekday, StartTime: fmtHM(a), EndTime: fmtHM(b)})
		}
	}
	if len(f) > 0 {
		return nil, apperr.Validation("Jadwal kerja tidak valid", f)
	}
	return days, nil
}

func (s *ScheduleService) List(a Actor) ([]model.WorkSchedule, error) {
	if !a.IsHR() && !a.IsDirector() {
		return nil, errForbiddenData
	}
	out, err := s.store.R().Schedules.List()
	if out == nil {
		out = []model.WorkSchedule{}
	}
	return out, err
}

func (s *ScheduleService) save(a Actor, id uint64, in ScheduleInput) (*model.WorkSchedule, error) {
	if err := requireHR(a); err != nil {
		return nil, err
	}
	days, err := in.validate()
	if err != nil {
		return nil, err
	}
	var out *model.WorkSchedule
	err = s.store.Tx(func(r repository.Repos) error {
		m := &model.WorkSchedule{}
		var old any
		if id != 0 {
			cur, err := r.Schedules.ByID(id)
			if err != nil {
				if err == repository.ErrNotFound {
					return apperr.NotFound("Jadwal tidak ditemukan")
				}
				return err
			}
			m, old = cur, *cur
			if cur.IsDefault && !in.IsDefault {
				return fieldErr("Harus ada satu jadwal default; jadikan jadwal lain default lebih dulu", "is_default")
			}
		}
		if taken, err := r.Schedules.NameTaken(in.Name, id); err != nil {
			return err
		} else if taken {
			return fieldErr("Nama jadwal sudah dipakai", "name")
		}
		m.Name, m.IsDefault, m.Days = in.Name, in.IsDefault, days
		m.BreakStart, m.BreakEnd = in.BreakStart, in.BreakEnd
		if id == 0 && !in.IsDefault {
			if _, err := r.Schedules.Default(); err == repository.ErrNotFound {
				m.IsDefault = true // jadwal pertama otomatis menjadi default
			}
		}
		if err := r.Schedules.Save(m); err != nil {
			return err
		}
		if m.IsDefault {
			if err := r.Schedules.ClearDefault(m.ID); err != nil {
				return err
			}
		}
		out = m
		return logActivity(r, a, AuditScheduleChange, "work_schedule", m.ID, old, m)
	})
	return out, err
}

func (s *ScheduleService) Create(a Actor, in ScheduleInput) (*model.WorkSchedule, error) {
	return s.save(a, 0, in)
}

func (s *ScheduleService) Update(a Actor, id uint64, in ScheduleInput) (*model.WorkSchedule, error) {
	return s.save(a, id, in)
}

func (s *ScheduleService) Delete(a Actor, id uint64) error {
	if err := requireHR(a); err != nil {
		return err
	}
	return s.store.Tx(func(r repository.Repos) error {
		m, err := r.Schedules.ByID(id)
		if err != nil {
			if err == repository.ErrNotFound {
				return apperr.NotFound("Jadwal tidak ditemukan")
			}
			return err
		}
		if m.IsDefault {
			return apperr.Conflict("Jadwal default tidak bisa dihapus; jadikan jadwal lain default lebih dulu")
		}
		if n, err := r.Schedules.DepartmentCount(id); err != nil {
			return err
		} else if n > 0 {
			return apperr.Conflict(fmt.Sprintf("Jadwal masih dipakai %d departemen; pindahkan departemennya lebih dulu", n))
		}
		if err := r.Schedules.Delete(id); err != nil {
			return err
		}
		return logActivity(r, a, AuditScheduleChange, "work_schedule", id, m, nil)
	})
}
