package model

import (
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// Status pengajuan izin/cuti.
const (
	LeavePendingManager = "pending_manager" // Menunggu Atasan
	LeavePendingHR      = "pending_hr"      // Menunggu HR
	LeaveApproved       = "approved"
	LeaveRejected       = "rejected"
	LeaveCancelled      = "cancelled"

	ActionApproved = "approved"
	ActionRejected = "rejected"

	HolidayNational = "nasional"
	HolidayJoint    = "cuti_bersama"

	SessionMorning   = "pagi"
	SessionAfternoon = "siang"
)

// LeavePending true bila pengajuan masih menunggu persetujuan.
func LeavePending(status string) bool {
	return status == LeavePendingManager || status == LeavePendingHR
}

// LeaveType: jenis izin/cuti. AnnualQuota nil = tidak dibatasi. DeductsBalance = saldo (quota/used)
// per tahun dikelola dan dipotong saat disetujui (cuti tahunan & izin); jenis lain hanya dicatat.
type LeaveType struct {
	Base
	Name              string           `gorm:"size:100;not null" json:"name"`
	Code              string           `gorm:"size:20;not null;uniqueIndex:uq_leave_types_code" json:"code"`
	AnnualQuota       *decimal.Decimal `gorm:"type:decimal(6,2)" json:"annual_quota"`
	DeductsBalance    bool             `gorm:"not null;default:false" json:"deducts_balance"`
	RequiresAttach    bool             `gorm:"column:requires_attachment;not null;default:false" json:"requires_attachment"`
	AttachMinDays     decimal.Decimal  `gorm:"column:attachment_min_days;type:decimal(6,2);not null;default:0" json:"attachment_min_days"`
	AllowHalfDay      bool             `gorm:"not null;default:false" json:"allow_half_day"`
	MinTenureMonths   int              `gorm:"not null;default:0" json:"min_tenure_months"`
	MaxDaysPerRequest *decimal.Decimal `gorm:"type:decimal(6,2)" json:"max_days_per_request"`
	Sort              int              `gorm:"not null;default:0" json:"sort"`
	IsActive          bool             `gorm:"not null;default:true" json:"is_active"`
	DeletedAt         gorm.DeletedAt   `gorm:"index" json:"-"`
}

type LeaveBalance struct {
	Base
	EmployeeID  uint64          `gorm:"not null;uniqueIndex:uq_leave_balances_etn,priority:1" json:"employee_id"`
	LeaveTypeID uint64          `gorm:"not null;uniqueIndex:uq_leave_balances_etn,priority:2" json:"leave_type_id"`
	Year        int             `gorm:"not null;uniqueIndex:uq_leave_balances_etn,priority:3" json:"year"`
	Quota       decimal.Decimal `gorm:"type:decimal(6,2);not null;default:0" json:"quota"`
	Used        decimal.Decimal `gorm:"type:decimal(6,2);not null;default:0" json:"used"`
}

type LeaveRequest struct {
	Base
	Number         *string         `gorm:"size:30;uniqueIndex:uq_leave_requests_number" json:"number"`
	EmployeeID     uint64          `gorm:"not null;index:idx_leave_requests_emp_start,priority:1" json:"employee_id"`
	LeaveTypeID    uint64          `gorm:"not null" json:"leave_type_id"`
	StartDate      time.Time       `gorm:"type:date;not null;index:idx_leave_requests_emp_start,priority:2;index:idx_leave_requests_range,priority:1" json:"start_date"`
	EndDate        time.Time       `gorm:"type:date;not null;index:idx_leave_requests_range,priority:2" json:"end_date"`
	HalfDay        bool            `gorm:"not null;default:false" json:"half_day"`
	HalfSession    *string         `gorm:"size:10" json:"half_session"`
	StartTime      *string         `gorm:"size:5" json:"start_time"` // izin sebagian hari: jam mulai "HH:MM"
	EndTime        *string         `gorm:"size:5" json:"end_time"`
	Days           decimal.Decimal `gorm:"type:decimal(6,2);not null" json:"days"`
	Reason         string          `gorm:"type:text" json:"reason"`
	AttachmentPath string          `gorm:"size:255" json:"-"`
	AttachmentName string          `gorm:"size:200" json:"attachment_name"`
	SubstituteID   *uint64         `json:"substitute_id"`
	Status         string          `gorm:"size:20;not null;index" json:"status"`
	Deducted       bool            `gorm:"not null;default:false" json:"deducted"`
	CancelledAt    *time.Time      `json:"cancelled_at"`

	Employee   *Employee       `json:"employee,omitempty"`
	LeaveType  *LeaveType      `json:"leave_type,omitempty"`
	Substitute *Employee       `gorm:"foreignKey:SubstituteID" json:"substitute,omitempty"`
	Approvals  []LeaveApproval `gorm:"foreignKey:LeaveRequestID;constraint:OnDelete:CASCADE" json:"approvals,omitempty"`
}

type LeaveApproval struct {
	Base
	LeaveRequestID uint64    `gorm:"not null;index:idx_leave_approvals_request" json:"leave_request_id"`
	ApproverID     uint64    `gorm:"not null;index:idx_leave_approvals_approver" json:"approver_id"`
	Level          int       `gorm:"not null" json:"level"`
	Action         string    `gorm:"size:15;not null" json:"action"`
	Note           string    `gorm:"size:500" json:"note"`
	ActedAt        time.Time `gorm:"not null" json:"acted_at"`
	Approver       *Employee `gorm:"foreignKey:ApproverID" json:"approver,omitempty"`
}

type Holiday struct {
	Base
	Date time.Time `gorm:"type:date;not null;uniqueIndex:uq_holidays_date" json:"date"`
	Name string    `gorm:"size:150;not null" json:"name"`
	Type string    `gorm:"size:15;not null;default:nasional" json:"type"`
}

// Setting: pasangan nama-nilai (saklar notifikasi). Tabel tambahan, lihat DECISIONS.md.
type Setting struct {
	Base
	Name  string `gorm:"size:60;not null;uniqueIndex:uq_app_settings_name" json:"name"`
	Value string `gorm:"size:255;not null" json:"value"`
}

func (Setting) TableName() string { return "app_settings" }

// WorkSchedule: jadwal kerja mingguan yang dipakai departemen. Hari yang tidak punya baris = libur mingguan.
type WorkSchedule struct {
	Base
	Name      string `gorm:"size:100;not null;uniqueIndex:uq_work_schedules_name" json:"name"`
	IsDefault bool   `gorm:"not null;default:false" json:"is_default"`
	// BreakStart/BreakEnd: jam istirahat "HH:MM" (kosong = tanpa istirahat); tidak dihitung sebagai jam kerja.
	BreakStart string            `gorm:"size:5" json:"break_start"`
	BreakEnd   string            `gorm:"size:5" json:"break_end"`
	Days       []WorkScheduleDay `gorm:"foreignKey:ScheduleID;constraint:OnDelete:CASCADE" json:"days"`
}

// WorkScheduleDay: jam kerja satu hari. Weekday 0 = Minggu ... 6 = Sabtu; jam "HH:MM".
type WorkScheduleDay struct {
	Base
	ScheduleID uint64 `gorm:"not null;uniqueIndex:uq_work_schedule_days_sw,priority:1" json:"-"`
	Weekday    int    `gorm:"not null;uniqueIndex:uq_work_schedule_days_sw,priority:2" json:"weekday"`
	StartTime  string `gorm:"size:5;not null" json:"start_time"`
	EndTime    string `gorm:"size:5;not null" json:"end_time"`
}
