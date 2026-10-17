package service_test

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/retechno/sikerja/backend/internal/apperr"
	"github.com/retechno/sikerja/backend/internal/model"
	"github.com/retechno/sikerja/backend/internal/repository"
	"github.com/retechno/sikerja/backend/internal/service"
	"github.com/retechno/sikerja/backend/internal/storage"
	"github.com/retechno/sikerja/backend/internal/testutil"
)

// recNotifier mencatat notifikasi tanpa menyentuh DB.
type recNotifier struct {
	mu    sync.Mutex
	items []recNotice
}

type recNotice struct {
	Notice service.Notice
	To     []string // nama penerima
}

func (r *recNotifier) Notify(_ context.Context, n service.Notice, to ...model.Employee) {
	r.mu.Lock()
	defer r.mu.Unlock()
	names := make([]string, 0, len(to))
	for _, e := range to {
		names = append(names, e.Name)
	}
	r.items = append(r.items, recNotice{n, names})
}

func (r *recNotifier) last(event string) *recNotice {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := len(r.items) - 1; i >= 0; i-- {
		if r.items[i].Notice.Type == event {
			return &r.items[i]
		}
	}
	return nil
}

func (r *recNotifier) count(event string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, it := range r.items {
		if it.Notice.Type == event {
			n++
		}
	}
	return n
}

type leaveEnv struct {
	w     *testutil.World
	svc   *service.LeaveService
	types *service.LeaveTypeService
	hols  *service.HolidayService
	rec   *recNotifier
	ct    *model.LeaveType // cuti tahunan 12, masa kerja >= 12 bln
	cs    *model.LeaveType // sakit, lampiran jika > 1 hari
	iz    *model.LeaveType // izin 6, boleh setengah hari
	dl    *model.LeaveType // dinas luar
	ck    *model.LeaveType // cuti menikah maks 3 hari
}

// "Hari ini" di test: Senin, 19 Okt 2026.
var today = time.Date(2026, 10, 19, 9, 0, 0, 0, time.UTC)

func newLeaveEnv(t *testing.T) *leaveEnv {
	t.Helper()
	w := testutil.NewWorld(t)
	files, err := storage.NewLocal(w.Cfg.StorageDir)
	require.NoError(t, err)
	rec := &recNotifier{}
	wd := service.NewWorkingDayService(w.Store)
	svc := service.NewLeaveService(w.Store, files, wd, rec)
	svc.SetClock(func() time.Time { return today })
	e := &leaveEnv{w: w, svc: svc, types: service.NewLeaveTypeService(w.Store), hols: service.NewHolidayService(w.Store), rec: rec}
	mk := func(t0 model.LeaveType) *model.LeaveType {
		t0.IsActive = true
		require.NoError(t, w.Store.R().LeaveTypes.Create(&t0))
		return &t0
	}
	q := func(s string) *decimal.Decimal { v := dd(s); return &v }
	e.ct = mk(model.LeaveType{Name: "Cuti tahunan", Code: "CT", AnnualQuota: q("12"), DeductsBalance: true, MinTenureMonths: 12})
	e.cs = mk(model.LeaveType{Name: "Cuti sakit", Code: "CS", RequiresAttach: true, AttachMinDays: dd("1")})
	e.iz = mk(model.LeaveType{Name: "Izin", Code: "IZ", AnnualQuota: q("6"), DeductsBalance: true, AllowHalfDay: true})
	e.dl = mk(model.LeaveType{Name: "Dinas luar", Code: "DL"})
	e.ck = mk(model.LeaveType{Name: "Cuti menikah", Code: "CK-N", MaxDaysPerRequest: q("3")})
	return e
}

func day(s string) time.Time {
	d, err := service.ParseDate(s)
	if err != nil {
		panic(err)
	}
	return d
}

func (e *leaveEnv) input(typ *model.LeaveType, start, end string) service.LeaveInput {
	return service.LeaveInput{LeaveTypeID: typ.ID, StartDate: day(start), EndDate: day(end), Reason: "Keperluan keluarga"}
}

func (e *leaveEnv) submit(t *testing.T, who *model.Employee, in service.LeaveInput) *service.LeaveView {
	t.Helper()
	v, err := e.svc.Submit(context.Background(), e.w.Actor(who), in)
	require.NoError(t, err)
	return v
}

func (e *leaveEnv) approve(t *testing.T, who *model.Employee, id uint64) *service.LeaveView {
	t.Helper()
	v, err := e.svc.Approve(context.Background(), e.w.Actor(who), id, "")
	require.NoError(t, err)
	return v
}

func (e *leaveEnv) used(t *testing.T, who *model.Employee, typ *model.LeaveType) decimal.Decimal {
	t.Helper()
	b, err := e.w.Store.R().Balances.Ensure(who.ID, typ.ID, 2026, *typ.AnnualQuota)
	require.NoError(t, err)
	return b.Used
}

func (e *leaveEnv) holiday(t *testing.T, date, name string) {
	t.Helper()
	require.NoError(t, e.w.Store.R().Holidays.Create(&model.Holiday{Date: day(date), Name: name, Type: model.HolidayNational}))
}

// ---------- hari kerja ----------

func TestCountWorkingDays(t *testing.T) {
	hol := map[string]string{"2026-10-28": "Libur contoh", "2026-11-02": "Libur contoh 2"}
	tests := []struct {
		name       string
		start, end string
		half       bool
		want       string
	}{
		{"satu hari kerja", "2026-10-20", "2026-10-20", false, "1"},
		{"seminggu penuh Senin-Jumat", "2026-10-19", "2026-10-23", false, "5"},
		{"Jumat sampai Selasa melewati akhir pekan", "2026-10-23", "2026-10-27", false, "3"},
		{"hanya akhir pekan", "2026-10-24", "2026-10-25", false, "0"},
		{"libur nasional di tengah rentang", "2026-10-27", "2026-10-29", false, "2"},
		{"libur di ujung awal dan akhir", "2026-10-28", "2026-10-28", false, "0"},
		{"akhir pekan + libur Senin", "2026-10-30", "2026-11-03", false, "2"},
		{"lintas bulan", "2026-10-29", "2026-11-04", false, "4"},
		{"setengah hari di hari kerja", "2026-10-20", "2026-10-20", true, "0.5"},
		{"setengah hari di hari Sabtu", "2026-10-24", "2026-10-24", true, "0"},
		{"setengah hari di hari libur", "2026-10-28", "2026-10-28", true, "0"},
		{"setengah hari tapi rentang lebih dari satu hari", "2026-10-20", "2026-10-21", true, "0"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := service.CountWorkingDays(day(tc.start), day(tc.end), hol, tc.half)
			assert.True(t, dd(tc.want).Equal(got.Days), "diharapkan %s, dapat %s", tc.want, got.Days)
		})
	}
}

func TestCountWorkingDaysBreakdown(t *testing.T) {
	got := service.CountWorkingDays(day("2026-10-30"), day("2026-11-02"), map[string]string{"2026-11-02": "Libur"}, false)
	require.Len(t, got.Breakdown, 4)
	assert.Equal(t, 4, got.CalendarDay)
	assert.True(t, got.Days.Equal(dd("1")))
	require.Len(t, got.Skipped, 3)
	assert.Equal(t, service.SkipWeekend, got.Skipped[0].Skip)
	assert.Equal(t, "Sab", got.Skipped[0].Weekday)
	assert.Equal(t, service.SkipHoliday, got.Skipped[2].Skip)
	assert.Equal(t, "Libur", got.Skipped[2].Holiday)
}

func TestWorkingDayServiceUsesHolidayTable(t *testing.T) {
	e := newLeaveEnv(t)
	wd := service.NewWorkingDayService(e.w.Store)
	before, err := wd.Preview(service.DefaultSchedule, day("2026-10-26"), day("2026-10-30"), nil)
	require.NoError(t, err)
	assert.True(t, before.Days.Equal(dd("5")))

	e.holiday(t, "2026-10-28", "Libur contoh")
	after, err := wd.Preview(service.DefaultSchedule, day("2026-10-26"), day("2026-10-30"), nil)
	require.NoError(t, err)
	assert.True(t, after.Days.Equal(dd("4")))

	_, err = wd.Preview(service.DefaultSchedule, day("2026-10-30"), day("2026-10-26"), nil)
	assert.Equal(t, 422, status(t, err), "tanggal terbalik")
	_, err = wd.Preview(service.DefaultSchedule, day("2026-10-26"), day("2026-10-27"), &service.Partial{From: 480, To: 600})
	assert.Equal(t, 422, status(t, err), "setengah hari > 1 tanggal")
	_, err = wd.Preview(service.DefaultSchedule, day("2026-01-01"), day("2027-12-31"), nil)
	assert.Equal(t, 422, status(t, err), "rentang terlalu panjang")
}

// ---------- validasi pengajuan ----------

func TestSubmitValidation(t *testing.T) {
	e := newLeaveEnv(t)
	rina := e.w.EmpIT1
	e.holiday(t, "2026-10-28", "Libur contoh")

	// pengajuan awal agar ada yang bisa ditabrak: Rabu 21 - Kamis 22 Okt
	first := e.submit(t, rina, e.input(e.ct, "2026-10-21", "2026-10-22"))
	require.NotNil(t, first.Number)
	assert.Equal(t, model.LeavePendingManager, first.Status)
	assert.True(t, first.Days.Equal(dd("2")))

	newbie := e.w.EmpIT2
	newbie.JoinDate = day("2026-05-01") // < 12 bulan
	require.NoError(t, e.w.Store.R().Employees.Update(newbie))

	cases := []struct {
		name  string
		who   *model.Employee
		in    func() service.LeaveInput
		field string
	}{
		{"tumpang tindih sebagian", rina, func() service.LeaveInput { return e.input(e.dl, "2026-10-22", "2026-10-23") }, "start_date"},
		{"tumpang tindih penuh", rina, func() service.LeaveInput { return e.input(e.cs, "2026-10-21", "2026-10-21") }, "start_date"},
		{"hanya akhir pekan", rina, func() service.LeaveInput { return e.input(e.dl, "2026-10-24", "2026-10-25") }, "start_date"},
		{"hanya hari libur", rina, func() service.LeaveInput { return e.input(e.dl, "2026-10-28", "2026-10-28") }, "start_date"},
		{"saldo kurang (13 hari kerja)", rina, func() service.LeaveInput { return e.input(e.ct, "2026-11-02", "2026-11-18") }, "end_date"},
		{"tanggal terbalik", rina, func() service.LeaveInput { return e.input(e.dl, "2026-11-05", "2026-11-04") }, "end_date"},
		{"terlalu jauh ke belakang", rina, func() service.LeaveInput { return e.input(e.dl, "2026-10-01", "2026-10-02") }, "start_date"},
		{"alasan kosong", rina, func() service.LeaveInput {
			in := e.input(e.dl, "2026-11-05", "2026-11-05")
			in.Reason = "  "
			return in
		}, "reason"},
		{"cuti tahunan setengah hari tidak diizinkan", rina, func() service.LeaveInput {
			in := e.input(e.ct, "2026-11-05", "2026-11-05")
			in.HalfDay = true
			return in
		}, "half_day"},
		{"cuti sakit > 1 hari wajib lampiran", rina, func() service.LeaveInput { return e.input(e.cs, "2026-11-05", "2026-11-06") }, "attachment"},
		{"melebihi batas hari per pengajuan", rina, func() service.LeaveInput { return e.input(e.ck, "2026-11-02", "2026-11-06") }, "end_date"},
		{"masa kerja belum 12 bulan", newbie, func() service.LeaveInput { return e.input(e.ct, "2026-11-05", "2026-11-05") }, "leave_type_id"},
		{"melintasi pergantian tahun", rina, func() service.LeaveInput { return e.input(e.ct, "2026-12-30", "2027-01-04") }, "end_date"},
		{"pengganti diri sendiri", rina, func() service.LeaveInput {
			in := e.input(e.dl, "2026-11-05", "2026-11-05")
			in.SubstituteID = &rina.ID
			return in
		}, "substitute_id"},
		{"pengganti beda departemen", rina, func() service.LeaveInput {
			in := e.input(e.dl, "2026-11-05", "2026-11-05")
			in.SubstituteID = &e.w.EmpCS1.ID
			return in
		}, "substitute_id"},
		{"jenis tidak ada", rina, func() service.LeaveInput {
			in := e.input(e.dl, "2026-11-05", "2026-11-05")
			in.LeaveTypeID = 9999
			return in
		}, "leave_type_id"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := e.svc.Submit(context.Background(), e.w.Actor(tc.who), tc.in())
			require.Error(t, err)
			assert.Equal(t, 422, status(t, err))
			ae, _ := apperrOf(err)
			assert.Contains(t, ae.Fields, tc.field, ae.Message)
		})
	}
}

func TestSubmitSickLeaveWithAttachment(t *testing.T) {
	e := newLeaveEnv(t)
	in := e.input(e.cs, "2026-10-20", "2026-10-21")
	in.Filename, in.File = "surat.pdf", strings.NewReader("%PDF-1.4\n%contoh\n1 0 obj\n<<>>\nendobj\n")
	v := e.submit(t, e.w.EmpIT1, in)
	assert.True(t, v.HasFile)

	// sakit 1 hari tidak perlu lampiran
	e.submit(t, e.w.EmpIT2, e.input(e.cs, "2026-10-20", "2026-10-20"))

	// jenis berkas dicek dari isi, bukan ekstensi
	bad := e.input(e.cs, "2026-11-03", "2026-11-04")
	bad.Filename, bad.File = "surat.pdf", strings.NewReader("MZ bukan pdf, ini executable")
	_, err := e.svc.Submit(context.Background(), e.w.Actor(e.w.EmpIT1), bad)
	assert.Equal(t, 422, status(t, err))
}

func TestAttachmentAccess(t *testing.T) {
	e := newLeaveEnv(t)
	in := e.input(e.cs, "2026-10-20", "2026-10-21")
	in.Filename, in.File = "surat.pdf", strings.NewReader("%PDF-1.4\n%contoh\n1 0 obj\n<<>>\nendobj\n")
	v := e.submit(t, e.w.EmpIT1, in)

	for _, who := range []*model.Employee{e.w.EmpIT1, e.w.MgrIT, e.w.HR, e.w.Director} {
		f, name, err := e.svc.OpenAttachment(e.w.Actor(who), v.ID)
		require.NoError(t, err, who.Name)
		assert.Equal(t, "surat.pdf", name)
		_ = f.Close()
	}
	for _, who := range []*model.Employee{e.w.EmpIT2, e.w.MgrCS, e.w.EmpCS1} {
		_, _, err := e.svc.OpenAttachment(e.w.Actor(who), v.ID)
		require.Error(t, err, who.Name)
		assert.Equal(t, 404, status(t, err), who.Name)
	}
}

// ---------- alur persetujuan ----------

func TestApprovalFlowEmployeeToManagerToHR(t *testing.T) {
	e := newLeaveEnv(t)
	rina := e.w.EmpIT1
	v := e.submit(t, rina, e.input(e.ct, "2026-10-27", "2026-10-29")) // Sel-Kam = 3 hari kerja
	assert.Equal(t, model.LeavePendingManager, v.Status)
	assert.Equal(t, "Menunggu Atasan", v.StatusLabel)
	assert.True(t, e.used(t, rina, e.ct).IsZero(), "saldo belum dipotong saat diajukan")

	// bukan penyetuju tahap 1: HR, rekan satu tim, manajer departemen lain, pemilik sendiri
	for _, who := range []*model.Employee{e.w.HR, e.w.EmpIT2, e.w.MgrCS, rina} {
		_, err := e.svc.Approve(context.Background(), e.w.Actor(who), v.ID, "")
		require.Error(t, err, who.Name)
		assert.Contains(t, []int{403, 404}, status(t, err), who.Name)
	}

	stage1 := e.approve(t, e.w.MgrIT, v.ID)
	assert.Equal(t, model.LeavePendingHR, stage1.Status)
	assert.Equal(t, "Menunggu HR", stage1.StatusLabel)
	assert.True(t, e.used(t, rina, e.ct).IsZero(), "belum dipotong sebelum persetujuan penuh")

	// tahap 2: atasan tidak boleh lagi, HR boleh
	_, err := e.svc.Approve(context.Background(), e.w.Actor(e.w.MgrIT), v.ID, "")
	assert.Equal(t, 403, status(t, err))

	final := e.approve(t, e.w.HR, v.ID)
	assert.Equal(t, model.LeaveApproved, final.Status)
	assert.True(t, final.Deducted)
	assert.True(t, e.used(t, rina, e.ct).Equal(dd("3")), "saldo terpotong 3 hari")

	// tidak bisa diproses dua kali
	_, err = e.svc.Approve(context.Background(), e.w.Actor(e.w.HR), v.ID, "")
	assert.Equal(t, 409, status(t, err))

	// linimasa: diajukan, disetujui atasan, disetujui HR
	got, err := e.svc.Get(e.w.Actor(rina), v.ID)
	require.NoError(t, err)
	require.Len(t, got.Timeline, 3)
	assert.Equal(t, "done", got.Timeline[1].State)
	assert.Equal(t, e.w.MgrIT.Name, got.Timeline[1].Actor)
	assert.Equal(t, e.w.HR.Name, got.Timeline[2].Actor)
}

func TestRejectRequiresNoteAndKeepsBalance(t *testing.T) {
	e := newLeaveEnv(t)
	rina := e.w.EmpIT1
	v := e.submit(t, rina, e.input(e.ct, "2026-10-27", "2026-10-28"))

	_, err := e.svc.Reject(context.Background(), e.w.Actor(e.w.MgrIT), v.ID, "  ")
	assert.Equal(t, 422, status(t, err), "alasan wajib saat menolak")

	rej, err := e.svc.Reject(context.Background(), e.w.Actor(e.w.MgrIT), v.ID, "Sedang ada rilis besar")
	require.NoError(t, err)
	assert.Equal(t, model.LeaveRejected, rej.Status)
	assert.True(t, e.used(t, rina, e.ct).IsZero())

	// pengajuan yang sudah ditolak tidak bisa disetujui atau dibatalkan
	_, err = e.svc.Approve(context.Background(), e.w.Actor(e.w.HR), v.ID, "")
	assert.Equal(t, 409, status(t, err))
	_, err = e.svc.Cancel(context.Background(), e.w.Actor(rina), v.ID)
	assert.Equal(t, 409, status(t, err))

	// penolakan oleh HR di tahap 2
	v2 := e.submit(t, rina, e.input(e.ct, "2026-11-03", "2026-11-03"))
	e.approve(t, e.w.MgrIT, v2.ID)
	rej2, err := e.svc.Reject(context.Background(), e.w.Actor(e.w.HR), v2.ID, "Kuota tim bulan ini penuh")
	require.NoError(t, err)
	assert.Equal(t, model.LeaveRejected, rej2.Status)
	assert.True(t, e.used(t, rina, e.ct).IsZero())
	assert.Equal(t, "rejected", rej2.Timeline[len(rej2.Timeline)-1].State)
	assert.Equal(t, "Kuota tim bulan ini penuh", rej2.Timeline[len(rej2.Timeline)-1].Note)
}

func TestCancelRefundsBalance(t *testing.T) {
	e := newLeaveEnv(t)
	rina := e.w.EmpIT1

	// batal saat masih menunggu: saldo tidak berubah
	p := e.submit(t, rina, e.input(e.ct, "2026-10-27", "2026-10-28"))
	c, err := e.svc.Cancel(context.Background(), e.w.Actor(rina), p.ID)
	require.NoError(t, err)
	assert.Equal(t, model.LeaveCancelled, c.Status)
	assert.NotNil(t, c.CancelledAt)
	assert.True(t, e.used(t, rina, e.ct).IsZero())

	// batal setelah disetujui penuh: saldo dikembalikan
	a := e.submit(t, rina, e.input(e.ct, "2026-11-03", "2026-11-05"))
	e.approve(t, e.w.MgrIT, a.ID)
	e.approve(t, e.w.HR, a.ID)
	assert.True(t, e.used(t, rina, e.ct).Equal(dd("3")))
	c, err = e.svc.Cancel(context.Background(), e.w.Actor(rina), a.ID)
	require.NoError(t, err)
	assert.Equal(t, model.LeaveCancelled, c.Status)
	assert.False(t, c.Deducted)
	assert.True(t, e.used(t, rina, e.ct).IsZero(), "saldo kembali")

	// dibatalkan dua kali → ditolak, saldo tidak dikembalikan dua kali
	_, err = e.svc.Cancel(context.Background(), e.w.Actor(rina), a.ID)
	assert.Equal(t, 409, status(t, err))
	assert.True(t, e.used(t, rina, e.ct).IsZero())

	// tanggal yang dibatalkan bebas diajukan lagi
	e.submit(t, rina, e.input(e.ct, "2026-11-03", "2026-11-05"))
}

func TestCancelRules(t *testing.T) {
	e := newLeaveEnv(t)
	rina := e.w.EmpIT1
	v := e.submit(t, rina, e.input(e.ct, "2026-10-21", "2026-10-22"))

	// orang lain tidak bisa membatalkan
	_, err := e.svc.Cancel(context.Background(), e.w.Actor(e.w.EmpIT2), v.ID)
	assert.Equal(t, 404, status(t, err))
	_, err = e.svc.Cancel(context.Background(), e.w.Actor(e.w.MgrIT), v.ID)
	assert.Equal(t, 403, status(t, err), "atasan boleh melihat tetapi tidak membatalkan")

	// batas pembatalan: hari sebelum tanggal mulai
	got, err := e.svc.Get(e.w.Actor(rina), v.ID)
	require.NoError(t, err)
	assert.True(t, got.CanCancel)
	assert.Equal(t, "2026-10-20", got.CancelUntil)

	// pada tanggal mulai sudah terlambat
	e.svc.SetClock(func() time.Time { return time.Date(2026, 10, 21, 8, 0, 0, 0, time.UTC) })
	_, err = e.svc.Cancel(context.Background(), e.w.Actor(rina), v.ID)
	assert.Equal(t, 409, status(t, err))
	got, err = e.svc.Get(e.w.Actor(rina), v.ID)
	require.NoError(t, err)
	assert.False(t, got.CanCancel)
}

func TestManagerRequestGoesToDirectorThenHR(t *testing.T) {
	e := newLeaveEnv(t)
	v := e.submit(t, e.w.MgrIT, e.input(e.ct, "2026-10-27", "2026-10-28"))
	assert.Equal(t, model.LeavePendingManager, v.Status)

	// manajer lain / HR tidak bisa menyetujui tahap 1 milik Direktur
	_, err := e.svc.Approve(context.Background(), e.w.Actor(e.w.HR), v.ID, "")
	assert.Equal(t, 403, status(t, err), "HR melihat tetapi bukan penyetuju tahap ini")
	_, err = e.svc.Approve(context.Background(), e.w.Actor(e.w.MgrCS), v.ID, "")
	assert.Equal(t, 404, status(t, err), "manajer departemen lain bahkan tidak melihatnya")
	assert.Equal(t, []string{e.w.Director.Name}, e.rec.last(service.EvLeaveSubmitted).To)

	assert.Equal(t, model.LeavePendingHR, e.approve(t, e.w.Director, v.ID).Status)
	assert.Equal(t, model.LeaveApproved, e.approve(t, e.w.HR, v.ID).Status)
	assert.True(t, e.used(t, e.w.MgrIT, e.ct).Equal(dd("2")))
}

func TestDirectorAndHRRequests(t *testing.T) {
	e := newLeaveEnv(t)
	// Direktur → langsung HR
	d := e.submit(t, e.w.Director, e.input(e.ct, "2026-10-27", "2026-10-27"))
	assert.Equal(t, model.LeavePendingHR, d.Status)
	assert.Equal(t, model.LeaveApproved, e.approve(t, e.w.HR, d.ID).Status)

	// HR → Direktur (HR tidak menyetujui pengajuannya sendiri)
	h := e.submit(t, e.w.HR, e.input(e.ct, "2026-10-27", "2026-10-27"))
	assert.Equal(t, model.LeavePendingManager, h.Status)
	_, err := e.svc.Approve(context.Background(), e.w.Actor(e.w.HR), h.ID, "")
	assert.Equal(t, 403, status(t, err))
	assert.Equal(t, model.LeaveApproved, e.approve(t, e.w.Director, h.ID).Status)
	assert.True(t, e.used(t, e.w.HR, e.ct).Equal(dd("1")))
}

func TestHalfDayNeedsOnlyDirectManager(t *testing.T) {
	e := newLeaveEnv(t)
	rina := e.w.EmpIT1
	in := e.input(e.iz, "2026-10-20", "2026-10-20")
	in.HalfDay, in.HalfSession = true, model.SessionMorning
	v := e.submit(t, rina, in)
	assert.True(t, v.Days.Equal(dd("0.5")))
	require.NotNil(t, v.HalfSession)

	// sesi yang sama bertabrakan, sesi lain boleh
	_, err := e.svc.Submit(context.Background(), e.w.Actor(rina), in)
	assert.Equal(t, 422, status(t, err))
	afternoon := in
	afternoon.HalfSession = model.SessionAfternoon
	e.submit(t, rina, afternoon)
	// satu hari penuh di tanggal yang sama bertabrakan
	_, err = e.svc.Submit(context.Background(), e.w.Actor(rina), e.input(e.dl, "2026-10-20", "2026-10-20"))
	assert.Equal(t, 422, status(t, err))

	final := e.approve(t, e.w.MgrIT, v.ID)
	assert.Equal(t, model.LeaveApproved, final.Status, "setengah hari cukup atasan langsung")
	assert.True(t, e.used(t, rina, e.iz).Equal(dd("0.5")))
}

// ---------- saldo ----------

func TestBalancesViewIncludesPending(t *testing.T) {
	e := newLeaveEnv(t)
	rina := e.w.EmpIT1
	a := e.submit(t, rina, e.input(e.ct, "2026-10-27", "2026-10-28")) // akan disetujui penuh (2)
	e.approve(t, e.w.MgrIT, a.ID)
	e.approve(t, e.w.HR, a.ID)
	e.submit(t, rina, e.input(e.ct, "2026-11-03", "2026-11-04")) // menunggu (2)
	sick := e.input(e.cs, "2026-11-10", "2026-11-10")
	s := e.submit(t, rina, sick)
	e.approve(t, e.w.MgrIT, s.ID)
	e.approve(t, e.w.HR, s.ID)

	bals, err := e.svc.Balances(e.w.Actor(rina), 0, 2026)
	require.NoError(t, err)
	by := map[string]service.BalanceView{}
	for _, b := range bals {
		by[b.LeaveType.Code] = b
	}
	ct := by["CT"]
	assert.True(t, ct.Tracked)
	assert.True(t, ct.Quota.Equal(dd("12")))
	assert.True(t, ct.Used.Equal(dd("2")))
	assert.True(t, ct.Pending.Equal(dd("2")))
	assert.True(t, ct.Available.Equal(dd("8")), "12 - 2 terpakai - 2 menunggu")
	assert.False(t, by["CS"].Tracked)
	assert.True(t, by["CS"].Used.Equal(dd("1")), "sakit hanya dicatat, tidak memotong saldo")
	assert.True(t, e.used(t, rina, e.ct).Equal(dd("2")))
	assert.Nil(t, by["CS"].Quota)

	// hak lihat saldo orang lain
	_, err = e.svc.Balances(e.w.Actor(e.w.EmpIT2), rina.ID, 2026)
	assert.Equal(t, 403, status(t, err))
	_, err = e.svc.Balances(e.w.Actor(e.w.MgrCS), rina.ID, 2026)
	assert.Equal(t, 403, status(t, err))
	_, err = e.svc.Balances(e.w.Actor(e.w.MgrIT), rina.ID, 2026)
	assert.NoError(t, err)
}

func TestPendingRequestsReserveBalance(t *testing.T) {
	e := newLeaveEnv(t)
	rina := e.w.EmpIT1
	e.submit(t, rina, e.input(e.ct, "2026-11-02", "2026-11-10")) // 7 hari kerja, menunggu
	_, err := e.svc.Submit(context.Background(), e.w.Actor(rina), e.input(e.ct, "2026-11-16", "2026-11-24"))
	require.Error(t, err, "7 + 7 > 12: pengajuan pertama masih menahan saldo")
	assert.Equal(t, 422, status(t, err))
	e.submit(t, rina, e.input(e.ct, "2026-11-16", "2026-11-20")) // 5 hari → pas 12
}

func TestApprovalRechecksBalance(t *testing.T) {
	e := newLeaveEnv(t)
	rina := e.w.EmpIT1
	a := e.submit(t, rina, e.input(e.ct, "2026-11-02", "2026-11-06")) // 5
	b := e.submit(t, rina, e.input(e.ct, "2026-11-09", "2026-11-13")) // 5
	for _, id := range []uint64{a.ID, b.ID} {
		e.approve(t, e.w.MgrIT, id)
	}
	// HR mengecilkan kuota setelah kedua pengajuan masuk
	q := dd("7")
	_, err := e.types.Update(e.w.Actor(e.w.HR), e.ct.ID, service.LeaveTypeInput{Name: e.ct.Name, Code: e.ct.Code,
		AnnualQuota: &q, DeductsBalance: true, MinTenureMonths: 12, IsActive: true})
	require.NoError(t, err)

	e.approve(t, e.w.HR, a.ID)
	_, err = e.svc.Approve(context.Background(), e.w.Actor(e.w.HR), b.ID, "")
	require.Error(t, err)
	assert.Equal(t, 409, status(t, err), "saldo tidak cukup saat persetujuan")
	assert.True(t, e.used(t, rina, e.ct).Equal(dd("5")), "saldo tidak melebihi kuota")
	got, err := e.svc.Get(e.w.Actor(rina), b.ID)
	require.NoError(t, err)
	assert.Equal(t, model.LeavePendingHR, got.Status, "tetap menunggu; HR bisa menolak")
}

// Dua persetujuan HR yang berjalan bersamaan pada pengajuan yang sama tidak boleh memotong saldo dua kali.
func TestConcurrentApprovalsDeductOnce(t *testing.T) {
	e := newLeaveEnv(t)
	rina := e.w.EmpIT1
	v := e.submit(t, rina, e.input(e.ct, "2026-11-02", "2026-11-04")) // 3
	e.approve(t, e.w.MgrIT, v.ID)

	hr2 := e.w.HR // aktor yang sama dua kali + HR kedua
	const workers = 8
	var wg sync.WaitGroup
	results := make([]error, workers)
	start := make(chan struct{})
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			_, results[i] = e.svc.Approve(context.Background(), e.w.Actor(hr2), v.ID, "")
		}(i)
	}
	close(start)
	wg.Wait()

	ok, conflict := 0, 0
	for _, err := range results {
		switch {
		case err == nil:
			ok++
		case status(t, err) == 409:
			conflict++
		default:
			t.Fatalf("hasil tak terduga: %v", err)
		}
	}
	assert.Equal(t, 1, ok, "tepat satu persetujuan yang berhasil")
	assert.Equal(t, workers-1, conflict)
	assert.True(t, e.used(t, rina, e.ct).Equal(dd("3")), "saldo hanya terpotong sekali")

	var approvals int64
	require.NoError(t, e.w.DB.Model(&model.LeaveApproval{}).Where("leave_request_id = ? AND level = 2", v.ID).Count(&approvals).Error)
	assert.EqualValues(t, 1, approvals)
}

// Persetujuan dan pembatalan yang berebut tidak menyisakan saldo terpotong untuk pengajuan yang dibatalkan.
func TestConcurrentApproveAndCancel(t *testing.T) {
	e := newLeaveEnv(t)
	rina := e.w.EmpIT1
	v := e.submit(t, rina, e.input(e.ct, "2026-11-02", "2026-11-04"))
	e.approve(t, e.w.MgrIT, v.ID)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, _ = e.svc.Approve(context.Background(), e.w.Actor(e.w.HR), v.ID, "")
	}()
	go func() {
		defer wg.Done()
		_, _ = e.svc.Cancel(context.Background(), e.w.Actor(rina), v.ID)
	}()
	wg.Wait()

	got, err := e.svc.Get(e.w.Actor(rina), v.ID)
	require.NoError(t, err)
	used := e.used(t, rina, e.ct)
	switch got.Status {
	case model.LeaveCancelled:
		assert.True(t, used.IsZero(), "dibatalkan → saldo kembali")
	case model.LeaveApproved:
		assert.True(t, used.Equal(dd("3")), "disetujui → terpotong sekali")
	default:
		t.Fatalf("status akhir tak terduga: %s", got.Status)
	}
}

// ---------- antrean, ringkasan, kalender ----------

func TestApprovalQueueScopes(t *testing.T) {
	e := newLeaveEnv(t)
	it := e.submit(t, e.w.EmpIT1, e.input(e.ct, "2026-11-02", "2026-11-03"))
	cs := e.submit(t, e.w.EmpCS1, e.input(e.ct, "2026-11-02", "2026-11-03"))
	mg := e.submit(t, e.w.MgrIT, e.input(e.ct, "2026-11-09", "2026-11-09"))

	ids := func(who *model.Employee) []uint64 {
		q, err := e.svc.Queue(e.w.Actor(who), "pending")
		require.NoError(t, err)
		var out []uint64
		for _, v := range q {
			out = append(out, v.ID)
			assert.True(t, v.CanAct)
		}
		return out
	}
	assert.Equal(t, []uint64{it.ID}, ids(e.w.MgrIT), "manajer IT hanya melihat timnya")
	assert.Equal(t, []uint64{cs.ID}, ids(e.w.MgrCS))
	assert.Equal(t, []uint64{mg.ID}, ids(e.w.Director), "Direktur menyetujui tahap 1 milik manajer")
	assert.Empty(t, ids(e.w.HR), "belum ada yang sampai tahap HR")

	_, err := e.svc.Queue(e.w.Actor(e.w.EmpIT1), "pending")
	assert.Equal(t, 403, status(t, err), "karyawan biasa tidak punya antrean")

	e.approve(t, e.w.MgrIT, it.ID)
	assert.Equal(t, []uint64{it.ID}, ids(e.w.HR))
	n, err := e.svc.PendingCount(e.w.Actor(e.w.HR))
	require.NoError(t, err)
	assert.Equal(t, 1, n)

	hist, err := e.svc.Queue(e.w.Actor(e.w.MgrIT), "history")
	require.NoError(t, err)
	require.Len(t, hist, 1)
	assert.Equal(t, it.ID, hist[0].ID)
}

func TestGetVisibility(t *testing.T) {
	e := newLeaveEnv(t)
	v := e.submit(t, e.w.EmpIT1, e.input(e.ct, "2026-11-02", "2026-11-03"))
	for _, who := range []*model.Employee{e.w.EmpIT1, e.w.MgrIT, e.w.HR, e.w.Director} {
		_, err := e.svc.Get(e.w.Actor(who), v.ID)
		assert.NoError(t, err, who.Name)
	}
	for _, who := range []*model.Employee{e.w.EmpIT2, e.w.MgrCS, e.w.EmpCS1} {
		_, err := e.svc.Get(e.w.Actor(who), v.ID)
		require.Error(t, err, who.Name)
		assert.Equal(t, 404, status(t, err), who.Name)
	}
	_, _, err := e.svc.List(e.w.Actor(e.w.EmpIT2), service.LeaveListFilter{EmployeeID: e.w.EmpIT1.ID})
	assert.Equal(t, 403, status(t, err))
}

func TestApproverSeesTeamConflicts(t *testing.T) {
	e := newLeaveEnv(t)
	other := e.submit(t, e.w.EmpIT2, e.input(e.ct, "2026-11-03", "2026-11-04"))
	e.approve(t, e.w.MgrIT, other.ID)
	e.approve(t, e.w.HR, other.ID)
	v := e.submit(t, e.w.EmpIT1, e.input(e.ct, "2026-11-04", "2026-11-05"))

	d, err := e.svc.Get(e.w.Actor(e.w.MgrIT), v.ID)
	require.NoError(t, err)
	require.Len(t, d.Conflicts, 1)
	assert.Equal(t, e.w.EmpIT2.Name, d.Conflicts[0].EmployeeName)
	require.NotNil(t, d.Balance)
	assert.True(t, d.Balance.Available.Equal(dd("12")))
	assert.True(t, d.Balance.After.Equal(dd("10")))

	own, err := e.svc.Get(e.w.Actor(e.w.EmpIT1), v.ID)
	require.NoError(t, err)
	assert.Empty(t, own.Conflicts, "pengaju tidak melihat jadwal rekan")
}

func TestCalendar(t *testing.T) {
	e := newLeaveEnv(t)
	e.holiday(t, "2026-11-11", "Libur contoh")
	a := e.submit(t, e.w.EmpIT1, e.input(e.ct, "2026-11-03", "2026-11-04"))
	e.approve(t, e.w.MgrIT, a.ID)
	e.approve(t, e.w.HR, a.ID)
	e.submit(t, e.w.EmpIT2, e.input(e.dl, "2026-11-16", "2026-11-17")) // menunggu
	e.submit(t, e.w.EmpCS1, e.input(e.ct, "2026-12-01", "2026-12-02")) // bulan lain

	cal, err := e.svc.Calendar(e.w.Actor(e.w.EmpCS1), day("2026-11-01"), 0, false)
	require.NoError(t, err)
	assert.Equal(t, "2026-11", cal.Month)
	require.Len(t, cal.Items, 1, "hanya yang disetujui")
	assert.Equal(t, e.w.EmpIT1.Name, cal.Items[0].EmployeeName)
	assert.Equal(t, "CT", cal.Items[0].TypeCode)
	require.Len(t, cal.Holidays, 1)

	withPending, err := e.svc.Calendar(e.w.Actor(e.w.EmpCS1), day("2026-11-01"), 0, true)
	require.NoError(t, err)
	assert.Len(t, withPending.Items, 2)

	onlyCS, err := e.svc.Calendar(e.w.Actor(e.w.EmpCS1), day("2026-11-01"), e.w.CS.ID, true)
	require.NoError(t, err)
	assert.Empty(t, onlyCS.Items, "filter departemen")
}

func TestSummaryForDashboards(t *testing.T) {
	e := newLeaveEnv(t)
	e.submit(t, e.w.EmpIT1, e.input(e.ct, "2026-11-02", "2026-11-03"))
	emp, err := e.svc.Summary(e.w.Actor(e.w.EmpIT1))
	require.NoError(t, err)
	assert.Equal(t, 2026, emp.Year)
	require.Len(t, emp.Recent, 1)
	assert.Zero(t, emp.PendingCount)

	mgr, err := e.svc.Summary(e.w.Actor(e.w.MgrIT))
	require.NoError(t, err)
	assert.Equal(t, 1, mgr.PendingCount)
	require.Len(t, mgr.Queue, 1)
}

// ---------- jenis cuti & hari libur ----------

func TestLeaveTypeAdmin(t *testing.T) {
	e := newLeaveEnv(t)
	hr, emp := e.w.Actor(e.w.HR), e.w.Actor(e.w.EmpIT1)
	q := dd("10")
	in := service.LeaveTypeInput{Name: "Cuti besar", Code: "cb-1", AnnualQuota: &q, DeductsBalance: true, IsActive: true}

	_, err := e.types.Create(emp, in)
	assert.Equal(t, 403, status(t, err), "hanya HR")
	got, err := e.types.Create(hr, in)
	require.NoError(t, err)
	assert.Equal(t, "CB-1", got.Code, "kode dibuat kapital")
	_, err = e.types.Create(hr, in)
	assert.Equal(t, 422, status(t, err), "kode kembar")
	bad := in
	bad.AnnualQuota = nil
	bad.Code = "X"
	_, err = e.types.Create(hr, bad)
	assert.Equal(t, 422, status(t, err), "potong saldo butuh kuota")

	// ubah kuota → saldo tahun berjalan ikut berubah (yang belum diubah manual)
	y := time.Now().Year()
	_, err = e.w.Store.R().Balances.Ensure(e.w.EmpIT1.ID, e.ct.ID, y, dd("12"))
	require.NoError(t, err)
	_, err = e.w.Store.R().Balances.Ensure(e.w.EmpIT2.ID, e.ct.ID, y, dd("15")) // sudah disesuaikan manual
	require.NoError(t, err)
	q14 := dd("14")
	_, err = e.types.Update(hr, e.ct.ID, service.LeaveTypeInput{Name: e.ct.Name, Code: e.ct.Code, AnnualQuota: &q14, DeductsBalance: true, MinTenureMonths: 12, IsActive: true})
	require.NoError(t, err)
	b1, _ := e.w.Store.R().Balances.Ensure(e.w.EmpIT1.ID, e.ct.ID, y, dd("0"))
	b2, _ := e.w.Store.R().Balances.Ensure(e.w.EmpIT2.ID, e.ct.ID, y, dd("0"))
	assert.True(t, b1.Quota.Equal(dd("14")))
	assert.True(t, b2.Quota.Equal(dd("15")), "penyesuaian manual tidak ditimpa")

	// jenis yang sudah dipakai tidak bisa dihapus
	e.submit(t, e.w.EmpIT1, e.input(e.dl, "2026-11-05", "2026-11-05"))
	assert.Equal(t, 409, status(t, e.types.Delete(hr, e.dl.ID)))
	assert.NoError(t, e.types.Delete(hr, got.ID))

	// nonaktif tidak muncul untuk non-HR dan tidak bisa dipilih
	e.ck.IsActive = false
	require.NoError(t, e.w.Store.R().LeaveTypes.Update(e.ck))
	list, err := e.types.List(emp)
	require.NoError(t, err)
	for _, lt := range list {
		assert.NotEqual(t, e.ck.ID, lt.ID)
	}
	_, err = e.svc.Submit(context.Background(), emp, e.input(e.ck, "2026-11-05", "2026-11-05"))
	assert.Equal(t, 422, status(t, err))
}

func TestHolidayAdminAndImport(t *testing.T) {
	e := newLeaveEnv(t)
	hr, emp := e.w.Actor(e.w.HR), e.w.Actor(e.w.EmpIT1)
	in := service.HolidayInput{Date: day("2026-11-20"), Name: "Libur perusahaan", Type: model.HolidayJoint}

	_, err := e.hols.Create(emp, in)
	assert.Equal(t, 403, status(t, err))
	h, err := e.hols.Create(hr, in)
	require.NoError(t, err)
	_, err = e.hols.Create(hr, in)
	assert.Equal(t, 422, status(t, err), "tanggal kembar")

	// libur baru langsung memengaruhi hitung hari kerja
	pv, err := e.svc.Preview(emp, e.ct.ID, day("2026-11-19"), day("2026-11-23"), service.PartialSpec{})
	require.NoError(t, err)
	assert.True(t, pv.WorkingDays.Days.Equal(dd("2")), "Kam, (Jum libur), Sen")

	in.Name = "Libur perusahaan (revisi)"
	up, err := e.hols.Update(hr, h.ID, in)
	require.NoError(t, err)
	assert.Equal(t, in.Name, up.Name)
	require.NoError(t, e.hols.Delete(hr, h.ID))
	assert.Equal(t, 404, status(t, e.hols.Delete(hr, h.ID)))

	added, err := e.hols.ImportOfficial(hr, 2026)
	require.NoError(t, err)
	assert.Greater(t, added, 20)
	again, err := e.hols.ImportOfficial(hr, 2026)
	require.NoError(t, err)
	assert.Zero(t, again, "impor ulang tidak menggandakan")
	_, err = e.hols.ImportOfficial(hr, 2031)
	assert.Equal(t, 404, status(t, err))
	list, err := e.hols.List(2026)
	require.NoError(t, err)
	assert.Equal(t, added, len(list))
}

func TestPreviewWarnsAboutBalanceAndOverlap(t *testing.T) {
	e := newLeaveEnv(t)
	rina := e.w.Actor(e.w.EmpIT1)
	pv, err := e.svc.Preview(rina, e.ct.ID, day("2026-11-02"), day("2026-11-04"), service.PartialSpec{})
	require.NoError(t, err)
	assert.True(t, pv.WorkingDays.Days.Equal(dd("3")))
	require.NotNil(t, pv.Balance)
	assert.True(t, pv.Balance.After.Equal(dd("9")))
	assert.Empty(t, pv.Warnings)
	require.Len(t, pv.Flow, 2)
	assert.Equal(t, []string{e.w.MgrIT.Name}, pv.Flow[0].Names)
	assert.Equal(t, "Atasan langsung", pv.Flow[0].Role)
	assert.Equal(t, []string{e.w.HR.Name}, pv.Flow[1].Names)

	e.submit(t, e.w.EmpIT1, e.input(e.ct, "2026-11-03", "2026-11-03"))
	pv, err = e.svc.Preview(rina, e.ct.ID, day("2026-11-02"), day("2026-11-04"), service.PartialSpec{})
	require.NoError(t, err)
	assert.NotEmpty(t, pv.Warnings, "tumpang tindih diberitahu sebelum dikirim")

	// setengah hari hanya satu tahap
	half, err := e.svc.Preview(rina, e.iz.ID, day("2026-11-05"), day("2026-11-05"), service.PartialSpec{Half: true})
	require.NoError(t, err)
	assert.Len(t, half.Flow, 1)
}

// ---------- notifikasi ----------

func TestNotificationsAcrossFlow(t *testing.T) {
	e := newLeaveEnv(t)
	rina := e.w.EmpIT1
	v := e.submit(t, rina, e.input(e.ct, "2026-11-02", "2026-11-03"))

	n := e.rec.last(service.EvLeaveSubmitted)
	require.NotNil(t, n)
	assert.Equal(t, []string{e.w.MgrIT.Name}, n.To, "pengajuan baru → atasan")
	assert.Contains(t, n.Notice.Message, rina.Name)
	assert.Equal(t, "/persetujuan", n.Notice.Link)

	e.approve(t, e.w.MgrIT, v.ID)
	assert.Equal(t, []string{rina.Name}, e.rec.last(service.EvLeaveProgress).To, "kemajuan → pengaju")
	assert.Equal(t, []string{e.w.HR.Name}, e.rec.last(service.EvLeaveSubmitted).To, "tahap 2 → HR")

	e.approve(t, e.w.HR, v.ID)
	ap := e.rec.last(service.EvLeaveApproved)
	require.NotNil(t, ap)
	assert.Equal(t, []string{rina.Name}, ap.To)

	rej := e.submit(t, rina, e.input(e.ct, "2026-11-09", "2026-11-09"))
	_, err := e.svc.Reject(context.Background(), e.w.Actor(e.w.MgrIT), rej.ID, "Jadwal penuh")
	require.NoError(t, err)
	r := e.rec.last(service.EvLeaveRejected)
	require.NotNil(t, r)
	assert.Contains(t, r.Notice.Message, "Jadwal penuh", "alasan ikut dikirim")

	can := e.submit(t, rina, e.input(e.ct, "2026-11-16", "2026-11-16"))
	_, err = e.svc.Cancel(context.Background(), e.w.Actor(rina), can.ID)
	require.NoError(t, err)
	assert.Equal(t, []string{e.w.MgrIT.Name}, e.rec.last(service.EvLeaveCancelled).To, "pembatalan → penyetuju tahap berjalan")
}

// newNotifyEnv memakai NotifyService sungguhan (DB + email palsu) agar baris notifications ikut diuji.
func newNotifyEnv(t *testing.T) (*leaveEnv, *service.NotifyService) {
	t.Helper()
	e := newLeaveEnv(t)
	ns := service.NewNotifyService(e.w.Store, zerolog.Nop(), service.EmailChannel{Sender: e.w.Mailer, BaseURL: "http://app.test"})
	files, err := storage.NewLocal(e.w.Cfg.StorageDir)
	require.NoError(t, err)
	e.svc = service.NewLeaveService(e.w.Store, files, service.NewWorkingDayService(e.w.Store), ns)
	e.svc.SetClock(func() time.Time { return today })
	return e, ns
}

func TestNotificationsStoredAndEmailed(t *testing.T) {
	e, _ := newNotifyEnv(t)
	rina := e.w.EmpIT1
	v := e.submit(t, rina, e.input(e.ct, "2026-11-02", "2026-11-03"))

	// in-app: atasan punya satu notifikasi belum dibaca
	list, err := service.NewNotificationService(e.w.Store).List(e.w.Actor(e.w.MgrIT))
	require.NoError(t, err)
	assert.EqualValues(t, 1, list.Unread)
	require.Len(t, list.Items, 1)
	assert.Contains(t, list.Items[0].Data, rina.Name)

	// email: terkirim ke alamat atasan, memuat tautan aplikasi
	mails := e.w.Mailer.NoticesTo("bayu@test.id")
	require.Len(t, mails, 1)
	assert.Contains(t, mails[0].Subject, "Pengajuan")
	assert.Contains(t, mails[0].Body, "http://app.test/persetujuan")

	e.approve(t, e.w.MgrIT, v.ID)
	e.approve(t, e.w.HR, v.ID)
	assert.NotEmpty(t, e.w.Mailer.NoticesTo("rina@test.id"), "pengaju menerima email kemajuan & hasil")
	assert.Len(t, e.w.Mailer.NoticesTo("rina@test.id"), 2)
	assert.Len(t, e.w.Mailer.NoticesTo("hr@test.id"), 1)

	// tandai terbaca
	require.NoError(t, service.NewNotificationService(e.w.Store).MarkAllRead(e.w.Actor(e.w.MgrIT)))
	list, _ = service.NewNotificationService(e.w.Store).List(e.w.Actor(e.w.MgrIT))
	assert.Zero(t, list.Unread)
}

func TestNotificationToggles(t *testing.T) {
	e, ns := newNotifyEnv(t)
	hr := e.w.Actor(e.w.HR)

	_, err := ns.Settings(e.w.Actor(e.w.EmpIT1))
	assert.Equal(t, 403, status(t, err))
	settings, err := ns.Settings(hr)
	require.NoError(t, err)
	require.Len(t, settings, 4)
	for _, s := range settings {
		assert.True(t, s.Enabled, "default aktif")
	}
	assert.Equal(t, 400, status(t, ns.SetSetting(hr, "notify.tidak_ada", false)))

	require.NoError(t, ns.SetSetting(hr, "notify.leave_submitted", false))
	e.submit(t, e.w.EmpIT1, e.input(e.ct, "2026-11-02", "2026-11-03"))
	assert.Empty(t, e.w.Mailer.NoticesTo("bayu@test.id"), "saklar pengajuan baru dimatikan")

	require.NoError(t, ns.SetSetting(hr, "notify.leave_submitted", true))
	e.submit(t, e.w.EmpIT2, e.input(e.ct, "2026-11-02", "2026-11-03"))
	assert.Len(t, e.w.Mailer.NoticesTo("bayu@test.id"), 1)
}

func TestPeriodOpenedNotifiesEveryone(t *testing.T) {
	w := testutil.NewWorld(t)
	rec := &recNotifier{}
	ps := service.NewPeriodService(w.Store, service.NewCache(w.Rdb))
	ps.SetNotifier(rec)
	draft := &model.AppraisalPeriod{Name: "November 2026", Type: model.PeriodMonthly, StartDate: day("2026-11-01"),
		EndDate: day("2026-11-30"), Status: model.PeriodDraft, KpiWeight: dd("70"), BehaviorWeight: dd("30")}
	require.NoError(t, w.Store.R().Periods.Create(draft))

	_, err := ps.Open(w.Actor(w.HR), draft.ID)
	require.NoError(t, err)
	n := rec.last(service.EvPeriodOpened)
	require.NotNil(t, n)
	assert.Len(t, n.To, 7, "semua karyawan aktif")
	assert.Contains(t, n.Notice.Message, "November 2026")

	// buka kembali periode yang ditutup bukan "periode dibuka"
	_, err = ps.Close(w.Actor(w.HR), draft.ID)
	require.NoError(t, err)
	_, err = ps.Reopen(w.Actor(w.HR), draft.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, rec.count(service.EvPeriodOpened))
}

func TestDeadlineReminders(t *testing.T) {
	w := testutil.NewWorld(t)
	rec := &recNotifier{}
	rs := service.NewReminderService(w.Store, rec, w.Rdb)
	// periode aktif berakhir 31 Okt 2026
	at := func(d string) func() time.Time { return func() time.Time { return day(d).Add(8 * time.Hour) } }

	rs.SetClock(at("2026-10-27")) // 4 hari lagi → belum
	n, err := rs.Run(context.Background())
	require.NoError(t, err)
	assert.Zero(t, n)

	// Rina sudah mengirim self-assessment → tidak diingatkan
	require.NoError(t, w.Store.R().Appraisals.Create(&model.Appraisal{PeriodID: w.Active.ID, EmployeeID: w.EmpIT1.ID, Status: model.AppraisalSelfDone}))

	rs.SetClock(at("2026-10-28")) // 3 hari lagi
	n, err = rs.Run(context.Background())
	require.NoError(t, err)
	assert.Positive(t, n)
	var names []string
	for _, it := range rec.items {
		assert.Equal(t, service.EvDeadline, it.Notice.Type)
		names = append(names, it.To...)
	}
	assert.NotContains(t, names, w.EmpIT1.Name, "sudah mengirim self-assessment")
	assert.Contains(t, names, w.EmpIT2.Name)
	assert.Contains(t, names, w.EmpCS1.Name)
	assert.Contains(t, names, w.MgrIT.Name, "manajer diingatkan menilai tim")
	assert.NotContains(t, names, w.HR.Name, "HR tanpa departemen tidak ikut dinilai")

	// dijalankan lagi di hari yang sama → tidak mengirim ulang
	before := len(rec.items)
	n, err = rs.Run(context.Background())
	require.NoError(t, err)
	assert.Zero(t, n)
	assert.Len(t, rec.items, before)

	rs.SetClock(at("2026-10-29")) // 2 hari lagi → tidak ada jadwal
	n, _ = rs.Run(context.Background())
	assert.Zero(t, n)
	rs.SetClock(at("2026-10-30")) // 1 hari lagi
	n, err = rs.Run(context.Background())
	require.NoError(t, err)
	assert.Positive(t, n)
	last := rec.items[len(rec.items)-1].Notice.Message
	assert.Contains(t, last, "besok")
}

func apperrOf(err error) (*apperr.Error, bool) { return apperr.As(err) }

func TestSubstitutesAndLastStageFlag(t *testing.T) {
	e := newLeaveEnv(t)
	subs, err := e.svc.Substitutes(e.w.Actor(e.w.EmpIT1))
	require.NoError(t, err)
	names := []string{}
	for _, s := range subs {
		names = append(names, s.Name)
	}
	assert.ElementsMatch(t, []string{e.w.EmpIT2.Name, e.w.MgrIT.Name}, names, "rekan satu departemen, tanpa diri sendiri dan departemen lain")
	none, err := e.svc.Substitutes(e.w.Actor(e.w.HR))
	require.NoError(t, err)
	assert.Empty(t, none, "HR tanpa departemen tidak punya calon pengganti")

	v := e.submit(t, e.w.EmpIT1, e.input(e.ct, "2026-11-02", "2026-11-03"))
	got, err := e.svc.Get(e.w.Actor(e.w.MgrIT), v.ID)
	require.NoError(t, err)
	assert.False(t, got.IsLastStage, "tahap 1 dari 2")
	e.approve(t, e.w.MgrIT, v.ID)
	got, err = e.svc.Get(e.w.Actor(e.w.HR), v.ID)
	require.NoError(t, err)
	assert.True(t, got.IsLastStage, "tahap HR menyelesaikan pengajuan")

	half := e.input(e.iz, "2026-11-04", "2026-11-04")
	half.HalfDay = true
	h := e.submit(t, e.w.EmpIT1, half)
	hv, err := e.svc.Get(e.w.Actor(e.w.MgrIT), h.ID)
	require.NoError(t, err)
	assert.True(t, hv.IsLastStage, "setengah hari: persetujuan atasan sudah final")
}

func TestCarryOverIsOffByDefaultAndConfigurable(t *testing.T) {
	e := newLeaveEnv(t)
	hr, emp := e.w.Actor(e.w.HR), e.w.EmpIT1
	// tahun lalu: kuota 12, terpakai 5 → sisa 7
	_, err := e.w.Store.R().Balances.Ensure(emp.ID, e.ct.ID, 2025, dd("12"))
	require.NoError(t, err)
	b, err := e.w.Store.R().Balances.Lock(emp.ID, e.ct.ID, 2025)
	require.NoError(t, err)
	b.Used = dd("5")
	require.NoError(t, e.w.Store.R().Balances.Save(b))

	quota := func(who *model.Employee, year int) decimal.Decimal {
		bals, err := e.svc.Balances(e.w.Actor(who), 0, year)
		require.NoError(t, err)
		for _, v := range bals {
			if v.LeaveType.ID == e.ct.ID {
				return *v.Quota
			}
		}
		t.Fatal("saldo cuti tahunan tidak ditemukan")
		return decimal.Zero
	}

	assert.True(t, quota(emp, 2026).Equal(dd("12")), "default: sisa tahun lalu hangus")

	p, err := e.types.Policy(hr)
	require.NoError(t, err)
	assert.False(t, p.CarryOver)
	_, err = e.types.SetPolicy(e.w.Actor(emp), service.LeavePolicy{CarryOver: true})
	assert.Error(t, err, "hanya HR")

	_, err = e.types.SetPolicy(hr, service.LeavePolicy{CarryOver: true})
	require.NoError(t, err)
	// karyawan lain dengan sisa 7 mendapat 12 + 7; saldo yang sudah ada tidak berubah
	other := e.w.EmpIT2
	_, err = e.w.Store.R().Balances.Ensure(other.ID, e.ct.ID, 2025, dd("12"))
	require.NoError(t, err)
	ob, err := e.w.Store.R().Balances.Lock(other.ID, e.ct.ID, 2025)
	require.NoError(t, err)
	ob.Used = dd("5")
	require.NoError(t, e.w.Store.R().Balances.Save(ob))
	assert.True(t, quota(other, 2026).Equal(dd("19")), "carry over: 12 + sisa 7")
	assert.True(t, quota(emp, 2026).Equal(dd("12")), "saldo yang sudah dibuat tidak diubah")
}

func TestLeaveListsHaveConstantQueryCount(t *testing.T) {
	e := newLeaveEnv(t)
	w := e.w
	queries := countQueries(w.DB)
	addIT(t, w, 100, 12)
	emps, _, err := w.Store.R().Employees.List(repository.EmployeeFilter{DepartmentID: w.ITS.ID, Page: repository.Page{Page: 1, PerPage: 100}})
	require.NoError(t, err)

	submitFor := func(from, to int) {
		for _, emp := range emps[from:to] {
			if emp.ID == w.MgrIT.ID {
				continue
			}
			emp := emp
			e.submit(t, &emp, e.input(e.iz, "2026-10-27", "2026-10-27"))
		}
	}
	measure := func() map[string]int64 {
		out := map[string]int64{}
		for name, fn := range map[string]func() error{
			"queue":    func() error { _, err := e.svc.Queue(w.Actor(w.MgrIT), "pending"); return err },
			"hr_queue": func() error { _, err := e.svc.Queue(w.Actor(w.HR), "pending"); return err },
			"summary":  func() error { _, err := e.svc.Summary(w.Actor(w.MgrIT)); return err },
			"calendar": func() error { _, err := e.svc.Calendar(w.Actor(w.HR), day("2026-10-01"), 0, true); return err },
			"history": func() error {
				_, _, err := e.svc.List(w.Actor(w.HR), service.LeaveListFilter{})
				return err
			},
		} {
			before := queries()
			require.NoError(t, fn(), name)
			out[name] = queries() - before
		}
		return out
	}

	submitFor(0, 4)
	measure() // pemanasan: saldo dibuat saat pertama dibaca
	few := measure()
	submitFor(4, len(emps))
	measure()
	many := measure()
	for name, n := range few {
		assert.Equal(t, n, many[name], "%s: jumlah query tidak boleh bertambah mengikuti jumlah pengajuan (N+1)", name)
	}
}
