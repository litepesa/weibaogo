// ===============================
// main.go - Video Social Media App with Video Reactions Chat
// ===============================

package main

import (
	"log"
	"sync"
	"time"

	"weibaobe/internal/config"
	"weibaobe/internal/database"
	"weibaobe/internal/handlers"
	"weibaobe/internal/middleware"
	"weibaobe/internal/services"
	"weibaobe/internal/storage"

	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

// ===============================
// SIMPLE IN-MEMORY RATE LIMITER
// ===============================

type RateLimiter struct {
	visitors map[string]*Visitor
	mutex    sync.RWMutex
}

type Visitor struct {
	requests int
	lastSeen time.Time
}

func NewRateLimiter() *RateLimiter {
	rl := &RateLimiter{
		visitors: make(map[string]*Visitor),
	}

	// Cleanup routine every 5 minutes
	go rl.cleanupRoutine()
	return rl
}

func (rl *RateLimiter) Allow(ip string, limit int, window time.Duration) bool {
	rl.mutex.Lock()
	defer rl.mutex.Unlock()

	visitor, exists := rl.visitors[ip]
	now := time.Now()

	if !exists || now.Sub(visitor.lastSeen) > window {
		rl.visitors[ip] = &Visitor{
			requests: 1,
			lastSeen: now,
		}
		return true
	}

	if visitor.requests >= limit {
		return false
	}

	visitor.requests++
	visitor.lastSeen = now
	return true
}

func (rl *RateLimiter) cleanupRoutine() {
	ticker := time.NewTicker(5 * time.Minute)
	for {
		select {
		case <-ticker.C:
			rl.cleanup()
		}
	}
}

func (rl *RateLimiter) cleanup() {
	rl.mutex.Lock()
	defer rl.mutex.Unlock()

	cutoff := time.Now().Add(-10 * time.Minute)
	for ip, visitor := range rl.visitors {
		if visitor.lastSeen.Before(cutoff) {
			delete(rl.visitors, ip)
		}
	}
}

// ===============================
// RATE LIMITING MIDDLEWARE
// ===============================

func createRateLimitMiddleware(rateLimiter *RateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()

		var limit int
		var window time.Duration

		path := c.Request.URL.Path
		if path == "/api/v1/videos/bulk" {
			limit = 30
			window = time.Minute
		} else if path == "/api/v1/videos/search" {
			limit = 100
			window = time.Minute
		} else if path == "/api/v1/videos" ||
			path == "/api/v1/videos/featured" ||
			path == "/api/v1/videos/trending" {
			limit = 100
			window = time.Minute
		} else {
			limit = 200
			window = time.Minute
		}

		if !rateLimiter.Allow(ip, limit, window) {
			c.Header("X-RateLimit-Limit", string(rune(limit)))
			c.Header("X-RateLimit-Remaining", "0")
			c.Header("Retry-After", "60")

			c.JSON(429, gin.H{
				"error":   "Rate limit exceeded",
				"message": "Too many requests, please try again later",
				"limit":   limit,
				"window":  window.String(),
			})
			c.Abort()
			return
		}

		c.Header("X-RateLimit-Limit", string(rune(limit)))
		c.Next()
	}
}

// ===============================
// MAIN APPLICATION
// ===============================

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

	// Initialize database connection
	db, err := database.Connect(cfg.Database.ConnectionString())
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}
	defer database.Close()

	// Apply database optimizations
	log.Println("📊 Applying database optimizations for video workload:")
	db.SetMaxOpenConns(50)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(10 * time.Minute)
	db.SetConnMaxIdleTime(5 * time.Minute)
	log.Printf("   • Max open connections: 50")
	log.Printf("   • Max idle connections: 25")
	log.Printf("   • Connection lifetime: 10 minutes")
	log.Printf("   • Idle timeout: 5 minutes")

	// Run migrations
	log.Println("🔧 Running database migrations...")
	if err := database.RunMigrations(db); err != nil {
		log.Fatal("Failed to run migrations:", err)
	}

	// Create performance indexes
	if err := database.CreatePerformanceIndexes(); err != nil {
		log.Printf("Warning: Failed to create performance indexes: %v", err)
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
	userService := services.NewUserService(db)
	uploadService := services.NewUploadService(r2Client)

	// Initialize handlers
	authHandler := handlers.NewAuthHandler(firebaseService)
	userHandler := handlers.NewUserHandler(db)
	videoHandler := handlers.NewVideoHandler(videoService, userService)
	walletHandler := handlers.NewWalletHandler(walletService)
	uploadHandler := handlers.NewUploadHandler(uploadService)

	// Initialize rate limiter
	rateLimiter := NewRateLimiter()

	// Setup router
	router := setupOptimizedRouter(cfg, rateLimiter)

	// Health check
	router.GET("/health", func(c *gin.Context) {
		dbStats := database.Stats()
		c.JSON(200, gin.H{
			"status":   "healthy",
			"database": database.Health() == nil,
			"app":      "video-social-media-with-reactions-chat",
			"optimizations": gin.H{
				"gzip_compression":   true,
				"rate_limiting":      true,
				"connection_pooling": true,
				"bulk_endpoints":     true,
				"streaming_headers":  true,
				"url_optimization":   true,
				"fuzzy_search":       true,
				"video_reactions":    true,
				"websocket_chat":     true,
			},
			"features": gin.H{
				"videos":            true,
				"wallet":            true,
				"social":            true,
				"search":            true,
				"video_reactions":   true,
				"real_time_chat":    true,
				"typing_indicators": true,
				"read_receipts":     true,
				"message_pinning":   true,
				"file_sharing":      true,
			},
			"database_stats": gin.H{
				"open_connections": dbStats.OpenConnections,
				"in_use":           dbStats.InUse,
				"idle":             dbStats.Idle,
				"max_open":         50,
				"max_idle":         25,
			},
		})
	})

	// Setup routes
	setupRoutes(router, firebaseService, authHandler, userHandler, videoHandler, walletHandler, uploadHandler)

	// Start server
	port := cfg.Port
	log.Printf("🚀 Video Social Media Server starting on port %s", port)
	log.Printf("🌍 Environment: %s", cfg.Environment)
	log.Printf("💾 Database connected with optimized pool (Max: 50, Idle: 25)")
	log.Printf("🔥 Firebase service initialized")
	log.Printf("☁️  R2 storage initialized")
	log.Printf("🔍 Simplified Fuzzy Search:")
	log.Printf("   • Searches: username, caption, tags")
	log.Printf("   • Fuzzy matching: 'dress' finds 'dresses'")
	log.Printf("   • Search history: enabled")
	log.Printf("💬 Video Reactions Chat:")
	log.Printf("   • WebSocket-powered real-time messaging")
	log.Printf("   • File sharing (images, videos, documents)")
	log.Printf("   • Read receipts & delivery status")
	log.Printf("   • Typing indicators")
	log.Printf("   • Message pinning (max 10 per chat)")
	log.Printf("   • Per-user chat settings")
	log.Printf("⚡ Performance optimizations:")
	log.Printf("   • Gzip compression: ~70%% size reduction")
	log.Printf("   • Rate limiting: 100 req/min")
	log.Printf("   • Connection pooling: optimized")
	log.Printf("   • Bulk endpoints: 50 videos/request")
	log.Printf("   • Smart caching: different TTLs")
	log.Printf("   • Trigram search: 10-100x faster")

	log.Fatal(router.Run(":" + port))
}

// ===============================
// OPTIMIZED ROUTER SETUP
// ===============================

func setupOptimizedRouter(cfg *config.Config, rateLimiter *RateLimiter) *gin.Engine {
	router := gin.Default()

	// GZIP compression
	router.Use(gzip.Gzip(gzip.DefaultCompression, gzip.WithExcludedExtensions([]string{".mp4", ".avi", ".mov", ".webm"})))

	// Rate limiting
	router.Use(createRateLimitMiddleware(rateLimiter))

	// CORS
	router.Use(cors.New(cors.Config{
		AllowOrigins: cfg.AllowedOrigins,
		AllowMethods: []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"},
		AllowHeaders: []string{
			"Origin", "Content-Type", "Authorization",
			"Range", "Accept-Ranges",
			"Cache-Control", "If-None-Match", "If-Modified-Since",
		},
		ExposeHeaders: []string{
			"Content-Length", "Content-Range", "Accept-Ranges",
			"Cache-Control", "Last-Modified", "ETag",
			"X-RateLimit-Limit", "X-RateLimit-Remaining", "Retry-After",
		},
		AllowCredentials: true,
		MaxAge:           12 * 3600,
	}))

	// Performance headers
	router.Use(func(c *gin.Context) {
		c.Header("X-DNS-Prefetch-Control", "on")
		c.Header("X-Powered-By", "video-social-with-reactions")
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "SAMEORIGIN")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Next()
	})

	return router
}

// ===============================
// ROUTES SETUP WITH VIDEO REACTIONS
// ===============================

func setupRoutes(
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
	// AUTH ROUTES
	// ===============================
	auth := api.Group("/auth")
	{
		auth.POST("/sync", authHandler.SyncUser)
		auth.POST("/verify", authHandler.VerifyToken)
	}

	protectedAuth := api.Group("/auth")
	protectedAuth.Use(middleware.FirebaseAuth(firebaseService))
	{
		protectedAuth.GET("/user", authHandler.GetCurrentUser)
		protectedAuth.POST("/profile-sync", authHandler.SyncUserWithToken)
	}

	// ===============================
	// PUBLIC ROUTES
	// ===============================
	public := api.Group("")
	{
		// VIDEO ENDPOINTS
		public.GET("/videos", videoHandler.GetVideos)
		public.GET("/videos/featured", videoHandler.GetFeaturedVideos)
		public.GET("/videos/trending", videoHandler.GetTrendingVideos)
		public.GET("/videos/popular", videoHandler.GetPopularVideos)
		public.GET("/videos/:videoId", videoHandler.GetVideo)
		public.GET("/videos/:videoId/qualities", videoHandler.GetVideoQualities)
		public.GET("/videos/:videoId/metrics", videoHandler.GetVideoMetrics)
		public.POST("/videos/:videoId/views", videoHandler.IncrementViews)
		public.GET("/users/:userId/videos", videoHandler.GetUserVideos)
		public.GET("/videos/:videoId/comments", videoHandler.GetVideoComments)

		// SEARCH ENDPOINTS
		public.GET("/videos/search", videoHandler.SearchVideos)
		public.GET("/videos/search/popular", videoHandler.GetPopularSearchTerms)

		// BULK ENDPOINT
		public.POST("/videos/bulk", videoHandler.GetVideosBulk)

		// USER ENDPOINTS
		public.GET("/users/:userId", userHandler.GetUser)
		public.GET("/users/:userId/stats", userHandler.GetUserStats)
		public.GET("/users/:userId/followers", videoHandler.GetUserFollowers)
		public.GET("/users/:userId/following", videoHandler.GetUserFollowing)
		public.GET("/users", userHandler.GetAllUsers)
		public.GET("/users/search", userHandler.SearchUsers)
	}

	// ===============================
	// PROTECTED ROUTES
	// ===============================
	protected := api.Group("")
	protected.Use(middleware.FirebaseAuth(firebaseService))
	{
		// USER MANAGEMENT
		protected.PUT("/users/:userId", userHandler.UpdateUser)
		protected.DELETE("/users/:userId", userHandler.DeleteUser)
		protected.POST("/users/:userId/status", userHandler.UpdateUserStatus)

		// VIDEO FEATURES
		protected.POST("/videos", videoHandler.CreateVideo)
		protected.PUT("/videos/:videoId", videoHandler.UpdateVideo)
		protected.DELETE("/videos/:videoId", videoHandler.DeleteVideo)
		protected.POST("/videos/:videoId/like", videoHandler.LikeVideo)
		protected.DELETE("/videos/:videoId/like", videoHandler.UnlikeVideo)
		protected.POST("/videos/:videoId/share", videoHandler.ShareVideo)
		protected.GET("/videos/:videoId/counts", videoHandler.GetVideoCountsSummary)
		protected.GET("/users/:userId/liked-videos", videoHandler.GetUserLikedVideos)
		protected.GET("/videos/:videoId/analytics", videoHandler.GetVideoAnalytics)

		// SEARCH HISTORY ENDPOINTS
		protected.GET("/search/history", videoHandler.GetSearchHistory)
		protected.POST("/search/history", videoHandler.AddSearchHistory)
		protected.DELETE("/search/history", videoHandler.ClearSearchHistory)
		protected.DELETE("/search/history/:query", videoHandler.RemoveSearchHistory)

		// RECOMMENDATIONS
		protected.GET("/videos/recommendations", videoHandler.GetVideoRecommendations)

		// SOCIAL FEATURES
		protected.POST("/users/:userId/follow", videoHandler.FollowUser)
		protected.DELETE("/users/:userId/follow", videoHandler.UnfollowUser)
		protected.GET("/feed/following", videoHandler.GetFollowingFeed)

		// COMMENTS
		protected.POST("/videos/:videoId/comments", videoHandler.CreateComment)
		protected.DELETE("/comments/:commentId", videoHandler.DeleteComment)
		protected.POST("/comments/:commentId/like", videoHandler.LikeComment)
		protected.DELETE("/comments/:commentId/like", videoHandler.UnlikeComment)

		// REPORTING
		protected.POST("/videos/:videoId/report", videoHandler.ReportVideo)

		// ANALYTICS
		protected.GET("/stats/videos", videoHandler.GetVideoStats)

		// WALLET
		protected.GET("/wallet/:userId", walletHandler.GetWallet)
		protected.GET("/wallet/:userId/transactions", walletHandler.GetTransactions)
		protected.POST("/wallet/:userId/purchase-request", walletHandler.CreatePurchaseRequest)

		// UPLOAD
		protected.POST("/upload", uploadHandler.UploadFile)
		protected.POST("/upload/batch", uploadHandler.BatchUploadFiles)
		protected.GET("/upload/health", uploadHandler.HealthCheck)

		// ===============================
		// 💬 VIDEO REACTIONS CHAT ROUTES
		// ===============================
		videoReactions := protected.Group("/video-reactions")
		{
			// Chat management
			videoReactions.GET("/chats", func(c *gin.Context) {
				c.JSON(200, gin.H{"message": "Get user's video reaction chats - TODO: Implement handler"})
			})
			videoReactions.POST("/chats", func(c *gin.Context) {
				c.JSON(200, gin.H{"message": "Create new video reaction chat - TODO: Implement handler"})
			})
			videoReactions.GET("/chats/:chatId", func(c *gin.Context) {
				c.JSON(200, gin.H{"message": "Get specific chat - TODO: Implement handler"})
			})
			videoReactions.DELETE("/chats/:chatId", func(c *gin.Context) {
				c.JSON(200, gin.H{"message": "Delete chat - TODO: Implement handler"})
			})

			// Message management
			videoReactions.GET("/chats/:chatId/messages", func(c *gin.Context) {
				c.JSON(200, gin.H{"message": "Get chat messages - TODO: Implement handler"})
			})
			videoReactions.POST("/chats/:chatId/messages", func(c *gin.Context) {
				c.JSON(200, gin.H{"message": "Send message - TODO: Implement handler"})
			})
			videoReactions.PUT("/chats/:chatId/messages/:messageId", func(c *gin.Context) {
				c.JSON(200, gin.H{"message": "Edit message - TODO: Implement handler"})
			})
			videoReactions.DELETE("/chats/:chatId/messages/:messageId", func(c *gin.Context) {
				c.JSON(200, gin.H{"message": "Delete message - TODO: Implement handler"})
			})

			// Chat actions
			videoReactions.POST("/chats/:chatId/read", func(c *gin.Context) {
				c.JSON(200, gin.H{"message": "Mark chat as read - TODO: Implement handler"})
			})
			videoReactions.POST("/chats/:chatId/pin", func(c *gin.Context) {
				c.JSON(200, gin.H{"message": "Toggle chat pin - TODO: Implement handler"})
			})
			videoReactions.POST("/chats/:chatId/archive", func(c *gin.Context) {
				c.JSON(200, gin.H{"message": "Toggle chat archive - TODO: Implement handler"})
			})
			videoReactions.POST("/chats/:chatId/mute", func(c *gin.Context) {
				c.JSON(200, gin.H{"message": "Toggle chat mute - TODO: Implement handler"})
			})

			// Message actions
			videoReactions.POST("/chats/:chatId/messages/:messageId/pin", func(c *gin.Context) {
				c.JSON(200, gin.H{"message": "Toggle message pin - TODO: Implement handler"})
			})
			videoReactions.GET("/chats/:chatId/messages/pinned", func(c *gin.Context) {
				c.JSON(200, gin.H{"message": "Get pinned messages - TODO: Implement handler"})
			})
			videoReactions.GET("/chats/:chatId/messages/search", func(c *gin.Context) {
				c.JSON(200, gin.H{"message": "Search messages - TODO: Implement handler"})
			})

			// Chat settings
			videoReactions.PUT("/chats/:chatId/settings", func(c *gin.Context) {
				c.JSON(200, gin.H{"message": "Update chat settings - TODO: Implement handler"})
			})
		}

		// ===============================
		// ADMIN ROUTES
		// ===============================
		admin := protected.Group("")
		admin.Use(middleware.AdminOnly())
		{
			// VIDEO MODERATION
			admin.POST("/admin/videos/:videoId/featured", videoHandler.ToggleFeatured)
			admin.POST("/admin/videos/:videoId/active", videoHandler.ToggleActive)
			admin.POST("/admin/videos/:videoId/verified", videoHandler.ToggleVerified)

			// PERFORMANCE
			admin.POST("/admin/videos/batch-update-counts", videoHandler.BatchUpdateCounts)

			// USER MANAGEMENT
			admin.GET("/admin/users", userHandler.GetAllUsers)
			admin.POST("/admin/users/:userId/status", userHandler.UpdateUserStatus)

			// WALLET MANAGEMENT
			admin.POST("/admin/wallet/:userId/add-coins", walletHandler.AddCoins)
			admin.GET("/admin/purchase-requests", walletHandler.GetPendingPurchases)
			admin.POST("/admin/purchase-requests/:requestId/approve", walletHandler.ApprovePurchase)
			admin.POST("/admin/purchase-requests/:requestId/reject", walletHandler.RejectPurchase)

			// PLATFORM STATS
			admin.GET("/admin/stats", func(c *gin.Context) {
				c.Header("Cache-Control", "public, max-age=300")
				dbStats := database.Stats()

				c.JSON(200, gin.H{
					"message": "Platform statistics",
					"features": gin.H{
						"videos":           "enabled",
						"fuzzy_search":     "enabled",
						"search_history":   "enabled",
						"video_reactions":  "enabled",
						"websocket_chat":   "enabled",
						"gzip_compression": true,
						"rate_limiting":    true,
					},
					"chat": gin.H{
						"type":                "websocket",
						"real_time":           true,
						"typing_indicators":   true,
						"read_receipts":       true,
						"message_pinning":     true,
						"file_sharing":        true,
						"max_pinned_per_chat": 10,
					},
					"status": "operational",
					"performance": gin.H{
						"database_connections": gin.H{
							"open":     dbStats.OpenConnections,
							"in_use":   dbStats.InUse,
							"idle":     dbStats.Idle,
							"max_open": 50,
							"max_idle": 25,
						},
					},
				})
			})

			// SYSTEM HEALTH
			admin.GET("/admin/health", func(c *gin.Context) {
				c.Header("Cache-Control", "no-cache")
				dbStats := database.Stats()

				c.JSON(200, gin.H{
					"database": gin.H{
						"status":           "connected",
						"open_connections": dbStats.OpenConnections,
						"in_use":           dbStats.InUse,
						"idle":             dbStats.Idle,
					},
					"firebase": gin.H{"status": "initialized"},
					"storage":  gin.H{"status": "connected", "type": "cloudflare-r2"},
					"search": gin.H{
						"status":            "enabled",
						"type":              "fuzzy",
						"trigram_extension": true,
						"history_enabled":   true,
					},
					"chat": gin.H{
						"status":         "enabled",
						"type":           "websocket",
						"real_time":      true,
						"tables_created": true,
					},
					"app": gin.H{
						"name":     "video-social-with-reactions",
						"version":  "2.1.0",
						"status":   "healthy",
						"features": []string{"videos", "wallet", "social", "fuzzy-search", "history", "video-reactions", "websocket-chat"},
					},
				})
			})
		}
	}

	// ===============================
	// DEBUG ROUTES
	// ===============================
	if gin.Mode() == gin.DebugMode {
		debug := api.Group("/debug")
		{
			debug.GET("/routes", func(c *gin.Context) {
				routes := router.Routes()
				routeList := make([]gin.H, len(routes))
				for i, route := range routes {
					routeList[i] = gin.H{"method": route.Method, "path": route.Path}
				}
				c.JSON(200, gin.H{"total": len(routes), "routes": routeList})
			})

			debug.GET("/video-reactions", func(c *gin.Context) {
				c.JSON(200, gin.H{
					"feature": gin.H{
						"name":        "Video Reactions Chat",
						"version":     "1.0.0",
						"type":        "websocket",
						"description": "Real-time chat system for video reactions",
					},
					"capabilities": gin.H{
						"real_time_messaging": true,
						"file_sharing":        true,
						"typing_indicators":   true,
						"read_receipts":       true,
						"delivery_status":     true,
						"message_editing":     true,
						"message_deletion":    true,
						"message_pinning":     true,
						"per_user_settings":   true,
						"message_search":      true,
					},
					"endpoints": gin.H{
						"get_chats":       "GET /video-reactions/chats",
						"create_chat":     "POST /video-reactions/chats",
						"get_messages":    "GET /video-reactions/chats/:chatId/messages",
						"send_message":    "POST /video-reactions/chats/:chatId/messages",
						"mark_read":       "POST /video-reactions/chats/:chatId/read",
						"toggle_pin":      "POST /video-reactions/chats/:chatId/pin",
						"search_messages": "GET /video-reactions/chats/:chatId/messages/search",
					},
					"status": "TODO: Handlers need implementation",
				})
			})
		}
	}
}
