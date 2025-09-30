// ===============================
// internal/models/video.go - UPDATED with Simplified Search Models
// ===============================

package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ===============================
// STRING SLICE TYPE (for PostgreSQL arrays)
// ===============================

type StringSlice []string

func (s StringSlice) Value() (driver.Value, error) {
	if len(s) == 0 {
		return "{}", nil
	}
	return "{" + strings.Join(s, ",") + "}", nil
}

func (s *StringSlice) Scan(value interface{}) error {
	if value == nil {
		*s = []string{}
		return nil
	}

	switch v := value.(type) {
	case []byte:
		str := string(v)
		str = strings.Trim(str, "{}")
		if str == "" {
			*s = []string{}
			return nil
		}
		*s = strings.Split(str, ",")
	case string:
		str := strings.Trim(v, "{}")
		if str == "" {
			*s = []string{}
			return nil
		}
		*s = strings.Split(str, ",")
	default:
		return fmt.Errorf("unsupported type: %T", value)
	}
	return nil
}

// ===============================
// VIDEO MODEL
// ===============================

type Video struct {
	ID               string      `db:"id" json:"id"`
	UserID           string      `db:"user_id" json:"userId"`
	UserName         string      `db:"user_name" json:"userName"`
	UserImage        string      `db:"user_image" json:"userImage"`
	VideoURL         string      `db:"video_url" json:"videoUrl"`
	ThumbnailURL     string      `db:"thumbnail_url" json:"thumbnailUrl"`
	Caption          string      `db:"caption" json:"caption"`
	Price            float64     `db:"price" json:"price"`
	LikesCount       int         `db:"likes_count" json:"likesCount"`
	CommentsCount    int         `db:"comments_count" json:"commentsCount"`
	ViewsCount       int         `db:"views_count" json:"viewsCount"`
	SharesCount      int         `db:"shares_count" json:"sharesCount"`
	Tags             StringSlice `db:"tags" json:"tags"`
	IsActive         bool        `db:"is_active" json:"isActive"`
	IsFeatured       bool        `db:"is_featured" json:"isFeatured"`
	IsVerified       bool        `db:"is_verified" json:"isVerified"`
	IsMultipleImages bool        `db:"is_multiple_images" json:"isMultipleImages"`
	ImageUrls        StringSlice `db:"image_urls" json:"imageUrls"`
	CreatedAt        time.Time   `db:"created_at" json:"createdAt"`
	UpdatedAt        time.Time   `db:"updated_at" json:"updatedAt"`
}

type VideoResponse struct {
	ID               string      `json:"id"`
	UserID           string      `json:"userId"`
	UserName         string      `json:"userName"`
	UserImage        string      `json:"userImage"`
	UserProfileImage string      `json:"userProfileImage"`
	VideoURL         string      `json:"videoUrl"`
	ThumbnailURL     string      `json:"thumbnailUrl"`
	Caption          string      `json:"caption"`
	Price            float64     `json:"price"`
	LikesCount       int         `json:"likesCount"`
	CommentsCount    int         `json:"commentsCount"`
	ViewsCount       int         `json:"viewsCount"`
	SharesCount      int         `json:"sharesCount"`
	Tags             StringSlice `json:"tags"`
	IsActive         bool        `json:"isActive"`
	IsFeatured       bool        `json:"isFeatured"`
	IsVerified       bool        `json:"isVerified"`
	IsMultipleImages bool        `json:"isMultipleImages"`
	ImageUrls        StringSlice `json:"imageUrls"`
	CreatedAt        time.Time   `json:"createdAt"`
	UpdatedAt        time.Time   `json:"updatedAt"`
	IsLiked          bool        `json:"isLiked"`
	IsFollowing      bool        `json:"isFollowing"`
}

type CreateVideoRequest struct {
	VideoURL         string   `json:"videoUrl"`
	ThumbnailURL     string   `json:"thumbnailUrl"`
	Caption          string   `json:"caption" binding:"required"`
	Price            *float64 `json:"price"`
	Tags             []string `json:"tags"`
	IsMultipleImages bool     `json:"isMultipleImages"`
	ImageUrls        []string `json:"imageUrls"`
}

func (v *Video) IsValidForCreation() bool {
	if v.Caption == "" {
		return false
	}
	if len(v.Caption) > 2200 {
		return false
	}
	if !v.IsMultipleImages && v.VideoURL == "" {
		return false
	}
	if v.IsMultipleImages && len(v.ImageUrls) == 0 {
		return false
	}
	return true
}

func (v *Video) ValidateForCreation() []string {
	var errors []string

	if v.Caption == "" {
		errors = append(errors, "caption is required")
	}
	if len(v.Caption) > 2200 {
		errors = append(errors, "caption must be 2200 characters or less")
	}
	if !v.IsMultipleImages && v.VideoURL == "" {
		errors = append(errors, "video URL is required for video posts")
	}
	if v.IsMultipleImages && len(v.ImageUrls) == 0 {
		errors = append(errors, "at least one image URL is required for image posts")
	}

	return errors
}

// ===============================
// VIDEO SEARCH PARAMS
// ===============================

type VideoSearchParams struct {
	Query     string
	UserID    string
	Limit     int
	Offset    int
	SortBy    string
	MediaType string
	Featured  *bool
	Role      *UserRole
}

// ===============================
// VIDEO COUNTS SUMMARY
// ===============================

type VideoCountsSummary struct {
	VideoID       string    `json:"videoId"`
	ViewsCount    int       `json:"viewsCount"`
	LikesCount    int       `json:"likesCount"`
	CommentsCount int       `json:"commentsCount"`
	SharesCount   int       `json:"sharesCount"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

// ===============================
// VIDEO PERFORMANCE
// ===============================

type VideoPerformance struct {
	VideoID        string    `json:"videoId"`
	Title          string    `json:"title"`
	ViewsCount     int       `json:"viewsCount"`
	LikesCount     int       `json:"likesCount"`
	CommentsCount  int       `json:"commentsCount"`
	SharesCount    int       `json:"sharesCount"`
	EngagementRate float64   `json:"engagementRate"`
	CreatedAt      time.Time `json:"createdAt"`
}

func (vp *VideoPerformance) CalculateEngagementRate() {
	if vp.ViewsCount > 0 {
		totalEngagement := float64(vp.LikesCount + vp.CommentsCount + vp.SharesCount)
		vp.EngagementRate = (totalEngagement / float64(vp.ViewsCount)) * 100
	}
}

// ===============================
// 🆕 SIMPLIFIED SEARCH MODELS
// ===============================

// SearchMode - Simple search modes
type SearchMode string

const (
	SearchModeFuzzy    SearchMode = "fuzzy"    // Default: handles typos and variations
	SearchModeExact    SearchMode = "exact"    // Exact phrase matching
	SearchModeFullText SearchMode = "fulltext" // PostgreSQL full-text search
	SearchModeCombined SearchMode = "combined" // Best of all methods
)

// SearchResult - Individual search result with video and relevance
type SearchResult struct {
	Video     *VideoResponse `json:"video"`
	Relevance float64        `json:"relevance"` // 0.0 to 1.0
	MatchType string         `json:"matchType"` // "username", "caption", "tag"
}

// SearchResponse - Complete search response
type SearchResponse struct {
	Results     []SearchResult `json:"results"`
	Total       int            `json:"total"`
	Query       string         `json:"query"`
	SearchMode  SearchMode     `json:"searchMode"`
	TimeTaken   int64          `json:"timeTaken"` // milliseconds
	Page        int            `json:"page"`
	HasMore     bool           `json:"hasMore"`
	Suggestions []string       `json:"suggestions,omitempty"` // Empty suggestions for no-suggestion system
}

// SearchFilters - Simplified filters (removed complex filters)
type SearchFilters struct {
	MediaType string `json:"mediaType"` // "video", "image", "all"
	TimeRange string `json:"timeRange"` // "day", "week", "month", "all"
	SortBy    string `json:"sortBy"`    // "relevance", "latest", "popular"
	MinLikes  int    `json:"minLikes"`
}

// ===============================
// 🆕 SEARCH HISTORY MODELS
// ===============================

// SearchHistoryItem - User's search history item
type SearchHistoryItem struct {
	Query          string    `json:"query" db:"query"`
	SearchCount    int       `json:"searchCount" db:"search_count"`
	LastSearchedAt time.Time `json:"lastSearchedAt" db:"last_searched_at"`
	CreatedAt      time.Time `json:"createdAt" db:"created_at"`
}

// Popular search term for trending/discovery
type PopularSearchTerm struct {
	Term      string `json:"term" db:"term"`
	Frequency int    `json:"frequency" db:"frequency"`
}

// SearchHistoryResponse - Response for search history endpoint
type SearchHistoryResponse struct {
	History []SearchHistoryItem `json:"history"`
	Total   int                 `json:"total"`
}

// PopularTermsResponse - Response for popular terms endpoint
type PopularTermsResponse struct {
	Terms []PopularSearchTerm `json:"terms"`
	Total int                 `json:"total"`
}

// ===============================
// SEARCH REQUEST
// ===============================

type SearchRequest struct {
	Query              string `json:"query" binding:"required"`
	FilterUsernameOnly bool   `json:"filterUsernameOnly"` // Optional filter to search username only
	Limit              int    `json:"limit"`
	Offset             int    `json:"offset"`
}

// ===============================
// COMMENT MODEL
// ===============================

type Comment struct {
	ID                  string    `db:"id" json:"id"`
	VideoID             string    `db:"video_id" json:"videoId"`
	AuthorID            string    `db:"author_id" json:"authorId"`
	AuthorName          string    `db:"author_name" json:"authorName"`
	AuthorImage         string    `db:"author_image" json:"authorImage"`
	Content             string    `db:"content" json:"content"`
	LikesCount          int       `db:"likes_count" json:"likesCount"`
	IsReply             bool      `db:"is_reply" json:"isReply"`
	RepliedToCommentID  *string   `db:"replied_to_comment_id" json:"repliedToCommentId,omitempty"`
	RepliedToAuthorName *string   `db:"replied_to_author_name" json:"repliedToAuthorName,omitempty"`
	CreatedAt           time.Time `db:"created_at" json:"createdAt"`
	UpdatedAt           time.Time `db:"updated_at" json:"updatedAt"`
}

type CreateCommentRequest struct {
	Content             string  `json:"content" binding:"required"`
	RepliedToCommentID  *string `json:"repliedToCommentId,omitempty"`
	RepliedToAuthorName *string `json:"repliedToAuthorName,omitempty"`
}

func (c *Comment) ValidateForCreation() []string {
	var errors []string

	content := strings.TrimSpace(c.Content)
	if content == "" {
		errors = append(errors, "content is required")
	}
	if len(content) > 500 {
		errors = append(errors, "content must be 500 characters or less")
	}
	if len(content) < 1 {
		errors = append(errors, "content must be at least 1 character")
	}

	return errors
}

// ===============================
// HELPER FUNCTIONS
// ===============================

func (sr *SearchResult) GetDisplayText() string {
	switch sr.MatchType {
	case "username":
		return fmt.Sprintf("@%s", sr.Video.UserName)
	case "caption":
		return sr.Video.Caption
	case "tag":
		if len(sr.Video.Tags) > 0 {
			return fmt.Sprintf("#%s", sr.Video.Tags[0])
		}
		return ""
	default:
		return ""
	}
}

func (sr *SearchResult) GetMatchTypeDisplay() string {
	switch sr.MatchType {
	case "username":
		return "Creator"
	case "caption":
		return "Caption"
	case "tag":
		return "Tag"
	default:
		return "Match"
	}
}

func (sr *SearchResult) IsHighRelevance() bool {
	return sr.Relevance >= 0.8
}

func (sr *SearchResult) IsMediumRelevance() bool {
	return sr.Relevance >= 0.5 && sr.Relevance < 0.8
}

func (sr *SearchResult) IsLowRelevance() bool {
	return sr.Relevance < 0.5
}

func (sf *SearchFilters) IsDefault() bool {
	return sf.MediaType == "all" &&
		sf.TimeRange == "all" &&
		sf.SortBy == "relevance" &&
		sf.MinLikes == 0
}

func (sf *SearchFilters) HasActiveFilters() bool {
	return !sf.IsDefault()
}

// Time ago helper for videos
func (v *VideoResponse) TimeAgo() string {
	return formatTimeAgo(v.CreatedAt)
}

// Time ago helper for comments
func (c *Comment) TimeAgo() string {
	return formatTimeAgo(c.CreatedAt)
}

// Time ago helper for search history
func (shi *SearchHistoryItem) TimeAgo() string {
	return formatTimeAgo(shi.LastSearchedAt)
}

func formatTimeAgo(t time.Time) string {
	now := time.Now()
	diff := now.Sub(t)

	seconds := int(diff.Seconds())
	minutes := seconds / 60
	hours := minutes / 60
	days := hours / 24
	weeks := days / 7
	months := days / 30
	years := days / 365

	switch {
	case seconds < 60:
		return "just now"
	case minutes == 1:
		return "1 minute ago"
	case minutes < 60:
		return fmt.Sprintf("%d minutes ago", minutes)
	case hours == 1:
		return "1 hour ago"
	case hours < 24:
		return fmt.Sprintf("%d hours ago", hours)
	case days == 1:
		return "1 day ago"
	case days < 7:
		return fmt.Sprintf("%d days ago", days)
	case weeks == 1:
		return "1 week ago"
	case weeks < 4:
		return fmt.Sprintf("%d weeks ago", weeks)
	case months == 1:
		return "1 month ago"
	case months < 12:
		return fmt.Sprintf("%d months ago", months)
	case years == 1:
		return "1 year ago"
	default:
		return fmt.Sprintf("%d years ago", years)
	}
}

// Format count helper
func FormatCount(count int) string {
	switch {
	case count < 1000:
		return fmt.Sprintf("%d", count)
	case count < 1000000:
		return fmt.Sprintf("%.1fK", float64(count)/1000)
	default:
		return fmt.Sprintf("%.1fM", float64(count)/1000000)
	}
}

// ===============================
// JSON MARSHALING HELPERS
// ===============================

func (v *VideoResponse) MarshalJSON() ([]byte, error) {
	type Alias VideoResponse
	return json.Marshal(&struct {
		*Alias
		TimeAgo      string `json:"timeAgo"`
		LikesDisplay string `json:"likesDisplay"`
		ViewsDisplay string `json:"viewsDisplay"`
	}{
		Alias:        (*Alias)(v),
		TimeAgo:      v.TimeAgo(),
		LikesDisplay: FormatCount(v.LikesCount),
		ViewsDisplay: FormatCount(v.ViewsCount),
	})
}

func (c *Comment) MarshalJSON() ([]byte, error) {
	type Alias Comment
	return json.Marshal(&struct {
		*Alias
		TimeAgo      string `json:"timeAgo"`
		LikesDisplay string `json:"likesDisplay"`
	}{
		Alias:        (*Alias)(c),
		TimeAgo:      c.TimeAgo(),
		LikesDisplay: FormatCount(c.LikesCount),
	})
}

// ===============================
// SEARCH CONSTANTS
// ===============================

const (
	MinSearchQueryLength = 2
	MaxSearchQueryLength = 100
	MaxSearchResults     = 50
	DefaultSearchLimit   = 20
	MaxSearchHistory     = 50
	SearchDebounceMs     = 300
)

// ===============================
// VALIDATION HELPERS
// ===============================

func ValidateSearchQuery(query string) (string, error) {
	cleaned := strings.TrimSpace(query)

	if len(cleaned) < MinSearchQueryLength {
		return "", fmt.Errorf("search query must be at least %d characters", MinSearchQueryLength)
	}

	if len(cleaned) > MaxSearchQueryLength {
		return "", fmt.Errorf("search query cannot exceed %d characters", MaxSearchQueryLength)
	}

	return cleaned, nil
}

func SanitizeSearchQuery(query string) string {
	// Remove excessive whitespace
	cleaned := strings.TrimSpace(query)
	cleaned = strings.Join(strings.Fields(cleaned), " ")

	// Remove potentially problematic characters
	cleaned = strings.Map(func(r rune) rune {
		if r == '<' || r == '>' || r == '{' || r == '}' || r == '[' || r == ']' || r == '\\' || r == '|' || r == '`' || r == '~' {
			return -1
		}
		return r
	}, cleaned)

	// Limit length
	if len(cleaned) > MaxSearchQueryLength {
		cleaned = cleaned[:MaxSearchQueryLength]
	}

	return cleaned
}
