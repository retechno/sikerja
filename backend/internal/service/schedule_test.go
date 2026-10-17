package service_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/retechno/sikerja/backend/internal/model"
	"github.com/retechno/sikerja/backend/internal/service"
)

// Jadwal B: Senin–Jumat 09.00–17.00 dan Sabtu 09.00–15.00 (jadwal Customer Service di masa depan).
func scheduleB() service.ScheduleInput {
	in := service.ScheduleInput{Name: "Shift Sabtu", BreakStart: "12:00", BreakEnd: "13:00"}
	for wd := 1; wd <= 5; wd++ {
		in.Days = append(in.Days, service.ScheduleDayInput{Weekday: wd, StartTime: "09:00", EndTime: "17:00"})
	}
	in.Days = append(in.Days, service.ScheduleDayInput{Weekday: 6, StartTime: "09:00", EndTime: "15:00"})
	return in
}

func (e *leaveEnv) saturdaySchedule(t *testing.T) *model.WorkSchedule {
	t.Helper()
	ss := service.NewScheduleService(e.w.Store)
	_, err := ss.Create(e.w.Actor(e.w.HR), scheduleA()) // default perusahaan
	require.NoError(t, err)
	m, err := ss.Create(e.w.Actor(e.w.HR), scheduleB())
	require.NoError(t, err)
	e.w.CS.ScheduleID = &m.ID
	require.NoError(t, e.w.Store.R().Departments.Update(e.w.CS))
	return m
}

func schedOf(t *testing.T, e *leaveEnv, m *model.WorkSchedule) service.Schedule {
	t.Helper()
	got, err := e.w.Store.R().Schedules.ByID(m.ID)
	require.NoError(t, err)
	return service.ScheduleFromModel(got)
}

func hm(h, m int) int { return h*60 + m }

func TestCountWorkingDaysBySchedule(t *testing.T) {
	e := newLeaveEnv(t)
	b := schedOf(t, e, e.saturdaySchedule(t))
	hol := map[string]string{}
	// 2026-10-23 Jumat, 24 Sabtu, 25 Minggu, 26 Senin
	tests := []struct {
		name       string
		sch        service.Schedule
		start, end string
		part       *service.Partial
		want       string
	}{
		{"jadwal A: Sabtu bukan hari kerja", service.DefaultSchedule, "2026-10-24", "2026-10-24", nil, "0"},
		{"jadwal B: Sabtu hari kerja", b, "2026-10-24", "2026-10-24", nil, "1"},
		{"jadwal B: Jumat–Senin = 3 (Minggu libur)", b, "2026-10-23", "2026-10-26", nil, "3"},
		{"jadwal A: Jumat–Senin = 2", service.DefaultSchedule, "2026-10-23", "2026-10-26", nil, "2"},
		{"jadwal B: Minggu bukan hari kerja", b, "2026-10-25", "2026-10-25", nil, "0"},
		{"Sabtu B 09–11:30 dari 5 jam efektif = 0,5", b, "2026-10-24", "2026-10-24", &service.Partial{From: hm(9, 0), To: hm(11, 30)}, "0.5"},
		{"Sabtu B 09–12 (3 jam efektif dari 5) = 0,6", b, "2026-10-24", "2026-10-24", &service.Partial{From: hm(9, 0), To: hm(12, 0)}, "0.6"},
		{"Sabtu B 11–14 (istirahat dipotong, 2 jam dari 5) = 0,4", b, "2026-10-24", "2026-10-24", &service.Partial{From: hm(11, 0), To: hm(14, 0)}, "0.4"},
		{"Senin B 09–11 dari 7 jam efektif = 0,29", b, "2026-10-26", "2026-10-26", &service.Partial{From: hm(9, 0), To: hm(11, 0)}, "0.29"},
		{"Senin A 08–10 dari 8 jam efektif = 0,25", service.DefaultSchedule, "2026-10-26", "2026-10-26", &service.Partial{From: hm(8, 0), To: hm(10, 0)}, "0.25"},
		{"Senin A pagi (08.00–12.00) = 0,5", service.DefaultSchedule, "2026-10-26", "2026-10-26", &service.Partial{From: hm(8, 0), To: hm(12, 0)}, "0.5"},
		{"Senin A siang (13.00–17.00) = 0,5", service.DefaultSchedule, "2026-10-26", "2026-10-26", &service.Partial{From: hm(13, 0), To: hm(17, 0)}, "0.5"},
		{"melintasi istirahat 11–14 → 2 jam efektif = 0,25", service.DefaultSchedule, "2026-10-26", "2026-10-26", &service.Partial{From: hm(11, 0), To: hm(14, 0)}, "0.25"},
		{"hanya jam istirahat 12–13 = 0", service.DefaultSchedule, "2026-10-26", "2026-10-26", &service.Partial{From: hm(12, 0), To: hm(13, 0)}, "0"},
		{"jam sebelum jam masuk = 0", service.DefaultSchedule, "2026-10-26", "2026-10-26", &service.Partial{From: hm(7, 0), To: hm(9, 0)}, "0"},
		{"jam melewati jam pulang Sabtu B = 0", b, "2026-10-24", "2026-10-24", &service.Partial{From: hm(13, 0), To: hm(16, 0)}, "0"},
		{"sebagian hari di hari libur = 0", b, "2026-10-28", "2026-10-28", &service.Partial{From: hm(9, 0), To: hm(10, 0)}, "0"},
	}
	hol["2026-10-28"] = "Libur contoh"
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := service.CountWorkingDaysFor(tc.sch, day(tc.start), day(tc.end), hol, tc.part)
			assert.True(t, dd(tc.want).Equal(got.Days), "diharapkan %s, dapat %s", tc.want, got.Days)
		})
	}

	// rincian menyebut jam kerja tiap hari
	r := service.CountWorkingDaysFor(b, day("2026-10-23"), day("2026-10-26"), hol, nil)
	require.Len(t, r.Breakdown, 4)
	assert.Equal(t, "09:00–17:00", r.Breakdown[0].Hours)
	assert.Equal(t, "09:00–15:00", r.Breakdown[1].Hours)
	assert.Equal(t, 300, r.Breakdown[1].Minutes, "Sabtu 09–15 dikurangi istirahat 1 jam")
	assert.Equal(t, "12:00–13:00", r.Break)
	assert.Equal(t, service.SkipWeekend, r.Breakdown[2].Skip, "Minggu")
}

func TestSaturdayLeaveFollowsDepartmentSchedule(t *testing.T) {
	e := newLeaveEnv(t)
	e.saturdaySchedule(t)

	// CS (jadwal B): Sabtu = 1 hari kerja
	v := e.submit(t, e.w.EmpCS1, e.input(e.dl, "2026-10-24", "2026-10-24"))
	assert.True(t, v.Days.Equal(dd("1")))
	// Jumat–Senin = Jumat + Sabtu + Senin
	v2 := e.submit(t, e.w.EmpCS1, e.input(e.dl, "2026-10-30", "2026-11-02"))
	assert.True(t, v2.Days.Equal(dd("3")))

	// IT (jadwal default A): Sabtu bukan hari kerja → ditolak; Jumat–Senin = 2
	_, err := e.svc.Submit(context.Background(), e.w.Actor(e.w.EmpIT1), e.input(e.dl, "2026-10-24", "2026-10-24"))
	assert.Equal(t, 422, status(t, err))
	v3 := e.submit(t, e.w.EmpIT1, e.input(e.dl, "2026-10-30", "2026-11-02"))
	assert.True(t, v3.Days.Equal(dd("2")))

	// pratinjau memakai jadwal pengaju dan menyebut nama jadwalnya
	pv, err := e.svc.Preview(e.w.Actor(e.w.EmpCS1), e.ct.ID, day("2026-11-06"), day("2026-11-09"), service.PartialSpec{})
	require.NoError(t, err)
	assert.True(t, pv.WorkingDays.Days.Equal(dd("3")))
	assert.Equal(t, "Shift Sabtu", pv.WorkingDays.Schedule)
	assert.Equal(t, "09:00–15:00", pv.WorkingDays.Breakdown[1].Hours)

	// detail: tanggal yang tidak dihitung mengikuti jadwal pengaju (Minggu saja untuk CS)
	d, err := e.svc.Get(e.w.Actor(e.w.EmpCS1), v2.ID)
	require.NoError(t, err)
	require.Len(t, d.Skipped, 1)
	assert.Equal(t, "Min", d.Skipped[0].Weekday)
}

func TestPartialLeaveByHours(t *testing.T) {
	e := newLeaveEnv(t)
	e.saturdaySchedule(t)
	ctx := context.Background()

	// CS, Sabtu 09.00–11.30 = setengah dari 5 jam kerja efektif (09–15 dikurangi istirahat 12–13)
	in := e.input(e.iz, "2026-10-24", "2026-10-24")
	in.HalfDay, in.StartTime, in.EndTime = true, "09:00", "11:30"
	v := e.submit(t, e.w.EmpCS1, in)
	assert.True(t, v.Days.Equal(dd("0.5")))
	require.NotNil(t, v.StartTime)
	assert.Equal(t, "09:00", *v.StartTime)
	assert.Equal(t, "11:30", *v.EndTime)
	require.NotNil(t, v.HalfSession, "tepat setengah hari pertama → sesi pagi")
	assert.Equal(t, model.SessionMorning, *v.HalfSession)
	e.approve(t, e.w.MgrCS, v.ID) // cukup atasan langsung
	assert.True(t, e.used(t, e.w.EmpCS1, e.iz).Equal(dd("0.5")))

	// IT, Senin 08.00–10.00 dari 8 jam efektif = 0,25; sesi tidak terisi (jam bebas)
	in2 := e.input(e.iz, "2026-10-26", "2026-10-26")
	in2.HalfDay, in2.StartTime, in2.EndTime = true, "08:00", "10:00"
	v2 := e.submit(t, e.w.EmpIT1, in2)
	assert.True(t, v2.Days.Equal(dd("0.25")))
	assert.Nil(t, v2.HalfSession)
	e.approve(t, e.w.MgrIT, v2.ID)
	assert.True(t, e.used(t, e.w.EmpIT1, e.iz).Equal(dd("0.25")))
	// batal mengembalikan tepat 0,25
	_, err := e.svc.Cancel(ctx, e.w.Actor(e.w.EmpIT1), v2.ID)
	require.NoError(t, err)
	assert.True(t, e.used(t, e.w.EmpIT1, e.iz).IsZero())

	// sesi pagi/siang tetap bekerja sebagai pintasan (jadwal A: 08.00–12.00 / 13.00–17.00, istirahat 12–13)
	sess := e.input(e.iz, "2026-10-27", "2026-10-27")
	sess.HalfDay, sess.HalfSession = true, model.SessionAfternoon
	v3 := e.submit(t, e.w.EmpIT1, sess)
	assert.True(t, v3.Days.Equal(dd("0.5")))
	assert.Equal(t, "13:00", *v3.StartTime)
	assert.Equal(t, "17:00", *v3.EndTime)
}

func TestPartialLeaveValidation(t *testing.T) {
	e := newLeaveEnv(t)
	cases := []struct {
		name       string
		date       string
		start, end string
		typ        *model.LeaveType
		field      string
	}{
		{"sebelum jam masuk", "2026-10-26", "07:00", "09:00", e.iz, "start_time"},
		{"melewati jam pulang", "2026-10-26", "16:00", "18:00", e.iz, "start_time"},
		{"jam selesai sebelum mulai", "2026-10-26", "10:00", "09:00", e.iz, "end_time"},
		{"kurang dari 15 menit", "2026-10-26", "10:00", "10:10", e.iz, "end_time"},
		{"hanya jam istirahat (12.10–12.50)", "2026-10-26", "12:10", "12:50", e.iz, "end_time"},
		{"seluruh jam kerja = satu hari penuh", "2026-10-26", "08:00", "17:00", e.iz, "end_time"},
		{"format jam salah", "2026-10-26", "pagi", "10:00", e.iz, "start_time"},
		{"hari bukan hari kerja (Sabtu, jadwal A)", "2026-10-24", "09:00", "11:00", e.iz, "start_date"},
		{"jenis tidak boleh sebagian hari", "2026-10-26", "09:00", "11:00", e.ct, "half_day"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := e.input(tc.typ, tc.date, tc.date)
			in.HalfDay, in.StartTime, in.EndTime = true, tc.start, tc.end
			_, err := e.svc.Submit(context.Background(), e.w.Actor(e.w.EmpIT1), in)
			require.Error(t, err)
			ae, _ := apperrOf(err)
			assert.Equal(t, 422, ae.Status)
			assert.Contains(t, ae.Fields, tc.field, ae.Message)
		})
	}
}

func TestPartialLeaveOverlapsByHours(t *testing.T) {
	e := newLeaveEnv(t)
	mk := func(from, to string) service.LeaveInput {
		in := e.input(e.iz, "2026-10-26", "2026-10-26")
		in.HalfDay, in.StartTime, in.EndTime = true, from, to
		return in
	}
	e.submit(t, e.w.EmpIT1, mk("08:00", "10:00"))
	e.submit(t, e.w.EmpIT1, mk("10:00", "12:00")) // bersebelahan: boleh
	e.submit(t, e.w.EmpIT1, mk("15:00", "16:00"))
	for _, c := range [][2]string{{"09:00", "11:00"}, {"11:30", "15:30"}, {"08:00", "09:00"}} {
		_, err := e.svc.Submit(context.Background(), e.w.Actor(e.w.EmpIT1), mk(c[0], c[1]))
		require.Error(t, err, "%v", c)
		assert.Equal(t, 422, status(t, err), "%v", c)
	}
	// satu hari penuh di tanggal itu juga bentrok
	_, err := e.svc.Submit(context.Background(), e.w.Actor(e.w.EmpIT1), e.input(e.dl, "2026-10-26", "2026-10-26"))
	assert.Equal(t, 422, status(t, err))
}

func TestCalendarCarriesPartialHours(t *testing.T) {
	e := newLeaveEnv(t)
	in := e.input(e.iz, "2026-10-26", "2026-10-26")
	in.HalfDay, in.StartTime, in.EndTime = true, "13:00", "15:00"
	v := e.submit(t, e.w.EmpIT1, in)
	e.approve(t, e.w.MgrIT, v.ID)
	cal, err := e.svc.Calendar(e.w.Actor(e.w.EmpIT2), day("2026-10-01"), 0, false)
	require.NoError(t, err)
	require.Len(t, cal.Items, 1)
	assert.Equal(t, "13:00", cal.Items[0].StartTime)
	assert.Equal(t, "15:00", cal.Items[0].EndTime)
	assert.Equal(t, "0,25", cal.Items[0].Days)
}

// ---------- administrasi jadwal ----------

func scheduleA() service.ScheduleInput {
	in := service.ScheduleInput{Name: "Jam kantor", IsDefault: true, BreakStart: "12:00", BreakEnd: "13:00"}
	for wd := 1; wd <= 5; wd++ {
		in.Days = append(in.Days, service.ScheduleDayInput{Weekday: wd, StartTime: "08:00", EndTime: "17:00"})
	}
	return in
}

func TestScheduleAdmin(t *testing.T) {
	e := newLeaveEnv(t)
	ss := service.NewScheduleService(e.w.Store)
	hr, emp := e.w.Actor(e.w.HR), e.w.Actor(e.w.EmpIT1)

	_, err := ss.Create(emp, scheduleB())
	assert.Equal(t, 403, status(t, err), "hanya HR")
	_, err = ss.List(emp)
	assert.Equal(t, 403, status(t, err))

	bad := []service.ScheduleInput{
		{Name: "", Days: []service.ScheduleDayInput{{Weekday: 1, StartTime: "08:00", EndTime: "17:00"}}},
		{Name: "Kosong"},
		{Name: "Hari kembar", Days: []service.ScheduleDayInput{{Weekday: 1, StartTime: "08:00", EndTime: "17:00"}, {Weekday: 1, StartTime: "08:00", EndTime: "12:00"}}},
		{Name: "Terbalik", Days: []service.ScheduleDayInput{{Weekday: 1, StartTime: "17:00", EndTime: "08:00"}}},
		{Name: "Format", Days: []service.ScheduleDayInput{{Weekday: 1, StartTime: "pagi", EndTime: "17:00"}}},
		{Name: "Hari 9", Days: []service.ScheduleDayInput{{Weekday: 9, StartTime: "08:00", EndTime: "17:00"}}},
		{Name: "Terlalu singkat", Days: []service.ScheduleDayInput{{Weekday: 1, StartTime: "08:00", EndTime: "08:30"}}},
	}
	for _, in := range bad {
		_, err := ss.Create(hr, in)
		require.Error(t, err, in.Name)
		assert.Equal(t, 422, status(t, err), in.Name)
	}

	badBreak := scheduleA()
	badBreak.Name = "Istirahat salah"
	badBreak.BreakStart, badBreak.BreakEnd = "13:00", "12:00"
	_, err = ss.Create(hr, badBreak)
	assert.Equal(t, 422, status(t, err), "istirahat terbalik")
	badBreak.BreakStart, badBreak.BreakEnd = "12:00", ""
	_, err = ss.Create(hr, badBreak)
	assert.Equal(t, 422, status(t, err), "istirahat setengah terisi")

	a, err := ss.Create(hr, scheduleA())
	require.NoError(t, err)
	assert.True(t, a.IsDefault)
	assert.Equal(t, "12:00", a.BreakStart)
	b, err := ss.Create(hr, scheduleB())
	require.NoError(t, err)
	assert.False(t, b.IsDefault)
	assert.Len(t, b.Days, 6)
	_, err = ss.Create(hr, scheduleB())
	assert.Equal(t, 422, status(t, err), "nama kembar")

	list, err := ss.List(e.w.Actor(e.w.Director))
	require.NoError(t, err)
	require.Len(t, list, 2)
	assert.Equal(t, a.ID, list[0].ID, "default tampil lebih dulu")

	// default tidak bisa dicopot langsung, dan tidak bisa dihapus
	in := scheduleA()
	in.IsDefault = false
	_, err = ss.Update(hr, a.ID, in)
	assert.Equal(t, 422, status(t, err))
	assert.Equal(t, 409, status(t, ss.Delete(hr, a.ID)))

	// mengganti default memindahkan penanda
	inB := scheduleB()
	inB.IsDefault = true
	up, err := ss.Update(hr, b.ID, inB)
	require.NoError(t, err)
	assert.True(t, up.IsDefault)
	def, err := e.w.Store.R().Schedules.Default()
	require.NoError(t, err)
	assert.Equal(t, b.ID, def.ID)
	cur, err := e.w.Store.R().Schedules.ByID(a.ID)
	require.NoError(t, err)
	assert.False(t, cur.IsDefault)

	// jadwal yang dipakai departemen tidak bisa dihapus
	e.w.CS.ScheduleID = &a.ID
	require.NoError(t, e.w.Store.R().Departments.Update(e.w.CS))
	assert.Equal(t, 409, status(t, ss.Delete(hr, a.ID)))
	e.w.CS.ScheduleID = nil
	require.NoError(t, e.w.Store.R().Departments.Update(e.w.CS))
	require.NoError(t, ss.Delete(hr, a.ID))
	assert.Equal(t, 404, status(t, ss.Delete(hr, a.ID)))
}

func TestDepartmentScheduleAssignment(t *testing.T) {
	e := newLeaveEnv(t)
	ss := service.NewScheduleService(e.w.Store)
	_, err := ss.Create(e.w.Actor(e.w.HR), scheduleA())
	require.NoError(t, err)
	b, err := ss.Create(e.w.Actor(e.w.HR), scheduleB())
	require.NoError(t, err)

	ms := service.NewMasterService(e.w.Store, service.NewCache(e.w.Rdb))
	hr := e.w.Actor(e.w.HR)
	d, err := ms.UpdateDepartment(hr, e.w.CS.ID, service.DepartmentInput{Name: e.w.CS.Name, Code: e.w.CS.Code, ScheduleID: &b.ID})
	require.NoError(t, err)
	require.NotNil(t, d.ScheduleID)
	assert.Equal(t, b.ID, *d.ScheduleID)

	// Sabtu langsung terhitung untuk CS
	v := e.submit(t, e.w.EmpCS1, e.input(e.dl, "2026-10-24", "2026-10-24"))
	assert.True(t, v.Days.Equal(dd("1")))

	// jadwal yang tidak ada ditolak; kosong = kembali ke default
	missing := uint64(9999)
	_, err = ms.UpdateDepartment(hr, e.w.CS.ID, service.DepartmentInput{Name: e.w.CS.Name, Code: e.w.CS.Code, ScheduleID: &missing})
	assert.Equal(t, 422, status(t, err))
	d, err = ms.UpdateDepartment(hr, e.w.CS.ID, service.DepartmentInput{Name: e.w.CS.Name, Code: e.w.CS.Code})
	require.NoError(t, err)
	assert.Nil(t, d.ScheduleID)
}
