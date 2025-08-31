// ===============================
// internal/handlers/drama.go - REFINED WITH SIMPLIFIED EPISODE MANAGEMENT AND UNLOCK TRACKING
// ===============================

package handlers

import (
	"net/http"
	"strconv"

	"weibaobe/internal/models"
	"weibaobe/internal/services"

	"github.com/gin-gonic/gin"
)

type DramaHandler struct {
	service *services.DramaService
}

func NewDramaHandler(service *services.DramaService) *DramaHandler {
	return &DramaHandler{service: service}
}

// ===============================
// PUBLIC DRAMA ENDPOINTS (unchanged)
// ===============================

func (h *DramaHandler) GetDramas(c *gin.Context) {
	limit := 20
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}

	offset := 0
	if o := c.Query("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	// Check for premium filter
	var premiumFilter *bool
	if p := c.Query("premium"); p != "" {
		if p == "true" {
			val := true
			premiumFilter = &val
		} else if p == "false" {
			val := false
			premiumFilter = &val
		}
	}

	dramas, err := h.service.GetDramas(c.Request.Context(), limit, offset, premiumFilter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch dramas"})
		return
	}

	c.JSON(http.StatusOK, dramas)
}

func (h *DramaHandler) GetFeaturedDramas(c *gin.Context) {
	limit := 10
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 50 {
			limit = parsed
		}
	}

	dramas, err := h.service.GetFeaturedDramas(c.Request.Context(), limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch featured dramas"})
		return
	}

	c.JSON(http.StatusOK, dramas)
}

func (h *DramaHandler) GetTrendingDramas(c *gin.Context) {
	limit := 10
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 50 {
			limit = parsed
		}
	}

	dramas, err := h.service.GetTrendingDramas(c.Request.Context(), limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch trending dramas"})
		return
	}

	c.JSON(http.StatusOK, dramas)
}

func (h *DramaHandler) SearchDramas(c *gin.Context) {
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

	dramas, err := h.service.SearchDramas(c.Request.Context(), query, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to search dramas"})
		return
	}

	c.JSON(http.StatusOK, dramas)
}

func (h *DramaHandler) GetDrama(c *gin.Context) {
	dramaID := c.Param("dramaId")
	if dramaID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Drama ID required"})
		return
	}

	drama, err := h.service.GetDrama(c.Request.Context(), dramaID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Drama not found"})
		return
	}

	c.JSON(http.StatusOK, drama)
}

// ===============================
// DRAMA UNLOCK ENDPOINT - UPDATED TO 99 COINS
// ===============================

func (h *DramaHandler) UnlockDrama(c *gin.Context) {
	userID := c.GetString("userID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	var request struct {
		DramaID string `json:"dramaId" binding:"required"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	success, newBalance, err := h.service.UnlockDrama(c.Request.Context(), userID, request.DramaID)
	if err != nil {
		switch err.Error() {
		case "insufficient_funds":
			c.JSON(http.StatusBadRequest, gin.H{
				"error":    "Insufficient coins. Drama unlock costs 99 coins.",
				"required": 99,
			})
		case "already_unlocked":
			c.JSON(http.StatusBadRequest, gin.H{"error": "Drama already unlocked"})
		case "drama_not_found":
			c.JSON(http.StatusNotFound, gin.H{"error": "Drama not found"})
		case "drama_free":
			c.JSON(http.StatusBadRequest, gin.H{"error": "This drama is free to watch"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to unlock drama"})
		}
		return
	}

	if !success {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to unlock drama"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    "Drama unlocked successfully",
		"newBalance": newBalance,
		"unlockCost": 99,
	})
}

// ===============================
// DRAMA INTERACTION ENDPOINTS (unchanged)
// ===============================

func (h *DramaHandler) IncrementViews(c *gin.Context) {
	dramaID := c.Param("dramaId")
	if dramaID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Drama ID required"})
		return
	}

	err := h.service.IncrementDramaViews(c.Request.Context(), dramaID)
	if err != nil {
		// Don't return error for view counting failures
		c.JSON(http.StatusOK, gin.H{"message": "View counted"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "View counted successfully"})
}

func (h *DramaHandler) ToggleFavorite(c *gin.Context) {
	dramaID := c.Param("dramaId")
	if dramaID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Drama ID required"})
		return
	}

	var request struct {
		IsAdding bool `json:"isAdding"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := h.service.IncrementDramaFavorites(c.Request.Context(), dramaID, request.IsAdding)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update favorites"})
		return
	}

	action := "added to"
	if !request.IsAdding {
		action = "removed from"
	}

	c.JSON(http.StatusOK, gin.H{"message": "Drama " + action + " favorites"})
}

// ===============================
// ADMIN DRAMA MANAGEMENT - WITH OWNERSHIP CHECKS
// ===============================

func (h *DramaHandler) CreateDramaWithEpisodes(c *gin.Context) {
	userID := c.GetString("userID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	var request struct {
		Title             string   `json:"title" binding:"required"`
		Description       string   `json:"description" binding:"required"`
		BannerImage       string   `json:"bannerImage"`
		EpisodeVideos     []string `json:"episodeVideos" binding:"required,min=1,max=100"`
		IsPremium         bool     `json:"isPremium"`
		FreeEpisodesCount int      `json:"freeEpisodesCount"`
		IsFeatured        bool     `json:"isFeatured"`
		IsActive          bool     `json:"isActive"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate episode videos
	if len(request.EpisodeVideos) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "At least one episode is required"})
		return
	}

	if len(request.EpisodeVideos) > 100 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Maximum 100 episodes allowed"})
		return
	}

	// Validate free episodes count for premium dramas
	if request.IsPremium && request.FreeEpisodesCount > len(request.EpisodeVideos) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Free episodes cannot exceed total episodes"})
		return
	}

	// Validate all episode URLs
	for i, url := range request.EpisodeVideos {
		if !h.service.ValidateVideoURL(url) {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Invalid video URL",
				"index": i,
				"url":   url,
				"note":  "Episode must be max 2 minutes duration and 50MB file size",
			})
			return
		}
	}

	drama := &models.Drama{
		Title:             request.Title,
		Description:       request.Description,
		BannerImage:       request.BannerImage,
		EpisodeVideos:     request.EpisodeVideos,
		IsPremium:         request.IsPremium,
		FreeEpisodesCount: request.FreeEpisodesCount,
		IsFeatured:        request.IsFeatured,
		IsActive:          request.IsActive,
		CreatedBy:         userID, // Set creator as current user
	}

	dramaID, err := h.service.CreateDramaWithEpisodes(c.Request.Context(), drama)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create drama"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"dramaId":       dramaID,
		"message":       "Drama created successfully with episodes",
		"totalEpisodes": len(request.EpisodeVideos),
		"specifications": gin.H{
			"maxEpisodeDuration": "2 minutes",
			"maxFileSize":        "50MB",
			"unlockCost":         "99 coins",
		},
	})
}

func (h *DramaHandler) UpdateDrama(c *gin.Context) {
	dramaID := c.Param("dramaId")
	if dramaID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Drama ID required"})
		return
	}

	userID := c.GetString("userID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	// Check ownership BEFORE allowing update
	hasAccess, err := h.service.CheckDramaOwnership(c.Request.Context(), dramaID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify ownership"})
		return
	}
	if !hasAccess {
		c.JSON(http.StatusForbidden, gin.H{"error": "You can only update dramas you created"})
		return
	}

	var drama models.Drama
	if err := c.ShouldBindJSON(&drama); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	drama.DramaID = dramaID
	drama.CreatedBy = userID // Ensure creator stays the same

	// Validate episode videos if provided
	if len(drama.EpisodeVideos) > 100 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Maximum 100 episodes allowed"})
		return
	}

	// Validate all episode URLs
	for i, url := range drama.EpisodeVideos {
		if !h.service.ValidateVideoURL(url) {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Invalid video URL",
				"index": i,
				"url":   url,
			})
			return
		}
	}

	// Validate free episodes count
	if drama.IsPremium && drama.FreeEpisodesCount > len(drama.EpisodeVideos) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Free episodes cannot exceed total episodes"})
		return
	}

	err = h.service.UpdateDrama(c.Request.Context(), &drama)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update drama"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Drama updated successfully"})
}

func (h *DramaHandler) DeleteDrama(c *gin.Context) {
	dramaID := c.Param("dramaId")
	if dramaID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Drama ID required"})
		return
	}

	userID := c.GetString("userID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	// Check ownership BEFORE allowing deletion
	hasAccess, err := h.service.CheckDramaOwnership(c.Request.Context(), dramaID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify ownership"})
		return
	}
	if !hasAccess {
		c.JSON(http.StatusForbidden, gin.H{"error": "You can only delete dramas you created"})
		return
	}

	err = h.service.DeleteDrama(c.Request.Context(), dramaID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete drama"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Drama deleted successfully"})
}

func (h *DramaHandler) ToggleFeatured(c *gin.Context) {
	dramaID := c.Param("dramaId")
	if dramaID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Drama ID required"})
		return
	}

	userID := c.GetString("userID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	// Check ownership BEFORE allowing toggle
	hasAccess, err := h.service.CheckDramaOwnership(c.Request.Context(), dramaID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify ownership"})
		return
	}
	if !hasAccess {
		c.JSON(http.StatusForbidden, gin.H{"error": "You can only modify dramas you created"})
		return
	}

	var request struct {
		IsFeatured bool `json:"isFeatured"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err = h.service.ToggleFeatured(c.Request.Context(), dramaID, request.IsFeatured)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to toggle featured status"})
		return
	}

	status := "featured"
	if !request.IsFeatured {
		status = "unfeatured"
	}

	c.JSON(http.StatusOK, gin.H{"message": "Drama " + status + " successfully"})
}

func (h *DramaHandler) ToggleActive(c *gin.Context) {
	dramaID := c.Param("dramaId")
	if dramaID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Drama ID required"})
		return
	}

	userID := c.GetString("userID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	// Check ownership BEFORE allowing toggle
	hasAccess, err := h.service.CheckDramaOwnership(c.Request.Context(), dramaID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify ownership"})
		return
	}
	if !hasAccess {
		c.JSON(http.StatusForbidden, gin.H{"error": "You can only modify dramas you created"})
		return
	}

	var request struct {
		IsActive bool `json:"isActive"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err = h.service.ToggleActive(c.Request.Context(), dramaID, request.IsActive)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to toggle active status"})
		return
	}

	status := "activated"
	if !request.IsActive {
		status = "deactivated"
	}

	c.JSON(http.StatusOK, gin.H{"message": "Drama " + status + " successfully"})
}

// Only return dramas created by the current admin
func (h *DramaHandler) GetAdminDramas(c *gin.Context) {
	userID := c.GetString("userID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	dramas, err := h.service.GetDramasByAdmin(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch admin dramas"})
		return
	}

	c.JSON(http.StatusOK, dramas)
}

// ===============================
// SIMPLIFIED EPISODE MANAGEMENT ENDPOINTS
// ===============================

// AddEpisodeToDrama adds a single episode to an existing drama (REFINED)
func (h *DramaHandler) AddEpisodeToDrama(c *gin.Context) {
	dramaID := c.Param("dramaId")
	if dramaID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Drama ID required"})
		return
	}

	userID := c.GetString("userID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	// Check ownership BEFORE allowing episode addition
	hasAccess, err := h.service.CheckDramaOwnership(c.Request.Context(), dramaID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify ownership"})
		return
	}
	if !hasAccess {
		c.JSON(http.StatusForbidden, gin.H{"error": "You can only add episodes to dramas you created"})
		return
	}

	var request struct {
		EpisodeVideoURL string `json:"episodeVideoUrl" binding:"required"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate video URL with new specifications
	if request.EpisodeVideoURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Episode video URL is required"})
		return
	}

	if !h.service.ValidateVideoURL(request.EpisodeVideoURL) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid video URL or file doesn't meet requirements",
			"requirements": gin.H{
				"maxDuration": "2 minutes",
				"maxFileSize": "50MB",
				"formats":     []string{"mp4", "mov", "avi", "mkv"},
			},
		})
		return
	}

	episodeNumber, totalEpisodes, err := h.service.AddEpisodeToDrama(c.Request.Context(), dramaID, request.EpisodeVideoURL, nil)
	if err != nil {
		switch err.Error() {
		case "drama_not_found":
			c.JSON(http.StatusNotFound, gin.H{"error": "Drama not found"})
		case "max_episodes_reached":
			c.JSON(http.StatusBadRequest, gin.H{"error": "Maximum 100 episodes allowed per drama"})
		case "invalid_video_url":
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Invalid video URL or file doesn't meet specifications",
				"requirements": gin.H{
					"maxDuration": "2 minutes",
					"maxFileSize": "50MB",
				},
			})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add episode"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":       "Episode added successfully",
		"episodeNumber": episodeNumber,
		"totalEpisodes": totalEpisodes,
		"dramaId":       dramaID,
		"specifications": gin.H{
			"maxDuration": "2 minutes",
			"maxFileSize": "50MB",
		},
	})
}

// RemoveEpisodeFromDrama removes a specific episode from a drama (KEPT)
func (h *DramaHandler) RemoveEpisodeFromDrama(c *gin.Context) {
	dramaID := c.Param("dramaId")
	episodeNumberStr := c.Param("episodeNumber")

	if dramaID == "" || episodeNumberStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Drama ID and episode number required"})
		return
	}

	episodeNumber, err := strconv.Atoi(episodeNumberStr)
	if err != nil || episodeNumber < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid episode number"})
		return
	}

	userID := c.GetString("userID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	// Check ownership BEFORE allowing deletion
	hasAccess, err := h.service.CheckDramaOwnership(c.Request.Context(), dramaID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify ownership"})
		return
	}
	if !hasAccess {
		c.JSON(http.StatusForbidden, gin.H{"error": "You can only remove episodes from dramas you created"})
		return
	}

	totalEpisodes, err := h.service.RemoveEpisodeFromDrama(c.Request.Context(), dramaID, episodeNumber)
	if err != nil {
		switch err.Error() {
		case "drama_not_found":
			c.JSON(http.StatusNotFound, gin.H{"error": "Drama not found"})
		case "episode_not_found":
			c.JSON(http.StatusNotFound, gin.H{"error": "Episode not found"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove episode"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":       "Episode removed successfully",
		"totalEpisodes": totalEpisodes,
		"dramaId":       dramaID,
	})
}

// ReplaceEpisodeInDrama replaces an existing episode with a new video URL (KEPT)
func (h *DramaHandler) ReplaceEpisodeInDrama(c *gin.Context) {
	dramaID := c.Param("dramaId")
	episodeNumberStr := c.Param("episodeNumber")

	if dramaID == "" || episodeNumberStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Drama ID and episode number required"})
		return
	}

	episodeNumber, err := strconv.Atoi(episodeNumberStr)
	if err != nil || episodeNumber < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid episode number"})
		return
	}

	userID := c.GetString("userID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	// Check ownership BEFORE allowing replacement
	hasAccess, err := h.service.CheckDramaOwnership(c.Request.Context(), dramaID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify ownership"})
		return
	}
	if !hasAccess {
		c.JSON(http.StatusForbidden, gin.H{"error": "You can only modify episodes in dramas you created"})
		return
	}

	var request struct {
		EpisodeVideoURL string `json:"episodeVideoUrl" binding:"required"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate new video URL
	if !h.service.ValidateVideoURL(request.EpisodeVideoURL) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid video URL or file doesn't meet requirements",
			"requirements": gin.H{
				"maxDuration": "2 minutes",
				"maxFileSize": "50MB",
			},
		})
		return
	}

	err = h.service.ReplaceEpisodeInDrama(c.Request.Context(), dramaID, episodeNumber, request.EpisodeVideoURL)
	if err != nil {
		switch err.Error() {
		case "drama_not_found":
			c.JSON(http.StatusNotFound, gin.H{"error": "Drama not found"})
		case "episode_not_found":
			c.JSON(http.StatusNotFound, gin.H{"error": "Episode not found"})
		case "invalid_video_url":
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Invalid video URL or file doesn't meet specifications",
			})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to replace episode"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":       "Episode replaced successfully",
		"dramaId":       dramaID,
		"episodeNumber": episodeNumber,
	})
}

// GetDramaEpisodes returns all episodes for a specific drama (KEPT)
func (h *DramaHandler) GetDramaEpisodes(c *gin.Context) {
	dramaID := c.Param("dramaId")
	if dramaID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Drama ID required"})
		return
	}

	episodes, err := h.service.GetDramaEpisodes(c.Request.Context(), dramaID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Drama not found"})
		return
	}

	c.JSON(http.StatusOK, episodes)
}

// GetEpisodeDetails returns details for a specific episode (KEPT)
func (h *DramaHandler) GetEpisodeDetails(c *gin.Context) {
	dramaID := c.Param("dramaId")
	episodeNumberStr := c.Param("episodeNumber")

	if dramaID == "" || episodeNumberStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Drama ID and episode number required"})
		return
	}

	episodeNumber, err := strconv.Atoi(episodeNumberStr)
	if err != nil || episodeNumber < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid episode number"})
		return
	}

	episode, err := h.service.GetDramaEpisodeDetails(c.Request.Context(), dramaID, episodeNumber)
	if err != nil {
		switch err.Error() {
		case "drama_not_found":
			c.JSON(http.StatusNotFound, gin.H{"error": "Drama not found"})
		case "episode_not_found":
			c.JSON(http.StatusNotFound, gin.H{"error": "Episode not found"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get episode details"})
		}
		return
	}

	c.JSON(http.StatusOK, episode)
}

// ===============================
// EPISODE STATISTICS WITH UNLOCK TRACKING - REFINED
// ===============================

// GetEpisodeStats returns enhanced statistics including unlock tracking
func (h *DramaHandler) GetEpisodeStats(c *gin.Context) {
	dramaID := c.Param("dramaId")
	if dramaID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Drama ID required"})
		return
	}

	userID := c.GetString("userID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	// Check ownership - only drama owners can view detailed stats
	hasAccess, err := h.service.CheckDramaOwnership(c.Request.Context(), dramaID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify ownership"})
		return
	}
	if !hasAccess {
		c.JSON(http.StatusForbidden, gin.H{"error": "You can only view stats for dramas you created"})
		return
	}

	stats, err := h.service.GetEpisodeStatsWithUnlocks(c.Request.Context(), dramaID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get episode statistics"})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// ===============================
// REFINED UPLOAD LIMITS AND UTILITIES
// ===============================

// GetDramaUploadLimits returns current upload limits with updated specifications
func (h *DramaHandler) GetDramaUploadLimits(c *gin.Context) {
	userID := c.GetString("userID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	// Get user's current drama and episode count
	userDramaCount, err := h.service.GetUserDramaCount(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user drama count"})
		return
	}

	totalEpisodes, err := h.service.GetUserTotalEpisodeCount(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user episode count"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"maxDramasPerUser":       100,
		"maxEpisodesPerDrama":    100,
		"maxEpisodesPerUser":     3000,
		"maxEpisodeDurationSecs": 120, // 2 minutes - UPDATED
		"maxFileSizeMB":          50,  // 50MB - UPDATED
		"dramaUnlockCostCoins":   99,  // 99 coins - UPDATED
		"allowedFormats":         []string{"mp4", "mov", "avi", "mkv"},
		"currentDramaCount":      userDramaCount,
		"currentEpisodeCount":    totalEpisodes,
		"remainingDramas":        100 - userDramaCount,
		"remainingEpisodes":      1000 - totalEpisodes,
		"episodeManagement":      "single-episode-at-a-time", // SIMPLIFIED
	})
}
