package service

import (
	"time"

	"github.com/shopspring/decimal"

	"github.com/retechno/sikerja/backend/internal/apperr"
	"github.com/retechno/sikerja/backend/internal/repository"
)

// maxLeaveSpanDays membatasi rentang tanggal yang dihitung (mencegah permintaan berlebihan).
const maxLeaveSpanDays = 400

var (
	decHalf = decimal.RequireFromString("0.5")
	decOne  = decimal.NewFromInt(1)
)

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
	SkipWeekend = "weekend"
	SkipHoliday = "holiday"
)

// DayInfo menjelaskan satu tanggal dalam rentang: dihitung atau tidak, dan alasannya.
type DayInfo struct {
	Date    string `json:"date"`
	Weekday string `json:"weekday"` // Sen, Sel, ...
	Counted bool   `json:"counted"`
	Skip    string `json:"skip,omitempty"`
	Holiday string `json:"holiday,omitempty"`
}

// WorkingDays adalah hasil perhitungan hari kerja untuk sebuah rentang.
type WorkingDays struct {
	Days        decimal.Decimal `json:"days"`
	Breakdown   []DayInfo       `json:"breakdown"`
	Skipped     []DayInfo       `json:"skipped"`
	HalfDay     bool            `json:"half_day"`
	StartDate   string          `json:"start_date"`
	EndDate     string          `json:"end_date"`
	CalendarDay int             `json:"calendar_days"`
}

var weekdayShort = [...]string{"Min", "Sen", "Sel", "Rab", "Kam", "Jum", "Sab"}

// CountWorkingDays menghitung hari kerja antara start dan end (inklusif): Sabtu, Minggu, dan tanggal pada
// holidays (tanggal → nama) tidak dihitung. Setengah hari hanya berlaku untuk satu tanggal dan bernilai 0,5.
func CountWorkingDays(start, end time.Time, holidays map[string]string, half bool) WorkingDays {
	start, end = DateOnly(start), DateOnly(end)
	out := WorkingDays{Days: decimal.Zero, HalfDay: half, StartDate: start.Format("2006-01-02"), EndDate: end.Format("2006-01-02")}
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		key := d.Format("2006-01-02")
		info := DayInfo{Date: key, Weekday: weekdayShort[d.Weekday()], Counted: true}
		switch {
		case d.Weekday() == time.Saturday || d.Weekday() == time.Sunday:
			info.Counted, info.Skip = false, SkipWeekend
		case holidays[key] != "":
			info.Counted, info.Skip, info.Holiday = false, SkipHoliday, holidays[key]
		}
		out.CalendarDay++
		out.Breakdown = append(out.Breakdown, info)
		if info.Counted {
			out.Days = out.Days.Add(decOne)
		} else {
			out.Skipped = append(out.Skipped, info)
		}
	}
	if half {
		if out.Days.IsPositive() && start.Equal(end) {
			out.Days = decHalf
		} else {
			out.Days = decimal.Zero
		}
	}
	return out
}

// WorkingDayService menghitung hari kerja memakai tabel holidays.
type WorkingDayService struct{ store repository.Store }

func NewWorkingDayService(store repository.Store) *WorkingDayService {
	return &WorkingDayService{store: store}
}

// Count menghitung hari kerja; error validasi bila tanggal terbalik atau rentang terlalu panjang.
func (s *WorkingDayService) Count(r repository.Repos, start, end time.Time, half bool) (*WorkingDays, error) {
	start, end = DateOnly(start), DateOnly(end)
	if end.Before(start) {
		return nil, apperr.Validation("Tanggal selesai tidak boleh sebelum tanggal mulai",
			map[string]string{"end_date": "Tanggal selesai tidak boleh sebelum tanggal mulai"})
	}
	if end.Sub(start) > maxLeaveSpanDays*24*time.Hour {
		return nil, apperr.Validation("Rentang tanggal terlalu panjang", map[string]string{"end_date": "Maksimal 400 hari"})
	}
	if half && !start.Equal(end) {
		return nil, apperr.Validation("Setengah hari hanya untuk satu tanggal",
			map[string]string{"end_date": "Setengah hari hanya berlaku untuk satu tanggal"})
	}
	hs, err := r.Holidays.InRange(start, end)
	if err != nil {
		return nil, err
	}
	m := make(map[string]string, len(hs))
	for _, h := range hs {
		m[h.Date.Format("2006-01-02")] = h.Name
	}
	res := CountWorkingDays(start, end, m, half)
	return &res, nil
}

// Preview: perhitungan live untuk form pengajuan (tanpa menyimpan apa pun).
func (s *WorkingDayService) Preview(start, end time.Time, half bool) (*WorkingDays, error) {
	return s.Count(s.store.R(), start, end, half)
}
