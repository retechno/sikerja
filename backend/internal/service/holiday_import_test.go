package service_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/retechno/sikerja/backend/internal/apperr"
	"github.com/retechno/sikerja/backend/internal/model"
	"github.com/retechno/sikerja/backend/internal/service"
)

// Potongan nyata dari kalender "Hari libur di Indonesia": libur nasional, cuti bersama, tanggal tentatif,
// entri "Perayaan" (bukan libur), tanggal kembar, deskripsi dengan baris lanjutan dan escape.
const sampleICS = "BEGIN:VCALENDAR\r\nVERSION:2.0\r\n" +
	"BEGIN:VEVENT\r\nDTSTART;VALUE=DATE:20270101\r\nDTEND;VALUE=DATE:20270102\r\nDESCRIPTION:Hari libur nasional\r\nSUMMARY:Hari Tahun Baru\r\nEND:VEVENT\r\n" +
	"BEGIN:VEVENT\r\nDTSTART;VALUE=DATE:20270209\r\nDTEND;VALUE=DATE:20270210\r\nDESCRIPTION:Hari libur nasional\\nTanggal bersifat tentatif dan dapat beruba\r\n h\r\nSUMMARY:Tahun Baru Imlek\r\nEND:VEVENT\r\n" +
	"BEGIN:VEVENT\r\nDTSTART;VALUE=DATE:20270208\r\nDTEND;VALUE=DATE:20270209\r\nDESCRIPTION:Hari libur nasional\r\nSUMMARY:Cuti Bersama Tahun Baru Imlek\r\nEND:VEVENT\r\n" +
	"BEGIN:VEVENT\r\nDTSTART;VALUE=DATE:20270328\r\nDTEND;VALUE=DATE:20270329\r\nDESCRIPTION:Perayaan\\nUntuk menyembunyikan kalender perayaan\\, buka Setelan\r\nSUMMARY:Hari Paskah\r\nEND:VEVENT\r\n" +
	"BEGIN:VEVENT\r\nDTSTART;VALUE=DATE:20270817\r\nDTEND;VALUE=DATE:20270818\r\nDESCRIPTION:Hari libur nasional\r\nSUMMARY:Hari Proklamasi\\, Kemerdekaan R.I.\r\nEND:VEVENT\r\n" +
	"BEGIN:VEVENT\r\nDTSTART;VALUE=DATE:20270817\r\nDTEND;VALUE=DATE:20270818\r\nDESCRIPTION:Hari libur nasional\r\nSUMMARY:Cuti Bersama Dobel\r\nEND:VEVENT\r\n" +
	"BEGIN:VEVENT\r\nDTSTART;VALUE=DATE:20270606\r\nDTEND;VALUE=DATE:20270607\r\nDESCRIPTION:Hari libur nasional\r\nSUMMARY:Satu Muharam (belum pasti)\r\nEND:VEVENT\r\n" +
	"BEGIN:VEVENT\r\nDTSTART;VALUE=DATE:20261225\r\nDTEND;VALUE=DATE:20261226\r\nDESCRIPTION:Hari libur nasional\r\nSUMMARY:Hari Raya Natal\r\nEND:VEVENT\r\n" +
	"BEGIN:VEVENT\r\nDTSTART;VALUE=DATE:20270303\r\nDTEND;VALUE=DATE:20270306\r\nDESCRIPTION:Hari libur nasional\r\nSUMMARY:Cuti Bersama Panjang\r\nEND:VEVENT\r\n" +
	"END:VCALENDAR\r\n"

func TestParseHolidayICS(t *testing.T) {
	got := service.ParseHolidayICS(sampleICS, 2027)
	type row struct {
		date, name, typ string
		tentative       bool
	}
	var rows []row
	for _, h := range got {
		rows = append(rows, row{h.Date.Format("2006-01-02"), h.Name, h.Type, h.Tentative})
	}
	assert.Equal(t, []row{
		{"2027-01-01", "Hari Tahun Baru", model.HolidayNational, false},
		{"2027-02-08", "Cuti Bersama Tahun Baru Imlek", model.HolidayJoint, false},
		{"2027-02-09", "Tahun Baru Imlek", model.HolidayNational, true},   // baris lanjutan dilipat, "tentatif" terbaca
		{"2027-03-03", "Cuti Bersama Panjang", model.HolidayJoint, false}, // rentang 3 hari dipecah per hari
		{"2027-03-04", "Cuti Bersama Panjang", model.HolidayJoint, false},
		{"2027-03-05", "Cuti Bersama Panjang", model.HolidayJoint, false},
		{"2027-06-06", "Satu Muharam", model.HolidayNational, true},                       // akhiran "(belum pasti)" jadi penanda tentatif
		{"2027-08-17", "Hari Proklamasi, Kemerdekaan R.I.", model.HolidayNational, false}, // , di-unescape; nasional menang atas cuti bersama
	}, rows)
	// Paskah ("Perayaan") dan Natal 2026 (tahun lain) tidak ikut
	assert.Len(t, service.ParseHolidayICS(sampleICS, 2026), 1)
	assert.Empty(t, service.ParseHolidayICS("bukan kalender", 2027))
}

func TestICSHolidaySourceOverHTTP(t *testing.T) {
	ok := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(sampleICS)) }))
	defer ok.Close()
	list, err := service.NewICSHolidaySource(ok.URL).Fetch(context.Background(), 2027)
	require.NoError(t, err)
	assert.Len(t, list, 8)

	down := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(500) }))
	defer down.Close()
	_, err = service.NewICSHolidaySource(down.URL).Fetch(context.Background(), 2027)
	assert.Equal(t, 502, status(t, err))

	down.Close()
	_, err = service.NewICSHolidaySource(down.URL).Fetch(context.Background(), 2027)
	assert.Equal(t, 502, status(t, err), "server mati")
}

type fakeSource struct {
	list []service.SourceHoliday
	err  error
}

func (f fakeSource) Fetch(context.Context, int) ([]service.SourceHoliday, error) {
	return f.list, f.err
}

func TestHolidayImportPreviewThenConfirm(t *testing.T) {
	e := newLeaveEnv(t)
	hr, emp := e.w.Actor(e.w.HR), e.w.Actor(e.w.EmpIT1)
	d := func(s string) time.Time { return day(s) }
	e.hols.SetSource(fakeSource{list: []service.SourceHoliday{
		{Date: d("2027-01-01"), Name: "Hari Tahun Baru", Type: model.HolidayNational},
		{Date: d("2027-02-09"), Name: "Tahun Baru Imlek", Type: model.HolidayNational, Tentative: true},
	}})
	ctx := context.Background()

	_, err := e.hols.PreviewImport(ctx, emp, 2027)
	assert.Equal(t, 403, status(t, err), "hanya HR")
	_, err = e.hols.PreviewImport(ctx, hr, 1800)
	assert.Equal(t, 422, status(t, err))

	// libur yang sudah ada ditandai dan tidak ditimpa
	_, err = e.hols.Create(hr, service.HolidayInput{Date: d("2027-01-01"), Name: "Tahun Baru (manual)", Type: model.HolidayNational})
	require.NoError(t, err)
	prev, err := e.hols.PreviewImport(ctx, hr, 2027)
	require.NoError(t, err)
	require.Len(t, prev, 2)
	assert.True(t, prev[0].Exists)
	assert.Equal(t, "Tahun Baru (manual)", prev[0].ExistingName)
	assert.False(t, prev[1].Exists)
	assert.True(t, prev[1].Tentative)
	left, err := e.hols.List(2027)
	require.NoError(t, err)
	assert.Len(t, left, 1, "pratinjau tidak menyimpan apa pun")

	// konfirmasi: tanggal yang sudah ada dan kembar dilewati
	in := []service.HolidayInput{
		{Date: d("2027-01-01"), Name: "Hari Tahun Baru", Type: model.HolidayNational},
		{Date: d("2027-02-09"), Name: "Tahun Baru Imlek", Type: model.HolidayNational},
		{Date: d("2027-02-09"), Name: "Tahun Baru Imlek", Type: model.HolidayNational},
	}
	_, _, err = e.hols.ImportSelected(emp, in)
	assert.Equal(t, 403, status(t, err))
	added, skipped, err := e.hols.ImportSelected(hr, in)
	require.NoError(t, err)
	assert.Equal(t, 1, added)
	assert.Equal(t, 2, skipped)
	left, _ = e.hols.List(2027)
	assert.Len(t, left, 2)
	assert.Equal(t, "Tahun Baru (manual)", left[0].Name, "data HR tidak tertimpa")

	_, _, err = e.hols.ImportSelected(hr, nil)
	assert.Equal(t, 422, status(t, err))
	_, _, err = e.hols.ImportSelected(hr, []service.HolidayInput{{Date: d("2027-05-01"), Name: " ", Type: model.HolidayNational}})
	assert.Equal(t, 422, status(t, err), "nama kosong ditolak")
}

func TestHolidayImportSourceFailureAndDisabled(t *testing.T) {
	e := newLeaveEnv(t)
	hr := e.w.Actor(e.w.HR)
	_, err := e.hols.PreviewImport(context.Background(), hr, 2027)
	assert.Equal(t, 404, status(t, err), "tanpa sumber")
	e.hols.SetSource(fakeSource{err: apperr.New(502, "holiday_source_unavailable", "down")})
	_, err = e.hols.PreviewImport(context.Background(), hr, 2027)
	assert.Equal(t, 502, status(t, err))
}
