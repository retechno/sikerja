// Package model berisi struct GORM. Skema produksi ada di backend/migrations/*.sql;
// test (model_test.go) memastikan keduanya tidak melenceng.
package model

import (
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

const (
	RoleEmployee = "employee"
	RoleManager  = "manager"
	RoleHR       = "hr"
	RoleDirector = "director"

	StatusActive   = "active"
	StatusInactive = "inactive"

	PeriodDraft  = "draft"
	PeriodActive = "active"
	PeriodClosed = "closed"

	PeriodMonthly   = "monthly"
	PeriodQuarterly = "quarterly"

	ResultSubmitted = "submitted"
	ResultValidated = "validated"
	ResultRejected  = "rejected"

	AppraisalDraft       = "draft"
	AppraisalSelfDone    = "self_done"
	AppraisalManagerDone = "manager_done"
	AppraisalFinal       = "final"

	SourceManual      = "manual"
	SourceIntegration = "integration"
)

type Base struct {
	ID        uint64    `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Department struct {
	Base
	Name      string         `gorm:"size:100;not null" json:"name"`
	Code      string         `gorm:"size:20;not null;uniqueIndex:uq_departments_code" json:"code"`
	HeadID    *uint64        `json:"head_id"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

type Position struct {
	Base
	Name         string         `gorm:"size:100;not null" json:"name"`
	DepartmentID *uint64        `gorm:"index" json:"department_id"` // nil = jabatan lintas departemen (Direktur, HR)
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
}

type Role struct {
	ID   uint64 `gorm:"primaryKey" json:"id"`
	Name string `gorm:"size:30;not null;uniqueIndex:uq_roles_name" json:"name"`
}

type User struct {
	Base
	Email       string         `gorm:"size:150;not null;uniqueIndex:uq_users_email" json:"email"`
	Password    string         `gorm:"size:100;not null" json:"-"`
	IsActive    bool           `gorm:"not null;default:true" json:"is_active"`
	LastLoginAt *time.Time     `json:"last_login_at"`
	Roles       []Role         `gorm:"many2many:user_roles;" json:"-"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

type Employee struct {
	Base
	UserID       uint64         `gorm:"not null;uniqueIndex:uq_employees_user" json:"user_id"`
	NIK          string         `gorm:"column:nik;size:30;not null;uniqueIndex:uq_employees_nik" json:"nik"`
	Name         string         `gorm:"size:150;not null" json:"name"`
	Phone        string         `gorm:"size:30" json:"phone"`
	DepartmentID *uint64        `gorm:"index" json:"department_id"` // nil untuk Direktur & HR
	PositionID   *uint64        `gorm:"index" json:"position_id"`
	ManagerID    *uint64        `gorm:"index" json:"manager_id"`
	JoinDate     time.Time      `gorm:"type:date;not null" json:"join_date"`
	Status       string         `gorm:"size:20;not null;default:active" json:"status"`
	Photo        string         `gorm:"size:255" json:"-"`
	User         *User          `json:"-"`
	Department   *Department    `json:"department,omitempty"`
	Position     *Position      `json:"position,omitempty"`
	Manager      *Employee      `gorm:"foreignKey:ManagerID" json:"-"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
}

type KpiTemplate struct {
	Base
	DepartmentID uint64         `gorm:"not null;index" json:"department_id"`
	Name         string         `gorm:"size:150;not null" json:"name"`
	IsActive     bool           `gorm:"not null;default:true" json:"is_active"`
	Indicators   []KpiIndicator `gorm:"foreignKey:KpiTemplateID;constraint:OnDelete:CASCADE" json:"indicators,omitempty"`
	Department   *Department    `json:"department,omitempty"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
}

type KpiIndicator struct {
	Base
	KpiTemplateID uint64          `gorm:"not null;index" json:"kpi_template_id"`
	Name          string          `gorm:"size:200;not null" json:"name"`
	Unit          string          `gorm:"size:30" json:"unit"`
	Target        decimal.Decimal `gorm:"type:decimal(14,2);not null" json:"target"`
	Weight        decimal.Decimal `gorm:"type:decimal(6,2);not null" json:"weight"` // persen
	Direction     string          `gorm:"size:10;not null" json:"direction"`
	Source        string          `gorm:"size:15;not null;default:manual" json:"source"`
	Sort          int             `gorm:"not null;default:0" json:"sort"`
	DeletedAt     gorm.DeletedAt  `gorm:"index" json:"-"`
}

type AppraisalPeriod struct {
	Base
	Name           string          `gorm:"size:100;not null" json:"name"`
	Type           string          `gorm:"size:15;not null;default:monthly" json:"type"`
	StartDate      time.Time       `gorm:"type:date;not null" json:"start_date"`
	EndDate        time.Time       `gorm:"type:date;not null" json:"end_date"`
	Status         string          `gorm:"size:15;not null;default:draft;index" json:"status"`
	KpiWeight      decimal.Decimal `gorm:"type:decimal(5,2);not null" json:"kpi_weight"`
	BehaviorWeight decimal.Decimal `gorm:"type:decimal(5,2);not null" json:"behavior_weight"`
}

type KpiResult struct {
	Base
	PeriodID       uint64          `gorm:"not null;uniqueIndex:uq_kpi_results_pei,priority:1" json:"period_id"`
	EmployeeID     uint64          `gorm:"not null;uniqueIndex:uq_kpi_results_pei,priority:2;index" json:"employee_id"`
	IndicatorID    uint64          `gorm:"not null;uniqueIndex:uq_kpi_results_pei,priority:3" json:"indicator_id"`
	Actual         decimal.Decimal `gorm:"type:decimal(14,2);not null" json:"actual"`
	Achievement    decimal.Decimal `gorm:"type:decimal(6,2);not null" json:"achievement"`
	Note           string          `gorm:"type:text" json:"note"`
	EvidencePath   string          `gorm:"size:255" json:"-"`
	EvidenceName   string          `gorm:"size:200" json:"evidence_name"`
	Status         string          `gorm:"size:15;not null;default:submitted" json:"status"`
	SubmittedBy    *uint64         `json:"submitted_by"`
	ValidatedBy    *uint64         `json:"validated_by"`
	ValidatedAt    *time.Time      `json:"validated_at"`
	ValidationNote string          `gorm:"size:255" json:"validation_note"`
}

type BehaviorAspect struct {
	Base
	Name        string         `gorm:"size:100;not null" json:"name"`
	Description string         `gorm:"size:255" json:"description"`
	Sort        int            `gorm:"not null;default:0" json:"sort"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

type Appraisal struct {
	Base
	PeriodID        uint64              `gorm:"not null;uniqueIndex:uq_appraisals_pe,priority:1" json:"period_id"`
	EmployeeID      uint64              `gorm:"not null;uniqueIndex:uq_appraisals_pe,priority:2;index" json:"employee_id"`
	AppraiserID     *uint64             `json:"appraiser_id"`
	KpiScore        decimal.NullDecimal `gorm:"type:decimal(6,2)" json:"kpi_score"`
	BehaviorScore   decimal.NullDecimal `gorm:"type:decimal(6,2)" json:"behavior_score"`
	FinalScore      decimal.NullDecimal `gorm:"type:decimal(6,2)" json:"final_score"`
	Grade           string              `gorm:"size:30" json:"grade"`
	EmployeeComment string              `gorm:"type:text" json:"employee_comment"`
	ManagerComment  string              `gorm:"type:text" json:"manager_comment"`
	DevelopmentPlan string              `gorm:"type:text" json:"development_plan"`
	Status          string              `gorm:"size:15;not null;default:draft;index" json:"status"`
	SelfSubmittedAt *time.Time          `json:"self_submitted_at"`
	MgrSubmittedAt  *time.Time          `json:"manager_submitted_at"`
	FinalizedAt     *time.Time          `json:"finalized_at"`
	FinalizedBy     *uint64             `json:"finalized_by"`
	Scores          []AppraisalScore    `gorm:"foreignKey:AppraisalID;constraint:OnDelete:CASCADE" json:"scores,omitempty"`
}

type AppraisalScore struct {
	Base
	AppraisalID  uint64 `gorm:"not null;uniqueIndex:uq_appraisal_scores_aa,priority:1" json:"appraisal_id"`
	AspectID     uint64 `gorm:"not null;uniqueIndex:uq_appraisal_scores_aa,priority:2" json:"aspect_id"`
	SelfScore    *int   `json:"self_score"`
	ManagerScore *int   `json:"manager_score"`
}

type Notification struct {
	Base
	UserID uint64     `gorm:"not null;index" json:"user_id"`
	Type   string     `gorm:"size:50;not null" json:"type"`
	Data   string     `gorm:"type:json" json:"data"`
	ReadAt *time.Time `json:"read_at"`
}

type ActivityLog struct {
	ID          uint64    `gorm:"primaryKey" json:"id"`
	CreatedAt   time.Time `json:"created_at"`
	UserID      *uint64   `gorm:"index" json:"user_id"`
	Action      string    `gorm:"size:60;not null;index" json:"action"`
	SubjectType string    `gorm:"size:60;not null;index:idx_activity_subject,priority:1" json:"subject_type"`
	SubjectID   uint64    `gorm:"not null;index:idx_activity_subject,priority:2" json:"subject_id"`
	OldValues   string    `gorm:"type:json" json:"old_values"`
	NewValues   string    `gorm:"type:json" json:"new_values"`
}

// PasswordReset: token lupa password (hanya hash yang disimpan). Tabel tambahan, lihat DECISIONS.md.
type PasswordReset struct {
	Base
	UserID    uint64     `gorm:"not null;index" json:"user_id"`
	TokenHash string     `gorm:"size:64;not null;uniqueIndex:uq_password_resets_token" json:"-"`
	ExpiresAt time.Time  `gorm:"not null" json:"expires_at"`
	UsedAt    *time.Time `json:"used_at"`
}

// All adalah daftar model untuk AutoMigrate (test) dan pengecekan paritas migrasi.
func All() []any {
	return []any{
		&Department{}, &Position{}, &Role{}, &User{}, &Employee{},
		&KpiTemplate{}, &KpiIndicator{}, &AppraisalPeriod{}, &KpiResult{},
		&BehaviorAspect{}, &Appraisal{}, &AppraisalScore{},
		&Notification{}, &ActivityLog{}, &PasswordReset{},
	}
}

// TableName: SPEC menamai tabel ini tunggal (activity_log).
func (ActivityLog) TableName() string { return "activity_log" }
