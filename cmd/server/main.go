package main

import (
	approuters "Confeet/internal/app_routers"
	"Confeet/internal/configuration"
	"log"
)

func main() {
	container, err := configuration.BuildContainer()
	if err != nil {
		log.Fatalf("Failed to build container: %v", err)
	}

	// Setup routers
	approuters.StartServer(container)
}
