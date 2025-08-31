// ===============================
// main.go - Updated Entry Point for Unified Architecture
// ===============================

package main

import (
	"fmt"
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

	// Initialize Firebase service
	firebaseService, err := services.NewFirebaseService(cfg)
	if err != nil {
		log.Fatal("Failed to initialize Firebase service:", err)
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
	authHandler := handlers.NewAuthHandler(firebaseService)
	userHandler := handlers.NewUserHandler(db)
	dramaHandler := handlers.NewDramaHandler(dramaService)
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

	// API routes - UNIFIED ARCHITECTURE (NO EPISODE HANDLER)
	// FIXED: Pass dramaService as parameter
	setupUnifiedRoutes(router, firebaseService, authHandler, userHandler, dramaHandler, walletHandler, uploadHandler, dramaService)

	// Start server
	port := cfg.Port
	log.Printf("🚀 Server starting on port %s", port)
	log.Printf("🌐 Environment: %s", cfg.Environment)
	log.Printf("💾 Database connected to %s:%s", cfg.Database.Host, cfg.Database.Port)
	log.Printf("🔥 Firebase service initialized")
	log.Printf("☁️  R2 storage initialized")
	log.Printf("🎭 Unified Drama Architecture enabled")

	log.Fatal(router.Run(":" + port))
}

func setupUnifiedRoutes(
	router *gin.Engine,
	firebaseService *services.FirebaseService,
	authHandler *handlers.AuthHandler,
	userHandler *handlers.UserHandler,
	dramaHandler *handlers.DramaHandler,
	walletHandler *handlers.WalletHandler,
	uploadHandler *handlers.UploadHandler,
	dramaService *services.DramaService, // ADDED: dramaService parameter
) {
	api := router.Group("/api/v1")

	// Auth routes
	auth := api.Group("/auth")
	{
		auth.POST("/verify", authHandler.VerifyToken)
		auth.POST("/sync", middleware.FirebaseAuth(firebaseService), authHandler.SyncUser)
		auth.GET("/user", middleware.FirebaseAuth(firebaseService), authHandler.GetCurrentUser)
	}

	// Public routes (no auth required)
	public := api.Group("")
	{
		// Drama discovery (public)
		public.GET("/dramas", dramaHandler.GetDramas)
		public.GET("/dramas/featured", dramaHandler.GetFeaturedDramas)
		public.GET("/dramas/trending", dramaHandler.GetTrendingDramas)
		public.GET("/dramas/search", dramaHandler.SearchDramas)
		public.GET("/dramas/:dramaId", dramaHandler.GetDrama)

		// Episode-like endpoints (compatibility) - return drama episodes
		public.GET("/dramas/:dramaId/episodes", func(c *gin.Context) {
			dramaID := c.Param("dramaId")
			if dramaID == "" {
				c.JSON(400, gin.H{"error": "Drama ID required"})
				return
			}

			// FIXED: Use dramaService parameter directly instead of c.MustGet
			episodes, err := dramaService.GetDramaEpisodes(c.Request.Context(), dramaID)
			if err != nil {
				c.JSON(404, gin.H{"error": "Drama not found"})
				return
			}

			c.JSON(200, episodes)
		})

		// Individual episode endpoint (compatibility)
		public.GET("/episodes/:dramaId/:episodeNumber", func(c *gin.Context) {
			dramaID := c.Param("dramaId")
			episodeNumber := c.Param("episodeNumber")

			if dramaID == "" || episodeNumber == "" {
				c.JSON(400, gin.H{"error": "Drama ID and episode number required"})
				return
			}

			// Parse episode number
			var epNum int
			if _, err := fmt.Sscanf(episodeNumber, "%d", &epNum); err != nil {
				c.JSON(400, gin.H{"error": "Invalid episode number"})
				return
			}

			// FIXED: Use dramaService parameter directly instead of c.MustGet
			episode, err := dramaService.GetEpisodeByNumber(c.Request.Context(), dramaID, epNum)
			if err != nil {
				c.JSON(404, gin.H{"error": "Episode not found"})
				return
			}

			c.JSON(200, episode)
		})
	}

	// Protected routes (Firebase auth required)
	protected := api.Group("")
	protected.Use(middleware.FirebaseAuth(firebaseService))
	{
		// REMOVED: The problematic middleware that was trying to set dramaService in context

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

		// Drama interactions
		protected.POST("/dramas/:dramaId/views", dramaHandler.IncrementViews)
		protected.POST("/dramas/:dramaId/favorites", dramaHandler.ToggleFavorite)

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
			// Unified drama management
			admin.POST("/admin/dramas/create-with-episodes", dramaHandler.CreateDramaWithEpisodes)
			admin.PUT("/admin/dramas/:dramaId", dramaHandler.UpdateDrama)
			admin.DELETE("/admin/dramas/:dramaId", dramaHandler.DeleteDrama)
			admin.POST("/admin/dramas/:dramaId/featured", dramaHandler.ToggleFeatured)
			admin.POST("/admin/dramas/:dramaId/active", dramaHandler.ToggleActive)
			admin.GET("/admin/dramas", dramaHandler.GetAdminDramas)

			// Wallet management
			admin.POST("/admin/wallet/:userId/add-coins", walletHandler.AddCoins)
			admin.GET("/admin/purchase-requests", walletHandler.GetPendingPurchases)
			admin.POST("/admin/purchase-requests/:requestId/approve", walletHandler.ApprovePurchase)
			admin.POST("/admin/purchase-requests/:requestId/reject", walletHandler.RejectPurchase)

			// Analytics
			admin.GET("/admin/stats", func(c *gin.Context) {
				// Simplified stats endpoint
				c.JSON(200, gin.H{
					"message": "Stats endpoint - implement based on your needs",
					"note":    "Drama-centric analytics only",
				})
			})
			admin.GET("/admin/users", userHandler.GetAllUsers)
		}
	}
}
