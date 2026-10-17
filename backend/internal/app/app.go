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

// NewNotifier membuat NotifyService dengan kanal email. Kanal lain (mis. WhatsApp) cukup ditambahkan di sini.
func NewNotifier(cfg config.Config, log zerolog.Logger, store repository.Store, mailer service.EmailSender) *service.NotifyService {
	return service.NewNotifyService(store, log, service.EmailChannel{Sender: mailer, BaseURL: cfg.FrontendURL})
}

// NewDeps membuat semua service di atas satu database, Redis, dan penyimpanan berkas.
func NewDeps(cfg config.Config, log zerolog.Logger, db *gorm.DB, rdb *redis.Client, mailer service.Mailer, files storage.Store) handler.Deps {
	store := repository.NewStore(db)
	cache := service.NewCache(rdb)
	notifier := NewNotifier(cfg, log, store, mailer)
	periods := service.NewPeriodService(store, cache)
	periods.SetNotifier(notifier)
	workingDays := service.NewWorkingDayService(store)
	return handler.Deps{
		Cfg:         cfg,
		Log:         log,
		Auth:        service.NewAuthService(store, rdb, cfg, mailer),
		Employees:   service.NewEmployeeService(store, files, cache),
		Master:      service.NewMasterService(store, cache),
		Periods:     periods,
		Templates:   service.NewTemplateService(store, cache),
		Kpi:         service.NewKpiService(store, files, cache),
		Appraisal:   service.NewAppraisalService(store, cache),
		Dashboard:   service.NewDashboardService(store, cache),
		Reports:     service.NewReportService(store),
		Notifs:      service.NewNotificationService(store),
		Audit:       service.NewAuditService(store),
		Leaves:      service.NewLeaveService(store, files, workingDays, notifier),
		LeaveTypes:  service.NewLeaveTypeService(store),
		Holidays:    service.NewHolidayService(store),
		WorkingDays: workingDays,
		NotifyAdmin: notifier,
		Schedules:   service.NewScheduleService(store),
	}
}
