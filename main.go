package main

import (
	"log"

	"fasttix-backend/config"
	"fasttix-backend/routes"

	"github.com/gofiber/fiber/v2"
	"github.com/joho/godotenv"
)

func main() {
	// Load .env file
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using OS environment variables")
	}

	// Connect to Database
	config.ConnectDB()

	// Ensure connection pool is closed on exit
	defer config.CloseDB()

	// Initialize Fiber app
	app := fiber.New()

	// Setup routes
	routes.SetupRoutes(app)

	// Start server
	log.Println("Server is running on port 3000")
	if err := app.Listen(":3000"); err != nil {
		log.Fatalf("Error starting server: %v", err)
	}
}
