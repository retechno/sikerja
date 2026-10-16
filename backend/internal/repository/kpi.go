package repository

import (
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/retechno/sikerja/backend/internal/model"
)

// ---------- periods ----------

type PeriodRepo interface {
	List() ([]model.AppraisalPeriod, error)
	ByID(id uint64) (*model.AppraisalPeriod, error)
	Create(p *model.AppraisalPeriod) error
	Save(p *model.AppraisalPeriod) error
	// Latest mengembalikan n periode terbaru (urut start_date naik) yang bukan draft.
	Latest(n int) ([]model.AppraisalPeriod, error)
}

type periodRepo struct{ db *gorm.DB }

func (r *periodRepo) List() ([]model.AppraisalPeriod, error) {
	var out []model.AppraisalPeriod
	err := r.db.Order("start_date DESC").Find(&out).Error
	return out, err
}

func (r *periodRepo) ByID(id uint64) (*model.AppraisalPeriod, error) {
	var p model.AppraisalPeriod
	if err := r.db.First(&p, id).Error; err != nil {
		return nil, notFound(err)
	}
	return &p, nil
}

func (r *periodRepo) Create(p *model.AppraisalPeriod) error { return r.db.Create(p).Error }
func (r *periodRepo) Save(p *model.AppraisalPeriod) error   { return r.db.Save(p).Error }

func (r *periodRepo) Latest(n int) ([]model.AppraisalPeriod, error) {
	var out []model.AppraisalPeriod
	err := r.db.Where("status <> ?", model.PeriodDraft).Order("start_date DESC").Limit(n).Find(&out).Error
	if err != nil {
		return nil, err
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, nil
}

// ---------- templates ----------

type TemplateRepo interface {
	List() ([]model.KpiTemplate, error)
	ByID(id uint64) (*model.KpiTemplate, error)
	// ActiveByDepartment: template aktif departemen beserta indikator berurutan.
	ActiveByDepartment(deptID uint64) (*model.KpiTemplate, error)
	// ActiveByDepartments: versi massal (hindari N+1) → map dept_id → template.
	ActiveByDepartments(deptIDs []uint64) (map[uint64]*model.KpiTemplate, error)
	Create(t *model.KpiTemplate) error
	Save(t *model.KpiTemplate) error
	IndicatorByID(id uint64) (*model.KpiIndicator, error)
	// IndicatorsUnscoped termasuk yang sudah dihapus (untuk riwayat periode).
	IndicatorsByIDs(ids []uint64) (map[uint64]model.KpiIndicator, error)
	CreateIndicator(i *model.KpiIndicator) error
	SaveIndicator(i *model.KpiIndicator) error
	DeleteIndicator(id uint64) error
}

type templateRepo struct{ db *gorm.DB }

func orderedIndicators(db *gorm.DB) *gorm.DB { return db.Order("sort ASC, id ASC") }

func (r *templateRepo) List() ([]model.KpiTemplate, error) {
	var out []model.KpiTemplate
	err := r.db.Preload("Department").Preload("Indicators", orderedIndicators).Order("id ASC").Find(&out).Error
	return out, err
}

func (r *templateRepo) ByID(id uint64) (*model.KpiTemplate, error) {
	var t model.KpiTemplate
	if err := r.db.Preload("Department").Preload("Indicators", orderedIndicators).First(&t, id).Error; err != nil {
		return nil, notFound(err)
	}
	return &t, nil
}

func (r *templateRepo) ActiveByDepartment(deptID uint64) (*model.KpiTemplate, error) {
	var t model.KpiTemplate
	err := r.db.Preload("Indicators", orderedIndicators).
		Where("department_id = ? AND is_active = ?", deptID, true).Order("id DESC").First(&t).Error
	if err != nil {
		return nil, notFound(err)
	}
	return &t, nil
}

func (r *templateRepo) ActiveByDepartments(deptIDs []uint64) (map[uint64]*model.KpiTemplate, error) {
	out := map[uint64]*model.KpiTemplate{}
	if len(deptIDs) == 0 {
		return out, nil
	}
	var ts []model.KpiTemplate
	err := r.db.Preload("Indicators", orderedIndicators).
		Where("department_id IN ? AND is_active = ?", deptIDs, true).Order("id ASC").Find(&ts).Error
	if err != nil {
		return nil, err
	}
	for i := range ts {
		out[ts[i].DepartmentID] = &ts[i] // template terbaru menimpa yang lama
	}
	return out, nil
}

func (r *templateRepo) Create(t *model.KpiTemplate) error { return r.db.Create(t).Error }

func (r *templateRepo) Save(t *model.KpiTemplate) error {
	return r.db.Omit("Indicators", "Department").Save(t).Error
}

func (r *templateRepo) IndicatorByID(id uint64) (*model.KpiIndicator, error) {
	var i model.KpiIndicator
	if err := r.db.Unscoped().First(&i, id).Error; err != nil {
		return nil, notFound(err)
	}
	return &i, nil
}

func (r *templateRepo) IndicatorsByIDs(ids []uint64) (map[uint64]model.KpiIndicator, error) {
	out := map[uint64]model.KpiIndicator{}
	if len(ids) == 0 {
		return out, nil
	}
	var is []model.KpiIndicator
	if err := r.db.Unscoped().Where("id IN ?", ids).Find(&is).Error; err != nil {
		return nil, err
	}
	for _, i := range is {
		out[i.ID] = i
	}
	return out, nil
}

func (r *templateRepo) CreateIndicator(i *model.KpiIndicator) error { return r.db.Create(i).Error }
func (r *templateRepo) SaveIndicator(i *model.KpiIndicator) error   { return r.db.Save(i).Error }
func (r *templateRepo) DeleteIndicator(id uint64) error {
	return r.db.Delete(&model.KpiIndicator{}, id).Error
}

// ---------- results ----------

type ResultRepo interface {
	ByID(id uint64) (*model.KpiResult, error)
	Find(periodID, employeeID, indicatorID uint64) (*model.KpiResult, error)
	ForEmployee(periodID, employeeID uint64) ([]model.KpiResult, error)
	ForEmployees(periodID uint64, employeeIDs []uint64) ([]model.KpiResult, error)
	Upsert(r *model.KpiResult) error
	Save(r *model.KpiResult) error
}

type resultRepo struct{ db *gorm.DB }

func (r *resultRepo) ByID(id uint64) (*model.KpiResult, error) {
	var x model.KpiResult
	if err := r.db.First(&x, id).Error; err != nil {
		return nil, notFound(err)
	}
	return &x, nil
}

func (r *resultRepo) Find(periodID, employeeID, indicatorID uint64) (*model.KpiResult, error) {
	var x model.KpiResult
	err := r.db.Where("period_id = ? AND employee_id = ? AND indicator_id = ?", periodID, employeeID, indicatorID).First(&x).Error
	if err != nil {
		return nil, notFound(err)
	}
	return &x, nil
}

func (r *resultRepo) ForEmployee(periodID, employeeID uint64) ([]model.KpiResult, error) {
	var out []model.KpiResult
	err := r.db.Where("period_id = ? AND employee_id = ?", periodID, employeeID).Find(&out).Error
	return out, err
}

func (r *resultRepo) ForEmployees(periodID uint64, ids []uint64) ([]model.KpiResult, error) {
	var out []model.KpiResult
	if len(ids) == 0 {
		return out, nil
	}
	err := r.db.Where("period_id = ? AND employee_id IN ?", periodID, ids).Find(&out).Error
	return out, err
}

func (r *resultRepo) Upsert(x *model.KpiResult) error {
	return r.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "period_id"}, {Name: "employee_id"}, {Name: "indicator_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"actual", "achievement", "note", "status", "submitted_by", "validated_by", "validated_at", "validation_note", "updated_at",
		}),
	}).Create(x).Error
}

func (r *resultRepo) Save(x *model.KpiResult) error { return r.db.Save(x).Error }

// ---------- aspects ----------

type AspectRepo interface {
	List() ([]model.BehaviorAspect, error)
	Create(a *model.BehaviorAspect) error
	Count() (int64, error)
}

type aspectRepo struct{ db *gorm.DB }

func (r *aspectRepo) List() ([]model.BehaviorAspect, error) {
	var out []model.BehaviorAspect
	err := r.db.Order("sort ASC, id ASC").Find(&out).Error
	return out, err
}
func (r *aspectRepo) Create(a *model.BehaviorAspect) error { return r.db.Create(a).Error }
func (r *aspectRepo) Count() (int64, error) {
	var n int64
	err := r.db.Model(&model.BehaviorAspect{}).Count(&n).Error
	return n, err
}

// ---------- appraisals ----------

type AppraisalRepo interface {
	Find(periodID, employeeID uint64) (*model.Appraisal, error)
	ByID(id uint64) (*model.Appraisal, error)
	ForEmployees(periodID uint64, employeeIDs []uint64) ([]model.Appraisal, error)
	// ByEmployee: penilaian satu karyawan pada beberapa periode (untuk riwayat).
	ByEmployee(employeeID uint64, periodIDs []uint64) ([]model.Appraisal, error)
	Create(a *model.Appraisal) error
	Save(a *model.Appraisal) error
	// UpsertScore menyimpan satu nilai; column = "self_score" atau "manager_score" (kolom lain tidak disentuh).
	UpsertScore(s *model.AppraisalScore, column string) error
	Count(periodID uint64, status string) (int64, error)
}

type appraisalRepo struct{ db *gorm.DB }

func (r *appraisalRepo) Find(periodID, employeeID uint64) (*model.Appraisal, error) {
	var a model.Appraisal
	err := r.db.Preload("Scores").Where("period_id = ? AND employee_id = ?", periodID, employeeID).First(&a).Error
	if err != nil {
		return nil, notFound(err)
	}
	return &a, nil
}

func (r *appraisalRepo) ByID(id uint64) (*model.Appraisal, error) {
	var a model.Appraisal
	if err := r.db.Preload("Scores").First(&a, id).Error; err != nil {
		return nil, notFound(err)
	}
	return &a, nil
}

func (r *appraisalRepo) ForEmployees(periodID uint64, ids []uint64) ([]model.Appraisal, error) {
	var out []model.Appraisal
	if len(ids) == 0 {
		return out, nil
	}
	err := r.db.Preload("Scores").Where("period_id = ? AND employee_id IN ?", periodID, ids).Find(&out).Error
	return out, err
}

func (r *appraisalRepo) ByEmployee(employeeID uint64, periodIDs []uint64) ([]model.Appraisal, error) {
	var out []model.Appraisal
	if len(periodIDs) == 0 {
		return out, nil
	}
	err := r.db.Where("employee_id = ? AND period_id IN ?", employeeID, periodIDs).Find(&out).Error
	return out, err
}

func (r *appraisalRepo) Create(a *model.Appraisal) error { return r.db.Omit("Scores").Create(a).Error }
func (r *appraisalRepo) Save(a *model.Appraisal) error   { return r.db.Omit("Scores").Save(a).Error }

func (r *appraisalRepo) UpsertScore(s *model.AppraisalScore, column string) error {
	if column != "self_score" && column != "manager_score" {
		return errors.New("kolom nilai tidak valid")
	}
	return r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "appraisal_id"}, {Name: "aspect_id"}},
		DoUpdates: clause.AssignmentColumns([]string{column, "updated_at"}),
	}).Create(s).Error
}

func (r *appraisalRepo) Count(periodID uint64, status string) (int64, error) {
	var n int64
	q := r.db.Model(&model.Appraisal{}).Where("period_id = ?", periodID)
	if status != "" {
		q = q.Where("status = ?", status)
	}
	err := q.Count(&n).Error
	return n, err
}

// ---------- audit ----------

type AuditFilter struct {
	Action      string
	SubjectType string
	SubjectID   uint64
	Page
}

type AuditRepo interface {
	Log(e *model.ActivityLog) error
	List(f AuditFilter) ([]model.ActivityLog, int64, error)
}

type auditRepo struct{ db *gorm.DB }

func (r *auditRepo) Log(e *model.ActivityLog) error { return r.db.Create(e).Error }

func (r *auditRepo) List(f AuditFilter) ([]model.ActivityLog, int64, error) {
	q := r.db.Model(&model.ActivityLog{})
	if f.Action != "" {
		q = q.Where("action = ?", f.Action)
	}
	if f.SubjectType != "" {
		q = q.Where("subject_type = ?", f.SubjectType)
	}
	if f.SubjectID != 0 {
		q = q.Where("subject_id = ?", f.SubjectID)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	off, lim := f.Page.norm()
	var out []model.ActivityLog
	err := q.Order("id DESC").Offset(off).Limit(lim).Find(&out).Error
	return out, total, err
}

// ---------- notifications ----------

type NotificationRepo interface {
	ListByUser(userID uint64, limit int) ([]model.Notification, error)
	UnreadCount(userID uint64) (int64, error)
	MarkRead(userID, id uint64, at time.Time) error
	MarkAllRead(userID uint64, at time.Time) error
	Create(n *model.Notification) error
}

type notificationRepo struct{ db *gorm.DB }

func (r *notificationRepo) ListByUser(userID uint64, limit int) ([]model.Notification, error) {
	var out []model.Notification
	err := r.db.Where("user_id = ?", userID).Order("id DESC").Limit(limit).Find(&out).Error
	return out, err
}

func (r *notificationRepo) UnreadCount(userID uint64) (int64, error) {
	var n int64
	err := r.db.Model(&model.Notification{}).Where("user_id = ? AND read_at IS NULL", userID).Count(&n).Error
	return n, err
}

func (r *notificationRepo) MarkRead(userID, id uint64, at time.Time) error {
	return r.db.Model(&model.Notification{}).Where("id = ? AND user_id = ?", id, userID).Update("read_at", at).Error
}

func (r *notificationRepo) MarkAllRead(userID uint64, at time.Time) error {
	return r.db.Model(&model.Notification{}).Where("user_id = ? AND read_at IS NULL", userID).Update("read_at", at).Error
}

func (r *notificationRepo) Create(n *model.Notification) error { return r.db.Create(n).Error }

// ---------- password reset ----------

type ResetRepo interface {
	Create(p *model.PasswordReset) error
	ByHash(hash string) (*model.PasswordReset, error)
	MarkUsed(id uint64, at time.Time) error
	InvalidateForUser(userID uint64, at time.Time) error
}

type resetRepo struct{ db *gorm.DB }

func (r *resetRepo) Create(p *model.PasswordReset) error { return r.db.Create(p).Error }

func (r *resetRepo) ByHash(hash string) (*model.PasswordReset, error) {
	var p model.PasswordReset
	if err := r.db.Where("token_hash = ?", hash).First(&p).Error; err != nil {
		return nil, notFound(err)
	}
	return &p, nil
}

func (r *resetRepo) MarkUsed(id uint64, at time.Time) error {
	return r.db.Model(&model.PasswordReset{}).Where("id = ?", id).Update("used_at", at).Error
}

func (r *resetRepo) InvalidateForUser(userID uint64, at time.Time) error {
	return r.db.Model(&model.PasswordReset{}).Where("user_id = ? AND used_at IS NULL", userID).Update("used_at", at).Error
}
