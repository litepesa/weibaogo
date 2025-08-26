// ===============================
// main.go - Updated Entry Point
// ===============================

package main

import (
	"log"

	"weibaobe/internal/config"
	"weibaobe/internal/database"
	"weibaobe/internal/handlers"
	"weibaobe/internal/middleware"
	"weibaobe/internal/services"
	"weibaobe/internal/storage"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file not found")
	}

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatal("Failed to load configuration:", err)
	}

	// Set Gin mode
	gin.SetMode(cfg.Environment)

	// Initialize database with new config structure
	db, err := database.Connect(cfg.Database.ConnectionString())
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}
	defer database.Close()

	// Run migrations if needed
	if err := database.RunMigrations(db); err != nil {
		log.Fatal("Failed to run migrations:", err)
	}

	// Initialize R2 storage
	r2Client, err := storage.NewR2Client(cfg.R2Config)
	if err != nil {
		log.Fatal("Failed to initialize R2 client:", err)
	}

	// Initialize services
	dramaService := services.NewDramaService(db, r2Client)
	walletService := services.NewWalletService(db)
	uploadService := services.NewUploadService(r2Client)

	// Initialize handlers
	userHandler := handlers.NewUserHandler(db)
	dramaHandler := handlers.NewDramaHandler(dramaService)
	episodeHandler := handlers.NewEpisodeHandler(dramaService)
	walletHandler := handlers.NewWalletHandler(walletService)
	uploadHandler := handlers.NewUploadHandler(uploadService)

	// Setup router
	router := gin.Default()

	// CORS middleware
	router.Use(cors.New(cors.Config{
		AllowOrigins:     cfg.AllowedOrigins,
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * 3600, // 12 hours
	}))

	// Health check
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":   "healthy",
			"database": database.Health() == nil,
		})
	})

	// API routes
	setupRoutes(router, userHandler, dramaHandler, episodeHandler, walletHandler, uploadHandler)

	// Start server
	port := cfg.Port
	log.Printf("🚀 Server starting on port %s", port)
	log.Printf("🌐 Environment: %s", cfg.Environment)
	log.Printf("💾 Database connected to %s:%s", cfg.Database.Host, cfg.Database.Port)
	log.Printf("☁️  R2 storage initialized")

	log.Fatal(router.Run(":" + port))
}

func setupRoutes(
	router *gin.Engine,
	userHandler *handlers.UserHandler,
	dramaHandler *handlers.DramaHandler,
	episodeHandler *handlers.EpisodeHandler,
	walletHandler *handlers.WalletHandler,
	uploadHandler *handlers.UploadHandler,
) {
	api := router.Group("/api/v1")

	// Public routes (no auth required)
	public := api.Group("")
	{
		// Drama discovery (public)
		public.GET("/dramas", dramaHandler.GetDramas)
		public.GET("/dramas/featured", dramaHandler.GetFeaturedDramas)
		public.GET("/dramas/trending", dramaHandler.GetTrendingDramas)
		public.GET("/dramas/search", dramaHandler.SearchDramas)
		public.GET("/dramas/:dramaId", dramaHandler.GetDrama)
		public.GET("/dramas/:dramaId/episodes", episodeHandler.GetDramaEpisodes)
		public.GET("/episodes/:episodeId", episodeHandler.GetEpisode)
	}

	// Protected routes (Firebase auth required)
	protected := api.Group("")
	protected.Use(middleware.FirebaseAuth())
	{
		// User management
		protected.POST("/users", userHandler.CreateUser)
		protected.GET("/users/:uid", userHandler.GetUser)
		protected.PUT("/users/:uid", userHandler.UpdateUser)
		protected.DELETE("/users/:uid", userHandler.DeleteUser)

		// User actions
		protected.POST("/users/:uid/favorites", userHandler.ToggleFavorite)
		protected.POST("/users/:uid/watch-history", userHandler.AddToWatchHistory)
		protected.POST("/users/:uid/drama-progress", userHandler.UpdateDramaProgress)
		protected.GET("/users/:uid/favorites", userHandler.GetFavorites)
		protected.GET("/users/:uid/continue-watching", userHandler.GetContinueWatching)

		// Drama unlock
		protected.POST("/unlock-drama", dramaHandler.UnlockDrama)

		// Wallet
		protected.GET("/wallet/:userId", walletHandler.GetWallet)
		protected.GET("/wallet/:userId/transactions", walletHandler.GetTransactions)
		protected.POST("/wallet/:userId/purchase-request", walletHandler.CreatePurchaseRequest)

		// File upload
		protected.POST("/upload", uploadHandler.UploadFile)

		// Admin routes
		admin := protected.Group("")
		admin.Use(middleware.AdminOnly())
		{
			// Drama management
			admin.POST("/admin/dramas", dramaHandler.CreateDrama)
			admin.PUT("/admin/dramas/:dramaId", dramaHandler.UpdateDrama)
			admin.DELETE("/admin/dramas/:dramaId", dramaHandler.DeleteDrama)
			admin.PATCH("/admin/dramas/:dramaId/featured", dramaHandler.ToggleFeatured)
			admin.PATCH("/admin/dramas/:dramaId/active", dramaHandler.ToggleActive)
			admin.GET("/admin/dramas", dramaHandler.GetAdminDramas)

			// Episode management
			admin.POST("/admin/dramas/:dramaId/episodes", episodeHandler.CreateEpisode)
			admin.PUT("/admin/episodes/:episodeId", episodeHandler.UpdateEpisode)
			admin.DELETE("/admin/episodes/:episodeId", episodeHandler.DeleteEpisode)

			// Wallet management
			admin.POST("/admin/wallet/:userId/add-coins", walletHandler.AddCoins)
			admin.GET("/admin/purchase-requests", walletHandler.GetPendingPurchases)
			admin.POST("/admin/purchase-requests/:requestId/approve", walletHandler.ApprovePurchase)
			admin.POST("/admin/purchase-requests/:requestId/reject", walletHandler.RejectPurchase)

			// Analytics
			admin.GET("/admin/stats", handlers.GetStats)
			admin.GET("/admin/users", userHandler.GetAllUsers)
		}
	}
}
