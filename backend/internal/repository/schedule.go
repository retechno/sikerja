package repository

import (
	"gorm.io/gorm"

	"github.com/retechno/sikerja/backend/internal/model"
)

type ScheduleRepo interface {
	List() ([]model.WorkSchedule, error)
	ByID(id uint64) (*model.WorkSchedule, error)
	// Default mengembalikan jadwal default; ErrNotFound bila belum ada.
	Default() (*model.WorkSchedule, error)
	// Save membuat/memperbarui jadwal beserta seluruh harinya (hari lama diganti).
	Save(s *model.WorkSchedule) error
	// ClearDefault menghapus penanda default dari semua jadwal selain keepID.
	ClearDefault(keepID uint64) error
	Delete(id uint64) error
	// DepartmentCount: jumlah departemen yang memakai jadwal ini.
	DepartmentCount(id uint64) (int64, error)
	NameTaken(name string, exceptID uint64) (bool, error)
}

type scheduleRepo struct{ db *gorm.DB }

func (r *scheduleRepo) List() ([]model.WorkSchedule, error) {
	var out []model.WorkSchedule
	err := r.db.Preload("Days", func(db *gorm.DB) *gorm.DB { return db.Order("weekday ASC") }).
		Order("is_default DESC, name ASC").Find(&out).Error
	return out, err
}

func (r *scheduleRepo) ByID(id uint64) (*model.WorkSchedule, error) {
	var s model.WorkSchedule
	err := r.db.Preload("Days", func(db *gorm.DB) *gorm.DB { return db.Order("weekday ASC") }).First(&s, id).Error
	if err != nil {
		return nil, notFound(err)
	}
	return &s, nil
}

func (r *scheduleRepo) Default() (*model.WorkSchedule, error) {
	var s model.WorkSchedule
	err := r.db.Preload("Days", func(db *gorm.DB) *gorm.DB { return db.Order("weekday ASC") }).
		Where("is_default = ?", true).Order("id ASC").First(&s).Error
	if err != nil {
		return nil, notFound(err)
	}
	return &s, nil
}

func (r *scheduleRepo) Save(s *model.WorkSchedule) error {
	days := s.Days
	if err := r.db.Omit("Days").Save(s).Error; err != nil {
		return err
	}
	if err := r.db.Where("schedule_id = ?", s.ID).Delete(&model.WorkScheduleDay{}).Error; err != nil {
		return err
	}
	for i := range days {
		days[i].ID, days[i].ScheduleID = 0, s.ID
	}
	if len(days) > 0 {
		if err := r.db.Create(&days).Error; err != nil {
			return err
		}
	}
	s.Days = days
	return nil
}

func (r *scheduleRepo) ClearDefault(keepID uint64) error {
	return r.db.Model(&model.WorkSchedule{}).Where("id <> ?", keepID).Update("is_default", false).Error
}

func (r *scheduleRepo) Delete(id uint64) error { return r.db.Delete(&model.WorkSchedule{}, id).Error }

func (r *scheduleRepo) DepartmentCount(id uint64) (int64, error) {
	var n int64
	err := r.db.Model(&model.Department{}).Where("schedule_id = ?", id).Count(&n).Error
	return n, err
}

func (r *scheduleRepo) NameTaken(name string, exceptID uint64) (bool, error) {
	var n int64
	q := r.db.Model(&model.WorkSchedule{}).Where("name = ?", name)
	if exceptID != 0 {
		q = q.Where("id <> ?", exceptID)
	}
	err := q.Count(&n).Error
	return n > 0, err
}
