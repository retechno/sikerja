package service_test

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/retechno/sikerja/backend/internal/apperr"
	"github.com/retechno/sikerja/backend/internal/testutil"
)

var ctx = context.Background()

func TestLoginSuccessAndAuthenticate(t *testing.T) {
	w := testutil.NewWorld(t)
	auth := w.Auth()

	pair, u, err := auth.Login(ctx, "RINA@test.id ", testutil.Password, false)
	require.NoError(t, err)
	assert.Equal(t, "rina@test.id", u.Email)

	actor, err := auth.Authenticate(ctx, pair.Access)
	require.NoError(t, err)
	assert.Equal(t, w.EmpIT1.ID, actor.EmployeeID)
	assert.Equal(t, []string{"employee"}, actor.Roles)

	// refresh token tidak boleh dipakai sebagai access token
	_, err = auth.Authenticate(ctx, pair.Refresh)
	assert.Error(t, err)
}

func TestLoginWrongPassword(t *testing.T) {
	w := testutil.NewWorld(t)
	_, _, err := w.Auth().Login(ctx, "rina@test.id", "salah-salah", false)
	ae, ok := apperr.As(err)
	require.True(t, ok)
	assert.Equal(t, http.StatusUnauthorized, ae.Status)
}

func TestLoginUnknownEmailLooksSame(t *testing.T) {
	w := testutil.NewWorld(t)
	_, _, err := w.Auth().Login(ctx, "tidak-ada@test.id", "password", false)
	ae, ok := apperr.As(err)
	require.True(t, ok)
	assert.Equal(t, http.StatusUnauthorized, ae.Status)
	assert.Equal(t, "Email atau password salah", ae.Message)
}

func TestLoginRateLimit5FailuresLocks1Minute(t *testing.T) {
	w := testutil.NewWorld(t)
	auth := w.Auth()

	for i := 0; i < 5; i++ {
		_, _, err := auth.Login(ctx, "rina@test.id", "salah-salah", false)
		ae, _ := apperr.As(err)
		require.Equal(t, http.StatusUnauthorized, ae.Status, "percobaan ke-%d", i+1)
	}

	// percobaan ke-6, walau password BENAR, ditolak 429 dengan Retry-After
	_, _, err := auth.Login(ctx, "rina@test.id", testutil.Password, false)
	ae, ok := apperr.As(err)
	require.True(t, ok)
	assert.Equal(t, http.StatusTooManyRequests, ae.Status)
	assert.InDelta(t, 60, ae.RetryAfter, 1)

	// akun lain tidak ikut terkunci
	_, _, err = auth.Login(ctx, "fajar@test.id", testutil.Password, false)
	assert.NoError(t, err)

	// setelah 1 menit boleh coba lagi
	w.AdvanceRedis(61 * time.Second)
	_, _, err = auth.Login(ctx, "rina@test.id", testutil.Password, false)
	assert.NoError(t, err)
}

func TestLoginFailureCounterResetsOnSuccess(t *testing.T) {
	w := testutil.NewWorld(t)
	auth := w.Auth()
	for i := 0; i < 4; i++ {
		_, _, _ = auth.Login(ctx, "rina@test.id", "salah-salah", false)
	}
	_, _, err := auth.Login(ctx, "rina@test.id", testutil.Password, false)
	require.NoError(t, err)
	// 4 gagal lagi tidak mengunci karena hitungan sudah di-reset
	for i := 0; i < 4; i++ {
		_, _, err = auth.Login(ctx, "rina@test.id", "salah-salah", false)
		ae, _ := apperr.As(err)
		require.Equal(t, http.StatusUnauthorized, ae.Status)
	}
}

func TestInactiveUserCannotLogin(t *testing.T) {
	w := testutil.NewWorld(t)
	require.NoError(t, w.Store.R().Users.SetActive(w.EmpIT1.UserID, false))
	_, _, err := w.Auth().Login(ctx, "rina@test.id", testutil.Password, false)
	assert.Error(t, err)
}

func TestRefreshRotatesAndOldTokenIsRevoked(t *testing.T) {
	w := testutil.NewWorld(t)
	auth := w.Auth()
	pair, _, err := auth.Login(ctx, "rina@test.id", testutil.Password, true)
	require.NoError(t, err)
	assert.True(t, pair.Remember)

	next, err := auth.Refresh(ctx, pair.Refresh)
	require.NoError(t, err)
	assert.True(t, next.Remember, "flag ingat-saya terbawa")
	assert.NotEqual(t, pair.Refresh, next.Refresh)

	_, err = auth.Refresh(ctx, pair.Refresh)
	assert.Error(t, err, "refresh token lama tidak boleh bisa dipakai ulang")
	_, err = auth.Refresh(ctx, next.Refresh)
	assert.NoError(t, err)
}

func TestRefreshRejectsAccessTokenAndGarbage(t *testing.T) {
	w := testutil.NewWorld(t)
	auth := w.Auth()
	pair, _, _ := auth.Login(ctx, "rina@test.id", testutil.Password, false)
	_, err := auth.Refresh(ctx, pair.Access)
	assert.Error(t, err)
	_, err = auth.Refresh(ctx, "bukan.token.jwt")
	assert.Error(t, err)
}

func TestExpiredAccessToken(t *testing.T) {
	w := testutil.NewWorld(t)
	auth := w.Auth()
	pair, _, _ := auth.Login(ctx, "rina@test.id", testutil.Password, false)
	later := time.Now().Add(16 * time.Minute)
	auth.SetClock(func() time.Time { return later })
	_, err := auth.Authenticate(ctx, pair.Access)
	assert.Error(t, err)
}

func TestLogoutRevokesRefresh(t *testing.T) {
	w := testutil.NewWorld(t)
	auth := w.Auth()
	pair, _, _ := auth.Login(ctx, "rina@test.id", testutil.Password, false)
	auth.Logout(ctx, pair.Refresh)
	_, err := auth.Refresh(ctx, pair.Refresh)
	assert.Error(t, err)
}

func TestChangePassword(t *testing.T) {
	w := testutil.NewWorld(t)
	auth := w.Auth()
	pair, _, _ := auth.Login(ctx, "rina@test.id", testutil.Password, false)
	actor := w.Actor(w.EmpIT1)

	_, err := auth.ChangePassword(ctx, actor, "salah", "password-baru-1")
	assert.Error(t, err)
	_, err = auth.ChangePassword(ctx, actor, testutil.Password, "pendek")
	assert.Error(t, err)
	_, err = auth.ChangePassword(ctx, actor, testutil.Password, testutil.Password)
	assert.Error(t, err)

	newPair, err := auth.ChangePassword(ctx, actor, testutil.Password, "password-baru-1")
	require.NoError(t, err)
	assert.NotEmpty(t, newPair.Access)

	_, err = auth.Refresh(ctx, pair.Refresh)
	assert.Error(t, err, "sesi lama dicabut setelah ganti password")
	_, _, err = auth.Login(ctx, "rina@test.id", testutil.Password, false)
	assert.Error(t, err)
	_, _, err = auth.Login(ctx, "rina@test.id", "password-baru-1", false)
	assert.NoError(t, err)
}

func tokenFromLink(t *testing.T, link string) string {
	t.Helper()
	i := strings.Index(link, "token=")
	require.GreaterOrEqual(t, i, 0)
	return link[i+len("token="):]
}

func TestForgotAndResetPassword(t *testing.T) {
	w := testutil.NewWorld(t)
	auth := w.Auth()

	require.NoError(t, auth.ForgotPassword("tidak-ada@test.id"))
	assert.Empty(t, w.Mailer.Sent, "email tak terdaftar: tidak ada email, tapi tetap sukses")

	require.NoError(t, auth.ForgotPassword("rina@test.id"))
	require.Len(t, w.Mailer.Sent, 1)
	assert.Equal(t, "rina@test.id", w.Mailer.Sent[0].To)
	token := tokenFromLink(t, w.Mailer.Sent[0].Link)

	assert.Error(t, auth.ResetPassword(ctx, "token-ngawur", "password-baru-1"))
	assert.Error(t, auth.ResetPassword(ctx, token, "pendek"))
	require.NoError(t, auth.ResetPassword(ctx, token, "password-baru-1"))
	assert.Error(t, auth.ResetPassword(ctx, token, "password-baru-2"), "token sekali pakai")

	_, _, err := auth.Login(ctx, "rina@test.id", "password-baru-1", false)
	assert.NoError(t, err)
}

func TestResetTokenExpiresAfterOneHour(t *testing.T) {
	w := testutil.NewWorld(t)
	auth := w.Auth()
	require.NoError(t, auth.ForgotPassword("rina@test.id"))
	token := tokenFromLink(t, w.Mailer.Sent[0].Link)
	auth.SetClock(func() time.Time { return time.Now().Add(61 * time.Minute) })
	assert.Error(t, auth.ResetPassword(ctx, token, "password-baru-1"))
}

func TestNewResetInvalidatesPreviousToken(t *testing.T) {
	w := testutil.NewWorld(t)
	auth := w.Auth()
	require.NoError(t, auth.ForgotPassword("rina@test.id"))
	require.NoError(t, auth.ForgotPassword("rina@test.id"))
	require.Len(t, w.Mailer.Sent, 2)
	assert.Error(t, auth.ResetPassword(ctx, tokenFromLink(t, w.Mailer.Sent[0].Link), "password-baru-1"))
	assert.NoError(t, auth.ResetPassword(ctx, tokenFromLink(t, w.Mailer.Sent[1].Link), "password-baru-1"))
}
