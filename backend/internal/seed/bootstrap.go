package seed

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/retechno/sikerja/backend/internal/model"
	"github.com/retechno/sikerja/backend/internal/repository"
	"github.com/retechno/sikerja/backend/internal/service"
)

// BootstrapOptions: data untuk instalasi pertama (production) — tanpa akun atau data demo.
type BootstrapOptions struct {
	Email, Name, Password string
	Now                   time.Time
}

// Bootstrap mengisi data master yang dibutuhkan aplikasi (role, 4 departemen + jabatan, template KPI default,
// aspek perilaku, jenis cuti, hari libur, jadwal kerja) dan satu akun HR pertama. Hanya berjalan di database kosong.
func Bootstrap(db *gorm.DB, opt BootstrapOptions) error {
	opt.Email = strings.ToLower(strings.TrimSpace(opt.Email))
	opt.Name = strings.TrimSpace(opt.Name)
	if opt.Email == "" || !strings.Contains(opt.Email, "@") || opt.Name == "" {
		return errors.New("email dan nama HR pertama wajib diisi")
	}
	if len(opt.Password) < 8 {
		return errors.New("password HR pertama minimal 8 karakter")
	}
	if opt.Now.IsZero() {
		opt.Now = time.Now()
	}
	var n int64
	if err := db.Model(&model.User{}).Count(&n).Error; err != nil {
		return err
	}
	if n > 0 {
		return ErrAlreadySeeded
	}
	hash, err := service.HashPassword(opt.Password)
	if err != nil {
		return err
	}
	return repository.NewStore(db).Tx(func(r repository.Repos) error {
		if err := r.Users.EnsureRoles([]string{model.RoleEmployee, model.RoleManager, model.RoleHR, model.RoleDirector}); err != nil {
			return err
		}
		depts, err := seedDepartments(r)
		if err != nil {
			return err
		}
		if _, err := seedTemplates(r, depts); err != nil {
			return err
		}
		if _, err := seedAspects(r); err != nil {
			return err
		}
		if _, _, err := seedLeaveMaster(r, opt.Now); err != nil {
			return err
		}
		u := &model.User{Email: opt.Email, Password: hash, IsActive: true}
		if err := r.Users.Create(u); err != nil {
			return err
		}
		if err := r.Users.SetRoles(u.ID, []string{model.RoleHR}); err != nil {
			return err
		}
		positions, err := r.Positions.List(0)
		if err != nil {
			return err
		}
		e := &model.Employee{UserID: u.ID, NIK: "HR-0001", Name: opt.Name, JoinDate: service.DateOnly(opt.Now), Status: model.StatusActive}
		for _, p := range positions {
			if p.Name == "Staf HR" {
				pid := p.ID
				e.PositionID = &pid
			}
		}
		if err := r.Employees.Create(e); err != nil {
			return err
		}
		return r.Settings.Set("bootstrap.completed", fmt.Sprint(opt.Now.Unix()))
	})
}
