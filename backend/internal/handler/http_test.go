package handler_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/retechno/sikerja/backend/internal/app"
	"github.com/retechno/sikerja/backend/internal/handler"
	"github.com/retechno/sikerja/backend/internal/model"
	"github.com/retechno/sikerja/backend/internal/storage"
	"github.com/retechno/sikerja/backend/internal/testutil"
)

type env struct {
	t   *testing.T
	w   *testutil.World
	srv *httptest.Server
}

func newEnv(t *testing.T) *env {
	w := testutil.NewWorld(t)
	files, err := storage.NewLocal(w.Cfg.StorageDir)
	require.NoError(t, err)
	w.Cfg.CORSOrigins = []string{"http://localhost:5173"}
	router := handler.NewRouter(app.NewDeps(w.Cfg, zerolog.Nop(), w.DB, w.Rdb, w.Mailer, files))
	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)
	return &env{t: t, w: w, srv: srv}
}

type client struct {
	e  *env
	hc *http.Client
}

func (e *env) anon() *client {
	jar, _ := cookiejar.New(nil)
	return &client{e: e, hc: &http.Client{Jar: jar, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}

func (e *env) login(email string) *client {
	c := e.anon()
	res, body := c.do("POST", "/api/v1/auth/login", map[string]any{"email": email, "password": testutil.Password})
	require.Equal(e.t, http.StatusOK, res.StatusCode, "login %s: %v", email, body)
	return c
}

type obj = map[string]any

func (c *client) raw(method, path string, body io.Reader, ctype string, hdr map[string]string) *http.Response {
	req, err := http.NewRequest(method, c.e.srv.URL+path, body)
	require.NoError(c.e.t, err)
	if ctype != "" {
		req.Header.Set("Content-Type", ctype)
	}
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	for k, v := range hdr {
		if v == "" {
			req.Header.Del(k)
		} else {
			req.Header.Set(k, v)
		}
	}
	res, err := c.hc.Do(req)
	require.NoError(c.e.t, err)
	return res
}

func (c *client) do(method, path string, body any) (*http.Response, obj) {
	var rd io.Reader
	ct := ""
	if body != nil {
		b, _ := json.Marshal(body)
		rd, ct = bytes.NewReader(b), "application/json"
	}
	res := c.raw(method, path, rd, ct, nil)
	defer res.Body.Close()
	var out obj
	b, _ := io.ReadAll(res.Body)
	_ = json.Unmarshal(b, &out)
	return res, out
}

func (c *client) upload(path, field, filename string, data []byte) (*http.Response, obj) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile(field, filename)
	_, _ = fw.Write(data)
	_ = mw.Close()
	res := c.raw("POST", path, &buf, mw.FormDataContentType(), nil)
	defer res.Body.Close()
	var out obj
	b, _ := io.ReadAll(res.Body)
	_ = json.Unmarshal(b, &out)
	return res, out
}

func errCode(o obj) string {
	if e, ok := o["error"].(obj); ok {
		s, _ := e["code"].(string)
		return s
	}
	return ""
}

// ---------- keamanan dasar ----------

func TestUnauthenticatedRequestsAre401(t *testing.T) {
	e := newEnv(t)
	c := e.anon()
	for _, ep := range []struct{ m, p string }{
		{"GET", "/api/v1/me"}, {"GET", "/api/v1/employees"}, {"GET", "/api/v1/periods"}, {"GET", "/api/v1/kpi/sheet"},
		{"PUT", "/api/v1/kpi/results"}, {"GET", "/api/v1/dashboard/employee"}, {"GET", "/api/v1/reports/scores"},
		{"GET", "/api/v1/kpi/results/1/evidence"}, {"POST", "/api/v1/periods/1/open"}, {"GET", "/api/v1/employees/1/photo"},
	} {
		res, body := c.do(ep.m, ep.p, nil)
		assert.Equal(t, http.StatusUnauthorized, res.StatusCode, "%s %s", ep.m, ep.p)
		assert.Equal(t, "unauthorized", errCode(body))
	}
}

func TestCSRFHeaderRequiredForMutations(t *testing.T) {
	e := newEnv(t)
	c := e.login("rina@test.id")
	res := c.raw("PUT", "/api/v1/me", strings.NewReader(`{"name":"X"}`), "application/json", map[string]string{"X-Requested-With": ""})
	defer res.Body.Close()
	assert.Equal(t, http.StatusForbidden, res.StatusCode)

	res2 := e.anon().raw("POST", "/api/v1/auth/login", strings.NewReader(`{"email":"rina@test.id","password":"password"}`), "application/json", map[string]string{"X-Requested-With": ""})
	defer res2.Body.Close()
	assert.Equal(t, http.StatusForbidden, res2.StatusCode, "login pun dilindungi")
}

func TestLoginSetsHttpOnlyLaxCookiesAndNoTokenInBody(t *testing.T) {
	e := newEnv(t)
	c := e.anon()
	b, _ := json.Marshal(obj{"email": "rina@test.id", "password": "password", "remember": true})
	res := c.raw("POST", "/api/v1/auth/login", bytes.NewReader(b), "application/json", nil)
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	require.Equal(t, 200, res.StatusCode)

	cookies := map[string]*http.Cookie{}
	for _, ck := range res.Cookies() {
		cookies[ck.Name] = ck
	}
	require.Contains(t, cookies, "sk_at")
	require.Contains(t, cookies, "sk_rt")
	for name, ck := range cookies {
		assert.True(t, ck.HttpOnly, name)
		assert.Equal(t, http.SameSiteLaxMode, ck.SameSite, name)
	}
	assert.Equal(t, 15*60, cookies["sk_at"].MaxAge)
	assert.Equal(t, 7*24*3600, cookies["sk_rt"].MaxAge, "ingat saya → 7 hari")
	assert.Equal(t, "/api/v1/auth", cookies["sk_rt"].Path, "refresh token hanya dikirim ke endpoint auth")
	assert.NotContains(t, string(raw), cookies["sk_at"].Value, "token tidak boleh ada di body")
	assert.NotContains(t, string(raw), "password")

	// tanpa "ingat saya" → cookie sesi
	b, _ = json.Marshal(obj{"email": "fajar@test.id", "password": "password"})
	res2 := e.anon().raw("POST", "/api/v1/auth/login", bytes.NewReader(b), "application/json", nil)
	defer res2.Body.Close()
	for _, ck := range res2.Cookies() {
		if ck.Name == "sk_rt" {
			assert.Zero(t, ck.MaxAge)
		}
	}
}

func TestSecureCookieInProduction(t *testing.T) {
	w := testutil.NewWorld(t)
	w.Cfg.CookieSecure = true
	files, _ := storage.NewLocal(w.Cfg.StorageDir)
	srv := httptest.NewServer(handler.NewRouter(app.NewDeps(w.Cfg, zerolog.Nop(), w.DB, w.Rdb, w.Mailer, files)))
	defer srv.Close()
	req, _ := http.NewRequest("POST", srv.URL+"/api/v1/auth/login", strings.NewReader(`{"email":"rina@test.id","password":"password"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	res, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer res.Body.Close()
	require.NotEmpty(t, res.Cookies())
	for _, ck := range res.Cookies() {
		assert.True(t, ck.Secure, ck.Name)
	}
}

func TestLoginRateLimitOverHTTP(t *testing.T) {
	e := newEnv(t)
	c := e.anon()
	for i := 0; i < 5; i++ {
		res, _ := c.do("POST", "/api/v1/auth/login", obj{"email": "rina@test.id", "password": "salah"})
		require.Equal(t, 401, res.StatusCode)
	}
	res, body := c.do("POST", "/api/v1/auth/login", obj{"email": "rina@test.id", "password": "password"})
	assert.Equal(t, http.StatusTooManyRequests, res.StatusCode)
	assert.Equal(t, "60", res.Header.Get("Retry-After"))
	assert.Equal(t, "too_many_requests", errCode(body))
}

func TestRefreshAndLogoutOverHTTP(t *testing.T) {
	e := newEnv(t)
	c := e.login("rina@test.id")
	res, _ := c.do("POST", "/api/v1/auth/refresh", nil)
	assert.Equal(t, 200, res.StatusCode)
	res, _ = c.do("GET", "/api/v1/me", nil)
	assert.Equal(t, 200, res.StatusCode)

	res, _ = c.do("POST", "/api/v1/auth/logout", nil)
	assert.Equal(t, 204, res.StatusCode)
	res, _ = c.do("GET", "/api/v1/me", nil)
	assert.Equal(t, 401, res.StatusCode, "cookie dihapus setelah logout")
	res, _ = c.do("POST", "/api/v1/auth/refresh", nil)
	assert.Equal(t, 401, res.StatusCode)
}

func TestForgotPasswordNeverRevealsAccounts(t *testing.T) {
	e := newEnv(t)
	c := e.anon()
	r1, b1 := c.do("POST", "/api/v1/auth/forgot-password", obj{"email": "rina@test.id"})
	r2, b2 := c.do("POST", "/api/v1/auth/forgot-password", obj{"email": "tidak-ada@test.id"})
	assert.Equal(t, 200, r1.StatusCode)
	assert.Equal(t, 200, r2.StatusCode)
	assert.Equal(t, b1, b2)
	require.Len(t, e.w.Mailer.Sent, 1)

	token := e.w.Mailer.Sent[0].Link[strings.Index(e.w.Mailer.Sent[0].Link, "token=")+6:]
	res, _ := c.do("POST", "/api/v1/auth/reset-password", obj{"token": token, "password": "password-baru-1"})
	assert.Equal(t, 200, res.StatusCode)
	res, _ = c.do("POST", "/api/v1/auth/login", obj{"email": "rina@test.id", "password": "password-baru-1"})
	assert.Equal(t, 200, res.StatusCode)
}

func TestResponseEnvelopesAndNotFound(t *testing.T) {
	e := newEnv(t)
	c := e.login("rina@test.id")
	res, body := c.do("GET", "/api/v1/me", nil)
	assert.Equal(t, 200, res.StatusCode)
	data, _ := body["data"].(obj)
	assert.Equal(t, "rina@test.id", data["email"])
	assert.NotContains(t, data, "password")

	res, body = c.do("GET", "/api/v1/tidak-ada", nil)
	assert.Equal(t, 404, res.StatusCode)
	assert.Equal(t, "not_found", errCode(body))

	res, body = c.do("PUT", "/api/v1/me", obj{"name": ""})
	assert.Equal(t, 422, res.StatusCode)
	assert.Equal(t, "validation_error", errCode(body))
	fields := body["error"].(obj)["fields"].(obj)
	assert.Contains(t, fields, "name")
}

func TestBodyTooLarge(t *testing.T) {
	e := newEnv(t)
	c := e.login("rina@test.id")
	big := `{"name":"` + strings.Repeat("a", 2<<20) + `"}`
	res := c.raw("PUT", "/api/v1/me", strings.NewReader(big), "application/json", nil)
	defer res.Body.Close()
	assert.Equal(t, http.StatusRequestEntityTooLarge, res.StatusCode)
}

func TestCORSOnlyForConfiguredOrigin(t *testing.T) {
	e := newEnv(t)
	c := e.anon()
	res := c.raw("OPTIONS", "/api/v1/me", nil, "", map[string]string{"Origin": "http://localhost:5173"})
	defer res.Body.Close()
	assert.Equal(t, 204, res.StatusCode)
	assert.Equal(t, "http://localhost:5173", res.Header.Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "true", res.Header.Get("Access-Control-Allow-Credentials"))

	res2 := c.raw("OPTIONS", "/api/v1/me", nil, "", map[string]string{"Origin": "https://evil.example"})
	defer res2.Body.Close()
	assert.Empty(t, res2.Header.Get("Access-Control-Allow-Origin"))
}

// ---------- matriks otorisasi per role ----------

func TestRoleMatrix(t *testing.T) {
	e := newEnv(t)
	w := e.w
	pid := w.Active.ID
	clients := map[string]*client{
		"employee": e.login("rina@test.id"),
		"manager":  e.login("bayu@test.id"),
		"hr":       e.login("hr@test.id"),
		"director": e.login("direktur@test.id"),
	}
	type ep struct {
		method, path string
		body         any
		allow        []string // role yang TIDAK boleh 403 (lolos middleware role)
	}
	p := func(f string, a ...any) string { return fmt.Sprintf(f, a...) }
	eps := []ep{
		{"GET", "/api/v1/employees", nil, []string{"hr"}},
		{"POST", "/api/v1/employees", obj{}, []string{"hr"}},
		{"PUT", "/api/v1/employees/1", obj{}, []string{"hr"}},
		{"DELETE", "/api/v1/employees/99999", nil, []string{"hr"}},
		{"GET", "/api/v1/employees-import-template", nil, []string{"hr"}},
		{"POST", "/api/v1/departments", obj{}, []string{"hr"}},
		{"PUT", "/api/v1/departments/1", obj{}, []string{"hr"}},
		{"DELETE", "/api/v1/departments/99999", nil, []string{"hr"}},
		{"POST", "/api/v1/positions", obj{}, []string{"hr"}},
		{"POST", "/api/v1/periods", obj{}, []string{"hr"}},
		{"PUT", p("/api/v1/periods/%d", pid), obj{}, []string{"hr"}},
		{"POST", p("/api/v1/periods/%d/open", pid), nil, []string{"hr"}},
		{"POST", p("/api/v1/periods/%d/close", w.Closed.ID), nil, []string{"hr"}},
		{"POST", p("/api/v1/periods/%d/reopen", w.Active.ID), nil, []string{"hr"}},
		{"POST", "/api/v1/kpi-templates", obj{}, []string{"hr"}},
		{"PUT", p("/api/v1/kpi-templates/%d", w.TplIT.ID), obj{}, []string{"hr"}},
		{"GET", "/api/v1/activity-log", nil, []string{"hr"}},
		{"GET", "/api/v1/kpi-templates", nil, []string{"hr", "director"}},
		{"GET", p("/api/v1/kpi-templates/%d", w.TplIT.ID), nil, []string{"hr", "director"}},
		{"GET", "/api/v1/dashboard/director", nil, []string{"hr", "director"}},
		{"GET", "/api/v1/reports/scores", nil, []string{"hr", "director"}},
		{"GET", "/api/v1/reports/scores/export?format=xlsx", nil, []string{"hr", "director"}},
		{"GET", "/api/v1/reports/scores/export?format=pdf", nil, []string{"hr", "director"}},
		{"POST", p("/api/v1/periods/%d/appraisals/%d/finalize", pid, w.EmpIT1.ID), nil, []string{"hr", "director"}},
		{"GET", p("/api/v1/periods/%d/team", pid), nil, []string{"manager", "director"}},
		{"GET", "/api/v1/dashboard/manager", nil, []string{"manager", "director"}},
		{"POST", "/api/v1/kpi/results/99999/validate", obj{"action": "validate"}, []string{"manager", "director"}},
		// target tidak ada: yang lolos gerbang role mendapat 404; kepemilikan data diuji di test terpisah
		{"PUT", p("/api/v1/periods/%d/appraisals/99999/manager", pid), obj{}, []string{"manager", "director"}},
	}
	for _, x := range eps {
		for role, c := range clients {
			res, _ := c.do(x.method, x.path, x.body)
			allowed := false
			for _, a := range x.allow {
				allowed = allowed || a == role
			}
			if allowed {
				assert.NotEqual(t, http.StatusForbidden, res.StatusCode, "%s %s harus lolos untuk %s", x.method, x.path, role)
				assert.NotEqual(t, http.StatusUnauthorized, res.StatusCode, "%s %s untuk %s", x.method, x.path, role)
			} else {
				assert.Equal(t, http.StatusForbidden, res.StatusCode, "%s %s harus 403 untuk %s", x.method, x.path, role)
			}
		}
	}
}

// ---------- kepemilikan data lewat HTTP ----------

func TestEmployeeCannotOpenOthersDataOverHTTP(t *testing.T) {
	e := newEnv(t)
	w := e.w
	rina, fajar := e.login("rina@test.id"), e.login("fajar@test.id")
	ind := w.TplIT.Indicators[0].ID

	res, body := fajar.do("PUT", "/api/v1/kpi/results", obj{"period_id": w.Active.ID, "employee_id": w.EmpIT2.ID, "indicator_id": ind, "actual": 100})
	require.Equal(t, 200, res.StatusCode, "%v", body)
	rid := fmt.Sprint(int(body["data"].(obj)["id"].(float64)))

	for _, ep := range []struct{ m, p string }{
		{"GET", fmt.Sprintf("/api/v1/kpi/sheet?employee_id=%d&period_id=%d", w.EmpIT2.ID, w.Active.ID)},
		{"GET", fmt.Sprintf("/api/v1/periods/%d/appraisals/%d", w.Active.ID, w.EmpIT2.ID)},
		{"GET", fmt.Sprintf("/api/v1/employees/%d", w.EmpIT2.ID)},
		{"GET", "/api/v1/kpi/results/" + rid + "/evidence"},
	} {
		res, body := rina.do(ep.m, ep.p, nil)
		assert.Equal(t, http.StatusForbidden, res.StatusCode, "%s %s → %v", ep.m, ep.p, body)
		assert.Equal(t, "forbidden", errCode(body))
	}
	res, _ = rina.do("PUT", "/api/v1/kpi/results", obj{"period_id": w.Active.ID, "employee_id": w.EmpIT2.ID, "indicator_id": ind, "actual": 1})
	assert.Equal(t, http.StatusForbidden, res.StatusCode)
	res, _ = rina.do("PUT", fmt.Sprintf("/api/v1/periods/%d/appraisals/%d/self", w.Active.ID, w.EmpIT2.ID), obj{"comment": "x"})
	assert.Equal(t, http.StatusForbidden, res.StatusCode)

	// diri sendiri boleh
	res, _ = rina.do("GET", fmt.Sprintf("/api/v1/kpi/sheet?employee_id=%d&period_id=%d", w.EmpIT1.ID, w.Active.ID), nil)
	assert.Equal(t, 200, res.StatusCode)
	res, _ = rina.do("GET", "/api/v1/kpi/sheet", nil)
	assert.Equal(t, 200, res.StatusCode, "default = diri sendiri, periode aktif")
}

func TestManagerCannotOpenOtherTeamOverHTTP(t *testing.T) {
	e := newEnv(t)
	w := e.w
	bayu := e.login("bayu@test.id")
	for _, p := range []string{
		fmt.Sprintf("/api/v1/kpi/sheet?employee_id=%d&period_id=%d", w.EmpCS1.ID, w.Active.ID),
		fmt.Sprintf("/api/v1/periods/%d/appraisals/%d", w.Active.ID, w.EmpCS1.ID),
		fmt.Sprintf("/api/v1/employees/%d", w.EmpCS1.ID),
	} {
		res, _ := bayu.do("GET", p, nil)
		assert.Equal(t, http.StatusForbidden, res.StatusCode, p)
	}
	res, _ := bayu.do("PUT", fmt.Sprintf("/api/v1/periods/%d/appraisals/%d/manager", w.Active.ID, w.EmpCS1.ID), obj{"scores": obj{}})
	assert.Equal(t, http.StatusForbidden, res.StatusCode)
	res, _ = bayu.do("GET", fmt.Sprintf("/api/v1/kpi/sheet?employee_id=%d&period_id=%d", w.EmpIT1.ID, w.Active.ID), nil)
	assert.Equal(t, 200, res.StatusCode)
}

func TestClosedPeriodRejectsWritesOverHTTP(t *testing.T) {
	e := newEnv(t)
	w := e.w
	rina := e.login("rina@test.id")
	bayu := e.login("bayu@test.id")
	hr := e.login("hr@test.id")

	res, body := rina.do("PUT", "/api/v1/kpi/results", obj{"period_id": w.Closed.ID, "employee_id": w.EmpIT1.ID, "indicator_id": w.TplIT.Indicators[0].ID, "actual": 5})
	assert.Equal(t, http.StatusLocked, res.StatusCode)
	assert.Equal(t, "period_locked", errCode(body))
	res, _ = rina.do("PUT", fmt.Sprintf("/api/v1/periods/%d/appraisals/%d/self", w.Closed.ID, w.EmpIT1.ID), obj{"comment": "x"})
	assert.Equal(t, http.StatusLocked, res.StatusCode)
	res, _ = bayu.do("PUT", fmt.Sprintf("/api/v1/periods/%d/appraisals/%d/manager", w.Closed.ID, w.EmpIT1.ID), obj{"manager_comment": "x"})
	assert.Equal(t, http.StatusLocked, res.StatusCode)

	// hanya HR yang bisa membuka kembali, lalu data bisa diubah
	res, _ = bayu.do("POST", fmt.Sprintf("/api/v1/periods/%d/reopen", w.Closed.ID), nil)
	assert.Equal(t, http.StatusForbidden, res.StatusCode)
	res, _ = hr.do("POST", fmt.Sprintf("/api/v1/periods/%d/reopen", w.Closed.ID), nil)
	assert.Equal(t, 200, res.StatusCode)
	res, _ = rina.do("PUT", "/api/v1/kpi/results", obj{"period_id": w.Closed.ID, "employee_id": w.EmpIT1.ID, "indicator_id": w.TplIT.Indicators[0].ID, "actual": 5})
	assert.Equal(t, 200, res.StatusCode)
}

func pngData() []byte {
	return []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01\x08\x06\x00\x00\x00\x1f\x15\xc4\x89\x00\x00\x00\rIDATx\x9cc\xf8\xff\xff?\x00\x05\xfe\x02\xfe\xa7\x9a\x9c\x88\x00\x00\x00\x00IEND\xaeB`\x82")
}

func TestEvidenceUploadDownloadOverHTTP(t *testing.T) {
	e := newEnv(t)
	w := e.w
	rina, bayu, fajar, citra := e.login("rina@test.id"), e.login("bayu@test.id"), e.login("fajar@test.id"), e.login("citra@test.id")
	_, body := rina.do("PUT", "/api/v1/kpi/results", obj{"period_id": w.Active.ID, "employee_id": w.EmpIT1.ID, "indicator_id": w.TplIT.Indicators[3].ID, "actual": 3})
	rid := int(body["data"].(obj)["id"].(float64))
	path := fmt.Sprintf("/api/v1/kpi/results/%d/evidence", rid)

	res, body := rina.upload(path, "file", "BA instalasi.png", pngData())
	require.Equal(t, 200, res.StatusCode, "%v", body)
	assert.Equal(t, "BA instalasi.png", body["data"].(obj)["evidence_name"])
	assert.NotContains(t, body["data"], "evidence_path", "path internal tidak boleh bocor")

	res, _ = rina.upload(path, "file", "x.html", []byte("<script>alert(1)</script>"))
	assert.Equal(t, 422, res.StatusCode)

	dl := bayu.raw("GET", path, nil, "", nil)
	defer dl.Body.Close()
	data, _ := io.ReadAll(dl.Body)
	assert.Equal(t, 200, dl.StatusCode)
	assert.Equal(t, pngData(), data)
	assert.Equal(t, "image/png", dl.Header.Get("Content-Type"))
	assert.Equal(t, "nosniff", dl.Header.Get("X-Content-Type-Options"))
	assert.Contains(t, dl.Header.Get("Content-Disposition"), "BA instalasi.png")
	assert.Contains(t, dl.Header.Get("Cache-Control"), "no-store")

	for name, c := range map[string]*client{"rekan": fajar, "manajer tim lain": citra} {
		r := c.raw("GET", path, nil, "", nil)
		r.Body.Close()
		assert.Equal(t, http.StatusForbidden, r.StatusCode, name)
	}
	// berkas tidak berada di web root dan tidak bisa ditebak lewat URL statis
	r := e.anon().raw("GET", "/storage/evidence/1/1/x.png", nil, "", nil)
	r.Body.Close()
	assert.Equal(t, 404, r.StatusCode)
}

// ---------- unduhan ----------

func TestExportAndTemplateContentTypes(t *testing.T) {
	e := newEnv(t)
	hr := e.login("hr@test.id")
	for path, ctype := range map[string]string{
		"/api/v1/reports/scores/export?format=xlsx": "spreadsheetml.sheet",
		"/api/v1/reports/scores/export?format=pdf":  "application/pdf",
		"/api/v1/employees-import-template":         "spreadsheetml.sheet",
	} {
		res := hr.raw("GET", path, nil, "", nil)
		b, _ := io.ReadAll(res.Body)
		res.Body.Close()
		assert.Equal(t, 200, res.StatusCode, path)
		assert.Contains(t, res.Header.Get("Content-Type"), ctype, path)
		assert.Contains(t, res.Header.Get("Content-Disposition"), "attachment", path)
		assert.NotEmpty(t, b)
	}
	res, _ := hr.do("GET", "/api/v1/reports/scores/export?format=docx", nil)
	assert.Equal(t, 422, res.StatusCode)
}

func TestImportOverHTTP(t *testing.T) {
	e := newEnv(t)
	hr := e.login("hr@test.id")
	// berkas bukan Excel
	res, body := hr.upload("/api/v1/employees/import", "file", "x.xlsx", []byte("bukan excel"))
	assert.Equal(t, 422, res.StatusCode, "%v", body)
}

func TestAdminCreatedEmployeeCanLogin(t *testing.T) {
	e := newEnv(t)
	w := e.w
	hr := e.login("hr@test.id")
	res, body := hr.do("POST", "/api/v1/employees", obj{"nik": "H-1", "name": "Baru Masuk", "email": "baru@test.id", "department_id": w.ITS.ID,
		"manager_id": w.MgrIT.ID, "join_date": "2026-10-01", "roles": []string{"employee"}})
	require.Equal(t, 201, res.StatusCode, "%v", body)
	temp := body["meta"].(obj)["temp_password"].(string)
	require.NotEmpty(t, temp)
	res, _ = e.anon().do("POST", "/api/v1/auth/login", obj{"email": "baru@test.id", "password": temp})
	assert.Equal(t, 200, res.StatusCode)

	res, body = hr.do("POST", "/api/v1/employees", obj{"nik": "H-2", "name": "Salah", "email": "salah@test.id", "department_id": w.ITS.ID, "join_date": "kemarin"})
	assert.Equal(t, 422, res.StatusCode)
	assert.Contains(t, body["error"].(obj)["fields"], "join_date")
}

func TestPeriodLifecycleOverHTTP(t *testing.T) {
	e := newEnv(t)
	hr := e.login("hr@test.id")
	res, body := hr.do("POST", "/api/v1/periods", obj{"name": "Nov 2026", "type": "monthly", "start_date": "2026-11-01", "end_date": "2026-11-30", "kpi_weight": 70, "behavior_weight": 30})
	require.Equal(t, 201, res.StatusCode, "%v", body)
	id := int(body["data"].(obj)["id"].(float64))
	assert.Equal(t, "draft", body["data"].(obj)["status"])
	res, body = hr.do("POST", fmt.Sprintf("/api/v1/periods/%d/open", id), nil)
	assert.Equal(t, 200, res.StatusCode)
	assert.Equal(t, "active", body["data"].(obj)["status"])

	res, _ = hr.do("POST", "/api/v1/periods", obj{"name": "x", "type": "monthly", "start_date": "2026-11-01", "end_date": "2026-11-30", "kpi_weight": 60, "behavior_weight": 30})
	assert.Equal(t, 422, res.StatusCode, "bobot harus 100")

	var logs []model.ActivityLog
	e.w.DB.Where("action = ?", "period.open").Find(&logs)
	assert.Len(t, logs, 1)
}
