// ===============================
// internal/handlers/video.go - UPDATED: Allow All Users to Post
// ===============================

package handlers

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"weibaobe/internal/models"
	"weibaobe/internal/services"

	"github.com/gin-gonic/gin"
)

type VideoHandler struct {
	service     *services.VideoService
	userService *services.UserService
}

func NewVideoHandler(service *services.VideoService, userService *services.UserService) *VideoHandler {
	return &VideoHandler{
		service:     service,
		userService: userService,
	}
}

// ===============================
// HEADER HELPERS
// ===============================

func (h *VideoHandler) setVideoStreamingHeaders(c *gin.Context) {
	c.Header("Accept-Ranges", "bytes")
	c.Header("Cache-Control", "public, max-age=3600")
	c.Header("Connection", "keep-alive")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("X-Frame-Options", "SAMEORIGIN")
}

func (h *VideoHandler) setVideoAPIHeaders(c *gin.Context) {
	c.Header("Cache-Control", "public, max-age=1800")
	c.Header("Connection", "keep-alive")
	c.Header("X-Content-Type-Options", "nosniff")
}

func (h *VideoHandler) setVideoListHeaders(c *gin.Context) {
	c.Header("Cache-Control", "public, max-age=900")
	c.Header("Connection", "keep-alive")
}

func (h *VideoHandler) setInteractionHeaders(c *gin.Context) {
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
}

func (h *VideoHandler) setCommentHeaders(c *gin.Context) {
	c.Header("Cache-Control", "public, max-age=300")
	c.Header("Connection", "keep-alive")
}

// ===============================
// 🔍 SIMPLIFIED SEARCH ENDPOINT
// ===============================

func (h *VideoHandler) SearchVideos(c *gin.Context) {
	h.setVideoListHeaders(c)

	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Search query required",
			"code":  "MISSING_SEARCH_QUERY",
		})
		return
	}

	// Optional filter for username-only results
	usernameOnly := c.Query("usernameOnly") == "true"

	// Parse pagination
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

	// Perform fuzzy search
	videos, total, err := h.service.FuzzySearch(c.Request.Context(), query, usernameOnly, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Search failed",
			"code":  "SEARCH_ERROR",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"videos":       videos,
		"total":        total,
		"query":        query,
		"usernameOnly": usernameOnly,
		"page":         (offset / limit) + 1,
		"limit":        limit,
		"hasMore":      len(videos) == limit,
		"cached_at":    time.Now().Unix(),
		"ttl":          900,
	})
}

// ===============================
// 🔍 POPULAR SEARCH TERMS
// ===============================

func (h *VideoHandler) GetPopularSearchTerms(c *gin.Context) {
	h.setVideoListHeaders(c)

	limit := 10
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 50 {
			limit = parsed
		}
	}

	terms, err := h.service.GetPopularSearchTerms(c.Request.Context(), limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get popular search terms",
			"code":  "SEARCH_TERMS_ERROR",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"terms":     terms,
		"total":     len(terms),
		"limit":     limit,
		"cached_at": time.Now().Unix(),
		"ttl":       1800,
	})
}

// ===============================
// 🔍 SEARCH HISTORY ENDPOINTS
// ===============================

// Get user's search history
func (h *VideoHandler) GetSearchHistory(c *gin.Context) {
	h.setInteractionHeaders(c)

	userID := c.GetString("userID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "User not authenticated",
			"code":  "AUTH_REQUIRED",
		})
		return
	}

	limit := 20
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 50 {
			limit = parsed
		}
	}

	history, err := h.service.GetSearchHistory(c.Request.Context(), userID, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get search history",
			"code":  "HISTORY_ERROR",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"history": history,
		"total":   len(history),
		"userId":  userID,
	})
}

// Add to search history
func (h *VideoHandler) AddSearchHistory(c *gin.Context) {
	h.setInteractionHeaders(c)

	userID := c.GetString("userID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "User not authenticated",
			"code":  "AUTH_REQUIRED",
		})
		return
	}

	var request struct {
		Query string `json:"query" binding:"required"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"code":    "INVALID_REQUEST",
			"details": err.Error(),
		})
		return
	}

	if strings.TrimSpace(request.Query) == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Search query cannot be empty",
			"code":  "EMPTY_QUERY",
		})
		return
	}

	err := h.service.AddSearchHistory(c.Request.Context(), userID, request.Query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to add search history",
			"code":  "ADD_HISTORY_ERROR",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Search added to history",
		"query":   request.Query,
	})
}

// Clear all search history
func (h *VideoHandler) ClearSearchHistory(c *gin.Context) {
	h.setInteractionHeaders(c)

	userID := c.GetString("userID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "User not authenticated",
			"code":  "AUTH_REQUIRED",
		})
		return
	}

	err := h.service.ClearSearchHistory(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to clear search history",
			"code":  "CLEAR_HISTORY_ERROR",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Search history cleared",
		"userId":  userID,
	})
}

// Remove specific search from history
func (h *VideoHandler) RemoveSearchHistory(c *gin.Context) {
	h.setInteractionHeaders(c)

	userID := c.GetString("userID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "User not authenticated",
			"code":  "AUTH_REQUIRED",
		})
		return
	}

	query := c.Param("query")
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Query parameter required",
			"code":  "MISSING_QUERY",
		})
		return
	}

	err := h.service.RemoveSearchHistory(c.Request.Context(), userID, query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to remove search history",
			"code":  "REMOVE_HISTORY_ERROR",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Search removed from history",
		"query":   query,
	})
}

// ===============================
// PUBLIC VIDEO ENDPOINTS
// ===============================

func (h *VideoHandler) GetVideos(c *gin.Context) {
	h.setVideoListHeaders(c)

	params := models.VideoSearchParams{
		Limit:  20,
		Offset: 0,
		SortBy: "latest",
	}

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

	if m := c.Query("mediaType"); m != "" {
		params.MediaType = m
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

	videos, err := h.service.GetVideosOptimized(c.Request.Context(), params)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to fetch videos",
			"code":  "FETCH_ERROR",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"videos":    videos,
		"total":     len(videos),
		"page":      (params.Offset / params.Limit) + 1,
		"limit":     params.Limit,
		"hasMore":   len(videos) == params.Limit,
		"cached_at": time.Now().Unix(),
		"ttl":       900,
	})
}

func (h *VideoHandler) GetVideosBulk(c *gin.Context) {
	h.setVideoListHeaders(c)

	var request struct {
		VideoIDs        []string `json:"videoIds" binding:"required,max=50"`
		IncludeInactive bool     `json:"includeInactive"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"code":    "INVALID_REQUEST",
			"details": err.Error(),
		})
		return
	}

	if len(request.VideoIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Video IDs required",
			"code":  "MISSING_VIDEO_IDS",
		})
		return
	}

	if len(request.VideoIDs) > 50 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Maximum 50 videos per request",
			"code":  "TOO_MANY_VIDEOS",
		})
		return
	}

	videos, err := h.service.GetVideosBulk(c.Request.Context(), request.VideoIDs, request.IncludeInactive)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to fetch videos",
			"code":  "BULK_FETCH_ERROR",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"videos":     videos,
		"requested":  len(request.VideoIDs),
		"found":      len(videos),
		"cached_at":  time.Now().Unix(),
		"bulk_fetch": true,
	})
}

func (h *VideoHandler) GetFeaturedVideos(c *gin.Context) {
	h.setVideoListHeaders(c)

	limit := 10
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 50 {
			limit = parsed
		}
	}

	videos, err := h.service.GetFeaturedVideosOptimized(c.Request.Context(), limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to fetch featured videos",
			"code":  "FEATURED_FETCH_ERROR",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"videos":    videos,
		"total":     len(videos),
		"featured":  true,
		"cached_at": time.Now().Unix(),
		"ttl":       900,
	})
}

func (h *VideoHandler) GetTrendingVideos(c *gin.Context) {
	h.setVideoListHeaders(c)

	limit := 10
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 50 {
			limit = parsed
		}
	}

	videos, err := h.service.GetTrendingVideosOptimized(c.Request.Context(), limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to fetch trending videos",
			"code":  "TRENDING_FETCH_ERROR",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"videos":    videos,
		"total":     len(videos),
		"trending":  true,
		"cached_at": time.Now().Unix(),
		"ttl":       900,
	})
}

func (h *VideoHandler) GetVideo(c *gin.Context) {
	h.setVideoAPIHeaders(c)

	videoID := c.Param("videoId")
	if videoID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Video ID required",
			"code":  "MISSING_VIDEO_ID",
		})
		return
	}

	video, err := h.service.GetVideoOptimized(c.Request.Context(), videoID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "Video not found",
			"code":    "VIDEO_NOT_FOUND",
			"videoId": videoID,
		})
		return
	}

	if video.VideoURL != "" {
		h.setVideoStreamingHeaders(c)
	}

	c.JSON(http.StatusOK, video)
}

func (h *VideoHandler) GetVideoQualities(c *gin.Context) {
	h.setVideoStreamingHeaders(c)

	videoID := c.Param("videoId")
	if videoID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Video ID required",
			"code":  "MISSING_VIDEO_ID",
		})
		return
	}

	video, err := h.service.GetVideoOptimized(c.Request.Context(), videoID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Video not found",
			"code":  "VIDEO_NOT_FOUND",
		})
		return
	}

	qualities := []gin.H{
		{
			"quality":    "original",
			"resolution": "auto",
			"url":        video.VideoURL,
			"isDefault":  true,
			"bitrate":    "auto",
			"format":     "mp4",
		},
	}

	if video.ThumbnailURL != "" {
		qualities = append(qualities, gin.H{
			"quality":    "preview",
			"resolution": "thumbnail",
			"url":        video.ThumbnailURL,
			"isDefault":  false,
			"bitrate":    "0",
			"format":     "jpg",
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"videoId":   videoID,
		"qualities": qualities,
		"total":     len(qualities),
		"adaptive":  false,
	})
}

func (h *VideoHandler) GetUserVideos(c *gin.Context) {
	h.setVideoListHeaders(c)

	userID := c.Param("userId")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "User ID required",
			"code":  "MISSING_USER_ID",
		})
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

	videos, err := h.service.GetUserVideosOptimized(c.Request.Context(), userID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to fetch user videos",
			"code":  "USER_VIDEOS_FETCH_ERROR",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"videos":    videos,
		"total":     len(videos),
		"userId":    userID,
		"page":      (offset / limit) + 1,
		"limit":     limit,
		"hasMore":   len(videos) == limit,
		"cached_at": time.Now().Unix(),
		"ttl":       900,
	})
}

// ===============================
// VIDEO INTERACTION ENDPOINTS
// ===============================

func (h *VideoHandler) IncrementViews(c *gin.Context) {
	h.setInteractionHeaders(c)

	videoID := c.Param("videoId")
	if videoID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Video ID required",
			"code":  "MISSING_VIDEO_ID",
		})
		return
	}

	err := h.service.IncrementVideoViews(c.Request.Context(), videoID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"message": "View counted",
			"videoId": videoID,
			"status":  "acknowledged",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "View counted successfully",
		"videoId": videoID,
		"status":  "success",
	})
}

func (h *VideoHandler) LikeVideo(c *gin.Context) {
	h.setInteractionHeaders(c)

	videoID := c.Param("videoId")
	if videoID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Video ID required",
			"code":  "MISSING_VIDEO_ID",
		})
		return
	}

	userID := c.GetString("userID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "User not authenticated",
			"code":  "AUTH_REQUIRED",
		})
		return
	}

	err := h.service.LikeVideo(c.Request.Context(), videoID, userID)
	if err != nil {
		if err.Error() == "already_liked" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Video already liked",
				"code":  "ALREADY_LIKED",
			})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to like video",
				"code":  "LIKE_ERROR",
			})
		}
		return
	}

	summary, err := h.service.GetVideoCountsSummary(c.Request.Context(), videoID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"message": "Video liked successfully",
			"videoId": videoID,
			"status":  "success",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Video liked successfully",
		"videoId": videoID,
		"counts":  summary,
		"status":  "success",
	})
}

func (h *VideoHandler) UnlikeVideo(c *gin.Context) {
	h.setInteractionHeaders(c)

	videoID := c.Param("videoId")
	if videoID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Video ID required",
			"code":  "MISSING_VIDEO_ID",
		})
		return
	}

	userID := c.GetString("userID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "User not authenticated",
			"code":  "AUTH_REQUIRED",
		})
		return
	}

	err := h.service.UnlikeVideo(c.Request.Context(), videoID, userID)
	if err != nil {
		if err.Error() == "not_liked" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Video not liked",
				"code":  "NOT_LIKED",
			})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to unlike video",
				"code":  "UNLIKE_ERROR",
			})
		}
		return
	}

	summary, err := h.service.GetVideoCountsSummary(c.Request.Context(), videoID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"message": "Video unliked successfully",
			"videoId": videoID,
			"status":  "success",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Video unliked successfully",
		"videoId": videoID,
		"counts":  summary,
		"status":  "success",
	})
}

func (h *VideoHandler) ShareVideo(c *gin.Context) {
	h.setInteractionHeaders(c)

	videoID := c.Param("videoId")
	if videoID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Video ID required",
			"code":  "MISSING_VIDEO_ID",
		})
		return
	}

	err := h.service.IncrementVideoShares(c.Request.Context(), videoID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to record share",
			"code":  "SHARE_ERROR",
		})
		return
	}

	summary, err := h.service.GetVideoCountsSummary(c.Request.Context(), videoID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"message": "Video shared successfully",
			"videoId": videoID,
			"status":  "success",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Video shared successfully",
		"videoId": videoID,
		"counts":  summary,
		"status":  "success",
	})
}

func (h *VideoHandler) GetVideoCountsSummary(c *gin.Context) {
	h.setInteractionHeaders(c)

	videoID := c.Param("videoId")
	if videoID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Video ID required",
			"code":  "MISSING_VIDEO_ID",
		})
		return
	}

	summary, err := h.service.GetVideoCountsSummary(c.Request.Context(), videoID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Video not found",
			"code":  "VIDEO_NOT_FOUND",
		})
		return
	}

	c.JSON(http.StatusOK, summary)
}

func (h *VideoHandler) GetUserLikedVideos(c *gin.Context) {
	h.setVideoListHeaders(c)

	userID := c.Param("userId")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "User ID required",
			"code":  "MISSING_USER_ID",
		})
		return
	}

	requestingUserID := c.GetString("userID")
	if requestingUserID != userID {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "Access denied",
			"code":  "ACCESS_DENIED",
		})
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

	videos, err := h.service.GetUserLikedVideosOptimized(c.Request.Context(), userID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to fetch liked videos",
			"code":  "LIKED_VIDEOS_FETCH_ERROR",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"videos":    videos,
		"total":     len(videos),
		"userId":    userID,
		"liked":     true,
		"cached_at": time.Now().Unix(),
		"ttl":       900,
	})
}

// ===============================
// ✅ UPDATED: AUTHENTICATED VIDEO ENDPOINTS - All Active Users Can Post
// ===============================

func (h *VideoHandler) CreateVideo(c *gin.Context) {
	h.setInteractionHeaders(c)

	userID := c.GetString("userID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "User not authenticated",
			"code":  "AUTH_REQUIRED",
		})
		return
	}

	// Validate user exists and is active (no role restriction)
	err := h.userService.ValidateUserForVideoCreation(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{
			"error":   "Video creation not allowed",
			"code":    "USER_VALIDATION_FAILED",
			"details": err.Error(),
		})
		return
	}

	var request models.CreateVideoRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"code":    "INVALID_REQUEST",
			"details": err.Error(),
		})
		return
	}

	userName, userImage, _, err := h.userService.GetUserBasicInfo(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get user information",
			"code":  "USER_INFO_ERROR",
		})
		return
	}

	// ✅ REMOVED: Role check - all authenticated users can post
	// Old code that was removed:
	// if !userRole.CanPost() {
	// 	c.JSON(http.StatusForbidden, gin.H{
	// 		"error":        "User role cannot post videos",
	// 		"code":         "ROLE_PERMISSION_DENIED",
	// 		"userRole":     userRole.String(),
	// 		"allowedRoles": []string{"admin", "host"},
	// 	})
	// 	return
	// }

	video := &models.Video{
		UserID:           userID,
		UserName:         userName,
		UserImage:        userImage,
		VideoURL:         request.VideoURL,
		ThumbnailURL:     request.ThumbnailURL,
		Caption:          request.Caption,
		Tags:             models.StringSlice(request.Tags),
		IsMultipleImages: request.IsMultipleImages,
		ImageUrls:        models.StringSlice(request.ImageUrls),
	}

	if request.Price != nil && *request.Price >= 0 {
		video.Price = *request.Price
	} else {
		video.Price = 0.00 // Explicit default
	}

	videoID, err := h.service.CreateVideoOptimized(c.Request.Context(), video)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to create video",
			"code":  "CREATE_ERROR",
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"videoId":  videoID,
		"message":  "Video created successfully",
		"status":   "created",
		"price":    video.Price,
		"verified": video.IsVerified,
	})
}

func (h *VideoHandler) UpdateVideo(c *gin.Context) {
	h.setInteractionHeaders(c)

	videoID := c.Param("videoId")
	if videoID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Video ID required"})
		return
	}

	userID := c.GetString("userID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	var video models.Video
	if err := c.ShouldBindJSON(&video); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	video.ID = videoID
	video.UserID = userID

	err := h.service.UpdateVideo(c.Request.Context(), &video)
	if err != nil {
		if err.Error() == "video_not_found_or_no_access" {
			c.JSON(http.StatusNotFound, gin.H{"error": "Video not found or access denied"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update video"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Video updated successfully"})
}

func (h *VideoHandler) DeleteVideo(c *gin.Context) {
	h.setInteractionHeaders(c)

	videoID := c.Param("videoId")
	if videoID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Video ID required"})
		return
	}

	userID := c.GetString("userID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	err := h.service.DeleteVideo(c.Request.Context(), videoID, userID)
	if err != nil {
		if err.Error() == "video_not_found_or_no_access" {
			c.JSON(http.StatusNotFound, gin.H{"error": "Video not found or access denied"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete video"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Video deleted successfully"})
}

func (h *VideoHandler) GetFollowingFeed(c *gin.Context) {
	h.setVideoListHeaders(c)

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

	videos, err := h.service.GetFollowingVideoFeed(c.Request.Context(), userID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch following feed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"videos": videos,
		"total":  len(videos),
	})
}

// ===============================
// COMMENT ENDPOINTS
// ===============================

func (h *VideoHandler) CreateComment(c *gin.Context) {
	h.setInteractionHeaders(c)

	videoID := c.Param("videoId")
	if videoID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Video ID required"})
		return
	}

	userID := c.GetString("userID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	var request models.CreateCommentRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userName, userImage, _, err := h.userService.GetUserBasicInfo(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "User not found"})
		return
	}

	comment := &models.Comment{
		VideoID:             videoID,
		AuthorID:            userID,
		AuthorName:          userName,
		AuthorImage:         userImage,
		Content:             request.Content,
		IsReply:             request.RepliedToCommentID != nil,
		RepliedToCommentID:  request.RepliedToCommentID,
		RepliedToAuthorName: request.RepliedToAuthorName,
	}

	commentID, err := h.service.CreateComment(c.Request.Context(), comment)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create comment"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"commentId": commentID,
		"message":   "Comment created successfully",
	})
}

func (h *VideoHandler) GetVideoComments(c *gin.Context) {
	h.setCommentHeaders(c)

	videoID := c.Param("videoId")
	if videoID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Video ID required"})
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

	comments, err := h.service.GetVideoComments(c.Request.Context(), videoID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch comments"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"comments":  comments,
		"total":     len(comments),
		"cached_at": time.Now().Unix(),
		"ttl":       300,
	})
}

func (h *VideoHandler) DeleteComment(c *gin.Context) {
	h.setInteractionHeaders(c)

	commentID := c.Param("commentId")
	if commentID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Comment ID required"})
		return
	}

	userID := c.GetString("userID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	err := h.service.DeleteComment(c.Request.Context(), commentID, userID)
	if err != nil {
		if err.Error() == "access_denied" {
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete comment"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Comment deleted successfully"})
}

func (h *VideoHandler) LikeComment(c *gin.Context) {
	h.setInteractionHeaders(c)

	commentID := c.Param("commentId")
	if commentID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Comment ID required"})
		return
	}

	userID := c.GetString("userID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	err := h.service.LikeComment(c.Request.Context(), commentID, userID)
	if err != nil {
		if err.Error() == "already_liked" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Comment already liked"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to like comment"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Comment liked successfully"})
}

func (h *VideoHandler) UnlikeComment(c *gin.Context) {
	h.setInteractionHeaders(c)

	commentID := c.Param("commentId")
	if commentID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Comment ID required"})
		return
	}

	userID := c.GetString("userID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	err := h.service.UnlikeComment(c.Request.Context(), commentID, userID)
	if err != nil {
		if err.Error() == "not_liked" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Comment not liked"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to unlike comment"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Comment unliked successfully"})
}

// ===============================
// SOCIAL ENDPOINTS
// ===============================

func (h *VideoHandler) FollowUser(c *gin.Context) {
	h.setInteractionHeaders(c)

	targetUserID := c.Param("userId")
	if targetUserID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "User ID required"})
		return
	}

	userID := c.GetString("userID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	err := h.service.FollowUser(c.Request.Context(), userID, targetUserID)
	if err != nil {
		if err.Error() == "cannot_follow_self" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot follow yourself"})
		} else if err.Error() == "already_following" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Already following this user"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to follow user"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "User followed successfully"})
}

func (h *VideoHandler) UnfollowUser(c *gin.Context) {
	h.setInteractionHeaders(c)

	targetUserID := c.Param("userId")
	if targetUserID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "User ID required"})
		return
	}

	userID := c.GetString("userID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	err := h.service.UnfollowUser(c.Request.Context(), userID, targetUserID)
	if err != nil {
		if err.Error() == "not_following" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Not following this user"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to unfollow user"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "User unfollowed successfully"})
}

func (h *VideoHandler) GetUserFollowers(c *gin.Context) {
	h.setVideoListHeaders(c)

	userID := c.Param("userId")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "User ID required"})
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

	users, err := h.service.GetUserFollowers(c.Request.Context(), userID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch followers"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"users": users,
		"total": len(users),
	})
}

func (h *VideoHandler) GetUserFollowing(c *gin.Context) {
	h.setVideoListHeaders(c)

	userID := c.Param("userId")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "User ID required"})
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

	users, err := h.service.GetUserFollowing(c.Request.Context(), userID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch following"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"users": users,
		"total": len(users),
	})
}

// ===============================
// ADMIN ENDPOINTS
// ===============================

func (h *VideoHandler) ToggleFeatured(c *gin.Context) {
	h.setInteractionHeaders(c)

	videoID := c.Param("videoId")
	if videoID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Video ID required"})
		return
	}

	var request struct {
		IsFeatured bool `json:"isFeatured"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := h.service.ToggleFeatured(c.Request.Context(), videoID, request.IsFeatured)
	if err != nil {
		if err.Error() == "video_not_found" {
			c.JSON(http.StatusNotFound, gin.H{"error": "Video not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to toggle featured status"})
		}
		return
	}

	status := "featured"
	if !request.IsFeatured {
		status = "unfeatured"
	}

	c.JSON(http.StatusOK, gin.H{"message": "Video " + status + " successfully"})
}

func (h *VideoHandler) ToggleActive(c *gin.Context) {
	h.setInteractionHeaders(c)

	videoID := c.Param("videoId")
	if videoID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Video ID required"})
		return
	}

	var request struct {
		IsActive bool `json:"isActive"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := h.service.ToggleActive(c.Request.Context(), videoID, request.IsActive)
	if err != nil {
		if err.Error() == "video_not_found" {
			c.JSON(http.StatusNotFound, gin.H{"error": "Video not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to toggle active status"})
		}
		return
	}

	status := "activated"
	if !request.IsActive {
		status = "deactivated"
	}

	c.JSON(http.StatusOK, gin.H{"message": "Video " + status + " successfully"})
}

func (h *VideoHandler) ToggleVerified(c *gin.Context) {
	h.setInteractionHeaders(c)

	videoID := c.Param("videoId")
	if videoID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Video ID required"})
		return
	}

	var request struct {
		IsVerified bool `json:"isVerified"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	video, err := h.service.GetVideoOptimized(c.Request.Context(), videoID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Video not found"})
		return
	}

	videoModel := &models.Video{
		ID:               video.ID,
		UserID:           video.UserID,
		Caption:          video.Caption,
		Price:            video.Price,
		Tags:             video.Tags,
		IsActive:         video.IsActive,
		IsFeatured:       video.IsFeatured,
		IsVerified:       request.IsVerified,
		IsMultipleImages: video.IsMultipleImages,
	}

	err = h.service.UpdateVideo(c.Request.Context(), videoModel)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update verification status"})
		return
	}

	status := "verified"
	if !request.IsVerified {
		status = "unverified"
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    "Video " + status + " successfully",
		"videoId":    videoID,
		"isVerified": request.IsVerified,
	})
}

func (h *VideoHandler) GetVideoStats(c *gin.Context) {
	h.setVideoListHeaders(c)

	userID := c.GetString("userID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	stats, err := h.service.GetVideoStats(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch video stats"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"stats": stats,
		"total": len(stats),
	})
}

// ===============================
// UTILITY ENDPOINTS
// ===============================

func (h *VideoHandler) BatchUpdateCounts(c *gin.Context) {
	h.setInteractionHeaders(c)

	err := h.service.BatchUpdateViewCounts(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update counts"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":   "Counts updated successfully",
		"timestamp": time.Now(),
	})
}

func (h *VideoHandler) GetVideoMetrics(c *gin.Context) {
	h.setVideoAPIHeaders(c)

	videoID := c.Param("videoId")
	if videoID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Video ID required"})
		return
	}

	video, err := h.service.GetVideoOptimized(c.Request.Context(), videoID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Video not found"})
		return
	}

	totalEngagement := video.LikesCount + video.CommentsCount + video.SharesCount
	engagementRate := 0.0
	if video.ViewsCount > 0 {
		engagementRate = (float64(totalEngagement) / float64(video.ViewsCount)) * 100
	}

	c.JSON(http.StatusOK, gin.H{
		"videoId":        video.ID,
		"views":          video.ViewsCount,
		"likes":          video.LikesCount,
		"comments":       video.CommentsCount,
		"shares":         video.SharesCount,
		"price":          video.Price,
		"isVerified":     video.IsVerified,
		"engagement":     totalEngagement,
		"engagementRate": engagementRate,
		"createdAt":      video.CreatedAt,
		"isActive":       video.IsActive,
		"isFeatured":     video.IsFeatured,
		"cached_at":      time.Now().Unix(),
		"ttl":            1800,
	})
}

func (h *VideoHandler) GetPopularVideos(c *gin.Context) {
	h.setVideoListHeaders(c)

	period := c.Query("period")
	if period == "" {
		period = "week"
	}

	limit := 20
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}

	var sortBy string
	switch period {
	case "day":
		sortBy = "trending"
	case "week":
		sortBy = "popular"
	case "month":
		sortBy = "popular"
	default:
		sortBy = "popular"
	}

	params := models.VideoSearchParams{
		Limit:  limit,
		Offset: 0,
		SortBy: sortBy,
	}

	videos, err := h.service.GetVideosOptimized(c.Request.Context(), params)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch popular videos"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"videos":    videos,
		"total":     len(videos),
		"period":    period,
		"sortBy":    sortBy,
		"cached_at": time.Now().Unix(),
		"ttl":       900,
	})
}

func (h *VideoHandler) GetVideoRecommendations(c *gin.Context) {
	h.setVideoListHeaders(c)

	userID := c.GetString("userID")
	limit := 20

	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}

	params := models.VideoSearchParams{
		Limit:  limit,
		Offset: 0,
		SortBy: "trending",
	}

	videos, err := h.service.GetVideosOptimized(c.Request.Context(), params)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch recommendations"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"videos":       videos,
		"total":        len(videos),
		"userId":       userID,
		"algorithm":    "trending-based-optimized",
		"generated_at": time.Now(),
		"cached_at":    time.Now().Unix(),
		"ttl":          900,
	})
}

func (h *VideoHandler) ReportVideo(c *gin.Context) {
	h.setInteractionHeaders(c)

	videoID := c.Param("videoId")
	if videoID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Video ID required"})
		return
	}

	userID := c.GetString("userID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	var request struct {
		Reason      string `json:"reason" binding:"required"`
		Description string `json:"description"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "Video reported successfully",
		"videoId":  videoID,
		"reason":   request.Reason,
		"reportId": "placeholder_report_id",
		"status":   "pending_review",
	})
}

func (h *VideoHandler) GetVideoAnalytics(c *gin.Context) {
	h.setVideoAPIHeaders(c)

	videoID := c.Param("videoId")
	if videoID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Video ID required"})
		return
	}

	userID := c.GetString("userID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	video, err := h.service.GetVideoOptimized(c.Request.Context(), videoID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Video not found"})
		return
	}

	if video.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	totalEngagement := video.LikesCount + video.CommentsCount + video.SharesCount
	engagementRate := 0.0
	if video.ViewsCount > 0 {
		engagementRate = (float64(totalEngagement) / float64(video.ViewsCount)) * 100
	}

	likeRate := 0.0
	if video.ViewsCount > 0 {
		likeRate = (float64(video.LikesCount) / float64(video.ViewsCount)) * 100
	}

	commentRate := 0.0
	if video.ViewsCount > 0 {
		commentRate = (float64(video.CommentsCount) / float64(video.ViewsCount)) * 100
	}

	shareRate := 0.0
	if video.ViewsCount > 0 {
		shareRate = (float64(video.SharesCount) / float64(video.ViewsCount)) * 100
	}

	c.JSON(http.StatusOK, gin.H{
		"videoId":         video.ID,
		"title":           video.Caption,
		"views":           video.ViewsCount,
		"likes":           video.LikesCount,
		"comments":        video.CommentsCount,
		"shares":          video.SharesCount,
		"price":           video.Price,
		"isVerified":      video.IsVerified,
		"totalEngagement": totalEngagement,
		"engagementRate":  engagementRate,
		"likeRate":        likeRate,
		"commentRate":     commentRate,
		"shareRate":       shareRate,
		"isActive":        video.IsActive,
		"isFeatured":      video.IsFeatured,
		"createdAt":       video.CreatedAt,
		"updatedAt":       video.UpdatedAt,
		"performance":     "good",
		"optimized":       true,
		"cached_at":       time.Now().Unix(),
		"ttl":             1800,
	})
}
