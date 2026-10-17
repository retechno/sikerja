package service

import (
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	dashboardTTL = 5 * time.Minute
	cacheVerKey  = "dash:ver"
)

// Cache menyimpan hasil dasbor 5 menit di Redis. Kunci memuat nomor versi yang dinaikkan
// (Bump) setiap ada perubahan realisasi/penilaian/periode sehingga data tidak basi.
type Cache struct{ rdb *redis.Client }

func NewCache(rdb *redis.Client) *Cache { return &Cache{rdb: rdb} }

func (c *Cache) version(ctx context.Context) string {
	if c == nil || c.rdb == nil {
		return "0"
	}
	v, err := c.rdb.Get(ctx, cacheVerKey).Result()
	if err != nil {
		return "0"
	}
	return v
}

// Bump membuang semua cache dasbor.
func (c *Cache) Bump(ctx context.Context) {
	if c == nil || c.rdb == nil {
		return
	}
	c.rdb.Incr(ctx, cacheVerKey)
}

// Remember mengembalikan nilai cache bila ada; jika tidak, memanggil fn dan menyimpannya.
// Kegagalan Redis tidak menggagalkan request (fallback ke fn).
func Remember[T any](ctx context.Context, c *Cache, key string, fn func() (T, error)) (T, error) {
	var zero T
	if c == nil || c.rdb == nil {
		return fn()
	}
	full := "dash:" + c.version(ctx) + ":" + key
	if raw, err := c.rdb.Get(ctx, full).Bytes(); err == nil {
		var out T
		if json.Unmarshal(raw, &out) == nil {
			return out, nil
		}
	}
	v, err := fn()
	if err != nil {
		return zero, err
	}
	if raw, err := json.Marshal(v); err == nil {
		c.rdb.Set(ctx, full, raw, dashboardTTL)
	}
	return v, nil
}
