// ===============================
// internal/handlers/upload.go
// ===============================

package handlers

import (
	"net/http"
	"path/filepath"
	"strings"

	"weibaobe/internal/services"

	"github.com/gin-gonic/gin"
)

type UploadHandler struct {
	service *services.UploadService
}

func NewUploadHandler(service *services.UploadService) *UploadHandler {
	return &UploadHandler{service: service}
}

func (h *UploadHandler) UploadFile(c *gin.Context) {
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No file uploaded"})
		return
	}
	defer file.Close()

	fileType := c.PostForm("type") // "banner", "thumbnail", "video", "profile"
	if fileType == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File type required"})
		return
	}

	// Validate file type - Added TS support for videos
	allowedTypes := map[string][]string{
		"banner":    {".jpg", ".jpeg", ".png", ".webp"},
		"thumbnail": {".jpg", ".jpeg", ".png", ".webp"},
		"profile":   {".jpg", ".jpeg", ".png", ".webp"},
		"video":     {".mp4", ".mov", ".avi", ".webm", ".ts"}, // Added .ts support
	}

	ext := strings.ToLower(filepath.Ext(header.Filename))
	if allowed, exists := allowedTypes[fileType]; exists {
		validExt := false
		for _, allowedExt := range allowed {
			if ext == allowedExt {
				validExt = true
				break
			}
		}
		if !validExt {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid file type for " + fileType})
			return
		}
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid file type category"})
		return
	}

	// Check file size (adjust limits as needed)
	maxSizes := map[string]int64{
		"banner":    5 * 1024 * 1024,   // 5MB
		"thumbnail": 2 * 1024 * 1024,   // 2MB
		"profile":   2 * 1024 * 1024,   // 2MB
		"video":     500 * 1024 * 1024, // 500MB
	}

	if maxSize, exists := maxSizes[fileType]; exists {
		if header.Size > maxSize {
			c.JSON(http.StatusBadRequest, gin.H{"error": "File too large"})
			return
		}
	}

	url, err := h.service.UploadFile(c.Request.Context(), file, header.Filename, fileType)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to upload file"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"url":     url,
		"message": "File uploaded successfully",
	})
}
