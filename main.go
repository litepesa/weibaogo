// ===============================
// main.go - Video Social Media App Entry Point (FIXED - Auth Chicken-and-Egg Problem Resolved)
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

	// ===============================
	// AUTH ROUTES (FIXED - Resolves Chicken-and-Egg Problem)
	// ===============================
	auth := api.Group("/auth")
	{
		// 🔧 CRITICAL FIX: User sync endpoint WITHOUT authentication middleware
		// This solves the chicken-and-egg problem by allowing user creation BEFORE auth check
		// Flutter should ALWAYS use this endpoint for new user creation/sync
		auth.POST("/sync", authHandler.SyncUser)

		// Token verification endpoint (for manual token validation)
		auth.POST("/verify", authHandler.VerifyToken)

		// 🗑️ REMOVED: The problematic protected sync endpoint that caused the chicken-and-egg issue
		// OLD: auth.POST("/sync-with-token", middleware.FirebaseAuth(firebaseService), authHandler.SyncUserWithToken)
		// This endpoint required user to exist in DB before they could be created - impossible!
	}

	// ===============================
	// PROTECTED AUTH ROUTES (Require existing user in database)
	// ===============================
	protectedAuth := api.Group("/auth")
	protectedAuth.Use(middleware.FirebaseAuth(firebaseService))
	{
		// Get current authenticated user info (requires user to exist in DB)
		protectedAuth.GET("/user", authHandler.GetCurrentUser)

		// Alternative sync that requires existing authentication (for profile updates)
		protectedAuth.POST("/profile-sync", authHandler.SyncUserWithToken)
	}

	// ===============================
	// PUBLIC ROUTES (NO AUTH REQUIRED)
	// ===============================
	public := api.Group("")
	{
		// Video discovery endpoints (public browsing)
		public.GET("/videos", videoHandler.GetVideos)
		public.GET("/videos/featured", videoHandler.GetFeaturedVideos)
		public.GET("/videos/trending", videoHandler.GetTrendingVideos)
		public.GET("/videos/:videoId", videoHandler.GetVideo)
		public.POST("/videos/:videoId/views", videoHandler.IncrementViews)
		public.GET("/users/:userId/videos", videoHandler.GetUserVideos)

		// Comment endpoints (public read access)
		public.GET("/videos/:videoId/comments", videoHandler.GetVideoComments)

		// User profile endpoints (public access for viewing profiles)
		public.GET("/users/:userId", userHandler.GetUser)
		public.GET("/users/:userId/stats", userHandler.GetUserStats)
		public.GET("/users/:userId/followers", videoHandler.GetUserFollowers)
		public.GET("/users/:userId/following", videoHandler.GetUserFollowing)

		// User search (public)
		public.GET("/users", userHandler.GetAllUsers)
		public.GET("/users/search", userHandler.SearchUsers)
	}

	// ===============================
	// PROTECTED ROUTES (FIREBASE AUTH REQUIRED + USER EXISTS IN DB)
	// ===============================
	protected := api.Group("")
	protected.Use(middleware.FirebaseAuth(firebaseService))
	{
		// User management endpoints (now work because sync creates user first)
		protected.PUT("/users/:userId", userHandler.UpdateUser)
		protected.DELETE("/users/:userId", userHandler.DeleteUser)
		protected.POST("/users/:userId/status", userHandler.UpdateUserStatus)

		// Video creation and management
		protected.POST("/videos", videoHandler.CreateVideo)
		protected.PUT("/videos/:videoId", videoHandler.UpdateVideo)
		protected.DELETE("/videos/:videoId", videoHandler.DeleteVideo)

		// Video interaction endpoints
		protected.POST("/videos/:videoId/like", videoHandler.LikeVideo)
		protected.DELETE("/videos/:videoId/like", videoHandler.UnlikeVideo)
		protected.POST("/videos/:videoId/share", videoHandler.ShareVideo)
		protected.GET("/users/:userId/liked-videos", videoHandler.GetUserLikedVideos)

		// Social features
		protected.POST("/users/:userId/follow", videoHandler.FollowUser)
		protected.DELETE("/users/:userId/follow", videoHandler.UnfollowUser)
		protected.GET("/feed/following", videoHandler.GetFollowingFeed)

		// Comment management (authenticated actions)
		protected.POST("/videos/:videoId/comments", videoHandler.CreateComment)
		protected.DELETE("/comments/:commentId", videoHandler.DeleteComment)
		protected.POST("/comments/:commentId/like", videoHandler.LikeComment)
		protected.DELETE("/comments/:commentId/like", videoHandler.UnlikeComment)

		// User analytics and statistics
		protected.GET("/stats/videos", videoHandler.GetVideoStats)

		// ===============================
		// WALLET ENDPOINTS (INDEPENDENT FEATURE)
		// ===============================
		protected.GET("/wallet/:userId", walletHandler.GetWallet)
		protected.GET("/wallet/:userId/transactions", walletHandler.GetTransactions)
		protected.POST("/wallet/:userId/purchase-request", walletHandler.CreatePurchaseRequest)

		// ===============================
		// FILE UPLOAD ENDPOINTS
		// ===============================
		protected.POST("/upload", uploadHandler.UploadFile)
		protected.POST("/upload/batch", uploadHandler.BatchUploadFiles)
		protected.GET("/upload/health", uploadHandler.HealthCheck)

		// ===============================
		// ADMIN ROUTES (ADMIN ACCESS REQUIRED)
		// ===============================
		admin := protected.Group("")
		admin.Use(middleware.AdminOnly())
		{
			// Video moderation
			admin.POST("/admin/videos/:videoId/featured", videoHandler.ToggleFeatured)
			admin.POST("/admin/videos/:videoId/active", videoHandler.ToggleActive)

			// User management (admin functions)
			admin.GET("/admin/users", userHandler.GetAllUsers)
			admin.POST("/admin/users/:userId/status", userHandler.UpdateUserStatus)

			// Wallet management (admin functions)
			admin.POST("/admin/wallet/:userId/add-coins", walletHandler.AddCoins)
			admin.GET("/admin/purchase-requests", walletHandler.GetPendingPurchases)
			admin.POST("/admin/purchase-requests/:requestId/approve", walletHandler.ApprovePurchase)
			admin.POST("/admin/purchase-requests/:requestId/reject", walletHandler.RejectPurchase)

			// Platform analytics
			admin.GET("/admin/stats", func(c *gin.Context) {
				c.JSON(200, gin.H{
					"message": "Platform statistics endpoint",
					"note":    "Video social media analytics dashboard",
					"status":  "operational",
				})
			})

			// System health and monitoring
			admin.GET("/admin/health", func(c *gin.Context) {
				dbStats := database.Stats()
				c.JSON(200, gin.H{
					"database": gin.H{
						"status":           "connected",
						"open_connections": dbStats.OpenConnections,
						"in_use":           dbStats.InUse,
						"idle":             dbStats.Idle,
					},
					"firebase": gin.H{
						"status": "initialized",
					},
					"storage": gin.H{
						"status": "connected",
						"type":   "cloudflare-r2",
					},
					"app": gin.H{
						"name":    "video-social-media",
						"version": "1.0.0",
						"status":  "healthy",
					},
				})
			})
		}
	}

	// ===============================
	// DEVELOPMENT ROUTES (DEBUG MODE ONLY)
	// ===============================
	if gin.Mode() == gin.DebugMode {
		debug := api.Group("/debug")
		{
			debug.GET("/routes", func(c *gin.Context) {
				routes := router.Routes()
				routeList := make([]gin.H, len(routes))
				for i, route := range routes {
					routeList[i] = gin.H{
						"method": route.Method,
						"path":   route.Path,
					}
				}
				c.JSON(200, gin.H{
					"total":  len(routes),
					"routes": routeList,
				})
			})

			debug.GET("/config", func(c *gin.Context) {
				c.JSON(200, gin.H{
					"environment": gin.Mode(),
					"database":    "connected",
					"firebase":    "initialized",
					"storage":     "r2-connected",
				})
			})

			// 🔍 DEBUG: Auth flow testing endpoint
			debug.GET("/auth-flow", func(c *gin.Context) {
				c.JSON(200, gin.H{
					"message": "Auth flow endpoints",
					"endpoints": gin.H{
						"public_sync":    "POST /api/v1/auth/sync (NO auth required - for new users)",
						"protected_sync": "POST /api/v1/auth/profile-sync (auth required - for updates)",
						"get_user":       "GET /api/v1/auth/user (auth required)",
						"verify_token":   "POST /api/v1/auth/verify (no auth required)",
					},
					"note": "New users should use /auth/sync, existing users can use /auth/profile-sync",
				})
			})
		}
	}
}
