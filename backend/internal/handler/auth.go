package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/retechno/sikerja/backend/internal/apperr"
	"github.com/retechno/sikerja/backend/internal/middleware"
	"github.com/retechno/sikerja/backend/internal/service"
)

func (h *H) setAuthCookies(c *gin.Context, p *service.TokenPair) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(middleware.AccessCookie, p.Access, int(p.AccessTTL.Seconds()), "/api", "", h.Cfg.CookieSecure, true)
	refreshAge := 0 // cookie sesi (hilang saat browser ditutup) bila "ingat saya" tidak dicentang
	if p.Remember {
		refreshAge = int(p.RefreshTTL.Seconds())
	}
	c.SetCookie(middleware.RefreshCookie, p.Refresh, refreshAge, "/api/v1/auth", "", h.Cfg.CookieSecure, true)
}

func (h *H) clearAuthCookies(c *gin.Context) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(middleware.AccessCookie, "", -1, "/api", "", h.Cfg.CookieSecure, true)
	c.SetCookie(middleware.RefreshCookie, "", -1, "/api/v1/auth", "", h.Cfg.CookieSecure, true)
}

func (h *H) login(c *gin.Context) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		Remember bool   `json:"remember"`
	}
	if !h.bind(c, &req) {
		return
	}
	pair, u, err := h.Auth.Login(c.Request.Context(), req.Email, req.Password, req.Remember)
	if err != nil {
		h.fail(c, err)
		return
	}
	a, err := h.Auth.ActorFor(u.ID)
	if err != nil {
		h.fail(c, err)
		return
	}
	me, err := h.Employees.Me(*a)
	if err != nil {
		h.fail(c, err)
		return
	}
	h.setAuthCookies(c, pair)
	ok(c, me)
}

func (h *H) refresh(c *gin.Context) {
	tok, err := c.Cookie(middleware.RefreshCookie)
	if err != nil || tok == "" {
		h.fail(c, apperr.Unauthorized("Sesi berakhir, silakan masuk kembali"))
		return
	}
	pair, err := h.Auth.Refresh(c.Request.Context(), tok)
	if err != nil {
		h.clearAuthCookies(c)
		h.fail(c, err)
		return
	}
	h.setAuthCookies(c, pair)
	ok(c, gin.H{"refreshed": true})
}

func (h *H) logout(c *gin.Context) {
	if tok, err := c.Cookie(middleware.RefreshCookie); err == nil {
		h.Auth.Logout(c.Request.Context(), tok)
	}
	h.clearAuthCookies(c)
	noContent(c)
}

func (h *H) forgotPassword(c *gin.Context) {
	var req struct {
		Email string `json:"email"`
	}
	if !h.bind(c, &req) {
		return
	}
	if err := h.Auth.ForgotPassword(req.Email); err != nil {
		// kegagalan kirim email dicatat tetapi tidak dibocorkan ke pemanggil
		h.Log.Error().Err(err).Msg("lupa password")
	}
	ok(c, gin.H{"message": "Jika email terdaftar, tautan pengaturan ulang password telah dikirim."})
}

func (h *H) resetPassword(c *gin.Context) {
	var req struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}
	if !h.bind(c, &req) {
		return
	}
	if err := h.Auth.ResetPassword(c.Request.Context(), req.Token, req.Password); err != nil {
		h.fail(c, err)
		return
	}
	ok(c, gin.H{"message": "Password berhasil diubah. Silakan masuk."})
}

func (h *H) changePassword(c *gin.Context) {
	var req struct {
		Current string `json:"current_password"`
		New     string `json:"new_password"`
	}
	if !h.bind(c, &req) {
		return
	}
	pair, err := h.Auth.ChangePassword(c.Request.Context(), actor(c), req.Current, req.New)
	if err != nil {
		h.fail(c, err)
		return
	}
	h.setAuthCookies(c, pair)
	ok(c, gin.H{"message": "Password berhasil diganti."})
}

func (h *H) me(c *gin.Context) {
	v, err := h.Employees.Me(actor(c))
	if err != nil {
		h.fail(c, err)
		return
	}
	ok(c, v)
}

func (h *H) updateMe(c *gin.Context) {
	var req struct {
		Name  string `json:"name"`
		Phone string `json:"phone"`
	}
	if !h.bind(c, &req) {
		return
	}
	v, err := h.Employees.UpdateProfile(actor(c), req.Name, req.Phone)
	if err != nil {
		h.fail(c, err)
		return
	}
	ok(c, v)
}

func (h *H) uploadMyPhoto(c *gin.Context) {
	a := actor(c)
	f, _, err := c.Request.FormFile("file")
	if err != nil {
		h.fail(c, apperr.Validation("Berkas wajib diunggah", map[string]string{"file": "Pilih berkas foto"}))
		return
	}
	defer func() { _ = f.Close() }()
	v, err := h.Employees.SetPhoto(a, a.EmployeeID, f)
	if err != nil {
		h.fail(c, err)
		return
	}
	ok(c, v)
}
