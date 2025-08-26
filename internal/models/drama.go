// ===============================
// internal/models/drama.go
// ===============================

package models

import "time"

type Drama struct {
	DramaID           string    `json:"dramaId" db:"drama_id"`
	Title             string    `json:"title" db:"title"`
	Description       string    `json:"description" db:"description"`
	BannerImage       string    `json:"bannerImage" db:"banner_image"`
	TotalEpisodes     int       `json:"totalEpisodes" db:"total_episodes"`
	IsPremium         bool      `json:"isPremium" db:"is_premium"`
	FreeEpisodesCount int       `json:"freeEpisodesCount" db:"free_episodes_count"`
	ViewCount         int       `json:"viewCount" db:"view_count"`
	FavoriteCount     int       `json:"favoriteCount" db:"favorite_count"`
	IsFeatured        bool      `json:"isFeatured" db:"is_featured"`
	IsActive          bool      `json:"isActive" db:"is_active"`
	CreatedBy         string    `json:"createdBy" db:"created_by"`
	CreatedAt         time.Time `json:"createdAt" db:"created_at"`
	UpdatedAt         time.Time `json:"updatedAt" db:"updated_at"`
	PublishedAt       time.Time `json:"publishedAt" db:"published_at"`
}

// Helper methods
func (d *Drama) CanWatchEpisode(episodeNumber int, hasUnlocked bool) bool {
	if !d.IsPremium {
		return true // Free drama
	}
	if episodeNumber <= d.FreeEpisodesCount {
		return true // Free episodes
	}
	return hasUnlocked // Premium episodes require unlock
}

func (d *Drama) IsEpisodeFree(episodeNumber int) bool {
	if !d.IsPremium {
		return true
	}
	return episodeNumber <= d.FreeEpisodesCount
}
