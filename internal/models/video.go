// ===============================
// internal/models/video.go - Video Social Media Model
// ===============================

package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

// StringSlice represents a slice of strings that can be stored in PostgreSQL as JSON
type StringSlice []string

// Value implements driver.Valuer for database storage
func (s StringSlice) Value() (driver.Value, error) {
	if s == nil {
		return nil, nil
	}
	return json.Marshal(s)
}

// Scan implements sql.Scanner for database retrieval
func (s *StringSlice) Scan(value interface{}) error {
	if value == nil {
		*s = nil
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("cannot scan %T into StringSlice", value)
	}

	return json.Unmarshal(bytes, s)
}

// Video model for social media content
type Video struct {
	ID               string      `json:"id" db:"id"`
	UserID           string      `json:"userId" db:"user_id" binding:"required"`
	UserName         string      `json:"userName" db:"user_name" binding:"required"`
	UserImage        string      `json:"userImage" db:"user_image"`
	VideoURL         string      `json:"videoUrl" db:"video_url"`
	ThumbnailURL     string      `json:"thumbnailUrl" db:"thumbnail_url"`
	Caption          string      `json:"caption" db:"caption"`
	LikesCount       int         `json:"likesCount" db:"likes_count"`
	CommentsCount    int         `json:"commentsCount" db:"comments_count"`
	ViewsCount       int         `json:"viewsCount" db:"views_count"`
	SharesCount      int         `json:"sharesCount" db:"shares_count"`
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

// Comment model for video comments
type Comment struct {
	ID                  string    `json:"id" db:"id"`
	VideoID             string    `json:"videoId" db:"video_id" binding:"required"`
	AuthorID            string    `json:"authorId" db:"author_id" binding:"required"`
	AuthorName          string    `json:"authorName" db:"author_name" binding:"required"`
	AuthorImage         string    `json:"authorImage" db:"author_image"`
	Content             string    `json:"content" db:"content" binding:"required"`
	LikesCount          int       `json:"likesCount" db:"likes_count"`
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

// Video response models for API
type VideoResponse struct {
	Video
	UserProfileImage string `json:"userProfileImage"`
	IsFollowing      bool   `json:"isFollowing"`
}

type VideoFeedResponse struct {
	Videos      []VideoResponse `json:"videos"`
	HasMore     bool            `json:"hasMore"`
	LastVideoID string          `json:"lastVideoId"`
	Total       int             `json:"total"`
}

type CommentResponse struct {
	Comment
	Replies []CommentResponse `json:"replies,omitempty"`
}

type VideoStatsResponse struct {
	VideoID       string `json:"videoId"`
	LikesCount    int    `json:"likesCount"`
	CommentsCount int    `json:"commentsCount"`
	ViewsCount    int    `json:"viewsCount"`
	SharesCount   int    `json:"sharesCount"`
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
	LikesCount     int       `json:"likesCount"`
	CommentsCount  int       `json:"commentsCount"`
	ViewsCount     int       `json:"viewsCount"`
	SharesCount    int       `json:"sharesCount"`
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
