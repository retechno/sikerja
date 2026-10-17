package seed_test

import (
	"context"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"github.com/retechno/sikerja/backend/internal/model"
	"github.com/retechno/sikerja/backend/internal/repository"
	"github.com/retechno/sikerja/backend/internal/seed"
	"github.com/retechno/sikerja/backend/internal/service"
	"github.com/retechno/sikerja/backend/internal/testutil"
)

var now = time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)

// Tabel "KPI default per departemen" dari docs/SPEC.md, disalin apa adanya (independen dari kode seeder).
var specTable = []struct {
	Dept, Name, Target, Direction, Weight string
}{
	{"IT Support", "Tiket terselesaikan", "120", "higher", "25"},
	{"IT Support", "Tiket selesai dalam SLA", "95", "higher", "25"},
	{"IT Support", "Rata-rata waktu respons pertama", "15", "lower", "15"},
	{"IT Support", "Instalasi / training PMS di hotel klien", "4", "higher", "20"},
	{"IT Support", "Rating kepuasan klien", "4.5", "higher", "15"},
	{"Customer Service", "Rata-rata waktu respons chat/telepon", "5", "lower", "25"},
	{"Customer Service", "Kasus selesai tanpa eskalasi", "80", "higher", "25"},
	{"Customer Service", "Skor kepuasan (CSAT)", "90", "higher", "30"},
	{"Customer Service", "Jumlah kasus ditangani", "300", "higher", "20"},
	{"Sales & Marketing", "Pendapatan tercapai vs target", "100", "higher", "35"},
	{"Sales & Marketing", "Hotel klien baru", "3", "higher", "20"},
	{"Sales & Marketing", "Demo PMS dilakukan", "12", "higher", "15"},
	{"Sales & Marketing", "Konversi lead ke deal", "20", "higher", "15"},
	{"Sales & Marketing", "Perpanjangan kontrak klien", "90", "higher", "15"},
	{"Developer", "Task selesai tepat waktu", "90", "higher", "30"},
	{"Developer", "Story point diselesaikan", "40", "higher", "20"},
	{"Developer", "Bug setelah rilis", "3", "lower", "25"},
	{"Developer", "Rilis sesuai jadwal", "100", "higher", "15"},
	{"Developer", "Code review dikerjakan", "15", "higher", "10"},
}

func seeded(t *testing.T) (*testutil.World, *seed.Result) {
	t.Helper()
	w := testutil.NewWorld(t)
	// NewWorld sudah mengisi data mini; kosongkan untuk menguji seeder di database bersih.
	res, err := seed.Run(w.DB, seed.Options{Now: now, Fresh: true})
	require.NoError(t, err)
	return w, res
}

func TestSeedStructure(t *testing.T) {
	w, res := seeded(t)
	count := func(m any) int64 {
		var n int64
		require.NoError(t, w.DB.Model(m).Count(&n).Error)
		return n
	}
	assert.EqualValues(t, 4, count(&model.Department{}))
	assert.EqualValues(t, 26, count(&model.Employee{}), "1 direktur + 1 HR + 4×(1 manajer + 5 karyawan)")
	assert.EqualValues(t, 26, count(&model.User{}))
	assert.Len(t, res.Accounts, 26)
	assert.EqualValues(t, 4, count(&model.KpiTemplate{}))
	assert.EqualValues(t, 19, count(&model.KpiIndicator{}))
	assert.EqualValues(t, 5, count(&model.BehaviorAspect{}))
	assert.EqualValues(t, 6, count(&model.AppraisalPeriod{}))

	byRole := map[string]int{}
	for _, a := range res.Accounts {
		byRole[a.Role]++
	}
	assert.Equal(t, map[string]int{"Direktur": 1, "HR": 1, "Manajer": 4, "Karyawan": 20}, byRole)

	var perDept []struct {
		DepartmentID uint64
		N            int
	}
	w.DB.Model(&model.Employee{}).Select("department_id, count(*) as n").Where("department_id IS NOT NULL").Group("department_id").Scan(&perDept)
	require.Len(t, perDept, 4)
	for _, p := range perDept {
		assert.Equal(t, 6, p.N, "1 manajer + 5 karyawan per departemen")
	}
	var heads int64
	w.DB.Model(&model.Department{}).Where("head_id IS NOT NULL").Count(&heads)
	assert.EqualValues(t, 4, heads)
}

func TestSeedKpiTemplatesMatchSpecExactly(t *testing.T) {
	w, _ := seeded(t)
	r := w.Store.R()
	depts, _ := r.Departments.List()
	var got []struct{ Dept, Name, Target, Direction, Weight string }
	for _, d := range depts {
		tpl, err := r.Templates.ActiveByDepartment(d.ID)
		require.NoError(t, err)
		sum := decimal.Zero
		for _, i := range tpl.Indicators {
			sum = sum.Add(i.Weight)
			got = append(got, struct{ Dept, Name, Target, Direction, Weight string }{d.Name, i.Name, i.Target.String(), i.Direction, i.Weight.String()})
		}
		assert.True(t, sum.Equal(decimal.NewFromInt(100)), "%s total bobot %s", d.Name, sum)
	}
	assert.ElementsMatch(t, specTable, got)
}

func TestSeedPeriods5ClosedAnd1Active(t *testing.T) {
	w, res := seeded(t)
	ps, err := w.Store.R().Periods.List() // terbaru dulu
	require.NoError(t, err)
	require.Len(t, ps, 6)
	assert.Equal(t, "Oktober 2026", ps[0].Name)
	assert.Equal(t, model.PeriodActive, ps[0].Status)
	for _, p := range ps[1:] {
		assert.Equal(t, model.PeriodClosed, p.Status, p.Name)
	}
	assert.Equal(t, "Mei 2026", ps[5].Name)
	assert.True(t, ps[0].KpiWeight.Equal(decimal.NewFromInt(70)))
	assert.Len(t, res.Periods, 6)
}

func TestSeedClosedPeriodsAreFullyFinalAndConsistent(t *testing.T) {
	w, _ := seeded(t)
	r := w.Store.R()
	ps, _ := r.Periods.List()
	appr := service.AppraisalService{}
	for _, p := range ps[1:] {
		aps, err := r.Reports.ScoresForPeriod(p.ID, 0, nil)
		require.NoError(t, err)
		require.Len(t, aps, 24, p.Name)
		for _, row := range aps {
			require.NotNil(t, row.Status, "%s %s", p.Name, row.EmployeeName)
			assert.Equal(t, model.AppraisalFinal, *row.Status)
			require.True(t, row.FinalScore.Valid)
			// skor akhir = 70% KPI + 30% perilaku dan predikat sesuai tabel
			want, err := appr.FinalScore(row.KpiScore.Decimal, row.BehaviorScore.Decimal, p.KpiWeight, p.BehaviorWeight)
			require.NoError(t, err)
			assert.True(t, want.Equal(row.FinalScore.Decimal), "%s %s: %s vs %s", p.Name, row.EmployeeName, want, row.FinalScore.Decimal)
			assert.Equal(t, appr.Grade(row.FinalScore.Decimal), *row.Grade)
			assert.True(t, row.FinalScore.Decimal.GreaterThan(decimal.NewFromInt(30)) && row.FinalScore.Decimal.LessThan(decimal.NewFromInt(120)), row.FinalScore.Decimal.String())
		}
		// semua realisasi tervalidasi pada periode tertutup
		var notValid int64
		w.DB.Model(&model.KpiResult{}).Where("period_id = ? AND status <> ?", p.ID, model.ResultValidated).Count(&notValid)
		assert.Zero(t, notValid, p.Name)
	}
}

func TestSeedActivePeriodIsInProgress(t *testing.T) {
	w, _ := seeded(t)
	r := w.Store.R()
	ps, _ := r.Periods.List()
	rows, err := r.Reports.ScoresForPeriod(ps[0].ID, 0, nil)
	require.NoError(t, err)
	statuses := map[string]int{}
	for _, row := range rows {
		s := "none"
		if row.Status != nil {
			s = *row.Status
		}
		statuses[s]++
	}
	assert.Zero(t, statuses[model.AppraisalFinal], "periode berjalan belum ada yang final")
	assert.Positive(t, statuses[model.AppraisalSelfDone]+statuses[model.AppraisalDraft])
	assert.Positive(t, statuses[model.AppraisalManagerDone], "ada yang sudah dinilai atasan agar dasbor terisi")
	var submitted, validated int64
	w.DB.Model(&model.KpiResult{}).Where("period_id = ? AND status = ?", ps[0].ID, model.ResultSubmitted).Count(&submitted)
	w.DB.Model(&model.KpiResult{}).Where("period_id = ? AND status = ?", ps[0].ID, model.ResultValidated).Count(&validated)
	assert.Positive(t, submitted, "ada realisasi menunggu validasi")
	assert.Positive(t, validated)
}

func TestSeedRinaMatchesMockupH05(t *testing.T) {
	w, _ := seeded(t)
	auth := w.Auth()
	pair, _, err := auth.Login(context.Background(), "rina.wulandari@sikerja.test", seed.DemoPassword, false)
	require.NoError(t, err)
	actor, err := auth.Authenticate(context.Background(), pair.Access)
	require.NoError(t, err)

	store := w.Store
	cache := service.NewCache(w.Rdb)
	ps, _ := store.R().Periods.List()
	sh, err := service.NewKpiService(store, nil, cache).Sheet(*actor, actor.EmployeeID, ps[0].ID)
	require.NoError(t, err)
	assert.Equal(t, "95,7", service.FormatScore(sh.Score))
	assert.Equal(t, 4, sh.Validated)
	assert.Equal(t, 1, sh.Pending)

	app, err := service.NewAppraisalService(store, cache).Get(*actor, actor.EmployeeID, ps[0].ID)
	require.NoError(t, err)
	assert.Equal(t, model.AppraisalSelfDone, app.Status)
}

func TestSeedEveryAccountCanLogInWithDemoPassword(t *testing.T) {
	w, res := seeded(t)
	for _, a := range res.Accounts {
		_, _, err := w.Auth().Login(context.Background(), a.Email, "password", false)
		assert.NoError(t, err, a.Email)
	}
}

func TestSeedDashboardsAreFilledForEveryRole(t *testing.T) {
	w, _ := seeded(t)
	store := w.Store
	cache := service.NewCache(w.Rdb)
	dash := service.NewDashboardService(store, cache)
	login := func(email string) service.Actor {
		pair, _, err := w.Auth().Login(context.Background(), email, "password", false)
		require.NoError(t, err)
		a, err := w.Auth().Authenticate(context.Background(), pair.Access)
		require.NoError(t, err)
		return *a
	}

	dir, err := dash.Director(login("hendra.gunawan@sikerja.test"), 0)
	require.NoError(t, err)
	assert.Equal(t, "Oktober 2026", dir.Period.Name)
	assert.Len(t, dir.Departments, 4)
	assert.Len(t, dir.Trend, 6)
	for _, tp := range dir.Trend[:5] {
		assert.NotNil(t, tp.Avg, tp.Name)
	}
	assert.Len(t, dir.Top, 5)
	assert.Len(t, dir.Bottom, 5)
	assert.Equal(t, 24, dir.Employees)

	// periode sebelumnya (final) lengkap: sebaran predikat berjumlah 24
	ps, _ := store.R().Periods.List()
	prev, err := dash.Director(login("hendra.gunawan@sikerja.test"), ps[1].ID)
	require.NoError(t, err)
	total := int64(0)
	for _, g := range prev.Grades {
		total += g.Count
	}
	assert.EqualValues(t, 24, total)
	require.NotNil(t, prev.CompanyAvg)
	require.NotNil(t, prev.BestDepartment)

	mgr, err := dash.Manager(login("bayu.pratama@sikerja.test"), 0)
	require.NoError(t, err)
	assert.Equal(t, 5, mgr.MembersTotal)
	assert.Len(t, mgr.Ranking, 5)
	assert.NotNil(t, mgr.PrevAvg)

	emp, err := dash.Employee(login("rina.wulandari@sikerja.test"), 0)
	require.NoError(t, err)
	assert.Len(t, emp.History, 6)
	assert.Len(t, emp.Sheet.Rows, 5)
	assert.NotNil(t, emp.Latest, "ada skor final bulan lalu")
}

func TestSeedIsDeterministicAndRefusesToOverwrite(t *testing.T) {
	w1, _ := seeded(t)
	finals := func(w *testutil.World) []string {
		var out []string
		var aps []model.Appraisal
		w.DB.Order("period_id, employee_id").Find(&aps)
		for _, a := range aps {
			out = append(out, a.FinalScore.Decimal.String()+"|"+a.Status)
		}
		return out
	}
	a := finals(w1)
	w2, _ := seeded(t)
	assert.Equal(t, a, finals(w2), "seed yang sama menghasilkan data yang sama")

	_, err := seed.Run(w1.DB, seed.Options{Now: now})
	assert.ErrorIs(t, err, seed.ErrAlreadySeeded)
	_ = repository.ErrNotFound
}

func TestBootstrapCreatesMasterDataAndOneHRAccountOnly(t *testing.T) {
	db := testutil.NewDB(t)
	require.Error(t, seed.Bootstrap(db, seed.BootstrapOptions{Email: "hr@perusahaan.id", Name: "HR", Password: "pendek"}))
	require.Error(t, seed.Bootstrap(db, seed.BootstrapOptions{Email: "bukan-email", Name: "HR", Password: "password-panjang"}))

	require.NoError(t, seed.Bootstrap(db, seed.BootstrapOptions{Email: " HR@Perusahaan.id ", Name: "Admin HR", Password: "password-panjang"}))

	count := func(m any) int64 {
		var n int64
		require.NoError(t, db.Model(m).Count(&n).Error)
		return n
	}
	assert.EqualValues(t, 1, count(&model.User{}), "hanya satu akun, tanpa akun demo")
	assert.EqualValues(t, 1, count(&model.Employee{}))
	assert.EqualValues(t, 4, count(&model.Department{}))
	assert.EqualValues(t, 4, count(&model.KpiTemplate{}))
	assert.EqualValues(t, 5, count(&model.BehaviorAspect{}))
	assert.Positive(t, count(&model.LeaveType{}))
	assert.Positive(t, count(&model.Holiday{}))
	assert.Zero(t, count(&model.AppraisalPeriod{}), "tanpa periode demo")
	assert.Zero(t, count(&model.LeaveRequest{}), "tanpa contoh pengajuan")

	u, err := repository.NewStore(db).R().Users.ByEmail("hr@perusahaan.id")
	require.NoError(t, err)
	require.Len(t, u.Roles, 1)
	assert.Equal(t, model.RoleHR, u.Roles[0].Name)
	assert.NoError(t, bcrypt.CompareHashAndPassword([]byte(u.Password), []byte("password-panjang")))

	// kedua kali: tidak menimpa apa pun
	assert.ErrorIs(t, seed.Bootstrap(db, seed.BootstrapOptions{Email: "lain@x.id", Name: "X", Password: "password-panjang"}), seed.ErrAlreadySeeded)
}
