package handler

import (
	"Confeet/internal/service"
	"net/http"

	"github.com/gin-gonic/gin"
)

type UserHandler interface {
	// Define methods for user handling here
	GetAllUsers(c *gin.Context)
	CreateUser(c *gin.Context)
	GetMeetingRooms(c *gin.Context)
}

type userHandler struct {
	// Add fields for dependencies here
	service service.UserService
}

// CreateUser implements UserHandler.
func (h *userHandler) CreateUser(c *gin.Context) {
	panic("unimplemented")
}

func (h *userHandler) GetMeetingRooms(c *gin.Context) {
	panic("unimplemented")
}

func NewUserHandler(service service.UserService) UserHandler {
	return &userHandler{
		service: service,
	}
}

func (h *userHandler) GetAllUsers(c *gin.Context) {
	// Placeholder for getting all users
	c.JSON(http.StatusOK, gin.H{
		"message": "NOT IMPLEMENTED YET",
	})
}
