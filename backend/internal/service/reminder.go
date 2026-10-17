package service

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/retechno/sikerja/backend/internal/model"
	"github.com/retechno/sikerja/backend/internal/repository"
)

// reminderDays: pengingat dikirim saat tenggat tinggal sekian hari (sesuai pengaturan H-16).
var reminderDays = map[int]bool{3: true, 1: true}

// ReminderService mengirim pengingat tenggat penilaian; dijalankan harian oleh worker.
type ReminderService struct {
	store    repository.Store
	notifier Notifier
	rdb      *redis.Client
	now      func() time.Time
}

func NewReminderService(store repository.Store, notifier Notifier, rdb *redis.Client) *ReminderService {
	return &ReminderService{store: store, notifier: notifier, rdb: rdb, now: time.Now}
}

// SetClock mengganti jam (untuk test).
func (s *ReminderService) SetClock(f func() time.Time) { s.now = f }

// Run memeriksa semua periode aktif yang tenggatnya 3 atau 1 hari lagi dan mengirim pengingat
// kepada karyawan yang self-assessment-nya belum dikirim serta manajer yang masih punya anggota belum dinilai.
// Aman dijalankan berulang pada hari yang sama: satu periode hanya diproses sekali per hari (kunci Redis).
// Mengembalikan jumlah penerima.
func (s *ReminderService) Run(ctx context.Context) (int, error) {
	r := s.store.R()
	periods, err := r.Periods.List()
	if err != nil {
		return 0, err
	}
	today := DateOnly(s.now())
	sent := 0
	for _, p := range periods {
		if p.Status != model.PeriodActive {
			continue
		}
		left := int(DateOnly(p.EndDate).Sub(today).Hours() / 24)
		if !reminderDays[left] {
			continue
		}
		key := fmt.Sprintf("reminder:%d:%s", p.ID, today.Format("20060102"))
		first, err := s.rdb.SetNX(ctx, key, 1, 36*time.Hour).Result()
		if err != nil {
			return sent, err
		}
		if !first {
			continue
		}
		n, err := s.remindPeriod(ctx, r, &p, left)
		sent += n
		if err != nil {
			_ = s.rdb.Del(ctx, key).Err() // beri kesempatan mencoba lagi
			return sent, err
		}
	}
	return sent, nil
}

func (s *ReminderService) remindPeriod(ctx context.Context, r repository.Repos, p *model.AppraisalPeriod, left int) (int, error) {
	emps, err := r.Employees.AllActive()
	if err != nil {
		return 0, err
	}
	ids := make([]uint64, 0, len(emps))
	for _, e := range emps {
		ids = append(ids, e.ID)
	}
	aps, err := r.Appraisals.ForEmployees(p.ID, ids)
	if err != nil {
		return 0, err
	}
	status := map[uint64]string{}
	for _, a := range aps {
		status[a.EmployeeID] = a.Status
	}
	when := "besok"
	if left > 1 {
		when = fmt.Sprintf("%d hari lagi", left)
	}
	deadline := fmtDate(DateOnly(p.EndDate))

	// anggota yang belum dinilai atasan, per departemen
	pendingByDept := map[uint64]int{}
	for _, e := range emps {
		if e.DepartmentID == nil || hasRole(&e, model.RoleManager) {
			continue
		}
		if st := status[e.ID]; st != model.AppraisalManagerDone && st != model.AppraisalFinal {
			pendingByDept[*e.DepartmentID]++
		}
	}

	sent := 0
	for _, e := range emps {
		if e.DepartmentID == nil {
			continue
		}
		st := status[e.ID]
		if st == "" || st == model.AppraisalDraft {
			s.notifier.Notify(ctx, Notice{
				Type:    EvDeadline,
				Title:   "Pengingat penilaian " + p.Name,
				Message: fmt.Sprintf("Periode %s berakhir %s (%s). Lengkapi realisasi KPI dan self-assessment Anda.", p.Name, when, deadline),
				Link:    "/penilaian/saya",
				Data:    map[string]any{"period_id": p.ID},
			}, e)
			sent++
		}
		if hasRole(&e, model.RoleManager) {
			if n := pendingByDept[*e.DepartmentID]; n > 0 {
				s.notifier.Notify(ctx, Notice{
					Type:    EvDeadline,
					Title:   "Pengingat penilaian tim " + p.Name,
					Message: fmt.Sprintf("Periode %s berakhir %s (%s). Masih ada %d anggota tim yang belum Anda nilai.", p.Name, when, deadline, n),
					Link:    "/penilaian/tim",
					Data:    map[string]any{"period_id": p.ID, "pending": n},
				}, e)
				sent++
			}
		}
	}
	return sent, nil
}
