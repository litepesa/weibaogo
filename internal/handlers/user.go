// ===============================
// internal/handlers/user.go - Minimal Update (Only Remove Balance from Queries)
// ===============================

package handlers

import (
	"net/http"
	"strconv"
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
		user.UserType = "viewer"
	}
	if user.FavoriteDramas == nil {
		user.FavoriteDramas = make(models.StringSlice, 0)
	}
	if user.WatchHistory == nil {
		user.WatchHistory = make(models.StringSlice, 0)
	}
	if user.DramaProgress == nil {
		user.DramaProgress = make(models.IntMap)
	}
	if user.UnlockedDramas == nil {
		user.UnlockedDramas = make(models.StringSlice, 0)
	}

	// UPDATED: Removed coins_balance from INSERT query
	query := `
		INSERT INTO users (uid, name, email, phone_number, profile_image, bio, user_type, 
		                   favorite_dramas, watch_history, drama_progress, 
		                   unlocked_dramas, preferences, created_at, updated_at, last_seen)
		VALUES (:uid, :name, :email, :phone_number, :profile_image, :bio, :user_type, 
		        :favorite_dramas, :watch_history, :drama_progress, 
		        :unlocked_dramas, :preferences, :created_at, :updated_at, :last_seen)
		ON CONFLICT (uid) DO UPDATE SET
		name = EXCLUDED.name,
		email = EXCLUDED.email,
		phone_number = EXCLUDED.phone_number,
		profile_image = EXCLUDED.profile_image,
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
	query := `SELECT * FROM users WHERE uid = $1`
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

	// UPDATED: Removed coins_balance from UPDATE query
	query := `
		UPDATE users SET 
			name = :name, 
			email = :email, 
			phone_number = :phone_number, 
			profile_image = :profile_image,
			bio = :bio, 
			favorite_dramas = :favorite_dramas,
			watch_history = :watch_history, 
			drama_progress = :drama_progress, 
			unlocked_dramas = :unlocked_dramas,
			preferences = :preferences, 
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

func (h *UserHandler) ToggleFavorite(c *gin.Context) {
	userID := c.Param("uid")

	// Verify user can only update their own favorites
	requestingUserID := c.GetString("userID")
	if requestingUserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	var request struct {
		DramaID string `json:"dramaId" binding:"required"`
		Action  string `json:"action" binding:"required"` // "add" or "remove"
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var user models.User
	err := h.db.Get(&user, "SELECT favorite_dramas FROM users WHERE uid = $1", userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	favorites := []string(user.FavoriteDramas)

	if request.Action == "add" {
		// Add to favorites if not already there
		found := false
		for _, id := range favorites {
			if id == request.DramaID {
				found = true
				break
			}
		}
		if !found {
			favorites = append(favorites, request.DramaID)
		}
	} else if request.Action == "remove" {
		// Remove from favorites
		for i, id := range favorites {
			if id == request.DramaID {
				favorites = append(favorites[:i], favorites[i+1:]...)
				break
			}
		}
	}

	// Update user favorites
	user.FavoriteDramas = models.StringSlice(favorites)
	user.UpdatedAt = time.Now()

	_, err = h.db.Exec("UPDATE users SET favorite_dramas = $1, updated_at = $2 WHERE uid = $3",
		user.FavoriteDramas, user.UpdatedAt, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update favorites"})
		return
	}

	// Update drama favorite count
	if request.Action == "add" {
		h.db.Exec("UPDATE dramas SET favorite_count = favorite_count + 1 WHERE drama_id = $1", request.DramaID)
	} else {
		h.db.Exec("UPDATE dramas SET favorite_count = favorite_count - 1 WHERE drama_id = $1 AND favorite_count > 0", request.DramaID)
	}

	c.JSON(http.StatusOK, gin.H{"message": "Favorites updated successfully"})
}

func (h *UserHandler) AddToWatchHistory(c *gin.Context) {
	userID := c.Param("uid")

	// Verify user can only update their own watch history
	requestingUserID := c.GetString("userID")
	if requestingUserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	var request struct {
		EpisodeID string `json:"episodeId" binding:"required"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var user models.User
	err := h.db.Get(&user, "SELECT watch_history FROM users WHERE uid = $1", userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	history := []string(user.WatchHistory)

	// Add to history if not already there
	found := false
	for _, id := range history {
		if id == request.EpisodeID {
			found = true
			break
		}
	}

	if !found {
		history = append(history, request.EpisodeID)

		// Limit history to last 1000 episodes
		if len(history) > 1000 {
			history = history[len(history)-1000:]
		}

		user.WatchHistory = models.StringSlice(history)
		user.UpdatedAt = time.Now()

		_, err = h.db.Exec("UPDATE users SET watch_history = $1, updated_at = $2 WHERE uid = $3",
			user.WatchHistory, user.UpdatedAt, userID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update watch history"})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "Watch history updated successfully"})
}

func (h *UserHandler) UpdateDramaProgress(c *gin.Context) {
	userID := c.Param("uid")

	// Verify user can only update their own progress
	requestingUserID := c.GetString("userID")
	if requestingUserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	var request struct {
		DramaID       string `json:"dramaId" binding:"required"`
		EpisodeNumber int    `json:"episodeNumber" binding:"required"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var user models.User
	err := h.db.Get(&user, "SELECT drama_progress FROM users WHERE uid = $1", userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	progress := map[string]int(user.DramaProgress)
	if progress == nil {
		progress = make(map[string]int)
	}

	// Update progress only if new episode number is higher
	if currentProgress, exists := progress[request.DramaID]; !exists || request.EpisodeNumber > currentProgress {
		progress[request.DramaID] = request.EpisodeNumber

		user.DramaProgress = models.IntMap(progress)
		user.UpdatedAt = time.Now()

		_, err = h.db.Exec("UPDATE users SET drama_progress = $1, updated_at = $2 WHERE uid = $3",
			user.DramaProgress, user.UpdatedAt, userID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update drama progress"})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "Drama progress updated successfully"})
}

func (h *UserHandler) GetFavorites(c *gin.Context) {
	userID := c.Param("uid")

	// Users can only view their own favorites unless admin
	requestingUserID := c.GetString("userID")
	if requestingUserID != userID {
		var requestingUser models.User
		err := h.db.Get(&requestingUser, "SELECT user_type FROM users WHERE uid = $1", requestingUserID)
		if err != nil || !requestingUser.IsAdmin() {
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
			return
		}
	}

	query := `
		SELECT d.* FROM dramas d
		JOIN users u ON u.uid = $1
		WHERE d.drama_id = ANY(
			SELECT jsonb_array_elements_text(u.favorite_dramas)
		) AND d.is_active = true
		ORDER BY d.created_at DESC`

	var dramas []models.Drama
	err := h.db.Select(&dramas, query, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch favorites"})
		return
	}

	c.JSON(http.StatusOK, dramas)
}

func (h *UserHandler) GetContinueWatching(c *gin.Context) {
	userID := c.Param("uid")

	// Users can only view their own continue watching unless admin
	requestingUserID := c.GetString("userID")
	if requestingUserID != userID {
		var requestingUser models.User
		err := h.db.Get(&requestingUser, "SELECT user_type FROM users WHERE uid = $1", requestingUserID)
		if err != nil || !requestingUser.IsAdmin() {
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
			return
		}
	}

	query := `
		SELECT d.* FROM dramas d
		JOIN users u ON u.uid = $1
		WHERE d.drama_id IN (
			SELECT key FROM jsonb_each_text(u.drama_progress)
		) AND d.is_active = true
		ORDER BY d.updated_at DESC`

	var dramas []models.Drama
	err := h.db.Select(&dramas, query, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch continue watching"})
		return
	}

	c.JSON(http.StatusOK, dramas)
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

	var users []models.User
	query := `SELECT * FROM users ORDER BY created_at DESC LIMIT $1 OFFSET $2`
	err := h.db.Select(&users, query, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch users"})
		return
	}

	c.JSON(http.StatusOK, users)
}
