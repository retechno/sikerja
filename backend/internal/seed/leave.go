package seed

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	"github.com/retechno/sikerja/backend/internal/model"
	"github.com/retechno/sikerja/backend/internal/repository"
	"github.com/retechno/sikerja/backend/internal/service"
)

func dec(s string) decimal.Decimal { return decimal.RequireFromString(s) }
func decp(s string) *decimal.Decimal {
	v := dec(s)
	return &v
}

// leaveTypes: jenis izin/cuti awal sesuai SPEC. Kuota cuti khusus mengikuti kebijakan perusahaan dan harus dicek HR.
var leaveTypes = []model.LeaveType{
	{Name: "Cuti tahunan", Code: "CT", AnnualQuota: decp("12"), DeductsBalance: true, MinTenureMonths: 12, Sort: 1, IsActive: true},
	{Name: "Cuti sakit", Code: "CS", RequiresAttach: true, AttachMinDays: dec("1"), Sort: 2, IsActive: true},
	{Name: "Izin", Code: "IZ", AnnualQuota: decp("6"), DeductsBalance: true, AllowHalfDay: true, Sort: 3, IsActive: true},
	{Name: "Dinas luar", Code: "DL", RequiresAttach: true, Sort: 4, IsActive: true},
	{Name: "Cuti menikah", Code: "CK-N", MaxDaysPerRequest: decp("3"), RequiresAttach: true, Sort: 5, IsActive: true},
	{Name: "Cuti melahirkan", Code: "CK-M", MaxDaysPerRequest: decp("65"), RequiresAttach: true, Sort: 6, IsActive: true},
	{Name: "Cuti duka", Code: "CK-D", MaxDaysPerRequest: decp("2"), Sort: 7, IsActive: true},
}

// leaveEvent: satu pengajuan contoh. Start = hari kerja pertama pada/setelah now+Offset.
type leaveEvent struct {
	Who, Type  string
	Offset     int // hari dari sekarang
	N          int // jumlah hari kerja (diabaikan bila Half)
	Half       bool
	Session    string
	Status     string
	Reason     string
	Substitute string
	// Approvals: level → nama penyetuju & aksi
	Approvals []seedApproval
	AgeHours  int // diajukan sekian jam yang lalu
}

type seedApproval struct {
	Level  int
	By     string
	Action string
	Note   string
}

var leaveEvents = []leaveEvent{
	// Rina Wulandari — angka sama dengan mockup H-08/H-09 (cuti tahunan terpakai 2 + menunggu 2, izin 1,5, sakit 2)
	{Who: "Rina Wulandari", Type: "CT", Offset: -228, N: 2, Status: model.LeaveCancelled, Reason: "Acara keluarga", AgeHours: 24 * 240,
		Approvals: []seedApproval{{1, "Bayu Pratama", model.ActionApproved, ""}}},
	{Who: "Rina Wulandari", Type: "CT", Offset: -128, N: 2, Status: model.LeaveRejected, Reason: "Liburan singkat", AgeHours: 24 * 140,
		Approvals: []seedApproval{{1, "Bayu Pratama", model.ActionApproved, ""}, {2, "Sinta Maharani", model.ActionRejected, "Bentrok dengan jadwal rilis besar; mohon ajukan di tanggal lain."}}},
	{Who: "Rina Wulandari", Type: "IZ", Offset: -85, N: 1, Status: model.LeaveApproved, Reason: "Mengurus dokumen", AgeHours: 24 * 90,
		Approvals: []seedApproval{{1, "Bayu Pratama", model.ActionApproved, ""}, {2, "Sinta Maharani", model.ActionApproved, ""}}},
	{Who: "Rina Wulandari", Type: "CT", Offset: -60, N: 2, Status: model.LeaveApproved, Reason: "Keperluan keluarga", AgeHours: 24 * 70, Substitute: "Dimas Saputra",
		Approvals: []seedApproval{{1, "Bayu Pratama", model.ActionApproved, ""}, {2, "Sinta Maharani", model.ActionApproved, ""}}},
	{Who: "Rina Wulandari", Type: "CS", Offset: -32, N: 2, Status: model.LeaveApproved, Reason: "Demam dan flu", AgeHours: 24 * 33,
		Approvals: []seedApproval{{1, "Bayu Pratama", model.ActionApproved, ""}, {2, "Sinta Maharani", model.ActionApproved, ""}}},
	{Who: "Rina Wulandari", Type: "IZ", Offset: -8, Half: true, Session: model.SessionMorning, Status: model.LeaveApproved, Reason: "Kontrol ke dokter", AgeHours: 24 * 10,
		Approvals: []seedApproval{{1, "Bayu Pratama", model.ActionApproved, ""}}},
	{Who: "Rina Wulandari", Type: "CT", Offset: 13, N: 2, Status: model.LeavePendingManager, Reason: "Menghadiri wisuda adik di Malang", Substitute: "Dimas Saputra", AgeHours: 2},

	// antrean persetujuan Bayu (tahap 1) dan Sinta (tahap 2)
	{Who: "Putri Maharani", Type: "CT", Offset: 18, N: 3, Status: model.LeavePendingManager, Reason: "Mengurus pindahan rumah", Substitute: "Agus Setiawan", AgeHours: 50},
	{Who: "Agus Setiawan", Type: "IZ", Offset: 3, Half: true, Session: model.SessionMorning, Status: model.LeavePendingManager, Reason: "Mengantar anak imunisasi", AgeHours: 26},
	{Who: "Fajar Nugroho", Type: "CT", Offset: 37, N: 5, Status: model.LeavePendingHR, Reason: "Acara keluarga di Yogyakarta", AgeHours: 30,
		Approvals: []seedApproval{{1, "Bayu Pratama", model.ActionApproved, "Silakan, pastikan serah terima tiket aktif."}}},
	{Who: "Indah Permatasari", Type: "CT", Offset: 45, N: 3, Status: model.LeavePendingManager, Reason: "Liburan akhir tahun bersama keluarga", AgeHours: 20},

	// sudah disetujui → tampil di kalender tim
	{Who: "Dimas Saputra", Type: "DL", Offset: 24, N: 3, Status: model.LeaveApproved, Reason: "Instalasi PMS di hotel klien, Lombok", AgeHours: 24 * 9,
		Approvals: []seedApproval{{1, "Bayu Pratama", model.ActionApproved, ""}, {2, "Sinta Maharani", model.ActionApproved, ""}}},
	{Who: "Agus Setiawan", Type: "CS", Offset: -4, N: 1, Status: model.LeaveApproved, Reason: "Sakit perut", AgeHours: 24 * 5,
		Approvals: []seedApproval{{1, "Bayu Pratama", model.ActionApproved, ""}, {2, "Sinta Maharani", model.ActionApproved, ""}}},
	{Who: "Wahyu Hidayat", Type: "CT", Offset: 16, N: 2, Status: model.LeaveApproved, Reason: "Acara keluarga", AgeHours: 24 * 7,
		Approvals: []seedApproval{{1, "Indah Permatasari", model.ActionApproved, ""}, {2, "Sinta Maharani", model.ActionApproved, ""}}},
	{Who: "Larasati Dewi", Type: "CT", Offset: 27, N: 4, Status: model.LeaveApproved, Reason: "Liburan", AgeHours: 24 * 12,
		Approvals: []seedApproval{{1, "Satria Nugraha", model.ActionApproved, ""}, {2, "Sinta Maharani", model.ActionApproved, ""}}},
}

// EnsureLeave mengisi data izin & cuti bila belum ada (aman untuk database Rilis 1 yang sudah terisi).
// Mengembalikan true bila ada yang diisi.
func EnsureLeave(db *gorm.DB, now time.Time) (bool, error) {
	if now.IsZero() {
		now = time.Now()
	}
	var n int64
	if err := db.Model(&model.LeaveType{}).Count(&n).Error; err != nil {
		return false, err
	}
	if n > 0 {
		return false, nil
	}
	var emps int64
	if err := db.Model(&model.Employee{}).Count(&emps).Error; err != nil {
		return false, err
	}
	if emps == 0 {
		return false, nil
	}
	err := repository.NewStore(db).Tx(func(r repository.Repos) error { return seedLeave(r, now) })
	return err == nil, err
}

func seedLeave(r repository.Repos, now time.Time) error {
	today := service.DateOnly(now)
	year := today.Year()

	// hari libur tahun berjalan (dan tahun lain bila ada daftar bawaan)
	hmap := map[string]string{}
	for y := year - 1; y <= year+1; y++ {
		for _, h := range service.OfficialHolidays[y] {
			row := h
			if err := r.Holidays.Create(&row); err != nil {
				return err
			}
			hmap[row.Date.Format("2006-01-02")] = row.Name
		}
	}

	if err := ensureSchedules(r); err != nil {
		return err
	}

	types := map[string]*model.LeaveType{}
	for i := range leaveTypes {
		t := leaveTypes[i]
		if err := r.LeaveTypes.Create(&t); err != nil {
			return err
		}
		types[t.Code] = &t
	}

	emps, err := r.Employees.AllActive()
	if err != nil {
		return err
	}
	byName := map[string]*model.Employee{}
	for i := range emps {
		byName[emps[i].Name] = &emps[i]
	}
	// saldo awal semua karyawan untuk tahun berjalan
	for _, e := range emps {
		for _, t := range types {
			if t.DeductsBalance && t.AnnualQuota != nil {
				if _, err := r.Balances.Ensure(e.ID, t.ID, year, *t.AnnualQuota); err != nil {
					return err
				}
			}
		}
	}

	for _, ev := range leaveEvents {
		who, t := byName[ev.Who], types[ev.Type]
		if who == nil || t == nil {
			continue // nama berubah/tidak ada: lewati saja
		}
		start := nextWorkday(today.AddDate(0, 0, ev.Offset), hmap)
		end, days := start, dec("0.5")
		if !ev.Half {
			end = spanEnd(start, ev.N, hmap)
			days = service.CountWorkingDays(start, end, hmap, false).Days
		}
		created := now.Add(-time.Duration(ev.AgeHours) * time.Hour)
		l := &model.LeaveRequest{
			EmployeeID: who.ID, LeaveTypeID: t.ID, StartDate: start, EndDate: end, HalfDay: ev.Half,
			Days: days, Reason: ev.Reason, Status: ev.Status,
		}
		l.CreatedAt, l.UpdatedAt = created, created
		if ev.Half {
			s := ev.Session
			l.HalfSession = &s
			// jadwal default 08.00–17.00 (istirahat 12.00–13.00): pagi = 08.00–12.00, siang = 13.00–17.00
			from, to := "08:00", "12:00"
			if ev.Session == model.SessionAfternoon {
				from, to = "13:00", "17:00"
			}
			l.StartTime, l.EndTime = &from, &to
		}
		if ev.Substitute != "" && byName[ev.Substitute] != nil {
			l.SubstituteID = &byName[ev.Substitute].ID
		}
		if err := r.Leaves.Create(l); err != nil {
			return err
		}
		no := fmt.Sprintf("%s-%d-%04d", t.Code, created.Year(), l.ID)
		l.Number = &no
		if ev.Status == model.LeaveCancelled {
			c := start.AddDate(0, 0, -5)
			l.CancelledAt = &c
		}
		for i, ap := range ev.Approvals {
			by := byName[ap.By]
			if by == nil {
				continue
			}
			at := created.Add(time.Duration(4*(i+1)) * time.Hour)
			if err := r.Approvals.Create(&model.LeaveApproval{LeaveRequestID: l.ID, ApproverID: by.ID,
				Level: ap.Level, Action: ap.Action, Note: ap.Note, ActedAt: at}); err != nil {
				return err
			}
		}
		if ev.Status == model.LeaveApproved && t.DeductsBalance && t.AnnualQuota != nil {
			bal, err := r.Balances.Lock(who.ID, t.ID, start.Year())
			if err != nil {
				// pengajuan tahun lain: buat saldonya lebih dulu
				if bal, err = r.Balances.Ensure(who.ID, t.ID, start.Year(), *t.AnnualQuota); err != nil {
					return err
				}
			}
			bal.Used = bal.Used.Add(days)
			if err := r.Balances.Save(bal); err != nil {
				return err
			}
			l.Deducted = true
		}
		if err := r.Leaves.Save(l); err != nil {
			return err
		}
	}

	return seedLeaveNotifications(r, byName)
}

// seedLeaveNotifications: beberapa notifikasi contoh agar lonceng langsung terisi.
func seedLeaveNotifications(r repository.Repos, byName map[string]*model.Employee) error {
	type n struct {
		To, Type, Title, Msg, Link string
		Read                       bool
	}
	items := []n{
		{"Bayu Pratama", service.EvLeaveSubmitted, "Pengajuan baru", "Rina Wulandari mengajukan Cuti tahunan (2 hari kerja). Menunggu persetujuan Anda.", "/persetujuan", false},
		{"Bayu Pratama", service.EvLeaveSubmitted, "Pengajuan baru", "Putri Maharani mengajukan Cuti tahunan (3 hari kerja). Menunggu persetujuan Anda.", "/persetujuan", false},
		{"Sinta Maharani", service.EvLeaveSubmitted, "Pengajuan baru", "Pengajuan dari Fajar Nugroho sudah disetujui atasan dan menunggu persetujuan Anda.", "/persetujuan", false},
		{"Hendra Gunawan", service.EvLeaveSubmitted, "Pengajuan baru", "Indah Permatasari mengajukan Cuti tahunan (3 hari kerja). Menunggu persetujuan Anda.", "/persetujuan", false},
		{"Rina Wulandari", service.EvLeaveApproved, "Pengajuan disetujui", "Pengajuan izin setengah hari Anda disetujui oleh Bayu Pratama.", "/izin-cuti/riwayat", true},
	}
	for _, it := range items {
		e := byName[it.To]
		if e == nil {
			continue
		}
		raw, _ := json.Marshal(map[string]any{"title": it.Title, "message": it.Msg, "link": it.Link})
		nt := &model.Notification{UserID: e.UserID, Type: it.Type, Data: string(raw)}
		if it.Read {
			t := time.Now()
			nt.ReadAt = &t
		}
		if err := r.Notifications.Create(nt); err != nil {
			return err
		}
	}
	return nil
}

func isOff(d time.Time, hol map[string]string) bool {
	return d.Weekday() == time.Saturday || d.Weekday() == time.Sunday || hol[d.Format("2006-01-02")] != ""
}

// nextWorkday: hari kerja pertama pada atau setelah d.
func nextWorkday(d time.Time, hol map[string]string) time.Time {
	d = service.DateOnly(d)
	for isOff(d, hol) {
		d = d.AddDate(0, 0, 1)
	}
	return d
}

// spanEnd: tanggal akhir bila rentang dimulai di start dan mencakup n hari kerja.
func spanEnd(start time.Time, n int, hol map[string]string) time.Time {
	d, counted := start, 1
	for counted < n {
		d = d.AddDate(0, 0, 1)
		if !isOff(d, hol) {
			counted++
		}
	}
	return d
}

// ensureSchedules membuat dua jadwal bawaan bila belum ada (di MySQL sudah diisi oleh migrasi 000003).
// "Jam kantor" (Sen–Jum 08.00–17.00) menjadi default. Jadwal kedua (Sen–Jum 09.00–17.00, Sab 09.00–15.00)
// disediakan untuk dipasang HR ke departemen yang bekerja pada hari Sabtu (mis. Customer Service) — penentuannya
// sengaja tidak ditebak oleh seeder.
func ensureSchedules(r repository.Repos) error {
	if _, err := r.Schedules.Default(); err == nil {
		return nil
	} else if err != repository.ErrNotFound {
		return err
	}
	mk := func(name string, def bool, start, satEnd string) error {
		m := &model.WorkSchedule{Name: name, IsDefault: def, BreakStart: "12:00", BreakEnd: "13:00"}
		for wd := 1; wd <= 5; wd++ {
			end := "17:00"
			m.Days = append(m.Days, model.WorkScheduleDay{Weekday: wd, StartTime: start, EndTime: end})
		}
		if satEnd != "" {
			m.Days = append(m.Days, model.WorkScheduleDay{Weekday: 6, StartTime: start, EndTime: satEnd})
		}
		return r.Schedules.Save(m)
	}
	if err := mk("Jam kantor (Sen-Jum 08.00-17.00)", true, "08:00", ""); err != nil {
		return err
	}
	return mk("Jam kantor + Sabtu (Sen-Jum 09.00-17.00, Sab 09.00-15.00)", false, "09:00", "15:00")
}
