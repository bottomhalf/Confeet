package approuters

import (
	"Confeet/internal/configuration"
	"Confeet/internal/hub"
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func StartServer(container *configuration.Container) {
	h := container.Hub

	// Create app server (includes WebSocket route)
	appServer := configureServer(container, h)

	// Channel to listen for errors from servers
	serverErrors := make(chan error, 1)

	// Start application server
	go func() {
		log.Printf("Application server starting at http://localhost:%d", container.Config.Server.SocketPort)
		log.Printf("WebSocket endpoint: ws://localhost:%d/cf/meet/%s", container.Config.Server.SocketPort, container.Config.ChatDatabase.SocketRoute)
		if err := appServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErrors <- fmt.Errorf("app server error: %w", err)
		}
	}()

	// Listen for shutdown signals
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Block until we receive a signal or server error
	select {
	case err := <-serverErrors:
		log.Printf("Server error: %v", err)
	case sig := <-quit:
		log.Printf("Received signal: %v. Initiating graceful shutdown...", sig)
	}

	// Create shutdown context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Shutdown sequence
	log.Println("Stopping hub and closing all WebSocket connections...")
	h.Stop()

	log.Println("Shutting down application server...")
	if err := appServer.Shutdown(ctx); err != nil {
		log.Printf("App server shutdown error: %v", err)
	}

	log.Println("Graceful shutdown complete")
}

func configureServer(container *configuration.Container, h *hub.Hub) *http.Server {
	router := gin.Default()

	// Configure CORS
	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:4200", "https://www.confeet.com"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	router.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "Welcome to Confeet Application Server!",
		})
	})

	// WebSocket route
	// User connects once with userId, then subscribes to conversations dynamically
	wsRoute := "/cf/meet/" + container.Config.ChatDatabase.SocketRoute
	router.GET(wsRoute, func(c *gin.Context) {
		userId := c.Query("userId")
		if userId == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "userId is required"})
			return
		}

		h.ServeWS(c, userId)
	})

	UserRouters(router, container)
	MeetingRouters(router, container)
	MonitorRouters(router, container)

	return &http.Server{
		Addr:         fmt.Sprintf(":%d", container.Config.Server.SocketPort),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
}
