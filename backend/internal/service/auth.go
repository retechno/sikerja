package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"

	"github.com/retechno/sikerja/backend/internal/apperr"
	"github.com/retechno/sikerja/backend/internal/config"
	"github.com/retechno/sikerja/backend/internal/model"
	"github.com/retechno/sikerja/backend/internal/repository"
)

// ctxT: alias agar signature service ringkas.
type ctxT = context.Context

const (
	MinPasswordLen = 8
	resetTTL       = time.Hour
	failWindow     = 15 * time.Minute
)

// PasswordResetMailer mengirim tautan reset (lewat antrian di produksi).
type PasswordResetMailer interface {
	SendPasswordReset(toEmail, name, link string) error
}

type TokenPair struct {
	Access     string
	Refresh    string
	AccessTTL  time.Duration
	RefreshTTL time.Duration
	Remember   bool
}

type AuthService struct {
	store  repository.Store
	rdb    *redis.Client
	cfg    config.Config
	mailer PasswordResetMailer
	now    func() time.Time
	// dummyHash dipakai agar waktu respons sama untuk email yang tidak terdaftar.
	dummyHash []byte
}

func NewAuthService(store repository.Store, rdb *redis.Client, cfg config.Config, mailer PasswordResetMailer) *AuthService {
	h, _ := bcrypt.GenerateFromPassword([]byte("dummy-password"), BcryptCost)
	return &AuthService{store: store, rdb: rdb, cfg: cfg, mailer: mailer, now: time.Now, dummyHash: h}
}

// SetClock mengganti jam (untuk test).
func (s *AuthService) SetClock(f func() time.Time) { s.now = f }

type claims struct {
	Type     string `json:"typ"` // access | refresh
	Remember bool   `json:"rem,omitempty"`
	jwt.RegisteredClaims
}

// BcryptCost bisa diturunkan oleh test agar cepat.
var BcryptCost = bcrypt.DefaultCost

func HashPassword(pw string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(pw), BcryptCost)
	return string(b), err
}

func ValidatePassword(pw string) error {
	if len([]rune(pw)) < MinPasswordLen {
		return apperr.Validation("Password terlalu pendek", map[string]string{"password": fmt.Sprintf("Minimal %d karakter", MinPasswordLen)})
	}
	if len(pw) > 72 { // batas bcrypt
		return apperr.Validation("Password terlalu panjang", map[string]string{"password": "Maksimal 72 karakter"})
	}
	return nil
}

// ---------- login & rate limit ----------

func failKey(email string) string { return "login:fail:" + email }
func lockKey(email string) string { return "login:lock:" + email }

var errBadLogin = apperr.Unauthorized("Email atau password salah")

// Login memverifikasi kredensial. 5 kali gagal berturut-turut → terkunci 1 menit (Redis).
func (s *AuthService) Login(ctx ctxT, email, password string, remember bool) (*TokenPair, *model.User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" || password == "" {
		return nil, nil, apperr.Validation("Email dan password wajib diisi", map[string]string{"email": "Wajib diisi"})
	}
	if ttl, err := s.rdb.TTL(ctx, lockKey(email)).Result(); err == nil && ttl > 0 {
		secs := int((ttl + time.Second - 1) / time.Second)
		return nil, nil, apperr.TooMany(fmt.Sprintf("Terlalu banyak percobaan. Coba lagi dalam %d detik", secs), secs)
	}

	u, err := s.store.R().Users.ByEmail(email)
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return nil, nil, err
	}
	hash := s.dummyHash
	if u != nil {
		hash = []byte(u.Password)
	}
	pwOK := bcrypt.CompareHashAndPassword(hash, []byte(password)) == nil
	if u == nil || !pwOK || !u.IsActive {
		if lockErr := s.registerFailure(ctx, email); lockErr != nil {
			return nil, nil, lockErr
		}
		return nil, nil, errBadLogin
	}

	s.rdb.Del(ctx, failKey(email), lockKey(email))
	_ = s.store.R().Users.TouchLogin(u.ID, s.now())
	pair, err := s.issue(ctx, u.ID, remember)
	return pair, u, err
}

func (s *AuthService) registerFailure(ctx ctxT, email string) error {
	n, err := s.rdb.Incr(ctx, failKey(email)).Result()
	if err != nil {
		return fmt.Errorf("redis: %w", err)
	}
	if n == 1 {
		s.rdb.Expire(ctx, failKey(email), failWindow)
	}
	if int(n) >= s.cfg.LoginMaxFails {
		s.rdb.Del(ctx, failKey(email))
		s.rdb.Set(ctx, lockKey(email), "1", s.cfg.LoginLockTTL)
	}
	return nil
}

// ---------- token ----------

func refreshSet(uid uint64) string { return fmt.Sprintf("rt:%d", uid) }

func (s *AuthService) sign(uid uint64, typ string, ttl time.Duration, jti string, remember bool) (string, error) {
	now := s.now()
	c := claims{
		Type:     typ,
		Remember: remember,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   fmt.Sprint(uid),
			ID:        jti,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString([]byte(s.cfg.JWTSecret))
}

func (s *AuthService) issue(ctx ctxT, uid uint64, remember bool) (*TokenPair, error) {
	jti := uuid.NewString()
	access, err := s.sign(uid, "access", s.cfg.AccessTTL, uuid.NewString(), false)
	if err != nil {
		return nil, err
	}
	refresh, err := s.sign(uid, "refresh", s.cfg.RefreshTTL, jti, remember)
	if err != nil {
		return nil, err
	}
	pipe := s.rdb.TxPipeline()
	pipe.SAdd(ctx, refreshSet(uid), jti)
	pipe.Expire(ctx, refreshSet(uid), s.cfg.RefreshTTL)
	if _, err := pipe.Exec(ctx); err != nil {
		return nil, fmt.Errorf("redis: %w", err)
	}
	return &TokenPair{Access: access, Refresh: refresh, AccessTTL: s.cfg.AccessTTL, RefreshTTL: s.cfg.RefreshTTL, Remember: remember}, nil
}

func (s *AuthService) parse(tok, typ string) (*claims, uint64, error) {
	var c claims
	t, err := jwt.ParseWithClaims(tok, &c, func(t *jwt.Token) (any, error) { return []byte(s.cfg.JWTSecret), nil },
		jwt.WithValidMethods([]string{"HS256"}), jwt.WithTimeFunc(s.now))
	if err != nil || !t.Valid || c.Type != typ {
		return nil, 0, apperr.Unauthorized("Sesi tidak valid atau sudah berakhir")
	}
	var uid uint64
	if _, err := fmt.Sscan(c.Subject, &uid); err != nil || uid == 0 {
		return nil, 0, apperr.Unauthorized("Sesi tidak valid")
	}
	return &c, uid, nil
}

// Authenticate memvalidasi access token dan memuat Actor dari DB.
func (s *AuthService) Authenticate(ctx ctxT, access string) (*Actor, error) {
	_, uid, err := s.parse(access, "access")
	if err != nil {
		return nil, err
	}
	return s.ActorFor(uid)
}

// ActorFor memuat user+karyawan+role; user nonaktif ditolak.
func (s *AuthService) ActorFor(uid uint64) (*Actor, error) {
	r := s.store.R()
	u, err := r.Users.ByID(uid)
	if err != nil || !u.IsActive {
		return nil, apperr.Unauthorized("Akun tidak aktif")
	}
	e, err := r.Employees.ByUserID(uid)
	if err != nil || e.Status != model.StatusActive {
		return nil, apperr.Unauthorized("Akun tidak aktif")
	}
	roles := make([]string, 0, len(u.Roles))
	for _, ro := range u.Roles {
		roles = append(roles, ro.Name)
	}
	return &Actor{UserID: uid, EmployeeID: e.ID, DepartmentID: e.DepartmentID, Name: e.Name, Roles: roles}, nil
}

// Refresh menukar refresh token dengan pasangan baru (rotasi); token lama dicabut.
func (s *AuthService) Refresh(ctx ctxT, refresh string) (*TokenPair, error) {
	c, uid, err := s.parse(refresh, "refresh")
	if err != nil {
		return nil, err
	}
	removed, err := s.rdb.SRem(ctx, refreshSet(uid), c.ID).Result()
	if err != nil {
		return nil, fmt.Errorf("redis: %w", err)
	}
	if removed == 0 { // sudah dicabut / dipakai ulang
		return nil, apperr.Unauthorized("Sesi tidak valid atau sudah berakhir")
	}
	if _, err := s.ActorFor(uid); err != nil {
		return nil, err
	}
	return s.issue(ctx, uid, c.Remember)
}

// Logout mencabut refresh token (bila valid). Selalu sukses.
func (s *AuthService) Logout(ctx ctxT, refresh string) {
	if refresh == "" {
		return
	}
	if c, uid, err := s.parse(refresh, "refresh"); err == nil {
		s.rdb.SRem(ctx, refreshSet(uid), c.ID)
	}
}

func (s *AuthService) revokeAll(ctx ctxT, uid uint64) { s.rdb.Del(ctx, refreshSet(uid)) }

// ---------- password ----------

// ChangePassword mengganti password; semua sesi lain dicabut dan sesi ini diberi token baru.
func (s *AuthService) ChangePassword(ctx ctxT, a Actor, current, next string) (*TokenPair, error) {
	if err := ValidatePassword(next); err != nil {
		return nil, err
	}
	u, err := s.store.R().Users.ByID(a.UserID)
	if err != nil {
		return nil, apperr.Unauthorized("Akun tidak ditemukan")
	}
	if bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(current)) != nil {
		return nil, apperr.Validation("Password saat ini salah", map[string]string{"current_password": "Password saat ini salah"})
	}
	if current == next {
		return nil, apperr.Validation("Password baru harus berbeda", map[string]string{"new_password": "Harus berbeda dari password saat ini"})
	}
	hash, err := HashPassword(next)
	if err != nil {
		return nil, err
	}
	if err := s.store.R().Users.UpdatePassword(u.ID, hash); err != nil {
		return nil, err
	}
	s.revokeAll(ctx, u.ID)
	return s.issue(ctx, u.ID, false)
}

func hashToken(t string) string {
	h := sha256.Sum256([]byte(t))
	return hex.EncodeToString(h[:])
}

// ForgotPassword selalu berhasil (tidak membocorkan apakah email terdaftar).
func (s *AuthService) ForgotPassword(email string) error {
	r := s.store.R()
	u, err := r.Users.ByEmail(email)
	if err != nil || !u.IsActive {
		return nil
	}
	e, err := r.Employees.ByUserID(u.ID)
	if err != nil {
		return nil
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	now := s.now()
	if err := r.Resets.InvalidateForUser(u.ID, now); err != nil {
		return err
	}
	if err := r.Resets.Create(&model.PasswordReset{UserID: u.ID, TokenHash: hashToken(token), ExpiresAt: now.Add(resetTTL)}); err != nil {
		return err
	}
	link := fmt.Sprintf("%s/reset-password?token=%s", s.cfg.FrontendURL, token)
	return s.mailer.SendPasswordReset(u.Email, e.Name, link)
}

var errBadToken = apperr.BadRequest("Tautan reset tidak valid atau sudah kedaluwarsa")

// ResetPassword memakai token (sekali pakai, 1 jam).
func (s *AuthService) ResetPassword(ctx ctxT, token, newPassword string) error {
	if err := ValidatePassword(newPassword); err != nil {
		return err
	}
	var uid uint64
	err := s.store.Tx(func(r repository.Repos) error {
		pr, err := r.Resets.ByHash(hashToken(token))
		if err != nil {
			return errBadToken
		}
		now := s.now()
		if pr.UsedAt != nil || now.After(pr.ExpiresAt) {
			return errBadToken
		}
		hash, err := HashPassword(newPassword)
		if err != nil {
			return err
		}
		if err := r.Users.UpdatePassword(pr.UserID, hash); err != nil {
			return err
		}
		uid = pr.UserID
		return r.Resets.MarkUsed(pr.ID, now)
	})
	if err != nil {
		return err
	}
	s.revokeAll(ctx, uid)
	return nil
}
