package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/retechno/sikerja/backend/internal/middleware"
	"github.com/retechno/sikerja/backend/internal/model"
)

const (
	apiBodyLimit    = 1 << 20 // 1 MB untuk JSON
	uploadBodyLimit = 6 << 20 // berkas maks. 5 MB + overhead multipart
)

// NewRouter merakit semua route di bawah /api/v1.
func NewRouter(d Deps) *gin.Engine {
	if d.Cfg.IsProd() {
		gin.SetMode(gin.ReleaseMode)
	}
	h := &H{Deps: d}
	r := gin.New()
	r.Use(middleware.Recovery(d.Log), middleware.Logger(d.Log), middleware.SecurityHeaders(), middleware.CORS(d.Cfg.CORSOrigins))
	r.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "not_found", "message": "Endpoint tidak ditemukan"}})
	})

	r.GET("/healthz", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })

	api := r.Group("/api/v1", middleware.CSRF(), middleware.BodyLimit(apiBodyLimit))
	upload := middleware.BodyLimit(uploadBodyLimit)

	// ---- publik ----
	au := api.Group("/auth")
	au.POST("/login", h.login)
	au.POST("/refresh", h.refresh)
	au.POST("/logout", h.logout)
	au.POST("/forgot-password", h.forgotPassword)
	au.POST("/reset-password", h.resetPassword)

	// ---- wajib login ----
	in := api.Group("", middleware.Auth(d.Auth))
	in.POST("/auth/change-password", h.changePassword)
	in.GET("/me", h.me)
	in.PUT("/me", h.updateMe)
	in.POST("/me/photo", upload, h.uploadMyPhoto)
	in.GET("/employees/:id/photo", h.employeePhoto)

	in.GET("/notifications", h.listNotifications)
	in.POST("/notifications/read-all", h.readAllNotifications)
	in.POST("/notifications/:id/read", h.readNotification)

	// izin & cuti: hak akses diperiksa di service (pemilik, atasan, HR, Direktur)
	in.GET("/leave-types", h.listLeaveTypes)
	in.GET("/holidays", h.listHolidays)
	in.GET("/leave/balances", h.leaveBalances)
	in.GET("/leave/preview", h.leavePreview)
	in.GET("/leave/summary", h.leaveSummary)
	in.GET("/leave/calendar", h.leaveCalendar)
	in.GET("/leave/requests", h.listLeaves)
	in.POST("/leave/requests", upload, h.submitLeave)
	in.GET("/leave/requests/:id", h.getLeave)
	in.POST("/leave/requests/:id/cancel", h.cancelLeave)
	in.GET("/leave/requests/:id/attachment", h.leaveAttachment)
	in.GET("/leave/approvals", h.leaveApprovals)
	in.GET("/leave/approvals/count", h.leaveApprovalCount)
	in.POST("/leave/requests/:id/approve", h.leaveDecision(true))
	in.POST("/leave/requests/:id/reject", h.leaveDecision(false))

	in.GET("/departments", h.listDepartments)
	in.GET("/positions", h.listPositions)
	in.GET("/periods", h.listPeriods)
	in.GET("/periods/:id", h.getPeriod)
	in.GET("/behavior-aspects", h.listAspects)

	// karyawan: akses data dibatasi di service (diri sendiri / tim / HR / Direktur)
	in.GET("/employees/:id", h.getEmployee)
	in.GET("/kpi/sheet", h.kpiSheet)
	in.PUT("/kpi/results", h.saveResult)
	in.POST("/kpi/results/:id/evidence", upload, h.uploadEvidence)
	in.GET("/kpi/results/:id/evidence", h.downloadEvidence)
	in.GET("/periods/:id/appraisals/:employee_id", h.getAppraisal)
	in.PUT("/periods/:id/appraisals/:employee_id/self", h.saveSelf)
	in.GET("/dashboard/employee", h.dashEmployee)

	// manajer (dan Direktur untuk bawahannya)
	mgr := in.Group("", middleware.RequireRoles(model.RoleManager, model.RoleDirector))
	mgr.POST("/kpi/results/:id/validate", h.validateResult)
	mgr.GET("/periods/:id/team", h.team)
	mgr.PUT("/periods/:id/appraisals/:employee_id/manager", h.saveManager)
	mgr.GET("/dashboard/manager", h.dashManager)

	// HR / Direktur
	fin := in.Group("", middleware.RequireRoles(model.RoleHR, model.RoleDirector))
	fin.POST("/periods/:id/appraisals/:employee_id/finalize", h.finalize)
	fin.GET("/dashboard/director", h.dashDirector)
	fin.GET("/reports/scores", h.reportScores)
	fin.GET("/reports/scores/export", h.reportExport)
	fin.GET("/kpi-templates", h.listTemplates)
	fin.GET("/kpi-templates/:id", h.getTemplate)

	// HR saja
	hr := in.Group("", middleware.RequireRoles(model.RoleHR))
	hr.POST("/departments", h.createDepartment)
	hr.PUT("/departments/:id", h.updateDepartment)
	hr.DELETE("/departments/:id", h.deleteDepartment)
	hr.POST("/positions", h.createPosition)
	hr.PUT("/positions/:id", h.updatePosition)
	hr.DELETE("/positions/:id", h.deletePosition)

	hr.GET("/employees", h.listEmployees)
	hr.POST("/employees", h.createEmployee)
	hr.POST("/employees/import", upload, h.importEmployees)
	hr.GET("/employees-import-template", h.importTemplate)
	hr.PUT("/employees/:id", h.updateEmployee)
	hr.DELETE("/employees/:id", h.deleteEmployee)

	hr.POST("/leave-types", h.createLeaveType)
	hr.PUT("/leave-types/:id", h.updateLeaveType)
	hr.DELETE("/leave-types/:id", h.deleteLeaveType)
	hr.POST("/holidays", h.createHoliday)
	hr.POST("/holidays/import", h.importHolidays)
	hr.PUT("/holidays/:id", h.updateHoliday)
	hr.DELETE("/holidays/:id", h.deleteHoliday)
	hr.GET("/notification-settings", h.notificationSettings)
	hr.PUT("/notification-settings", h.setNotificationSetting)

	hr.GET("/periods/progress", h.periodProgress)
	hr.POST("/periods", h.createPeriod)
	hr.PUT("/periods/:id", h.updatePeriod)
	hr.POST("/periods/:id/open", h.periodAction(h.Periods.Open))
	hr.POST("/periods/:id/close", h.periodAction(h.Periods.Close))
	hr.POST("/periods/:id/reopen", h.periodAction(h.Periods.Reopen))

	hr.POST("/kpi-templates", h.createTemplate)
	hr.PUT("/kpi-templates/:id", h.saveTemplate)
	hr.GET("/activity-log", h.listAudit)
	return r
}
