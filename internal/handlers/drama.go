// ===============================
// internal/handlers/drama.go - UNIFIED ARCHITECTURE
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
// PUBLIC DRAMA ENDPOINTS
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
// DRAMA UNLOCK ENDPOINT
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
			c.JSON(http.StatusBadRequest, gin.H{"error": "Insufficient coins"})
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
	})
}

// ===============================
// DRAMA INTERACTION ENDPOINTS
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
// ADMIN DRAMA MANAGEMENT
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

	drama := &models.Drama{
		Title:             request.Title,
		Description:       request.Description,
		BannerImage:       request.BannerImage,
		EpisodeVideos:     request.EpisodeVideos,
		IsPremium:         request.IsPremium,
		FreeEpisodesCount: request.FreeEpisodesCount,
		IsFeatured:        request.IsFeatured,
		IsActive:          request.IsActive,
		CreatedBy:         userID,
	}

	dramaID, err := h.service.CreateDramaWithEpisodes(c.Request.Context(), drama)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create drama"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"dramaId": dramaID,
		"message": "Drama created successfully with episodes",
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

	var drama models.Drama
	if err := c.ShouldBindJSON(&drama); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	drama.DramaID = dramaID

	// Validate episode videos if provided
	if len(drama.EpisodeVideos) > 100 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Maximum 100 episodes allowed"})
		return
	}

	// Validate free episodes count
	if drama.IsPremium && drama.FreeEpisodesCount > len(drama.EpisodeVideos) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Free episodes cannot exceed total episodes"})
		return
	}

	err := h.service.UpdateDrama(c.Request.Context(), &drama)
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

	err := h.service.DeleteDrama(c.Request.Context(), dramaID)
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

	var request struct {
		IsFeatured bool `json:"isFeatured"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := h.service.ToggleFeatured(c.Request.Context(), dramaID, request.IsFeatured)
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

	var request struct {
		IsActive bool `json:"isActive"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := h.service.ToggleActive(c.Request.Context(), dramaID, request.IsActive)
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
