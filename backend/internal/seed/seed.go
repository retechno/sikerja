// Package seed mengisi data awal dan data demo (go run ./cmd/seed).
package seed

import (
	"errors"
	"fmt"
	"math"
	"math/rand"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	"github.com/retechno/sikerja/backend/internal/model"
	"github.com/retechno/sikerja/backend/internal/repository"
	"github.com/retechno/sikerja/backend/internal/service"
)

// Password semua akun demo.
const DemoPassword = "password"

var ErrAlreadySeeded = errors.New("database sudah berisi data; jalankan dengan -fresh untuk mengosongkan lebih dulu")

type Options struct {
	Now   time.Time // acuan "bulan berjalan"; default time.Now()
	Fresh bool      // kosongkan data lebih dulu
	Seed  int64     // sumber acak (default 42 → data demo selalu sama)
}

type Account struct {
	Name, Email, Role, Department string
}

// Result ringkasan untuk dicetak CLI/README.
type Result struct {
	Accounts []Account
	Periods  []string
}

type person struct {
	Name, Dept, Position string
	Role                 string // employee | manager | hr | director
	Skill                float64
	Join                 time.Time
}

var months = []string{"Januari", "Februari", "Maret", "April", "Mei", "Juni", "Juli", "Agustus", "September", "Oktober", "November", "Desember"}

// people: nama Indonesia. Skill = kemampuan dasar (memengaruhi realisasi & nilai perilaku).
var people = []person{
	{"Hendra Gunawan", "", "Direktur", model.RoleDirector, 1, d(2018, 1, 8)},
	{"Sinta Maharani", "", "Staf HR", model.RoleHR, 1, d(2019, 3, 4)},

	{"Bayu Pratama", "ITS", "Kepala IT Support", model.RoleManager, 1.02, d(2019, 7, 15)},
	{"Rina Wulandari", "ITS", "IT Support Engineer", model.RoleEmployee, 1.05, d(2021, 5, 3)},
	{"Dimas Saputra", "ITS", "IT Support Engineer", model.RoleEmployee, 1.08, d(2020, 2, 10)},
	{"Agus Setiawan", "ITS", "IT Support Engineer", model.RoleEmployee, 0.93, d(2022, 8, 1)},
	{"Putri Maharani", "ITS", "IT Support Engineer", model.RoleEmployee, 0.97, d(2022, 11, 14)},
	{"Fajar Nugroho", "ITS", "IT Support Engineer", model.RoleEmployee, 0.84, d(2023, 4, 3)},

	{"Indah Permatasari", "CS", "Kepala Customer Service", model.RoleManager, 1.03, d(2019, 9, 2)},
	{"Melati Anggraini", "CS", "Customer Service Officer", model.RoleEmployee, 1.07, d(2021, 1, 18)},
	{"Wahyu Hidayat", "CS", "Customer Service Officer", model.RoleEmployee, 0.88, d(2022, 6, 6)},
	{"Sari Puspita", "CS", "Customer Service Officer", model.RoleEmployee, 1.00, d(2021, 10, 25)},
	{"Eko Prasetyo", "CS", "Customer Service Officer", model.RoleEmployee, 0.95, d(2023, 2, 13)},
	{"Dewi Lestari", "CS", "Customer Service Officer", model.RoleEmployee, 1.01, d(2024, 1, 8)},

	{"Rudi Hartono", "SM", "Kepala Sales & Marketing", model.RoleManager, 0.99, d(2019, 5, 20)},
	{"Reza Firmansyah", "SM", "Account Executive", model.RoleEmployee, 0.80, d(2023, 7, 17)},
	{"Nanda Kusuma", "SM", "Account Executive", model.RoleEmployee, 0.86, d(2022, 9, 5)},
	{"Citra Lestari", "SM", "Account Executive", model.RoleEmployee, 0.90, d(2021, 12, 1)},
	{"Ayu Permata", "SM", "Account Executive", model.RoleEmployee, 0.98, d(2022, 3, 21)},
	{"Bagas Saputra", "SM", "Account Executive", model.RoleEmployee, 1.04, d(2020, 8, 10)},

	{"Satria Nugraha", "DEV", "Kepala Developer", model.RoleManager, 1.06, d(2018, 11, 12)},
	{"Larasati Dewi", "DEV", "Software Developer", model.RoleEmployee, 1.10, d(2020, 6, 1)},
	{"Yoga Prasetyo", "DEV", "Software Developer", model.RoleEmployee, 1.09, d(2021, 3, 8)},
	{"Andi Kurniawan", "DEV", "Software Developer", model.RoleEmployee, 0.96, d(2022, 1, 17)},
	{"Mega Wulan", "DEV", "Software Developer", model.RoleEmployee, 1.01, d(2023, 5, 22)},
	{"Tegar Aditya", "DEV", "Software Developer", model.RoleEmployee, 0.92, d(2024, 2, 5)},
}

func d(y, m, day int) time.Time { return time.Date(y, time.Month(m), day, 0, 0, 0, 0, time.UTC) }

func emailOf(name string) string {
	return strings.ReplaceAll(strings.ToLower(name), " ", ".") + "@sikerja.test"
}

// Run mengisi seluruh data. Aman dipanggil di database kosong; di database terisi mengembalikan ErrAlreadySeeded.
func Run(db *gorm.DB, opt Options) (*Result, error) {
	if opt.Now.IsZero() {
		opt.Now = time.Now()
	}
	if opt.Seed == 0 {
		opt.Seed = 42
	}
	if opt.Fresh {
		if err := wipe(db); err != nil {
			return nil, fmt.Errorf("mengosongkan data: %w", err)
		}
	}
	var n int64
	if err := db.Model(&model.Department{}).Count(&n).Error; err != nil {
		return nil, err
	}
	if n > 0 {
		return nil, ErrAlreadySeeded
	}

	res := &Result{}
	rng := rand.New(rand.NewSource(opt.Seed))
	store := repository.NewStore(db)
	err := store.Tx(func(r repository.Repos) error {
		if err := r.Users.EnsureRoles([]string{model.RoleEmployee, model.RoleManager, model.RoleHR, model.RoleDirector}); err != nil {
			return err
		}
		depts, err := seedDepartments(r)
		if err != nil {
			return err
		}
		emps, err := seedPeople(r, depts, res)
		if err != nil {
			return err
		}
		tpls, err := seedTemplates(r, depts)
		if err != nil {
			return err
		}
		aspects, err := seedAspects(r)
		if err != nil {
			return err
		}
		return seedPeriods(r, rng, opt.Now, emps, tpls, aspects, res)
	})
	return res, err
}

func wipe(db *gorm.DB) error {
	for _, q := range []string{
		"DELETE FROM activity_log", "DELETE FROM notifications", "DELETE FROM password_resets",
		"DELETE FROM appraisal_scores", "DELETE FROM appraisals", "DELETE FROM kpi_results",
		"DELETE FROM appraisal_periods", "DELETE FROM behavior_aspects", "DELETE FROM kpi_indicators",
		"DELETE FROM kpi_templates", "DELETE FROM user_roles", "UPDATE departments SET head_id = NULL",
		"UPDATE employees SET manager_id = NULL", "DELETE FROM employees", "DELETE FROM positions",
		"DELETE FROM departments", "DELETE FROM users",
	} {
		if err := db.Exec(q).Error; err != nil {
			return fmt.Errorf("%s: %w", q, err)
		}
	}
	return nil
}

func seedDepartments(r repository.Repos) (map[string]*model.Department, error) {
	out := map[string]*model.Department{}
	for _, def := range service.DefaultDepartments {
		dep := &model.Department{Name: def.Name, Code: def.Code}
		if err := r.Departments.Create(dep); err != nil {
			return nil, err
		}
		for _, p := range def.Positions {
			if err := r.Positions.Create(&model.Position{Name: p, DepartmentID: &dep.ID}); err != nil {
				return nil, err
			}
		}
		out[def.Code] = dep
	}
	// jabatan lintas departemen
	for _, p := range []string{"Direktur", "Staf HR"} {
		if err := r.Positions.Create(&model.Position{Name: p}); err != nil {
			return nil, err
		}
	}
	return out, nil
}

type emp struct {
	E      *model.Employee
	P      person
	Appr   *emp // penilai (atasan langsung)
	Scored bool // punya KPI & penilaian
}

func seedPeople(r repository.Repos, depts map[string]*model.Department, res *Result) ([]*emp, error) {
	hash, err := service.HashPassword(DemoPassword)
	if err != nil {
		return nil, err
	}
	positions, err := r.Positions.List(0)
	if err != nil {
		return nil, err
	}
	posByName := map[string]uint64{}
	for _, p := range positions {
		posByName[p.Name] = p.ID
	}

	var all []*emp
	byDept := map[string]*emp{}
	var director *emp
	for i, p := range people {
		u := &model.User{Email: emailOf(p.Name), Password: hash, IsActive: true}
		if err := r.Users.Create(u); err != nil {
			return nil, err
		}
		roles := []string{p.Role}
		if p.Role == model.RoleManager {
			roles = []string{model.RoleManager, model.RoleEmployee}
		} else if p.Role == model.RoleEmployee {
			roles = []string{model.RoleEmployee}
		}
		if err := r.Users.SetRoles(u.ID, roles); err != nil {
			return nil, err
		}
		e := &model.Employee{UserID: u.ID, NIK: fmt.Sprintf("%d-%04d", p.Join.Year(), 10+i*7), Name: p.Name,
			Phone: fmt.Sprintf("0812%08d", 31000000+i*1379), JoinDate: p.Join, Status: model.StatusActive}
		pid := posByName[p.Position]
		e.PositionID = &pid
		deptName := ""
		if p.Dept != "" {
			dep := depts[p.Dept]
			e.DepartmentID = &dep.ID
			deptName = dep.Name
		}
		x := &emp{E: e, P: p, Scored: p.Dept != ""}
		switch {
		case p.Role == model.RoleDirector:
			director = x
		case p.Role == model.RoleHR:
			x.Appr = director
		case p.Role == model.RoleManager:
			x.Appr = director
		default:
			x.Appr = byDept[p.Dept]
		}
		if x.Appr != nil {
			mid := x.Appr.E.ID
			e.ManagerID = &mid
		}
		if err := r.Employees.Create(e); err != nil {
			return nil, err
		}
		if p.Role == model.RoleManager {
			byDept[p.Dept] = x
			dep := depts[p.Dept]
			dep.HeadID = &e.ID
			if err := r.Departments.Update(dep); err != nil {
				return nil, err
			}
		}
		all = append(all, x)
		res.Accounts = append(res.Accounts, Account{Name: p.Name, Email: u.Email, Role: roleLabel(p.Role), Department: deptName})
	}
	return all, nil
}

func roleLabel(r string) string {
	switch r {
	case model.RoleDirector:
		return "Direktur"
	case model.RoleHR:
		return "HR"
	case model.RoleManager:
		return "Manajer"
	}
	return "Karyawan"
}

func seedTemplates(r repository.Repos, depts map[string]*model.Department) (map[uint64]*model.KpiTemplate, error) {
	out := map[uint64]*model.KpiTemplate{}
	for _, def := range service.DefaultDepartments {
		dep := depts[def.Code]
		t := &model.KpiTemplate{DepartmentID: dep.ID, Name: "Template KPI " + dep.Name, IsActive: true}
		for i, in := range service.DefaultKpiTemplates[def.Name] {
			t.Indicators = append(t.Indicators, model.KpiIndicator{
				Name: in.Name, Unit: in.Unit, Target: decimal.RequireFromString(in.Target), Weight: decimal.RequireFromString(in.Weight),
				Direction: in.Direction, Source: model.SourceManual, Sort: i + 1})
		}
		if err := r.Templates.Create(t); err != nil {
			return nil, err
		}
		out[dep.ID] = t
	}
	return out, nil
}

func seedAspects(r repository.Repos) ([]model.BehaviorAspect, error) {
	var out []model.BehaviorAspect
	for i, a := range service.DefaultAspects {
		asp := model.BehaviorAspect{Name: a.Name, Description: a.Description, Sort: i + 1}
		if err := r.Aspects.Create(&asp); err != nil {
			return nil, err
		}
		out = append(out, asp)
	}
	return out, nil
}

// ---------- periode, realisasi, penilaian ----------

func monthStart(now time.Time, back int) time.Time {
	return time.Date(now.Year(), now.Month()-time.Month(back), 1, 0, 0, 0, 0, time.UTC)
}

func seedPeriods(r repository.Repos, rng *rand.Rand, now time.Time, emps []*emp, tpls map[uint64]*model.KpiTemplate, aspects []model.BehaviorAspect, res *Result) error {
	var hr *emp
	for _, e := range emps {
		if e.P.Role == model.RoleHR {
			hr = e
		}
	}
	kpiCalc, appr := service.KpiScoreService{}, service.AppraisalService{}

	for back := 5; back >= 0; back-- {
		start := monthStart(now, back)
		end := start.AddDate(0, 1, -1)
		p := &model.AppraisalPeriod{Name: fmt.Sprintf("%s %d", months[start.Month()-1], start.Year()), Type: model.PeriodMonthly,
			StartDate: start, EndDate: end, Status: model.PeriodClosed, KpiWeight: decimal.NewFromInt(70), BehaviorWeight: decimal.NewFromInt(30)}
		active := back == 0
		if active {
			p.Status = model.PeriodActive
		}
		if err := r.Periods.Create(p); err != nil {
			return err
		}
		res.Periods = append(res.Periods, fmt.Sprintf("%s (%s)", p.Name, p.Status))
		hrUID := hr.E.UserID
		for _, act := range []string{service.AuditPeriodOpen, service.AuditPeriodClose} {
			if act == service.AuditPeriodClose && active {
				continue
			}
			if err := r.Audit.Log(&model.ActivityLog{UserID: &hrUID, Action: act, SubjectType: "appraisal_period", SubjectID: p.ID,
				OldValues: `{"status":"draft"}`, NewValues: `{"status":"` + map[bool]string{true: "active", false: "closed"}[act == service.AuditPeriodOpen] + `"}`,
				CreatedAt: start.Add(time.Hour)}); err != nil {
				return err
			}
		}

		for _, e := range emps {
			if !e.Scored {
				continue
			}
			tpl := tpls[*e.E.DepartmentID]
			if err := seedEmployeePeriod(r, rng, p, e, tpl, aspects, hr, active, back, kpiCalc, appr); err != nil {
				return err
			}
		}
	}
	return nil
}

var alwaysAppraised = map[string]bool{"Dimas Saputra": true, "Melati Anggraini": true, "Bagas Saputra": true, "Larasati Dewi": true}

func clamp(v, lo, hi float64) float64 { return math.Max(lo, math.Min(hi, v)) }

// realisation menghasilkan angka realisasi yang masuk akal untuk satu indikator.
func realisation(rng *rand.Rand, ind model.KpiIndicator, skill float64) decimal.Decimal {
	f := clamp(skill+rng.NormFloat64()*0.06, 0.5, 1.28)
	target, _ := ind.Target.Float64()
	var v float64
	if ind.Direction == service.DirectionLower {
		v = target * (2 - f)
	} else {
		v = target * f
	}
	switch ind.Unit {
	case "%":
		if target < 100 || strings.Contains(ind.Name, "tanpa eskalasi") || strings.Contains(ind.Name, "SLA") || strings.Contains(ind.Name, "CSAT") || strings.Contains(ind.Name, "tepat waktu") {
			v = math.Min(v, 100)
		}
		v = math.Round(v*10) / 10
	case "menit":
		v = math.Round(v*10) / 10
	case "dari 5":
		v = math.Round(math.Min(v, 5)*10) / 10
	default:
		v = math.Max(0, math.Round(v))
	}
	return decimal.NewFromFloat(v).Round(1)
}

var (
	mgrComments = []string{
		"Konsisten memenuhi target dan komunikasi dengan klien sangat baik.",
		"Kinerja stabil; perlu meningkatkan ketepatan waktu pada akhir bulan.",
		"Inisiatif meningkat dibanding bulan lalu dan aktif membantu rekan.",
		"Capaian beberapa KPI di bawah target; sudah disepakati rencana perbaikan.",
		"Sangat bertanggung jawab dan cepat belajar; layak diberi tugas yang lebih menantang.",
	}
	devPlans = []string{
		"Mengikuti pelatihan komunikasi dengan klien hotel.",
		"Menjadi mentor bagi anggota baru dan mengikuti sertifikasi teknis.",
		"Menyusun rencana peningkatan KPI yang masih di bawah target bersama atasan.",
		"Mengambil peran koordinator pada proyek instalasi berikutnya.",
	}
	selfNotes = []string{
		"Menyelesaikan seluruh tugas bulan ini tepat waktu dan membantu onboarding rekan baru.",
		"Fokus pada kepuasan klien; beberapa jadwal bergeser karena permintaan hotel.",
		"Menangani lonjakan pekerjaan di pertengahan bulan tanpa menurunkan kualitas.",
	}
)

func seedEmployeePeriod(r repository.Repos, rng *rand.Rand, p *model.AppraisalPeriod, e *emp, tpl *model.KpiTemplate,
	aspects []model.BehaviorAspect, hr *emp, active bool, back int, kpiCalc service.KpiScoreService, appr service.AppraisalService) error {

	skill := e.P.Skill + (float64(5-back))*0.004 // tren naik tipis
	approver := e.Appr
	approverID := approver.E.ID

	// --- nasib data bulan berjalan: sebagian belum lengkap/belum divalidasi ---
	state := "closed"
	if active {
		switch x := rng.Float64(); {
		case x < 0.20:
			state = "nothing"
		case x < 0.45:
			state = "self"
		case x < 0.70:
			state = "mgr"
		default:
			state = "kpi"
		}
	}
	isRina := active && e.P.Name == "Rina Wulandari"
	if isRina {
		state = "self"
	}
	// satu karyawan per departemen sudah dinilai atasan agar dasbor Direktur memuat keempat departemen
	if active && alwaysAppraised[e.P.Name] {
		state = "mgr"
	}

	var inputs []service.IndicatorInput
	for i, ind := range tpl.Indicators {
		in := service.IndicatorInput{IndicatorID: ind.ID, Target: ind.Target, Weight: ind.Weight, Direction: ind.Direction}
		status, make := model.ResultValidated, true
		if active {
			switch {
			case state == "nothing" && rng.Float64() < 0.7:
				make = false
			case rng.Float64() < 0.12:
				make = false
			case rng.Float64() < 0.35:
				status = model.ResultSubmitted
			}
		}
		actual := realisation(rng, ind, skill)
		if isRina { // angka mockup H-05
			actual = decimal.RequireFromString([]string{"104", "96.2", "12", "3", "4.7"}[i])
			status, make = model.ResultValidated, true
			if i == 3 {
				status = model.ResultSubmitted
			}
		}
		if !make {
			inputs = append(inputs, in)
			continue
		}
		ach, err := kpiCalc.Achievement(actual, ind.Target, ind.Direction)
		if err != nil {
			return err
		}
		a := actual
		in.Actual = &a
		inputs = append(inputs, in)
		at := p.StartDate.AddDate(0, 0, 20+rng.Intn(8))
		row := &model.KpiResult{PeriodID: p.ID, EmployeeID: e.E.ID, IndicatorID: ind.ID, Actual: actual, Achievement: ach, Status: status,
			SubmittedBy: &e.E.ID, Note: ""}
		row.CreatedAt, row.UpdatedAt = at, at
		if status == model.ResultValidated {
			vb := approverID
			row.ValidatedBy, row.ValidatedAt = &vb, &at
		}
		if err := r.Results.Upsert(row); err != nil {
			return err
		}
	}

	scored, err := kpiCalc.Score(inputs)
	if err != nil {
		return err
	}

	// --- penilaian ---
	if active && state == "nothing" {
		return nil
	}
	ap := &model.Appraisal{PeriodID: p.ID, EmployeeID: e.E.ID, Status: model.AppraisalDraft}
	if err := r.Appraisals.Create(ap); err != nil {
		return err
	}
	selfAt := p.StartDate.AddDate(0, 0, 22+rng.Intn(4))
	mgrAt := selfAt.AddDate(0, 0, 2)
	mgrScores := make([]int, len(aspects))
	for i := range aspects {
		mgrScores[i] = int(clamp(math.Round(3+(skill-0.95)*9+rng.NormFloat64()*0.6), 1, 5))
	}
	if isRina {
		mgrScores = []int{4, 5, 4, 4, 5}
	}
	writeScores := func(self, mgr bool) error {
		for i, as := range aspects {
			sc := model.AppraisalScore{AppraisalID: ap.ID, AspectID: as.ID}
			if self {
				v := int(clamp(float64(mgrScores[i]+rng.Intn(3)-1+boolInt(rng.Float64() < 0.5)), 1, 5))
				if isRina {
					v = []int{5, 4, 4, 4, 5}[i]
				}
				sc.SelfScore = &v
				if err := r.Appraisals.UpsertScore(&sc, "self_score"); err != nil {
					return err
				}
			}
			if mgr {
				v := mgrScores[i]
				sc.ManagerScore = &v
				if err := r.Appraisals.UpsertScore(&sc, "manager_score"); err != nil {
					return err
				}
			}
		}
		return nil
	}

	wantSelf := !active || state != "kpi"
	if active && state == "kpi" && rng.Float64() < 0.5 {
		wantSelf = true // sebagian sudah mulai self-assessment
	}
	if wantSelf {
		if err := writeScores(true, false); err != nil {
			return err
		}
		ap.Status, ap.SelfSubmittedAt = model.AppraisalSelfDone, &selfAt
		ap.EmployeeComment = selfNotes[rng.Intn(len(selfNotes))]
	}
	if !active || state == "mgr" {
		if err := writeScores(false, true); err != nil {
			return err
		}
		beh, err := appr.BehaviorScore(mgrScores)
		if err != nil {
			return err
		}
		fin, err := appr.FinalScore(scored.Score, beh, p.KpiWeight, p.BehaviorWeight)
		if err != nil {
			return err
		}
		ap.KpiScore = decimal.NullDecimal{Decimal: scored.Score, Valid: true}
		ap.BehaviorScore = decimal.NullDecimal{Decimal: beh, Valid: true}
		ap.FinalScore = decimal.NullDecimal{Decimal: fin, Valid: true}
		ap.Grade = appr.Grade(fin)
		ap.AppraiserID = &approverID
		ap.ManagerComment = mgrComments[rng.Intn(len(mgrComments))]
		ap.DevelopmentPlan = devPlans[rng.Intn(len(devPlans))]
		ap.Status, ap.MgrSubmittedAt = model.AppraisalManagerDone, &mgrAt
		if !active {
			fAt := mgrAt.AddDate(0, 0, 1)
			hid := hr.E.ID
			ap.Status, ap.FinalizedAt, ap.FinalizedBy = model.AppraisalFinal, &fAt, &hid
			if err := r.Audit.Log(&model.ActivityLog{UserID: &hr.E.UserID, Action: service.AuditFinalize, SubjectType: "appraisal", SubjectID: ap.ID,
				OldValues: `{"status":"manager_done"}`, NewValues: fmt.Sprintf(`{"status":"final","final_score":%q,"grade":%q}`, fin.String(), ap.Grade), CreatedAt: fAt}); err != nil {
				return err
			}
		}
	}
	return r.Appraisals.Save(ap)
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
