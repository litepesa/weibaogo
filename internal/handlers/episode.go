// ===============================
// internal/handlers/episode.go - Simplified Fixed Implementation
// ===============================

package handlers

import (
	"net/http"
	"strconv"

	"weibaobe/internal/models"
	"weibaobe/internal/services"

	"github.com/gin-gonic/gin"
)

type EpisodeHandler struct {
	service *services.DramaService
}

func NewEpisodeHandler(service *services.DramaService) *EpisodeHandler {
	return &EpisodeHandler{service: service}
}

// ===============================
// PUBLIC EPISODE ENDPOINTS
// ===============================

func (h *EpisodeHandler) GetDramaEpisodes(c *gin.Context) {
	dramaID := c.Param("dramaId")
	if dramaID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Drama ID required"})
		return
	}

	episodes, err := h.service.GetDramaEpisodes(c.Request.Context(), dramaID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch episodes"})
		return
	}

	c.JSON(http.StatusOK, episodes)
}

func (h *EpisodeHandler) GetEpisode(c *gin.Context) {
	episodeID := c.Param("episodeId")
	if episodeID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Episode ID required"})
		return
	}

	episode, err := h.service.GetEpisode(c.Request.Context(), episodeID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Episode not found"})
		return
	}

	c.JSON(http.StatusOK, episode)
}

// ===============================
// ADMIN EPISODE MANAGEMENT ENDPOINTS
// ===============================

func (h *EpisodeHandler) CreateEpisode(c *gin.Context) {
	dramaID := c.Param("dramaId")
	userID := c.GetString("userID")

	if dramaID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Drama ID required"})
		return
	}

	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	var episode models.Episode
	if err := c.ShouldBindJSON(&episode); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate required fields
	if episode.EpisodeNumber <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Episode number must be greater than 0"})
		return
	}

	if episode.EpisodeTitle == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Episode title is required"})
		return
	}

	// Set episode metadata
	episode.DramaID = dramaID
	episode.UploadedBy = userID

	episodeID, err := h.service.CreateEpisode(c.Request.Context(), &episode)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create episode"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"episodeId": episodeID,
		"message":   "Episode created successfully",
	})
}

func (h *EpisodeHandler) UpdateEpisode(c *gin.Context) {
	episodeID := c.Param("episodeId")
	userID := c.GetString("userID")

	if episodeID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Episode ID required"})
		return
	}

	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	var episode models.Episode
	if err := c.ShouldBindJSON(&episode); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate required fields
	if episode.EpisodeNumber <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Episode number must be greater than 0"})
		return
	}

	if episode.EpisodeTitle == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Episode title is required"})
		return
	}

	episode.EpisodeID = episodeID

	err := h.service.UpdateEpisode(c.Request.Context(), &episode)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update episode"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Episode updated successfully"})
}

func (h *EpisodeHandler) DeleteEpisode(c *gin.Context) {
	episodeID := c.Param("episodeId")
	userID := c.GetString("userID")

	if episodeID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Episode ID required"})
		return
	}

	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	err := h.service.DeleteEpisode(c.Request.Context(), episodeID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete episode"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Episode deleted successfully"})
}

// ===============================
// BULK EPISODE OPERATIONS
// ===============================

func (h *EpisodeHandler) BulkCreateEpisodes(c *gin.Context) {
	dramaID := c.Param("dramaId")
	userID := c.GetString("userID")

	if dramaID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Drama ID required"})
		return
	}

	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	var request struct {
		Episodes []models.Episode `json:"episodes" binding:"required,min=1,max=50"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate each episode
	for i, episode := range request.Episodes {
		if episode.EpisodeNumber <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Episode number must be greater than 0",
				"index": i,
			})
			return
		}
		if episode.EpisodeTitle == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Episode title is required",
				"index": i,
			})
			return
		}
	}

	// Set metadata for all episodes
	for i := range request.Episodes {
		request.Episodes[i].DramaID = dramaID
		request.Episodes[i].UploadedBy = userID
	}

	episodeIDs, err := h.service.BulkCreateEpisodes(c.Request.Context(), request.Episodes)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create episodes"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"episodeIds": episodeIDs,
		"message":    "Episodes created successfully",
		"count":      len(episodeIDs),
	})
}

// ===============================
// SEARCH AND FILTERING
// ===============================

func (h *EpisodeHandler) SearchEpisodes(c *gin.Context) {
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

	dramaID := c.Query("dramaId") // Optional filter by drama

	episodes, err := h.service.SearchEpisodes(c.Request.Context(), query, dramaID, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to search episodes"})
		return
	}

	c.JSON(http.StatusOK, episodes)
}
