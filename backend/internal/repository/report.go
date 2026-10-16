package repository

import (
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// ScoreRow adalah satu baris tabel skor (laporan H-15 dan dasbor). Skor nil = belum dinilai.
type ScoreRow struct {
	EmployeeID     uint64
	NIK            string
	EmployeeName   string
	DepartmentID   uint64
	DepartmentName string
	PositionName   string
	AppraisalID    *uint64
	Status         *string
	KpiScore       decimal.NullDecimal
	BehaviorScore  decimal.NullDecimal
	FinalScore     decimal.NullDecimal
	Grade          *string
}

// DeptAvgRow = rata-rata skor akhir satu departemen pada satu periode.
type DeptAvgRow struct {
	PeriodID       uint64
	DepartmentID   uint64
	DepartmentName string
	Avg            float64
	Count          int64
}

type GradeCountRow struct {
	Grade string
	Count int64
}

type ReportRepo interface {
	// ScoresForPeriod: semua karyawan aktif ber-departemen (opsional difilter satu departemen
	// atau daftar karyawan) beserta penilaiannya. Satu query, tanpa N+1.
	ScoresForPeriod(periodID uint64, deptID uint64, employeeIDs []uint64) ([]ScoreRow, error)
	DeptAverages(periodIDs []uint64) ([]DeptAvgRow, error)
	GradeCounts(periodID uint64, deptID uint64) ([]GradeCountRow, error)
}

type reportRepo struct{ db *gorm.DB }

func (r *reportRepo) ScoresForPeriod(periodID, deptID uint64, employeeIDs []uint64) ([]ScoreRow, error) {
	q := r.db.Table("employees e").
		Select(`e.id AS employee_id, e.nik AS nik, e.name AS employee_name,
			e.department_id AS department_id, d.name AS department_name, p.name AS position_name,
			a.id AS appraisal_id, a.status AS status, a.kpi_score AS kpi_score,
			a.behavior_score AS behavior_score, a.final_score AS final_score, a.grade AS grade`).
		Joins("JOIN departments d ON d.id = e.department_id AND d.deleted_at IS NULL").
		Joins("LEFT JOIN positions p ON p.id = e.position_id").
		Joins("LEFT JOIN appraisals a ON a.employee_id = e.id AND a.period_id = ?", periodID).
		Where("e.deleted_at IS NULL AND e.status = ?", "active")
	if deptID != 0 {
		q = q.Where("e.department_id = ?", deptID)
	}
	if employeeIDs != nil {
		if len(employeeIDs) == 0 {
			return nil, nil
		}
		q = q.Where("e.id IN ?", employeeIDs)
	}
	var out []ScoreRow
	err := q.Order("d.name ASC, e.name ASC").Scan(&out).Error
	return out, err
}

func (r *reportRepo) DeptAverages(periodIDs []uint64) ([]DeptAvgRow, error) {
	if len(periodIDs) == 0 {
		return nil, nil
	}
	var out []DeptAvgRow
	err := r.db.Table("appraisals a").
		Select(`a.period_id AS period_id, e.department_id AS department_id, d.name AS department_name,
			AVG(a.final_score) AS avg, COUNT(a.id) AS count`).
		Joins("JOIN employees e ON e.id = a.employee_id AND e.deleted_at IS NULL").
		Joins("JOIN departments d ON d.id = e.department_id AND d.deleted_at IS NULL").
		Where("a.period_id IN ? AND a.final_score IS NOT NULL", periodIDs).
		Group("a.period_id, e.department_id, d.name").
		Order("a.period_id ASC, d.name ASC").Scan(&out).Error
	return out, err
}

func (r *reportRepo) GradeCounts(periodID, deptID uint64) ([]GradeCountRow, error) {
	q := r.db.Table("appraisals a").
		Select("a.grade AS grade, COUNT(a.id) AS count").
		Joins("JOIN employees e ON e.id = a.employee_id AND e.deleted_at IS NULL").
		Where("a.period_id = ? AND a.final_score IS NOT NULL", periodID)
	if deptID != 0 {
		q = q.Where("e.department_id = ?", deptID)
	}
	var out []GradeCountRow
	err := q.Group("a.grade").Scan(&out).Error
	return out, err
}
