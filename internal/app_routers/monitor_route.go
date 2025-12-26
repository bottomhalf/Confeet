package approuters

import (
	"Confeet/internal/configuration"
	"Confeet/internal/handler"
	"Confeet/internal/hub"

	"github.com/gin-gonic/gin"
)

// MonitorRouters sets up monitoring API routes
func MonitorRouters(router *gin.Engine, container *configuration.Container) {
	// Create monitor service with hub reference
	monitorService := hub.NewMonitorService(container.Hub)

	// Create monitor handler
	monitorHandler := handler.NewMonitorHandler(monitorService)

	// Monitor API group
	monitorGroup := router.Group("/cf/api/monitor")
	{
		// GET /api/monitor/stats - Get hub statistics
		monitorGroup.GET("/stats", monitorHandler.GetHubStats)
	}
}
