// ===============================
// internal/handlers/auth.go - Fixed Authentication Handler
// ===============================

package handlers

import (
	"context"
	"net/http"
	"time"

	"weibaobe/internal/config"
	"weibaobe/internal/database"
	"weibaobe/internal/models"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/auth"
	"github.com/gin-gonic/gin"
	"google.golang.org/api/option"
)

type AuthHandler struct {
	firebaseAuth *auth.Client
}

func NewAuthHandler(cfg *config.Config) (*AuthHandler, error) {
	// Initialize Firebase Admin SDK
	opt := option.WithCredentialsFile(cfg.FirebaseCredentials)

	firebaseApp, err := firebase.NewApp(context.Background(), &firebase.Config{
		ProjectID: cfg.FirebaseProjectID,
	}, opt)
	if err != nil {
		return nil, err
	}

	authClient, err := firebaseApp.Auth(context.Background())
	if err != nil {
		return nil, err
	}

	return &AuthHandler{
		firebaseAuth: authClient,
	}, nil
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

	// Verify the token with Firebase
	token, err := h.firebaseAuth.VerifyIDToken(c.Request.Context(), idToken)
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
	err := db.Get(&user, "SELECT * FROM users WHERE uid = $1", userID)
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

// Sync Firebase user with our database
func (h *AuthHandler) SyncUser(c *gin.Context) {
	userID := c.GetString("userID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	// Get Firebase user record
	firebaseUser, err := h.firebaseAuth.GetUser(c.Request.Context(), userID)
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
			Name:           firebaseUser.DisplayName,
			Email:          firebaseUser.Email,
			PhoneNumber:    firebaseUser.PhoneNumber,
			ProfileImage:   firebaseUser.PhotoURL,
			UserType:       "viewer", // Default to viewer
			CoinsBalance:   0,
			FavoriteDramas: make(models.StringSlice, 0),
			WatchHistory:   make(models.StringSlice, 0),
			DramaProgress:  make(models.IntMap),
			UnlockedDramas: make(models.StringSlice, 0),
			Preferences: models.UserPreferences{
				AutoPlay:             true,
				ReceiveNotifications: true,
				DarkMode:             false,
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			LastSeen:  time.Now(),
		}

		query := `
			INSERT INTO users (uid, name, email, phone_number, profile_image, bio, user_type, 
			                   coins_balance, favorite_dramas, watch_history, drama_progress, 
			                   unlocked_dramas, preferences, created_at, updated_at, last_seen)
			VALUES (:uid, :name, :email, :phone_number, :profile_image, :bio, :user_type, 
			        :coins_balance, :favorite_dramas, :watch_history, :drama_progress, 
			        :unlocked_dramas, :preferences, :created_at, :updated_at, :last_seen)`

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
