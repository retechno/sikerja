// Package repository adalah satu-satunya tempat akses DB. Service bergantung pada
// interface di sini sehingga mudah diuji/di-mock.
package repository

import (
	"errors"

	"gorm.io/gorm"

	"github.com/retechno/sikerja/backend/internal/model"
)

// ErrNotFound dikembalikan repository bila baris tidak ada.
var ErrNotFound = errors.New("data tidak ditemukan")

// Repos mengelompokkan semua repository pada satu koneksi (atau satu transaksi).
type Repos struct {
	Users         UserRepo
	Employees     EmployeeRepo
	Departments   DepartmentRepo
	Positions     PositionRepo
	Periods       PeriodRepo
	Templates     TemplateRepo
	Results       ResultRepo
	Aspects       AspectRepo
	Appraisals    AppraisalRepo
	Audit         AuditRepo
	Notifications NotificationRepo
	Resets        ResetRepo
	Reports       ReportRepo
	LeaveTypes    LeaveTypeRepo
	Balances      BalanceRepo
	Leaves        LeaveRequestRepo
	Approvals     ApprovalRepo
	Holidays      HolidayRepo
	Settings      SettingRepo
	Schedules     ScheduleRepo
}

// Store membuat Repos dan menjalankan fungsi dalam transaksi.
type Store interface {
	R() Repos
	Tx(fn func(Repos) error) error
}

type gormStore struct{ db *gorm.DB }

func NewStore(db *gorm.DB) Store { return &gormStore{db: db} }

func (s *gormStore) R() Repos { return build(s.db) }

func (s *gormStore) Tx(fn func(Repos) error) error {
	return s.db.Transaction(func(tx *gorm.DB) error { return fn(build(tx)) })
}

func build(db *gorm.DB) Repos {
	return Repos{
		Users:         &userRepo{db},
		Employees:     &employeeRepo{db},
		Departments:   &departmentRepo{db},
		Positions:     &positionRepo{db},
		Periods:       &periodRepo{db},
		Templates:     &templateRepo{db},
		Results:       &resultRepo{db},
		Aspects:       &aspectRepo{db},
		Appraisals:    &appraisalRepo{db},
		Audit:         &auditRepo{db},
		Notifications: &notificationRepo{db},
		Resets:        &resetRepo{db},
		Reports:       &reportRepo{db},
		LeaveTypes:    &leaveTypeRepo{db},
		Balances:      &balanceRepo{db},
		Leaves:        &leaveRequestRepo{db},
		Approvals:     &approvalRepo{db},
		Holidays:      &holidayRepo{db},
		Settings:      &settingRepo{db},
		Schedules:     &scheduleRepo{db},
	}
}

// notFound mengubah gorm.ErrRecordNotFound menjadi ErrNotFound.
func notFound(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrNotFound
	}
	return err
}

// Page adalah parameter paginasi.
type Page struct {
	Page    int
	PerPage int
}

func (p Page) norm() (offset, limit int) {
	if p.Page < 1 {
		p.Page = 1
	}
	if p.PerPage < 1 || p.PerPage > 200 {
		p.PerPage = 20
	}
	return (p.Page - 1) * p.PerPage, p.PerPage
}

// compile-time guard agar model tidak terhapus dari import.
var _ = model.RoleEmployee
