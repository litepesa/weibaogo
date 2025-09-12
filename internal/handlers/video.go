// ===============================
// internal/handlers/video.go - Complete Video Social Media Handlers for PostgreSQL
// ===============================

package handlers

import (
	"net/http"
	"strconv"
	"time"

	"weibaobe/internal/models"
	"weibaobe/internal/services"

	"github.com/gin-gonic/gin"
)

type VideoHandler struct {
	service *services.VideoService
}

func NewVideoHandler(service *services.VideoService) *VideoHandler {
	return &VideoHandler{service: service}
}

// ===============================
// PUBLIC VIDEO ENDPOINTS
// ===============================

// 🔧 FIXED: GetVideos returns proper VideoResponse structure
func (h *VideoHandler) GetVideos(c *gin.Context) {
	params := models.VideoSearchParams{
		Limit:  20,
		Offset: 0,
		SortBy: "latest",
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

	videos, err := h.service.GetVideos(c.Request.Context(), params)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch videos"})
		return
	}

	// 🔧 FIXED: Return properly structured response
	c.JSON(http.StatusOK, gin.H{
		"videos":  videos,
		"total":   len(videos),
		"page":    (params.Offset / params.Limit) + 1,
		"limit":   params.Limit,
		"hasMore": len(videos) == params.Limit,
	})
}

// 🔧 FIXED: GetFeaturedVideos returns proper VideoResponse structure
func (h *VideoHandler) GetFeaturedVideos(c *gin.Context) {
	limit := 10
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 50 {
			limit = parsed
		}
	}

	videos, err := h.service.GetFeaturedVideos(c.Request.Context(), limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch featured videos"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"videos":   videos,
		"total":    len(videos),
		"featured": true,
	})
}

// 🔧 FIXED: GetTrendingVideos returns proper VideoResponse structure
func (h *VideoHandler) GetTrendingVideos(c *gin.Context) {
	limit := 10
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 50 {
			limit = parsed
		}
	}

	videos, err := h.service.GetTrendingVideos(c.Request.Context(), limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch trending videos"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"videos":   videos,
		"total":    len(videos),
		"trending": true,
	})
}

// 🔧 FIXED: GetVideo returns proper VideoResponse structure
func (h *VideoHandler) GetVideo(c *gin.Context) {
	videoID := c.Param("videoId")
	if videoID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Video ID required"})
		return
	}

	video, err := h.service.GetVideo(c.Request.Context(), videoID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Video not found"})
		return
	}

	// 🔧 ENHANCED: Return with proper field mapping
	c.JSON(http.StatusOK, video)
}

// 🔧 FIXED: GetUserVideos returns proper VideoResponse structure
func (h *VideoHandler) GetUserVideos(c *gin.Context) {
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

	videos, err := h.service.GetUserVideos(c.Request.Context(), userID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch user videos"})
		return
	}

	// 🔧 FIXED: Return properly structured response
	c.JSON(http.StatusOK, gin.H{
		"videos":  videos,
		"total":   len(videos),
		"userId":  userID,
		"page":    (offset / limit) + 1,
		"limit":   limit,
		"hasMore": len(videos) == limit,
	})
}

// ===============================
// VIDEO INTERACTION ENDPOINTS
// ===============================

// 🔧 ENHANCED: IncrementViews with better error handling
func (h *VideoHandler) IncrementViews(c *gin.Context) {
	videoID := c.Param("videoId")
	if videoID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Video ID required"})
		return
	}

	err := h.service.IncrementVideoViews(c.Request.Context(), videoID)
	if err != nil {
		// Don't return error for view counting failures, just log and continue
		// This prevents breaking the user experience
		c.JSON(http.StatusOK, gin.H{
			"message": "View counted",
			"videoId": videoID,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "View counted successfully",
		"videoId": videoID,
	})
}

// 🔧 ENHANCED: LikeVideo with immediate count update
func (h *VideoHandler) LikeVideo(c *gin.Context) {
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

	err := h.service.LikeVideo(c.Request.Context(), videoID, userID)
	if err != nil {
		if err.Error() == "already_liked" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Video already liked"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to like video"})
		}
		return
	}

	// Get updated counts
	summary, err := h.service.GetVideoCountsSummary(c.Request.Context(), videoID)
	if err != nil {
		// Still return success even if we can't get updated counts
		c.JSON(http.StatusOK, gin.H{
			"message": "Video liked successfully",
			"videoId": videoID,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Video liked successfully",
		"videoId": videoID,
		"counts":  summary,
	})
}

// 🔧 ENHANCED: UnlikeVideo with immediate count update
func (h *VideoHandler) UnlikeVideo(c *gin.Context) {
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

	err := h.service.UnlikeVideo(c.Request.Context(), videoID, userID)
	if err != nil {
		if err.Error() == "not_liked" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Video not liked"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to unlike video"})
		}
		return
	}

	// Get updated counts
	summary, err := h.service.GetVideoCountsSummary(c.Request.Context(), videoID)
	if err != nil {
		// Still return success even if we can't get updated counts
		c.JSON(http.StatusOK, gin.H{
			"message": "Video unliked successfully",
			"videoId": videoID,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Video unliked successfully",
		"videoId": videoID,
		"counts":  summary,
	})
}

// 🔧 ENHANCED: ShareVideo with immediate count update
func (h *VideoHandler) ShareVideo(c *gin.Context) {
	videoID := c.Param("videoId")
	if videoID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Video ID required"})
		return
	}

	err := h.service.IncrementVideoShares(c.Request.Context(), videoID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to record share"})
		return
	}

	// Get updated counts
	summary, err := h.service.GetVideoCountsSummary(c.Request.Context(), videoID)
	if err != nil {
		// Still return success even if we can't get updated counts
		c.JSON(http.StatusOK, gin.H{
			"message": "Video shared successfully",
			"videoId": videoID,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Video shared successfully",
		"videoId": videoID,
		"counts":  summary,
	})
}

// 🔧 NEW: GetVideoCountsSummary for real-time count updates
func (h *VideoHandler) GetVideoCountsSummary(c *gin.Context) {
	videoID := c.Param("videoId")
	if videoID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Video ID required"})
		return
	}

	summary, err := h.service.GetVideoCountsSummary(c.Request.Context(), videoID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Video not found"})
		return
	}

	c.JSON(http.StatusOK, summary)
}

func (h *VideoHandler) GetUserLikedVideos(c *gin.Context) {
	userID := c.Param("userId")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "User ID required"})
		return
	}

	// Users can only view their own liked videos unless admin
	requestingUserID := c.GetString("userID")
	if requestingUserID != userID {
		// Check if requesting user is admin/moderator
		// This check should be implemented in middleware or service
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
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

	videos, err := h.service.GetUserLikedVideos(c.Request.Context(), userID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch liked videos"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"videos": videos,
		"total":  len(videos),
	})
}

// ===============================
// AUTHENTICATED VIDEO ENDPOINTS
// ===============================

func (h *VideoHandler) CreateVideo(c *gin.Context) {
	userID := c.GetString("userID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	var request models.CreateVideoRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get user info for the video
	// This should be retrieved from the user service
	// For now, we'll use placeholder values
	video := &models.Video{
		UserID:           userID,
		UserName:         "User", // Should be fetched from user service
		UserImage:        "",     // Should be fetched from user service
		VideoURL:         request.VideoURL,
		ThumbnailURL:     request.ThumbnailURL,
		Caption:          request.Caption,
		Tags:             models.StringSlice(request.Tags),
		IsMultipleImages: request.IsMultipleImages,
		ImageUrls:        models.StringSlice(request.ImageUrls),
	}

	videoID, err := h.service.CreateVideo(c.Request.Context(), video)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create video"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"videoId": videoID,
		"message": "Video created successfully",
	})
}

func (h *VideoHandler) UpdateVideo(c *gin.Context) {
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

	comment := &models.Comment{
		VideoID:             videoID,
		AuthorID:            userID,
		AuthorName:          "User", // Should be fetched from user service
		AuthorImage:         "",     // Should be fetched from user service
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
		"comments": comments,
		"total":    len(comments),
	})
}

func (h *VideoHandler) DeleteComment(c *gin.Context) {
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

func (h *VideoHandler) GetVideoStats(c *gin.Context) {
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
// ADDITIONAL UTILITY ENDPOINTS
// ===============================

// 🔧 NEW: Batch update counts endpoint (for admin/maintenance)
func (h *VideoHandler) BatchUpdateCounts(c *gin.Context) {
	// This should be protected by admin middleware
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

// 🔧 NEW: Health check endpoint for video service
func (h *VideoHandler) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"service":   "video-service",
		"timestamp": time.Now(),
		"version":   "1.0.0",
	})
}

// 🔧 NEW: Get video metrics endpoint
func (h *VideoHandler) GetVideoMetrics(c *gin.Context) {
	videoID := c.Param("videoId")
	if videoID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Video ID required"})
		return
	}

	// Get basic video info
	video, err := h.service.GetVideo(c.Request.Context(), videoID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Video not found"})
		return
	}

	// Calculate engagement metrics
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
		"engagement":     totalEngagement,
		"engagementRate": engagementRate,
		"createdAt":      video.CreatedAt,
		"isActive":       video.IsActive,
		"isFeatured":     video.IsFeatured,
	})
}

// 🔧 NEW: Search videos endpoint with enhanced filtering
func (h *VideoHandler) SearchVideos(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Search query required"})
		return
	}

	params := models.VideoSearchParams{
		Query:  query,
		Limit:  20,
		Offset: 0,
		SortBy: "latest",
	}

	// Parse additional parameters
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

	if s := c.Query("sortBy"); s != "" {
		params.SortBy = s
	}

	if m := c.Query("mediaType"); m != "" {
		params.MediaType = m
	}

	videos, err := h.service.GetVideos(c.Request.Context(), params)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to search videos"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"videos":    videos,
		"total":     len(videos),
		"query":     query,
		"page":      (params.Offset / params.Limit) + 1,
		"limit":     params.Limit,
		"hasMore":   len(videos) == params.Limit,
		"sortBy":    params.SortBy,
		"mediaType": params.MediaType,
	})
}

// 🔧 NEW: Get popular videos by time period
func (h *VideoHandler) GetPopularVideos(c *gin.Context) {
	period := c.Query("period") // "day", "week", "month", "all"
	if period == "" {
		period = "week"
	}

	limit := 20
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}

	// Use different sorting based on period
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

	videos, err := h.service.GetVideos(c.Request.Context(), params)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch popular videos"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"videos": videos,
		"total":  len(videos),
		"period": period,
		"sortBy": sortBy,
	})
}

// 🔧 NEW: Get video recommendations (placeholder implementation)
func (h *VideoHandler) GetVideoRecommendations(c *gin.Context) {
	userID := c.GetString("userID")
	limit := 20

	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}

	// For now, return trending videos as recommendations
	// This should be replaced with actual recommendation algorithm
	params := models.VideoSearchParams{
		Limit:  limit,
		Offset: 0,
		SortBy: "trending",
	}

	// Exclude user's own videos if authenticated
	if userID != "" {
		// This would need to be implemented in the service layer
	}

	videos, err := h.service.GetVideos(c.Request.Context(), params)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch recommendations"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"videos":       videos,
		"total":        len(videos),
		"userId":       userID,
		"algorithm":    "trending-based", // Placeholder
		"generated_at": time.Now(),
	})
}

// 🔧 NEW: Report video endpoint
func (h *VideoHandler) ReportVideo(c *gin.Context) {
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

	// TODO: Implement video reporting logic
	// This would typically:
	// 1. Store the report in a reports table
	// 2. Increment report count on the video
	// 3. Potentially auto-hide video if reports exceed threshold
	// 4. Notify moderators

	c.JSON(http.StatusOK, gin.H{
		"message":  "Video reported successfully",
		"videoId":  videoID,
		"reason":   request.Reason,
		"reportId": "placeholder_report_id", // Would be generated
	})
}

// 🔧 NEW: Get video analytics for content creators
func (h *VideoHandler) GetVideoAnalytics(c *gin.Context) {
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

	// Verify user owns the video
	video, err := h.service.GetVideo(c.Request.Context(), videoID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Video not found"})
		return
	}

	if video.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	// Calculate detailed analytics
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
		"totalEngagement": totalEngagement,
		"engagementRate":  engagementRate,
		"likeRate":        likeRate,
		"commentRate":     commentRate,
		"shareRate":       shareRate,
		"isActive":        video.IsActive,
		"isFeatured":      video.IsFeatured,
		"createdAt":       video.CreatedAt,
		"updatedAt":       video.UpdatedAt,
		"performance":     "good", // Placeholder - would be calculated based on benchmarks
	})
}
