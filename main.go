// ===============================
// main.go - Video Social Media App Entry Point
// ===============================

package main

import (
	//"fmt"
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

	// Initialize database
	db, err := database.Connect(cfg.Database.ConnectionString())
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}
	defer database.Close()

	// Run migrations
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
	videoService := services.NewVideoService(db, r2Client)
	walletService := services.NewWalletService(db)
	uploadService := services.NewUploadService(r2Client)

	// Initialize handlers
	authHandler := handlers.NewAuthHandler(firebaseService)
	userHandler := handlers.NewUserHandler(db)
	videoHandler := handlers.NewVideoHandler(videoService)
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
			"app":      "video-social-media",
		})
	})

	// Setup video social media routes
	setupVideoSocialMediaRoutes(router, firebaseService, authHandler, userHandler, videoHandler, walletHandler, uploadHandler)

	// Start server
	port := cfg.Port
	log.Printf("🚀 Video Social Media Server starting on port %s", port)
	log.Printf("🌍 Environment: %s", cfg.Environment)
	log.Printf("💾 Database connected to %s:%s", cfg.Database.Host, cfg.Database.Port)
	log.Printf("🔥 Firebase service initialized")
	log.Printf("☁️  R2 storage initialized")
	log.Printf("📱 Video Social Media API ready")

	log.Fatal(router.Run(":" + port))
}

func setupVideoSocialMediaRoutes(
	router *gin.Engine,
	firebaseService *services.FirebaseService,
	authHandler *handlers.AuthHandler,
	userHandler *handlers.UserHandler,
	videoHandler *handlers.VideoHandler,
	walletHandler *handlers.WalletHandler,
	uploadHandler *handlers.UploadHandler,
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
		// Video discovery (public)
		public.GET("/videos", videoHandler.GetVideos)
		public.GET("/videos/featured", videoHandler.GetFeaturedVideos)
		public.GET("/videos/trending", videoHandler.GetTrendingVideos)
		public.GET("/videos/:videoId", videoHandler.GetVideo)
		public.POST("/videos/:videoId/views", videoHandler.IncrementViews)
		public.GET("/users/:userId/videos", videoHandler.GetUserVideos)

		// Comment endpoints (public read)
		public.GET("/videos/:videoId/comments", videoHandler.GetVideoComments)

		// User profiles (public) - Using :userId to be consistent
		public.GET("/users/:userId", userHandler.GetUser)
		public.GET("/users/:userId/followers", videoHandler.GetUserFollowers)
		public.GET("/users/:userId/following", videoHandler.GetUserFollowing)
	}

	// Protected routes (Firebase auth required)
	protected := api.Group("")
	protected.Use(middleware.FirebaseAuth(firebaseService))
	{
		// User management - Using :userId for consistency
		protected.POST("/users", userHandler.CreateUser)
		protected.PUT("/users/:userId", userHandler.UpdateUser)
		protected.DELETE("/users/:userId", userHandler.DeleteUser)

		// Video creation and management
		protected.POST("/videos", videoHandler.CreateVideo)
		protected.PUT("/videos/:videoId", videoHandler.UpdateVideo)
		protected.DELETE("/videos/:videoId", videoHandler.DeleteVideo)

		// Video interactions
		protected.POST("/videos/:videoId/like", videoHandler.LikeVideo)
		protected.DELETE("/videos/:videoId/like", videoHandler.UnlikeVideo)
		protected.POST("/videos/:videoId/share", videoHandler.ShareVideo)
		protected.GET("/users/:userId/liked-videos", videoHandler.GetUserLikedVideos)

		// Social features
		protected.POST("/users/:userId/follow", videoHandler.FollowUser)
		protected.DELETE("/users/:userId/follow", videoHandler.UnfollowUser)
		protected.GET("/feed/following", videoHandler.GetFollowingFeed)

		// Comments
		protected.POST("/videos/:videoId/comments", videoHandler.CreateComment)
		protected.DELETE("/comments/:commentId", videoHandler.DeleteComment)
		protected.POST("/comments/:commentId/like", videoHandler.LikeComment)
		protected.DELETE("/comments/:commentId/like", videoHandler.UnlikeComment)

		// User stats and analytics
		protected.GET("/stats/videos", videoHandler.GetVideoStats)

		// Wallet
		protected.GET("/wallet/:userId", walletHandler.GetWallet)
		protected.GET("/wallet/:userId/transactions", walletHandler.GetTransactions)
		protected.POST("/wallet/:userId/purchase-request", walletHandler.CreatePurchaseRequest)

		// File upload
		protected.POST("/upload", uploadHandler.UploadFile)
		protected.POST("/upload/batch", uploadHandler.BatchUploadFiles)

		// Admin routes
		admin := protected.Group("")
		admin.Use(middleware.AdminOnly())
		{
			// Video moderation
			admin.POST("/admin/videos/:videoId/featured", videoHandler.ToggleFeatured)
			admin.POST("/admin/videos/:videoId/active", videoHandler.ToggleActive)

			// User management
			admin.GET("/admin/users", userHandler.GetAllUsers)

			// Wallet management
			admin.POST("/admin/wallet/:userId/add-coins", walletHandler.AddCoins)
			admin.GET("/admin/purchase-requests", walletHandler.GetPendingPurchases)
			admin.POST("/admin/purchase-requests/:requestId/approve", walletHandler.ApprovePurchase)
			admin.POST("/admin/purchase-requests/:requestId/reject", walletHandler.RejectPurchase)

			// Analytics
			admin.GET("/admin/stats", func(c *gin.Context) {
				// Get overall platform statistics
				c.JSON(200, gin.H{
					"message": "Platform stats endpoint",
					"note":    "Video social media analytics",
				})
			})
		}
	}
}
