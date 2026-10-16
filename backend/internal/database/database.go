// Package database membuka koneksi MySQL (GORM) dan menjalankan migrasi golang-migrate.
package database

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-migrate/migrate/v4"
	migmysql "github.com/golang-migrate/migrate/v4/database/mysql"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/rs/zerolog"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/retechno/sikerja/backend/migrations"
)

// Open membuka koneksi dan mencoba ulang selama MySQL masih start (docker compose).
func Open(dsn string, log zerolog.Logger) (*gorm.DB, error) {
	var db *gorm.DB
	var err error
	for i := 0; i < 30; i++ {
		db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Warn)})
		if err == nil {
			var sqlDB *sql.DB
			if sqlDB, err = db.DB(); err == nil {
				if err = sqlDB.Ping(); err == nil {
					sqlDB.SetMaxOpenConns(25)
					sqlDB.SetMaxIdleConns(10)
					sqlDB.SetConnMaxLifetime(30 * time.Minute)
					return db, nil
				}
			}
		}
		log.Warn().Err(err).Int("percobaan", i+1).Msg("menunggu database")
		time.Sleep(2 * time.Second)
	}
	return nil, fmt.Errorf("tidak bisa terhubung ke database: %w", err)
}

// Migrate menerapkan semua migrasi yang belum dijalankan (aman dipanggil berulang).
func Migrate(dsn string) error {
	// golang-migrate memerlukan multiStatements=true untuk berkas dengan banyak statement.
	if !strings.Contains(dsn, "multiStatements=") {
		sep := "?"
		if strings.Contains(dsn, "?") {
			sep = "&"
		}
		dsn += sep + "multiStatements=true"
	}
	sqlDB, err := sql.Open("mysql", dsn)
	if err != nil {
		return err
	}
	defer func() { _ = sqlDB.Close() }()
	drv, err := migmysql.WithInstance(sqlDB, &migmysql.Config{})
	if err != nil {
		return err
	}
	src, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return err
	}
	m, err := migrate.NewWithInstance("iofs", src, "mysql", drv)
	if err != nil {
		return err
	}
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return err
	}
	return nil
}
