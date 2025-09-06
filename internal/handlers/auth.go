// ===============================
// internal/handlers/auth.go - Video Social Media Auth Handler (Updated with Sync)
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

	// Get user from our database
	db := database.GetDB()
	var user models.User
	err := db.Get(&user, "SELECT * FROM users WHERE uid = $1 AND is_active = true", userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found in database"})
		return
	}

	c.JSON(http.StatusOK, user)
}

// Validate admin role
func (h *AuthHandler) RequireAdmin(c *gin.Context) {
	userID := c.GetString("userID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		c.Abort()
		return
	}

	// Check if user is admin in our database
	db := database.GetDB()
	var userType string
	err := db.QueryRow("SELECT user_type FROM users WHERE uid = $1", userID).Scan(&userType)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "User not found"})
		c.Abort()
		return
	}

	if userType != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
		c.Abort()
		return
	}

	c.Next()
}

// NEW: SyncUser handles the user sync after Firebase authentication (NO AUTH MIDDLEWARE REQUIRED)
// This endpoint solves the chicken-and-egg problem by creating users without requiring backend auth
func (h *AuthHandler) SyncUser(c *gin.Context) {
	// Get user data from request body (sent from frontend after Firebase auth)
	var requestData struct {
		UID          string `json:"uid" binding:"required"`
		Name         string `json:"name"`
		Email        string `json:"email"`
		PhoneNumber  string `json:"phoneNumber"`
		ProfileImage string `json:"profileImage"`
		Bio          string `json:"bio"`
	}

	if err := c.ShouldBindJSON(&requestData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request data"})
		return
	}

	// Validate that we have a UID
	if requestData.UID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "UID is required"})
		return
	}

	// Check if user exists in our database
	db := database.GetDB()
	var existingUser models.User
	err := db.Get(&existingUser, "SELECT * FROM users WHERE uid = $1", requestData.UID)

	if err != nil {
		// User doesn't exist, create new user with minimal data
		newUser := models.User{
			UID:            requestData.UID,
			Name:           getValidName(requestData.Name),
			PhoneNumber:    requestData.PhoneNumber,
			ProfileImage:   requestData.ProfileImage,
			Bio:            getValidBio(requestData.Bio),
			UserType:       "user", // Default to user
			FollowersCount: 0,
			FollowingCount: 0,
			VideosCount:    0,
			LikesCount:     0,
			IsVerified:     false,
			IsActive:       true,
			IsFeatured:     false,
			Tags:           make(models.StringSlice, 0),
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
			LastSeen:       time.Now(),
		}

		query := `
			INSERT INTO users (uid, name, phone_number, profile_image, cover_image, bio, email, user_type, 
			                   followers_count, following_count, videos_count, likes_count,
			                   is_verified, is_active, is_featured, tags, created_at, updated_at, last_seen)
			VALUES (:uid, :name, :phone_number, :profile_image, :cover_image, :bio, :email, :user_type, 
			        :followers_count, :following_count, :videos_count, :likes_count,
			        :is_verified, :is_active, :is_featured, :tags, :created_at, :updated_at, :last_seen)`

		_, err = db.NamedExec(query, newUser)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
			return
		}

		c.JSON(http.StatusCreated, gin.H{
			"message": "User created successfully",
			"user":    newUser,
		})
		return
	}

	// User exists, update last seen and return existing user
	existingUser.LastSeen = time.Now()
	existingUser.UpdatedAt = time.Now()

	_, err = db.Exec("UPDATE users SET last_seen = $1, updated_at = $2 WHERE uid = $3",
		existingUser.LastSeen, existingUser.UpdatedAt, requestData.UID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "User synced successfully",
		"user":    existingUser,
	})
}

// Alternative sync endpoint that uses Firebase token for validation
// This is for cases where you want to verify the Firebase token before syncing
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
	err = db.Get(&existingUser, "SELECT * FROM users WHERE uid = $1", userID)

	if err != nil {
		// User doesn't exist, create new user with Firebase data
		newUser := models.User{
			UID:            userID,
			Name:           getFirebaseDisplayName(firebaseUser),
			PhoneNumber:    firebaseUser.PhoneNumber,
			ProfileImage:   getFirebasePhotoURL(firebaseUser),
			UserType:       "user", // Default to user
			FollowersCount: 0,
			FollowingCount: 0,
			VideosCount:    0,
			LikesCount:     0,
			IsVerified:     false,
			IsActive:       true,
			IsFeatured:     false,
			Tags:           make(models.StringSlice, 0),
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
			LastSeen:       time.Now(),
		}

		query := `
			INSERT INTO users (uid, name, phone_number, profile_image, cover_image, bio, email, user_type, 
			                   followers_count, following_count, videos_count, likes_count,
			                   is_verified, is_active, is_featured, tags, created_at, updated_at, last_seen)
			VALUES (:uid, :name, :phone_number, :profile_image, :cover_image, :bio, :email, :user_type, 
			        :followers_count, :following_count, :videos_count, :likes_count,
			        :is_verified, :is_active, :is_featured, :tags, :created_at, :updated_at, :last_seen)`

		_, err = db.NamedExec(query, newUser)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
			return
		}

		c.JSON(http.StatusCreated, gin.H{
			"message": "User created successfully",
			"user":    newUser,
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

	c.JSON(http.StatusOK, gin.H{
		"message": "User synced successfully",
		"user":    existingUser,
	})
}

// Legacy method: Sync Firebase user with our backend database
// This method uses the authenticated middleware and queries Firebase directly
func (h *AuthHandler) SyncUserLegacy(c *gin.Context) {
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
	err = db.Get(&existingUser, "SELECT * FROM users WHERE uid = $1", userID)

	if err != nil {
		// User doesn't exist, create new user
		newUser := models.User{
			UID:            userID,
			Name:           getFirebaseDisplayName(firebaseUser),
			PhoneNumber:    firebaseUser.PhoneNumber,
			ProfileImage:   getFirebasePhotoURL(firebaseUser),
			UserType:       "user", // Default to user
			FollowersCount: 0,
			FollowingCount: 0,
			VideosCount:    0,
			LikesCount:     0,
			IsVerified:     false,
			IsActive:       true,
			IsFeatured:     false,
			Tags:           make(models.StringSlice, 0),
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
			LastSeen:       time.Now(),
		}

		// Ensure we have a name
		if newUser.Name == "" {
			newUser.Name = "User" // Default name
		}

		query := `
			INSERT INTO users (uid, name, phone_number, profile_image, cover_image, bio, email, user_type, 
			                   followers_count, following_count, videos_count, likes_count,
			                   is_verified, is_active, is_featured, tags, created_at, updated_at, last_seen)
			VALUES (:uid, :name, :phone_number, :profile_image, :cover_image, :bio, :email, :user_type, 
			        :followers_count, :following_count, :videos_count, :likes_count,
			        :is_verified, :is_active, :is_featured, :tags, :created_at, :updated_at, :last_seen)`

		_, err = db.NamedExec(query, newUser)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
			return
		}

		c.JSON(http.StatusCreated, gin.H{
			"message": "User created successfully",
			"user":    newUser,
		})
		return
	}

	// User exists, update last seen
	existingUser.LastSeen = time.Now()
	_, err = db.Exec("UPDATE users SET last_seen = $1 WHERE uid = $2",
		existingUser.LastSeen, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "User synced successfully",
		"user":    existingUser,
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

// Helper functions to safely extract Firebase user data
func getFirebaseDisplayName(user *auth.UserRecord) string {
	if user.DisplayName != "" {
		return user.DisplayName
	}
	return "User" // Default name
}

func getFirebasePhotoURL(user *auth.UserRecord) string {
	if user.PhotoURL != "" {
		return user.PhotoURL
	}
	return "" // Empty string for no photo
}
