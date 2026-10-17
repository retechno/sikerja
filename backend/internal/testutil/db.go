// Package testutil berisi helper test. Bawaan: SQLite in-memory + miniredis (tanpa Docker).
// Dengan TEST_MYSQL_DSN dan/atau TEST_REDIS_ADDR (dipakai CI) test berjalan pada MySQL dan Redis sungguhan:
//
//	TEST_MYSQL_DSN="root:root@tcp(127.0.0.1:3307)/?parseTime=true&loc=UTC"   (akun yang boleh CREATE DATABASE)
//	TEST_REDIS_ADDR=127.0.0.1:6380                                              (DB 15 dikosongkan tiap test)
//
// Skema MySQL dibuat dari migrasi asli (golang-migrate), sehingga migrasi ikut teruji. Jalankan dengan `go test -p 1`
// agar paket tidak berebut Redis yang sama.
package testutil

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/glebarez/sqlite"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/retechno/sikerja/backend/internal/database"
	"github.com/retechno/sikerja/backend/internal/model"
	"github.com/retechno/sikerja/backend/internal/service"
)

func init() { service.BcryptCost = bcrypt.MinCost }

var dbSeq atomic.Int64

// NewDB membuat database baru: MySQL (skema dari migrasi) bila TEST_MYSQL_DSN diisi, selain itu SQLite in-memory
// dengan skema dari model.
func NewDB(t testing.TB) *gorm.DB {
	t.Helper()
	if dsn := os.Getenv("TEST_MYSQL_DSN"); dsn != "" {
		return newMySQL(t, dsn)
	}
	name := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	dsn := fmt.Sprintf("file:%s_%d?mode=memory&cache=shared", name, dbSeq.Add(1))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("buka sqlite: %v", err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(model.All()...); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}

// UsingRealServices: true bila test berjalan pada MySQL/Redis sungguhan.
func UsingRealServices() bool {
	return os.Getenv("TEST_MYSQL_DSN") != "" || os.Getenv("TEST_REDIS_ADDR") != ""
}

var (
	mysqlOnce sync.Once
	mysqlDB   *gorm.DB
	mysqlErr  error
)

// newMySQL memakai satu database per proses test (dibuat ulang dari migrasi asli sekali saja), lalu
// mengosongkan semua tabel di awal setiap test — jauh lebih cepat daripada migrasi per test, dan foreign key tetap ada.
func newMySQL(t testing.TB, adminDSN string) *gorm.DB {
	t.Helper()
	mysqlOnce.Do(func() { mysqlDB, mysqlErr = openMySQL(adminDSN) })
	if mysqlErr != nil {
		t.Fatalf("MySQL untuk test: %v", mysqlErr)
	}
	var tables []string
	if err := mysqlDB.Raw("SELECT table_name FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name <> 'schema_migrations'").
		Scan(&tables).Error; err != nil {
		t.Fatalf("daftar tabel: %v", err)
	}
	// satu koneksi agar FOREIGN_KEY_CHECKS=0 berlaku untuk semua TRUNCATE
	err := mysqlDB.Connection(func(tx *gorm.DB) error {
		if err := tx.Exec("SET FOREIGN_KEY_CHECKS = 0").Error; err != nil {
			return err
		}
		for _, tb := range tables {
			if err := tx.Exec("TRUNCATE TABLE `" + tb + "`").Error; err != nil {
				return err
			}
		}
		return tx.Exec("SET FOREIGN_KEY_CHECKS = 1").Error
	})
	if err != nil {
		t.Fatalf("kosongkan tabel: %v", err)
	}
	return mysqlDB
}

func openMySQL(adminDSN string) (*gorm.DB, error) {
	slash := strings.Index(adminDSN, ")/")
	if slash < 0 {
		return nil, fmt.Errorf("TEST_MYSQL_DSN tidak dikenali: butuh bentuk user:pass@tcp(host:port)/?params")
	}
	rest := adminDSN[slash+2:]
	params := ""
	if q := strings.Index(rest, "?"); q >= 0 {
		params = rest[q:]
	}
	if params == "" {
		params = "?parseTime=true&loc=UTC"
	}
	// satu database stabil per binary test (paket) supaya tidak menumpuk sisa database
	name := "sikerja_test_" + strings.NewReplacer(".", "_", "-", "_").Replace(filepath.Base(os.Args[0]))
	if len(name) > 60 {
		name = name[:60]
	}
	admin, err := gorm.Open(mysql.Open(adminDSN), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		return nil, err
	}
	adminSQL, _ := admin.DB()
	defer func() { _ = adminSQL.Close() }()
	if err := admin.Exec("DROP DATABASE IF EXISTS `" + name + "`").Error; err != nil {
		return nil, err
	}
	if err := admin.Exec("CREATE DATABASE `" + name + "` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci").Error; err != nil {
		return nil, err
	}
	dsn := adminDSN[:slash+2] + name + params
	if err := database.Migrate(dsn); err != nil {
		return nil, fmt.Errorf("migrasi: %w", err)
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		return nil, err
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(20)
	return db, nil
}

// NewRedis mengembalikan klien Redis. Bawaan: miniredis (server dikembalikan agar test bisa memajukan waktu lewat
// FastForward). Dengan TEST_REDIS_ADDR: Redis sungguhan (DB 15, dikosongkan), dan server miniredis bernilai nil.
func NewRedis(t testing.TB) (*redis.Client, *miniredis.Miniredis) {
	t.Helper()
	if addr := os.Getenv("TEST_REDIS_ADDR"); addr != "" {
		rdb := redis.NewClient(&redis.Options{Addr: addr, DB: 15})
		if err := rdb.FlushDB(context.Background()).Err(); err != nil {
			t.Fatalf("redis (TEST_REDIS_ADDR): %v", err)
		}
		t.Cleanup(func() {
			_ = rdb.FlushDB(context.Background()).Err()
			_ = rdb.Close()
		})
		return rdb, nil
	}
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return rdb, mr
}
