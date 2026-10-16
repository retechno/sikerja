// Package testutil berisi helper test: SQLite in-memory dan Redis palsu.
package testutil

import (
	"fmt"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/glebarez/sqlite"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/retechno/sikerja/backend/internal/model"
)

var dbSeq atomic.Int64

// NewDB membuat database SQLite in-memory baru dengan skema dari model.
func NewDB(t testing.TB) *gorm.DB {
	t.Helper()
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

// NewRedis menjalankan miniredis dan mengembalikan klien serta servernya
// (server dikembalikan agar test bisa memajukan waktu lewat FastForward).
func NewRedis(t testing.TB) (*redis.Client, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return rdb, mr
}
