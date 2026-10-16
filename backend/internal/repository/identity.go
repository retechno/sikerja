package repository

import (
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/retechno/sikerja/backend/internal/model"
)

// ---------- users ----------

type UserRepo interface {
	ByEmail(email string) (*model.User, error)
	ByID(id uint64) (*model.User, error)
	Create(u *model.User) error
	SetRoles(userID uint64, roles []string) error
	UpdatePassword(userID uint64, hash string) error
	SetActive(userID uint64, active bool) error
	UpdateEmail(userID uint64, email string) error
	TouchLogin(userID uint64, at time.Time) error
	EnsureRoles(names []string) error
}

type userRepo struct{ db *gorm.DB }

func (r *userRepo) ByEmail(email string) (*model.User, error) {
	var u model.User
	err := r.db.Preload("Roles").Where("email = ?", strings.ToLower(strings.TrimSpace(email))).First(&u).Error
	if err != nil {
		return nil, notFound(err)
	}
	return &u, nil
}

func (r *userRepo) ByID(id uint64) (*model.User, error) {
	var u model.User
	if err := r.db.Preload("Roles").First(&u, id).Error; err != nil {
		return nil, notFound(err)
	}
	return &u, nil
}

func (r *userRepo) Create(u *model.User) error {
	u.Email = strings.ToLower(strings.TrimSpace(u.Email))
	return r.db.Omit("Roles").Create(u).Error
}

func (r *userRepo) EnsureRoles(names []string) error {
	for _, n := range names {
		if err := r.db.Clauses(clause.OnConflict{DoNothing: true}).Create(&model.Role{Name: n}).Error; err != nil {
			return err
		}
	}
	return nil
}

func (r *userRepo) SetRoles(userID uint64, roles []string) error {
	var rs []model.Role
	if len(roles) > 0 {
		if err := r.db.Where("name IN ?", roles).Find(&rs).Error; err != nil {
			return err
		}
	}
	u := model.User{Base: model.Base{ID: userID}}
	return r.db.Model(&u).Association("Roles").Replace(rs)
}

func (r *userRepo) UpdatePassword(userID uint64, hash string) error {
	return r.db.Model(&model.User{}).Where("id = ?", userID).Update("password", hash).Error
}

func (r *userRepo) SetActive(userID uint64, active bool) error {
	return r.db.Model(&model.User{}).Where("id = ?", userID).Update("is_active", active).Error
}

func (r *userRepo) UpdateEmail(userID uint64, email string) error {
	return r.db.Model(&model.User{}).Where("id = ?", userID).Update("email", strings.ToLower(strings.TrimSpace(email))).Error
}

func (r *userRepo) TouchLogin(userID uint64, at time.Time) error {
	return r.db.Model(&model.User{}).Where("id = ?", userID).Update("last_login_at", at).Error
}

// ---------- employees ----------

type EmployeeFilter struct {
	Search       string
	DepartmentID uint64
	Status       string
	Page
}

type EmployeeRepo interface {
	ByID(id uint64) (*model.Employee, error)
	ByUserID(userID uint64) (*model.Employee, error)
	ByIDs(ids []uint64) ([]model.Employee, error)
	List(f EmployeeFilter) ([]model.Employee, int64, error)
	ByDepartment(deptID uint64, activeOnly bool) ([]model.Employee, error)
	AllActive() ([]model.Employee, error)
	Create(e *model.Employee) error
	Update(e *model.Employee) error
	Delete(id uint64) error
	NIKTaken(nik string, exceptID uint64) (bool, error)
	EmailMap() (map[string]uint64, error)
	NIKMap() (map[string]uint64, error)
}

type employeeRepo struct{ db *gorm.DB }

func (r *employeeRepo) base() *gorm.DB {
	return r.db.Preload("Department").Preload("Position").Preload("User").Preload("User.Roles")
}

func (r *employeeRepo) ByID(id uint64) (*model.Employee, error) {
	var e model.Employee
	if err := r.base().First(&e, id).Error; err != nil {
		return nil, notFound(err)
	}
	return &e, nil
}

func (r *employeeRepo) ByUserID(userID uint64) (*model.Employee, error) {
	var e model.Employee
	if err := r.base().Where("user_id = ?", userID).First(&e).Error; err != nil {
		return nil, notFound(err)
	}
	return &e, nil
}

func (r *employeeRepo) ByIDs(ids []uint64) ([]model.Employee, error) {
	var out []model.Employee
	if len(ids) == 0 {
		return out, nil
	}
	err := r.base().Where("id IN ?", ids).Find(&out).Error
	return out, err
}

func (r *employeeRepo) List(f EmployeeFilter) ([]model.Employee, int64, error) {
	q := r.db.Model(&model.Employee{})
	if s := strings.TrimSpace(f.Search); s != "" {
		like := "%" + strings.ToLower(s) + "%"
		q = q.Where("LOWER(name) LIKE ? OR LOWER(nik) LIKE ?", like, like)
	}
	if f.DepartmentID != 0 {
		q = q.Where("department_id = ?", f.DepartmentID)
	}
	if f.Status != "" {
		q = q.Where("status = ?", f.Status)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	off, lim := f.Page.norm()
	var out []model.Employee
	err := q.Preload("Department").Preload("Position").Preload("User").Preload("User.Roles").
		Order("name ASC").Offset(off).Limit(lim).Find(&out).Error
	return out, total, err
}

func (r *employeeRepo) ByDepartment(deptID uint64, activeOnly bool) ([]model.Employee, error) {
	var out []model.Employee
	q := r.base().Where("department_id = ?", deptID)
	if activeOnly {
		q = q.Where("status = ?", model.StatusActive)
	}
	err := q.Order("name ASC").Find(&out).Error
	return out, err
}

func (r *employeeRepo) AllActive() ([]model.Employee, error) {
	var out []model.Employee
	err := r.base().Where("status = ?", model.StatusActive).Order("name ASC").Find(&out).Error
	return out, err
}

func (r *employeeRepo) Create(e *model.Employee) error {
	return r.db.Omit("User", "Department", "Position", "Manager").Create(e).Error
}

func (r *employeeRepo) Update(e *model.Employee) error {
	return r.db.Omit("User", "Department", "Position", "Manager").Save(e).Error
}

func (r *employeeRepo) Delete(id uint64) error {
	return r.db.Delete(&model.Employee{}, id).Error
}

func (r *employeeRepo) NIKTaken(nik string, exceptID uint64) (bool, error) {
	var n int64
	q := r.db.Unscoped().Model(&model.Employee{}).Where("nik = ?", nik)
	if exceptID != 0 {
		q = q.Where("id <> ?", exceptID)
	}
	err := q.Count(&n).Error
	return n > 0, err
}

func (r *employeeRepo) EmailMap() (map[string]uint64, error) {
	var us []model.User
	if err := r.db.Unscoped().Select("id", "email").Find(&us).Error; err != nil {
		return nil, err
	}
	m := make(map[string]uint64, len(us))
	for _, u := range us {
		m[u.Email] = u.ID
	}
	return m, nil
}

func (r *employeeRepo) NIKMap() (map[string]uint64, error) {
	var es []model.Employee
	if err := r.db.Unscoped().Select("id", "nik").Find(&es).Error; err != nil {
		return nil, err
	}
	m := make(map[string]uint64, len(es))
	for _, e := range es {
		m[e.NIK] = e.ID
	}
	return m, nil
}

// ---------- departments & positions ----------

type DepartmentRepo interface {
	List() ([]model.Department, error)
	ByID(id uint64) (*model.Department, error)
	Create(d *model.Department) error
	Update(d *model.Department) error
	Delete(id uint64) error
	InUse(id uint64) (bool, error)
	CodeTaken(code string, exceptID uint64) (bool, error)
}

type departmentRepo struct{ db *gorm.DB }

func (r *departmentRepo) List() ([]model.Department, error) {
	var out []model.Department
	err := r.db.Order("name ASC").Find(&out).Error
	return out, err
}

func (r *departmentRepo) ByID(id uint64) (*model.Department, error) {
	var d model.Department
	if err := r.db.First(&d, id).Error; err != nil {
		return nil, notFound(err)
	}
	return &d, nil
}

func (r *departmentRepo) Create(d *model.Department) error { return r.db.Create(d).Error }
func (r *departmentRepo) Update(d *model.Department) error { return r.db.Save(d).Error }
func (r *departmentRepo) Delete(id uint64) error           { return r.db.Delete(&model.Department{}, id).Error }

func (r *departmentRepo) InUse(id uint64) (bool, error) {
	var n int64
	if err := r.db.Model(&model.Employee{}).Where("department_id = ?", id).Count(&n).Error; err != nil {
		return false, err
	}
	if n > 0 {
		return true, nil
	}
	err := r.db.Model(&model.KpiTemplate{}).Where("department_id = ?", id).Count(&n).Error
	return n > 0, err
}

func (r *departmentRepo) CodeTaken(code string, exceptID uint64) (bool, error) {
	var n int64
	q := r.db.Unscoped().Model(&model.Department{}).Where("code = ?", code)
	if exceptID != 0 {
		q = q.Where("id <> ?", exceptID)
	}
	err := q.Count(&n).Error
	return n > 0, err
}

type PositionRepo interface {
	List(deptID uint64) ([]model.Position, error)
	ByID(id uint64) (*model.Position, error)
	Create(p *model.Position) error
	Update(p *model.Position) error
	Delete(id uint64) error
	InUse(id uint64) (bool, error)
}

type positionRepo struct{ db *gorm.DB }

func (r *positionRepo) List(deptID uint64) ([]model.Position, error) {
	var out []model.Position
	q := r.db.Order("name ASC")
	if deptID != 0 {
		q = q.Where("department_id = ? OR department_id IS NULL", deptID)
	}
	err := q.Find(&out).Error
	return out, err
}

func (r *positionRepo) ByID(id uint64) (*model.Position, error) {
	var p model.Position
	if err := r.db.First(&p, id).Error; err != nil {
		return nil, notFound(err)
	}
	return &p, nil
}

func (r *positionRepo) Create(p *model.Position) error { return r.db.Create(p).Error }
func (r *positionRepo) Update(p *model.Position) error { return r.db.Save(p).Error }
func (r *positionRepo) Delete(id uint64) error         { return r.db.Delete(&model.Position{}, id).Error }

func (r *positionRepo) InUse(id uint64) (bool, error) {
	var n int64
	err := r.db.Model(&model.Employee{}).Where("position_id = ?", id).Count(&n).Error
	return n > 0, err
}
