package handler_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/retechno/sikerja/backend/internal/model"
)

// leaveEnv menambah jenis cuti ke dunia test.
func (e *env) withLeaveTypes() (ct, cs *model.LeaveType) {
	q := decimal.NewFromInt(12)
	ct = &model.LeaveType{Name: "Cuti tahunan", Code: "CT", AnnualQuota: &q, DeductsBalance: true, MinTenureMonths: 12, IsActive: true}
	cs = &model.LeaveType{Name: "Cuti sakit", Code: "CS", RequiresAttach: true, AttachMinDays: decimal.NewFromInt(1), IsActive: true}
	require.NoError(e.t, e.w.Store.R().LeaveTypes.Create(ct))
	require.NoError(e.t, e.w.Store.R().LeaveTypes.Create(cs))
	return ct, cs
}

// monday3Weeks: Senin sekitar 3 minggu ke depan (di tahun yang sama dengan sekarang bila memungkinkan).
func monday3Weeks() time.Time {
	d := time.Now().AddDate(0, 0, 21)
	for d.Weekday() != time.Monday {
		d = d.AddDate(0, 0, 1)
	}
	return d
}

func ymd(t time.Time) string { return t.Format("2006-01-02") }

func (c *client) postForm(path string, fields map[string]string, file []byte, filename string) (*http.Response, obj) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for k, v := range fields {
		_ = mw.WriteField(k, v)
	}
	if file != nil {
		fw, _ := mw.CreateFormFile("attachment", filename)
		_, _ = fw.Write(file)
	}
	_ = mw.Close()
	res := c.raw("POST", path, &buf, mw.FormDataContentType(), nil)
	defer res.Body.Close()
	var out obj
	b, _ := io.ReadAll(res.Body)
	_ = json.Unmarshal(b, &out)
	return res, out
}

func data(o obj) obj { d, _ := o["data"].(obj); return d }

func num(v any) uint64 {
	switch x := v.(type) {
	case float64:
		return uint64(x)
	case string:
		n, _ := strconv.ParseUint(x, 10, 64)
		return n
	}
	return 0
}

func TestLeaveEndpointsRequireLogin(t *testing.T) {
	e := newEnv(t)
	c := e.anon()
	for _, ep := range []struct{ m, p string }{
		{"GET", "/api/v1/leave-types"}, {"GET", "/api/v1/holidays"}, {"GET", "/api/v1/leave/balances"},
		{"GET", "/api/v1/leave/preview"}, {"GET", "/api/v1/leave/summary"}, {"GET", "/api/v1/leave/calendar"},
		{"GET", "/api/v1/leave/requests"}, {"POST", "/api/v1/leave/requests"}, {"GET", "/api/v1/leave/requests/1"},
		{"POST", "/api/v1/leave/requests/1/cancel"}, {"GET", "/api/v1/leave/requests/1/attachment"},
		{"POST", "/api/v1/leave/requests/1/approve"}, {"POST", "/api/v1/leave/requests/1/reject"},
		{"GET", "/api/v1/leave/approvals"}, {"GET", "/api/v1/leave/approvals/count"},
		{"POST", "/api/v1/leave-types"}, {"POST", "/api/v1/holidays"}, {"GET", "/api/v1/notification-settings"},
	} {
		res, body := c.do(ep.m, ep.p, nil)
		assert.Equal(t, http.StatusUnauthorized, res.StatusCode, "%s %s", ep.m, ep.p)
		assert.Equal(t, "unauthorized", errCode(body))
	}
}

func TestLeaveAdminEndpointsAreHROnly(t *testing.T) {
	e := newEnv(t)
	ct, _ := e.withLeaveTypes()
	body := obj{"name": "Cuti besar", "code": "CB", "is_active": true}
	for _, who := range []string{"rina@test.id", "bayu@test.id", "direktur@test.id"} {
		c := e.login(who)
		for _, ep := range []struct {
			m, p string
			b    any
		}{
			{"POST", "/api/v1/leave-types", body},
			{"PUT", fmt.Sprintf("/api/v1/leave-types/%d", ct.ID), body},
			{"DELETE", fmt.Sprintf("/api/v1/leave-types/%d", ct.ID), nil},
			{"POST", "/api/v1/holidays", obj{"date": "2026-12-25", "name": "Natal"}},
			{"POST", "/api/v1/holidays/import", obj{"year": 2026}},
			{"GET", "/api/v1/notification-settings", nil},
			{"PUT", "/api/v1/notification-settings", obj{"key": "notify.leave_result", "enabled": false}},
		} {
			res, out := c.do(ep.m, ep.p, ep.b)
			assert.Equal(t, http.StatusForbidden, res.StatusCode, "%s %s %s", who, ep.m, ep.p)
			assert.Equal(t, "forbidden", errCode(out))
		}
	}

	hr := e.login("hr@test.id")
	res, out := hr.do("POST", "/api/v1/leave-types", body)
	require.Equal(t, http.StatusCreated, res.StatusCode, "%v", out)
	res, out = hr.do("POST", "/api/v1/holidays", obj{"date": "2026-12-25", "name": "Natal", "type": "nasional"})
	require.Equal(t, http.StatusCreated, res.StatusCode, "%v", out)
	res, _ = hr.do("POST", "/api/v1/holidays", obj{"date": "bukan-tanggal", "name": "Salah"})
	assert.Equal(t, http.StatusUnprocessableEntity, res.StatusCode)
	res, out = hr.do("GET", "/api/v1/notification-settings", nil)
	require.Equal(t, http.StatusOK, res.StatusCode)
	assert.Len(t, out["data"], 4)
}

func TestLeaveFullFlowOverHTTP(t *testing.T) {
	e := newEnv(t)
	ct, _ := e.withLeaveTypes()
	rina, bayu, hr := e.login("rina@test.id"), e.login("bayu@test.id"), e.login("hr@test.id")
	start := monday3Weeks()
	end := start.AddDate(0, 0, 2) // Sen-Rab = 3 hari kerja

	// pratinjau hari kerja (live)
	res, out := rina.do("GET", fmt.Sprintf("/api/v1/leave/preview?type_id=%d&start_date=%s&end_date=%s", ct.ID, ymd(start), ymd(end)), nil)
	require.Equal(t, 200, res.StatusCode, "%v", out)
	assert.Equal(t, "3", data(out)["working_days"].(obj)["days"])
	assert.Len(t, data(out)["flow"], 2)

	// ajukan lewat multipart
	fields := map[string]string{"leave_type_id": fmt.Sprint(ct.ID), "start_date": ymd(start), "end_date": ymd(end), "reason": "Acara keluarga"}
	res, out = rina.postForm("/api/v1/leave/requests", fields, nil, "")
	require.Equal(t, http.StatusCreated, res.StatusCode, "%v", out)
	id := num(data(out)["id"])
	assert.Equal(t, "pending_manager", data(out)["status"])
	assert.Equal(t, "Menunggu Atasan", data(out)["status_label"])
	assert.Equal(t, true, data(out)["can_cancel"])

	// pengajuan yang sama lagi → tumpang tindih
	res, out = rina.postForm("/api/v1/leave/requests", fields, nil, "")
	assert.Equal(t, http.StatusUnprocessableEntity, res.StatusCode)
	assert.Contains(t, out["error"].(obj)["fields"], "start_date")

	// kepemilikan: rekan setim & manajer departemen lain tidak melihat pengajuan ini
	path := fmt.Sprintf("/api/v1/leave/requests/%d", id)
	for _, who := range []string{"fajar@test.id", "citra@test.id", "dimas@test.id"} {
		r, o := e.login(who).do("GET", path, nil)
		assert.Equal(t, http.StatusNotFound, r.StatusCode, who)
		assert.Equal(t, "not_found", errCode(o))
	}
	res, _ = e.login("fajar@test.id").do("GET", "/api/v1/leave/requests?employee_id="+fmt.Sprint(e.w.EmpIT1.ID), nil)
	assert.Equal(t, http.StatusForbidden, res.StatusCode)

	// antrean: karyawan tidak punya antrean; manajer IT melihat, manajer CS tidak
	res, _ = rina.do("GET", "/api/v1/leave/approvals", nil)
	assert.Equal(t, http.StatusForbidden, res.StatusCode)
	res, out = bayu.do("GET", "/api/v1/leave/approvals", nil)
	require.Equal(t, 200, res.StatusCode)
	assert.Len(t, out["data"], 1)
	res, out = e.login("citra@test.id").do("GET", "/api/v1/leave/approvals", nil)
	require.Equal(t, 200, res.StatusCode)
	assert.Empty(t, out["data"])
	res, out = bayu.do("GET", "/api/v1/leave/approvals/count", nil)
	require.Equal(t, 200, res.StatusCode)
	assert.EqualValues(t, 1, data(out)["pending"])

	// tahap 1: HR tidak boleh melompat; manajer menyetujui
	res, _ = hr.do("POST", path+"/approve", obj{})
	assert.Equal(t, http.StatusForbidden, res.StatusCode)
	res, out = bayu.do("POST", path+"/approve", obj{"note": "Silakan"})
	require.Equal(t, 200, res.StatusCode, "%v", out)
	assert.Equal(t, "pending_hr", data(out)["status"])
	res, _ = bayu.do("POST", path+"/approve", nil)
	assert.Equal(t, http.StatusForbidden, res.StatusCode, "atasan tidak bisa menyetujui tahap HR")

	// tahap 2 dan saldo
	res, out = hr.do("POST", path+"/approve", nil)
	require.Equal(t, 200, res.StatusCode, "%v", out)
	assert.Equal(t, "approved", data(out)["status"])
	res, _ = hr.do("POST", path+"/approve", nil)
	assert.Equal(t, http.StatusConflict, res.StatusCode)

	res, out = rina.do("GET", fmt.Sprintf("/api/v1/leave/balances?year=%d", start.Year()), nil)
	require.Equal(t, 200, res.StatusCode)
	var used string
	for _, b := range out["data"].([]any) {
		bm := b.(obj)
		if bm["leave_type"].(obj)["code"] == "CT" {
			used = bm["used"].(string)
			assert.Equal(t, "9", bm["available"])
		}
	}
	assert.Equal(t, "3", used)

	// kalender menampilkan pengajuan yang disetujui
	res, out = e.login("dimas@test.id").do("GET", "/api/v1/leave/calendar?month="+start.Format("2006-01"), nil)
	require.Equal(t, 200, res.StatusCode)
	items := data(out)["items"].([]any)
	require.Len(t, items, 1)
	assert.Equal(t, "Rina Karyawan", items[0].(obj)["employee_name"])
	assert.NotContains(t, fmt.Sprint(items[0]), "keluarga", "alasan tidak boleh bocor ke kalender")

	// pembatalan: hanya pemilik; saldo kembali
	res, _ = bayu.do("POST", path+"/cancel", nil)
	assert.Equal(t, http.StatusForbidden, res.StatusCode)
	res, out = rina.do("POST", path+"/cancel", nil)
	require.Equal(t, 200, res.StatusCode, "%v", out)
	assert.Equal(t, "cancelled", data(out)["status"])
	bals, err := e.w.Store.R().Balances.Ensure(e.w.EmpIT1.ID, ct.ID, start.Year(), decimal.NewFromInt(12))
	require.NoError(t, err)
	assert.True(t, bals.Used.IsZero())
}

func TestLeaveRejectNeedsReasonOverHTTP(t *testing.T) {
	e := newEnv(t)
	ct, _ := e.withLeaveTypes()
	start := monday3Weeks()
	rina, bayu := e.login("rina@test.id"), e.login("bayu@test.id")
	_, out := rina.postForm("/api/v1/leave/requests", map[string]string{"leave_type_id": fmt.Sprint(ct.ID),
		"start_date": ymd(start), "end_date": ymd(start), "reason": "Urusan keluarga"}, nil, "")
	path := fmt.Sprintf("/api/v1/leave/requests/%d", num(data(out)["id"]))

	res, _ := bayu.do("POST", path+"/reject", obj{"note": ""})
	assert.Equal(t, http.StatusUnprocessableEntity, res.StatusCode)
	res, out = bayu.do("POST", path+"/reject", obj{"note": "Bentrok jadwal"})
	require.Equal(t, 200, res.StatusCode, "%v", out)
	assert.Equal(t, "rejected", data(out)["status"])
}

func TestLeaveAttachmentOverHTTP(t *testing.T) {
	e := newEnv(t)
	_, cs := e.withLeaveTypes()
	start := monday3Weeks()
	rina := e.login("rina@test.id")
	fields := map[string]string{"leave_type_id": fmt.Sprint(cs.ID), "start_date": ymd(start), "end_date": ymd(start.AddDate(0, 0, 1)), "reason": "Demam tinggi"}

	// sakit > 1 hari tanpa lampiran ditolak; berkas bukan PDF/JPG/PNG ditolak
	res, out := rina.postForm("/api/v1/leave/requests", fields, nil, "")
	assert.Equal(t, http.StatusUnprocessableEntity, res.StatusCode)
	assert.Contains(t, out["error"].(obj)["fields"], "attachment")
	res, _ = rina.postForm("/api/v1/leave/requests", fields, []byte("MZ ini bukan dokumen"), "surat.pdf")
	assert.Equal(t, http.StatusUnprocessableEntity, res.StatusCode)

	res, out = rina.postForm("/api/v1/leave/requests", fields, []byte("%PDF-1.4\n1 0 obj\n<<>>\nendobj\n"), "surat dokter.pdf")
	require.Equal(t, http.StatusCreated, res.StatusCode, "%v", out)
	id := num(data(out)["id"])
	assert.Equal(t, true, data(out)["has_attachment"])
	assert.NotContains(t, fmt.Sprint(out), "attachment_path", "path penyimpanan tidak boleh bocor")

	path := fmt.Sprintf("/api/v1/leave/requests/%d/attachment", id)
	for _, who := range []string{"rina@test.id", "bayu@test.id", "hr@test.id"} {
		r := e.login(who).raw("GET", path+"?download=1", nil, "", nil)
		body, _ := io.ReadAll(r.Body)
		_ = r.Body.Close()
		require.Equal(t, 200, r.StatusCode, who)
		assert.Contains(t, string(body), "%PDF")
		assert.Contains(t, r.Header.Get("Content-Disposition"), "attachment")
		assert.Equal(t, "private, no-store", r.Header.Get("Cache-Control"))
		assert.Contains(t, r.Header.Get("Content-Security-Policy"), "sandbox")
	}
	for _, who := range []string{"fajar@test.id", "citra@test.id"} {
		r := e.login(who).raw("GET", path, nil, "", nil)
		_ = r.Body.Close()
		assert.Equal(t, http.StatusNotFound, r.StatusCode, who)
	}
}

func TestLeaveSubmitRejectsBadInput(t *testing.T) {
	e := newEnv(t)
	ct, _ := e.withLeaveTypes()
	rina := e.login("rina@test.id")
	start := monday3Weeks()
	cases := []struct {
		name   string
		fields map[string]string
		field  string
	}{
		{"tanggal bukan format", map[string]string{"leave_type_id": fmt.Sprint(ct.ID), "start_date": "besok", "end_date": ymd(start), "reason": "x"}, "start_date"},
		{"pengganti bukan angka", map[string]string{"leave_type_id": fmt.Sprint(ct.ID), "start_date": ymd(start), "end_date": ymd(start), "reason": "x", "substitute_id": "abc"}, "substitute_id"},
		{"jenis kosong", map[string]string{"start_date": ymd(start), "end_date": ymd(start), "reason": "x"}, "leave_type_id"},
		{"alasan kosong", map[string]string{"leave_type_id": fmt.Sprint(ct.ID), "start_date": ymd(start), "end_date": ymd(start)}, "reason"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, out := rina.postForm("/api/v1/leave/requests", tc.fields, nil, "")
			assert.Equal(t, http.StatusUnprocessableEntity, res.StatusCode)
			assert.Contains(t, out["error"].(obj)["fields"], tc.field)
		})
	}

	// CSRF: tanpa header X-Requested-With ditolak
	res := rina.raw("POST", "/api/v1/leave/requests/1/cancel", strings.NewReader(""), "", map[string]string{"X-Requested-With": ""})
	_ = res.Body.Close()
	assert.Equal(t, http.StatusForbidden, res.StatusCode)
}
