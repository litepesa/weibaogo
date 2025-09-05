// ===============================
// internal/handlers/user.go - Video Social Media User Handler
// ===============================

package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"weibaobe/internal/models"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
)

type UserHandler struct {
	db *sqlx.DB
}

func NewUserHandler(db *sqlx.DB) *UserHandler {
	return &UserHandler{db: db}
}

func (h *UserHandler) CreateUser(c *gin.Context) {
	var user models.User
	if err := c.ShouldBindJSON(&user); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user.CreatedAt = time.Now()
	user.UpdatedAt = time.Now()
	user.LastSeen = time.Now()

	// Ensure default values
	if user.UserType == "" {
		user.UserType = "user"
	}
	if user.Tags == nil {
		user.Tags = make(models.StringSlice, 0)
	}

	// Validate user
	if !user.IsValidForCreation() {
		errors := user.ValidateForCreation()
		c.JSON(http.StatusBadRequest, gin.H{"error": "Validation failed", "details": errors})
		return
	}

	query := `
		INSERT INTO users (uid, name, phone_number, profile_image, cover_image, bio, user_type, 
		                   followers_count, following_count, videos_count, likes_count,
		                   is_verified, is_active, is_featured, tags, created_at, updated_at, last_seen)
		VALUES (:uid, :name, :phone_number, :profile_image, :cover_image, :bio, :user_type, 
		        :followers_count, :following_count, :videos_count, :likes_count,
		        :is_verified, :is_active, :is_featured, :tags, :created_at, :updated_at, :last_seen)
		ON CONFLICT (uid) DO UPDATE SET
		name = EXCLUDED.name,
		phone_number = EXCLUDED.phone_number,
		profile_image = EXCLUDED.profile_image,
		cover_image = EXCLUDED.cover_image,
		bio = EXCLUDED.bio,
		updated_at = EXCLUDED.updated_at,
		last_seen = EXCLUDED.last_seen`

	_, err := h.db.NamedExec(query, user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"uid":     user.UID,
		"message": "User created successfully",
	})
}

func (h *UserHandler) GetUser(c *gin.Context) {
	userID := c.Param("uid")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "User ID required"})
		return
	}

	var user models.User
	query := `SELECT * FROM users WHERE uid = $1 AND is_active = true`
	err := h.db.Get(&user, query, userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	c.JSON(http.StatusOK, user)
}

func (h *UserHandler) UpdateUser(c *gin.Context) {
	userID := c.Param("uid")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "User ID required"})
		return
	}

	// Verify user can only update their own profile (unless admin)
	requestingUserID := c.GetString("userID")
	if requestingUserID != userID {
		// Check if requesting user is admin
		var requestingUser models.User
		err := h.db.Get(&requestingUser, "SELECT user_type FROM users WHERE uid = $1", requestingUserID)
		if err != nil || !requestingUser.IsAdmin() {
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
			return
		}
	}

	var user models.User
	if err := c.ShouldBindJSON(&user); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user.UID = userID
	user.UpdatedAt = time.Now()
	user.LastSeen = time.Now()

	query := `
		UPDATE users SET 
			name = :name, 
			phone_number = :phone_number, 
			profile_image = :profile_image,
			cover_image = :cover_image,
			bio = :bio, 
			tags = :tags,
			updated_at = :updated_at, 
			last_seen = :last_seen
		WHERE uid = :uid`

	_, err := h.db.NamedExec(query, user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "User updated successfully"})
}

func (h *UserHandler) DeleteUser(c *gin.Context) {
	userID := c.Param("uid")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "User ID required"})
		return
	}

	// Only allow users to delete their own account or admin to delete any
	requestingUserID := c.GetString("userID")
	if requestingUserID != userID {
		// Check if requesting user is admin
		var requestingUser models.User
		err := h.db.Get(&requestingUser, "SELECT user_type FROM users WHERE uid = $1", requestingUserID)
		if err != nil || !requestingUser.IsAdmin() {
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
			return
		}
	}

	// Use transaction to delete user and related data
	tx, err := h.db.Beginx()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start transaction"})
		return
	}
	defer tx.Rollback()

	// Delete user follows
	_, err = tx.Exec("DELETE FROM user_follows WHERE follower_id = $1 OR following_id = $1", userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete user follows"})
		return
	}

	// Delete comment likes
	_, err = tx.Exec(`
		DELETE FROM comment_likes 
		WHERE user_id = $1 OR comment_id IN (
			SELECT id FROM comments WHERE author_id = $1
		)`, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete comment likes"})
		return
	}

	// Delete video likes
	_, err = tx.Exec("DELETE FROM video_likes WHERE user_id = $1", userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete video likes"})
		return
	}

	// Delete comments
	_, err = tx.Exec("DELETE FROM comments WHERE author_id = $1", userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete comments"})
		return
	}

	// Delete videos
	_, err = tx.Exec("DELETE FROM videos WHERE user_id = $1", userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete videos"})
		return
	}

	// Delete wallet transactions
	_, err = tx.Exec("DELETE FROM wallet_transactions WHERE user_id = $1", userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete wallet transactions"})
		return
	}

	// Delete wallet
	_, err = tx.Exec("DELETE FROM wallets WHERE user_id = $1", userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete wallet"})
		return
	}

	// Delete purchase requests
	_, err = tx.Exec("DELETE FROM coin_purchase_requests WHERE user_id = $1", userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete purchase requests"})
		return
	}

	// Delete user
	_, err = tx.Exec("DELETE FROM users WHERE uid = $1", userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete user"})
		return
	}

	if err = tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "User deleted successfully"})
}

func (h *UserHandler) GetAllUsers(c *gin.Context) {
	limit := 50
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 1000 {
			limit = parsed
		}
	}

	offset := 0
	if o := c.Query("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	// Optional filters
	var whereClause string
	var args []interface{}
	argIndex := 1

	whereClause = "WHERE is_active = true"

	if userType := c.Query("userType"); userType != "" {
		whereClause += fmt.Sprintf(" AND user_type = $%d", argIndex)
		args = append(args, userType)
		argIndex++
	}

	if verified := c.Query("verified"); verified != "" {
		if verified == "true" {
			whereClause += " AND is_verified = true"
		} else if verified == "false" {
			whereClause += " AND is_verified = false"
		}
	}

	if query := c.Query("q"); query != "" {
		whereClause += fmt.Sprintf(" AND (name ILIKE $%d OR phone_number ILIKE $%d)", argIndex, argIndex)
		searchPattern := "%" + query + "%"
		args = append(args, searchPattern)
		argIndex++
	}

	// Add pagination
	limitOffset := fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", argIndex, argIndex+1)
	args = append(args, limit, offset)

	var users []models.User
	query := "SELECT * FROM users " + whereClause + limitOffset
	err := h.db.Select(&users, query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch users"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"users": users,
		"total": len(users),
	})
}

func (h *UserHandler) SearchUsers(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Search query required"})
		return
	}

	limit := 20
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}

	searchPattern := "%" + query + "%"

	var users []models.User
	searchQuery := `
		SELECT * FROM users 
		WHERE is_active = true AND (
			name ILIKE $1 OR 
			phone_number ILIKE $1 OR
			bio ILIKE $1
		)
		ORDER BY 
			CASE WHEN name ILIKE $1 THEN 1 ELSE 2 END,
			followers_count DESC,
			created_at DESC 
		LIMIT $2`

	err := h.db.Select(&users, searchQuery, searchPattern, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to search users"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"users": users,
		"total": len(users),
		"query": query,
	})
}

func (h *UserHandler) GetUserStats(c *gin.Context) {
	userID := c.Param("uid")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "User ID required"})
		return
	}

	// Get user with basic stats
	var user models.User
	err := h.db.Get(&user, "SELECT * FROM users WHERE uid = $1 AND is_active = true", userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Get additional stats
	var totalViews, totalLikes int
	err = h.db.QueryRow(`
		SELECT 
			COALESCE(SUM(views_count), 0) as total_views,
			COALESCE(SUM(likes_count), 0) as total_likes
		FROM videos 
		WHERE user_id = $1 AND is_active = true`, userID).Scan(&totalViews, &totalLikes)
	if err != nil {
		totalViews = 0
		totalLikes = 0
	}

	stats := gin.H{
		"user":           user,
		"totalViews":     totalViews,
		"totalLikes":     totalLikes,
		"videosCount":    user.VideosCount,
		"followersCount": user.FollowersCount,
		"followingCount": user.FollowingCount,
		"engagementRate": user.GetEngagementRate(),
		"joinDate":       user.CreatedAt,
		"lastActiveDate": user.LastSeen,
	}

	c.JSON(http.StatusOK, stats)
}

func (h *UserHandler) UpdateUserStatus(c *gin.Context) {
	userID := c.Param("uid")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "User ID required"})
		return
	}

	var request struct {
		IsActive   *bool  `json:"isActive"`
		IsVerified *bool  `json:"isVerified"`
		IsFeatured *bool  `json:"isFeatured"`
		UserType   string `json:"userType"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Build dynamic update query
	setParts := []string{"updated_at = $1"}
	args := []interface{}{time.Now()}
	argIndex := 2

	if request.IsActive != nil {
		setParts = append(setParts, fmt.Sprintf("is_active = $%d", argIndex))
		args = append(args, *request.IsActive)
		argIndex++
	}

	if request.IsVerified != nil {
		setParts = append(setParts, fmt.Sprintf("is_verified = $%d", argIndex))
		args = append(args, *request.IsVerified)
		argIndex++
	}

	if request.IsFeatured != nil {
		setParts = append(setParts, fmt.Sprintf("is_featured = $%d", argIndex))
		args = append(args, *request.IsFeatured)
		argIndex++
	}

	if request.UserType != "" {
		setParts = append(setParts, fmt.Sprintf("user_type = $%d", argIndex))
		args = append(args, request.UserType)
		argIndex++
	}

	if len(setParts) == 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No fields to update"})
		return
	}

	query := fmt.Sprintf("UPDATE users SET %s WHERE uid = $%d",
		strings.Join(setParts, ", "), argIndex)
	args = append(args, userID)

	result, err := h.db.Exec(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user status"})
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check update result"})
		return
	}

	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "User status updated successfully"})
}
