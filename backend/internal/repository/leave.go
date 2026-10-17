package repository

import (
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/retechno/sikerja/backend/internal/model"
)

// ---------- jenis cuti ----------

type LeaveTypeRepo interface {
	List(activeOnly bool) ([]model.LeaveType, error)
	ByID(id uint64) (*model.LeaveType, error)
	Create(t *model.LeaveType) error
	Update(t *model.LeaveType) error
	Delete(id uint64) error
	InUse(id uint64) (bool, error)
	CodeTaken(code string, exceptID uint64) (bool, error)
}

type leaveTypeRepo struct{ db *gorm.DB }

func (r *leaveTypeRepo) List(activeOnly bool) ([]model.LeaveType, error) {
	var out []model.LeaveType
	q := r.db.Order("sort ASC, id ASC")
	if activeOnly {
		q = q.Where("is_active = ?", true)
	}
	err := q.Find(&out).Error
	return out, err
}

func (r *leaveTypeRepo) ByID(id uint64) (*model.LeaveType, error) {
	var t model.LeaveType
	if err := r.db.First(&t, id).Error; err != nil {
		return nil, notFound(err)
	}
	return &t, nil
}

func (r *leaveTypeRepo) Create(t *model.LeaveType) error { return r.db.Create(t).Error }
func (r *leaveTypeRepo) Update(t *model.LeaveType) error { return r.db.Save(t).Error }
func (r *leaveTypeRepo) Delete(id uint64) error          { return r.db.Delete(&model.LeaveType{}, id).Error }

func (r *leaveTypeRepo) InUse(id uint64) (bool, error) {
	var n int64
	err := r.db.Model(&model.LeaveRequest{}).Where("leave_type_id = ?", id).Count(&n).Error
	return n > 0, err
}

func (r *leaveTypeRepo) CodeTaken(code string, exceptID uint64) (bool, error) {
	var n int64
	q := r.db.Unscoped().Model(&model.LeaveType{}).Where("code = ?", code)
	if exceptID != 0 {
		q = q.Where("id <> ?", exceptID)
	}
	err := q.Count(&n).Error
	return n > 0, err
}

// ---------- saldo ----------

type BalanceRepo interface {
	List(employeeID uint64, year int) ([]model.LeaveBalance, error)
	// Ensure membuat baris saldo bila belum ada (aman dipanggil bersamaan) lalu mengembalikannya.
	Ensure(employeeID, typeID uint64, year int, quota decimal.Decimal) (*model.LeaveBalance, error)
	// Find membaca satu saldo; ErrNotFound bila barisnya belum dibuat.
	Find(employeeID, typeID uint64, year int) (*model.LeaveBalance, error)
	// Lock membaca saldo dengan SELECT ... FOR UPDATE; hanya bermakna di dalam transaksi.
	Lock(employeeID, typeID uint64, year int) (*model.LeaveBalance, error)
	Save(b *model.LeaveBalance) error
	// ApplyQuota mengubah kuota tahun berjalan pada baris yang masih memakai kuota lama (belum diubah manual).
	ApplyQuota(typeID uint64, year int, oldQuota, newQuota decimal.Decimal) (int64, error)
}

type balanceRepo struct{ db *gorm.DB }

func (r *balanceRepo) List(employeeID uint64, year int) ([]model.LeaveBalance, error) {
	var out []model.LeaveBalance
	err := r.db.Where("employee_id = ? AND year = ?", employeeID, year).Find(&out).Error
	return out, err
}

func (r *balanceRepo) Ensure(employeeID, typeID uint64, year int, quota decimal.Decimal) (*model.LeaveBalance, error) {
	row := model.LeaveBalance{EmployeeID: employeeID, LeaveTypeID: typeID, Year: year, Quota: quota, Used: decimal.Zero}
	if err := r.db.Clauses(clause.OnConflict{DoNothing: true}).Create(&row).Error; err != nil {
		return nil, err
	}
	var b model.LeaveBalance
	err := r.db.Where("employee_id = ? AND leave_type_id = ? AND year = ?", employeeID, typeID, year).First(&b).Error
	if err != nil {
		return nil, notFound(err)
	}
	return &b, nil
}

func (r *balanceRepo) Find(employeeID, typeID uint64, year int) (*model.LeaveBalance, error) {
	var b model.LeaveBalance
	err := r.db.Where("employee_id = ? AND leave_type_id = ? AND year = ?", employeeID, typeID, year).First(&b).Error
	if err != nil {
		return nil, notFound(err)
	}
	return &b, nil
}

func (r *balanceRepo) Lock(employeeID, typeID uint64, year int) (*model.LeaveBalance, error) {
	var b model.LeaveBalance
	err := r.db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("employee_id = ? AND leave_type_id = ? AND year = ?", employeeID, typeID, year).First(&b).Error
	if err != nil {
		return nil, notFound(err)
	}
	return &b, nil
}

func (r *balanceRepo) Save(b *model.LeaveBalance) error { return r.db.Save(b).Error }

func (r *balanceRepo) ApplyQuota(typeID uint64, year int, oldQuota, newQuota decimal.Decimal) (int64, error) {
	res := r.db.Model(&model.LeaveBalance{}).
		Where("leave_type_id = ? AND year = ? AND quota = ?", typeID, year, oldQuota).
		Update("quota", newQuota)
	return res.RowsAffected, res.Error
}

// ---------- pengajuan ----------

type LeaveFilter struct {
	EmployeeID uint64
	Statuses   []string
	Year       int
	Page
}

type LeaveRequestRepo interface {
	Create(l *model.LeaveRequest) error
	Save(l *model.LeaveRequest) error
	ByID(id uint64) (*model.LeaveRequest, error)
	// Lock mengunci baris pengajuan (SELECT ... FOR UPDATE) lalu memuatnya lengkap.
	Lock(id uint64) (*model.LeaveRequest, error)
	List(f LeaveFilter) ([]model.LeaveRequest, int64, error)
	ByStatuses(statuses []string, limit int) ([]model.LeaveRequest, error)
	Overlapping(employeeID uint64, start, end time.Time, excludeID uint64) ([]model.LeaveRequest, error)
	SumDays(employeeID, typeID uint64, year int, statuses []string) (decimal.Decimal, error)
	Calendar(from, to time.Time, deptID uint64, statuses []string) ([]model.LeaveRequest, error)
	CountByStatuses(statuses []string) (int64, error)
}

type leaveRequestRepo struct{ db *gorm.DB }

func (r *leaveRequestRepo) full() *gorm.DB {
	return r.db.Preload("Employee").Preload("Employee.Department").Preload("Employee.User").Preload("Employee.User.Roles").
		Preload("LeaveType").Preload("Substitute").
		Preload("Approvals", func(db *gorm.DB) *gorm.DB { return db.Order("acted_at ASC, id ASC") }).
		Preload("Approvals.Approver")
}

func (r *leaveRequestRepo) Create(l *model.LeaveRequest) error {
	return r.db.Omit("Employee", "LeaveType", "Substitute", "Approvals").Create(l).Error
}

func (r *leaveRequestRepo) Save(l *model.LeaveRequest) error {
	return r.db.Omit("Employee", "LeaveType", "Substitute", "Approvals").Save(l).Error
}

func (r *leaveRequestRepo) ByID(id uint64) (*model.LeaveRequest, error) {
	var l model.LeaveRequest
	if err := r.full().First(&l, id).Error; err != nil {
		return nil, notFound(err)
	}
	return &l, nil
}

func (r *leaveRequestRepo) Lock(id uint64) (*model.LeaveRequest, error) {
	var l model.LeaveRequest
	if err := r.db.Clauses(clause.Locking{Strength: "UPDATE"}).First(&l, id).Error; err != nil {
		return nil, notFound(err)
	}
	return r.ByID(id)
}

func (r *leaveRequestRepo) List(f LeaveFilter) ([]model.LeaveRequest, int64, error) {
	q := r.db.Model(&model.LeaveRequest{})
	if f.EmployeeID != 0 {
		q = q.Where("employee_id = ?", f.EmployeeID)
	}
	if len(f.Statuses) > 0 {
		q = q.Where("status IN ?", f.Statuses)
	}
	if f.Year != 0 {
		q = q.Where("start_date >= ? AND start_date < ?", yearStart(f.Year), yearStart(f.Year+1))
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	off, lim := f.norm()
	var out []model.LeaveRequest
	err := q.Preload("LeaveType").Preload("Employee").Preload("Substitute").
		Order("start_date DESC, id DESC").Offset(off).Limit(lim).Find(&out).Error
	return out, total, err
}

func (r *leaveRequestRepo) ByStatuses(statuses []string, limit int) ([]model.LeaveRequest, error) {
	var out []model.LeaveRequest
	err := r.full().Where("status IN ?", statuses).Order("created_at ASC, id ASC").Limit(limit).Find(&out).Error
	return out, err
}

func (r *leaveRequestRepo) Overlapping(employeeID uint64, start, end time.Time, excludeID uint64) ([]model.LeaveRequest, error) {
	var out []model.LeaveRequest
	q := r.db.Where("employee_id = ? AND status IN ? AND start_date <= ? AND end_date >= ?",
		employeeID, []string{model.LeavePendingManager, model.LeavePendingHR, model.LeaveApproved}, end, start)
	if excludeID != 0 {
		q = q.Where("id <> ?", excludeID)
	}
	err := q.Find(&out).Error
	return out, err
}

func (r *leaveRequestRepo) SumDays(employeeID, typeID uint64, year int, statuses []string) (decimal.Decimal, error) {
	var days []decimal.Decimal
	err := r.db.Model(&model.LeaveRequest{}).
		Where("employee_id = ? AND leave_type_id = ? AND status IN ? AND start_date >= ? AND start_date < ?",
			employeeID, typeID, statuses, yearStart(year), yearStart(year+1)).
		Pluck("days", &days).Error
	sum := decimal.Zero
	for _, d := range days {
		sum = sum.Add(d)
	}
	return sum, err
}

func (r *leaveRequestRepo) Calendar(from, to time.Time, deptID uint64, statuses []string) ([]model.LeaveRequest, error) {
	var out []model.LeaveRequest
	q := r.db.Preload("LeaveType").Preload("Employee").Preload("Employee.Department").
		Where("status IN ? AND start_date <= ? AND end_date >= ?", statuses, to, from)
	if deptID != 0 {
		q = q.Where("employee_id IN (?)", r.db.Model(&model.Employee{}).Select("id").Where("department_id = ?", deptID))
	}
	err := q.Order("start_date ASC, id ASC").Find(&out).Error
	return out, err
}

func (r *leaveRequestRepo) CountByStatuses(statuses []string) (int64, error) {
	var n int64
	err := r.db.Model(&model.LeaveRequest{}).Where("status IN ?", statuses).Count(&n).Error
	return n, err
}

func yearStart(y int) time.Time { return time.Date(y, 1, 1, 0, 0, 0, 0, time.UTC) }

// ---------- persetujuan ----------

type ApprovalRepo interface {
	Create(a *model.LeaveApproval) error
	ByApprover(approverID uint64, limit int) ([]model.LeaveApproval, error)
}

type approvalRepo struct{ db *gorm.DB }

func (r *approvalRepo) Create(a *model.LeaveApproval) error {
	return r.db.Omit("Approver").Create(a).Error
}

func (r *approvalRepo) ByApprover(approverID uint64, limit int) ([]model.LeaveApproval, error) {
	var out []model.LeaveApproval
	err := r.db.Where("approver_id = ?", approverID).Order("acted_at DESC, id DESC").Limit(limit).Find(&out).Error
	return out, err
}

// ---------- hari libur ----------

type HolidayRepo interface {
	InRange(from, to time.Time) ([]model.Holiday, error)
	ByYear(year int) ([]model.Holiday, error)
	ByID(id uint64) (*model.Holiday, error)
	Create(h *model.Holiday) error
	Update(h *model.Holiday) error
	Delete(id uint64) error
	DateTaken(date time.Time, exceptID uint64) (bool, error)
	CountYear(year int) (int64, error)
}

type holidayRepo struct{ db *gorm.DB }

func (r *holidayRepo) InRange(from, to time.Time) ([]model.Holiday, error) {
	var out []model.Holiday
	err := r.db.Where("date >= ? AND date <= ?", from, to).Order("date ASC").Find(&out).Error
	return out, err
}

func (r *holidayRepo) ByYear(year int) ([]model.Holiday, error) {
	var out []model.Holiday
	err := r.db.Where("date >= ? AND date < ?", yearStart(year), yearStart(year+1)).Order("date ASC").Find(&out).Error
	return out, err
}

func (r *holidayRepo) ByID(id uint64) (*model.Holiday, error) {
	var h model.Holiday
	if err := r.db.First(&h, id).Error; err != nil {
		return nil, notFound(err)
	}
	return &h, nil
}

func (r *holidayRepo) Create(h *model.Holiday) error { return r.db.Create(h).Error }
func (r *holidayRepo) Update(h *model.Holiday) error { return r.db.Save(h).Error }
func (r *holidayRepo) Delete(id uint64) error        { return r.db.Delete(&model.Holiday{}, id).Error }

func (r *holidayRepo) DateTaken(date time.Time, exceptID uint64) (bool, error) {
	var n int64
	q := r.db.Model(&model.Holiday{}).Where("date = ?", date)
	if exceptID != 0 {
		q = q.Where("id <> ?", exceptID)
	}
	err := q.Count(&n).Error
	return n > 0, err
}

func (r *holidayRepo) CountYear(year int) (int64, error) {
	var n int64
	err := r.db.Model(&model.Holiday{}).Where("date >= ? AND date < ?", yearStart(year), yearStart(year+1)).Count(&n).Error
	return n, err
}

// ---------- pengaturan ----------

type SettingRepo interface {
	All() (map[string]string, error)
	Set(name, value string) error
}

type settingRepo struct{ db *gorm.DB }

func (r *settingRepo) All() (map[string]string, error) {
	var rows []model.Setting
	if err := r.db.Find(&rows).Error; err != nil {
		return nil, err
	}
	m := make(map[string]string, len(rows))
	for _, s := range rows {
		m[s.Name] = s.Value
	}
	return m, nil
}

func (r *settingRepo) Set(name, value string) error {
	name = strings.TrimSpace(name)
	return r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "name"}},
		DoUpdates: clause.AssignmentColumns([]string{"value", "updated_at"}),
	}).Create(&model.Setting{Name: name, Value: value}).Error
}
