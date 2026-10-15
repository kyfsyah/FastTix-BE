package routes

import (
	"fasttix-backend/controllers"
	"fasttix-backend/middleware"

	"github.com/gofiber/fiber/v2"
)

func SetupRoutes(app *fiber.App) {
	api := app.Group("/api")

	// Auth routes
	api.Post("/register", controllers.Register)
	api.Post("/login", controllers.Login)
	api.Get("/profile", middleware.Protected, controllers.Profile)

	// Singer routes
	api.Get("/singers", controllers.GetSingers)
	api.Get("/singers/:id", controllers.GetSingerByID)
	api.Post("/singers", middleware.Protected, middleware.AdminOnly, controllers.CreateSinger)
	api.Put("/singers/:id", middleware.Protected, middleware.AdminOnly, controllers.UpdateSinger)
	api.Delete("/singers/:id", middleware.Protected, middleware.AdminOnly, controllers.DeleteSinger)

	// Band routes
	api.Get("/bands", controllers.GetBands)
	api.Get("/bands/:id", controllers.GetBandByID)
	api.Post("/bands", middleware.Protected, middleware.AdminOnly, controllers.CreateBand)
	api.Put("/bands/:id", middleware.Protected, middleware.AdminOnly, controllers.UpdateBand)
	api.Delete("/bands/:id", middleware.Protected, middleware.AdminOnly, controllers.DeleteBand)

	// Event routes
	api.Get("/events", controllers.GetEvents)
	api.Get("/events/:id", controllers.GetEventByID)
	api.Post("/events", middleware.Protected, middleware.AdminOnly, controllers.CreateEvent)
	api.Put("/events/:id", middleware.Protected, middleware.AdminOnly, controllers.UpdateEvent)
	api.Delete("/events/:id", middleware.Protected, middleware.AdminOnly, controllers.DeleteEvent)
}
