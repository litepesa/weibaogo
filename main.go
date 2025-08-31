// ===============================
// main.go - Updated Entry Point for Refined Architecture with Simplified Episode Management
// ===============================

package main

import (
	"fmt"
	"log"
	"strconv"

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

	// API routes - REFINED ARCHITECTURE WITH SIMPLIFIED EPISODE MANAGEMENT
	setupRefinedRoutesWithUnlockTracking(router, firebaseService, authHandler, userHandler, dramaHandler, walletHandler, uploadHandler, dramaService)

	// Start server
	port := cfg.Port
	log.Printf("🚀 Server starting on port %s", port)
	log.Printf("🌍 Environment: %s", cfg.Environment)
	log.Printf("💾 Database connected to %s:%s", cfg.Database.Host, cfg.Database.Port)
	log.Printf("🔥 Firebase service initialized")
	log.Printf("☁️  R2 storage initialized")
	log.Printf("🎭 Refined Drama Architecture with Unlock Tracking enabled")
	log.Printf("📊 Episode specs: 2min max, 50MB max, 99 coins unlock")

	log.Fatal(router.Run(":" + port))
}

func setupRefinedRoutesWithUnlockTracking(
	router *gin.Engine,
	firebaseService *services.FirebaseService,
	authHandler *handlers.AuthHandler,
	userHandler *handlers.UserHandler,
	dramaHandler *handlers.DramaHandler,
	walletHandler *handlers.WalletHandler,
	uploadHandler *handlers.UploadHandler,
	dramaService *services.DramaService,
) {
	api := router.Group("/api/v1")

	// ===============================
	// AUTHENTICATION ROUTES
	// ===============================
	auth := api.Group("/auth")
	{
		auth.POST("/verify", authHandler.VerifyToken)
		auth.POST("/sync", middleware.FirebaseAuth(firebaseService), authHandler.SyncUser)
		auth.GET("/user", middleware.FirebaseAuth(firebaseService), authHandler.GetCurrentUser)
	}

	// ===============================
	// PUBLIC ROUTES (no auth required)
	// ===============================
	public := api.Group("")
	{
		// Drama discovery (public)
		public.GET("/dramas", dramaHandler.GetDramas)
		public.GET("/dramas/featured", dramaHandler.GetFeaturedDramas)
		public.GET("/dramas/trending", dramaHandler.GetTrendingDramas)
		public.GET("/dramas/popular", func(c *gin.Context) {
			limit := 20
			if l := c.Query("limit"); l != "" {
				if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
					limit = parsed
				}
			}

			timeframe := c.Query("timeframe") // week, month, all
			if timeframe == "" {
				timeframe = "all"
			}

			dramas, err := dramaService.GetPopularDramas(c.Request.Context(), limit, timeframe)
			if err != nil {
				c.JSON(500, gin.H{"error": "Failed to get popular dramas"})
				return
			}

			c.JSON(200, dramas)
		})
		public.GET("/dramas/recent", func(c *gin.Context) {
			limit := 10
			if l := c.Query("limit"); l != "" {
				if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 50 {
					limit = parsed
				}
			}

			dramas, err := dramaService.GetRecentlyUpdatedDramas(c.Request.Context(), limit)
			if err != nil {
				c.JSON(500, gin.H{"error": "Failed to get recently updated dramas"})
				return
			}

			c.JSON(200, dramas)
		})
		public.GET("/dramas/search", dramaHandler.SearchDramas)
		public.GET("/dramas/:dramaId", dramaHandler.GetDrama)

		// Episode endpoints (public) - return drama episodes
		public.GET("/dramas/:dramaId/episodes", dramaHandler.GetDramaEpisodes)
		public.GET("/dramas/:dramaId/episodes/:episodeNumber", dramaHandler.GetEpisodeDetails)

		// Legacy episode endpoint (compatibility)
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

			episode, err := dramaService.GetEpisodeByNumber(c.Request.Context(), dramaID, epNum)
			if err != nil {
				c.JSON(404, gin.H{"error": "Episode not found"})
				return
			}

			c.JSON(200, episode)
		})
	}

	// ===============================
	// PROTECTED ROUTES (Firebase auth required)
	// ===============================
	protected := api.Group("")
	protected.Use(middleware.FirebaseAuth(firebaseService))
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

		// Drama interactions
		protected.POST("/dramas/:dramaId/views", dramaHandler.IncrementViews)
		protected.POST("/dramas/:dramaId/favorites", dramaHandler.ToggleFavorite)

		// Drama unlock (99 coins)
		protected.POST("/unlock-drama", dramaHandler.UnlockDrama)

		// Wallet
		protected.GET("/wallet/:userId", walletHandler.GetWallet)
		protected.GET("/wallet/:userId/transactions", walletHandler.GetTransactions)
		protected.POST("/wallet/:userId/purchase-request", walletHandler.CreatePurchaseRequest)

		// File upload (50MB max, 2min duration)
		protected.POST("/upload", uploadHandler.UploadFile)

		// ===============================
		// ADMIN ROUTES (Admin authentication required)
		// ===============================
		admin := protected.Group("")
		admin.Use(middleware.AdminOnly())
		{
			// ===============================
			// DRAMA MANAGEMENT (CRUD Operations)
			// ===============================

			// Create drama with episodes (unified approach)
			admin.POST("/admin/dramas/create-with-episodes", dramaHandler.CreateDramaWithEpisodes)

			// Update and delete dramas (ownership-based)
			admin.PUT("/admin/dramas/:dramaId", dramaHandler.UpdateDrama)
			admin.DELETE("/admin/dramas/:dramaId", dramaHandler.DeleteDrama)

			// Drama status management (ownership-based)
			admin.POST("/admin/dramas/:dramaId/featured", dramaHandler.ToggleFeatured)
			admin.POST("/admin/dramas/:dramaId/active", dramaHandler.ToggleActive)

			// Get admin's own dramas (ownership-based)
			admin.GET("/admin/dramas", dramaHandler.GetAdminDramas)

			// Get dramas with episode counts (for dashboard)
			admin.GET("/admin/dramas/with-counts", func(c *gin.Context) {
				userID := c.GetString("userID")
				if userID == "" {
					c.JSON(401, gin.H{"error": "User not authenticated"})
					return
				}

				limit := 50
				if l := c.Query("limit"); l != "" {
					if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 200 {
						limit = parsed
					}
				}

				offset := 0
				if o := c.Query("offset"); o != "" {
					if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
						offset = parsed
					}
				}

				dramas, err := dramaService.GetDramasWithEpisodeCount(c.Request.Context(), userID, limit, offset)
				if err != nil {
					c.JSON(500, gin.H{"error": "Failed to get dramas with episode counts"})
					return
				}

				c.JSON(200, dramas)
			})

			// ===============================
			// SIMPLIFIED EPISODE MANAGEMENT (Single Operations Only)
			// ===============================

			// Add single episode to existing drama
			admin.POST("/admin/dramas/:dramaId/episodes", dramaHandler.AddEpisodeToDrama)

			// Remove specific episode from drama
			admin.DELETE("/admin/dramas/:dramaId/episodes/:episodeNumber", dramaHandler.RemoveEpisodeFromDrama)

			// Replace existing episode with new video
			admin.PUT("/admin/dramas/:dramaId/episodes/:episodeNumber", dramaHandler.ReplaceEpisodeInDrama)

			// ===============================
			// EPISODE INFORMATION & STATISTICS (With Unlock Tracking)
			// ===============================

			// Get episode statistics for a drama (owner only) - now includes unlock tracking
			admin.GET("/admin/dramas/:dramaId/episodes/stats", dramaHandler.GetEpisodeStats)

			// Get detailed episode information
			admin.GET("/admin/dramas/:dramaId/episodes/:episodeNumber/details", func(c *gin.Context) {
				dramaID := c.Param("dramaId")
				episodeNumberStr := c.Param("episodeNumber")

				if dramaID == "" || episodeNumberStr == "" {
					c.JSON(400, gin.H{"error": "Drama ID and episode number required"})
					return
				}

				episodeNumber, err := strconv.Atoi(episodeNumberStr)
				if err != nil || episodeNumber < 1 {
					c.JSON(400, gin.H{"error": "Invalid episode number"})
					return
				}

				userID := c.GetString("userID")
				if userID == "" {
					c.JSON(401, gin.H{"error": "User not authenticated"})
					return
				}

				// Check ownership
				hasAccess, err := dramaService.CheckDramaOwnership(c.Request.Context(), dramaID, userID)
				if err != nil {
					c.JSON(500, gin.H{"error": "Failed to verify ownership"})
					return
				}
				if !hasAccess {
					c.JSON(403, gin.H{"error": "You can only view details for episodes in dramas you created"})
					return
				}

				episode, err := dramaService.GetDramaEpisodeDetails(c.Request.Context(), dramaID, episodeNumber)
				if err != nil {
					switch err.Error() {
					case "drama_not_found":
						c.JSON(404, gin.H{"error": "Drama not found"})
					case "episode_not_found":
						c.JSON(404, gin.H{"error": "Episode not found"})
					default:
						c.JSON(500, gin.H{"error": "Failed to get episode details"})
					}
					return
				}

				c.JSON(200, episode)
			})

			// ===============================
			// UPLOAD LIMITS & VALIDATION
			// ===============================

			// Get upload limits and current usage (updated with new specs)
			admin.GET("/admin/upload-limits", dramaHandler.GetDramaUploadLimits)

			// Validate drama for episode operations
			admin.GET("/admin/dramas/:dramaId/validate", func(c *gin.Context) {
				dramaID := c.Param("dramaId")
				if dramaID == "" {
					c.JSON(400, gin.H{"error": "Drama ID required"})
					return
				}

				userID := c.GetString("userID")
				if userID == "" {
					c.JSON(401, gin.H{"error": "User not authenticated"})
					return
				}

				// Check ownership
				hasAccess, err := dramaService.CheckDramaOwnership(c.Request.Context(), dramaID, userID)
				if err != nil {
					c.JSON(500, gin.H{"error": "Failed to verify ownership"})
					return
				}
				if !hasAccess {
					c.JSON(403, gin.H{"error": "Access denied"})
					return
				}

				// Validate drama for episode operations
				err = dramaService.ValidateDramaForEpisodeOperation(c.Request.Context(), dramaID)
				if err != nil {
					c.JSON(400, gin.H{"error": err.Error()})
					return
				}

				c.JSON(200, gin.H{
					"valid":   true,
					"message": "Drama is ready for episode operations",
					"dramaId": dramaID,
					"specs": gin.H{
						"maxEpisodeDurationSecs": 120, // 2 minutes
						"maxFileSizeMB":          50,  // 50MB
						"unlockCostCoins":        99,  // 99 coins
					},
				})
			})

			// ===============================
			// ANALYTICS & REVENUE TRACKING
			// ===============================

			// Get current user's drama statistics (enhanced with unlock tracking)
			admin.GET("/admin/my-stats", func(c *gin.Context) {
				userID := c.GetString("userID")
				if userID == "" {
					c.JSON(401, gin.H{"error": "User not authenticated"})
					return
				}

				stats, err := dramaService.GetUserDramaStats(c.Request.Context(), userID)
				if err != nil {
					c.JSON(500, gin.H{"error": "Failed to get user statistics"})
					return
				}

				c.JSON(200, stats)
			})

			// NEW: Get top earning dramas by unlock revenue
			admin.GET("/admin/top-earning", func(c *gin.Context) {
				userID := c.GetString("userID")
				if userID == "" {
					c.JSON(401, gin.H{"error": "User not authenticated"})
					return
				}

				limit := 10
				if l := c.Query("limit"); l != "" {
					if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 50 {
						limit = parsed
					}
				}

				topEarning, err := dramaService.GetTopEarningDramas(c.Request.Context(), limit)
				if err != nil {
					c.JSON(500, gin.H{"error": "Failed to get top earning dramas"})
					return
				}

				c.JSON(200, gin.H{
					"topEarningDramas": topEarning,
					"totalResults":     len(topEarning),
					"unlockCost":       99,
				})
			})

			// Enhanced analytics overview with unlock revenue tracking
			admin.GET("/admin/analytics/overview", func(c *gin.Context) {
				userID := c.GetString("userID")
				if userID == "" {
					c.JSON(401, gin.H{"error": "User not authenticated"})
					return
				}

				// Get user's drama statistics with unlock tracking
				stats, err := dramaService.GetUserDramaStats(c.Request.Context(), userID)
				if err != nil {
					c.JSON(500, gin.H{"error": "Failed to get analytics"})
					return
				}

				// Add platform specifications to the response
				platformConfig := map[string]interface{}{
					"episodeMaxDurationSecs": 120, // 2 minutes
					"episodeMaxSizeMB":       50,  // 50MB
					"dramaUnlockCost":        99,  // 99 coins
					"maxEpisodesPerDrama":    100,
				}

				c.JSON(200, gin.H{
					"userStats":      stats,
					"platformConfig": platformConfig,
					"message":        "Analytics overview with unlock tracking",
				})
			})

			// ===============================
			// WALLET MANAGEMENT (Admin)
			// ===============================
			admin.POST("/admin/wallet/:userId/add-coins", walletHandler.AddCoins)
			admin.GET("/admin/purchase-requests", walletHandler.GetPendingPurchases)
			admin.POST("/admin/purchase-requests/:requestId/approve", walletHandler.ApprovePurchase)
			admin.POST("/admin/purchase-requests/:requestId/reject", walletHandler.RejectPurchase)

			// ===============================
			// USER MANAGEMENT (Admin)
			// ===============================
			admin.GET("/admin/users", userHandler.GetAllUsers)
		}
	}

	// ===============================
	// API DOCUMENTATION ENDPOINT (Updated)
	// ===============================
	api.GET("/docs", func(c *gin.Context) {
		docs := map[string]interface{}{
			"version": "1.0.0",
			"title":   "Drama Platform API - Simplified Episode Management with Unlock Tracking",
			"specifications": map[string]interface{}{
				"episodeMaxDuration":  "2 minutes (120 seconds)",
				"episodeMaxFileSize":  "50MB",
				"dramaUnlockCost":     "99 coins",
				"maxEpisodesPerDrama": 100,
				"supportedFormats":    []string{"mp4", "mov", "avi", "mkv", "webm"},
			},
			"endpoints": map[string]interface{}{
				"public": map[string]interface{}{
					"dramas":   "GET /api/v1/dramas - Get all active dramas",
					"featured": "GET /api/v1/dramas/featured - Get featured dramas",
					"trending": "GET /api/v1/dramas/trending - Get trending dramas",
					"popular":  "GET /api/v1/dramas/popular?timeframe=all - Get popular dramas",
					"recent":   "GET /api/v1/dramas/recent - Get recently updated dramas",
					"search":   "GET /api/v1/dramas/search?q=query - Search dramas",
					"drama":    "GET /api/v1/dramas/:id - Get specific drama",
					"episodes": "GET /api/v1/dramas/:id/episodes - Get drama episodes",
					"episode":  "GET /api/v1/dramas/:id/episodes/:number - Get specific episode",
				},
				"protected": map[string]interface{}{
					"auth":         "POST /api/v1/auth/* - Authentication endpoints",
					"users":        "CRUD /api/v1/users/* - User management",
					"interactions": "POST /api/v1/dramas/:id/views - Drama interactions",
					"unlock":       "POST /api/v1/unlock-drama - Unlock premium drama (99 coins)",
					"wallet":       "GET /api/v1/wallet/* - Wallet operations",
					"upload":       "POST /api/v1/upload - File upload (max 50MB, 2min duration)",
				},
				"admin": map[string]interface{}{
					"drama_management": map[string]string{
						"create":        "POST /api/v1/admin/dramas/create-with-episodes",
						"update":        "PUT /api/v1/admin/dramas/:id",
						"delete":        "DELETE /api/v1/admin/dramas/:id",
						"toggle_status": "POST /api/v1/admin/dramas/:id/featured|active",
						"list_own":      "GET /api/v1/admin/dramas",
						"with_counts":   "GET /api/v1/admin/dramas/with-counts",
					},
					"episode_management": map[string]string{
						"add_episode":     "POST /api/v1/admin/dramas/:id/episodes - Add single episode",
						"remove_episode":  "DELETE /api/v1/admin/dramas/:id/episodes/:number",
						"replace_episode": "PUT /api/v1/admin/dramas/:id/episodes/:number",
						"stats":           "GET /api/v1/admin/dramas/:id/episodes/stats - Includes unlock tracking",
						"details":         "GET /api/v1/admin/dramas/:id/episodes/:number/details",
					},
					"analytics": map[string]string{
						"user_stats":    "GET /api/v1/admin/my-stats",
						"top_earning":   "GET /api/v1/admin/top-earning - Top dramas by unlock revenue",
						"overview":      "GET /api/v1/admin/analytics/overview",
						"upload_limits": "GET /api/v1/admin/upload-limits",
					},
					"utilities": map[string]string{
						"validate_drama": "GET /api/v1/admin/dramas/:id/validate",
					},
				},
			},
			"features": []string{
				"Simplified episode management (one-at-a-time uploads)",
				"Unlock tracking and revenue analytics",
				"2-minute episode duration limit",
				"50MB file size limit",
				"99-coin unlock cost",
				"Ownership-based drama management",
				"Transaction safety and audit logging",
				"Revenue tracking per drama",
				"Conversion rate analytics (views to unlocks)",
			},
			"removed_features": []string{
				"Bulk episode operations (simplified to single uploads)",
				"Episode reordering (episodes added sequentially)",
				"Bulk URL validation (single episode validation only)",
				"Episode upload status tracking (not needed for single uploads)",
				"Maintenance cleanup utilities (simplified management)",
			},
		}

		c.JSON(200, docs)
	})
}

// ===============================
// HELPER FUNCTIONS
// ===============================

// validateAdminAccess is a helper to check admin access in inline handlers
func validateAdminAccess(c *gin.Context) (string, bool) {
	userID := c.GetString("userID")
	if userID == "" {
		c.JSON(401, gin.H{"error": "User not authenticated"})
		return "", false
	}

	// Additional admin validation could go here
	// For now, we rely on the AdminOnly middleware

	return userID, true
}
