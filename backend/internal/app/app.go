// Package app merakit service dan handler; dipakai oleh cmd/api dan test HTTP.
package app

import (
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/retechno/sikerja/backend/internal/config"
	"github.com/retechno/sikerja/backend/internal/handler"
	"github.com/retechno/sikerja/backend/internal/repository"
	"github.com/retechno/sikerja/backend/internal/service"
	"github.com/retechno/sikerja/backend/internal/storage"
)

// NewDeps membuat semua service di atas satu database, Redis, dan penyimpanan berkas.
func NewDeps(cfg config.Config, log zerolog.Logger, db *gorm.DB, rdb *redis.Client, mailer service.PasswordResetMailer, files storage.Store) handler.Deps {
	store := repository.NewStore(db)
	cache := service.NewCache(rdb)
	return handler.Deps{
		Cfg:       cfg,
		Log:       log,
		Auth:      service.NewAuthService(store, rdb, cfg, mailer),
		Employees: service.NewEmployeeService(store, files, cache),
		Master:    service.NewMasterService(store, cache),
		Periods:   service.NewPeriodService(store, cache),
		Templates: service.NewTemplateService(store, cache),
		Kpi:       service.NewKpiService(store, files, cache),
		Appraisal: service.NewAppraisalService(store, cache),
		Dashboard: service.NewDashboardService(store, cache),
		Reports:   service.NewReportService(store),
		Notifs:    service.NewNotificationService(store),
		Audit:     service.NewAuditService(store),
	}
}
