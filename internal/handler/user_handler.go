package handler

import (
	"Confeet/internal/service"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type UserHandler interface {
	// Define methods for user handling here
	GetAllUsers(c *gin.Context)
	CreateUser(c *gin.Context)
	GetMeetingRooms(c *gin.Context)
	GetRoomMessages(c *gin.Context)
}

type userHandler struct {
	// Add fields for dependencies here
	service service.UserService
}

// CreateUser implements UserHandler.
func (h *userHandler) CreateUser(c *gin.Context) {
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

func (h *userHandler) GetMeetingRooms(c *gin.Context) {
	cvs := h.service.GetMeetingRooms(c.Request.Context())

	c.JSON(http.StatusOK, gin.H{
		"conversations": cvs,
	})
}

func (h *userHandler) GetRoomMessages(c *gin.Context) {
	conversationId := c.Param("conversationId")
	page := c.DefaultQuery("page", "1")
	pageNumber, err := strconv.ParseInt(page, 10, 64)
	if err != nil || pageNumber < 1 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid page number",
		})
		return
	}

	msgs, err := h.service.GetRoomMessagesService(c.Request.Context(), conversationId, pageNumber)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get messages",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"messages": msgs,
	})
}
