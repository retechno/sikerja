package service

import (
	"encoding/json"
	"slices"

	"github.com/retechno/sikerja/backend/internal/apperr"
	"github.com/retechno/sikerja/backend/internal/model"
	"github.com/retechno/sikerja/backend/internal/repository"
)

// Actor adalah pengguna yang sedang melakukan request, dimuat dari DB pada setiap request
// (bukan dari token) sehingga perubahan role/departemen/nonaktif langsung berlaku.
type Actor struct {
	UserID       uint64
	EmployeeID   uint64
	DepartmentID *uint64
	Name         string
	Roles        []string
}

func (a Actor) Has(role string) bool { return slices.Contains(a.Roles, role) }

func (a Actor) IsHR() bool       { return a.Has(model.RoleHR) }
func (a Actor) IsDirector() bool { return a.Has(model.RoleDirector) }
func (a Actor) IsManager() bool  { return a.Has(model.RoleManager) }

// HasAny true bila actor punya salah satu role.
func (a Actor) HasAny(roles ...string) bool {
	for _, r := range roles {
		if a.Has(r) {
			return true
		}
	}
	return false
}

func sameDept(a *uint64, b *uint64) bool { return a != nil && b != nil && *a == *b }

// CanView: karyawan hanya dirinya; manajer anggota departemennya; HR & Direktur semua.
func (a Actor) CanView(e *model.Employee) bool {
	switch {
	case e.ID == a.EmployeeID:
		return true
	case a.IsHR(), a.IsDirector():
		return true
	case a.IsManager():
		return sameDept(a.DepartmentID, e.DepartmentID)
	}
	return false
}

// CanAppraise: boleh memvalidasi KPI, menginput realisasi untuk, dan menilai karyawan e.
// Manajer → anggota departemennya (bukan dirinya); atasan langsung (mis. Direktur → Manajer).
func (a Actor) CanAppraise(e *model.Employee) bool {
	if e.ID == a.EmployeeID {
		return false
	}
	if a.IsManager() && sameDept(a.DepartmentID, e.DepartmentID) {
		return true
	}
	return e.ManagerID != nil && *e.ManagerID == a.EmployeeID && a.HasAny(model.RoleManager, model.RoleDirector)
}

// CanFinalize: HR atau Direktur.
func (a Actor) CanFinalize() bool { return a.IsHR() || a.IsDirector() }

var errForbiddenData = apperr.Forbidden("Anda tidak berhak mengakses data ini")

func requireView(a Actor, e *model.Employee) error {
	if !a.CanView(e) {
		return errForbiddenData
	}
	return nil
}

func requireAppraise(a Actor, e *model.Employee) error {
	if !a.CanAppraise(e) {
		return errForbiddenData
	}
	return nil
}

// loadEmployee memuat karyawan atau mengembalikan 404.
func loadEmployee(r repository.Repos, id uint64) (*model.Employee, error) {
	e, err := r.Employees.ByID(id)
	if err != nil {
		if err == repository.ErrNotFound {
			return nil, apperr.NotFound("Karyawan tidak ditemukan")
		}
		return nil, err
	}
	return e, nil
}

// ---------- audit ----------

// logActivity mencatat satu baris activity_log (siapa, apa, nilai lama → baru).
func logActivity(r repository.Repos, actor Actor, action, subjectType string, subjectID uint64, oldV, newV any) error {
	uid := actor.UserID
	e := &model.ActivityLog{
		UserID:      &uid,
		Action:      action,
		SubjectType: subjectType,
		SubjectID:   subjectID,
		OldValues:   toJSON(oldV),
		NewValues:   toJSON(newV),
	}
	return r.Audit.Log(e)
}

func toJSON(v any) string {
	if v == nil {
		return "null"
	}
	b, err := json.Marshal(v)
	if err != nil {
		return "null"
	}
	return string(b)
}

// Aksi audit.
const (
	AuditKpiValidate    = "kpi.validate"
	AuditKpiReject      = "kpi.reject"
	AuditKpiChange      = "kpi.change"
	AuditScoreChange    = "appraisal.score_change"
	AuditFinalize       = "appraisal.finalize"
	AuditPeriodOpen     = "period.open"
	AuditPeriodClose    = "period.close"
	AuditPeriodReopen   = "period.reopen"
	AuditTemplateChange = "kpi_template.change"
	AuditRoleChange     = "employee.roles_change"
)
