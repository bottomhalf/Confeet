package configuration

import (
	"Confeet/internal/db"
	"Confeet/internal/event"
	"Confeet/internal/handler"
	"Confeet/internal/repo"
	"Confeet/internal/service"
	"fmt"
	"log"
)

type Container struct {
	// Add fields for dependencies here
	UserHandler handler.UserHandler
	Config      Config
}

func BuildContainer() (*Container, error) {
	config, err := LoadConfig("../../shared/config.dev.json")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	fmt.Printf("Config loaded: %+v\n", config)

	con, err := db.OpenConnection(config.ChatDatabase.Uri, config.ChatDatabase.Database)
	if err != nil {
		return nil, err
	}

	mongoRepo := db.NewRepository[event.WsEvent](con, config.ChatDatabase.Database)

	messageRepo := repo.NewMessageRepository(con, mongoRepo, nil)
	userRepo := repo.NewUserRepository(con, mongoRepo)
	userService := service.NewUserService(userRepo, messageRepo)
	userHandler := handler.NewUserHandler(userService)

	return &Container{
		UserHandler: userHandler,
		Config:      *config,
	}, nil
}
