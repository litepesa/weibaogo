// ===============================
// internal/models/episode.go
// ===============================

package models

import (
	"fmt"
	"time"
)

type Episode struct {
	EpisodeID        string    `json:"episodeId" db:"episode_id"`
	DramaID          string    `json:"dramaId" db:"drama_id"`
	EpisodeNumber    int       `json:"episodeNumber" db:"episode_number"`
	EpisodeTitle     string    `json:"episodeTitle" db:"episode_title"`
	ThumbnailURL     string    `json:"thumbnailUrl" db:"thumbnail_url"`
	VideoURL         string    `json:"videoUrl" db:"video_url"`
	VideoDuration    int       `json:"videoDuration" db:"video_duration"`
	EpisodeViewCount int       `json:"episodeViewCount" db:"episode_view_count"`
	UploadedBy       string    `json:"uploadedBy" db:"uploaded_by"`
	CreatedAt        time.Time `json:"createdAt" db:"created_at"`
	UpdatedAt        time.Time `json:"updatedAt" db:"updated_at"`
	ReleasedAt       time.Time `json:"releasedAt" db:"released_at"`
}

// Helper methods
func (e *Episode) GetDisplayTitle() string {
	if e.EpisodeTitle != "" {
		return e.EpisodeTitle
	}
	return fmt.Sprintf("Episode %d", e.EpisodeNumber)
}

func (e *Episode) IsWatchable() bool {
	return e.VideoURL != ""
}
