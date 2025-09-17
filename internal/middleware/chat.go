// ===============================
// internal/middleware/chat.go - Chat and Contact Middleware
// ===============================

package middleware

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// ChatParticipantMiddleware ensures user is a participant in the chat
func ChatParticipantMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		chatID := c.Param("chatId")
		if chatID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Chat ID required"})
			c.Abort()
			return
		}

		userID := c.GetString("userID")
		if userID == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
			c.Abort()
			return
		}

		// Check if user is participant in chat (simplified - in real implementation,
		// you'd query the database to verify participation)
		if !strings.Contains(chatID, userID) {
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied - not a chat participant"})
			c.Abort()
			return
		}

		c.Next()
	}
}

// MessageOwnershipMiddleware ensures user can only modify their own messages
func MessageOwnershipMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		messageID := c.Param("messageId")
		if messageID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Message ID required"})
			c.Abort()
			return
		}

		userID := c.GetString("userID")
		if userID == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
			c.Abort()
			return
		}

		// In a real implementation, you'd query the database to check message ownership
		// For now, we'll let the service layer handle this validation

		c.Next()
	}
}

// ContactOwnershipMiddleware ensures user can only modify their own contacts
func ContactOwnershipMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetString("userID")
		if userID == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
			c.Abort()
			return
		}

		// Store user ID in context for use by handlers
		c.Set("ownerUserID", userID)
		c.Next()
	}
}

// RateLimitChatMiddleware provides enhanced rate limiting for chat operations
func RateLimitChatMiddleware() gin.HandlerFunc {
	// Simple in-memory rate limiter for chat operations
	userLimits := make(map[string][]time.Time)

	return func(c *gin.Context) {
		userID := c.GetString("userID")
		if userID == "" {
			c.Next()
			return
		}

		now := time.Now()

		// Clean old entries (older than 1 minute)
		if times, exists := userLimits[userID]; exists {
			var validTimes []time.Time
			for _, t := range times {
				if now.Sub(t) < time.Minute {
					validTimes = append(validTimes, t)
				}
			}
			userLimits[userID] = validTimes
		}

		// Check rate limits based on endpoint
		var limit int
		path := c.Request.URL.Path

		if strings.Contains(path, "/messages") && c.Request.Method == "POST" {
			limit = 60 // 60 messages per minute
		} else if strings.Contains(path, "/contacts/sync") {
			limit = 5 // 5 syncs per minute
		} else if strings.Contains(path, "/chats") {
			limit = 100 // 100 chat operations per minute
		} else {
			limit = 200 // Default limit
		}

		if len(userLimits[userID]) >= limit {
			c.Header("X-RateLimit-Limit", string(rune(limit)))
			c.Header("X-RateLimit-Remaining", "0")
			c.Header("Retry-After", "60")

			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":   "Rate limit exceeded for chat operations",
				"message": "Please slow down your requests",
				"limit":   limit,
				"resetIn": "60 seconds",
			})
			c.Abort()
			return
		}

		// Add current request time
		userLimits[userID] = append(userLimits[userID], now)

		c.Header("X-RateLimit-Limit", string(rune(limit)))
		c.Header("X-RateLimit-Remaining", string(rune(limit-len(userLimits[userID]))))

		c.Next()
	}
}

// BlockedUserMiddleware prevents interactions with blocked users
func BlockedUserMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetString("userID")
		if userID == "" {
			c.Next()
			return
		}

		// Get target user ID from various possible parameters
		targetUserID := ""
		if id := c.Param("userId"); id != "" {
			targetUserID = id
		} else if id := c.Param("contactUserId"); id != "" {
			targetUserID = id
		}

		if targetUserID != "" && targetUserID != userID {
			// In a real implementation, you'd check if users have blocked each other
			// For now, we'll let the service layer handle this validation
		}

		c.Next()
	}
}

// ChatMetricsMiddleware adds metrics and logging for chat operations
func ChatMetricsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		method := c.Request.Method

		// Add chat-specific headers
		if strings.Contains(path, "/chats") {
			c.Header("X-Chat-API-Version", "1.0")
			c.Header("X-Chat-Timestamp", start.Format(time.RFC3339))
		}

		if strings.Contains(path, "/contacts") {
			c.Header("X-Contacts-API-Version", "1.0")
			c.Header("X-Contacts-Timestamp", start.Format(time.RFC3339))
		}

		c.Next()

		// Log metrics after request completion
		duration := time.Since(start)
		status := c.Writer.Status()

		// Log chat/contact operations for monitoring
		if strings.Contains(path, "/chats") || strings.Contains(path, "/contacts") {
			userID := c.GetString("userID")
			// In a production environment, you'd send these metrics to your monitoring system
			_ = map[string]interface{}{
				"method":    method,
				"path":      path,
				"status":    status,
				"duration":  duration.Milliseconds(),
				"user_id":   userID,
				"timestamp": start,
				"feature":   getFeatureFromPath(path),
			}
		}
	}
}

func getFeatureFromPath(path string) string {
	if strings.Contains(path, "/chats") {
		return "chat"
	} else if strings.Contains(path, "/contacts") {
		return "contacts"
	}
	return "unknown"
}

// ValidateJSONMiddleware ensures request body is valid JSON for POST/PUT requests
func ValidateJSONMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method == "POST" || c.Request.Method == "PUT" {
			contentType := c.GetHeader("Content-Type")
			if !strings.Contains(contentType, "application/json") {
				c.JSON(http.StatusBadRequest, gin.H{
					"error": "Content-Type must be application/json",
				})
				c.Abort()
				return
			}
		}
		c.Next()
	}
}

// ContactSyncThrottleMiddleware throttles contact sync operations
func ContactSyncThrottleMiddleware() gin.HandlerFunc {
	lastSync := make(map[string]time.Time)

	return func(c *gin.Context) {
		if !strings.Contains(c.Request.URL.Path, "/contacts/sync") {
			c.Next()
			return
		}

		userID := c.GetString("userID")
		if userID == "" {
			c.Next()
			return
		}

		if lastSyncTime, exists := lastSync[userID]; exists {
			if time.Since(lastSyncTime) < 5*time.Minute { // 5 minute cooldown
				c.JSON(http.StatusTooManyRequests, gin.H{
					"error":           "Contact sync rate limit exceeded",
					"message":         "Please wait before syncing again",
					"cooldownSeconds": int((5*time.Minute - time.Since(lastSyncTime)).Seconds()),
				})
				c.Abort()
				return
			}
		}

		c.Next()

		// Update last sync time after successful request
		if c.Writer.Status() < 400 {
			lastSync[userID] = time.Now()
		}
	}
}

// MessageSizeMiddleware validates message content size
func MessageSizeMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !strings.Contains(c.Request.URL.Path, "/messages") || c.Request.Method != "POST" {
			c.Next()
			return
		}

		// Check content length
		if c.Request.ContentLength > 10*1024*1024 { // 10MB limit for messages with media
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{
				"error":   "Message too large",
				"message": "Message content cannot exceed 10MB",
				"maxSize": "10MB",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// WebSocketUpgradeMiddleware handles WebSocket upgrade requests for real-time chat
func WebSocketUpgradeMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check if this is a WebSocket upgrade request
		if c.GetHeader("Upgrade") == "websocket" && c.GetHeader("Connection") == "Upgrade" {
			// Validate authentication for WebSocket connections
			userID := c.GetString("userID")
			if userID == "" {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required for WebSocket"})
				c.Abort()
				return
			}

			// Add WebSocket-specific headers
			c.Header("X-WebSocket-Support", "enabled")
			c.Header("X-WebSocket-Version", "13")
		}

		c.Next()
	}
}

// CacheControlMiddleware sets appropriate cache headers for chat/contact endpoints
func CacheControlMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		method := c.Request.Method

		// Set cache headers based on endpoint
		if strings.Contains(path, "/chats") || strings.Contains(path, "/messages") {
			// Chat content should not be cached (real-time nature)
			c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
			c.Header("Pragma", "no-cache")
			c.Header("Expires", "0")
		} else if strings.Contains(path, "/contacts") && method == "GET" {
			// Contact lists can be cached briefly
			c.Header("Cache-Control", "private, max-age=300") // 5 minutes
		}

		c.Next()
	}
}

// SecurityHeadersMiddleware adds security headers for chat/contact endpoints
func SecurityHeadersMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path

		if strings.Contains(path, "/chats") || strings.Contains(path, "/contacts") {
			// Additional security headers for sensitive operations
			c.Header("X-Content-Security-Policy", "default-src 'self'")
			c.Header("X-Privacy-Protected", "true")
			c.Header("X-Data-Classification", "private")
		}

		c.Next()
	}
}

// RequestIDMiddleware adds a unique request ID for tracing chat operations
func RequestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			// Generate a simple request ID (in production, use a proper UUID library)
			requestID = generateSimpleID()
		}

		c.Header("X-Request-ID", requestID)
		c.Set("requestID", requestID)

		c.Next()
	}
}

func generateSimpleID() string {
	// Simple ID generation - in production, use crypto/rand or UUID library
	return time.Now().Format("20060102150405") + "-" + string(rune(time.Now().Nanosecond()%1000))
}

// ===============================
// Convenience middleware combinations
// ===============================

// ChatEndpointMiddleware combines common middleware for chat endpoints
func ChatEndpointMiddleware() []gin.HandlerFunc {
	return []gin.HandlerFunc{
		RequestIDMiddleware(),
		ChatMetricsMiddleware(),
		RateLimitChatMiddleware(),
		CacheControlMiddleware(),
		SecurityHeadersMiddleware(),
		ValidateJSONMiddleware(),
		MessageSizeMiddleware(),
	}
}

// ContactEndpointMiddleware combines common middleware for contact endpoints
func ContactEndpointMiddleware() []gin.HandlerFunc {
	return []gin.HandlerFunc{
		RequestIDMiddleware(),
		ChatMetricsMiddleware(),
		ContactOwnershipMiddleware(),
		ContactSyncThrottleMiddleware(),
		CacheControlMiddleware(),
		SecurityHeadersMiddleware(),
		ValidateJSONMiddleware(),
	}
}

// RealtimeMiddleware combines middleware for real-time features
func RealtimeMiddleware() []gin.HandlerFunc {
	return []gin.HandlerFunc{
		RequestIDMiddleware(),
		WebSocketUpgradeMiddleware(),
		ChatMetricsMiddleware(),
		SecurityHeadersMiddleware(),
	}
}
