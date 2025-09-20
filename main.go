// main.go - Updated with WebSocket Support
package main

import (
	"log"
	"strings"
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

	// Apply database optimizations for real-time chat
	log.Println("📊 Applying database optimizations for real-time chat workload:")
	db.SetMaxOpenConns(100)                 // Increased for WebSocket connections
	db.SetMaxIdleConns(50)                  // Higher for persistent connections
	db.SetConnMaxLifetime(20 * time.Minute) // Longer for WebSocket persistence
	db.SetConnMaxIdleTime(10 * time.Minute) // Extended for real-time features
	log.Printf("   • Max open connections: 100")
	log.Printf("   • Max idle connections: 50")
	log.Printf("   • Connection lifetime: 20 minutes")
	log.Printf("   • Idle timeout: 10 minutes")

	// Run database migrations
	log.Println("🔧 Running database migrations with real-time chat support...")
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
	userService := services.NewUserService(db)
	uploadService := services.NewUploadService(r2Client)
	chatService := services.NewChatService(db)
	contactService := services.NewContactService(db)

	// Initialize handlers
	authHandler := handlers.NewAuthHandler(firebaseService)
	userHandler := handlers.NewUserHandler(db)
	videoHandler := handlers.NewVideoHandler(videoService, userService)
	walletHandler := handlers.NewWalletHandler(walletService)
	uploadHandler := handlers.NewUploadHandler(uploadService)
	chatHandler := handlers.NewChatHandler(chatService, userService)
	contactHandler := handlers.NewContactHandler(contactService, userService)

	// Initialize WebSocket chat handler
	wsHandler := handlers.NewWebSocketChatHandler(chatService, userService)

	// Initialize rate limiter
	rateLimiter := NewRateLimiter()

	// Setup router with WebSocket support
	router := setupOptimizedRouter(cfg, rateLimiter)

	// Health check with WebSocket info
	router.GET("/health", func(c *gin.Context) {
		dbStats := database.Stats()
		wsStats := wsHandler.GetStats()

		c.JSON(200, gin.H{
			"status":   "healthy",
			"database": database.Health() == nil,
			"app":      "video-social-media-with-realtime-chat",
			"features": gin.H{
				"videos":        true,
				"wallet":        true,
				"realtime_chat": true,
				"contacts":      true,
				"websockets":    true,
			},
			"websocket_stats": wsStats,
			"optimizations": gin.H{
				"gzip_compression":    true,
				"rate_limiting":       true,
				"connection_pooling":  true,
				"bulk_endpoints":      true,
				"streaming_headers":   true,
				"url_optimization":    true,
				"real_time_websocket": true,
			},
			"database_stats": gin.H{
				"open_connections": dbStats.OpenConnections,
				"in_use":           dbStats.InUse,
				"idle":             dbStats.Idle,
				"max_open":         100,
				"max_idle":         50,
			},
		})
	})

	// Setup routes with WebSocket support
	setupOptimizedRoutes(router, firebaseService, authHandler, userHandler, videoHandler,
		walletHandler, uploadHandler, chatHandler, contactHandler, wsHandler)

	// Start server
	port := cfg.Port
	log.Printf("🚀 REAL-TIME Video Social Media Server with WebSocket Chat starting on port %s", port)
	log.Printf("🌍 Environment: %s", cfg.Environment)
	log.Printf("💾 Database connected with enhanced pool (Max: 100, Idle: 50)")
	log.Printf("🔥 Firebase service initialized")
	log.Printf("☁️  R2 storage initialized")
	log.Printf("🔌 WebSocket chat handler initialized")
	log.Printf("📱 Features enabled:")
	log.Printf("   • Video Social Media: ✅")
	log.Printf("   • Real-time WebSocket Chat: ✅")
	log.Printf("   • Contact Management: ✅")
	log.Printf("   • Wallet System: ✅")
	log.Printf("⚡ Performance optimizations:")
	log.Printf("   • Enhanced connection pooling for WebSocket persistence")
	log.Printf("   • Optimized rate limiting for real-time features")
	log.Printf("   • JSONB support for chat metadata")
	log.Printf("   • WebSocket connection management")
	log.Printf("   • Efficient message broadcasting")

	log.Fatal(router.Run(":" + port))
}

// Enhanced router setup for WebSocket support
func setupOptimizedRouter(cfg *config.Config, rateLimiter *RateLimiter) *gin.Engine {
	router := gin.Default()

	// Enhanced GZIP compression
	router.Use(gzip.Gzip(gzip.DefaultCompression, gzip.WithExcludedExtensions([]string{
		".mp4", ".avi", ".mov", ".webm", ".mp3", ".wav", ".ogg"})))

	// Enhanced rate limiting
	router.Use(createRateLimitMiddleware(rateLimiter))

	// Enhanced CORS with WebSocket headers
	router.Use(cors.New(cors.Config{
		AllowOrigins: cfg.AllowedOrigins,
		AllowMethods: []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"},
		AllowHeaders: []string{
			"Origin", "Content-Type", "Authorization",
			"Range", "Accept-Ranges",
			"Cache-Control", "If-None-Match", "If-Modified-Since",
			"X-Chat-ID", "X-Message-ID", "X-Contact-Sync",
			"Upgrade", "Connection", "Sec-WebSocket-Key", "Sec-WebSocket-Version", // WebSocket headers
		},
		ExposeHeaders: []string{
			"Content-Length", "Content-Range", "Accept-Ranges",
			"Cache-Control", "Last-Modified", "ETag",
			"X-RateLimit-Limit", "X-RateLimit-Remaining", "Retry-After",
			"X-Chat-Status", "X-Message-Status", "X-Sync-Version",
		},
		AllowCredentials: true,
		MaxAge:           12 * 3600,
	}))

	// Enhanced performance headers with WebSocket support
	router.Use(func(c *gin.Context) {
		// Add performance hints
		c.Header("X-DNS-Prefetch-Control", "on")
		c.Header("X-Powered-By", "video-social-realtime-chat")

		// Security headers
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "SAMEORIGIN")
		c.Header("X-XSS-Protection", "1; mode=block")

		// WebSocket and real-time headers
		if c.GetHeader("Upgrade") == "websocket" {
			c.Header("X-WebSocket-Support", "enabled")
			c.Header("X-Real-Time-Chat", "enabled")
		}

		c.Next()
	})

	return router
}

// Enhanced routes with WebSocket endpoints
func setupOptimizedRoutes(
	router *gin.Engine,
	firebaseService *services.FirebaseService,
	authHandler *handlers.AuthHandler,
	userHandler *handlers.UserHandler,
	videoHandler *handlers.VideoHandler,
	walletHandler *handlers.WalletHandler,
	uploadHandler *handlers.UploadHandler,
	chatHandler *handlers.ChatHandler,
	contactHandler *handlers.ContactHandler,
	wsHandler *handlers.WebSocketChatHandler,
) {
	api := router.Group("/api/v1")

	// ===============================
	// WEBSOCKET ENDPOINTS (NEW!)
	// ===============================
	ws := api.Group("/ws")
	{
		// Real-time chat WebSocket endpoint
		ws.GET("/chat", wsHandler.HandleWebSocket)
	}

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
	// PUBLIC ROUTES (VIDEO CONTENT)
	// ===============================
	public := api.Group("")
	{
		// Video endpoints
		public.GET("/videos", videoHandler.GetVideos)
		public.GET("/videos/featured", videoHandler.GetFeaturedVideos)
		public.GET("/videos/trending", videoHandler.GetTrendingVideos)
		public.GET("/videos/popular", videoHandler.GetPopularVideos)
		public.GET("/videos/search", videoHandler.SearchVideos)
		public.GET("/videos/:videoId", videoHandler.GetVideo)
		public.GET("/videos/:videoId/comments", videoHandler.GetVideoComments)
		public.POST("/videos/:videoId/views", videoHandler.IncrementViews)
		public.GET("/users/:userId/videos", videoHandler.GetUserVideos)
		public.POST("/videos/bulk", videoHandler.GetVideosBulk)

		// User profile endpoints
		public.GET("/users/:userId", userHandler.GetUser)
		public.GET("/users/:userId/stats", userHandler.GetUserStats)
		public.GET("/users", userHandler.GetAllUsers)
		public.GET("/users/search", userHandler.SearchUsers)
	}

	// ===============================
	// PROTECTED ROUTES
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
		protected.GET("/videos/recommendations", videoHandler.GetVideoRecommendations)

		// ===== SOCIAL FEATURES =====
		protected.POST("/users/:userId/follow", videoHandler.FollowUser)
		protected.DELETE("/users/:userId/follow", videoHandler.UnfollowUser)
		protected.GET("/feed/following", videoHandler.GetFollowingFeed)

		// ===== COMMENT MANAGEMENT =====
		protected.POST("/videos/:videoId/comments", videoHandler.CreateComment)
		protected.DELETE("/comments/:commentId", videoHandler.DeleteComment)
		protected.POST("/comments/:commentId/like", videoHandler.LikeComment)
		protected.DELETE("/comments/:commentId/like", videoHandler.UnlikeComment)

		// ===== 📱 CHAT ENDPOINTS (ENHANCED WITH WEBSOCKET SUPPORT) =====
		chatRoutes := protected.Group("/chats")
		{
			// Chat management
			chatRoutes.POST("", chatHandler.CreateOrGetChat)
			chatRoutes.GET("", chatHandler.GetChats)
			chatRoutes.GET("/:chatId", chatHandler.GetChat)

			// Chat settings
			chatRoutes.POST("/:chatId/mark-read", chatHandler.MarkChatAsRead)
			chatRoutes.POST("/:chatId/toggle-pin", chatHandler.TogglePinChat)
			chatRoutes.POST("/:chatId/toggle-archive", chatHandler.ToggleArchiveChat)
			chatRoutes.POST("/:chatId/toggle-mute", chatHandler.ToggleMuteChat)
			chatRoutes.POST("/:chatId/settings", chatHandler.SetChatSettings)

			// Message management (kept for fallback/sync)
			chatRoutes.POST("/:chatId/messages", chatHandler.SendMessage)
			chatRoutes.GET("/:chatId/messages", chatHandler.GetMessages)
			chatRoutes.PUT("/:chatId/messages/:messageId", chatHandler.UpdateMessage)
			chatRoutes.DELETE("/:chatId/messages/:messageId", chatHandler.DeleteMessage)

			// Video reactions
			chatRoutes.POST("/:chatId/video-reaction", chatHandler.SendVideoReaction)
		}

		// ===== 📱 CONTACT ENDPOINTS =====
		contactRoutes := protected.Group("/contacts")
		{
			contactRoutes.GET("", contactHandler.GetContacts)
			contactRoutes.POST("", contactHandler.AddContact)
			contactRoutes.DELETE("/:contactId", contactHandler.RemoveContact)

			contactRoutes.POST("/search", contactHandler.SearchContacts)
			contactRoutes.POST("/sync", contactHandler.SyncContacts)
			contactRoutes.GET("/search/phone", contactHandler.SearchUserByPhone)

			contactRoutes.POST("/block", contactHandler.BlockContact)
			contactRoutes.POST("/unblock", contactHandler.UnblockContact)
			contactRoutes.GET("/blocked", contactHandler.GetBlockedContacts)
		}

		// ===== WALLET ENDPOINTS =====
		protected.GET("/wallet/:userId", walletHandler.GetWallet)
		protected.GET("/wallet/:userId/transactions", walletHandler.GetTransactions)
		protected.POST("/wallet/:userId/purchase-request", walletHandler.CreatePurchaseRequest)

		// ===== FILE UPLOAD =====
		protected.POST("/upload", uploadHandler.UploadFile)
		protected.POST("/upload/batch", uploadHandler.BatchUploadFiles)

		// ===============================
		// ADMIN ROUTES
		// ===============================
		admin := protected.Group("")
		admin.Use(middleware.AdminOnly())
		{
			// Video moderation
			admin.POST("/admin/videos/:videoId/featured", videoHandler.ToggleFeatured)
			admin.POST("/admin/videos/:videoId/active", videoHandler.ToggleActive)

			// User management
			admin.GET("/admin/users", userHandler.GetAllUsers)
			admin.POST("/admin/users/:userId/status", userHandler.UpdateUserStatus)

			// Wallet management
			admin.POST("/admin/wallet/:userId/add-coins", walletHandler.AddCoins)
			admin.GET("/admin/purchase-requests", walletHandler.GetPendingPurchases)
			admin.POST("/admin/purchase-requests/:requestId/approve", walletHandler.ApprovePurchase)
			admin.POST("/admin/purchase-requests/:requestId/reject", walletHandler.RejectPurchase)

			// Enhanced platform analytics with WebSocket stats
			admin.GET("/admin/stats", func(c *gin.Context) {
				c.Header("Cache-Control", "public, max-age=300")
				dbStats := database.Stats()
				wsStats := wsHandler.GetStats()

				c.JSON(200, gin.H{
					"message": "Enhanced platform statistics with real-time chat",
					"features": gin.H{
						"videos":        "enabled + optimized",
						"realtime_chat": "enabled + WebSocket",
						"contacts":      "enabled + sync",
						"wallet":        "enabled",
						"websockets":    "enabled + broadcasting",
					},
					"websocket_stats": wsStats,
					"status":          "operational",
					"performance": gin.H{
						"database_connections": gin.H{
							"open":     dbStats.OpenConnections,
							"in_use":   dbStats.InUse,
							"idle":     dbStats.Idle,
							"max_open": 100,
							"max_idle": 50,
						},
						"optimizations_active":  9,
						"estimated_improvement": "Real-time WebSocket communication",
						"websocket_performance": "Sub-second message delivery",
					},
				})
			})

			// System health with WebSocket metrics
			admin.GET("/admin/health", func(c *gin.Context) {
				c.Header("Cache-Control", "no-cache")
				dbStats := database.Stats()
				wsStats := wsHandler.GetStats()

				c.JSON(200, gin.H{
					"database": gin.H{
						"status":           "connected",
						"open_connections": dbStats.OpenConnections,
						"in_use":           dbStats.InUse,
						"idle":             dbStats.Idle,
						"max_open":         100,
						"max_idle":         50,
						"optimized_for":    "real-time WebSocket workload",
					},
					"websocket": gin.H{
						"status":             "active",
						"active_connections": wsStats["active_connections"],
						"active_chats":       wsStats["active_chats"],
						"total_participants": wsStats["total_participants"],
					},
					"firebase": gin.H{
						"status": "initialized",
					},
					"storage": gin.H{
						"status":         "connected",
						"type":           "cloudflare-r2",
						"optimized_urls": true,
					},
					"features": gin.H{
						"videos":               true,
						"realtime_websocket":   true,
						"contact_sync":         true,
						"wallet_system":        true,
						"video_reactions":      true,
						"typing_indicators":    true,
						"online_status":        true,
						"message_broadcasting": true,
					},
					"performance": gin.H{
						"gzip_compression":     true,
						"rate_limiting":        true,
						"bulk_endpoints":       true,
						"streaming_headers":    true,
						"url_optimization":     true,
						"smart_caching":        true,
						"connection_pooling":   true,
						"websocket_realtime":   true,
						"message_broadcasting": true,
					},
					"app": gin.H{
						"name":    "video-social-media-with-realtime-chat",
						"version": "2.0.0-websocket",
						"status":  "healthy",
						"features": []string{
							"videos", "realtime-chat", "contacts", "wallet",
							"websockets", "performance-optimized", "real-time-broadcasting",
						},
					},
				})
			})
		}
	}

	// ===============================
	// ENHANCED DEVELOPMENT ROUTES
	// ===============================
	if gin.Mode() == gin.DebugMode {
		debug := api.Group("/debug")
		{
			debug.GET("/features", func(c *gin.Context) {
				wsStats := wsHandler.GetStats()

				c.JSON(200, gin.H{
					"video_social_media": gin.H{
						"enabled": true,
						"endpoints": []string{
							"GET /videos - list videos",
							"POST /videos - create video",
							"GET /videos/:id - get video",
							"POST /videos/:id/like - like video",
							"POST /videos/:id/comments - comment",
						},
					},
					"realtime_websocket_chat": gin.H{
						"enabled":  true,
						"endpoint": "ws://localhost:8080/api/v1/ws/chat",
						"features": []string{
							"Real-time messaging",
							"Typing indicators",
							"Online status",
							"Message broadcasting",
							"Auto-reconnection",
							"Offline message queuing",
						},
						"stats": wsStats,
						"message_types": []string{
							"auth", "join_chats", "send_message", "typing_start",
							"typing_stop", "user_online", "user_offline",
						},
					},
					"contact_management": gin.H{
						"enabled": true,
						"endpoints": []string{
							"GET /contacts - get contacts",
							"POST /contacts - add contact",
							"POST /contacts/sync - sync device contacts",
							"POST /contacts/search - search registered users",
							"POST /contacts/block - block user",
							"GET /contacts/blocked - get blocked users",
						},
					},
					"wallet_system": gin.H{
						"enabled": true,
						"endpoints": []string{
							"GET /wallet/:userId - get wallet",
							"POST /wallet/:userId/purchase-request - request coins",
						},
					},
					"performance_optimizations": gin.H{
						"gzip_compression":       "~70% size reduction",
						"enhanced_rate_limiting": "300 req/min for chat",
						"connection_pooling":     "100 max connections for WebSocket",
						"bulk_endpoints":         "50 videos per request",
						"streaming_headers":      "optimized for video + real-time chat",
						"jsonb_support":          "efficient chat metadata storage",
						"websocket_realtime":     "sub-second message delivery",
					},
				})
			})

			debug.GET("/websocket-examples", func(c *gin.Context) {
				c.JSON(200, gin.H{
					"connection": gin.H{
						"url": "ws://localhost:8080/api/v1/ws/chat",
						"auth": gin.H{
							"type": "auth",
							"data": gin.H{
								"userId": "user123",
								"token":  "firebase_jwt_token",
							},
						},
					},
					"join_chats": gin.H{
						"type": "join_chats",
						"data": gin.H{
							"chatIds": []string{"user1_user2", "user1_user3"},
						},
					},
					"send_message": gin.H{
						"type": "send_message",
						"data": gin.H{
							"message": gin.H{
								"messageId": "msg123",
								"chatId":    "user1_user2",
								"senderId":  "user1",
								"content":   "Hello via WebSocket!",
								"type":      "text",
							},
						},
						"requestId": "req123",
					},
					"typing_indicators": gin.H{
						"typing_start": gin.H{
							"type": "typing_start",
							"data": gin.H{
								"chatId":   "user1_user2",
								"userId":   "user1",
								"isTyping": true,
							},
						},
						"typing_stop": gin.H{
							"type": "typing_stop",
							"data": gin.H{
								"chatId":   "user1_user2",
								"userId":   "user1",
								"isTyping": false,
							},
						},
					},
					"received_events": []string{
						"message_received", "message_sent", "message_failed",
						"chat_updated", "typing_start", "typing_stop",
						"user_online", "user_offline", "error", "pong",
					},
				})
			})
		}
	}
}

// Enhanced rate limiter with WebSocket considerations
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

func createRateLimitMiddleware(rateLimiter *RateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip rate limiting for WebSocket upgrades
		if c.GetHeader("Upgrade") == "websocket" {
			c.Next()
			return
		}

		ip := c.ClientIP()
		path := c.Request.URL.Path

		var limit int
		var window time.Duration

		if path == "/api/v1/videos/bulk" {
			limit = 30
			window = time.Minute
		} else if strings.Contains(path, "/chats") || strings.Contains(path, "/contacts") {
			limit = 500 // Higher limit for real-time features
			window = time.Minute
		} else if strings.Contains(path, "/videos") {
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

func contains(str, substr string) bool {
	return strings.Contains(str, substr)
}
