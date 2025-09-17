// ===============================
// main.go - Updated with Chat and Contacts Support
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

		// Different limits for different endpoint types
		var limit int
		var window time.Duration

		path := c.Request.URL.Path
		if path == "/api/v1/videos/bulk" {
			// Stricter limits for bulk endpoints
			limit = 30
			window = time.Minute
		} else if path == "/api/v1/videos" ||
			path == "/api/v1/videos/featured" ||
			path == "/api/v1/videos/trending" {
			// Standard limits for video list endpoints
			limit = 100
			window = time.Minute
		} else if contains(path, "/chats") || contains(path, "/contacts") {
			// More lenient for chat/contact endpoints
			limit = 300
			window = time.Minute
		} else {
			// Default for other endpoints
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
	for i := 0; i <= len(str)-len(substr); i++ {
		if str[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
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
	log.Println("📊 Applying database optimizations for video + chat workload:")
	db.SetMaxOpenConns(75)                  // Increased for chat + video
	db.SetMaxIdleConns(35)                  // Higher for real-time features
	db.SetConnMaxLifetime(15 * time.Minute) // Longer for persistent connections
	db.SetConnMaxIdleTime(8 * time.Minute)  // Extended for chat connections
	log.Printf("   • Max open connections: 75")
	log.Printf("   • Max idle connections: 35")
	log.Printf("   • Connection lifetime: 15 minutes")
	log.Printf("   • Idle timeout: 8 minutes")

	// Run database migrations
	log.Println("🔧 Running database migrations with chat and contacts support...")
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

	// Initialize rate limiter
	rateLimiter := NewRateLimiter()

	// Setup router
	router := setupOptimizedRouter(cfg, rateLimiter)

	// Health check with chat/contacts info
	router.GET("/health", func(c *gin.Context) {
		dbStats := database.Stats()
		c.JSON(200, gin.H{
			"status":   "healthy",
			"database": database.Health() == nil,
			"app":      "video-social-media-with-chat",
			"features": gin.H{
				"videos":   true,
				"wallet":   true,
				"chat":     true,
				"contacts": true,
			},
			"optimizations": gin.H{
				"gzip_compression":   true,
				"rate_limiting":      true,
				"connection_pooling": true,
				"bulk_endpoints":     true,
				"streaming_headers":  true,
				"url_optimization":   true,
				"real_time_chat":     true,
			},
			"database_stats": gin.H{
				"open_connections": dbStats.OpenConnections,
				"in_use":           dbStats.InUse,
				"idle":             dbStats.Idle,
				"max_open":         75,
				"max_idle":         35,
			},
		})
	})

	// Setup routes
	setupOptimizedRoutes(router, firebaseService, authHandler, userHandler, videoHandler,
		walletHandler, uploadHandler, chatHandler, contactHandler)

	// Start server
	port := cfg.Port
	log.Printf("🚀 ENHANCED Video Social Media Server with Chat & Contacts starting on port %s", port)
	log.Printf("🌍 Environment: %s", cfg.Environment)
	log.Printf("💾 Database connected with enhanced pool (Max: 75, Idle: 35)")
	log.Printf("🔥 Firebase service initialized")
	log.Printf("☁️  R2 storage initialized")
	log.Printf("📱 Features enabled:")
	log.Printf("   • Video Social Media: ✅")
	log.Printf("   • Real-time Chat: ✅")
	log.Printf("   • Contact Management: ✅")
	log.Printf("   • Wallet System: ✅")
	log.Printf("⚡ Performance optimizations:")
	log.Printf("   • Enhanced connection pooling for real-time features")
	log.Printf("   • Optimized rate limiting for chat endpoints")
	log.Printf("   • JSONB support for chat metadata")
	log.Printf("   • Efficient contact synchronization")
	log.Printf("   • Video reaction messages support")

	log.Fatal(router.Run(":" + port))
}

// ===============================
// OPTIMIZED ROUTER SETUP
// ===============================

func setupOptimizedRouter(cfg *config.Config, rateLimiter *RateLimiter) *gin.Engine {
	router := gin.Default()

	// Enhanced GZIP compression (excluding video/audio formats)
	router.Use(gzip.Gzip(gzip.DefaultCompression, gzip.WithExcludedExtensions([]string{
		".mp4", ".avi", ".mov", ".webm", ".mp3", ".wav", ".ogg"})))

	// Enhanced rate limiting
	router.Use(createRateLimitMiddleware(rateLimiter))

	// Enhanced CORS with chat headers
	router.Use(cors.New(cors.Config{
		AllowOrigins: cfg.AllowedOrigins,
		AllowMethods: []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"},
		AllowHeaders: []string{
			"Origin", "Content-Type", "Authorization",
			"Range", "Accept-Ranges",
			"Cache-Control", "If-None-Match", "If-Modified-Since",
			"X-Chat-ID", "X-Message-ID", "X-Contact-Sync", // Chat-specific headers
		},
		ExposeHeaders: []string{
			"Content-Length", "Content-Range", "Accept-Ranges",
			"Cache-Control", "Last-Modified", "ETag",
			"X-RateLimit-Limit", "X-RateLimit-Remaining", "Retry-After",
			"X-Chat-Status", "X-Message-Status", "X-Sync-Version", // Chat-specific headers
		},
		AllowCredentials: true,
		MaxAge:           12 * 3600,
	}))

	// Enhanced performance headers
	router.Use(func(c *gin.Context) {
		// Add performance hints
		c.Header("X-DNS-Prefetch-Control", "on")
		c.Header("X-Powered-By", "video-social-chat-optimized")

		// Security headers
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "SAMEORIGIN")
		c.Header("X-XSS-Protection", "1; mode=block")

		// Chat-specific headers
		if contains(c.Request.URL.Path, "/chats") || contains(c.Request.URL.Path, "/messages") {
			c.Header("X-Chat-Support", "enabled")
		}

		c.Next()
	})

	return router
}

// ===============================
// ENHANCED ROUTES WITH CHAT & CONTACTS
// ===============================

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
) {
	api := router.Group("/api/v1")

	// ===============================
	// AUTH ROUTES (UNCHANGED)
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
		// Video endpoints (existing)
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

		// User profile endpoints (existing)
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
		// ===== USER MANAGEMENT (EXISTING) =====
		protected.PUT("/users/:userId", userHandler.UpdateUser)
		protected.DELETE("/users/:userId", userHandler.DeleteUser)
		protected.POST("/users/:userId/status", userHandler.UpdateUserStatus)

		// ===== VIDEO FEATURES (EXISTING) =====
		protected.POST("/videos", videoHandler.CreateVideo)
		protected.PUT("/videos/:videoId", videoHandler.UpdateVideo)
		protected.DELETE("/videos/:videoId", videoHandler.DeleteVideo)
		protected.POST("/videos/:videoId/like", videoHandler.LikeVideo)
		protected.DELETE("/videos/:videoId/like", videoHandler.UnlikeVideo)
		protected.POST("/videos/:videoId/share", videoHandler.ShareVideo)
		protected.GET("/videos/recommendations", videoHandler.GetVideoRecommendations)

		// ===== SOCIAL FEATURES (EXISTING) =====
		protected.POST("/users/:userId/follow", videoHandler.FollowUser)
		protected.DELETE("/users/:userId/follow", videoHandler.UnfollowUser)
		protected.GET("/feed/following", videoHandler.GetFollowingFeed)

		// ===== COMMENT MANAGEMENT (EXISTING) =====
		protected.POST("/videos/:videoId/comments", videoHandler.CreateComment)
		protected.DELETE("/comments/:commentId", videoHandler.DeleteComment)
		protected.POST("/comments/:commentId/like", videoHandler.LikeComment)
		protected.DELETE("/comments/:commentId/like", videoHandler.UnlikeComment)

		// ===== 📱 NEW: CHAT ENDPOINTS =====
		chatRoutes := protected.Group("/chats")
		{
			// Chat management
			chatRoutes.POST("", chatHandler.CreateOrGetChat) // Create/get chat
			chatRoutes.GET("", chatHandler.GetChats)         // Get user's chats
			chatRoutes.GET("/:chatId", chatHandler.GetChat)  // Get specific chat

			// Chat settings
			chatRoutes.POST("/:chatId/mark-read", chatHandler.MarkChatAsRead)         // Mark as read
			chatRoutes.POST("/:chatId/toggle-pin", chatHandler.TogglePinChat)         // Pin/unpin
			chatRoutes.POST("/:chatId/toggle-archive", chatHandler.ToggleArchiveChat) // Archive
			chatRoutes.POST("/:chatId/toggle-mute", chatHandler.ToggleMuteChat)       // Mute/unmute
			chatRoutes.POST("/:chatId/settings", chatHandler.SetChatSettings)         // Wallpaper, font

			// Message management
			chatRoutes.POST("/:chatId/messages", chatHandler.SendMessage)                // Send message
			chatRoutes.GET("/:chatId/messages", chatHandler.GetMessages)                 // Get messages
			chatRoutes.PUT("/:chatId/messages/:messageId", chatHandler.UpdateMessage)    // Edit message
			chatRoutes.DELETE("/:chatId/messages/:messageId", chatHandler.DeleteMessage) // Delete message

			// Special message types
			chatRoutes.POST("/:chatId/video-reaction", chatHandler.SendVideoReaction)   // Video reactions
			chatRoutes.POST("/:chatId/moment-reaction", chatHandler.SendMomentReaction) // Moment reactions
		}

		// ===== 📱 NEW: CONTACT ENDPOINTS =====
		contactRoutes := protected.Group("/contacts")
		{
			// Contact management
			contactRoutes.GET("", contactHandler.GetContacts)                 // Get contacts
			contactRoutes.POST("", contactHandler.AddContact)                 // Add contact
			contactRoutes.DELETE("/:contactId", contactHandler.RemoveContact) // Remove contact

			// Contact search and sync
			contactRoutes.POST("/search", contactHandler.SearchContacts)         // Search by phone numbers
			contactRoutes.POST("/sync", contactHandler.SyncContacts)             // Sync device contacts
			contactRoutes.GET("/search/phone", contactHandler.SearchUserByPhone) // Search by single phone

			// Blocking
			contactRoutes.POST("/block", contactHandler.BlockContact)        // Block contact
			contactRoutes.POST("/unblock", contactHandler.UnblockContact)    // Unblock contact
			contactRoutes.GET("/blocked", contactHandler.GetBlockedContacts) // Get blocked contacts
		}

		// ===== WALLET ENDPOINTS (EXISTING) =====
		protected.GET("/wallet/:userId", walletHandler.GetWallet)
		protected.GET("/wallet/:userId/transactions", walletHandler.GetTransactions)
		protected.POST("/wallet/:userId/purchase-request", walletHandler.CreatePurchaseRequest)

		// ===== FILE UPLOAD (EXISTING) =====
		protected.POST("/upload", uploadHandler.UploadFile)
		protected.POST("/upload/batch", uploadHandler.BatchUploadFiles)

		// ===============================
		// ADMIN ROUTES
		// ===============================
		admin := protected.Group("")
		admin.Use(middleware.AdminOnly())
		{
			// Video moderation (existing)
			admin.POST("/admin/videos/:videoId/featured", videoHandler.ToggleFeatured)
			admin.POST("/admin/videos/:videoId/active", videoHandler.ToggleActive)

			// User management (existing)
			admin.GET("/admin/users", userHandler.GetAllUsers)
			admin.POST("/admin/users/:userId/status", userHandler.UpdateUserStatus)

			// Wallet management (existing)
			admin.POST("/admin/wallet/:userId/add-coins", walletHandler.AddCoins)
			admin.GET("/admin/purchase-requests", walletHandler.GetPendingPurchases)
			admin.POST("/admin/purchase-requests/:requestId/approve", walletHandler.ApprovePurchase)
			admin.POST("/admin/purchase-requests/:requestId/reject", walletHandler.RejectPurchase)

			// ===== 📱 NEW: CHAT & CONTACT MODERATION =====
			//chatAdmin := admin.Group("/admin/chats")
			//{
			// Chat moderation endpoints can be added here
			// chatAdmin.GET("", adminChatHandler.GetAllChats)              // Future: Get all chats
			// chatAdmin.DELETE("/:chatId", adminChatHandler.DeleteChat)    // Future: Delete chat
			// chatAdmin.GET("/reports", adminChatHandler.GetChatReports)   // Future: Chat reports
			//}

			//contactAdmin := admin.Group("/admin/contacts")
			//{
			// Contact moderation endpoints can be added here
			// contactAdmin.GET("/blocked", adminContactHandler.GetAllBlocked) // Future: All blocked users
			// contactAdmin.GET("/reports", adminContactHandler.GetReports)     // Future: Contact reports
			//}

			// Enhanced platform analytics
			admin.GET("/admin/stats", func(c *gin.Context) {
				c.Header("Cache-Control", "public, max-age=300")
				dbStats := database.Stats()

				c.JSON(200, gin.H{
					"message": "Enhanced platform statistics",
					"features": gin.H{
						"videos":             "enabled + optimized",
						"chat":               "enabled + real-time",
						"contacts":           "enabled + sync",
						"wallet":             "enabled",
						"gzip_compression":   true,
						"rate_limiting":      true,
						"bulk_endpoints":     true,
						"streaming_headers":  true,
						"url_optimization":   true,
						"real_time_features": true,
					},
					"status": "operational",
					"performance": gin.H{
						"database_connections": gin.H{
							"open":     dbStats.OpenConnections,
							"in_use":   dbStats.InUse,
							"idle":     dbStats.Idle,
							"max_open": 75,
							"max_idle": 35,
						},
						"optimizations_active":  8,
						"estimated_improvement": "Enhanced for real-time features",
					},
				})
			})

			// System health with enhanced metrics
			admin.GET("/admin/health", func(c *gin.Context) {
				c.Header("Cache-Control", "no-cache")
				dbStats := database.Stats()

				c.JSON(200, gin.H{
					"database": gin.H{
						"status":           "connected",
						"open_connections": dbStats.OpenConnections,
						"in_use":           dbStats.InUse,
						"idle":             dbStats.Idle,
						"max_open":         75,
						"max_idle":         35,
						"optimized_for":    "video + chat workload",
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
						"videos":           true,
						"real_time_chat":   true,
						"contact_sync":     true,
						"wallet_system":    true,
						"video_reactions":  true,
						"moment_reactions": true,
					},
					"performance": gin.H{
						"gzip_compression":   true,
						"rate_limiting":      true,
						"bulk_endpoints":     true,
						"streaming_headers":  true,
						"url_optimization":   true,
						"smart_caching":      true,
						"connection_pooling": true,
						"real_time_features": true,
					},
					"app": gin.H{
						"name":    "video-social-media-with-chat",
						"version": "1.2.0-chat-contacts",
						"status":  "healthy",
						"features": []string{
							"videos", "chat", "contacts", "wallet",
							"real-time", "performance-optimized",
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
					"real_time_chat": gin.H{
						"enabled": true,
						"endpoints": []string{
							"POST /chats - create/get chat",
							"GET /chats - list user chats",
							"POST /chats/:id/messages - send message",
							"GET /chats/:id/messages - get messages",
							"POST /chats/:id/video-reaction - send video reaction",
							"POST /chats/:id/moment-reaction - send moment reaction",
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
						"connection_pooling":     "75 max connections for real-time",
						"bulk_endpoints":         "50 videos per request",
						"streaming_headers":      "optimized for video + chat",
						"jsonb_support":          "efficient chat metadata storage",
					},
				})
			})

			debug.GET("/chat-examples", func(c *gin.Context) {
				c.JSON(200, gin.H{
					"create_chat": gin.H{
						"method":   "POST",
						"endpoint": "/chats",
						"body": gin.H{
							"participants": []string{"user1", "user2"},
						},
					},
					"send_message": gin.H{
						"method":   "POST",
						"endpoint": "/chats/:chatId/messages",
						"body": gin.H{
							"content": "Hello!",
							"type":    "text",
						},
					},
					"send_video_reaction": gin.H{
						"method":   "POST",
						"endpoint": "/chats/:chatId/video-reaction",
						"body": gin.H{
							"videoId":      "video123",
							"videoUrl":     "https://example.com/video.mp4",
							"thumbnailUrl": "https://example.com/thumb.jpg",
							"channelName":  "Creator Name",
							"channelImage": "https://example.com/avatar.jpg",
							"reaction":     "😍 Amazing video!",
						},
					},
					"sync_contacts": gin.H{
						"method":   "POST",
						"endpoint": "/contacts/sync",
						"body": gin.H{
							"deviceContacts": []gin.H{
								{
									"id":          "contact1",
									"displayName": "John Doe",
									"phoneNumbers": []gin.H{
										{"number": "+1234567890", "label": "mobile"},
									},
								},
							},
						},
					},
				})
			})
		}
	}
}
