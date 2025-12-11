package approuters

import (
	"Confeet/internal/configuration"

	"github.com/gin-gonic/gin"
)

func UserRouters(router *gin.Engine, container *configuration.Container) {
	// Placeholder for user-related route setup
	userRoute := router.Group("/cf/api/users")
	{
		userRoute.GET("/get-all-users", container.UserHandler.GetAllUsers)
		userRoute.POST("/create-user", container.UserHandler.CreateUser)
		userRoute.GET("/meeting-rooms", container.UserHandler.GetMeetingRooms)
	}
}
