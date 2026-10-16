// Command migrate menerapkan migrasi database: go run ./cmd/migrate
package main

import (
	"fmt"
	"os"

	"github.com/retechno/sikerja/backend/internal/config"
	"github.com/retechno/sikerja/backend/internal/database"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "konfigurasi:", err)
		os.Exit(1)
	}
	if err := database.Migrate(cfg.DBDSN); err != nil {
		fmt.Fprintln(os.Stderr, "migrasi gagal:", err)
		os.Exit(1)
	}
	fmt.Println("Migrasi selesai.")
}
