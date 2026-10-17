// Package middleware berisi middleware Gin: autentikasi, RBAC, CSRF, CORS, log, dan pemulihan panic.
package middleware

import (
	"net/http"
	"runtime/debug"
	"slices"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"

	"github.com/retechno/sikerja/backend/internal/apperr"
	"github.com/retechno/sikerja/backend/internal/service"
)

const (
	AccessCookie  = "sk_at"
	RefreshCookie = "sk_rt"
	actorKey      = "actor"
)

// WriteError menulis respons error standar { "error": { code, message, fields } }.
func WriteError(c *gin.Context, e *apperr.Error) {
	if e.RetryAfter > 0 {
		c.Header("Retry-After", strconv.Itoa(e.RetryAfter))
	}
	c.AbortWithStatusJSON(e.Status, gin.H{"error": gin.H{"code": e.Code, "message": e.Message, "fields": e.Fields}})
}

// Auth memvalidasi cookie access token dan menaruh Actor (dimuat dari DB) di context.
func Auth(auth *service.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		tok, err := c.Cookie(AccessCookie)
		if err != nil || tok == "" {
			WriteError(c, apperr.Unauthorized("Silakan masuk terlebih dahulu"))
			return
		}
		actor, err := auth.Authenticate(c.Request.Context(), tok)
		if err != nil {
			if ae, ok := apperr.As(err); ok {
				WriteError(c, ae)
			} else {
				WriteError(c, apperr.Unauthorized("Sesi tidak valid"))
			}
			return
		}
		c.Set(actorKey, *actor)
		c.Next()
	}
}

// ActorFrom mengambil Actor yang disimpan middleware Auth.
func ActorFrom(c *gin.Context) service.Actor {
	v, _ := c.Get(actorKey)
	a, _ := v.(service.Actor)
	return a
}

// RequireRoles menolak (403) bila actor tidak punya salah satu role. Cek kepemilikan data tetap di service.
func RequireRoles(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		a := ActorFrom(c)
		if !a.HasAny(roles...) {
			WriteError(c, apperr.Forbidden("Anda tidak berhak mengakses fitur ini"))
			return
		}
		c.Next()
	}
}

// CSRF: request yang mengubah data wajib membawa header X-Requested-With. Header kustom tidak bisa dikirim
// lintas-origin tanpa lolos preflight CORS, ditambah cookie SameSite=Lax.
func CSRF() gin.HandlerFunc {
	return func(c *gin.Context) {
		switch c.Request.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			c.Next()
			return
		}
		if c.GetHeader("X-Requested-With") != "XMLHttpRequest" {
			WriteError(c, apperr.Forbidden("Permintaan ditolak (CSRF)"))
			return
		}
		c.Next()
	}
}

// CORS hanya mengizinkan origin web yang terdaftar.
func CORS(origins []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" && slices.Contains(origins, origin) {
			h := c.Writer.Header()
			h.Set("Access-Control-Allow-Origin", origin)
			h.Set("Access-Control-Allow-Credentials", "true")
			h.Set("Access-Control-Allow-Headers", "Content-Type, X-Requested-With")
			h.Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			h.Add("Vary", "Origin")
		}
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// RateLimit membatasi jumlah request per IP klien pada satu kelompok route (jendela tetap, counter di Redis).
// Bila Redis tidak terjangkau request tetap dilayani (pembatasan per akun di service tetap berlaku).
func RateLimit(rdb *redis.Client, bucket string, max int, window time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		if rdb == nil {
			c.Next()
			return
		}
		key := "rl:" + bucket + ":" + c.ClientIP()
		ctx := c.Request.Context()
		n, err := rdb.Incr(ctx, key).Result()
		if err != nil {
			c.Next()
			return
		}
		if n == 1 {
			rdb.Expire(ctx, key, window)
		}
		if int(n) > max {
			secs := int(window.Seconds())
			if ttl, err := rdb.TTL(ctx, key).Result(); err == nil && ttl > 0 {
				secs = int((ttl + time.Second - 1) / time.Second)
			}
			WriteError(c, apperr.TooMany("Terlalu banyak permintaan. Coba lagi sebentar lagi", secs))
			return
		}
		c.Next()
	}
}

// SecurityHeaders menambah header keamanan dasar.
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.Writer.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "same-origin")
		c.Next()
	}
}

// BodyLimit membatasi ukuran body (upload diberi batas lebih besar di route-nya).
func BodyLimit(max int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, max)
		c.Next()
	}
}

// Logger mencatat setiap request lewat zerolog.
func Logger(log zerolog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		ev := log.Info()
		if c.Writer.Status() >= 500 {
			ev = log.Error()
		}
		ev.Str("method", c.Request.Method).Str("path", c.Request.URL.Path).Int("status", c.Writer.Status()).
			Dur("dur", time.Since(start)).Str("ip", c.ClientIP()).Msg("http")
	}
}

// Recovery mengubah panic menjadi 500 tanpa membocorkan detail.
func Recovery(log zerolog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				log.Error().Interface("panic", r).Bytes("stack", debug.Stack()).Msg("panic")
				WriteError(c, apperr.New(http.StatusInternalServerError, "internal_error", "Terjadi kesalahan pada server"))
			}
		}()
		c.Next()
	}
}
