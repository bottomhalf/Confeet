package configuration

import (
	"Confeet/internal/db"
	"Confeet/internal/handler"
	"Confeet/internal/hub"
	"Confeet/internal/model"
	"Confeet/internal/repo"
	"Confeet/internal/service"
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
)

type Container struct {
	UserHandler handler.UserHandler
	Hub         *hub.Hub
	Config      Config
	Logger      *zap.Logger

	// private - for cleanup
	mongoClient *mongo.Database
}

func BuildContainer() (*Container, error) {
	// Get config path from environment variable, default to local dev path
	configPath := "../../shared/config.dev.json"
	if envPath := os.Getenv("CONFIG_PATH"); envPath != "" {
		configPath = envPath
	}

	config, err := LoadConfig(configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	fmt.Printf("Config loaded: %+v\n", config)

	con, err := db.OpenConnection(config.ChatDatabase.Uri, config.ChatDatabase.Database)
	if err != nil {
		return nil, err
	}

	mongoRepo := db.NewRepository[model.Message](con, config.ChatDatabase.MessagesCollection)

	logger, _ := zap.NewProduction()

	messageRepo := repo.NewMessageRepository(con, mongoRepo, logger)
	conversationRepo := repo.NewConversationRepository(con, logger)
	userRepo := repo.NewUserRepository(con, mongoRepo)
	userService := service.NewUserService(userRepo, messageRepo)
	userHandler := handler.NewUserHandler(userService)

	// Create Hub with repositories
	Hub := hub.NewHub(messageRepo, conversationRepo)

	return &Container{
		UserHandler: userHandler,
		Hub:         Hub,
		Config:      *config,
		Logger:      logger,
		mongoClient: con,
	}, nil
}

// Close gracefully shuts down all connections
func (c *Container) Close() error {
	// Stop the hub first (closes all WebSocket connections)
	if c.Hub != nil {
		c.Hub.Stop()
	}

	// Sync logger
	if c.Logger != nil {
		_ = c.Logger.Sync()
	}

	// Close MongoDB connection pool
	if c.mongoClient != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := c.mongoClient.Client().Disconnect(ctx); err != nil {
			return fmt.Errorf("failed to close MongoDB connection: %w", err)
		}
	}

	return nil
}
