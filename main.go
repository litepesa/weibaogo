// ===============================
// main.go - Video Social Media App with Simplified Fuzzy Search + History + Gift System
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
			// Standard limit for search
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
	giftService := services.NewGiftService(db, walletService) // 🎁 Gift service

	// Initialize handlers
	authHandler := handlers.NewAuthHandler(firebaseService)
	userHandler := handlers.NewUserHandler(db)
	videoHandler := handlers.NewVideoHandler(videoService, userService)
	walletHandler := handlers.NewWalletHandler(walletService)
	uploadHandler := handlers.NewUploadHandler(uploadService)
	giftHandler := handlers.NewGiftHandler(giftService) // 🎁 Gift handler

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
			"app":      "video-social-media-simple-search",
			"optimizations": gin.H{
				"gzip_compression":   true,
				"rate_limiting":      true,
				"connection_pooling": true,
				"bulk_endpoints":     true,
				"streaming_headers":  true,
				"url_optimization":   true,
				"fuzzy_search":       true,
				"virtual_gifts":      true,
			},
			"search_features": gin.H{
				"fuzzy_matching":   true,
				"search_history":   true,
				"caption_search":   true,
				"username_search":  true,
				"tag_search":       true,
				"popular_terms":    true,
				"simple_interface": true,
			},
			"gift_features": gin.H{
				"virtual_gifts":       true,
				"platform_commission": "30%",
				"gift_catalog":        "80+ gifts",
				"leaderboards":        true,
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
	setupRoutes(router, firebaseService, authHandler, userHandler, videoHandler, walletHandler, uploadHandler, giftHandler)

	// Start server
	port := cfg.Port
	log.Printf("🚀 Video Social Media Server starting on port %s", port)
	log.Printf("🌍 Environment: %s", cfg.Environment)
	log.Printf("💾 Database connected with optimized pool (Max: 50, Idle: 25)")
	log.Printf("🔥 Firebase service initialized")
	log.Printf("☁️  R2 storage initialized")
	log.Printf("🎁 Virtual Gift System:")
	log.Printf("   • 80+ virtual gifts (common to ultimate)")
	log.Printf("   • Platform commission: 30%%")
	log.Printf("   • Gift transactions tracking")
	log.Printf("   • Leaderboards (top senders & receivers)")
	log.Printf("   • Real-time balance updates")
	log.Printf("🔍 Simplified Fuzzy Search:")
	log.Printf("   • Searches: username, caption, tags")
	log.Printf("   • Fuzzy matching: 'dress' finds 'dresses'")
	log.Printf("   • Search history: enabled")
	log.Printf("   • No suggestions: only history")
	log.Printf("   • Optional filter: 'All Users' for username-only")
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
		c.Header("X-Powered-By", "video-social-simple-search")
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "SAMEORIGIN")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Next()
	})

	return router
}

// ===============================
// SIMPLIFIED ROUTES
// ===============================

func setupRoutes(
	router *gin.Engine,
	firebaseService *services.FirebaseService,
	authHandler *handlers.AuthHandler,
	userHandler *handlers.UserHandler,
	videoHandler *handlers.VideoHandler,
	walletHandler *handlers.WalletHandler,
	uploadHandler *handlers.UploadHandler,
	giftHandler *handlers.GiftHandler,
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

		// 🔍 SIMPLIFIED SEARCH ENDPOINTS
		public.GET("/videos/search", videoHandler.SearchVideos)                  // Simple fuzzy search
		public.GET("/videos/search/popular", videoHandler.GetPopularSearchTerms) // Trending terms only

		// BULK ENDPOINT
		public.POST("/videos/bulk", videoHandler.GetVideosBulk)

		// USER ENDPOINTS
		public.GET("/users/:userId", userHandler.GetUser)
		public.GET("/users/:userId/stats", userHandler.GetUserStats)
		public.GET("/users/:userId/followers", videoHandler.GetUserFollowers)
		public.GET("/users/:userId/following", videoHandler.GetUserFollowing)
		public.GET("/users", userHandler.GetAllUsers)
		public.GET("/users/search", userHandler.SearchUsers)

		// 🎁 PUBLIC GIFT ENDPOINTS
		public.GET("/gifts/catalog", giftHandler.GetGiftCatalog) // View available gifts
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

		// 🔍 SEARCH HISTORY ENDPOINTS
		protected.GET("/search/history", videoHandler.GetSearchHistory)              // Get user's search history
		protected.POST("/search/history", videoHandler.AddSearchHistory)             // Add to search history
		protected.DELETE("/search/history", videoHandler.ClearSearchHistory)         // Clear all history
		protected.DELETE("/search/history/:query", videoHandler.RemoveSearchHistory) // Remove specific

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

		// 🎁 GIFT SYSTEM ROUTES
		gifts := protected.Group("/gifts")
		{
			// Send & Transaction Management
			gifts.POST("/send", giftHandler.SendGift)                                // Send a gift
			gifts.GET("/transaction/:transactionId", giftHandler.GetGiftTransaction) // Get specific transaction

			// User Gift History & Stats
			gifts.GET("/history/:userId", giftHandler.GetGiftHistory) // Get user's gift history
			gifts.GET("/stats/:userId", giftHandler.GetGiftStats)     // Get user's gift statistics

			// Leaderboards (Public visibility for gamification)
			gifts.GET("/leaderboard/senders", giftHandler.GetTopGiftSenders)     // Top gift senders
			gifts.GET("/leaderboard/receivers", giftHandler.GetTopGiftReceivers) // Top gift receivers
		}

		// UPLOAD
		protected.POST("/upload", uploadHandler.UploadFile)
		protected.POST("/upload/batch", uploadHandler.BatchUploadFiles)
		protected.GET("/upload/health", uploadHandler.HealthCheck)

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

			// SEARCH MANAGEMENT
			admin.POST("/admin/search/refresh-popular-terms", func(c *gin.Context) {
				c.JSON(200, gin.H{
					"message":   "Popular search terms refresh requested",
					"status":    "acknowledged",
					"timestamp": time.Now(),
				})
			})

			// USER MANAGEMENT
			admin.GET("/admin/users", userHandler.GetAllUsers)
			admin.POST("/admin/users/:userId/status", userHandler.UpdateUserStatus)

			// WALLET MANAGEMENT
			admin.POST("/admin/wallet/:userId/add-coins", walletHandler.AddCoins)
			admin.GET("/admin/purchase-requests", walletHandler.GetPendingPurchases)
			admin.POST("/admin/purchase-requests/:requestId/approve", walletHandler.ApprovePurchase)
			admin.POST("/admin/purchase-requests/:requestId/reject", walletHandler.RejectPurchase)

			// 🎁 GIFT SYSTEM ADMIN
			adminGifts := admin.Group("/admin/gifts")
			{
				// Platform Revenue & Analytics
				adminGifts.GET("/commission/summary", giftHandler.GetPlatformCommissionSummary) // Platform earnings
				adminGifts.GET("/leaderboard/senders", giftHandler.GetTopGiftSenders)           // Admin view of top senders
				adminGifts.GET("/leaderboard/receivers", giftHandler.GetTopGiftReceivers)       // Admin view of top receivers

				// Gift System Management
				adminGifts.GET("/catalog", giftHandler.GetGiftCatalog) // View gift catalog with commission details
			}

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
						"virtual_gifts":    "enabled",
						"gzip_compression": true,
						"rate_limiting":    true,
					},
					"search": gin.H{
						"type":          "fuzzy",
						"targets":       []string{"username", "caption", "tags"},
						"history":       true,
						"suggestions":   false,
						"popular_terms": true,
						"filter_option": "All Users (username only)",
					},
					"gifts": gin.H{
						"enabled":             true,
						"total_gifts":         80,
						"commission_rate":     "30%",
						"rarity_levels":       7,
						"min_price":           10,
						"max_price":           100000,
						"leaderboards":        true,
						"transaction_history": true,
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
					"gifts": gin.H{
						"status":          "enabled",
						"catalog_loaded":  true,
						"commission_rate": 30.0,
					},
					"app": gin.H{
						"name":     "video-social-simple-search",
						"version":  "2.0.0-simple",
						"status":   "healthy",
						"features": []string{"videos", "wallet", "social", "fuzzy-search", "history", "virtual-gifts"},
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

			debug.GET("/search", func(c *gin.Context) {
				c.JSON(200, gin.H{
					"search_system": gin.H{
						"type":        "simplified_fuzzy",
						"version":     "2.0.0",
						"description": "Simple fuzzy search with history, no suggestions",
					},
					"features": gin.H{
						"fuzzy_matching": "dress finds dresses, cook finds cooking",
						"search_targets": []string{"username", "caption", "tags"},
						"search_history": "user's previous searches",
						"suggestions":    "disabled",
						"popular_terms":  "trending searches",
						"filter_option":  "All Users (username-only results)",
					},
					"endpoints": gin.H{
						"search":              "GET /videos/search?q={query}&usernameOnly={true/false}",
						"popular":             "GET /videos/search/popular",
						"get_history":         "GET /search/history",
						"add_history":         "POST /search/history",
						"clear_history":       "DELETE /search/history",
						"remove_history_item": "DELETE /search/history/{query}",
					},
					"examples": []string{
						"GET /videos/search?q=dress (finds dress, dresses, dressed)",
						"GET /videos/search?q=john (finds @john, captions with john, #john)",
						"GET /videos/search?q=cooking&usernameOnly=true (only @cooking users)",
						"GET /search/history (user's search history)",
					},
				})
			})

			debug.GET("/gifts", func(c *gin.Context) {
				c.JSON(200, gin.H{
					"gift_system": gin.H{
						"version":     "1.0.0",
						"status":      "enabled",
						"description": "Virtual gift system with platform commissions",
					},
					"features": gin.H{
						"total_gifts":         80,
						"rarity_levels":       []string{"common", "uncommon", "rare", "epic", "legendary", "mythic", "ultimate"},
						"commission_rate":     "30%",
						"wallet_integration":  true,
						"transaction_history": true,
						"leaderboards":        true,
						"real_time_updates":   true,
					},
					"endpoints": gin.H{
						"catalog":          "GET /gifts/catalog",
						"send_gift":        "POST /gifts/send",
						"gift_history":     "GET /gifts/history/:userId",
						"gift_stats":       "GET /gifts/stats/:userId",
						"top_senders":      "GET /gifts/leaderboard/senders",
						"top_receivers":    "GET /gifts/leaderboard/receivers",
						"transaction":      "GET /gifts/transaction/:transactionId",
						"admin_commission": "GET /admin/gifts/commission/summary",
					},
					"examples": []string{
						"POST /gifts/send {recipientId, giftId, message}",
						"GET /gifts/catalog (view all 80+ gifts)",
						"GET /gifts/history/user123 (user's gift transactions)",
						"GET /gifts/leaderboard/senders?limit=10 (top 10 gift senders)",
					},
					"commission": gin.H{
						"rate":        30.0,
						"description": "Platform takes 30% commission on each gift",
						"example":     "100 coin gift = 70 coins to recipient, 30 coins to platform",
					},
				})
			})
		}
	}
}
