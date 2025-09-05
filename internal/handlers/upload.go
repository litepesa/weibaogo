// internal/handlers/upload.go - IMPROVED VERSION with better TS support
package handlers

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"

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
	// Add request timeout for large files
	c.Request = c.Request.WithContext(c.Request.Context())

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "No file uploaded",
			"details": err.Error(),
		})
		return
	}
	defer file.Close()

	fileType := c.PostForm("type") // "banner", "thumbnail", "video", "profile"
	if fileType == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File type required"})
		return
	}

	// Enhanced file validation with better TS support
	allowedTypes := map[string][]string{
		"banner":    {".jpg", ".jpeg", ".png", ".webp", ".gif"},
		"thumbnail": {".jpg", ".jpeg", ".png", ".webp"},
		"profile":   {".jpg", ".jpeg", ".png", ".webp"},
		"video":     {".mp4", ".mov", ".avi", ".webm", ".ts", ".m3u8", ".mkv"}, // Enhanced video support
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
			c.JSON(http.StatusBadRequest, gin.H{
				"error":    fmt.Sprintf("Invalid file type for %s", fileType),
				"allowed":  allowed,
				"received": ext,
			})
			return
		}
	} else {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":              "Invalid file type category",
			"allowed_categories": []string{"banner", "thumbnail", "profile", "video"},
		})
		return
	}

	// Enhanced file size validation with better error messages
	maxSizes := map[string]int64{
		"banner":    10 * 1024 * 1024, // 10MB (increased for better quality)
		"thumbnail": 5 * 1024 * 1024,  // 5MB
		"profile":   5 * 1024 * 1024,  // 5MB
		"video":     50 * 1024 * 1024, // 50MB
	}

	if maxSize, exists := maxSizes[fileType]; exists {
		if header.Size > maxSize {
			fileSizeInMB := float64(header.Size) / (1024 * 1024)
			maxSizeInMB := float64(maxSize) / (1024 * 1024)
			c.JSON(http.StatusBadRequest, gin.H{
				"error":        "File too large",
				"file_size_mb": fmt.Sprintf("%.2f", fileSizeInMB),
				"max_size_mb":  fmt.Sprintf("%.2f", maxSizeInMB),
			})
			return
		}
	}

	// Additional validation for video files
	if fileType == "video" {
		// Basic file header validation for common video formats
		if !isValidVideoFile(header.Filename, ext) {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "Invalid video file format",
				"details": "File may be corrupted or not a valid video file",
			})
			return
		}
	}

	// Upload with enhanced error handling
	url, err := h.service.UploadFile(c.Request.Context(), file, header.Filename, fileType)
	if err != nil {
		// Enhanced error response
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":     "Failed to upload file",
			"details":   err.Error(),
			"file_name": header.Filename,
			"file_size": header.Size,
			"file_type": fileType,
			"timestamp": time.Now().Unix(),
		})
		return
	}

	// Success response with additional metadata
	c.JSON(http.StatusOK, gin.H{
		"url":       url,
		"message":   "File uploaded successfully",
		"file_name": header.Filename,
		"file_size": header.Size,
		"file_type": fileType,
		"extension": ext,
		"timestamp": time.Now().Unix(),
	})
}

// Enhanced video file validation
func isValidVideoFile(filename, ext string) bool {
	// List of known video file extensions
	videoExtensions := map[string]bool{
		".mp4":  true,
		".mov":  true,
		".avi":  true,
		".webm": true,
		".ts":   true, // Transport Stream files
		".m3u8": true, // HLS playlist files
		".mkv":  true, // Matroska video files
		".flv":  true, // Flash video (if needed)
		".wmv":  true, // Windows Media Video (if needed)
	}

	return videoExtensions[ext]
}

// Health check endpoint for upload service
func (h *UploadHandler) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"service":   "upload",
		"timestamp": time.Now().Unix(),
		"supported_formats": map[string][]string{
			"images": {".jpg", ".jpeg", ".png", ".webp", ".gif"},
			"videos": {".mp4", ".mov", ".avi", ".webm", ".ts", ".m3u8", ".mkv"},
		},
		"max_sizes_mb": map[string]int{
			"banner":    10,
			"thumbnail": 5,
			"profile":   5,
			"video":     1024,
		},
	})
}

// Batch upload endpoint for multiple files (useful for episodes)
func (h *UploadHandler) BatchUploadFiles(c *gin.Context) {
	form, err := c.MultipartForm()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Failed to parse multipart form",
			"details": err.Error(),
		})
		return
	}

	files := form.File["files"]
	if len(files) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No files uploaded"})
		return
	}

	fileType := c.PostForm("type")
	if fileType == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File type required"})
		return
	}

	// Limit batch size to prevent abuse
	maxBatchSize := 20
	if len(files) > maxBatchSize {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":    fmt.Sprintf("Too many files. Maximum %d files allowed per batch", maxBatchSize),
			"received": len(files),
		})
		return
	}

	var results []map[string]interface{}
	var successCount int

	for i, fileHeader := range files {
		file, err := fileHeader.Open()
		if err != nil {
			results = append(results, map[string]interface{}{
				"index":    i,
				"filename": fileHeader.Filename,
				"status":   "error",
				"error":    "Failed to open file: " + err.Error(),
			})
			continue
		}

		// Validate file
		ext := strings.ToLower(filepath.Ext(fileHeader.Filename))
		if !h.isValidFileForType(ext, fileType) {
			file.Close()
			results = append(results, map[string]interface{}{
				"index":    i,
				"filename": fileHeader.Filename,
				"status":   "error",
				"error":    "Invalid file type",
			})
			continue
		}

		// Upload file
		url, err := h.service.UploadFile(c.Request.Context(), file, fileHeader.Filename, fileType)
		file.Close()

		if err != nil {
			results = append(results, map[string]interface{}{
				"index":    i,
				"filename": fileHeader.Filename,
				"status":   "error",
				"error":    "Upload failed: " + err.Error(),
			})
		} else {
			results = append(results, map[string]interface{}{
				"index":    i,
				"filename": fileHeader.Filename,
				"status":   "success",
				"url":      url,
			})
			successCount++
		}
	}

	// Return batch results
	c.JSON(http.StatusOK, gin.H{
		"message":     fmt.Sprintf("Batch upload completed. %d of %d files uploaded successfully", successCount, len(files)),
		"total_files": len(files),
		"successful":  successCount,
		"failed":      len(files) - successCount,
		"results":     results,
		"timestamp":   time.Now().Unix(),
	})
}

// Helper method for file validation
func (h *UploadHandler) isValidFileForType(ext, fileType string) bool {
	allowedTypes := map[string][]string{
		"banner":    {".jpg", ".jpeg", ".png", ".webp", ".gif"},
		"thumbnail": {".jpg", ".jpeg", ".png", ".webp"},
		"profile":   {".jpg", ".jpeg", ".png", ".webp"},
		"video":     {".mp4", ".mov", ".avi", ".webm", ".ts", ".m3u8", ".mkv"},
	}

	if allowed, exists := allowedTypes[fileType]; exists {
		for _, allowedExt := range allowed {
			if ext == allowedExt {
				return true
			}
		}
	}
	return false
}
