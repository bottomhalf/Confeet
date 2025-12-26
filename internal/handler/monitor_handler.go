package handler

import (
	"Confeet/internal/hub"
	"net/http"

	"github.com/gin-gonic/gin"
)

// MonitorHandler handles monitoring API endpoints
type MonitorHandler interface {
	GetHubStats(c *gin.Context)
}

type monitorHandler struct {
	monitorService *hub.MonitorService
}

// NewMonitorHandler creates a new monitor handler
func NewMonitorHandler(monitorService *hub.MonitorService) MonitorHandler {
	return &monitorHandler{
		monitorService: monitorService,
	}
}

// GetHubStats returns current hub statistics
// @Summary Get WebSocket hub statistics
// @Description Returns information about connected clients, rooms, and active calls
// @Tags Monitor
// @Produce json
// @Success 200 {object} model.MonitorResponse
// @Router /api/monitor/stats [get]
func (h *monitorHandler) GetHubStats(c *gin.Context) {
	stats := h.monitorService.GetStats()

	c.JSON(http.StatusOK, gin.H{
		"HttpStatusCode": http.StatusOK,
		"ResponseBody":   stats,
		"IsSuccess":      true,
		"Message":        "Hub statistics retrieved successfully",
	})
}
