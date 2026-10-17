// Command seed mengisi data awal + data demo: go run ./cmd/seed [-fresh]
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"

	"github.com/retechno/sikerja/backend/internal/config"
	"github.com/retechno/sikerja/backend/internal/database"
	"github.com/retechno/sikerja/backend/internal/seed"
	"github.com/retechno/sikerja/backend/internal/service"
)

func main() {
	fresh := flag.Bool("fresh", false, "kosongkan semua data lebih dulu")
	flag.Parse()

	log := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339}).With().Timestamp().Logger()
	cfg, err := config.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("konfigurasi")
	}
	if cfg.IsProd() && !*fresh {
		log.Warn().Msg("seed dijalankan di production; pastikan ini disengaja")
	}
	if err := database.Migrate(cfg.DBDSN); err != nil {
		log.Fatal().Err(err).Msg("migrasi")
	}
	db, err := database.Open(cfg.DBDSN, log)
	if err != nil {
		log.Fatal().Err(err).Msg("database")
	}
	res, err := seed.Run(db, seed.Options{Fresh: *fresh})
	if errors.Is(err, seed.ErrAlreadySeeded) {
		// database Rilis 1 yang sudah terisi: lengkapi hanya data izin & cuti bila belum ada
		added, lerr := seed.EnsureLeave(db, time.Now())
		if lerr != nil {
			log.Fatal().Err(lerr).Msg("seed izin & cuti")
		}
		if added {
			fmt.Println("Data izin & cuti (jenis, hari libur, saldo, contoh pengajuan) ditambahkan; data lain tidak diubah.")
			return
		}
		fmt.Println("Data sudah ada — tidak ada yang diubah. Gunakan `go run ./cmd/seed -fresh` untuk mengisi ulang dari nol.")
		return
	}
	if err != nil {
		log.Fatal().Err(err).Msg("seed")
	}
	// buang cache dasbor lama (best effort; Redis boleh tidak tersedia saat seed)
	rdb := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr, Password: cfg.RedisPass})
	service.NewCache(rdb).Bump(context.Background())
	_ = rdb.Close()
	fmt.Printf("Seed selesai: %d akun, periode: %v\n\nAkun demo (password semua: %q)\n", len(res.Accounts), res.Periods, seed.DemoPassword)
	for _, a := range res.Accounts {
		fmt.Printf("  %-9s %-20s %-26s %s\n", a.Role, a.Name, a.Email, a.Department)
	}
}
