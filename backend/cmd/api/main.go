// Command api menjalankan server HTTP SiKerja.
package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"

	"github.com/retechno/sikerja/backend/internal/app"
	"github.com/retechno/sikerja/backend/internal/config"
	"github.com/retechno/sikerja/backend/internal/database"
	"github.com/retechno/sikerja/backend/internal/handler"
	"github.com/retechno/sikerja/backend/internal/mail"
	"github.com/retechno/sikerja/backend/internal/storage"
)

func main() {
	log := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339}).With().Timestamp().Logger()
	cfg, err := config.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("konfigurasi")
	}
	if cfg.IsProd() {
		log = zerolog.New(os.Stderr).With().Timestamp().Logger()
	}

	if os.Getenv("AUTO_MIGRATE") != "false" {
		if err := database.Migrate(cfg.DBDSN); err != nil {
			log.Fatal().Err(err).Msg("migrasi database")
		}
	}
	db, err := database.Open(cfg.DBDSN, log)
	if err != nil {
		log.Fatal().Err(err).Msg("database")
	}
	rdb := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr, Password: cfg.RedisPass})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		log.Fatal().Err(err).Msg("redis")
	}
	files, err := storage.NewLocal(cfg.StorageDir)
	if err != nil {
		log.Fatal().Err(err).Msg("storage")
	}
	mailer := mail.NewQueueMailer(cfg.RedisAddr, cfg.RedisPass)
	defer mailer.Close()

	router := handler.NewRouter(app.NewDeps(cfg, log, db, rdb, mailer, files))
	srv := &http.Server{
		Addr: cfg.HTTPAddr, Handler: router,
		ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 60 * time.Second, IdleTimeout: 90 * time.Second,
	}
	go func() {
		log.Info().Str("addr", cfg.HTTPAddr).Str("env", cfg.Env).Msg("API berjalan")
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal().Err(err).Msg("server")
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
	log.Info().Msg("API berhenti")
}
