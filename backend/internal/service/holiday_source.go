package service

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/retechno/sikerja/backend/internal/apperr"
	"github.com/retechno/sikerja/backend/internal/model"
)

const (
	icsHolidayDesc  = "hari libur nasional" // baris pertama DESCRIPTION untuk entri yang benar-benar libur
	icsUnsureSuffix = " (belum pasti)"
	icsMaxBytes     = 5 << 20
	icsMaxSpanDays  = 31
)

// SourceHoliday satu hari libur dari sumber eksternal.
type SourceHoliday struct {
	Date      time.Time
	Name      string
	Type      string
	Tentative bool // sumber menandai tanggalnya belum pasti
}

// HolidaySource mengambil daftar libur satu tahun dari luar. Dibuat interface agar sumbernya bisa diganti
// (dan dipalsukan di test) tanpa menyentuh service.
type HolidaySource interface {
	Fetch(ctx context.Context, year int) ([]SourceHoliday, error)
}

// ICSHolidaySource membaca kalender iCalendar lewat HTTP(S). URL hanya dari konfigurasi server, tidak pernah dari pengguna.
type ICSHolidaySource struct {
	URL    string
	Client *http.Client
}

func NewICSHolidaySource(url string) *ICSHolidaySource {
	return &ICSHolidaySource{URL: url, Client: &http.Client{Timeout: 15 * time.Second}}
}

func (s *ICSHolidaySource) Fetch(ctx context.Context, year int) ([]SourceHoliday, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.URL, nil)
	if err != nil {
		return nil, upstreamErr(err)
	}
	resp, err := s.Client.Do(req)
	if err != nil {
		return nil, upstreamErr(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, upstreamErr(fmt.Errorf("status %d", resp.StatusCode))
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, icsMaxBytes+1))
	if err != nil {
		return nil, upstreamErr(err)
	}
	if len(body) > icsMaxBytes {
		return nil, upstreamErr(fmt.Errorf("respons terlalu besar"))
	}
	return ParseHolidayICS(string(body), year), nil
}

func upstreamErr(error) *apperr.Error {
	return apperr.New(http.StatusBadGateway, "holiday_source_unavailable",
		"Sumber hari libur sedang tidak bisa dihubungi. Coba lagi nanti atau tambahkan manual.")
}

type icsEvent map[string]string

// ParseHolidayICS mengambil libur nasional & cuti bersama untuk satu tahun. Hasil terurut menurut tanggal,
// satu entri per tanggal (bila sumber punya dua entri pada tanggal yang sama, libur nasional diutamakan).
func ParseHolidayICS(raw string, year int) []SourceHoliday {
	byDate := map[time.Time]SourceHoliday{}
	for _, ev := range icsEvents(raw) {
		desc := icsUnescape(ev["DESCRIPTION"])
		first, _, _ := strings.Cut(desc, "\n")
		if strings.ToLower(strings.TrimSpace(first)) != icsHolidayDesc {
			continue
		}
		name := strings.TrimSpace(icsUnescape(ev["SUMMARY"]))
		start, err := time.Parse("20060102", ev["DTSTART"])
		if name == "" || err != nil {
			continue
		}
		end := start.AddDate(0, 0, 1)
		if e, err := time.Parse("20060102", ev["DTEND"]); err == nil && e.After(start) {
			end = e
		}
		if r := []rune(name); len(r) > 100 {
			name = string(r[:100])
		}
		typ := model.HolidayNational
		if strings.HasPrefix(strings.ToLower(name), "cuti bersama") {
			typ = model.HolidayJoint
		}
		tentative := strings.Contains(strings.ToLower(desc), "tentatif")
		if base, found := strings.CutSuffix(name, icsUnsureSuffix); found { // "… (belum pasti)": penanda sudah ada di flag
			name, tentative = strings.TrimSpace(base), true
		}
		for d, n := start, 0; d.Before(end) && n < icsMaxSpanDays; d, n = d.AddDate(0, 0, 1), n+1 {
			if d.Year() != year {
				continue
			}
			h := SourceHoliday{Date: DateOnly(d), Name: name, Type: typ, Tentative: tentative}
			if old, dup := byDate[h.Date]; dup && (old.Type != model.HolidayJoint || h.Type != model.HolidayNational) {
				continue
			}
			byDate[h.Date] = h
		}
	}
	out := make([]SourceHoliday, 0, len(byDate))
	for _, h := range byDate {
		out = append(out, h)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Date.Before(out[j].Date) })
	return out
}

// icsEvents memecah isi iCalendar menjadi event {NAMA: nilai}; parameter properti (";VALUE=DATE") dibuang.
func icsEvents(raw string) []icsEvent {
	raw = strings.ReplaceAll(strings.ReplaceAll(raw, "\r\n", "\n"), "\r", "\n")
	var lines []string
	for _, l := range strings.Split(raw, "\n") { // lipat baris lanjutan (diawali spasi/tab)
		if (strings.HasPrefix(l, " ") || strings.HasPrefix(l, "\t")) && len(lines) > 0 {
			lines[len(lines)-1] += l[1:]
			continue
		}
		lines = append(lines, l)
	}
	var out []icsEvent
	var cur icsEvent
	for _, l := range lines {
		switch {
		case l == "BEGIN:VEVENT":
			cur = icsEvent{}
		case l == "END:VEVENT":
			if cur != nil {
				out = append(out, cur)
			}
			cur = nil
		case cur != nil:
			k, v, ok := strings.Cut(l, ":")
			if !ok {
				continue
			}
			k, _, _ = strings.Cut(k, ";")
			if _, seen := cur[k]; !seen {
				cur[k] = v
			}
		}
	}
	return out
}

func icsUnescape(s string) string {
	return strings.NewReplacer(`\n`, "\n", `\N`, "\n", `\,`, ",", `\;`, ";", `\\`, `\`).Replace(s)
}
