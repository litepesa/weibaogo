// ===============================
// internal/handlers/drama.go - Drama Handlers for Video Social Media App
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
	params := models.DramaSearchParams{
		Limit:  20,
		Offset: 0,
		SortBy: models.DramaSortByLatest,
	}

	// Parse query parameters
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			params.Limit = parsed
		}
	}

	if o := c.Query("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			params.Offset = parsed
		}
	}

	if q := c.Query("q"); q != "" {
		params.Query = q
	}

	if u := c.Query("userId"); u != "" {
		params.UserID = u
	}

	if s := c.Query("sortBy"); s != "" {
		params.SortBy = s
	}

	if p := c.Query("premium"); p != "" {
		if p == "true" {
			val := true
			params.Premium = &val
		} else if p == "false" {
			val := false
			params.Premium = &val
		}
	}

	if f := c.Query("featured"); f != "" {
		if f == "true" {
			val := true
			params.Featured = &val
		} else if f == "false" {
			val := false
			params.Featured = &val
		}
	}

	dramas, err := h.service.GetDramas(c.Request.Context(), params)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch dramas"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"dramas": dramas,
		"total":  len(dramas),
	})
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

	c.JSON(http.StatusOK, gin.H{
		"dramas": dramas,
		"total":  len(dramas),
	})
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

	c.JSON(http.StatusOK, gin.H{
		"dramas": dramas,
		"total":  len(dramas),
	})
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

	c.JSON(http.StatusOK, gin.H{
		"dramas": dramas,
		"total":  len(dramas),
		"query":  query,
	})
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

// Get drama episodes (compatibility with old episode endpoints)
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

	c.JSON(http.StatusOK, gin.H{
		"episodes": episodes,
		"total":    len(episodes),
	})
}

// Get individual episode (compatibility endpoint)
func (h *DramaHandler) GetEpisode(c *gin.Context) {
	dramaID := c.Param("dramaId")
	episodeNumberStr := c.Param("episodeNumber")

	if dramaID == "" || episodeNumberStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Drama ID and episode number required"})
		return
	}

	episodeNumber, err := strconv.Atoi(episodeNumberStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid episode number"})
		return
	}

	// Check if user is authenticated for unlock context
	userID := c.GetString("userID")
	if userID != "" {
		// Return episode with user context (unlock status)
		episode, err := h.service.GetEpisodeWithUserContext(c.Request.Context(), dramaID, episodeNumber, userID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Episode not found"})
			return
		}
		c.JSON(http.StatusOK, episode)
		return
	}

	// Return episode without user context
	episode, err := h.service.GetEpisodeByNumber(c.Request.Context(), dramaID, episodeNumber)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Episode not found"})
		return
	}

	c.JSON(http.StatusOK, episode)
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

	userID := c.GetString("userID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	var request struct {
		IsAdding bool `json:"isAdding"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := h.service.ToggleDramaFavorite(c.Request.Context(), userID, dramaID, request.IsAdding)
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

func (h *DramaHandler) UpdateProgress(c *gin.Context) {
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

	var request struct {
		EpisodeNumber int `json:"episodeNumber" binding:"required,min=1"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := h.service.UpdateDramaProgress(c.Request.Context(), userID, dramaID, request.EpisodeNumber)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update progress"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Progress updated successfully"})
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

	var request models.UnlockDramaRequest
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
		case "user_not_found":
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		case "wallet_not_found":
			c.JSON(http.StatusNotFound, gin.H{"error": "Wallet not found"})
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
		"success":    true,
		"message":    "Drama unlocked successfully",
		"newBalance": newBalance,
	})
}

// ===============================
// USER DRAMA ENDPOINTS (AUTHENTICATED)
// ===============================

func (h *DramaHandler) GetUserFavorites(c *gin.Context) {
	userID := c.GetString("userID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

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

	dramas, err := h.service.GetUserFavoriteDramas(c.Request.Context(), userID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch favorite dramas"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"dramas": dramas,
		"total":  len(dramas),
	})
}

func (h *DramaHandler) GetContinueWatching(c *gin.Context) {
	userID := c.GetString("userID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	limit := 10
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 50 {
			limit = parsed
		}
	}

	dramas, err := h.service.GetUserContinueWatchingDramas(c.Request.Context(), userID, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch continue watching dramas"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"dramas": dramas,
		"total":  len(dramas),
	})
}

func (h *DramaHandler) GetUserProgress(c *gin.Context) {
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

	progress, err := h.service.GetUserDramaProgress(c.Request.Context(), userID, dramaID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "No progress found for this drama"})
		return
	}

	c.JSON(http.StatusOK, progress)
}

// ===============================
// VERIFIED USER DRAMA MANAGEMENT (VERIFIED USERS ONLY)
// ===============================

func (h *DramaHandler) CreateDramaWithEpisodes(c *gin.Context) {
	userID := c.GetString("userID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	var request models.CreateDramaRequest
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
		EpisodeVideos:     models.StringSlice(request.EpisodeVideos),
		IsPremium:         request.IsPremium,
		FreeEpisodesCount: request.FreeEpisodesCount,
		IsFeatured:        request.IsFeatured,
		IsActive:          request.IsActive,
		CreatedBy:         userID, // Set creator as current user
	}

	dramaID, err := h.service.CreateDramaWithEpisodes(c.Request.Context(), drama)
	if err != nil {
		if err.Error() == "user_not_verified_to_create_dramas" {
			c.JSON(http.StatusForbidden, gin.H{"error": "Only verified users can create dramas"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create drama"})
		}
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

	var request models.UpdateDramaRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	drama := &models.Drama{
		DramaID:           dramaID,
		Title:             request.Title,
		Description:       request.Description,
		BannerImage:       request.BannerImage,
		EpisodeVideos:     models.StringSlice(request.EpisodeVideos),
		IsPremium:         request.IsPremium,
		FreeEpisodesCount: request.FreeEpisodesCount,
		IsFeatured:        request.IsFeatured,
		IsActive:          request.IsActive,
		CreatedBy:         userID, // Ensure creator stays the same
	}

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

	err := h.service.UpdateDrama(c.Request.Context(), drama)
	if err != nil {
		if err.Error() == "drama_not_found_or_no_access" {
			c.JSON(http.StatusForbidden, gin.H{"error": "You can only update dramas you created"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update drama"})
		}
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

	err := h.service.DeleteDrama(c.Request.Context(), dramaID, userID)
	if err != nil {
		if err.Error() == "drama_not_found_or_no_access" {
			c.JSON(http.StatusForbidden, gin.H{"error": "You can only delete dramas you created"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete drama"})
		}
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

	var request struct {
		IsFeatured bool `json:"isFeatured"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := h.service.ToggleFeatured(c.Request.Context(), dramaID, userID, request.IsFeatured)
	if err != nil {
		if err.Error() == "drama_not_found_or_no_access" {
			c.JSON(http.StatusForbidden, gin.H{"error": "You can only modify dramas you created"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to toggle featured status"})
		}
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

	var request struct {
		IsActive bool `json:"isActive"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := h.service.ToggleActive(c.Request.Context(), dramaID, userID, request.IsActive)
	if err != nil {
		if err.Error() == "drama_not_found_or_no_access" {
			c.JSON(http.StatusForbidden, gin.H{"error": "You can only modify dramas you created"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to toggle active status"})
		}
		return
	}

	status := "activated"
	if !request.IsActive {
		status = "deactivated"
	}

	c.JSON(http.StatusOK, gin.H{"message": "Drama " + status + " successfully"})
}

// Get dramas created by current verified user
func (h *DramaHandler) GetMyDramas(c *gin.Context) {
	userID := c.GetString("userID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	dramas, err := h.service.GetDramasByVerifiedUser(c.Request.Context(), userID)
	if err != nil {
		if err.Error() == "user_not_verified" {
			c.JSON(http.StatusForbidden, gin.H{"error": "Only verified users can view their dramas"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch your dramas"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"dramas": dramas,
		"total":  len(dramas),
	})
}

// ===============================
// ANALYTICS ENDPOINTS (VERIFIED USERS)
// ===============================

func (h *DramaHandler) GetMyDramaAnalytics(c *gin.Context) {
	userID := c.GetString("userID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	performances, err := h.service.GetVerifiedUserDramasWithRevenue(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch drama analytics"})
		return
	}

	// Get total revenue
	totalRevenue, err := h.service.GetTotalVerifiedUserRevenue(c.Request.Context(), userID)
	if err != nil {
		totalRevenue = 0 // Continue with 0 if error
	}

	c.JSON(http.StatusOK, gin.H{
		"performances": performances,
		"totalRevenue": totalRevenue,
		"totalDramas":  len(performances),
		"unlockCost":   models.DramaUnlockCost,
	})
}

func (h *DramaHandler) GetDramaRevenue(c *gin.Context) {
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

	// Check ownership
	hasAccess, err := h.service.CheckDramaOwnership(c.Request.Context(), dramaID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify ownership"})
		return
	}
	if !hasAccess {
		c.JSON(http.StatusForbidden, gin.H{"error": "You can only view revenue for dramas you created"})
		return
	}

	revenue, err := h.service.GetDramaRevenue(c.Request.Context(), dramaID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch drama revenue"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"dramaId":    dramaID,
		"revenue":    revenue,
		"unlockCost": models.DramaUnlockCost,
	})
}
