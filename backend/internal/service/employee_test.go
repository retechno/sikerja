package service_test

import (
	"bytes"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"

	"github.com/retechno/sikerja/backend/internal/model"
	"github.com/retechno/sikerja/backend/internal/repository"
	"github.com/retechno/sikerja/backend/internal/service"
	"github.com/retechno/sikerja/backend/internal/storage"
	"github.com/retechno/sikerja/backend/internal/testutil"
)

func newEmpSvc(t *testing.T) (*service.EmployeeService, *testutil.World) {
	w := testutil.NewWorld(t)
	files, err := storage.NewLocal(w.Cfg.StorageDir)
	require.NoError(t, err)
	return service.NewEmployeeService(w.Store, files, service.NewCache(w.Rdb)), w
}

func xlsx(t *testing.T, rows ...[]any) *bytes.Reader {
	t.Helper()
	f := excelize.NewFile()
	_ = f.SetSheetName("Sheet1", "Karyawan")
	hdr := []any{"NIK", "Nama", "Email", "Telepon", "Kode Departemen", "Jabatan", "Email Atasan", "Tanggal Masuk", "Role", "Password"}
	require.NoError(t, f.SetSheetRow("Karyawan", "A1", &hdr))
	for i, r := range rows {
		r := r
		require.NoError(t, f.SetSheetRow("Karyawan", "A"+string(rune('2'+i)), &r))
	}
	var buf bytes.Buffer
	require.NoError(t, f.Write(&buf))
	return bytes.NewReader(buf.Bytes())
}

func countEmployees(t *testing.T, w *testutil.World) int64 {
	var n int64
	require.NoError(t, w.DB.Model(&model.Employee{}).Count(&n).Error)
	return n
}

func TestImportEmployeesSuccess(t *testing.T) {
	svc, w := newEmpSvc(t)
	before := countEmployees(t, w)
	file := xlsx(t,
		[]any{"2026-1001", "Putu Ayu", "putu.ayu@sikerja.test", "0811", "ITS", "IT Support Engineer", "bayu@test.id", "2026-01-15", "karyawan", ""},
		[]any{"2026-1002", "Wayan Sari", "wayan.sari@sikerja.test", "", "Customer Service", "Customer Service Officer", "putu.ayu@sikerja.test", "15/02/2026", "karyawan, manajer", "rahasia123"},
	)
	res, err := svc.ImportEmployees(w.Actor(w.HR), file)
	require.NoError(t, err)
	require.Empty(t, res.Errors)
	assert.Equal(t, 2, res.Created)
	assert.Equal(t, before+2, countEmployees(t, w))
	require.Len(t, res.Credentials, 1, "hanya baris tanpa Password yang dapat password sementara")
	assert.Equal(t, "putu.ayu@sikerja.test", res.Credentials[0].Email)
	assert.GreaterOrEqual(t, len(res.Credentials[0].TempPassword), 10)
	assert.Equal(t, 2, res.PositionsMade)

	// atasan dari baris lain & dari database terselesaikan
	var putu, wayan model.Employee
	require.NoError(t, w.DB.Where("nik = ?", "2026-1001").First(&putu).Error)
	require.NoError(t, w.DB.Where("nik = ?", "2026-1002").First(&wayan).Error)
	require.NotNil(t, putu.ManagerID)
	assert.Equal(t, w.MgrIT.ID, *putu.ManagerID)
	require.NotNil(t, wayan.ManagerID)
	assert.Equal(t, putu.ID, *wayan.ManagerID)
	assert.Equal(t, time.Date(2026, 2, 15, 0, 0, 0, 0, time.UTC).Format("2006-01-02"), wayan.JoinDate.Format("2006-01-02"))

	// dapat login dengan password sementara maupun password yang ditentukan
	_, _, err = w.Auth().Login(ctx, "putu.ayu@sikerja.test", res.Credentials[0].TempPassword, false)
	assert.NoError(t, err)
	_, _, err = w.Auth().Login(ctx, "wayan.sari@sikerja.test", "rahasia123", false)
	assert.NoError(t, err)
	u, err := w.Store.R().Users.ByEmail("wayan.sari@sikerja.test")
	require.NoError(t, err)
	var roles []string
	for _, r := range u.Roles {
		roles = append(roles, r.Name)
	}
	assert.ElementsMatch(t, []string{"employee", "manager"}, roles)
}

func TestImportEmployeesIsAllOrNothing(t *testing.T) {
	svc, w := newEmpSvc(t)
	before := countEmployees(t, w)
	file := xlsx(t,
		[]any{"2026-2001", "Baris Bagus", "bagus@sikerja.test", "", "ITS", "", "", "2026-01-15", "", ""},
		[]any{"T-0001", "NIK Kembar", "kembar@sikerja.test", "", "ITS", "", "", "2026-01-15", "", ""},
		[]any{"2026-2003", "Dept Salah", "dept@sikerja.test", "", "XXX", "", "", "2026-01-15", "", ""},
		[]any{"2026-2004", "Email Salah", "bukan-email", "", "ITS", "", "", "2026-01-15", "", ""},
		[]any{"2026-2005", "Tanggal Salah", "tgl@sikerja.test", "", "ITS", "", "", "kemarin sore", "", ""},
		[]any{"2026-2006", "Email Kembar", "rina@test.id", "", "ITS", "", "", "2026-01-15", "", ""},
		[]any{"2026-2007", "Atasan Hantu", "hantu@sikerja.test", "", "ITS", "", "tidakada@x.id", "2026-01-15", "", ""},
		[]any{"2026-2008", "Role Salah", "role@sikerja.test", "", "ITS", "", "", "2026-01-15", "bos", ""},
	)
	res, err := svc.ImportEmployees(w.Actor(w.HR), file)
	require.NoError(t, err)
	assert.Equal(t, 0, res.Created)
	assert.Equal(t, before, countEmployees(t, w), "tidak ada yang tersimpan, termasuk baris yang benar")

	rows := map[int]bool{}
	for _, e := range res.Errors {
		rows[e.Row] = true
	}
	assert.False(t, rows[2], "baris 2 benar")
	for _, r := range []int{3, 4, 5, 6, 7, 8, 9} {
		assert.True(t, rows[r], "baris %d harus dilaporkan salah", r)
	}
	var n int64
	w.DB.Model(&model.User{}).Where("email = ?", "bagus@sikerja.test").Count(&n)
	assert.Zero(t, n)
}

func TestImportRejectsNonExcelAndMissingColumns(t *testing.T) {
	svc, w := newEmpSvc(t)
	_, err := svc.ImportEmployees(w.Actor(w.HR), bytes.NewReader([]byte("nik,nama\n1,a")))
	assert.Equal(t, http.StatusUnprocessableEntity, status(t, err))

	f := excelize.NewFile()
	_ = f.SetSheetRow("Sheet1", "A1", &[]any{"Foo", "Bar"})
	var buf bytes.Buffer
	_ = f.Write(&buf)
	_, err = svc.ImportEmployees(w.Actor(w.HR), &buf)
	assert.Equal(t, http.StatusUnprocessableEntity, status(t, err))
}

func TestOnlyHRCanManageEmployees(t *testing.T) {
	svc, w := newEmpSvc(t)
	in := service.EmployeeInput{NIK: "X-1", Name: "Baru", Email: "baru@test.id", DepartmentID: &w.ITS.ID, JoinDate: time.Now()}
	for _, e := range []*model.Employee{w.EmpIT1, w.MgrIT, w.Director} {
		a := w.Actor(e)
		_, _, err := svc.Create(a, in)
		assert.Equal(t, http.StatusForbidden, status(t, err), e.Name)
		_, err = svc.Update(a, w.EmpIT2.ID, in)
		assert.Equal(t, http.StatusForbidden, status(t, err), e.Name)
		_, _, err = svc.List(a, repository.EmployeeFilter{})
		assert.Equal(t, http.StatusForbidden, status(t, err), e.Name)
		_, err = svc.ImportEmployees(a, xlsx(t))
		assert.Equal(t, http.StatusForbidden, status(t, err), e.Name)
		_, err = svc.ImportTemplate(a)
		assert.Equal(t, http.StatusForbidden, status(t, err), e.Name)
		assert.Equal(t, http.StatusForbidden, status(t, svc.Delete(a, w.EmpIT2.ID)), e.Name)
	}
}

func TestEmployeeGetIsScoped(t *testing.T) {
	svc, w := newEmpSvc(t)
	_, err := svc.Get(w.Actor(w.EmpIT1), w.EmpIT1.ID)
	assert.NoError(t, err)
	_, err = svc.Get(w.Actor(w.EmpIT1), w.EmpIT2.ID)
	assert.Equal(t, http.StatusForbidden, status(t, err))
	_, err = svc.Get(w.Actor(w.MgrIT), w.EmpCS1.ID)
	assert.Equal(t, http.StatusForbidden, status(t, err))
	_, err = svc.Get(w.Actor(w.MgrIT), w.EmpIT2.ID)
	assert.NoError(t, err)
	_, err = svc.Get(w.Actor(w.HR), w.EmpCS1.ID)
	assert.NoError(t, err)
}

func TestCreateUpdateEmployeeAndRoles(t *testing.T) {
	svc, w := newEmpSvc(t)
	hr := w.Actor(w.HR)
	in := service.EmployeeInput{NIK: "N-77", Name: "Joko", Email: "JOKO@test.id", DepartmentID: &w.ITS.ID, ManagerID: &w.MgrIT.ID,
		JoinDate: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)}
	v, temp, err := svc.Create(hr, in)
	require.NoError(t, err)
	assert.NotEmpty(t, temp)
	assert.Equal(t, "joko@test.id", v.Email)
	assert.Equal(t, []string{"employee"}, v.Roles)

	// duplikat
	_, _, err = svc.Create(hr, in)
	assert.Equal(t, http.StatusUnprocessableEntity, status(t, err))

	// jadikan manajer + pindah atasan; perubahan role tercatat di audit
	in.Roles = []string{"employee", "manager"}
	in.ManagerID = &w.Director.ID
	v, err = svc.Update(hr, v.ID, in)
	require.NoError(t, err)
	assert.Equal(t, []string{"employee", "manager"}, v.Roles)
	var logs []model.ActivityLog
	w.DB.Where("action = ?", service.AuditRoleChange).Find(&logs)
	assert.Len(t, logs, 1)

	// atasan tidak boleh membentuk siklus / diri sendiri
	in.ManagerID = &v.ID
	_, err = svc.Update(hr, v.ID, in)
	assert.Equal(t, http.StatusUnprocessableEntity, status(t, err))
	in.ManagerID = &w.EmpIT1.ID
	_, err = svc.Update(hr, w.MgrIT.ID, service.EmployeeInput{NIK: w.MgrIT.NIK, Name: w.MgrIT.Name, Email: "bayu@test.id",
		DepartmentID: &w.ITS.ID, ManagerID: &w.EmpIT1.ID, JoinDate: w.MgrIT.JoinDate, Roles: []string{"manager"}})
	assert.Equal(t, http.StatusUnprocessableEntity, status(t, err), "Rina (bawahan Bayu) tidak boleh menjadi atasan Bayu")

	// role tidak dikenal
	in.ManagerID = nil
	in.Roles = []string{"superuser"}
	_, err = svc.Update(hr, v.ID, in)
	assert.Equal(t, http.StatusUnprocessableEntity, status(t, err))
}

func TestHRCannotDeactivateOrRemoveOwnRole(t *testing.T) {
	svc, w := newEmpSvc(t)
	hr := w.Actor(w.HR)
	assert.Equal(t, http.StatusBadRequest, status(t, svc.Delete(hr, w.HR.ID)))
	_, err := svc.Update(hr, w.HR.ID, service.EmployeeInput{NIK: w.HR.NIK, Name: w.HR.Name, Email: "hr@test.id",
		JoinDate: w.HR.JoinDate, Roles: []string{"employee"}, Status: "active"})
	assert.Equal(t, http.StatusUnprocessableEntity, status(t, err))
}

func TestDeactivatedEmployeeCannotLogin(t *testing.T) {
	svc, w := newEmpSvc(t)
	hr := w.Actor(w.HR)
	pair, _, err := w.Auth().Login(ctx, "fajar@test.id", testutil.Password, false)
	require.NoError(t, err)
	require.NoError(t, svc.Delete(hr, w.EmpIT2.ID))
	_, _, err = w.Auth().Login(ctx, "fajar@test.id", testutil.Password, false)
	assert.Error(t, err)
	_, err = w.Auth().Authenticate(ctx, pair.Access)
	assert.Error(t, err, "token yang sudah terbit pun langsung tidak berlaku")
}

func TestImportTemplateIsValidWorkbook(t *testing.T) {
	svc, w := newEmpSvc(t)
	b, err := svc.ImportTemplate(w.Actor(w.HR))
	require.NoError(t, err)
	f, err := excelize.OpenReader(bytes.NewReader(b))
	require.NoError(t, err)
	rows, err := f.GetRows("Karyawan")
	require.NoError(t, err)
	assert.Equal(t, "NIK", rows[0][0])
	assert.Equal(t, "Role", rows[0][8])
	helpRows, err := f.GetRows("Petunjuk")
	require.NoError(t, err)
	codes := map[string]bool{}
	for _, r := range helpRows {
		if len(r) > 0 {
			codes[r[0]] = true
		}
	}
	assert.True(t, codes["ITS"] && codes["CS"], "daftar kode departemen ada di sheet Petunjuk")

	// template yang diisi lalu diunggah kembali harus lolos
	require.NoError(t, f.SetSheetRow("Karyawan", "A2", &[]any{"TPL-1", "Dari Template", "tpl@test.id", "", "ITS", "Staf", "", "2026-05-01", "", ""}))
	var buf bytes.Buffer
	require.NoError(t, f.Write(&buf))
	res, err := svc.ImportEmployees(w.Actor(w.HR), &buf)
	require.NoError(t, err)
	assert.Equal(t, 1, res.Created, "%+v", res.Errors)
}

func TestUpdateProfileAndPhoto(t *testing.T) {
	svc, w := newEmpSvc(t)
	rina := w.Actor(w.EmpIT1)
	v, err := svc.UpdateProfile(rina, "Rina W", "0812")
	require.NoError(t, err)
	assert.Equal(t, "Rina W", v.Name)
	_, err = svc.UpdateProfile(rina, " ", "")
	assert.Equal(t, http.StatusUnprocessableEntity, status(t, err))

	_, err = svc.SetPhoto(rina, w.EmpIT2.ID, bytes.NewReader(pngBytes()))
	assert.Equal(t, http.StatusForbidden, status(t, err), "tidak boleh mengganti foto orang lain")
	_, err = svc.SetPhoto(rina, rina.EmployeeID, bytes.NewReader([]byte("GIF89a....")))
	assert.Equal(t, http.StatusUnprocessableEntity, status(t, err))
	v, err = svc.SetPhoto(rina, rina.EmployeeID, bytes.NewReader(pngBytes()))
	require.NoError(t, err)
	assert.True(t, v.HasPhoto)
	f, err := svc.OpenPhoto(rina.EmployeeID)
	require.NoError(t, err)
	f.Close()
}
