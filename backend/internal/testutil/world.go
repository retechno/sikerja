package testutil

import (
	"fmt"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	"github.com/retechno/sikerja/backend/internal/config"
	"github.com/retechno/sikerja/backend/internal/model"
	"github.com/retechno/sikerja/backend/internal/repository"
	"github.com/retechno/sikerja/backend/internal/service"
)

// Password semua akun test.
const Password = "password"

// World adalah data mini untuk test integrasi service: 2 departemen (IT Support, Customer Service),
// HR, Direktur, 1 manajer + 2 karyawan IT, 1 manajer + 1 karyawan CS, periode aktif & tertutup.
type World struct {
	T      testing.TB
	DB     *gorm.DB
	Store  repository.Store
	Rdb    *redis.Client
	MR     *miniredis.Miniredis
	Cfg    config.Config
	Mailer *FakeMailer

	nikSeq int

	ITS, CS *model.Department

	HR, Director          *model.Employee
	MgrIT, EmpIT1, EmpIT2 *model.Employee
	MgrCS, EmpCS1         *model.Employee

	TplIT, TplCS *model.KpiTemplate
	Active       *model.AppraisalPeriod
	Closed       *model.AppraisalPeriod
	Aspects      []model.BehaviorAspect
}

// FakeMailer menampung email reset yang "terkirim".
type FakeMailer struct{ Sent []Sent }

type Sent struct{ To, Name, Link string }

func (f *FakeMailer) SendPasswordReset(to, name, link string) error {
	f.Sent = append(f.Sent, Sent{to, name, link})
	return nil
}

func NewWorld(t testing.TB) *World {
	t.Helper()
	db := NewDB(t)
	rdb, mr := NewRedis(t)
	w := &World{T: t, DB: db, Store: repository.NewStore(db), Rdb: rdb, MR: mr, Mailer: &FakeMailer{}}
	w.Cfg, _ = config.Load()
	w.Cfg.JWTSecret = "test-secret-test-secret-test-secret-123"
	w.Cfg.StorageDir = t.TempDir()

	r := w.Store.R()
	must(t, r.Users.EnsureRoles([]string{model.RoleEmployee, model.RoleManager, model.RoleHR, model.RoleDirector}))

	w.ITS = &model.Department{Name: "IT Support", Code: "ITS"}
	w.CS = &model.Department{Name: "Customer Service", Code: "CS"}
	must(t, r.Departments.Create(w.ITS))
	must(t, r.Departments.Create(w.CS))

	w.Director = w.person("Dewi Direktur", "direktur@test.id", nil, nil, model.RoleDirector)
	w.HR = w.person("Hana HR", "hr@test.id", nil, &w.Director.ID, model.RoleHR, model.RoleEmployee)
	w.MgrIT = w.person("Bayu Manajer", "bayu@test.id", &w.ITS.ID, &w.Director.ID, model.RoleManager, model.RoleEmployee)
	w.EmpIT1 = w.person("Rina Karyawan", "rina@test.id", &w.ITS.ID, &w.MgrIT.ID, model.RoleEmployee)
	w.EmpIT2 = w.person("Fajar Karyawan", "fajar@test.id", &w.ITS.ID, &w.MgrIT.ID, model.RoleEmployee)
	w.MgrCS = w.person("Citra Manajer", "citra@test.id", &w.CS.ID, &w.Director.ID, model.RoleManager, model.RoleEmployee)
	w.EmpCS1 = w.person("Dimas Karyawan", "dimas@test.id", &w.CS.ID, &w.MgrCS.ID, model.RoleEmployee)

	w.TplIT = w.template(w.ITS, "IT Support")
	w.TplCS = w.template(w.CS, "Customer Service")

	for i, a := range service.DefaultAspects {
		asp := model.BehaviorAspect{Name: a.Name, Description: a.Description, Sort: i + 1}
		must(t, r.Aspects.Create(&asp))
		w.Aspects = append(w.Aspects, asp)
	}

	w.Closed = &model.AppraisalPeriod{Name: "September 2026", Type: model.PeriodMonthly,
		StartDate: date(2026, 9, 1), EndDate: date(2026, 9, 30), Status: model.PeriodClosed,
		KpiWeight: decimal.NewFromInt(70), BehaviorWeight: decimal.NewFromInt(30)}
	w.Active = &model.AppraisalPeriod{Name: "Oktober 2026", Type: model.PeriodMonthly,
		StartDate: date(2026, 10, 1), EndDate: date(2026, 10, 31), Status: model.PeriodActive,
		KpiWeight: decimal.NewFromInt(70), BehaviorWeight: decimal.NewFromInt(30)}
	must(t, r.Periods.Create(w.Closed))
	must(t, r.Periods.Create(w.Active))
	return w
}

func date(y, m, d int) time.Time { return time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.UTC) }

func (w *World) person(name, email string, dept, manager *uint64, roles ...string) *model.Employee {
	t := w.T
	t.Helper()
	hash, err := service.HashPassword(Password)
	must(t, err)
	r := w.Store.R()
	u := &model.User{Email: email, Password: hash, IsActive: true}
	must(t, r.Users.Create(u))
	must(t, r.Users.SetRoles(u.ID, roles))
	w.nikSeq++
	e := &model.Employee{UserID: u.ID, NIK: fmt.Sprintf("T-%04d", w.nikSeq), Name: name, DepartmentID: dept,
		ManagerID: manager, JoinDate: date(2021, 5, 3), Status: model.StatusActive}
	must(t, r.Employees.Create(e))
	return e
}

func (w *World) template(d *model.Department, key string) *model.KpiTemplate {
	t := w.T
	t.Helper()
	tpl := &model.KpiTemplate{DepartmentID: d.ID, Name: "Template " + key, IsActive: true}
	for i, in := range service.DefaultKpiTemplates[key] {
		tpl.Indicators = append(tpl.Indicators, model.KpiIndicator{
			Name: in.Name, Unit: in.Unit, Target: decimal.RequireFromString(in.Target),
			Weight: decimal.RequireFromString(in.Weight), Direction: in.Direction,
			Source: model.SourceManual, Sort: i + 1})
	}
	must(t, w.Store.R().Templates.Create(tpl))
	return tpl
}

// Actor membentuk Actor langsung dari karyawan (tanpa lewat login).
func (w *World) Actor(e *model.Employee) service.Actor {
	a, err := w.Auth().ActorFor(e.UserID)
	must(w.T, err)
	return *a
}

func (w *World) Auth() *service.AuthService {
	return service.NewAuthService(w.Store, w.Rdb, w.Cfg, w.Mailer)
}

func must(t testing.TB, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
}
