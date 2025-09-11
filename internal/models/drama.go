// ===============================
// internal/models/drama.go - Drama Models for Video Social Media App
// ===============================

package models

import (
	//"database/sql/driver"
	//"encoding/json"
	"fmt"
	"time"
)

// Constants for drama specifications (same as original but adapted)
const (
	MaxEpisodeDurationSeconds = 120      // 2 minutes
	MaxEpisodeFileSizeBytes   = 52428800 // 50MB
	MaxEpisodesPerDrama       = 100      // Unchanged
	//DramaUnlockCost           = 99       // Coins needed to unlock premium drama
)

// Drama model with embedded episode videos and unlock tracking
type Drama struct {
	DramaID           string      `json:"dramaId" db:"drama_id"`
	Title             string      `json:"title" db:"title" binding:"required"`
	Description       string      `json:"description" db:"description" binding:"required"`
	BannerImage       string      `json:"bannerImage" db:"banner_image"`
	EpisodeVideos     StringSlice `json:"episodeVideos" db:"episode_videos"` // Array of video URLs
	IsPremium         bool        `json:"isPremium" db:"is_premium"`
	FreeEpisodesCount int         `json:"freeEpisodesCount" db:"free_episodes_count"`
	ViewCount         int         `json:"viewCount" db:"view_count"`
	FavoriteCount     int         `json:"favoriteCount" db:"favorite_count"`
	UnlockCount       int         `json:"unlockCount" db:"unlock_count"` // Track successful unlocks
	IsFeatured        bool        `json:"isFeatured" db:"is_featured"`
	IsActive          bool        `json:"isActive" db:"is_active"`
	CreatedBy         string      `json:"createdBy" db:"created_by"`
	CreatedAt         time.Time   `json:"createdAt" db:"created_at"`
	UpdatedAt         time.Time   `json:"updatedAt" db:"updated_at"`

	// Runtime fields (not stored in DB)
	IsUnlocked bool `json:"isUnlocked" db:"-"` // Set based on user context
	IsFavorite bool `json:"isFavorite" db:"-"` // Set based on user context
}

// Helper methods
func (d *Drama) GetTotalEpisodes() int {
	return len(d.EpisodeVideos)
}

func (d *Drama) HasEpisodes() bool {
	return len(d.EpisodeVideos) > 0
}

func (d *Drama) IsFree() bool {
	return !d.IsPremium
}

// Get total revenue from unlocks
func (d *Drama) GetTotalRevenue() int {
	if !d.IsPremium {
		return 0
	}
	return d.UnlockCount * DramaUnlockCost
}

// Calculate conversion rate from views to unlocks
func (d *Drama) GetConversionRate() float64 {
	if d.ViewCount == 0 {
		return 0.0
	}
	return (float64(d.UnlockCount) / float64(d.ViewCount)) * 100.0
}

// Check if drama is profitable (has unlocks)
func (d *Drama) IsProfitable() bool {
	return d.IsPremium && d.UnlockCount > 0
}

// Get video URL for specific episode (1-indexed)
func (d *Drama) GetEpisodeVideo(episodeNumber int) string {
	if episodeNumber < 1 || episodeNumber > len(d.EpisodeVideos) {
		return ""
	}
	return d.EpisodeVideos[episodeNumber-1] // Convert to 0-indexed
}

// Check if user can watch specific episode
func (d *Drama) CanWatchEpisode(episodeNumber int, hasUnlocked bool) bool {
	if episodeNumber < 1 || episodeNumber > d.GetTotalEpisodes() {
		return false
	}
	if !d.IsPremium {
		return true // All episodes free
	}
	if episodeNumber <= d.FreeEpisodesCount {
		return true // Free episodes
	}
	return hasUnlocked // Premium episodes require unlock
}

// Check if specific episode is free
func (d *Drama) IsEpisodeFree(episodeNumber int) bool {
	if !d.IsPremium {
		return true
	}
	return episodeNumber <= d.FreeEpisodesCount
}

// Get episode title for display
func (d *Drama) GetEpisodeTitle(episodeNumber int) string {
	return fmt.Sprintf("Episode %d", episodeNumber)
}

// Get premium info for display
func (d *Drama) GetPremiumInfo() string {
	if !d.IsPremium {
		return "Free Drama - All episodes included"
	}
	if d.FreeEpisodesCount == 0 {
		return fmt.Sprintf("Premium Drama - Unlock for %d coins to watch all episodes", DramaUnlockCost)
	}
	totalEpisodes := d.GetTotalEpisodes()
	if d.FreeEpisodesCount >= totalEpisodes {
		return "Free Drama - All episodes included"
	}
	return fmt.Sprintf("First %d episodes free, unlock remaining %d episodes for %d coins",
		d.FreeEpisodesCount, totalEpisodes-d.FreeEpisodesCount, DramaUnlockCost)
}

// Get unlock statistics
func (d *Drama) GetUnlockStats() map[string]interface{} {
	return map[string]interface{}{
		"totalUnlocks":   d.UnlockCount,
		"totalRevenue":   d.GetTotalRevenue(),
		"conversionRate": d.GetConversionRate(),
		"isProfitable":   d.IsProfitable(),
		"unlockCost":     DramaUnlockCost,
	}
}

// Validate drama for creation
func (d *Drama) ValidateForCreation() []string {
	var errors []string

	if d.Title == "" {
		errors = append(errors, "Title is required")
	}

	if d.Description == "" {
		errors = append(errors, "Description is required")
	}

	if len(d.EpisodeVideos) == 0 {
		errors = append(errors, "At least one episode is required")
	}

	if len(d.EpisodeVideos) > MaxEpisodesPerDrama {
		errors = append(errors, fmt.Sprintf("Maximum %d episodes allowed", MaxEpisodesPerDrama))
	}

	if d.CreatedBy == "" {
		errors = append(errors, "Creator is required")
	}

	if d.IsPremium && d.FreeEpisodesCount > len(d.EpisodeVideos) {
		errors = append(errors, "Free episodes cannot exceed total episodes")
	}

	// Validate episode URLs are not empty
	for i, videoURL := range d.EpisodeVideos {
		if videoURL == "" {
			errors = append(errors, fmt.Sprintf("Episode %d video URL is required", i+1))
		}
	}

	return errors
}

// Check if drama is valid for creation
func (d *Drama) IsValidForCreation() bool {
	return len(d.ValidateForCreation()) == 0
}

// Episode convenience struct for frontend compatibility
type Episode struct {
	Number       int    `json:"number"`
	VideoURL     string `json:"videoUrl"`
	DramaID      string `json:"dramaId"`
	DramaTitle   string `json:"dramaTitle"`
	IsFree       bool   `json:"isFree"`       // Indicate if episode is free
	IsWatchable  bool   `json:"isWatchable"`  // Indicate if user can watch
	ThumbnailURL string `json:"thumbnailUrl"` // Optional thumbnail for episode
}

// Helper methods for Episode
func (e *Episode) GetTitle() string {
	return fmt.Sprintf("Episode %d", e.Number)
}

func (e *Episode) GetDisplayTitle() string {
	return fmt.Sprintf("%s - Episode %d", e.DramaTitle, e.Number)
}

func (e *Episode) IsValid() bool {
	return e.VideoURL != "" && e.Number > 0
}

// Get episode status for display
func (e *Episode) GetStatus() string {
	if !e.IsValid() {
		return "Invalid"
	}
	if e.IsFree {
		return "Free"
	}
	if e.IsWatchable {
		return "Unlocked"
	}
	return "Premium - Unlock Required"
}

// Create episode from drama and episode number with context
func (d *Drama) GetEpisode(episodeNumber int) *Episode {
	if episodeNumber < 1 || episodeNumber > len(d.EpisodeVideos) {
		return nil
	}

	return &Episode{
		Number:      episodeNumber,
		VideoURL:    d.EpisodeVideos[episodeNumber-1],
		DramaID:     d.DramaID,
		DramaTitle:  d.Title,
		IsFree:      d.IsEpisodeFree(episodeNumber),
		IsWatchable: true, // Default - should be set based on user unlock status
	}
}

// Get episode with user context (whether they can watch)
func (d *Drama) GetEpisodeWithUserContext(episodeNumber int, hasUnlocked bool) *Episode {
	episode := d.GetEpisode(episodeNumber)
	if episode == nil {
		return nil
	}

	episode.IsWatchable = d.CanWatchEpisode(episodeNumber, hasUnlocked)
	return episode
}

// Get all episodes as Episode structs
func (d *Drama) GetAllEpisodes() []Episode {
	var episodes []Episode
	for i, videoURL := range d.EpisodeVideos {
		episodes = append(episodes, Episode{
			Number:      i + 1,
			VideoURL:    videoURL,
			DramaID:     d.DramaID,
			DramaTitle:  d.Title,
			IsFree:      d.IsEpisodeFree(i + 1),
			IsWatchable: true, // Default - should be set based on user context
		})
	}
	return episodes
}

// Get all episodes with user context
func (d *Drama) GetAllEpisodesWithUserContext(hasUnlocked bool) []Episode {
	var episodes []Episode
	for i, videoURL := range d.EpisodeVideos {
		episodeNumber := i + 1
		episodes = append(episodes, Episode{
			Number:      episodeNumber,
			VideoURL:    videoURL,
			DramaID:     d.DramaID,
			DramaTitle:  d.Title,
			IsFree:      d.IsEpisodeFree(episodeNumber),
			IsWatchable: d.CanWatchEpisode(episodeNumber, hasUnlocked),
		})
	}
	return episodes
}

// UserDramaProgress model for tracking user progress through dramas
type UserDramaProgress struct {
	ID             string    `json:"id" db:"id"`
	UserID         string    `json:"userId" db:"user_id"`
	DramaID        string    `json:"dramaId" db:"drama_id"`
	CurrentEpisode int       `json:"currentEpisode" db:"current_episode"`
	LastWatchedAt  time.Time `json:"lastWatchedAt" db:"last_watched_at"`
	Completed      bool      `json:"completed" db:"completed"`
	CreatedAt      time.Time `json:"createdAt" db:"created_at"`
	UpdatedAt      time.Time `json:"updatedAt" db:"updated_at"`
}

// DramaAnalytics model for tracking daily performance
type DramaAnalytics struct {
	ID           string    `json:"id" db:"id"`
	DramaID      string    `json:"dramaId" db:"drama_id"`
	Date         time.Time `json:"date" db:"date"`
	ViewsCount   int       `json:"viewsCount" db:"views_count"`
	LikesCount   int       `json:"likesCount" db:"likes_count"`
	UnlocksCount int       `json:"unlocksCount" db:"unlocks_count"`
	Revenue      int       `json:"revenue" db:"revenue"`
	CreatedAt    time.Time `json:"createdAt" db:"created_at"`
}

// Drama performance summary
type DramaPerformance struct {
	DramaID        string    `json:"dramaId"`
	Title          string    `json:"title"`
	TotalEpisodes  int       `json:"totalEpisodes"`
	ViewCount      int       `json:"viewCount"`
	FavoriteCount  int       `json:"favoriteCount"`
	UnlockCount    int       `json:"unlockCount"`
	Revenue        int       `json:"revenue"`
	ConversionRate float64   `json:"conversionRate"`
	IsPremium      bool      `json:"isPremium"`
	CreatedAt      time.Time `json:"createdAt"`
}

// Get performance summary
func (d *Drama) GetPerformanceSummary() DramaPerformance {
	return DramaPerformance{
		DramaID:        d.DramaID,
		Title:          d.Title,
		TotalEpisodes:  d.GetTotalEpisodes(),
		ViewCount:      d.ViewCount,
		FavoriteCount:  d.FavoriteCount,
		UnlockCount:    d.UnlockCount,
		Revenue:        d.GetTotalRevenue(),
		ConversionRate: d.GetConversionRate(),
		IsPremium:      d.IsPremium,
		CreatedAt:      d.CreatedAt,
	}
}

// Drama creation request models
type CreateDramaRequest struct {
	Title             string   `json:"title" binding:"required"`
	Description       string   `json:"description" binding:"required"`
	BannerImage       string   `json:"bannerImage"`
	EpisodeVideos     []string `json:"episodeVideos" binding:"required,min=1,max=100"`
	IsPremium         bool     `json:"isPremium"`
	FreeEpisodesCount int      `json:"freeEpisodesCount"`
	IsFeatured        bool     `json:"isFeatured"`
	IsActive          bool     `json:"isActive"`
}

type UpdateDramaRequest struct {
	Title             string   `json:"title"`
	Description       string   `json:"description"`
	BannerImage       string   `json:"bannerImage"`
	EpisodeVideos     []string `json:"episodeVideos"`
	IsPremium         bool     `json:"isPremium"`
	FreeEpisodesCount int      `json:"freeEpisodesCount"`
	IsFeatured        bool     `json:"isFeatured"`
	IsActive          bool     `json:"isActive"`
}

// Drama search parameters
type DramaSearchParams struct {
	Query    string `json:"query"`
	UserID   string `json:"userId"`   // Filter by creator
	Premium  *bool  `json:"premium"`  // Filter by premium status
	Featured *bool  `json:"featured"` // Filter by featured status
	SortBy   string `json:"sortBy"`   // "latest", "popular", "trending"
	Limit    int    `json:"limit"`
	Offset   int    `json:"offset"`
}

// Drama response models
type DramaResponse struct {
	Drama
	CreatorName     string `json:"creatorName"`
	CreatorVerified bool   `json:"creatorVerified"`
	CreatorImage    string `json:"creatorImage"`
}

type DramaListResponse struct {
	Dramas  []DramaResponse `json:"dramas"`
	HasMore bool            `json:"hasMore"`
	Total   int             `json:"total"`
}

// Drama unlock request
type UnlockDramaRequest struct {
	DramaID string `json:"dramaId" binding:"required"`
}

// Drama unlock response
type UnlockDramaResponse struct {
	Success    bool   `json:"success"`
	NewBalance int    `json:"newBalance"`
	Message    string `json:"message"`
}

// Validate episode specifications
func ValidateEpisodeSpecs(durationSeconds int, fileSizeBytes int64) []string {
	var errors []string

	if durationSeconds > MaxEpisodeDurationSeconds {
		errors = append(errors, fmt.Sprintf("Episode duration cannot exceed %d seconds (2 minutes)", MaxEpisodeDurationSeconds))
	}

	if fileSizeBytes > MaxEpisodeFileSizeBytes {
		errors = append(errors, fmt.Sprintf("Episode file size cannot exceed %d bytes (50MB)", MaxEpisodeFileSizeBytes))
	}

	return errors
}

// Drama constants
const (
	DramaSortByLatest   = "latest"
	DramaSortByPopular  = "popular"
	DramaSortByTrending = "trending"
	DramaSortByRevenue  = "revenue"
)
