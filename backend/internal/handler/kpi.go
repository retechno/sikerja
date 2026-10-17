package handler

import (
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/retechno/sikerja/backend/internal/apperr"
	"github.com/retechno/sikerja/backend/internal/service"
)

// GET /kpi/sheet?employee_id=&period_id=  (default: diri sendiri, periode aktif)
func (h *H) kpiSheet(c *gin.Context) {
	a := actor(c)
	empID := qUint(c, "employee_id")
	if empID == 0 {
		empID = a.EmployeeID
	}
	pid, err := h.Periods.Resolve(a, qUint(c, "period_id"))
	if err != nil {
		h.fail(c, err)
		return
	}
	v, err := h.Kpi.Sheet(a, empID, pid)
	if err != nil {
		h.fail(c, err)
		return
	}
	ok(c, v)
}

func (h *H) saveResult(c *gin.Context) {
	var in service.SaveResultInput
	if !h.bind(c, &in) {
		return
	}
	if in.EmployeeID == 0 {
		in.EmployeeID = actor(c).EmployeeID
	}
	v, err := h.Kpi.SaveResult(actor(c), in)
	if err != nil {
		h.fail(c, err)
		return
	}
	ok(c, v)
}

func (h *H) validateResult(c *gin.Context) {
	id, good := h.idParam(c, "id")
	if !good {
		return
	}
	var req struct {
		Action string `json:"action"`
		Note   string `json:"note"`
	}
	if !h.bind(c, &req) {
		return
	}
	v, err := h.Kpi.Validate(actor(c), id, req.Action, req.Note)
	if err != nil {
		h.fail(c, err)
		return
	}
	ok(c, v)
}

func (h *H) uploadEvidence(c *gin.Context) {
	id, good := h.idParam(c, "id")
	if !good {
		return
	}
	fh, hdr, err := c.Request.FormFile("file")
	if err != nil {
		h.fail(c, apperr.Validation("Berkas wajib diunggah", map[string]string{"file": "Pilih berkas PDF, JPG, atau PNG"}))
		return
	}
	defer func() { _ = fh.Close() }()
	v, err := h.Kpi.SaveEvidence(actor(c), id, hdr.Filename, fh)
	if err != nil {
		h.fail(c, err)
		return
	}
	ok(c, v)
}

// GET /kpi/results/:id/evidence — hak akses diperiksa di service (pemilik, atasan timnya, HR, Direktur).
func (h *H) downloadEvidence(c *gin.Context) {
	id, good := h.idParam(c, "id")
	if !good {
		return
	}
	f, name, err := h.Kpi.OpenEvidence(actor(c), id)
	if err != nil {
		h.fail(c, err)
		return
	}
	defer func() { _ = f.Close() }()
	ctype := mime.TypeByExtension(strings.ToLower(filepath.Ext(f.Name())))
	if ctype == "" {
		ctype = "application/octet-stream"
	}
	disp := "inline"
	if c.Query("download") == "1" {
		disp = "attachment"
	}
	c.Header("Content-Type", ctype)
	c.Header("Content-Disposition", mime.FormatMediaType(disp, map[string]string{"filename": name}))
	c.Header("Cache-Control", "private, no-store")
	c.Header("Content-Security-Policy", "default-src 'none'; sandbox")
	c.Status(http.StatusOK)
	_, _ = io.Copy(c.Writer, f)
}

// ---------- penilaian ----------

func (h *H) getAppraisal(c *gin.Context) {
	pid, good := h.idParam(c, "id")
	if !good {
		return
	}
	eid, good := h.idParam(c, "employee_id")
	if !good {
		return
	}
	v, err := h.Appraisal.Get(actor(c), eid, pid)
	if err != nil {
		h.fail(c, err)
		return
	}
	ok(c, v)
}

func (h *H) saveSelf(c *gin.Context) {
	pid, good := h.idParam(c, "id")
	if !good {
		return
	}
	eid, good := h.idParam(c, "employee_id")
	if !good {
		return
	}
	var in service.SelfInput
	if !h.bind(c, &in) {
		return
	}
	v, err := h.Appraisal.SaveSelf(actor(c), eid, pid, in)
	if err != nil {
		h.fail(c, err)
		return
	}
	ok(c, v)
}

func (h *H) saveManager(c *gin.Context) {
	pid, good := h.idParam(c, "id")
	if !good {
		return
	}
	eid, good := h.idParam(c, "employee_id")
	if !good {
		return
	}
	var in service.ManagerInput
	if !h.bind(c, &in) {
		return
	}
	v, err := h.Appraisal.SaveManager(actor(c), eid, pid, in)
	if err != nil {
		h.fail(c, err)
		return
	}
	ok(c, v)
}

func (h *H) finalize(c *gin.Context) {
	pid, good := h.idParam(c, "id")
	if !good {
		return
	}
	eid, good := h.idParam(c, "employee_id")
	if !good {
		return
	}
	v, err := h.Appraisal.Finalize(actor(c), eid, pid)
	if err != nil {
		h.fail(c, err)
		return
	}
	ok(c, v)
}

func (h *H) team(c *gin.Context) {
	pid, good := h.idParam(c, "id")
	if !good {
		return
	}
	v, err := h.Appraisal.Team(actor(c), pid)
	if err != nil {
		h.fail(c, err)
		return
	}
	ok(c, v)
}

// ---------- dasbor & laporan ----------

func (h *H) dashEmployee(c *gin.Context) {
	v, err := h.Dashboard.Employee(actor(c), qUint(c, "period_id"))
	if err != nil {
		h.fail(c, err)
		return
	}
	ok(c, v)
}

func (h *H) dashManager(c *gin.Context) {
	v, err := h.Dashboard.Manager(actor(c), qUint(c, "period_id"))
	if err != nil {
		h.fail(c, err)
		return
	}
	ok(c, v)
}

func (h *H) dashDirector(c *gin.Context) {
	v, err := h.Dashboard.Director(actor(c), qUint(c, "period_id"))
	if err != nil {
		h.fail(c, err)
		return
	}
	ok(c, v)
}

func (h *H) reportScores(c *gin.Context) {
	v, err := h.Reports.Build(actor(c), qUint(c, "period_id"), qUint(c, "department_id"))
	if err != nil {
		h.fail(c, err)
		return
	}
	ok(c, v)
}

func (h *H) reportExport(c *gin.Context) {
	a, pid, did := actor(c), qUint(c, "period_id"), qUint(c, "department_id")
	var (
		b     []byte
		name  string
		ctype string
		err   error
	)
	switch c.DefaultQuery("format", "xlsx") {
	case "xlsx":
		b, name, err = h.Reports.ExportXLSX(a, pid, did)
		ctype = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	case "pdf":
		b, name, err = h.Reports.ExportPDF(a, pid, did)
		ctype = "application/pdf"
	default:
		h.fail(c, apperr.Validation("Format tidak didukung", map[string]string{"format": "Gunakan xlsx atau pdf"}))
		return
	}
	if err != nil {
		h.fail(c, err)
		return
	}
	c.Header("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": name}))
	c.Data(http.StatusOK, ctype, b)
}
