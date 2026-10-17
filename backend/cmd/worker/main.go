// Command worker memproses antrian asynq: email (reset password, notifikasi) dan jadwal harian
// (pengingat tenggat penilaian).
package main

import (
	"context"
	"os"
	"time"
	_ "time/tzdata" // zona waktu tertanam agar jadwal tetap benar di image minimal

	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"

	"github.com/retechno/sikerja/backend/internal/config"
	"github.com/retechno/sikerja/backend/internal/database"
	"github.com/retechno/sikerja/backend/internal/mail"
	"github.com/retechno/sikerja/backend/internal/repository"
	"github.com/retechno/sikerja/backend/internal/service"
)

// TaskDeadlineReminder dijadwalkan harian; handler-nya memanggil ReminderService.
const TaskDeadlineReminder = "job:deadline_reminder"

// reminderCron: setiap hari pukul 08.00 menurut zona waktu aplikasi (APP_TIMEZONE, default Asia/Jakarta).
const reminderCron = "0 8 * * *"

func main() {
	log := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339}).With().Timestamp().Logger()
	cfg, err := config.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("konfigurasi")
	}
	db, err := database.Open(cfg.DBDSN, log)
	if err != nil {
		log.Fatal().Err(err).Msg("database")
	}
	rdb := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr, Password: cfg.RedisPass})
	store := repository.NewStore(db)
	smtp := mail.NewSMTPMailer(cfg)
	notifier := service.NewNotifyService(store, log, service.EmailChannel{Sender: smtp, BaseURL: cfg.FrontendURL})
	reminders := service.NewReminderService(store, notifier, rdb)

	redisOpt := asynq.RedisClientOpt{Addr: cfg.RedisAddr, Password: cfg.RedisPass}
	srv := asynq.NewServer(redisOpt, asynq.Config{Concurrency: 5, Queues: map[string]int{"default": 1}})
	mux := mail.Handler(smtp)
	mux.HandleFunc(TaskDeadlineReminder, func(ctx context.Context, _ *asynq.Task) error {
		n, err := reminders.Run(ctx)
		log.Info().Int("penerima", n).Msg("pengingat tenggat penilaian")
		return err
	})

	loc, err := time.LoadLocation(getenv("APP_TIMEZONE", "Asia/Jakarta"))
	if err != nil {
		log.Warn().Err(err).Msg("APP_TIMEZONE tidak valid, memakai UTC")
		loc = time.UTC
	}
	sched := asynq.NewScheduler(redisOpt, &asynq.SchedulerOpts{Location: loc})
	// Unique mencegah antrean ganda bila beberapa worker berjalan bersamaan.
	if _, err := sched.Register(reminderCron, asynq.NewTask(TaskDeadlineReminder, nil),
		asynq.Queue("default"), asynq.Unique(time.Hour), asynq.MaxRetry(3)); err != nil {
		log.Fatal().Err(err).Msg("jadwal")
	}
	if err := sched.Start(); err != nil {
		log.Fatal().Err(err).Msg("scheduler")
	}
	defer sched.Shutdown()

	log.Info().Str("smtp", cfg.SMTPHost).Str("jadwal", reminderCron).Str("zona", loc.String()).Msg("worker berjalan")
	if err := srv.Run(mux); err != nil {
		log.Fatal().Err(err).Msg("worker")
	}
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
