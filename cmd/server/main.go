package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"spsc-loaneasy/internal/adapters/http/middleware"
	"spsc-loaneasy/internal/adapters/http/routes"
	"spsc-loaneasy/internal/adapters/persistence/models"
	"spsc-loaneasy/internal/config"
	"spsc-loaneasy/internal/core/services"

	"github.com/gofiber/fiber/v2"

	_ "spsc-loaneasy/docs" // Swagger docs
)

// @title SPSC loanEasy API
// @version 1.0
// @description ระบบสินเชื่อสหกรณ์ SPSC loanEasy v1.0 API
// @termsOfService http://swagger.io/terms/

// @contact.name API Support
// @contact.email support@spsc.or.th

// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html

// @host api.loanspsc.com
// @BasePath /api/v1
// @schemes https

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and JWT token.

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("❌ Failed to load configuration: %v", err)
	}

	// Connect to database
	db, err := config.ConnectDatabase(cfg)
	if err != nil {
		log.Fatalf("❌ Failed to connect to database: %v", err)
	}
	defer config.CloseDatabase()

	// Auto migrate (creates tables if not exist)
	// Note: This only migrates new tables, NOT the legacy flommast table
	if err := models.AutoMigrate(db); err != nil {
		log.Fatalf("❌ Failed to auto migrate: %v", err)
	}
	log.Println("✅ Database migration completed")

	// Seed master data (Phase 4)
	if err := config.SeedMasterData(db); err != nil {
		log.Printf("⚠️ Warning: Failed to seed master data: %v", err)
	}

	// Start Cron Service for LINE reminders (08:30 daily)
	cronService := services.NewCronService(db)
	cronService.Start()
	defer cronService.Stop()

	// Create Fiber app
	app := fiber.New(fiber.Config{
		AppName:      "SPSC loanEasy API v1.0",
		ErrorHandler: middleware.CustomErrorHandler,
	})

	// Setup middlewares
	middleware.Setup(app, cfg)

	// Setup routes (pass db and cfg for dependency injection)
	routes.Setup(app, db, cfg)

	// Graceful shutdown
	go gracefulShutdown(app)

	// Start server
	log.Printf("🚀 Server starting on port %s [MODE: %s]", cfg.Port, cfg.AppMode)
	if err := app.Listen(":" + cfg.Port); err != nil {
		log.Fatalf("❌ Failed to start server: %v", err)
	}
}

// gracefulShutdown handles graceful shutdown
func gracefulShutdown(app *fiber.App) {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("🛑 Shutting down server...")
	if err := app.Shutdown(); err != nil {
		log.Printf("❌ Error during shutdown: %v", err)
	}
	log.Println("✅ Server stopped gracefully")
}
