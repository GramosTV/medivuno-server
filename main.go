package main

import (
	"fmt"
	"log"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"

	"healthcare-app-server/internal/config"
	"healthcare-app-server/internal/models"
	"healthcare-app-server/internal/routes"
)

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Fatalf("Error loading .env file: %v", err)
	}

	// Initialize configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Error loading config: %v", err)
	}

	// Create a DatabaseConfig for models
	modelDbConfig := models.DatabaseConfig{
		DSN: cfg.Database.DSN,
	}

	// Initialize database connection
	db, err := models.InitDB(modelDbConfig)
	if err != nil {
		log.Fatalf("Error connecting to database: %v", err)
	}

	// Initialize Gin router
	router := gin.Default()

	// Configure CORS
	corsConfig := cors.DefaultConfig()
	corsConfig.AllowOrigins = []string{cfg.Origin}
	corsConfig.AllowCredentials = true
	corsConfig.AllowMethods = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"}
	corsConfig.AllowHeaders = []string{"Origin", "Content-Type", "Accept", "Authorization"}
	router.Use(cors.New(corsConfig))

	// Set up routes - passing DB and config to let routes.go create the handlers
	routes.SetupRoutes(router, db, cfg)

	// Start server
	serverAddr := fmt.Sprintf(":%s", cfg.Port)
	fmt.Printf("Server running on port %s\n", cfg.Port)
	if err := router.Run(serverAddr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
