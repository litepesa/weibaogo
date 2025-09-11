// ===============================
// main.go - Video Social Media + Drama App Entry Point
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
	dramaService := services.NewDramaService(db, r2Client) // NEW: Drama service
	walletService := services.NewWalletService(db)
	uploadService := services.NewUploadService(r2Client)

	// Initialize handlers
	authHandler := handlers.NewAuthHandler(firebaseService)
	userHandler := handlers.NewUserHandler(db)
	videoHandler := handlers.NewVideoHandler(videoService)
	dramaHandler := handlers.NewDramaHandler(dramaService) // NEW: Drama handler
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
			"app":      "video-social-media-with-dramas",
		})
	})

	// Setup combined video + drama routes
	setupCombinedRoutes(router, firebaseService, authHandler, userHandler, videoHandler, dramaHandler, walletHandler, uploadHandler)

	// Start server
	port := cfg.Port
	log.Printf("🚀 Video Social Media + Drama Server starting on port %s", port)
	log.Printf("🌍 Environment: %s", cfg.Environment)
	log.Printf("💾 Database connected to %s:%s", cfg.Database.Host, cfg.Database.Port)
	log.Printf("🔥 Firebase service initialized")
	log.Printf("☁️  R2 storage initialized")
	log.Printf("📱 Video Social Media features: enabled")
	log.Printf("🎭 Drama features: enabled (verified users only)")

	log.Fatal(router.Run(":" + port))
}

func setupCombinedRoutes(
	router *gin.Engine,
	firebaseService *services.FirebaseService,
	authHandler *handlers.AuthHandler,
	userHandler *handlers.UserHandler,
	videoHandler *handlers.VideoHandler,
	dramaHandler *handlers.DramaHandler, // NEW: Drama handler
	walletHandler *handlers.WalletHandler,
	uploadHandler *handlers.UploadHandler,
) {
	api := router.Group("/api/v1")

	// ===============================
	// AUTH ROUTES
	// ===============================
	auth := api.Group("/auth")
	{
		// User sync endpoint WITHOUT authentication middleware (solves chicken-and-egg problem)
		auth.POST("/sync", authHandler.SyncUser)
		auth.POST("/verify", authHandler.VerifyToken)
	}

	// Protected auth routes
	protectedAuth := api.Group("/auth")
	protectedAuth.Use(middleware.FirebaseAuth(firebaseService))
	{
		protectedAuth.GET("/user", authHandler.GetCurrentUser)
		protectedAuth.POST("/profile-sync", authHandler.SyncUserWithToken)
	}

	// ===============================
	// PUBLIC ROUTES (NO AUTH REQUIRED)
	// ===============================
	public := api.Group("")
	{
		// ===== VIDEO ENDPOINTS =====
		public.GET("/videos", videoHandler.GetVideos)
		public.GET("/videos/featured", videoHandler.GetFeaturedVideos)
		public.GET("/videos/trending", videoHandler.GetTrendingVideos)
		public.GET("/videos/:videoId", videoHandler.GetVideo)
		public.POST("/videos/:videoId/views", videoHandler.IncrementViews)
		public.GET("/users/:userId/videos", videoHandler.GetUserVideos)
		public.GET("/videos/:videoId/comments", videoHandler.GetVideoComments)

		// ===== DRAMA ENDPOINTS (NEW) =====
		public.GET("/dramas", dramaHandler.GetDramas)
		public.GET("/dramas/featured", dramaHandler.GetFeaturedDramas)
		public.GET("/dramas/trending", dramaHandler.GetTrendingDramas)
		public.GET("/dramas/search", dramaHandler.SearchDramas)
		public.GET("/dramas/:dramaId", dramaHandler.GetDrama)
		public.POST("/dramas/:dramaId/views", dramaHandler.IncrementViews)

		// Episode endpoints (compatibility with old API)
		public.GET("/dramas/:dramaId/episodes", dramaHandler.GetDramaEpisodes)
		public.GET("/dramas/:dramaId/episodes/:episodeNumber", dramaHandler.GetEpisode)

		// ===== USER PROFILE ENDPOINTS (PUBLIC) =====
		public.GET("/users/:userId", userHandler.GetUser)
		public.GET("/users/:userId/stats", userHandler.GetUserStats)
		public.GET("/users/:userId/followers", videoHandler.GetUserFollowers)
		public.GET("/users/:userId/following", videoHandler.GetUserFollowing)
		public.GET("/users", userHandler.GetAllUsers)
		public.GET("/users/search", userHandler.SearchUsers)
	}

	// ===============================
	// PROTECTED ROUTES (FIREBASE AUTH REQUIRED)
	// ===============================
	protected := api.Group("")
	protected.Use(middleware.FirebaseAuth(firebaseService))
	{
		// ===== USER MANAGEMENT =====
		protected.PUT("/users/:userId", userHandler.UpdateUser)
		protected.DELETE("/users/:userId", userHandler.DeleteUser)
		protected.POST("/users/:userId/status", userHandler.UpdateUserStatus)

		// ===== VIDEO FEATURES =====
		protected.POST("/videos", videoHandler.CreateVideo)
		protected.PUT("/videos/:videoId", videoHandler.UpdateVideo)
		protected.DELETE("/videos/:videoId", videoHandler.DeleteVideo)
		protected.POST("/videos/:videoId/like", videoHandler.LikeVideo)
		protected.DELETE("/videos/:videoId/like", videoHandler.UnlikeVideo)
		protected.POST("/videos/:videoId/share", videoHandler.ShareVideo)
		protected.GET("/users/:userId/liked-videos", videoHandler.GetUserLikedVideos)

		// ===== DRAMA FEATURES (NEW) =====
		// Drama interactions
		protected.POST("/dramas/:dramaId/favorite", dramaHandler.ToggleFavorite)
		protected.POST("/dramas/:dramaId/progress", dramaHandler.UpdateProgress)
		protected.POST("/dramas/unlock", dramaHandler.UnlockDrama)

		// User drama data
		protected.GET("/my/dramas/favorites", dramaHandler.GetUserFavorites)
		protected.GET("/my/dramas/continue-watching", dramaHandler.GetContinueWatching)
		protected.GET("/dramas/:dramaId/progress", dramaHandler.GetUserProgress)

		// ===== SOCIAL FEATURES =====
		protected.POST("/users/:userId/follow", videoHandler.FollowUser)
		protected.DELETE("/users/:userId/follow", videoHandler.UnfollowUser)
		protected.GET("/feed/following", videoHandler.GetFollowingFeed)

		// ===== COMMENT MANAGEMENT =====
		protected.POST("/videos/:videoId/comments", videoHandler.CreateComment)
		protected.DELETE("/comments/:commentId", videoHandler.DeleteComment)
		protected.POST("/comments/:commentId/like", videoHandler.LikeComment)
		protected.DELETE("/comments/:commentId/like", videoHandler.UnlikeComment)

		// ===== ANALYTICS =====
		protected.GET("/stats/videos", videoHandler.GetVideoStats)

		// ===== WALLET ENDPOINTS =====
		protected.GET("/wallet/:userId", walletHandler.GetWallet)
		protected.GET("/wallet/:userId/transactions", walletHandler.GetTransactions)
		protected.POST("/wallet/:userId/purchase-request", walletHandler.CreatePurchaseRequest)

		// ===== FILE UPLOAD =====
		protected.POST("/upload", uploadHandler.UploadFile)
		protected.POST("/upload/batch", uploadHandler.BatchUploadFiles)
		protected.GET("/upload/health", uploadHandler.HealthCheck)

		// ===============================
		// VERIFIED USER DRAMA CREATION (VERIFIED USERS ONLY)
		// ===============================
		verifiedUser := protected.Group("")
		// Note: Verification check is done in the handlers, not middleware
		{
			// Drama creation and management (verified users only)
			verifiedUser.POST("/dramas", dramaHandler.CreateDramaWithEpisodes)
			verifiedUser.PUT("/dramas/:dramaId", dramaHandler.UpdateDrama)
			verifiedUser.DELETE("/dramas/:dramaId", dramaHandler.DeleteDrama)
			verifiedUser.POST("/dramas/:dramaId/featured", dramaHandler.ToggleFeatured)
			verifiedUser.POST("/dramas/:dramaId/active", dramaHandler.ToggleActive)

			// Verified user drama management
			verifiedUser.GET("/my/dramas", dramaHandler.GetMyDramas)
			verifiedUser.GET("/my/dramas/analytics", dramaHandler.GetMyDramaAnalytics)
			verifiedUser.GET("/dramas/:dramaId/revenue", dramaHandler.GetDramaRevenue)
		}

		// ===============================
		// ADMIN ROUTES (ADMIN ACCESS REQUIRED)
		// ===============================
		admin := protected.Group("")
		admin.Use(middleware.AdminOnly())
		{
			// ===== VIDEO MODERATION =====
			admin.POST("/admin/videos/:videoId/featured", videoHandler.ToggleFeatured)
			admin.POST("/admin/videos/:videoId/active", videoHandler.ToggleActive)

			// ===== USER MANAGEMENT (ADMIN) =====
			admin.GET("/admin/users", userHandler.GetAllUsers)
			admin.POST("/admin/users/:userId/status", userHandler.UpdateUserStatus)

			// ===== WALLET MANAGEMENT (ADMIN) =====
			admin.POST("/admin/wallet/:userId/add-coins", walletHandler.AddCoins)
			admin.GET("/admin/purchase-requests", walletHandler.GetPendingPurchases)
			admin.POST("/admin/purchase-requests/:requestId/approve", walletHandler.ApprovePurchase)
			admin.POST("/admin/purchase-requests/:requestId/reject", walletHandler.RejectPurchase)

			// ===== PLATFORM ANALYTICS =====
			admin.GET("/admin/stats", func(c *gin.Context) {
				c.JSON(200, gin.H{
					"message": "Platform statistics endpoint",
					"features": gin.H{
						"videos":        "enabled",
						"dramas":        "enabled",
						"verified_only": true,
					},
					"status": "operational",
				})
			})

			// ===== SYSTEM HEALTH =====
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
						"name":     "video-social-media-with-dramas",
						"version":  "1.0.0",
						"status":   "healthy",
						"features": []string{"videos", "dramas", "wallet", "social"},
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
					"features": gin.H{
						"videos": gin.H{
							"enabled":      true,
							"creation":     "all authenticated users",
							"interactions": "likes, comments, shares, follows",
						},
						"dramas": gin.H{
							"enabled":      true,
							"creation":     "verified users only",
							"monetization": "coin-based unlocking",
							"unlock_cost":  99,
						},
						"wallet": gin.H{
							"enabled":      true,
							"transactions": true,
							"purchases":    "admin approval required",
						},
					},
				})
			})

			debug.GET("/features", func(c *gin.Context) {
				c.JSON(200, gin.H{
					"video_endpoints": gin.H{
						"public": []string{
							"GET /videos - list videos",
							"GET /videos/featured - featured videos",
							"GET /videos/trending - trending videos",
							"GET /videos/:id - get specific video",
							"GET /videos/:id/comments - video comments",
						},
						"authenticated": []string{
							"POST /videos - create video",
							"PUT /videos/:id - update video",
							"DELETE /videos/:id - delete video",
							"POST /videos/:id/like - like video",
							"POST /videos/:id/comments - add comment",
						},
					},
					"drama_endpoints": gin.H{
						"public": []string{
							"GET /dramas - list dramas",
							"GET /dramas/featured - featured dramas",
							"GET /dramas/trending - trending dramas",
							"GET /dramas/:id - get specific drama",
							"GET /dramas/:id/episodes - drama episodes",
							"GET /dramas/:id/episodes/:num - specific episode",
						},
						"authenticated": []string{
							"POST /dramas/:id/favorite - toggle favorite",
							"POST /dramas/:id/progress - update progress",
							"POST /dramas/unlock - unlock premium drama",
							"GET /my/dramas/favorites - user favorites",
							"GET /my/dramas/continue-watching - continue watching",
						},
						"verified_users": []string{
							"POST /dramas - create drama (verified only)",
							"PUT /dramas/:id - update drama (owner only)",
							"DELETE /dramas/:id - delete drama (owner only)",
							"GET /my/dramas - my created dramas",
							"GET /my/dramas/analytics - revenue analytics",
						},
					},
					"auth_flow": gin.H{
						"public_sync":    "POST /auth/sync (no auth required - for new users)",
						"protected_sync": "POST /auth/profile-sync (auth required - for updates)",
						"get_user":       "GET /auth/user (auth required)",
						"verify_token":   "POST /auth/verify (no auth required)",
					},
					"permission_levels": gin.H{
						"public":        "view content, no auth required",
						"authenticated": "create videos, interact with content",
						"verified":      "create dramas + all authenticated features",
						"admin":         "moderate content, manage users, approve purchases",
					},
				})
			})
		}
	}
}
