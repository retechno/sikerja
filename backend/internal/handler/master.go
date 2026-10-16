package handler

import (
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/retechno/sikerja/backend/internal/apperr"
	"github.com/retechno/sikerja/backend/internal/model"
	"github.com/retechno/sikerja/backend/internal/repository"
	"github.com/retechno/sikerja/backend/internal/service"
)

// ---------- departemen & jabatan ----------

func (h *H) listDepartments(c *gin.Context) {
	v, err := h.Master.Departments()
	if err != nil {
		h.fail(c, err)
		return
	}
	ok(c, v)
}

func (h *H) createDepartment(c *gin.Context) {
	var in service.DepartmentInput
	if !h.bind(c, &in) {
		return
	}
	v, err := h.Master.CreateDepartment(actor(c), in)
	if err != nil {
		h.fail(c, err)
		return
	}
	created(c, v)
}

func (h *H) updateDepartment(c *gin.Context) {
	id, good := h.idParam(c, "id")
	if !good {
		return
	}
	var in service.DepartmentInput
	if !h.bind(c, &in) {
		return
	}
	v, err := h.Master.UpdateDepartment(actor(c), id, in)
	if err != nil {
		h.fail(c, err)
		return
	}
	ok(c, v)
}

func (h *H) deleteDepartment(c *gin.Context) {
	id, good := h.idParam(c, "id")
	if !good {
		return
	}
	if err := h.Master.DeleteDepartment(actor(c), id); err != nil {
		h.fail(c, err)
		return
	}
	noContent(c)
}

func (h *H) listPositions(c *gin.Context) {
	v, err := h.Master.Positions(qUint(c, "department_id"))
	if err != nil {
		h.fail(c, err)
		return
	}
	ok(c, v)
}

func (h *H) createPosition(c *gin.Context) {
	var in service.PositionInput
	if !h.bind(c, &in) {
		return
	}
	v, err := h.Master.CreatePosition(actor(c), in)
	if err != nil {
		h.fail(c, err)
		return
	}
	created(c, v)
}

func (h *H) updatePosition(c *gin.Context) {
	id, good := h.idParam(c, "id")
	if !good {
		return
	}
	var in service.PositionInput
	if !h.bind(c, &in) {
		return
	}
	v, err := h.Master.UpdatePosition(actor(c), id, in)
	if err != nil {
		h.fail(c, err)
		return
	}
	ok(c, v)
}

func (h *H) deletePosition(c *gin.Context) {
	id, good := h.idParam(c, "id")
	if !good {
		return
	}
	if err := h.Master.DeletePosition(actor(c), id); err != nil {
		h.fail(c, err)
		return
	}
	noContent(c)
}

// ---------- karyawan ----------

type employeeReq struct {
	service.EmployeeInput
	JoinDateStr string `json:"join_date"`
}

func (r employeeReq) input() (service.EmployeeInput, error) {
	in := r.EmployeeInput
	f := map[string]string{}
	in.JoinDate = parseDateField("join_date", r.JoinDateStr, f)
	if len(f) > 0 {
		return in, apperr.Validation("Data karyawan tidak valid", f)
	}
	return in, nil
}

func (h *H) listEmployees(c *gin.Context) {
	f := repository.EmployeeFilter{Search: c.Query("q"), DepartmentID: qUint(c, "department_id"), Status: c.Query("status"),
		Page: repository.Page{Page: qInt(c, "page", 1), PerPage: qInt(c, "per_page", 20)}}
	v, total, err := h.Employees.List(actor(c), f)
	if err != nil {
		h.fail(c, err)
		return
	}
	okMeta(c, v, pageMeta{Page: f.Page.Page, PerPage: f.Page.PerPage, Total: total})
}

func (h *H) getEmployee(c *gin.Context) {
	id, good := h.idParam(c, "id")
	if !good {
		return
	}
	v, err := h.Employees.Get(actor(c), id)
	if err != nil {
		h.fail(c, err)
		return
	}
	ok(c, v)
}

func (h *H) createEmployee(c *gin.Context) {
	var req employeeReq
	if !h.bind(c, &req) {
		return
	}
	in, err := req.input()
	if err != nil {
		h.fail(c, err)
		return
	}
	v, temp, err := h.Employees.Create(actor(c), in)
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": v, "meta": gin.H{"temp_password": temp}})
}

func (h *H) updateEmployee(c *gin.Context) {
	id, good := h.idParam(c, "id")
	if !good {
		return
	}
	var req employeeReq
	if !h.bind(c, &req) {
		return
	}
	in, err := req.input()
	if err != nil {
		h.fail(c, err)
		return
	}
	v, err := h.Employees.Update(actor(c), id, in)
	if err != nil {
		h.fail(c, err)
		return
	}
	ok(c, v)
}

func (h *H) deleteEmployee(c *gin.Context) {
	id, good := h.idParam(c, "id")
	if !good {
		return
	}
	if err := h.Employees.Delete(actor(c), id); err != nil {
		h.fail(c, err)
		return
	}
	noContent(c)
}

func (h *H) importEmployees(c *gin.Context) {
	f, _, err := c.Request.FormFile("file")
	if err != nil {
		h.fail(c, apperr.Validation("Berkas wajib diunggah", map[string]string{"file": "Pilih berkas .xlsx"}))
		return
	}
	defer f.Close()
	res, err := h.Employees.ImportEmployees(actor(c), f)
	if err != nil {
		h.fail(c, err)
		return
	}
	if len(res.Errors) > 0 { // tidak ada yang tersimpan
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"error": gin.H{"code": "import_invalid", "message": "Berkas memiliki kesalahan; tidak ada data yang diimpor", "fields": nil},
			"data":  res,
		})
		return
	}
	ok(c, res)
}

func (h *H) importTemplate(c *gin.Context) {
	b, err := h.Employees.ImportTemplate(actor(c))
	if err != nil {
		h.fail(c, err)
		return
	}
	c.Header("Content-Disposition", `attachment; filename="template-import-karyawan.xlsx"`)
	c.Data(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", b)
}

func (h *H) employeePhoto(c *gin.Context) {
	id, good := h.idParam(c, "id")
	if !good {
		return
	}
	f, err := h.Employees.OpenPhoto(id)
	if err != nil {
		h.fail(c, err)
		return
	}
	defer f.Close()
	c.Header("Cache-Control", "private, max-age=300")
	c.Header("Content-Type", mime.TypeByExtension(strings.ToLower(filepath.Ext(f.Name()))))
	c.Status(http.StatusOK)
	_, _ = io.Copy(c.Writer, f)
}

// ---------- periode ----------

type periodReq struct {
	service.PeriodInput
	StartStr string `json:"start_date"`
	EndStr   string `json:"end_date"`
}

func (r periodReq) input() (service.PeriodInput, error) {
	in := r.PeriodInput
	f := map[string]string{}
	in.StartDate = parseDateField("start_date", r.StartStr, f)
	in.EndDate = parseDateField("end_date", r.EndStr, f)
	if len(f) > 0 {
		return in, apperr.Validation("Data periode tidak valid", f)
	}
	return in, nil
}

func (h *H) listPeriods(c *gin.Context) {
	v, err := h.Periods.List(actor(c))
	if err != nil {
		h.fail(c, err)
		return
	}
	ok(c, v)
}

func (h *H) getPeriod(c *gin.Context) {
	id, good := h.idParam(c, "id")
	if !good {
		return
	}
	v, err := h.Periods.Get(actor(c), id)
	if err != nil {
		h.fail(c, err)
		return
	}
	ok(c, v)
}

func (h *H) createPeriod(c *gin.Context) {
	var req periodReq
	if !h.bind(c, &req) {
		return
	}
	in, err := req.input()
	if err != nil {
		h.fail(c, err)
		return
	}
	v, err := h.Periods.Create(actor(c), in)
	if err != nil {
		h.fail(c, err)
		return
	}
	created(c, v)
}

func (h *H) updatePeriod(c *gin.Context) {
	id, good := h.idParam(c, "id")
	if !good {
		return
	}
	var req periodReq
	if !h.bind(c, &req) {
		return
	}
	in, err := req.input()
	if err != nil {
		h.fail(c, err)
		return
	}
	v, err := h.Periods.Update(actor(c), id, in)
	if err != nil {
		h.fail(c, err)
		return
	}
	ok(c, v)
}

func (h *H) periodAction(fn func(service.Actor, uint64) (*model.AppraisalPeriod, error)) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, good := h.idParam(c, "id")
		if !good {
			return
		}
		v, err := fn(actor(c), id)
		if err != nil {
			h.fail(c, err)
			return
		}
		ok(c, v)
	}
}

// ---------- template KPI ----------

func (h *H) listTemplates(c *gin.Context) {
	v, err := h.Templates.List(actor(c))
	if err != nil {
		h.fail(c, err)
		return
	}
	ok(c, v)
}

func (h *H) getTemplate(c *gin.Context) {
	id, good := h.idParam(c, "id")
	if !good {
		return
	}
	v, err := h.Templates.Get(actor(c), id)
	if err != nil {
		h.fail(c, err)
		return
	}
	ok(c, v)
}

func (h *H) createTemplate(c *gin.Context) {
	var req struct {
		DepartmentID uint64 `json:"department_id"`
		Name         string `json:"name"`
	}
	if !h.bind(c, &req) {
		return
	}
	v, err := h.Templates.Create(actor(c), req.DepartmentID, req.Name)
	if err != nil {
		h.fail(c, err)
		return
	}
	created(c, v)
}

func (h *H) saveTemplate(c *gin.Context) {
	id, good := h.idParam(c, "id")
	if !good {
		return
	}
	var in service.TemplateIn
	if !h.bind(c, &in) {
		return
	}
	v, err := h.Templates.Save(actor(c), id, in)
	if err != nil {
		h.fail(c, err)
		return
	}
	ok(c, v)
}

func (h *H) listAspects(c *gin.Context) {
	v, err := h.Appraisal.Aspects()
	if err != nil {
		h.fail(c, err)
		return
	}
	ok(c, v)
}

// ---------- audit & notifikasi ----------

func (h *H) listAudit(c *gin.Context) {
	f := repository.AuditFilter{Action: c.Query("action"), SubjectType: c.Query("subject_type"), SubjectID: qUint(c, "subject_id"),
		Page: repository.Page{Page: qInt(c, "page", 1), PerPage: qInt(c, "per_page", 50)}}
	v, total, err := h.Audit.List(actor(c), f)
	if err != nil {
		h.fail(c, err)
		return
	}
	okMeta(c, v, pageMeta{Page: f.Page.Page, PerPage: f.Page.PerPage, Total: total})
}

func (h *H) listNotifications(c *gin.Context) {
	v, err := h.Notifs.List(actor(c))
	if err != nil {
		h.fail(c, err)
		return
	}
	ok(c, v)
}

func (h *H) readNotification(c *gin.Context) {
	id, good := h.idParam(c, "id")
	if !good {
		return
	}
	if err := h.Notifs.MarkRead(actor(c), id); err != nil {
		h.fail(c, err)
		return
	}
	noContent(c)
}

func (h *H) readAllNotifications(c *gin.Context) {
	if err := h.Notifs.MarkAllRead(actor(c)); err != nil {
		h.fail(c, err)
		return
	}
	noContent(c)
}
