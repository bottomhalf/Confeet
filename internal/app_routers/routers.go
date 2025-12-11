package approuters

import (
	"Confeet/internal/configuration"
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

	// WebSocket handler
	http.HandleFunc("/"+container.Config.ChatDatabase.SocketRoute, func(w http.ResponseWriter, r *http.Request) {
		userId := r.URL.Query().Get("userId")
		if userId == "" {
			http.Error(w, "userId is required", http.StatusBadRequest)
			return
		}
		conversationID := r.URL.Query().Get("conversationId")
		if conversationID == "" {
			http.Error(w, "conversationId is required", http.StatusBadRequest)
			return
		}

		h.ServeWS(w, r, userId, conversationID)
	})

	// Create servers with explicit configuration
	socketServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", container.Config.Server.SocketPort),
		Handler:      nil, // uses DefaultServeMux
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	appServer := createAppServer(container)

	// Channel to listen for errors from servers
	serverErrors := make(chan error, 2)

	// Start socket server
	go func() {
		log.Printf("Socket server starting at ws://localhost:%d", container.Config.Server.SocketPort)
		if err := socketServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErrors <- fmt.Errorf("socket server error: %w", err)
		}
	}()

	// Start application server
	go func() {
		log.Printf("Application server starting at http://localhost:%d", container.Config.Server.AppPort)
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

	log.Println("Shutting down socket server...")
	if err := socketServer.Shutdown(ctx); err != nil {
		log.Printf("Socket server shutdown error: %v", err)
	}

	log.Println("Shutting down application server...")
	if err := appServer.Shutdown(ctx); err != nil {
		log.Printf("App server shutdown error: %v", err)
	}

	log.Println("Graceful shutdown complete")
}

func createAppServer(container *configuration.Container) *http.Server {
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

	UserRouters(router, container)
	MeetingRouters(router, container)

	return &http.Server{
		Addr:         fmt.Sprintf(":%d", container.Config.Server.AppPort),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
}
