package handler

import (
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/retechno/sikerja/backend/internal/apperr"
	"github.com/retechno/sikerja/backend/internal/repository"
	"github.com/retechno/sikerja/backend/internal/service"
)

// ---------- jenis cuti & hari libur ----------

func (h *H) listLeaveTypes(c *gin.Context) {
	v, err := h.LeaveTypes.List(actor(c))
	if err != nil {
		h.fail(c, err)
		return
	}
	ok(c, v)
}

func (h *H) createLeaveType(c *gin.Context) {
	var in service.LeaveTypeInput
	if !h.bind(c, &in) {
		return
	}
	v, err := h.LeaveTypes.Create(actor(c), in)
	if err != nil {
		h.fail(c, err)
		return
	}
	created(c, v)
}

func (h *H) updateLeaveType(c *gin.Context) {
	id, good := h.idParam(c, "id")
	if !good {
		return
	}
	var in service.LeaveTypeInput
	if !h.bind(c, &in) {
		return
	}
	v, err := h.LeaveTypes.Update(actor(c), id, in)
	if err != nil {
		h.fail(c, err)
		return
	}
	ok(c, v)
}

func (h *H) deleteLeaveType(c *gin.Context) {
	id, good := h.idParam(c, "id")
	if !good {
		return
	}
	if err := h.LeaveTypes.Delete(actor(c), id); err != nil {
		h.fail(c, err)
		return
	}
	noContent(c)
}

func (h *H) listHolidays(c *gin.Context) {
	v, err := h.Holidays.List(qInt(c, "year", 0))
	if err != nil {
		h.fail(c, err)
		return
	}
	ok(c, v)
}

type holidayBody struct {
	Date string `json:"date"`
	Name string `json:"name"`
	Type string `json:"type"`
}

func (b holidayBody) input() (service.HolidayInput, error) {
	f := map[string]string{}
	d := parseDateField("date", b.Date, f)
	if len(f) > 0 {
		return service.HolidayInput{}, apperr.Validation("Data hari libur tidak valid", f)
	}
	return service.HolidayInput{Date: d, Name: b.Name, Type: b.Type}, nil
}

func (h *H) createHoliday(c *gin.Context) {
	var b holidayBody
	if !h.bind(c, &b) {
		return
	}
	in, err := b.input()
	if err != nil {
		h.fail(c, err)
		return
	}
	v, err := h.Holidays.Create(actor(c), in)
	if err != nil {
		h.fail(c, err)
		return
	}
	created(c, v)
}

func (h *H) updateHoliday(c *gin.Context) {
	id, good := h.idParam(c, "id")
	if !good {
		return
	}
	var b holidayBody
	if !h.bind(c, &b) {
		return
	}
	in, err := b.input()
	if err != nil {
		h.fail(c, err)
		return
	}
	v, err := h.Holidays.Update(actor(c), id, in)
	if err != nil {
		h.fail(c, err)
		return
	}
	ok(c, v)
}

func (h *H) deleteHoliday(c *gin.Context) {
	id, good := h.idParam(c, "id")
	if !good {
		return
	}
	if err := h.Holidays.Delete(actor(c), id); err != nil {
		h.fail(c, err)
		return
	}
	noContent(c)
}

func (h *H) importHolidays(c *gin.Context) {
	var b struct {
		Year int `json:"year"`
	}
	if !h.bind(c, &b) {
		return
	}
	n, err := h.Holidays.ImportOfficial(actor(c), b.Year)
	if err != nil {
		h.fail(c, err)
		return
	}
	ok(c, gin.H{"added": n})
}

func (h *H) previewHolidayImport(c *gin.Context) {
	var b struct {
		Year int `json:"year"`
	}
	if !h.bind(c, &b) {
		return
	}
	v, err := h.Holidays.PreviewImport(c.Request.Context(), actor(c), b.Year)
	if err != nil {
		h.fail(c, err)
		return
	}
	ok(c, v)
}

func (h *H) confirmHolidayImport(c *gin.Context) {
	var b struct {
		Items []holidayBody `json:"items"`
	}
	if !h.bind(c, &b) {
		return
	}
	items := make([]service.HolidayInput, 0, len(b.Items))
	for _, it := range b.Items {
		in, err := it.input()
		if err != nil {
			h.fail(c, err)
			return
		}
		items = append(items, in)
	}
	added, skipped, err := h.Holidays.ImportSelected(actor(c), items)
	if err != nil {
		h.fail(c, err)
		return
	}
	ok(c, gin.H{"added": added, "skipped": skipped})
}

// ---------- notifikasi: pengaturan ----------

func (h *H) notificationSettings(c *gin.Context) {
	v, err := h.NotifyAdmin.Settings(actor(c))
	if err != nil {
		h.fail(c, err)
		return
	}
	ok(c, v)
}

func (h *H) setNotificationSetting(c *gin.Context) {
	var b struct {
		Key     string `json:"key"`
		Enabled bool   `json:"enabled"`
	}
	if !h.bind(c, &b) {
		return
	}
	if err := h.NotifyAdmin.SetSetting(actor(c), b.Key, b.Enabled); err != nil {
		h.fail(c, err)
		return
	}
	v, err := h.NotifyAdmin.Settings(actor(c))
	if err != nil {
		h.fail(c, err)
		return
	}
	ok(c, v)
}

// ---------- saldo & pratinjau ----------

func (h *H) leaveBalances(c *gin.Context) {
	v, err := h.Leaves.Balances(actor(c), qUint(c, "employee_id"), qInt(c, "year", 0))
	if err != nil {
		h.fail(c, err)
		return
	}
	ok(c, v)
}

func (h *H) leavePreview(c *gin.Context) {
	f := map[string]string{}
	start := parseDateField("start_date", c.Query("start_date"), f)
	end := parseDateField("end_date", c.Query("end_date"), f)
	if len(f) > 0 {
		h.fail(c, apperr.Validation("Tanggal tidak valid", f))
		return
	}
	half, _ := strconv.ParseBool(c.Query("half_day"))
	sp := service.PartialSpec{Half: half, Session: c.Query("half_session"), Start: c.Query("start_time"), End: c.Query("end_time")}
	v, err := h.Leaves.Preview(actor(c), qUint(c, "type_id"), start, end, sp)
	if err != nil {
		h.fail(c, err)
		return
	}
	ok(c, v)
}

func (h *H) leaveSubstitutes(c *gin.Context) {
	v, err := h.Leaves.Substitutes(actor(c))
	if err != nil {
		h.fail(c, err)
		return
	}
	ok(c, v)
}

func (h *H) leaveSummary(c *gin.Context) {
	v, err := h.Leaves.Summary(actor(c))
	if err != nil {
		h.fail(c, err)
		return
	}
	ok(c, v)
}

// ---------- pengajuan ----------

func (h *H) submitLeave(c *gin.Context) {
	f := map[string]string{}
	typeID, _ := strconv.ParseUint(c.PostForm("leave_type_id"), 10, 64)
	in := service.LeaveInput{
		LeaveTypeID: typeID,
		HalfSession: c.PostForm("half_session"),
		StartTime:   c.PostForm("start_time"),
		EndTime:     c.PostForm("end_time"),
		Reason:      c.PostForm("reason"),
	}
	in.StartDate = parseDateField("start_date", c.PostForm("start_date"), f)
	in.EndDate = parseDateField("end_date", c.PostForm("end_date"), f)
	in.HalfDay, _ = strconv.ParseBool(c.PostForm("half_day"))
	if s := c.PostForm("substitute_id"); s != "" {
		id, err := strconv.ParseUint(s, 10, 64)
		if err != nil {
			f["substitute_id"] = "Pengganti tidak valid"
		} else {
			in.SubstituteID = &id
		}
	}
	if len(f) > 0 {
		h.fail(c, apperr.Validation("Data pengajuan tidak valid", f))
		return
	}
	if fh, err := c.FormFile("attachment"); err == nil {
		file, err := fh.Open()
		if err != nil {
			h.fail(c, err)
			return
		}
		defer func() { _ = file.Close() }()
		in.File, in.Filename = file, fh.Filename
	}
	v, err := h.Leaves.Submit(c.Request.Context(), actor(c), in)
	if err != nil {
		h.fail(c, err)
		return
	}
	created(c, v)
}

func (h *H) listLeaves(c *gin.Context) {
	page, per := qInt(c, "page", 1), qInt(c, "per_page", 20)
	items, total, err := h.Leaves.List(actor(c), service.LeaveListFilter{
		EmployeeID: qUint(c, "employee_id"), Status: c.Query("status"), Year: qInt(c, "year", 0),
		Page: repository.Page{Page: page, PerPage: per},
	})
	if err != nil {
		h.fail(c, err)
		return
	}
	okMeta(c, items, pageMeta{Page: page, PerPage: per, Total: total})
}

func (h *H) getLeave(c *gin.Context) {
	id, good := h.idParam(c, "id")
	if !good {
		return
	}
	v, err := h.Leaves.Get(actor(c), id)
	if err != nil {
		h.fail(c, err)
		return
	}
	ok(c, v)
}

func (h *H) cancelLeave(c *gin.Context) {
	id, good := h.idParam(c, "id")
	if !good {
		return
	}
	v, err := h.Leaves.Cancel(c.Request.Context(), actor(c), id)
	if err != nil {
		h.fail(c, err)
		return
	}
	ok(c, v)
}

func (h *H) leaveApprovals(c *gin.Context) {
	v, err := h.Leaves.Queue(actor(c), c.DefaultQuery("tab", "pending"))
	if err != nil {
		h.fail(c, err)
		return
	}
	ok(c, v)
}

func (h *H) leaveApprovalCount(c *gin.Context) {
	n, err := h.Leaves.PendingCount(actor(c))
	if err != nil {
		h.fail(c, err)
		return
	}
	ok(c, gin.H{"pending": n})
}

// leaveDecision membangun handler setuju/tolak.
func (h *H) leaveDecision(approve bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, good := h.idParam(c, "id")
		if !good {
			return
		}
		var b struct {
			Note string `json:"note"`
		}
		if c.Request.ContentLength != 0 && !h.bind(c, &b) {
			return
		}
		var v *service.LeaveView
		var err error
		if approve {
			v, err = h.Leaves.Approve(c.Request.Context(), actor(c), id, b.Note)
		} else {
			v, err = h.Leaves.Reject(c.Request.Context(), actor(c), id, b.Note)
		}
		if err != nil {
			h.fail(c, err)
			return
		}
		ok(c, v)
	}
}

func (h *H) leaveAttachment(c *gin.Context) {
	id, good := h.idParam(c, "id")
	if !good {
		return
	}
	f, name, err := h.Leaves.OpenAttachment(actor(c), id)
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

func (h *H) leaveCalendar(c *gin.Context) {
	month := time.Now()
	if m := c.Query("month"); m != "" {
		t, err := time.Parse("2006-01", m)
		if err != nil {
			h.fail(c, apperr.Validation("Bulan tidak valid", map[string]string{"month": "Format bulan harus YYYY-MM"}))
			return
		}
		month = t
	}
	pending, _ := strconv.ParseBool(c.Query("include_pending"))
	v, err := h.Leaves.Calendar(actor(c), month, qUint(c, "department_id"), pending)
	if err != nil {
		h.fail(c, err)
		return
	}
	ok(c, v)
}

func (h *H) leavePolicy(c *gin.Context) {
	v, err := h.LeaveTypes.Policy(actor(c))
	if err != nil {
		h.fail(c, err)
		return
	}
	ok(c, v)
}

func (h *H) setLeavePolicy(c *gin.Context) {
	var b service.LeavePolicy
	if !h.bind(c, &b) {
		return
	}
	v, err := h.LeaveTypes.SetPolicy(actor(c), b)
	if err != nil {
		h.fail(c, err)
		return
	}
	ok(c, v)
}
