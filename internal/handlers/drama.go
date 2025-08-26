// ===============================
// internal/handlers/drama.go
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

func (h *DramaHandler) GetDramas(c *gin.Context) {
	limit := 20
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}

	dramas, err := h.service.GetDramas(c.Request.Context(), limit)
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

// Admin handlers
func (h *DramaHandler) CreateDrama(c *gin.Context) {
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

	drama.CreatedBy = userID

	dramaID, err := h.service.CreateDrama(c.Request.Context(), &drama)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create drama"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"dramaId": dramaID,
		"message": "Drama created successfully",
	})
}

func (h *DramaHandler) UpdateDrama(c *gin.Context) {
	dramaID := c.Param("dramaId")
	if dramaID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Drama ID required"})
		return
	}

	var drama models.Drama
	if err := c.ShouldBindJSON(&drama); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	drama.DramaID = dramaID

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
