// ===============================
// internal/middleware/auth.go - Firebase Auth Middleware
// ===============================

package middleware

import (
	"context"
	"net/http"
	"strings"

	"weibaobe/internal/database"
	"weibaobe/internal/models"

	"firebase.google.com/go/v4/auth"
	"github.com/gin-gonic/gin"
)

func FirebaseAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header required"})
			c.Abort()
			return
		}

		// Extract token from "Bearer <token>"
		tokenParts := strings.Split(authHeader, " ")
		if len(tokenParts) != 2 || tokenParts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid authorization header format"})
			c.Abort()
			return
		}

		token := tokenParts[1]

		// Verify Firebase token
		firebaseToken, err := verifyFirebaseToken(c.Request.Context(), token)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			c.Abort()
			return
		}

		// Set user ID in context
		c.Set("userID", firebaseToken.UID)
		c.Set("firebaseToken", firebaseToken)
		c.Next()
	}
}

func AdminOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetString("userID")
		if userID == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
			c.Abort()
			return
		}

		// Check if user is admin
		db := database.GetDB()
		var user models.User
		err := db.Get(&user, "SELECT user_type FROM users WHERE uid = $1", userID)
		if err != nil {
			c.JSON(http.StatusForbidden, gin.H{"error": "User not found"})
			c.Abort()
			return
		}

		if !user.IsAdmin() {
			c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
			c.Abort()
			return
		}

		c.Next()
	}
}

// You'll need to implement this with Firebase Admin SDK
func verifyFirebaseToken(ctx context.Context, idToken string) (*auth.Token, error) {
	// Initialize Firebase Admin SDK here
	// This is a placeholder - implement with proper Firebase Admin SDK
	// Example:
	// client, err := app.Auth(ctx)
	// if err != nil {
	//     return nil, err
	// }
	// return client.VerifyIDToken(ctx, idToken)

	// Placeholder implementation
	return &auth.Token{UID: "placeholder"}, nil
}
