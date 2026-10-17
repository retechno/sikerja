package service

import (
	"strings"
	"unicode/utf8"

	"github.com/retechno/sikerja/backend/internal/apperr"
	"github.com/retechno/sikerja/backend/internal/model"
	"github.com/retechno/sikerja/backend/internal/repository"
)

// MasterService mengelola departemen dan jabatan (H-16). Baca: semua pengguna login; tulis: HR.
type MasterService struct {
	store repository.Store
	cache *Cache
}

func NewMasterService(store repository.Store, cache *Cache) *MasterService {
	return &MasterService{store: store, cache: cache}
}

type DepartmentInput struct {
	Name   string  `json:"name"`
	Code   string  `json:"code"`
	HeadID *uint64 `json:"head_id"`
	// ScheduleID: jadwal kerja departemen; kosong = jadwal default perusahaan.
	ScheduleID *uint64 `json:"schedule_id"`
}

type PositionInput struct {
	Name         string  `json:"name"`
	DepartmentID *uint64 `json:"department_id"`
}

func requireMasterWrite(a Actor) error {
	if !a.IsHR() {
		return apperr.Forbidden("Hanya HR yang dapat mengubah data master")
	}
	return nil
}

func (s *MasterService) Departments() ([]model.Department, error) {
	return s.store.R().Departments.List()
}

func (s *MasterService) Positions(deptID uint64) ([]model.Position, error) {
	return s.store.R().Positions.List(deptID)
}

func (in *DepartmentInput) validate() map[string]string {
	f := map[string]string{}
	in.Name, in.Code = strings.TrimSpace(in.Name), strings.ToUpper(strings.TrimSpace(in.Code))
	if in.Name == "" || utf8.RuneCountInString(in.Name) > 100 {
		f["name"] = "Nama wajib diisi (maks. 100 karakter)"
	}
	if in.Code == "" || utf8.RuneCountInString(in.Code) > 20 {
		f["code"] = "Kode wajib diisi (maks. 20 karakter)"
	}
	return f
}

func (s *MasterService) headCheck(r repository.Repos, deptID uint64, headID *uint64, f map[string]string) {
	if headID == nil {
		return
	}
	e, err := r.Employees.ByID(*headID)
	if err != nil || e.DepartmentID == nil || (deptID != 0 && *e.DepartmentID != deptID) {
		f["head_id"] = "Kepala harus karyawan departemen ini"
	}
}

func (s *MasterService) CreateDepartment(a Actor, in DepartmentInput) (*model.Department, error) {
	if err := requireMasterWrite(a); err != nil {
		return nil, err
	}
	f := in.validate()
	r := s.store.R()
	if len(f) == 0 {
		if taken, err := r.Departments.CodeTaken(in.Code, 0); err != nil {
			return nil, err
		} else if taken {
			f["code"] = "Kode sudah dipakai"
		}
	}
	if in.HeadID != nil {
		f["head_id"] = "Kepala bisa diatur setelah departemen punya anggota"
	}
	if len(f) > 0 {
		return nil, apperr.Validation("Departemen tidak valid", f)
	}
	if in.ScheduleID != nil {
		if _, err := r.Schedules.ByID(*in.ScheduleID); err != nil {
			return nil, apperr.Validation("Departemen tidak valid", map[string]string{"schedule_id": "Jadwal kerja tidak ditemukan"})
		}
	}
	d := &model.Department{Name: in.Name, Code: in.Code, ScheduleID: in.ScheduleID}
	if err := r.Departments.Create(d); err != nil {
		return nil, err
	}
	return d, nil
}

func (s *MasterService) UpdateDepartment(a Actor, id uint64, in DepartmentInput) (*model.Department, error) {
	if err := requireMasterWrite(a); err != nil {
		return nil, err
	}
	f := in.validate()
	r := s.store.R()
	d, err := r.Departments.ByID(id)
	if err != nil {
		return nil, apperr.NotFound("Departemen tidak ditemukan")
	}
	if len(f) == 0 {
		if taken, err := r.Departments.CodeTaken(in.Code, id); err != nil {
			return nil, err
		} else if taken {
			f["code"] = "Kode sudah dipakai"
		}
	}
	s.headCheck(r, id, in.HeadID, f)
	if in.ScheduleID != nil {
		if _, err := r.Schedules.ByID(*in.ScheduleID); err != nil {
			f["schedule_id"] = "Jadwal kerja tidak ditemukan"
		}
	}
	if len(f) > 0 {
		return nil, apperr.Validation("Departemen tidak valid", f)
	}
	d.Name, d.Code, d.HeadID, d.ScheduleID = in.Name, in.Code, in.HeadID, in.ScheduleID
	return d, r.Departments.Update(d)
}

func (s *MasterService) DeleteDepartment(a Actor, id uint64) error {
	if err := requireMasterWrite(a); err != nil {
		return err
	}
	r := s.store.R()
	if _, err := r.Departments.ByID(id); err != nil {
		return apperr.NotFound("Departemen tidak ditemukan")
	}
	if used, err := r.Departments.InUse(id); err != nil {
		return err
	} else if used {
		return apperr.Conflict("Departemen masih dipakai karyawan atau template KPI")
	}
	return r.Departments.Delete(id)
}

func (in *PositionInput) validate() map[string]string {
	f := map[string]string{}
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" || utf8.RuneCountInString(in.Name) > 100 {
		f["name"] = "Nama wajib diisi (maks. 100 karakter)"
	}
	return f
}

func (s *MasterService) CreatePosition(a Actor, in PositionInput) (*model.Position, error) {
	if err := requireMasterWrite(a); err != nil {
		return nil, err
	}
	f := in.validate()
	r := s.store.R()
	if in.DepartmentID != nil {
		if _, err := r.Departments.ByID(*in.DepartmentID); err != nil {
			f["department_id"] = "Departemen tidak ditemukan"
		}
	}
	if len(f) > 0 {
		return nil, apperr.Validation("Jabatan tidak valid", f)
	}
	p := &model.Position{Name: in.Name, DepartmentID: in.DepartmentID}
	return p, r.Positions.Create(p)
}

func (s *MasterService) UpdatePosition(a Actor, id uint64, in PositionInput) (*model.Position, error) {
	if err := requireMasterWrite(a); err != nil {
		return nil, err
	}
	f := in.validate()
	r := s.store.R()
	p, err := r.Positions.ByID(id)
	if err != nil {
		return nil, apperr.NotFound("Jabatan tidak ditemukan")
	}
	if in.DepartmentID != nil {
		if _, err := r.Departments.ByID(*in.DepartmentID); err != nil {
			f["department_id"] = "Departemen tidak ditemukan"
		}
	}
	if len(f) > 0 {
		return nil, apperr.Validation("Jabatan tidak valid", f)
	}
	p.Name, p.DepartmentID = in.Name, in.DepartmentID
	return p, r.Positions.Update(p)
}

func (s *MasterService) DeletePosition(a Actor, id uint64) error {
	if err := requireMasterWrite(a); err != nil {
		return err
	}
	r := s.store.R()
	if _, err := r.Positions.ByID(id); err != nil {
		return apperr.NotFound("Jabatan tidak ditemukan")
	}
	if used, err := r.Positions.InUse(id); err != nil {
		return err
	} else if used {
		return apperr.Conflict("Jabatan masih dipakai karyawan")
	}
	return r.Positions.Delete(id)
}
