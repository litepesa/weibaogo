// ===============================
// internal/models/video.go - Video Social Media Model (FIXED PostgreSQL Array Issue)
// ===============================

package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// StringSlice represents a slice of strings that can be stored in PostgreSQL as TEXT[]
type StringSlice []string

// 🔧 CRITICAL FIX: Value implements driver.Valuer for database storage
// Fixed to generate PostgreSQL array format instead of JSON format
func (s StringSlice) Value() (driver.Value, error) {
	if s == nil {
		return nil, nil
	}

	// 🔧 FIXED: Generate PostgreSQL array literal format instead of JSON
	if len(s) == 0 {
		return "{}", nil // PostgreSQL empty array format
	}

	// Escape each string and wrap in quotes for PostgreSQL
	escapedStrings := make([]string, len(s))
	for i, str := range s {
		// Escape quotes and backslashes for PostgreSQL
		escaped := strings.ReplaceAll(str, `\`, `\\`)
		escaped = strings.ReplaceAll(escaped, `"`, `\"`)
		escapedStrings[i] = `"` + escaped + `"`
	}

	// Create PostgreSQL array format: {"item1","item2","item3"}
	return "{" + strings.Join(escapedStrings, ",") + "}", nil
}

// 🔧 FIXED: Scan implements sql.Scanner for database retrieval
// Enhanced to handle both PostgreSQL array format and JSON format
func (s *StringSlice) Scan(value interface{}) error {
	if value == nil {
		*s = nil
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("cannot scan %T into StringSlice", value)
	}

	str := string(bytes)

	// Handle empty cases
	if str == "" || str == "{}" || str == "[]" {
		*s = StringSlice{}
		return nil
	}

	// 🔧 FIXED: Handle PostgreSQL array format: {item1,item2,item3}
	if strings.HasPrefix(str, "{") && strings.HasSuffix(str, "}") {
		// Remove braces
		content := str[1 : len(str)-1]
		if content == "" {
			*s = StringSlice{}
			return nil
		}

		// Split by comma and clean up quotes
		items := strings.Split(content, ",")
		result := make([]string, 0, len(items))

		for _, item := range items {
			// Trim spaces and quotes
			cleaned := strings.TrimSpace(item)
			if strings.HasPrefix(cleaned, `"`) && strings.HasSuffix(cleaned, `"`) {
				cleaned = cleaned[1 : len(cleaned)-1]
			}
			// Unescape PostgreSQL escapes
			cleaned = strings.ReplaceAll(cleaned, `\"`, `"`)
			cleaned = strings.ReplaceAll(cleaned, `\\`, `\`)

			if cleaned != "" {
				result = append(result, cleaned)
			}
		}

		*s = result
		return nil
	}

	// Fallback: Try parsing as JSON (for backward compatibility)
	var jsonSlice []string
	if err := json.Unmarshal(bytes, &jsonSlice); err != nil {
		return fmt.Errorf("cannot parse StringSlice from %s: %w", str, err)
	}

	*s = jsonSlice
	return nil
}

// 🔧 FIXED: Video model with proper JSON tags for frontend compatibility
type Video struct {
	ID           string `json:"id" db:"id"`
	UserID       string `json:"userId" db:"user_id" binding:"required"`
	UserName     string `json:"userName" db:"user_name" binding:"required"`
	UserImage    string `json:"userImage" db:"user_image"`
	VideoURL     string `json:"videoUrl" db:"video_url"`
	ThumbnailURL string `json:"thumbnailUrl" db:"thumbnail_url"`
	Caption      string `json:"caption" db:"caption"`

	// 🔧 CRITICAL FIX: Map database fields to frontend-expected JSON names
	LikesCount    int `json:"likes" db:"likes_count"`       // Frontend expects "likes"
	CommentsCount int `json:"comments" db:"comments_count"` // Frontend expects "comments"
	ViewsCount    int `json:"views" db:"views_count"`       // Frontend expects "views"
	SharesCount   int `json:"shares" db:"shares_count"`     // Frontend expects "shares"

	Tags             StringSlice `json:"tags" db:"tags"`
	IsActive         bool        `json:"isActive" db:"is_active"`
	IsFeatured       bool        `json:"isFeatured" db:"is_featured"`
	IsMultipleImages bool        `json:"isMultipleImages" db:"is_multiple_images"`
	ImageUrls        StringSlice `json:"imageUrls" db:"image_urls"`
	CreatedAt        time.Time   `json:"createdAt" db:"created_at"`
	UpdatedAt        time.Time   `json:"updatedAt" db:"updated_at"`

	// Runtime fields (not stored in DB)
	IsLiked     bool `json:"isLiked" db:"-"`
	IsFollowing bool `json:"isFollowing" db:"-"`
}

// 🔧 NEW: VideoResponse struct for API responses with proper mapping
type VideoResponse struct {
	ID           string `json:"id"`
	UserID       string `json:"userId"`
	UserName     string `json:"userName"`
	UserImage    string `json:"userImage"`
	VideoURL     string `json:"videoUrl"`
	ThumbnailURL string `json:"thumbnailUrl"`
	Caption      string `json:"caption"`

	// 🔧 CRITICAL: Frontend-compatible field names
	LikesCount    int `json:"likes"`    // Database: likes_count -> JSON: likes
	CommentsCount int `json:"comments"` // Database: comments_count -> JSON: comments
	ViewsCount    int `json:"views"`    // Database: views_count -> JSON: views
	SharesCount   int `json:"shares"`   // Database: shares_count -> JSON: shares

	Tags             StringSlice `json:"tags"`
	IsActive         bool        `json:"isActive"`
	IsFeatured       bool        `json:"isFeatured"`
	IsMultipleImages bool        `json:"isMultipleImages"`
	ImageUrls        StringSlice `json:"imageUrls"`
	CreatedAt        time.Time   `json:"createdAt"`
	UpdatedAt        time.Time   `json:"updatedAt"`

	// Runtime fields
	IsLiked          bool   `json:"isLiked"`
	IsFollowing      bool   `json:"isFollowing"`
	UserProfileImage string `json:"userProfileImage"`
}

// 🔧 NEW: Convert Video to VideoResponse
func (v *Video) ToResponse() *VideoResponse {
	return &VideoResponse{
		ID:               v.ID,
		UserID:           v.UserID,
		UserName:         v.UserName,
		UserImage:        v.UserImage,
		VideoURL:         v.VideoURL,
		ThumbnailURL:     v.ThumbnailURL,
		Caption:          v.Caption,
		LikesCount:       v.LikesCount,
		CommentsCount:    v.CommentsCount,
		ViewsCount:       v.ViewsCount,
		SharesCount:      v.SharesCount,
		Tags:             v.Tags,
		IsActive:         v.IsActive,
		IsFeatured:       v.IsFeatured,
		IsMultipleImages: v.IsMultipleImages,
		ImageUrls:        v.ImageUrls,
		CreatedAt:        v.CreatedAt,
		UpdatedAt:        v.UpdatedAt,
		IsLiked:          v.IsLiked,
		IsFollowing:      v.IsFollowing,
		UserProfileImage: v.UserImage,
	}
}

// Helper methods
func (v *Video) IsImagePost() bool {
	return v.IsMultipleImages && len(v.ImageUrls) > 0
}

func (v *Video) IsVideoPost() bool {
	return !v.IsMultipleImages && v.VideoURL != ""
}

func (v *Video) GetDisplayURL() string {
	if v.IsImagePost() && len(v.ImageUrls) > 0 {
		return v.ImageUrls[0]
	}
	if v.ThumbnailURL != "" {
		return v.ThumbnailURL
	}
	return v.VideoURL
}

func (v *Video) GetMediaCount() int {
	if v.IsImagePost() {
		return len(v.ImageUrls)
	}
	return 1 // Single video
}

func (v *Video) HasContent() bool {
	return v.IsVideoPost() || v.IsImagePost()
}

// Calculate engagement rate
func (v *Video) CalculateEngagementRate() float64 {
	if v.ViewsCount == 0 {
		return 0.0
	}
	totalEngagement := v.LikesCount + v.CommentsCount + v.SharesCount
	return (float64(totalEngagement) / float64(v.ViewsCount)) * 100
}

// Calculate trending score
func (v *Video) CalculateTrendingScore() float64 {
	hoursOld := time.Since(v.CreatedAt).Hours()
	if hoursOld == 0 {
		hoursOld = 1 // Avoid division by zero
	}
	// Weight recent videos higher
	timeDecay := 1.0 / (1.0 + hoursOld/24.0) // Decay over days
	// Engagement score
	engagementScore := float64(v.LikesCount*2 + v.CommentsCount*3 + v.SharesCount*5 + v.ViewsCount)
	return engagementScore * timeDecay
}

// Validation methods
func (v *Video) ValidateForCreation() []string {
	var errors []string

	if v.UserID == "" {
		errors = append(errors, "User ID is required")
	}

	if v.UserName == "" {
		errors = append(errors, "User name is required")
	}

	if v.Caption == "" {
		errors = append(errors, "Caption is required")
	}

	if !v.HasContent() {
		errors = append(errors, "Either video URL or image URLs are required")
	}

	if v.IsImagePost() && len(v.ImageUrls) == 0 {
		errors = append(errors, "Image URLs are required for image posts")
	}

	if v.IsVideoPost() && v.VideoURL == "" {
		errors = append(errors, "Video URL is required for video posts")
	}

	return errors
}

func (v *Video) IsValidForCreation() bool {
	return len(v.ValidateForCreation()) == 0
}

// 🔧 FIXED: Comment model with proper JSON tags for frontend compatibility
type Comment struct {
	ID                  string    `json:"id" db:"id"`
	VideoID             string    `json:"videoId" db:"video_id" binding:"required"`
	AuthorID            string    `json:"authorId" db:"author_id" binding:"required"`
	AuthorName          string    `json:"authorName" db:"author_name" binding:"required"`
	AuthorImage         string    `json:"authorImage" db:"author_image"`
	Content             string    `json:"content" db:"content" binding:"required"`
	LikesCount          int       `json:"likes" db:"likes_count"` // Frontend expects "likes"
	IsReply             bool      `json:"isReply" db:"is_reply"`
	RepliedToCommentID  *string   `json:"repliedToCommentId" db:"replied_to_comment_id"`
	RepliedToAuthorName *string   `json:"repliedToAuthorName" db:"replied_to_author_name"`
	CreatedAt           time.Time `json:"createdAt" db:"created_at"`
	UpdatedAt           time.Time `json:"updatedAt" db:"updated_at"`

	// Runtime fields
	IsLiked   bool `json:"isLiked" db:"-"`
	CanDelete bool `json:"canDelete" db:"-"`
}

// Helper methods for comments
func (c *Comment) IsValidReply() bool {
	return c.IsReply && c.RepliedToCommentID != nil
}

func (c *Comment) ValidateForCreation() []string {
	var errors []string

	if c.VideoID == "" {
		errors = append(errors, "Video ID is required")
	}

	if c.AuthorID == "" {
		errors = append(errors, "Author ID is required")
	}

	if c.AuthorName == "" {
		errors = append(errors, "Author name is required")
	}

	if c.Content == "" {
		errors = append(errors, "Comment content is required")
	}

	if len(c.Content) > 500 {
		errors = append(errors, "Comment content cannot exceed 500 characters")
	}

	if c.IsReply && c.RepliedToCommentID == nil {
		errors = append(errors, "Replied to comment ID is required for replies")
	}

	return errors
}

// VideoLike model for tracking likes
type VideoLike struct {
	ID        string    `json:"id" db:"id"`
	VideoID   string    `json:"videoId" db:"video_id"`
	UserID    string    `json:"userId" db:"user_id"`
	CreatedAt time.Time `json:"createdAt" db:"created_at"`
}

// CommentLike model for tracking comment likes
type CommentLike struct {
	ID        string    `json:"id" db:"id"`
	CommentID string    `json:"commentId" db:"comment_id"`
	UserID    string    `json:"userId" db:"user_id"`
	CreatedAt time.Time `json:"createdAt" db:"created_at"`
}

// UserFollow model for social following
type UserFollow struct {
	ID          string    `json:"id" db:"id"`
	FollowerID  string    `json:"followerId" db:"follower_id"`
	FollowingID string    `json:"followingId" db:"following_id"`
	CreatedAt   time.Time `json:"createdAt" db:"created_at"`
}

// Video creation request models
type CreateVideoRequest struct {
	Caption          string   `json:"caption" binding:"required"`
	VideoURL         string   `json:"videoUrl"`
	ThumbnailURL     string   `json:"thumbnailUrl"`
	Tags             []string `json:"tags"`
	IsMultipleImages bool     `json:"isMultipleImages"`
	ImageUrls        []string `json:"imageUrls"`
}

type CreateCommentRequest struct {
	Content             string  `json:"content" binding:"required"`
	RepliedToCommentID  *string `json:"repliedToCommentId"`
	RepliedToAuthorName *string `json:"repliedToAuthorName"`
}

// 🔧 NEW: VideoCountsSummary for quick count updates
type VideoCountsSummary struct {
	VideoID       string    `json:"videoId" db:"id"`
	ViewsCount    int       `json:"views" db:"views_count"`
	LikesCount    int       `json:"likes" db:"likes_count"`
	CommentsCount int       `json:"comments" db:"comments_count"`
	SharesCount   int       `json:"shares" db:"shares_count"`
	UpdatedAt     time.Time `json:"updatedAt" db:"updated_at"`
}

// Video response models for API
type VideoFeedResponse struct {
	Videos      []VideoResponse `json:"videos"`
	HasMore     bool            `json:"hasMore"`
	LastVideoID string          `json:"lastVideoId"`
	Total       int             `json:"total"`
	Page        int             `json:"page"`
	Limit       int             `json:"limit"`
}

type CommentResponse struct {
	Comment
	Replies []CommentResponse `json:"replies,omitempty"`
}

type VideoStatsResponse struct {
	VideoID        string  `json:"videoId"`
	LikesCount     int     `json:"likes"`
	CommentsCount  int     `json:"comments"`
	ViewsCount     int     `json:"views"`
	SharesCount    int     `json:"shares"`
	EngagementRate float64 `json:"engagementRate"`
	TrendingScore  float64 `json:"trendingScore"`
}

// Video constants
const (
	MaxCaptionLength = 2200               // TikTok-style caption limit
	MaxTagsPerVideo  = 30                 // Maximum hashtags per video
	MaxImagesPerPost = 10                 // Maximum images in a carousel post
	MaxCommentLength = 500                // Maximum comment length
	MaxVideoFileSize = 1024 * 1024 * 1024 // 1GB
	MaxImageFileSize = 50 * 1024 * 1024   // 50MB per image
	MaxVideoDuration = 300                // 5 minutes in seconds
)

// Video search and filtering
type VideoSearchParams struct {
	Query     string   `json:"query"`
	Tags      []string `json:"tags"`
	UserID    string   `json:"userId"`
	Featured  *bool    `json:"featured"`
	MediaType string   `json:"mediaType"` // "video", "image", "all"
	SortBy    string   `json:"sortBy"`    // "latest", "popular", "trending"
	Limit     int      `json:"limit"`
	Offset    int      `json:"offset"`
	LastID    string   `json:"lastId"` // For cursor-based pagination
}

type VideoSortOption string

const (
	SortByLatest   VideoSortOption = "latest"
	SortByPopular  VideoSortOption = "popular"
	SortByTrending VideoSortOption = "trending"
	SortByViews    VideoSortOption = "views"
	SortByLikes    VideoSortOption = "likes"
)

// Video performance metrics
type VideoPerformance struct {
	VideoID        string    `json:"videoId"`
	Title          string    `json:"title"`
	LikesCount     int       `json:"likes"`
	CommentsCount  int       `json:"comments"`
	ViewsCount     int       `json:"views"`
	SharesCount    int       `json:"shares"`
	EngagementRate float64   `json:"engagementRate"`
	CreatedAt      time.Time `json:"createdAt"`
}

func (vp *VideoPerformance) CalculateEngagementRate() {
	if vp.ViewsCount > 0 {
		totalEngagement := vp.LikesCount + vp.CommentsCount + vp.SharesCount
		vp.EngagementRate = (float64(totalEngagement) / float64(vp.ViewsCount)) * 100
	}
}

// Trending calculation helpers
type TrendingScore struct {
	VideoID   string    `json:"videoId"`
	Score     float64   `json:"score"`
	CreatedAt time.Time `json:"createdAt"`
}

func CalculateTrendingScore(video *Video) float64 {
	// Simple trending algorithm based on engagement and recency
	hoursOld := time.Since(video.CreatedAt).Hours()
	if hoursOld == 0 {
		hoursOld = 1 // Avoid division by zero
	}

	// Weight recent videos higher
	timeDecay := 1.0 / (1.0 + hoursOld/24.0) // Decay over days

	// Engagement score
	engagementScore := float64(video.LikesCount*2 + video.CommentsCount*3 + video.SharesCount*5 + video.ViewsCount)

	return engagementScore * timeDecay
}

// 🔧 ADDITIONAL FIX: Helper functions for StringSlice operations
func NewStringSlice(items ...string) StringSlice {
	if len(items) == 0 {
		return StringSlice{}
	}
	return StringSlice(items)
}

func (s StringSlice) Contains(item string) bool {
	for _, v := range s {
		if v == item {
			return true
		}
	}
	return false
}

func (s StringSlice) Add(item string) StringSlice {
	if s.Contains(item) {
		return s
	}
	return append(s, item)
}

func (s StringSlice) Remove(item string) StringSlice {
	result := make(StringSlice, 0, len(s))
	for _, v := range s {
		if v != item {
			result = append(result, v)
		}
	}
	return result
}

func (s StringSlice) IsEmpty() bool {
	return len(s) == 0
}

func (s StringSlice) Length() int {
	return len(s)
}

// 🔧 DEBUG: Helper function to debug StringSlice formatting
func (s StringSlice) DebugString() string {
	return fmt.Sprintf("StringSlice(len=%d, items=%v, pgFormat=%s)",
		len(s), []string(s), s.toPostgreSQLFormat())
}

func (s StringSlice) toPostgreSQLFormat() string {
	value, _ := s.Value()
	if value == nil {
		return "NULL"
	}
	return fmt.Sprintf("%v", value)
}
