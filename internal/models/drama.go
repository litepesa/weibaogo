// ===============================
// internal/models/drama.go - UNIFIED ARCHITECTURE
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

// Drama model with embedded episode videos
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
	IsFeatured        bool        `json:"isFeatured" db:"is_featured"`
	IsActive          bool        `json:"isActive" db:"is_active"`
	CreatedBy         string      `json:"createdBy" db:"created_by"`
	CreatedAt         time.Time   `json:"createdAt" db:"created_at"`
	UpdatedAt         time.Time   `json:"updatedAt" db:"updated_at"`
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
		return "Premium Drama - Unlock required for all episodes"
	}
	totalEpisodes := d.GetTotalEpisodes()
	if d.FreeEpisodesCount >= totalEpisodes {
		return "Free Drama - All episodes included"
	}
	return fmt.Sprintf("First %d episodes free, unlock for remaining %d",
		d.FreeEpisodesCount, totalEpisodes-d.FreeEpisodesCount)
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

	if len(d.EpisodeVideos) > 100 {
		errors = append(errors, "Maximum 100 episodes allowed")
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
	Number     int    `json:"number"`
	VideoURL   string `json:"videoUrl"`
	DramaID    string `json:"dramaId"`
	DramaTitle string `json:"dramaTitle"`
}

// Helper methods for Episode
func (e *Episode) GetTitle() string {
	return fmt.Sprintf("Episode %d", e.Number)
}

func (e *Episode) GetDisplayTitle() string {
	return fmt.Sprintf("%s - Episode %d", e.DramaTitle, e.Number)
}

func (e *Episode) IsWatchable() bool {
	return e.VideoURL != ""
}

// Create episode from drama and episode number
func (d *Drama) GetEpisode(episodeNumber int) *Episode {
	if episodeNumber < 1 || episodeNumber > len(d.EpisodeVideos) {
		return nil
	}

	return &Episode{
		Number:     episodeNumber,
		VideoURL:   d.EpisodeVideos[episodeNumber-1],
		DramaID:    d.DramaID,
		DramaTitle: d.Title,
	}
}

// Get all episodes as Episode structs
func (d *Drama) GetAllEpisodes() []Episode {
	var episodes []Episode
	for i, videoURL := range d.EpisodeVideos {
		episodes = append(episodes, Episode{
			Number:     i + 1,
			VideoURL:   videoURL,
			DramaID:    d.DramaID,
			DramaTitle: d.Title,
		})
	}
	return episodes
}
