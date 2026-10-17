// Command worker memproses antrian asynq (email lupa password; notifikasi & pengingat menyusul di Rilis 2).
package main

import (
	"os"
	"time"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog"

	"github.com/retechno/sikerja/backend/internal/config"
	"github.com/retechno/sikerja/backend/internal/mail"
)

func main() {
	log := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339}).With().Timestamp().Logger()
	cfg, err := config.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("konfigurasi")
	}
	srv := asynq.NewServer(
		asynq.RedisClientOpt{Addr: cfg.RedisAddr, Password: cfg.RedisPass},
		asynq.Config{Concurrency: 5, Queues: map[string]int{"default": 1}},
	)
	log.Info().Str("smtp", cfg.SMTPHost).Msg("worker berjalan")
	if err := srv.Run(mail.Handler(mail.NewSMTPMailer(cfg))); err != nil {
		log.Fatal().Err(err).Msg("worker")
	}
}
