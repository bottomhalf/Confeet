package approuters

import (
	"Confeet/internal/configuration"

	"github.com/gin-gonic/gin"
)

func MeetingRouters(router *gin.Engine, container *configuration.Container) {
	// Placeholder for meeting-related route setup
	meetingRoute := router.Group("/cf/api/meetings")
	{
		meetingRoute.GET("/get-all-meeting-rooms", container.UserHandler.GetMeetingRooms)
		meetingRoute.GET("/get-room-messages/:conversationId", container.UserHandler.GetRoomMessages)
	}
}
