package service

import (
	"bytes"
	"crypto/rand"
	"io"
	"math/big"
	"net/http"
	"net/mail"
	"os"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/retechno/sikerja/backend/internal/apperr"
	"github.com/retechno/sikerja/backend/internal/model"
	"github.com/retechno/sikerja/backend/internal/repository"
	"github.com/retechno/sikerja/backend/internal/storage"
)

var allRoles = []string{model.RoleEmployee, model.RoleManager, model.RoleHR, model.RoleDirector}

const maxPhotoBytes = 2 << 20

type EmployeeService struct {
	store repository.Store
	files storage.Store
	cache *Cache
}

func NewEmployeeService(store repository.Store, files storage.Store, cache *Cache) *EmployeeService {
	return &EmployeeService{store: store, files: files, cache: cache}
}

type EmployeeView struct {
	model.Employee
	Email string   `json:"email"`
	Roles []string `json:"roles"`
	// HasPhoto agar UI tahu perlu memuat foto atau menampilkan inisial.
	HasPhoto bool `json:"has_photo"`
}

func toView(e model.Employee) EmployeeView {
	v := EmployeeView{Employee: e, HasPhoto: e.Photo != ""}
	if e.User != nil {
		v.Email = e.User.Email
		for _, r := range e.User.Roles {
			v.Roles = append(v.Roles, r.Name)
		}
		slices.Sort(v.Roles)
	}
	return v
}

type EmployeeInput struct {
	NIK          string    `json:"nik"`
	Name         string    `json:"name"`
	Email        string    `json:"email"`
	Phone        string    `json:"phone"`
	DepartmentID *uint64   `json:"department_id"`
	PositionID   *uint64   `json:"position_id"`
	ManagerID    *uint64   `json:"manager_id"`
	JoinDate     time.Time `json:"-"`
	Status       string    `json:"status"`
	Roles        []string  `json:"roles"`
	Password     string    `json:"password"` // opsional saat membuat; kosong = dibuat acak
}

func requireHR(a Actor) error {
	if !a.IsHR() {
		return apperr.Forbidden("Hanya HR yang dapat mengelola karyawan")
	}
	return nil
}

func normalizeRoles(in []string) ([]string, error) {
	seen := map[string]bool{}
	var out []string
	for _, r := range in {
		r = strings.ToLower(strings.TrimSpace(r))
		if r == "" || seen[r] {
			continue
		}
		if !slices.Contains(allRoles, r) {
			return nil, apperr.Validation("Role tidak dikenal", map[string]string{"roles": "Role harus salah satu dari employee, manager, hr, director"})
		}
		seen[r] = true
		out = append(out, r)
	}
	if len(out) == 0 {
		out = []string{model.RoleEmployee}
	}
	return out, nil
}

// needsDepartment: karyawan dan manajer wajib punya departemen; HR/Direktur murni tidak.
func needsDepartment(roles []string) bool {
	return slices.Contains(roles, model.RoleEmployee) || slices.Contains(roles, model.RoleManager)
}

func (in *EmployeeInput) validate(creating bool) (map[string]string, error) {
	f := map[string]string{}
	in.NIK, in.Name, in.Phone = strings.TrimSpace(in.NIK), strings.TrimSpace(in.Name), strings.TrimSpace(in.Phone)
	if in.NIK == "" || utf8.RuneCountInString(in.NIK) > 30 {
		f["nik"] = "NIK wajib diisi (maks. 30 karakter)"
	}
	if in.Name == "" || utf8.RuneCountInString(in.Name) > 150 {
		f["name"] = "Nama wajib diisi (maks. 150 karakter)"
	}
	if addr, err := mail.ParseAddress(strings.TrimSpace(in.Email)); err != nil || addr.Address != strings.TrimSpace(in.Email) {
		f["email"] = "Email tidak valid"
	} else {
		in.Email = strings.ToLower(addr.Address)
	}
	if in.JoinDate.IsZero() {
		f["join_date"] = "Tanggal masuk wajib diisi"
	}
	if in.Status == "" {
		in.Status = model.StatusActive
	}
	if in.Status != model.StatusActive && in.Status != model.StatusInactive {
		f["status"] = "Status harus active atau inactive"
	}
	roles, err := normalizeRoles(in.Roles)
	if err != nil {
		return nil, err
	}
	in.Roles = roles
	if needsDepartment(roles) && in.DepartmentID == nil {
		f["department_id"] = "Departemen wajib diisi"
	}
	if creating && in.Password != "" {
		if err := ValidatePassword(in.Password); err != nil {
			f["password"] = "Minimal 8 karakter"
		}
	}
	return f, nil
}

// checkRefs memastikan departemen, jabatan, dan atasan valid (tanpa siklus atasan).
func checkRefs(r repository.Repos, in EmployeeInput, selfID uint64, f map[string]string) error {
	if in.DepartmentID != nil {
		if _, err := r.Departments.ByID(*in.DepartmentID); err != nil {
			f["department_id"] = "Departemen tidak ditemukan"
		}
	}
	if in.PositionID != nil {
		p, err := r.Positions.ByID(*in.PositionID)
		if err != nil {
			f["position_id"] = "Jabatan tidak ditemukan"
		} else if p.DepartmentID != nil && in.DepartmentID != nil && *p.DepartmentID != *in.DepartmentID {
			f["position_id"] = "Jabatan bukan milik departemen ini"
		}
	}
	if in.ManagerID != nil {
		if selfID != 0 && *in.ManagerID == selfID {
			f["manager_id"] = "Atasan tidak boleh diri sendiri"
		} else if m, err := r.Employees.ByID(*in.ManagerID); err != nil || m.Status != model.StatusActive {
			f["manager_id"] = "Atasan tidak ditemukan atau tidak aktif"
		} else if selfID != 0 {
			// cegah siklus: telusuri rantai atasan dari calon atasan
			cur, hops := m, 0
			for cur.ManagerID != nil && hops < 20 {
				if *cur.ManagerID == selfID {
					f["manager_id"] = "Atasan membentuk siklus"
					break
				}
				next, err := r.Employees.ByID(*cur.ManagerID)
				if err != nil {
					break
				}
				cur, hops = next, hops+1
			}
		}
	}
	return nil
}

const pwAlphabet = "abcdefghjkmnpqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789"

func randomPassword(n int) string {
	b := make([]byte, n)
	for i := range b {
		idx, _ := rand.Int(rand.Reader, big.NewInt(int64(len(pwAlphabet))))
		b[i] = pwAlphabet[idx.Int64()]
	}
	return string(b)
}

func (s *EmployeeService) List(a Actor, f repository.EmployeeFilter) ([]EmployeeView, int64, error) {
	if err := requireHR(a); err != nil {
		return nil, 0, err
	}
	es, total, err := s.store.R().Employees.List(f)
	if err != nil {
		return nil, 0, err
	}
	out := make([]EmployeeView, len(es))
	for i, e := range es {
		out[i] = toView(e)
	}
	return out, total, nil
}

func (s *EmployeeService) Get(a Actor, id uint64) (*EmployeeView, error) {
	e, err := loadEmployee(s.store.R(), id)
	if err != nil {
		return nil, err
	}
	if err := requireView(a, e); err != nil {
		return nil, err
	}
	v := toView(*e)
	return &v, nil
}

// Me = data karyawan milik actor.
func (s *EmployeeService) Me(a Actor) (*EmployeeView, error) {
	e, err := loadEmployee(s.store.R(), a.EmployeeID)
	if err != nil {
		return nil, err
	}
	v := toView(*e)
	return &v, nil
}

// Create membuat user + karyawan. Mengembalikan password sementara bila tidak diisi.
func (s *EmployeeService) Create(a Actor, in EmployeeInput) (*EmployeeView, string, error) {
	if err := requireHR(a); err != nil {
		return nil, "", err
	}
	f, err := in.validate(true)
	if err != nil {
		return nil, "", err
	}
	temp := ""
	pw := in.Password
	if pw == "" {
		temp = randomPassword(10)
		pw = temp
	}
	var out *EmployeeView
	err = s.store.Tx(func(r repository.Repos) error {
		if err := checkRefs(r, in, 0, f); err != nil {
			return err
		}
		if taken, err := r.Employees.NIKTaken(in.NIK, 0); err != nil {
			return err
		} else if taken {
			f["nik"] = "NIK sudah terdaftar"
		}
		if _, err := r.Users.ByEmail(in.Email); err == nil {
			f["email"] = "Email sudah terdaftar"
		}
		if len(f) > 0 {
			return apperr.Validation("Data karyawan tidak valid", f)
		}
		e, err := createEmployee(r, in, pw)
		if err != nil {
			return err
		}
		full, err := r.Employees.ByID(e.ID)
		if err != nil {
			return err
		}
		v := toView(*full)
		out = &v
		return logActivity(r, a, "employee.create", "employee", e.ID, nil, map[string]any{"nik": in.NIK, "email": in.Email, "roles": in.Roles})
	})
	if err != nil {
		return nil, "", err
	}
	s.cache.Bump(bgCtx())
	return out, temp, nil
}

func createEmployee(r repository.Repos, in EmployeeInput, password string) (*model.Employee, error) {
	hash, err := HashPassword(password)
	if err != nil {
		return nil, err
	}
	u := &model.User{Email: in.Email, Password: hash, IsActive: in.Status == model.StatusActive}
	if err := r.Users.Create(u); err != nil {
		return nil, err
	}
	if err := r.Users.SetRoles(u.ID, in.Roles); err != nil {
		return nil, err
	}
	e := &model.Employee{UserID: u.ID, NIK: in.NIK, Name: in.Name, Phone: in.Phone, DepartmentID: in.DepartmentID,
		PositionID: in.PositionID, ManagerID: in.ManagerID, JoinDate: in.JoinDate, Status: in.Status}
	if err := r.Employees.Create(e); err != nil {
		return nil, err
	}
	return e, nil
}

// Update mengubah data karyawan (termasuk atasan, status, dan role).
func (s *EmployeeService) Update(a Actor, id uint64, in EmployeeInput) (*EmployeeView, error) {
	if err := requireHR(a); err != nil {
		return nil, err
	}
	f, err := in.validate(false)
	if err != nil {
		return nil, err
	}
	var out *EmployeeView
	err = s.store.Tx(func(r repository.Repos) error {
		e, err := loadEmployee(r, id)
		if err != nil {
			return err
		}
		if err := checkRefs(r, in, e.ID, f); err != nil {
			return err
		}
		if taken, err := r.Employees.NIKTaken(in.NIK, e.ID); err != nil {
			return err
		} else if taken {
			f["nik"] = "NIK sudah terdaftar"
		}
		if other, err := r.Users.ByEmail(in.Email); err == nil && other.ID != e.UserID {
			f["email"] = "Email sudah terdaftar"
		}
		if e.ID == a.EmployeeID && in.Status != model.StatusActive {
			f["status"] = "Anda tidak dapat menonaktifkan akun sendiri"
		}
		if e.ID == a.EmployeeID && !slices.Contains(in.Roles, model.RoleHR) && a.IsHR() {
			f["roles"] = "Anda tidak dapat mencabut role HR dari akun sendiri"
		}
		if len(f) > 0 {
			return apperr.Validation("Data karyawan tidak valid", f)
		}
		before := toView(*e)
		e.NIK, e.Name, e.Phone, e.DepartmentID, e.PositionID = in.NIK, in.Name, in.Phone, in.DepartmentID, in.PositionID
		e.ManagerID, e.JoinDate, e.Status = in.ManagerID, in.JoinDate, in.Status
		if err := r.Employees.Update(e); err != nil {
			return err
		}
		if err := r.Users.UpdateEmail(e.UserID, in.Email); err != nil {
			return err
		}
		if err := r.Users.SetActive(e.UserID, in.Status == model.StatusActive); err != nil {
			return err
		}
		if err := r.Users.SetRoles(e.UserID, in.Roles); err != nil {
			return err
		}
		full, err := r.Employees.ByID(e.ID)
		if err != nil {
			return err
		}
		v := toView(*full)
		out = &v
		if !slices.Equal(before.Roles, v.Roles) {
			if err := logActivity(r, a, AuditRoleChange, "employee", e.ID, map[string]any{"roles": before.Roles}, map[string]any{"roles": v.Roles}); err != nil {
				return err
			}
		}
		return logActivity(r, a, "employee.update", "employee", e.ID,
			map[string]any{"name": before.Name, "department_id": before.DepartmentID, "manager_id": before.ManagerID, "status": before.Status},
			map[string]any{"name": v.Name, "department_id": v.DepartmentID, "manager_id": v.ManagerID, "status": v.Status})
	})
	if err == nil {
		s.cache.Bump(bgCtx())
	}
	return out, err
}

// Delete menonaktifkan akun lalu soft-delete karyawan (riwayat penilaian tetap ada).
func (s *EmployeeService) Delete(a Actor, id uint64) error {
	if err := requireHR(a); err != nil {
		return err
	}
	if id == a.EmployeeID {
		return apperr.BadRequest("Anda tidak dapat menghapus akun sendiri")
	}
	err := s.store.Tx(func(r repository.Repos) error {
		e, err := loadEmployee(r, id)
		if err != nil {
			return err
		}
		if err := r.Users.SetActive(e.UserID, false); err != nil {
			return err
		}
		if err := r.Employees.Delete(e.ID); err != nil {
			return err
		}
		return logActivity(r, a, "employee.delete", "employee", e.ID, map[string]any{"nik": e.NIK}, nil)
	})
	if err == nil {
		s.cache.Bump(bgCtx())
	}
	return err
}

// ---------- profil & foto ----------

// UpdateProfile: pengguna mengubah nama dan telepon miliknya sendiri.
func (s *EmployeeService) UpdateProfile(a Actor, name, phone string) (*EmployeeView, error) {
	name, phone = strings.TrimSpace(name), strings.TrimSpace(phone)
	f := map[string]string{}
	if name == "" || utf8.RuneCountInString(name) > 150 {
		f["name"] = "Nama wajib diisi (maks. 150 karakter)"
	}
	if utf8.RuneCountInString(phone) > 30 {
		f["phone"] = "Maksimal 30 karakter"
	}
	if len(f) > 0 {
		return nil, apperr.Validation("Profil tidak valid", f)
	}
	r := s.store.R()
	e, err := loadEmployee(r, a.EmployeeID)
	if err != nil {
		return nil, err
	}
	e.Name, e.Phone = name, phone
	if err := r.Employees.Update(e); err != nil {
		return nil, err
	}
	v := toView(*e)
	return &v, nil
}

var photoExt = map[string]string{"image/jpeg": ".jpg", "image/png": ".png"}

// SetPhoto: pemilik akun (atau HR) mengganti foto profil (JPG/PNG, maks. 2 MB).
func (s *EmployeeService) SetPhoto(a Actor, employeeID uint64, body io.Reader) (*EmployeeView, error) {
	if employeeID != a.EmployeeID && !a.IsHR() {
		return nil, errForbiddenData
	}
	data, err := io.ReadAll(io.LimitReader(body, maxPhotoBytes+1))
	if err != nil {
		return nil, err
	}
	ext, ok := photoExt[http.DetectContentType(data)]
	switch {
	case len(data) == 0:
		return nil, apperr.Validation("Berkas kosong", map[string]string{"file": "Berkas kosong"})
	case len(data) > maxPhotoBytes:
		return nil, apperr.Validation("Foto terlalu besar", map[string]string{"file": "Maksimal 2 MB"})
	case !ok:
		return nil, apperr.Validation("Jenis foto tidak didukung", map[string]string{"file": "Hanya JPG atau PNG"})
	}
	r := s.store.R()
	e, err := loadEmployee(r, employeeID)
	if err != nil {
		return nil, err
	}
	rel, err := s.files.Save("photos", ext, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	old := e.Photo
	e.Photo = rel
	if err := r.Employees.Update(e); err != nil {
		_ = s.files.Delete(rel)
		return nil, err
	}
	if old != "" {
		_ = s.files.Delete(old)
	}
	v := toView(*e)
	return &v, nil
}

// OpenPhoto: foto dapat dilihat semua pengguna yang login (hanya nama + foto, tanpa data sensitif).
func (s *EmployeeService) OpenPhoto(employeeID uint64) (*os.File, error) {
	e, err := loadEmployee(s.store.R(), employeeID)
	if err != nil {
		return nil, err
	}
	if e.Photo == "" {
		return nil, apperr.NotFound("Belum ada foto")
	}
	f, err := s.files.Open(e.Photo)
	if err != nil {
		return nil, apperr.NotFound("Foto tidak ditemukan")
	}
	return f, nil
}
