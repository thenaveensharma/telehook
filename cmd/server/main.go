package main

import (
	"log"
	"os"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/joho/godotenv"
	"github.com/thenaveensharma/telehook/internal/database"
	"github.com/thenaveensharma/telehook/internal/handlers"
	"github.com/thenaveensharma/telehook/internal/middleware"
	"github.com/thenaveensharma/telehook/internal/queue"
	"github.com/thenaveensharma/telehook/internal/telegram"
)

func main() {
	// Load .env file
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	// Initialize database
	db, err := database.NewDB()
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Initialize Telegram bot
	bot, err := telegram.NewBot()
	if err != nil {
		log.Printf("WARNING: Failed to initialize Telegram bot: %v", err)
		log.Println("The server will start, but webhook functionality will not work.")
		log.Println("Please configure TELEGRAM_BOT_TOKEN and TELEGRAM_CHANNEL_ID in your .env file.")
		bot = nil // Set to nil so we can check later
	}

	// Initialize Fiber app
	app := fiber.New(fiber.Config{
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			if e, ok := err.(*fiber.Error); ok {
				code = e.Code
			}
			return c.Status(code).JSON(fiber.Map{
				"error": err.Error(),
			})
		},
	})

	// Middleware
	app.Use(recover.New())
	app.Use(logger.New())
	app.Use(cors.New())

	// Initialize alert queue system
	processor := queue.NewTelegramProcessor(bot, db)
	processor.InitializeDefaultRules()

	// Alert queue sized to handle burst traffic:
	// - 20 workers for concurrent processing
	// - 15000 queue capacity to buffer stress test (12,000 alerts + headroom)
	alertQueue := queue.NewAlertQueue(20, 15000, processor)
	alertQueue.Start()
	defer alertQueue.Stop()

	log.Println("Alert queue system initialized (20 workers, 15k capacity)")

	// Initialize rate limiter with high limits for webhook endpoint
	rateLimiter := middleware.NewRateLimiter()

	// Initialize handlers
	authHandler := handlers.NewAuthHandler(db)
	webhookHandler := handlers.NewWebhookHandler(db, bot, alertQueue)
	telegramConfigHandler := handlers.NewTelegramConfigHandler(db)
	analyticsHandler := handlers.NewAnalyticsHandler(db)

	// Serve static files
	app.Static("/static", "./web/static")

	// Web routes (HTML pages)
	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendFile("./web/templates/index.html")
	})

	app.Get("/login", func(c *fiber.Ctx) error {
		return c.SendFile("./web/templates/login.html")
	})

	app.Get("/signup", func(c *fiber.Ctx) error {
		return c.SendFile("./web/templates/signup.html")
	})

	app.Get("/dashboard", func(c *fiber.Ctx) error {
		return c.SendFile("./web/templates/dashboard.html")
	})

	// API Routes
	api := app.Group("/api")

	// Health check
	api.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status": "healthy",
			"service": "telegram-webhook-bot",
		})
	})

	// Auth routes (public)
	auth := api.Group("/auth")
	auth.Post("/signup", authHandler.Signup)
	auth.Post("/login", authHandler.Login)

	// Protected routes
	user := api.Group("/user", middleware.JWTMiddleware())
	user.Get("/webhook-info", webhookHandler.GetWebhookInfo)
	user.Get("/queue-stats", webhookHandler.GetQueueStats)

	// Telegram bot configuration routes (protected)
	bots := user.Group("/bots")
	bots.Post("/", telegramConfigHandler.CreateBot)
	bots.Get("/", telegramConfigHandler.GetBots)
	bots.Get("/with-channels", telegramConfigHandler.GetBotsWithChannels)
	bots.Get("/:id", telegramConfigHandler.GetBot)
	bots.Put("/:id", telegramConfigHandler.UpdateBot)
	bots.Delete("/:id", telegramConfigHandler.DeleteBot)

	// Telegram channel configuration routes (protected)
	channels := user.Group("/channels")
	channels.Post("/", telegramConfigHandler.CreateChannel)
	channels.Get("/", telegramConfigHandler.GetChannels)
	channels.Get("/:id", telegramConfigHandler.GetChannel)
	channels.Put("/:id", telegramConfigHandler.UpdateChannel)
	channels.Delete("/:id", telegramConfigHandler.DeleteChannel)

	// Analytics routes (protected)
	user.Get("/analytics", analyticsHandler.GetAnalytics)

	// Webhook endpoint (uses webhook token, not JWT) - Rate limited to prevent abuse
	api.Post("/webhook/:token", rateLimiter.Middleware(), webhookHandler.HandleWebhook)

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "10000" // Render's default port
	}

	host := os.Getenv("SERVER_HOST")
	if host == "" {
		host = "0.0.0.0"
	}

	log.Printf("Server starting on %s:%s", host, port)
	if err := app.Listen(host + ":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
