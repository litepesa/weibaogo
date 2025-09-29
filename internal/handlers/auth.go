// ===============================
// internal/handlers/auth.go - UPDATED Auth Handler with Role Support
// ===============================

package handlers

import (
	"net/http"
	"time"

	"weibaobe/internal/database"
	"weibaobe/internal/models"
	"weibaobe/internal/services"

	"firebase.google.com/go/v4/auth"
	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	firebaseService *services.FirebaseService
}

func NewAuthHandler(firebaseService *services.FirebaseService) *AuthHandler {
	return &AuthHandler{
		firebaseService: firebaseService,
	}
}

// Verify Firebase token and return token claims
func (h *AuthHandler) VerifyToken(c *gin.Context) {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header required"})
		c.Abort()
		return
	}

	// Extract token from "Bearer <token>"
	if len(authHeader) < 7 || authHeader[:7] != "Bearer " {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid authorization header format"})
		c.Abort()
		return
	}

	idToken := authHeader[7:]

	// Verify the token with Firebase using the service
	token, err := h.firebaseService.VerifyIDToken(c.Request.Context(), idToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
		c.Abort()
		return
	}

	// Set user information in context
	c.Set("userID", token.UID)
	c.Set("firebaseToken", token)
	c.Next()
}

// Get current user info from Firebase token
func (h *AuthHandler) GetCurrentUser(c *gin.Context) {
	userID := c.GetString("userID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	// Get user from our database with role information
	db := database.GetDB()
	var user models.User
	query := `
		SELECT uid, name, phone_number, whatsapp_number, profile_image, cover_image, bio, 
		       user_type, role, followers_count, following_count, videos_count, likes_count,
		       is_verified, is_active, is_featured, tags,
		       favorite_dramas, unlocked_dramas, drama_progress,
		       created_at, updated_at, last_seen, last_post_at
		FROM users 
		WHERE uid = $1 AND is_active = true`

	err := db.Get(&user, query, userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found in database"})
		return
	}

	// Create enhanced response with role information
	response := models.UserResponse{
		User:                    user,
		RoleDisplayName:         user.Role.DisplayName(),
		CanPost:                 user.CanPost(),
		HasWhatsApp:             user.HasWhatsApp(),
		WhatsAppLink:            user.GetWhatsAppLink(),
		WhatsAppLinkWithMessage: user.GetWhatsAppLinkWithMessage(),
		HasPostedVideos:         user.HasPostedVideos(),
		LastPostTimeAgo:         user.GetLastPostTimeAgo(),
		FavoriteDramasCount:     len(user.FavoriteDramas),
		UnlockedDramasCount:     len(user.UnlockedDramas),
	}

	c.JSON(http.StatusOK, response)
}

// 🚀 UPDATED: Validate admin role with new role system
func (h *AuthHandler) RequireAdmin(c *gin.Context) {
	userID := c.GetString("userID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		c.Abort()
		return
	}

	// Check if user is admin in our database (check both old and new systems)
	db := database.GetDB()
	var userType string
	var role models.UserRole
	err := db.QueryRow("SELECT user_type, role FROM users WHERE uid = $1", userID).Scan(&userType, &role)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "User not found"})
		c.Abort()
		return
	}

	// Check admin access using both old and new role systems
	if userType != "admin" && role != models.UserRoleAdmin {
		c.JSON(http.StatusForbidden, gin.H{
			"error":        "Admin access required",
			"userRole":     role.String(),
			"allowedRoles": []string{"admin"},
		})
		c.Abort()
		return
	}

	c.Next()
}

// 🆕 NEW: Validate content creator role (admin or host)
func (h *AuthHandler) RequireContentCreator(c *gin.Context) {
	userID := c.GetString("userID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		c.Abort()
		return
	}

	// Check if user can post content
	db := database.GetDB()
	var role models.UserRole
	err := db.QueryRow("SELECT role FROM users WHERE uid = $1 AND is_active = true", userID).Scan(&role)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "User not found"})
		c.Abort()
		return
	}

	if !role.CanPost() {
		c.JSON(http.StatusForbidden, gin.H{
			"error":        "Content creation access required",
			"userRole":     role.String(),
			"allowedRoles": []string{"admin", "host", "guest"},
		})
		c.Abort()
		return
	}

	c.Next()
}

// 🚀 UPDATED: SyncUser with role support and WhatsApp number
func (h *AuthHandler) SyncUser(c *gin.Context) {
	// Get user data from request body with new fields
	var requestData struct {
		UID            string  `json:"uid" binding:"required"`
		Name           string  `json:"name"`
		PhoneNumber    string  `json:"phoneNumber"`
		WhatsappNumber *string `json:"whatsappNumber"` // NEW: Optional WhatsApp number
		ProfileImage   string  `json:"profileImage"`   // Will be empty initially, filled after R2 upload
		Bio            string  `json:"bio"`
		Role           *string `json:"role"` // NEW: Optional role (defaults to guest)
	}

	if err := c.ShouldBindJSON(&requestData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request data", "details": err.Error()})
		return
	}

	// Validate required fields
	if requestData.UID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "UID is required"})
		return
	}

	if requestData.PhoneNumber == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Phone number is required"})
		return
	}

	// Format WhatsApp number if provided
	var whatsappNumber *string
	if requestData.WhatsappNumber != nil && *requestData.WhatsappNumber != "" {
		formatted, err := models.FormatWhatsAppNumber(*requestData.WhatsappNumber)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid WhatsApp number format", "details": err.Error()})
			return
		}
		whatsappNumber = formatted
	}

	// Set default role
	role := models.UserRoleGuest
	if requestData.Role != nil {
		role = models.ParseUserRole(*requestData.Role)
	}

	// Check if user exists in our database
	db := database.GetDB()
	var existingUser models.User
	err := db.Get(&existingUser, "SELECT * FROM users WHERE uid = $1", requestData.UID)

	if err != nil {
		// User doesn't exist, create new user with role and WhatsApp support
		newUser := models.User{
			UID:            requestData.UID,
			Name:           getValidName(requestData.Name),
			PhoneNumber:    requestData.PhoneNumber,
			WhatsappNumber: whatsappNumber, // NEW: WhatsApp number
			ProfileImage:   "",             // Empty - will be uploaded to R2 during profile setup
			CoverImage:     "",             // Empty initially
			Bio:            getValidBio(requestData.Bio),
			UserType:       "user", // Keep for backward compatibility
			Role:           role,   // NEW: User role
			FollowersCount: 0,
			FollowingCount: 0,
			VideosCount:    0,
			LikesCount:     0,
			IsVerified:     false,
			IsActive:       true,
			IsFeatured:     false,
			Tags:           make(models.StringSlice, 0),

			// Initialize drama-related fields
			FavoriteDramas: make(models.StringSlice, 0),
			UnlockedDramas: make(models.StringSlice, 0),
			DramaProgress:  make(models.IntMap),

			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			LastSeen:  time.Now(),
		}

		// Validate user data
		if !newUser.IsValidForCreation() {
			errors := newUser.ValidateForCreation()
			c.JSON(http.StatusBadRequest, gin.H{"error": "Validation failed", "details": errors})
			return
		}

		// Insert new user with role and WhatsApp support
		query := `
			INSERT INTO users (uid, name, phone_number, whatsapp_number, profile_image, cover_image, bio, 
			                   user_type, role, followers_count, following_count, videos_count, likes_count,
			                   is_verified, is_active, is_featured, tags, 
			                   favorite_dramas, unlocked_dramas, drama_progress,
			                   created_at, updated_at, last_seen)
			VALUES (:uid, :name, :phone_number, :whatsapp_number, :profile_image, :cover_image, :bio, 
			        :user_type, :role, :followers_count, :following_count, :videos_count, :likes_count,
			        :is_verified, :is_active, :is_featured, :tags,
			        :favorite_dramas, :unlocked_dramas, :drama_progress,
			        :created_at, :updated_at, :last_seen)`

		_, err = db.NamedExec(query, newUser)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to create user",
				"details": err.Error(),
			})
			return
		}

		// Create enhanced response
		response := models.UserResponse{
			User:                    newUser,
			RoleDisplayName:         newUser.Role.DisplayName(),
			CanPost:                 newUser.CanPost(),
			HasWhatsApp:             newUser.HasWhatsApp(),
			WhatsAppLink:            newUser.GetWhatsAppLink(),
			WhatsAppLinkWithMessage: newUser.GetWhatsAppLinkWithMessage(),
			HasPostedVideos:         newUser.HasPostedVideos(),
			LastPostTimeAgo:         newUser.GetLastPostTimeAgo(),
			FavoriteDramasCount:     len(newUser.FavoriteDramas),
			UnlockedDramasCount:     len(newUser.UnlockedDramas),
		}

		c.JSON(http.StatusCreated, gin.H{
			"message": "User created successfully",
			"user":    response,
		})
		return
	}

	// User exists, update last seen and return existing user
	existingUser.LastSeen = time.Now()
	existingUser.UpdatedAt = time.Now()

	_, err = db.Exec("UPDATE users SET last_seen = $1, updated_at = $2 WHERE uid = $3",
		existingUser.LastSeen, existingUser.UpdatedAt, requestData.UID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to update user",
			"details": err.Error(),
		})
		return
	}

	// Create enhanced response for existing user
	response := models.UserResponse{
		User:                    existingUser,
		RoleDisplayName:         existingUser.Role.DisplayName(),
		CanPost:                 existingUser.CanPost(),
		HasWhatsApp:             existingUser.HasWhatsApp(),
		WhatsAppLink:            existingUser.GetWhatsAppLink(),
		WhatsAppLinkWithMessage: existingUser.GetWhatsAppLinkWithMessage(),
		HasPostedVideos:         existingUser.HasPostedVideos(),
		LastPostTimeAgo:         existingUser.GetLastPostTimeAgo(),
		FavoriteDramasCount:     len(existingUser.FavoriteDramas),
		UnlockedDramasCount:     len(existingUser.UnlockedDramas),
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "User synced successfully",
		"user":    response,
	})
}

// 🚀 UPDATED: SyncUserWithToken with role support
func (h *AuthHandler) SyncUserWithToken(c *gin.Context) {
	userID := c.GetString("userID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	// Get Firebase user record using the service
	firebaseUser, err := h.firebaseService.GetUser(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get Firebase user"})
		return
	}

	// Check if user exists in our database
	db := database.GetDB()
	var existingUser models.User
	query := `
		SELECT uid, name, phone_number, whatsapp_number, profile_image, cover_image, bio, 
		       user_type, role, followers_count, following_count, videos_count, likes_count,
		       is_verified, is_active, is_featured, tags,
		       favorite_dramas, unlocked_dramas, drama_progress,
		       created_at, updated_at, last_seen, last_post_at
		FROM users 
		WHERE uid = $1`

	err = db.Get(&existingUser, query, userID)

	if err != nil {
		// User doesn't exist, create new user with Firebase data and role support
		newUser := models.User{
			UID:            userID,
			Name:           getFirebaseDisplayName(firebaseUser),
			PhoneNumber:    firebaseUser.PhoneNumber,
			WhatsappNumber: nil, // Will be set later during profile setup
			ProfileImage:   "",  // Empty - will be uploaded to R2 during profile setup
			CoverImage:     "",  // Empty initially
			Bio:            "",  // Empty initially
			UserType:       "user",
			Role:           models.UserRoleGuest, // Default role for Firebase users
			FollowersCount: 0,
			FollowingCount: 0,
			VideosCount:    0,
			LikesCount:     0,
			IsVerified:     false,
			IsActive:       true,
			IsFeatured:     false,
			Tags:           make(models.StringSlice, 0),

			// Initialize drama-related fields
			FavoriteDramas: make(models.StringSlice, 0),
			UnlockedDramas: make(models.StringSlice, 0),
			DramaProgress:  make(models.IntMap),

			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			LastSeen:  time.Now(),
		}

		insertQuery := `
			INSERT INTO users (uid, name, phone_number, whatsapp_number, profile_image, cover_image, bio, 
			                   user_type, role, followers_count, following_count, videos_count, likes_count,
			                   is_verified, is_active, is_featured, tags,
			                   favorite_dramas, unlocked_dramas, drama_progress,
			                   created_at, updated_at, last_seen)
			VALUES (:uid, :name, :phone_number, :whatsapp_number, :profile_image, :cover_image, :bio, 
			        :user_type, :role, :followers_count, :following_count, :videos_count, :likes_count,
			        :is_verified, :is_active, :is_featured, :tags,
			        :favorite_dramas, :unlocked_dramas, :drama_progress,
			        :created_at, :updated_at, :last_seen)`

		_, err = db.NamedExec(insertQuery, newUser)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
			return
		}

		// Create enhanced response
		response := models.UserResponse{
			User:                    newUser,
			RoleDisplayName:         newUser.Role.DisplayName(),
			CanPost:                 newUser.CanPost(),
			HasWhatsApp:             newUser.HasWhatsApp(),
			WhatsAppLink:            newUser.GetWhatsAppLink(),
			WhatsAppLinkWithMessage: newUser.GetWhatsAppLinkWithMessage(),
			HasPostedVideos:         newUser.HasPostedVideos(),
			LastPostTimeAgo:         newUser.GetLastPostTimeAgo(),
			FavoriteDramasCount:     len(newUser.FavoriteDramas),
			UnlockedDramasCount:     len(newUser.UnlockedDramas),
		}

		c.JSON(http.StatusCreated, gin.H{
			"message": "User created successfully",
			"user":    response,
		})
		return
	}

	// User exists, update last seen
	existingUser.LastSeen = time.Now()
	existingUser.UpdatedAt = time.Now()

	_, err = db.Exec("UPDATE users SET last_seen = $1, updated_at = $2 WHERE uid = $3",
		existingUser.LastSeen, existingUser.UpdatedAt, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user"})
		return
	}

	// Create enhanced response for existing user
	response := models.UserResponse{
		User:                    existingUser,
		RoleDisplayName:         existingUser.Role.DisplayName(),
		CanPost:                 existingUser.CanPost(),
		HasWhatsApp:             existingUser.HasWhatsApp(),
		WhatsAppLink:            existingUser.GetWhatsAppLink(),
		WhatsAppLinkWithMessage: existingUser.GetWhatsAppLinkWithMessage(),
		HasPostedVideos:         existingUser.HasPostedVideos(),
		LastPostTimeAgo:         existingUser.GetLastPostTimeAgo(),
		FavoriteDramasCount:     len(existingUser.FavoriteDramas),
		UnlockedDramasCount:     len(existingUser.UnlockedDramas),
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "User synced successfully",
		"user":    response,
	})
}

// Helper function to get valid display name
func getValidName(name string) string {
	if name != "" && len(name) >= 2 {
		return name
	}
	return "User" // Default name if empty or too short
}

// Helper function to get valid bio
func getValidBio(bio string) string {
	if bio != "" {
		return bio
	}
	return "" // Empty bio is fine, will be filled later
}

// Helper function to safely extract Firebase display name
func getFirebaseDisplayName(user *auth.UserRecord) string {
	if user.DisplayName != "" {
		return user.DisplayName
	}
	return "User" // Default name
}
