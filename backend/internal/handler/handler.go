// Package handler berisi handler HTTP tipis: bind input → panggil service → format respons.
package handler

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"

	"github.com/retechno/sikerja/backend/internal/apperr"
	"github.com/retechno/sikerja/backend/internal/config"
	"github.com/retechno/sikerja/backend/internal/middleware"
	"github.com/retechno/sikerja/backend/internal/service"
)

// Deps adalah semua service yang dibutuhkan handler.
type Deps struct {
	Cfg       config.Config
	Log       zerolog.Logger
	Auth      *service.AuthService
	Employees *service.EmployeeService
	Master    *service.MasterService
	Periods   *service.PeriodService
	Templates *service.TemplateService
	Kpi       *service.KpiService
	Appraisal service.AppraisalService
	Dashboard *service.DashboardService
	Reports   *service.ReportService
	Notifs    *service.NotificationService
	Audit     *service.AuditService

	Leaves      *service.LeaveService
	LeaveTypes  *service.LeaveTypeService
	Holidays    *service.HolidayService
	WorkingDays *service.WorkingDayService
	NotifyAdmin *service.NotifyService
}

type H struct{ Deps }

type pageMeta struct {
	Page    int   `json:"page"`
	PerPage int   `json:"per_page"`
	Total   int64 `json:"total"`
}

func ok(c *gin.Context, data any) { c.JSON(http.StatusOK, gin.H{"data": data}) }

func okMeta(c *gin.Context, data, meta any) { c.JSON(http.StatusOK, gin.H{"data": data, "meta": meta}) }

func created(c *gin.Context, data any) { c.JSON(http.StatusCreated, gin.H{"data": data}) }

func noContent(c *gin.Context) { c.Status(http.StatusNoContent) }

// fail menerjemahkan error service ke respons; error tak dikenal menjadi 500 generik (detail hanya di log).
func (h *H) fail(c *gin.Context, err error) {
	if ae, isApp := apperr.As(err); isApp {
		middleware.WriteError(c, ae)
		return
	}
	var tooBig *http.MaxBytesError
	if errors.As(err, &tooBig) {
		middleware.WriteError(c, apperr.New(http.StatusRequestEntityTooLarge, "payload_too_large", "Ukuran permintaan terlalu besar"))
		return
	}
	h.Log.Error().Err(err).Str("path", c.Request.URL.Path).Msg("kesalahan tak terduga")
	middleware.WriteError(c, apperr.New(http.StatusInternalServerError, "internal_error", "Terjadi kesalahan pada server"))
}

// bind membaca JSON; mengembalikan false (dan sudah menulis respons) bila gagal.
func (h *H) bind(c *gin.Context, v any) bool {
	if err := c.ShouldBindJSON(v); err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			h.fail(c, err)
			return false
		}
		middleware.WriteError(c, apperr.BadRequest("Format permintaan tidak valid"))
		return false
	}
	return true
}

func (h *H) idParam(c *gin.Context, name string) (uint64, bool) {
	id, err := strconv.ParseUint(c.Param(name), 10, 64)
	if err != nil || id == 0 {
		middleware.WriteError(c, apperr.NotFound("Data tidak ditemukan"))
		return 0, false
	}
	return id, true
}

// qUint membaca query integer; kosong/tidak valid → 0.
func qUint(c *gin.Context, name string) uint64 {
	v, _ := strconv.ParseUint(c.Query(name), 10, 64)
	return v
}

func qInt(c *gin.Context, name string, def int) int {
	if v, err := strconv.Atoi(c.Query(name)); err == nil && v > 0 {
		return v
	}
	return def
}

func parseDateField(field, s string, fields map[string]string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		fields[field] = "Format tanggal harus YYYY-MM-DD"
	}
	return t
}

func actor(c *gin.Context) service.Actor { return middleware.ActorFrom(c) }
