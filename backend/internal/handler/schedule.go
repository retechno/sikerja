package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/retechno/sikerja/backend/internal/service"
)

func (h *H) listSchedules(c *gin.Context) {
	v, err := h.Schedules.List(actor(c))
	if err != nil {
		h.fail(c, err)
		return
	}
	ok(c, v)
}

func (h *H) createSchedule(c *gin.Context) {
	var in service.ScheduleInput
	if !h.bind(c, &in) {
		return
	}
	v, err := h.Schedules.Create(actor(c), in)
	if err != nil {
		h.fail(c, err)
		return
	}
	created(c, v)
}

func (h *H) updateSchedule(c *gin.Context) {
	id, good := h.idParam(c, "id")
	if !good {
		return
	}
	var in service.ScheduleInput
	if !h.bind(c, &in) {
		return
	}
	v, err := h.Schedules.Update(actor(c), id, in)
	if err != nil {
		h.fail(c, err)
		return
	}
	ok(c, v)
}

func (h *H) deleteSchedule(c *gin.Context) {
	id, good := h.idParam(c, "id")
	if !good {
		return
	}
	if err := h.Schedules.Delete(actor(c), id); err != nil {
		h.fail(c, err)
		return
	}
	noContent(c)
}
