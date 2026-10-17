package service

import (
	"time"

	"github.com/shopspring/decimal"

	"github.com/retechno/sikerja/backend/internal/apperr"
	"github.com/retechno/sikerja/backend/internal/repository"
)

// maxLeaveSpanDays membatasi rentang tanggal yang dihitung (mencegah permintaan berlebihan).
const maxLeaveSpanDays = 400

var decOne = decimal.NewFromInt(1)

// DateOnly membuang jam dan zona: hanya tahun-bulan-tanggal yang dipakai (disimpan sebagai UTC 00:00).
func DateOnly(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// ParseDate membaca "YYYY-MM-DD".
func ParseDate(s string) (time.Time, error) {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return time.Time{}, err
	}
	return DateOnly(t), nil
}

// Alasan sebuah tanggal tidak dihitung sebagai hari kerja.
const (
	SkipWeekend = "weekend" // hari yang bukan hari kerja menurut jadwal (mis. Sabtu/Minggu)
	SkipHoliday = "holiday"
)

// DayInfo menjelaskan satu tanggal dalam rentang: dihitung atau tidak, jam kerjanya, dan alasan bila tidak.
type DayInfo struct {
	Date    string `json:"date"`
	Weekday string `json:"weekday"` // Sen, Sel, ...
	Counted bool   `json:"counted"`
	Hours   string `json:"hours,omitempty"` // jam kerja hari itu, mis. "09:00–15:00" (hanya hari kerja)
	Minutes int    `json:"minutes,omitempty"`
	Skip    string `json:"skip,omitempty"`
	Holiday string `json:"holiday,omitempty"`
}

// Partial adalah izin sebagian hari: jam mulai dan selesai (menit sejak 00:00) pada satu tanggal.
type Partial struct{ From, To int }

func (p Partial) Minutes() int { return p.To - p.From }

// WorkingDays adalah hasil perhitungan hari kerja untuk sebuah rentang.
type WorkingDays struct {
	Days           decimal.Decimal `json:"days"`
	Breakdown      []DayInfo       `json:"breakdown"`
	Skipped        []DayInfo       `json:"skipped"`
	HalfDay        bool            `json:"half_day"`                  // izin sebagian hari
	PartialMinutes int             `json:"partial_minutes,omitempty"` // menit kerja efektif izin (tanpa istirahat)
	DayMinutes     int             `json:"day_minutes,omitempty"`     // menit kerja efektif hari itu (tanpa istirahat)
	Break          string          `json:"break,omitempty"`           // jam istirahat, mis. "12:00–13:00"
	Morning        string          `json:"morning,omitempty"`         // paruh pertama hari kerja (rentang jam), untuk satu tanggal
	Afternoon      string          `json:"afternoon,omitempty"`
	Schedule       string          `json:"schedule"`
	StartDate      string          `json:"start_date"`
	EndDate        string          `json:"end_date"`
	CalendarDay    int             `json:"calendar_days"`
}

var weekdayShort = [...]string{"Min", "Sen", "Sel", "Rab", "Kam", "Jum", "Sab"}

// CountWorkingDaysFor menghitung hari kerja antara start dan end (inklusif) menurut jadwal sch: hari di luar jadwal
// (mis. Sabtu/Minggu) dan tanggal pada holidays (tanggal → nama) tidak dihitung.
// Bila partial diberikan, rentang harus satu hari kerja dan hasilnya = lama izin / jam kerja hari itu (2 desimal);
// jam di luar jam kerja hari itu atau rentang lebih dari satu hari menghasilkan 0.
func CountWorkingDaysFor(sch Schedule, start, end time.Time, holidays map[string]string, partial *Partial) WorkingDays {
	start, end = DateOnly(start), DateOnly(end)
	out := WorkingDays{
		Days: decimal.Zero, Schedule: sch.Name, HalfDay: partial != nil,
		Breakdown: []DayInfo{}, Skipped: []DayInfo{}, // jangan null di JSON
		StartDate: start.Format("2006-01-02"), EndDate: end.Format("2006-01-02"),
	}
	var hours DayHours
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		key := d.Format("2006-01-02")
		info := DayInfo{Date: key, Weekday: weekdayShort[d.Weekday()], Counted: true}
		h, works := sch.Hours(d.Weekday())
		switch {
		case !works:
			info.Counted, info.Skip = false, SkipWeekend
		case holidays[key] != "":
			info.Counted, info.Skip, info.Holiday = false, SkipHoliday, holidays[key]
		default:
			info.Hours, info.Minutes = h.String(), sch.NetDay(h)
			hours = h
		}
		out.CalendarDay++
		out.Breakdown = append(out.Breakdown, info)
		if info.Counted {
			out.Days = out.Days.Add(decOne)
		} else {
			out.Skipped = append(out.Skipped, info)
		}
	}
	if sch.Break != nil {
		out.Break = DayHours{Start: sch.Break.Start, End: sch.Break.End}.String()
	}
	if start.Equal(end) && out.Days.Equal(decOne) {
		me, as := sch.Halves(hours)
		out.Morning = DayHours{Start: hours.Start, End: me}.String()
		out.Afternoon = DayHours{Start: as, End: hours.End}.String()
	}
	if partial != nil {
		out.PartialMinutes, out.DayMinutes = sch.Net(partial.From, partial.To), sch.NetDay(hours)
		ok := out.Days.Equal(decOne) && start.Equal(end) && partial.From >= hours.Start && partial.To <= hours.End &&
			out.PartialMinutes > 0 && out.DayMinutes > 0
		if ok {
			out.Days = decimal.NewFromInt(int64(out.PartialMinutes)).Div(decimal.NewFromInt(int64(out.DayMinutes))).Round(2)
		} else {
			out.Days = decimal.Zero
		}
	}
	return out
}

// CountWorkingDays: hitung dengan jadwal bawaan (Sen–Jum 08.00–17.00). half = setengah hari pertama (0,5).
func CountWorkingDays(start, end time.Time, holidays map[string]string, half bool) WorkingDays {
	var p *Partial
	if half {
		h := DefaultSchedule.Days[time.Monday]
		me, _ := DefaultSchedule.Halves(h)
		p = &Partial{From: h.Start, To: me}
	}
	return CountWorkingDaysFor(DefaultSchedule, start, end, holidays, p)
}

// WorkingDayService menghitung hari kerja memakai tabel holidays dan jadwal kerja.
type WorkingDayService struct{ store repository.Store }

func NewWorkingDayService(store repository.Store) *WorkingDayService {
	return &WorkingDayService{store: store}
}

// Count menghitung hari kerja; error validasi bila tanggal terbalik, rentang terlalu panjang,
// atau izin sebagian hari tidak berada dalam satu tanggal.
func (s *WorkingDayService) Count(r repository.Repos, sch Schedule, start, end time.Time, partial *Partial) (*WorkingDays, error) {
	start, end = DateOnly(start), DateOnly(end)
	if end.Before(start) {
		return nil, apperr.Validation("Tanggal selesai tidak boleh sebelum tanggal mulai",
			map[string]string{"end_date": "Tanggal selesai tidak boleh sebelum tanggal mulai"})
	}
	if end.Sub(start) > maxLeaveSpanDays*24*time.Hour {
		return nil, apperr.Validation("Rentang tanggal terlalu panjang", map[string]string{"end_date": "Maksimal 400 hari"})
	}
	if partial != nil && !start.Equal(end) {
		return nil, apperr.Validation("Izin sebagian hari hanya untuk satu tanggal",
			map[string]string{"end_date": "Izin sebagian hari hanya berlaku untuk satu tanggal"})
	}
	hs, err := r.Holidays.InRange(start, end)
	if err != nil {
		return nil, err
	}
	m := make(map[string]string, len(hs))
	for _, h := range hs {
		m[h.Date.Format("2006-01-02")] = h.Name
	}
	res := CountWorkingDaysFor(sch, start, end, m, partial)
	return &res, nil
}

// Preview: perhitungan tanpa menyimpan, untuk jadwal dan rentang tertentu.
func (s *WorkingDayService) Preview(sch Schedule, start, end time.Time, partial *Partial) (*WorkingDays, error) {
	return s.Count(s.store.R(), sch, start, end, partial)
}
