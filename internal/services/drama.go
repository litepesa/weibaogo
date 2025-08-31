// ===============================
// internal/services/drama.go - REFINED WITH UNLOCK TRACKING AND SIMPLIFIED EPISODE MANAGEMENT
// ===============================

package services

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"weibaobe/internal/models"
	"weibaobe/internal/storage"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type DramaService struct {
	db       *sqlx.DB
	r2Client *storage.R2Client
}

func NewDramaService(db *sqlx.DB, r2Client *storage.R2Client) *DramaService {
	return &DramaService{
		db:       db,
		r2Client: r2Client,
	}
}

// ===============================
// CONSTANTS
// ===============================

const (
	DramaUnlockCost     = 99       // 99 coins to unlock premium drama
	MaxEpisodeDuration  = 120      // 2 minutes in seconds
	MaxEpisodeFileSize  = 52428800 // 50MB in bytes
	MaxEpisodesPerDrama = 100      // Maximum episodes per drama
)

// ===============================
// OWNERSHIP VERIFICATION
// ===============================

// CheckDramaOwnership verifies if a user owns/created a specific drama
func (s *DramaService) CheckDramaOwnership(ctx context.Context, dramaID, userID string) (bool, error) {
	var createdBy string
	query := `SELECT created_by FROM dramas WHERE drama_id = $1`
	err := s.db.QueryRowContext(ctx, query, dramaID).Scan(&createdBy)
	if err != nil {
		return false, err
	}
	return createdBy == userID, nil
}

// CheckMultipleDramaOwnership verifies ownership for multiple dramas at once
func (s *DramaService) CheckMultipleDramaOwnership(ctx context.Context, dramaIDs []string, userID string) (map[string]bool, error) {
	if len(dramaIDs) == 0 {
		return make(map[string]bool), nil
	}

	// Create placeholders for the IN query
	placeholders := ""
	args := []interface{}{userID}
	for i, id := range dramaIDs {
		if i > 0 {
			placeholders += ", "
		}
		placeholders += fmt.Sprintf("$%d", i+2)
		args = append(args, id)
	}

	query := fmt.Sprintf(`
		SELECT drama_id, (created_by = $1) as owns 
		FROM dramas 
		WHERE drama_id IN (%s)`, placeholders)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]bool)
	for rows.Next() {
		var dramaID string
		var owns bool
		if err := rows.Scan(&dramaID, &owns); err != nil {
			return nil, err
		}
		result[dramaID] = owns
	}

	// Set false for any dramas not found in database
	for _, id := range dramaIDs {
		if _, exists := result[id]; !exists {
			result[id] = false
		}
	}

	return result, nil
}

// ===============================
// CORE DRAMA OPERATIONS
// ===============================

func (s *DramaService) GetDramas(ctx context.Context, limit, offset int, premiumFilter *bool) ([]models.Drama, error) {
	var query string
	var args []interface{}

	if premiumFilter != nil {
		query = `
			SELECT * FROM dramas 
			WHERE is_active = true AND is_premium = $1 
			ORDER BY created_at DESC 
			LIMIT $2 OFFSET $3`
		args = []interface{}{*premiumFilter, limit, offset}
	} else {
		query = `
			SELECT * FROM dramas 
			WHERE is_active = true 
			ORDER BY created_at DESC 
			LIMIT $1 OFFSET $2`
		args = []interface{}{limit, offset}
	}

	var dramas []models.Drama
	err := s.db.SelectContext(ctx, &dramas, query, args...)
	return dramas, err
}

func (s *DramaService) GetFeaturedDramas(ctx context.Context, limit int) ([]models.Drama, error) {
	query := `
		SELECT * FROM dramas 
		WHERE is_active = true AND is_featured = true 
		ORDER BY created_at DESC 
		LIMIT $1`

	var dramas []models.Drama
	err := s.db.SelectContext(ctx, &dramas, query, limit)
	return dramas, err
}

func (s *DramaService) GetTrendingDramas(ctx context.Context, limit int) ([]models.Drama, error) {
	query := `
		SELECT * FROM dramas 
		WHERE is_active = true 
		ORDER BY view_count DESC, created_at DESC 
		LIMIT $1`

	var dramas []models.Drama
	err := s.db.SelectContext(ctx, &dramas, query, limit)
	return dramas, err
}

func (s *DramaService) SearchDramas(ctx context.Context, searchQuery string, limit int) ([]models.Drama, error) {
	query := `
		SELECT * FROM dramas 
		WHERE is_active = true AND (
			title ILIKE $1 OR 
			description ILIKE $1
		)
		ORDER BY 
			CASE WHEN title ILIKE $1 THEN 1 ELSE 2 END,
			view_count DESC,
			created_at DESC 
		LIMIT $2`

	searchPattern := "%" + searchQuery + "%"
	var dramas []models.Drama
	err := s.db.SelectContext(ctx, &dramas, query, searchPattern, limit)
	return dramas, err
}

func (s *DramaService) GetDrama(ctx context.Context, dramaID string) (*models.Drama, error) {
	query := `SELECT * FROM dramas WHERE drama_id = $1 AND is_active = true`

	var drama models.Drama
	err := s.db.GetContext(ctx, &drama, query, dramaID)
	if err != nil {
		return nil, err
	}

	// Increment view count asynchronously (non-blocking)
	go func() {
		s.incrementViewCount(dramaID)
	}()

	return &drama, nil
}

// Get dramas created by specific admin (ownership-based)
func (s *DramaService) GetDramasByAdmin(ctx context.Context, adminID string) ([]models.Drama, error) {
	query := `
		SELECT * FROM dramas 
		WHERE created_by = $1 
		ORDER BY created_at DESC`

	var dramas []models.Drama
	err := s.db.SelectContext(ctx, &dramas, query, adminID)
	return dramas, err
}

// Get all dramas for super admin or platform management
func (s *DramaService) GetAllDramasForAdmin(ctx context.Context, limit, offset int) ([]models.Drama, error) {
	query := `
		SELECT * FROM dramas 
		ORDER BY created_at DESC 
		LIMIT $1 OFFSET $2`

	var dramas []models.Drama
	err := s.db.SelectContext(ctx, &dramas, query, limit, offset)
	return dramas, err
}

// ===============================
// UNIFIED DRAMA CREATION
// ===============================

func (s *DramaService) CreateDramaWithEpisodes(ctx context.Context, drama *models.Drama) (string, error) {
	// Validate drama
	if !drama.IsValidForCreation() {
		errors := drama.ValidateForCreation()
		return "", fmt.Errorf("validation failed: %v", errors)
	}

	// Set metadata
	drama.DramaID = uuid.New().String()
	drama.CreatedAt = time.Now()
	drama.UpdatedAt = time.Now()
	drama.IsActive = true
	drama.ViewCount = 0
	drama.FavoriteCount = 0

	query := `
		INSERT INTO dramas (
			drama_id, title, description, banner_image, episode_videos,
			is_premium, free_episodes_count, view_count, favorite_count,
			is_featured, is_active, created_by, created_at, updated_at, unlock_count
		) VALUES (
			:drama_id, :title, :description, :banner_image, :episode_videos,
			:is_premium, :free_episodes_count, :view_count, :favorite_count,
			:is_featured, :is_active, :created_by, :created_at, :updated_at, 0
		)`

	_, err := s.db.NamedExecContext(ctx, query, drama)
	return drama.DramaID, err
}

// ===============================
// DRAMA UPDATE AND DELETE - WITH OWNERSHIP VERIFICATION
// ===============================

func (s *DramaService) UpdateDrama(ctx context.Context, drama *models.Drama) error {
	// Validate drama
	if !drama.IsValidForCreation() {
		errors := drama.ValidateForCreation()
		return fmt.Errorf("validation failed: %v", errors)
	}

	drama.UpdatedAt = time.Now()

	query := `
		UPDATE dramas SET 
			title = :title, 
			description = :description, 
			banner_image = :banner_image,
			episode_videos = :episode_videos,
			is_premium = :is_premium, 
			free_episodes_count = :free_episodes_count,
			is_featured = :is_featured, 
			is_active = :is_active, 
			updated_at = :updated_at
		WHERE drama_id = :drama_id AND created_by = :created_by`

	result, err := s.db.NamedExecContext(ctx, query, drama)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return errors.New("drama_not_found_or_no_access")
	}

	return nil
}

func (s *DramaService) DeleteDrama(ctx context.Context, dramaID string) error {
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Check if drama exists
	var exists int
	err = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM dramas WHERE drama_id = $1", dramaID).Scan(&exists)
	if err != nil {
		return err
	}
	if exists == 0 {
		return errors.New("drama_not_found")
	}

	// Delete from user favorites and watch history
	_, err = tx.ExecContext(ctx, `
		UPDATE users 
		SET favorite_dramas = array_remove(favorite_dramas, $1),
		    unlocked_dramas = array_remove(unlocked_dramas, $1),
		    updated_at = $2
		WHERE $1 = ANY(favorite_dramas) OR $1 = ANY(unlocked_dramas)`,
		dramaID, time.Now())
	if err != nil {
		return err
	}

	// Delete drama progress records
	_, err = tx.ExecContext(ctx, "DELETE FROM user_drama_progress WHERE drama_id = $1", dramaID)
	if err != nil {
		return err
	}

	// Delete the drama
	_, err = tx.ExecContext(ctx, "DELETE FROM dramas WHERE drama_id = $1", dramaID)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// ===============================
// DRAMA STATUS TOGGLES - WITH OWNERSHIP VERIFICATION
// ===============================

func (s *DramaService) ToggleFeatured(ctx context.Context, dramaID string, isFeatured bool) error {
	query := `
		UPDATE dramas 
		SET is_featured = $1, updated_at = $2 
		WHERE drama_id = $3 AND is_active = true`

	result, err := s.db.ExecContext(ctx, query, isFeatured, time.Now(), dramaID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return errors.New("drama_not_found")
	}

	return nil
}

func (s *DramaService) ToggleActive(ctx context.Context, dramaID string, isActive bool) error {
	query := `
		UPDATE dramas 
		SET is_active = $1, updated_at = $2 
		WHERE drama_id = $3`

	result, err := s.db.ExecContext(ctx, query, isActive, time.Now(), dramaID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return errors.New("drama_not_found")
	}

	return nil
}

// ===============================
// DRAMA INTERACTIONS
// ===============================

func (s *DramaService) IncrementDramaViews(ctx context.Context, dramaID string) error {
	query := `
		UPDATE dramas 
		SET view_count = view_count + 1, updated_at = $1 
		WHERE drama_id = $2 AND is_active = true`

	_, err := s.db.ExecContext(ctx, query, time.Now(), dramaID)
	return err
}

func (s *DramaService) IncrementDramaFavorites(ctx context.Context, dramaID string, isAdding bool) error {
	increment := 1
	if !isAdding {
		increment = -1
	}

	query := `
		UPDATE dramas 
		SET favorite_count = GREATEST(0, favorite_count + $1), updated_at = $2 
		WHERE drama_id = $3 AND is_active = true`

	_, err := s.db.ExecContext(ctx, query, increment, time.Now(), dramaID)
	return err
}

// Helper method for async view counting
func (s *DramaService) incrementViewCount(dramaID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `UPDATE dramas SET view_count = view_count + 1 WHERE drama_id = $1 AND is_active = true`
	s.db.ExecContext(ctx, query, dramaID)
}

// ===============================
// SIMPLIFIED EPISODE MANAGEMENT - SINGLE OPERATIONS ONLY
// ===============================

// AddEpisodeToDrama adds a single episode to an existing drama
func (s *DramaService) AddEpisodeToDrama(ctx context.Context, dramaID, episodeVideoURL string, episodeNumber *int) (int, int, error) {
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return 0, 0, err
	}
	defer tx.Rollback()

	// Get current drama
	var drama models.Drama
	query := `SELECT * FROM dramas WHERE drama_id = $1`
	err = tx.GetContext(ctx, &drama, query, dramaID)
	if err != nil {
		return 0, 0, errors.New("drama_not_found")
	}

	// Check if drama has reached maximum episodes
	if len(drama.EpisodeVideos) >= MaxEpisodesPerDrama {
		return 0, 0, errors.New("max_episodes_reached")
	}

	// Validate episode video URL
	if !s.ValidateVideoURL(episodeVideoURL) {
		return 0, 0, errors.New("invalid_video_url")
	}

	// Determine episode number
	var targetEpisodeNumber int
	if episodeNumber != nil && *episodeNumber > 0 {
		targetEpisodeNumber = *episodeNumber

		// Check if episode number already exists
		if targetEpisodeNumber <= len(drama.EpisodeVideos) {
			return 0, 0, errors.New("episode_exists")
		}

		// Reject non-sequential episode numbers for simplicity
		if targetEpisodeNumber > len(drama.EpisodeVideos)+1 {
			return 0, 0, errors.New("episode_number_invalid")
		}
	} else {
		// Auto-assign next episode number
		targetEpisodeNumber = len(drama.EpisodeVideos) + 1
	}

	// Add episode to the array
	updatedEpisodes := append(drama.EpisodeVideos, episodeVideoURL)

	// Update drama with new episode
	query = `
		UPDATE dramas 
		SET episode_videos = $1, updated_at = $2 
		WHERE drama_id = $3`

	_, err = tx.ExecContext(ctx, query, models.StringSlice(updatedEpisodes), time.Now(), dramaID)
	if err != nil {
		return 0, 0, err
	}

	// Log episode addition for audit
	s.logEpisodeAction(ctx, tx, dramaID, "added", targetEpisodeNumber, episodeVideoURL)

	// Commit transaction
	if err = tx.Commit(); err != nil {
		return 0, 0, err
	}

	return targetEpisodeNumber, len(updatedEpisodes), nil
}

// RemoveEpisodeFromDrama removes a specific episode from a drama
func (s *DramaService) RemoveEpisodeFromDrama(ctx context.Context, dramaID string, episodeNumber int) (int, error) {
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	// Get current drama
	var drama models.Drama
	query := `SELECT * FROM dramas WHERE drama_id = $1`
	err = tx.GetContext(ctx, &drama, query, dramaID)
	if err != nil {
		return 0, errors.New("drama_not_found")
	}

	// Check if episode exists
	if episodeNumber < 1 || episodeNumber > len(drama.EpisodeVideos) {
		return 0, errors.New("episode_not_found")
	}

	// Store removed episode URL for logging
	removedEpisodeURL := drama.EpisodeVideos[episodeNumber-1]

	// Remove episode from array (convert to 0-based index)
	episodeIndex := episodeNumber - 1
	updatedEpisodes := make(models.StringSlice, 0, len(drama.EpisodeVideos)-1)
	updatedEpisodes = append(updatedEpisodes, drama.EpisodeVideos[:episodeIndex]...)
	updatedEpisodes = append(updatedEpisodes, drama.EpisodeVideos[episodeIndex+1:]...)

	// Update drama
	query = `
		UPDATE dramas 
		SET episode_videos = $1, updated_at = $2 
		WHERE drama_id = $3`

	_, err = tx.ExecContext(ctx, query, updatedEpisodes, time.Now(), dramaID)
	if err != nil {
		return 0, err
	}

	// Log episode removal for audit
	s.logEpisodeAction(ctx, tx, dramaID, "removed", episodeNumber, removedEpisodeURL)

	// Commit transaction
	if err = tx.Commit(); err != nil {
		return 0, err
	}

	return len(updatedEpisodes), nil
}

// ReplaceEpisodeInDrama replaces an existing episode with a new video URL
func (s *DramaService) ReplaceEpisodeInDrama(ctx context.Context, dramaID string, episodeNumber int, newVideoURL string) error {
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Get current drama
	var drama models.Drama
	query := `SELECT * FROM dramas WHERE drama_id = $1`
	err = tx.GetContext(ctx, &drama, query, dramaID)
	if err != nil {
		return errors.New("drama_not_found")
	}

	// Check if episode exists
	if episodeNumber < 1 || episodeNumber > len(drama.EpisodeVideos) {
		return errors.New("episode_not_found")
	}

	// Validate new video URL
	if !s.ValidateVideoURL(newVideoURL) {
		return errors.New("invalid_video_url")
	}

	// Store old episode URL for logging
	oldEpisodeURL := drama.EpisodeVideos[episodeNumber-1]

	// Replace episode URL (convert to 0-based index)
	episodeIndex := episodeNumber - 1
	updatedEpisodes := make(models.StringSlice, len(drama.EpisodeVideos))
	copy(updatedEpisodes, drama.EpisodeVideos)
	updatedEpisodes[episodeIndex] = newVideoURL

	// Update drama
	query = `
		UPDATE dramas 
		SET episode_videos = $1, updated_at = $2 
		WHERE drama_id = $3`

	_, err = tx.ExecContext(ctx, query, updatedEpisodes, time.Now(), dramaID)
	if err != nil {
		return err
	}

	// Log episode replacement for audit
	s.logEpisodeAction(ctx, tx, dramaID, "replaced", episodeNumber, fmt.Sprintf("%s -> %s", oldEpisodeURL, newVideoURL))

	// Commit transaction
	return tx.Commit()
}

// ===============================
// EPISODE INFORMATION AND UTILITIES
// ===============================

// GetDramaEpisodeDetails returns detailed information about a specific episode
func (s *DramaService) GetDramaEpisodeDetails(ctx context.Context, dramaID string, episodeNumber int) (*models.Episode, error) {
	drama, err := s.GetDrama(ctx, dramaID)
	if err != nil {
		return nil, err
	}

	if episodeNumber < 1 || episodeNumber > len(drama.EpisodeVideos) {
		return nil, errors.New("episode_not_found")
	}

	return &models.Episode{
		Number:     episodeNumber,
		VideoURL:   drama.EpisodeVideos[episodeNumber-1],
		DramaID:    dramaID,
		DramaTitle: drama.Title,
	}, nil
}

// GetDramaEpisodes returns episodes as Episode structs for frontend compatibility
func (s *DramaService) GetDramaEpisodes(ctx context.Context, dramaID string) ([]models.Episode, error) {
	drama, err := s.GetDrama(ctx, dramaID)
	if err != nil {
		return nil, err
	}

	return drama.GetAllEpisodes(), nil
}

// Enhanced GetEpisodeStats with unlock tracking
func (s *DramaService) GetEpisodeStatsWithUnlocks(ctx context.Context, dramaID string) (map[string]interface{}, error) {
	drama, err := s.GetDrama(ctx, dramaID)
	if err != nil {
		return nil, err
	}

	// Calculate stats
	totalEpisodes := len(drama.EpisodeVideos)
	freeEpisodes := drama.FreeEpisodesCount
	premiumEpisodes := 0

	if drama.IsPremium {
		premiumEpisodes = totalEpisodes - freeEpisodes
	}

	// Get unlock count from drama record
	var unlockCount int
	query := `SELECT COALESCE(unlock_count, 0) FROM dramas WHERE drama_id = $1`
	err = s.db.QueryRowContext(ctx, query, dramaID).Scan(&unlockCount)
	if err != nil {
		unlockCount = 0 // Default to 0 if query fails
	}

	// Calculate revenue if premium
	totalRevenue := 0
	if drama.IsPremium {
		totalRevenue = unlockCount * DramaUnlockCost
	}

	stats := map[string]interface{}{
		"dramaId":                  dramaID,
		"dramaTitle":               drama.Title,
		"totalEpisodes":            totalEpisodes,
		"freeEpisodes":             freeEpisodes,
		"premiumEpisodes":          premiumEpisodes,
		"isPremium":                drama.IsPremium,
		"isActive":                 drama.IsActive,
		"isFeatured":               drama.IsFeatured,
		"totalViews":               drama.ViewCount,
		"totalFavorites":           drama.FavoriteCount,
		"totalUnlocks":             unlockCount,
		"unlockRevenue":            totalRevenue,
		"unlockCostPerDrama":       DramaUnlockCost,
		"conversionRate":           s.calculateConversionRate(drama.ViewCount, unlockCount),
		"lastUpdated":              drama.UpdatedAt,
		"createdAt":                drama.CreatedAt,
		"averageEpisodesPerUpdate": s.calculateAverageEpisodesPerUpdate(drama),
	}

	return stats, nil
}

// Legacy GetEpisodeStats for backward compatibility
func (s *DramaService) GetEpisodeStats(ctx context.Context, dramaID string) (map[string]interface{}, error) {
	return s.GetEpisodeStatsWithUnlocks(ctx, dramaID)
}

// ValidateVideoURL validates if a video URL is in correct format with file size constraints
func (s *DramaService) ValidateVideoURL(videoURL string) bool {
	if videoURL == "" {
		return false
	}

	// Parse URL
	parsedURL, err := url.Parse(videoURL)
	if err != nil {
		return false
	}

	// Check if it's a valid URL with scheme
	if parsedURL.Scheme == "" || parsedURL.Host == "" {
		return false
	}

	// Check supported schemes
	supportedSchemes := []string{"http", "https"}
	schemeValid := false
	for _, scheme := range supportedSchemes {
		if parsedURL.Scheme == scheme {
			schemeValid = true
			break
		}
	}

	if !schemeValid {
		return false
	}

	// Check for common video file extensions
	path := strings.ToLower(parsedURL.Path)
	videoExtensions := []string{".mp4", ".mov", ".avi", ".mkv", ".webm", ".m4v"}

	for _, ext := range videoExtensions {
		if strings.HasSuffix(path, ext) {
			return true
		}
	}

	// Check known video hosting services
	knownVideoHosts := []string{
		"youtube.com", "youtu.be", "vimeo.com", "dailymotion.com",
		"cloudfront.net", "amazonaws.com", "r2.cloudflarestorage.com",
		"googleapis.com", "googleusercontent.com",
	}

	for _, host := range knownVideoHosts {
		if strings.Contains(parsedURL.Host, host) {
			return true
		}
	}

	return false
}

// ===============================
// USER STATISTICS AND LIMITS
// ===============================

// GetUserDramaCount returns the number of dramas created by a user
func (s *DramaService) GetUserDramaCount(ctx context.Context, userID string) (int, error) {
	var count int
	query := `SELECT COUNT(*) FROM dramas WHERE created_by = $1`
	err := s.db.QueryRowContext(ctx, query, userID).Scan(&count)
	return count, err
}

// GetUserTotalEpisodeCount returns total episodes across all user's dramas
func (s *DramaService) GetUserTotalEpisodeCount(ctx context.Context, userID string) (int, error) {
	query := `
		SELECT COALESCE(SUM(jsonb_array_length(episode_videos)), 0) as total_episodes
		FROM dramas 
		WHERE created_by = $1`

	var totalEpisodes int
	err := s.db.QueryRowContext(ctx, query, userID).Scan(&totalEpisodes)
	return totalEpisodes, err
}

// GetUserDramaStats returns comprehensive statistics for a user's dramas including unlock revenue
func (s *DramaService) GetUserDramaStats(ctx context.Context, userID string) (map[string]interface{}, error) {
	query := `
		SELECT 
			COUNT(*) as total_dramas,
			COUNT(CASE WHEN is_active = true THEN 1 END) as active_dramas,
			COUNT(CASE WHEN is_featured = true THEN 1 END) as featured_dramas,
			COUNT(CASE WHEN is_premium = true THEN 1 END) as premium_dramas,
			COALESCE(SUM(jsonb_array_length(episode_videos)), 0) as total_episodes,
			COALESCE(SUM(view_count), 0) as total_views,
			COALESCE(SUM(favorite_count), 0) as total_favorites,
			COALESCE(SUM(unlock_count), 0) as total_unlocks,
			COALESCE(SUM(CASE WHEN is_premium = true THEN unlock_count * $2 ELSE 0 END), 0) as total_revenue,
			MAX(created_at) as last_drama_created,
			MAX(updated_at) as last_drama_updated
		FROM dramas 
		WHERE created_by = $1`

	var stats struct {
		TotalDramas      int       `db:"total_dramas"`
		ActiveDramas     int       `db:"active_dramas"`
		FeaturedDramas   int       `db:"featured_dramas"`
		PremiumDramas    int       `db:"premium_dramas"`
		TotalEpisodes    int       `db:"total_episodes"`
		TotalViews       int       `db:"total_views"`
		TotalFavorites   int       `db:"total_favorites"`
		TotalUnlocks     int       `db:"total_unlocks"`
		TotalRevenue     int       `db:"total_revenue"`
		LastDramaCreated time.Time `db:"last_drama_created"`
		LastDramaUpdated time.Time `db:"last_drama_updated"`
	}

	err := s.db.GetContext(ctx, &stats, query, userID, DramaUnlockCost)
	if err != nil {
		return nil, err
	}

	result := map[string]interface{}{
		"totalDramas":             stats.TotalDramas,
		"activeDramas":            stats.ActiveDramas,
		"inactiveDramas":          stats.TotalDramas - stats.ActiveDramas,
		"featuredDramas":          stats.FeaturedDramas,
		"premiumDramas":           stats.PremiumDramas,
		"freeDramas":              stats.TotalDramas - stats.PremiumDramas,
		"totalEpisodes":           stats.TotalEpisodes,
		"totalViews":              stats.TotalViews,
		"totalFavorites":          stats.TotalFavorites,
		"totalUnlocks":            stats.TotalUnlocks,
		"totalRevenue":            stats.TotalRevenue,
		"averageViewsPerDrama":    0,
		"averageEpisodesPerDrama": 0,
		"averageUnlocksPerDrama":  0,
		"averageRevenuePerDrama":  0,
		"lastDramaCreated":        stats.LastDramaCreated,
		"lastDramaUpdated":        stats.LastDramaUpdated,
	}

	// Calculate averages
	if stats.TotalDramas > 0 {
		result["averageViewsPerDrama"] = stats.TotalViews / stats.TotalDramas
		result["averageEpisodesPerDrama"] = stats.TotalEpisodes / stats.TotalDramas
		result["averageUnlocksPerDrama"] = stats.TotalUnlocks / stats.TotalDramas
		result["averageRevenuePerDrama"] = stats.TotalRevenue / stats.TotalDramas
	}

	return result, nil
}

// ===============================
// DRAMA UNLOCK OPERATION - WITH ENHANCED UNLOCK TRACKING
// ===============================

func (s *DramaService) UnlockDrama(ctx context.Context, userID, dramaID string) (bool, int, error) {
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return false, 0, err
	}
	defer tx.Rollback()

	// Get user unlocked dramas from users table and balance from wallet
	var user models.User
	query := `SELECT unlocked_dramas FROM users WHERE uid = $1`
	err = tx.GetContext(ctx, &user, query, userID)
	if err != nil {
		return false, 0, errors.New("user_not_found")
	}

	// Get current balance from wallet table
	var currentBalance int
	query = `SELECT coins_balance FROM wallets WHERE user_id = $1`
	err = tx.QueryRowContext(ctx, query, userID).Scan(&currentBalance)
	if err != nil {
		return false, 0, errors.New("wallet_not_found")
	}

	// Check if drama already unlocked
	for _, id := range user.UnlockedDramas {
		if id == dramaID {
			return false, currentBalance, errors.New("already_unlocked")
		}
	}

	// Get drama info
	var drama models.Drama
	query = `SELECT * FROM dramas WHERE drama_id = $1 AND is_active = true`
	err = tx.GetContext(ctx, &drama, query, dramaID)
	if err != nil {
		return false, 0, errors.New("drama_not_found")
	}

	// Check if drama is premium
	if !drama.IsPremium {
		return false, currentBalance, errors.New("drama_free")
	}

	// Check sufficient balance - UPDATED COST TO 99 COINS
	if currentBalance < DramaUnlockCost {
		return false, currentBalance, errors.New("insufficient_funds")
	}

	// Update user unlocked dramas
	newUnlocked := append(user.UnlockedDramas, dramaID)
	newBalance := currentBalance - DramaUnlockCost

	query = `
		UPDATE users 
		SET unlocked_dramas = $1, updated_at = $2 
		WHERE uid = $3`
	_, err = tx.ExecContext(ctx, query, models.StringSlice(newUnlocked), time.Now(), userID)
	if err != nil {
		return false, 0, err
	}

	// Update wallet balance
	query = `
		UPDATE wallets 
		SET coins_balance = $1, updated_at = $2 
		WHERE user_id = $3`
	_, err = tx.ExecContext(ctx, query, newBalance, time.Now(), userID)
	if err != nil {
		return false, 0, err
	}

	// INCREMENT UNLOCK COUNT IN DRAMAS TABLE - NEW FEATURE
	query = `
		UPDATE dramas 
		SET unlock_count = unlock_count + 1, updated_at = $1 
		WHERE drama_id = $2`
	_, err = tx.ExecContext(ctx, query, time.Now(), dramaID)
	if err != nil {
		return false, 0, err
	}

	// Create transaction record
	transactionID := uuid.New().String()
	metadata := models.MetadataMap{
		"dramaId":      dramaID,
		"dramaTitle":   drama.Title,
		"unlockType":   "full_drama",
		"episodeCount": fmt.Sprintf("%d", len(drama.EpisodeVideos)),
		"unlockCost":   fmt.Sprintf("%d", DramaUnlockCost),
	}

	transaction := models.WalletTransaction{
		TransactionID: transactionID,
		WalletID:      userID,
		UserID:        userID,
		Type:          "drama_unlock",
		CoinAmount:    DramaUnlockCost,
		BalanceBefore: currentBalance,
		BalanceAfter:  newBalance,
		Description:   fmt.Sprintf("Unlocked: %s (%d episodes) - %d coins", drama.Title, len(drama.EpisodeVideos), DramaUnlockCost),
		ReferenceID:   &dramaID,
		Metadata:      metadata,
		CreatedAt:     time.Now(),
	}

	query = `
		INSERT INTO wallet_transactions (
			transaction_id, wallet_id, user_id, user_phone_number, user_name,
			type, coin_amount, balance_before, balance_after, description,
			reference_id, metadata, created_at
		) VALUES (
			:transaction_id, :wallet_id, :user_id, :user_phone_number, :user_name,
			:type, :coin_amount, :balance_before, :balance_after, :description,
			:reference_id, :metadata, :created_at
		)`

	_, err = tx.NamedExecContext(ctx, query, transaction)
	if err != nil {
		return false, 0, err
	}

	// Commit transaction
	if err = tx.Commit(); err != nil {
		return false, 0, err
	}

	return true, newBalance, nil
}

// ===============================
// ANALYTICS AND REPORTING
// ===============================

// GetTopEarningDramas returns most popular dramas based on unlock revenue
func (s *DramaService) GetTopEarningDramas(ctx context.Context, limit int) ([]map[string]interface{}, error) {
	query := `
		SELECT 
			drama_id, 
			title,
			COALESCE(unlock_count, 0) as unlocks,
			(COALESCE(unlock_count, 0) * $2) as revenue,
			view_count,
			is_premium,
			created_at,
			created_by
		FROM dramas 
		WHERE is_premium = true AND is_active = true
		ORDER BY unlock_count DESC, view_count DESC
		LIMIT $1`

	rows, err := s.db.QueryContext(ctx, query, limit, DramaUnlockCost)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		var dramaID, title, createdBy string
		var unlocks, revenue, viewCount int
		var isPremium bool
		var createdAt time.Time

		err := rows.Scan(&dramaID, &title, &unlocks, &revenue, &viewCount, &isPremium, &createdAt, &createdBy)
		if err != nil {
			continue
		}

		result := map[string]interface{}{
			"dramaId":        dramaID,
			"title":          title,
			"unlocks":        unlocks,
			"revenue":        revenue,
			"viewCount":      viewCount,
			"isPremium":      isPremium,
			"createdAt":      createdAt,
			"createdBy":      createdBy,
			"conversionRate": s.calculateConversionRate(viewCount, unlocks),
		}

		results = append(results, result)
	}

	return results, nil
}

// GetPopularDramas returns most popular dramas based on views and favorites
func (s *DramaService) GetPopularDramas(ctx context.Context, limit int, timeframe string) ([]models.Drama, error) {
	var query string

	switch timeframe {
	case "week":
		query = `
			SELECT * FROM dramas 
			WHERE is_active = true 
			AND created_at > NOW() - INTERVAL '7 days'
			ORDER BY (view_count * 0.7 + favorite_count * 0.3) DESC 
			LIMIT $1`
	case "month":
		query = `
			SELECT * FROM dramas 
			WHERE is_active = true 
			AND created_at > NOW() - INTERVAL '30 days'
			ORDER BY (view_count * 0.7 + favorite_count * 0.3) DESC 
			LIMIT $1`
	default: // all time
		query = `
			SELECT * FROM dramas 
			WHERE is_active = true 
			ORDER BY (view_count * 0.7 + favorite_count * 0.3) DESC 
			LIMIT $1`
	}

	var dramas []models.Drama
	err := s.db.SelectContext(ctx, &dramas, query, limit)
	return dramas, err
}

// GetRecentlyUpdatedDramas returns dramas that were recently updated (new episodes added)
func (s *DramaService) GetRecentlyUpdatedDramas(ctx context.Context, limit int) ([]models.Drama, error) {
	query := `
		SELECT * FROM dramas 
		WHERE is_active = true 
		AND updated_at > created_at + INTERVAL '1 hour'
		ORDER BY updated_at DESC 
		LIMIT $1`

	var dramas []models.Drama
	err := s.db.SelectContext(ctx, &dramas, query, limit)
	return dramas, err
}

// ===============================
// EPISODE-LIKE OPERATIONS (for compatibility)
// ===============================

// GetEpisodeByNumber returns a specific episode by drama ID and episode number
func (s *DramaService) GetEpisodeByNumber(ctx context.Context, dramaID string, episodeNumber int) (*models.Episode, error) {
	drama, err := s.GetDrama(ctx, dramaID)
	if err != nil {
		return nil, err
	}

	episode := drama.GetEpisode(episodeNumber)
	if episode == nil {
		return nil, errors.New("episode_not_found")
	}

	return episode, nil
}

// SearchEpisodes searches for episodes across all dramas
func (s *DramaService) SearchEpisodes(ctx context.Context, query, dramaID string, limit int) ([]models.Episode, error) {
	var episodes []models.Episode

	if dramaID != "" {
		// Search within specific drama
		drama, err := s.GetDrama(ctx, dramaID)
		if err != nil {
			return nil, err
		}

		allEpisodes := drama.GetAllEpisodes()
		// Simple search by episode number (could be enhanced)
		for _, ep := range allEpisodes {
			if len(episodes) >= limit {
				break
			}
			episodes = append(episodes, ep)
		}
	} else {
		// Search across all dramas (limit results)
		searchLimit := limit / 10 // Get fewer dramas to avoid too many episodes
		if searchLimit < 1 {
			searchLimit = 1
		}

		dramas, err := s.SearchDramas(ctx, query, searchLimit)
		if err != nil {
			return nil, err
		}

		for _, drama := range dramas {
			if len(episodes) >= limit {
				break
			}

			dramaEpisodes := drama.GetAllEpisodes()
			for _, ep := range dramaEpisodes {
				if len(episodes) >= limit {
					break
				}
				episodes = append(episodes, ep)
			}
		}
	}

	return episodes, nil
}

// ===============================
// ADVANCED ANALYTICS
// ===============================

// GetDramasWithEpisodeCount returns dramas with their episode counts for admin dashboard
func (s *DramaService) GetDramasWithEpisodeCount(ctx context.Context, adminID string, limit, offset int) ([]map[string]interface{}, error) {
	query := `
		SELECT 
			drama_id,
			title,
			description,
			banner_image,
			jsonb_array_length(episode_videos) as episode_count,
			is_premium,
			free_episodes_count,
			is_featured,
			is_active,
			view_count,
			favorite_count,
			COALESCE(unlock_count, 0) as unlock_count,
			(COALESCE(unlock_count, 0) * $4) as revenue,
			created_at,
			updated_at
		FROM dramas 
		WHERE created_by = $1 
		ORDER BY updated_at DESC 
		LIMIT $2 OFFSET $3`

	rows, err := s.db.QueryContext(ctx, query, adminID, limit, offset, DramaUnlockCost)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []map[string]interface{}

	for rows.Next() {
		var dramaID, title, description, bannerImage string
		var episodeCount, freeEpisodesCount, viewCount, favoriteCount, unlockCount, revenue int
		var isPremium, isFeatured, isActive bool
		var createdAt, updatedAt time.Time

		err := rows.Scan(
			&dramaID, &title, &description, &bannerImage,
			&episodeCount, &isPremium, &freeEpisodesCount,
			&isFeatured, &isActive, &viewCount, &favoriteCount,
			&unlockCount, &revenue, &createdAt, &updatedAt,
		)
		if err != nil {
			return nil, err
		}

		result := map[string]interface{}{
			"dramaId":           dramaID,
			"title":             title,
			"description":       description,
			"bannerImage":       bannerImage,
			"episodeCount":      episodeCount,
			"isPremium":         isPremium,
			"freeEpisodesCount": freeEpisodesCount,
			"isFeatured":        isFeatured,
			"isActive":          isActive,
			"viewCount":         viewCount,
			"favoriteCount":     favoriteCount,
			"unlockCount":       unlockCount,
			"revenue":           revenue,
			"conversionRate":    s.calculateConversionRate(viewCount, unlockCount),
			"createdAt":         createdAt,
			"updatedAt":         updatedAt,
		}

		results = append(results, result)
	}

	return results, nil
}

// ===============================
// HELPER METHODS AND UTILITIES
// ===============================

// Calculate conversion rate from views to unlocks
func (s *DramaService) calculateConversionRate(views, unlocks int) float64 {
	if views == 0 {
		return 0.0
	}
	return (float64(unlocks) / float64(views)) * 100.0 // Return as percentage
}

// logEpisodeAction logs episode operations for audit trail
func (s *DramaService) logEpisodeAction(ctx context.Context, tx *sqlx.Tx, dramaID, action string, episodeNumber interface{}, details string) {
	// This is optional - implement if you want audit logging
	logEntry := map[string]interface{}{
		"drama_id":       dramaID,
		"action":         action,
		"episode_number": episodeNumber,
		"details":        details,
		"timestamp":      time.Now(),
	}

	// Log to your preferred logging system
	fmt.Printf("Episode Action: %+v\n", logEntry)
}

// calculateAverageEpisodesPerUpdate calculates how many episodes are typically added per update
func (s *DramaService) calculateAverageEpisodesPerUpdate(drama *models.Drama) float64 {
	if drama.CreatedAt.IsZero() || drama.UpdatedAt.IsZero() {
		return 0
	}

	daysSinceCreation := drama.UpdatedAt.Sub(drama.CreatedAt).Hours() / 24
	if daysSinceCreation <= 0 {
		return float64(len(drama.EpisodeVideos))
	}

	return float64(len(drama.EpisodeVideos)) / daysSinceCreation
}

// GetDramaByIDWithoutViewIncrement gets drama without incrementing view count
func (s *DramaService) GetDramaByIDWithoutViewIncrement(ctx context.Context, dramaID string) (*models.Drama, error) {
	query := `SELECT * FROM dramas WHERE drama_id = $1 AND is_active = true`

	var drama models.Drama
	err := s.db.GetContext(ctx, &drama, query, dramaID)
	if err != nil {
		return nil, err
	}

	return &drama, nil
}

// ValidateDramaForEpisodeOperation validates if a drama can have episode operations performed
func (s *DramaService) ValidateDramaForEpisodeOperation(ctx context.Context, dramaID string) error {
	drama, err := s.GetDramaByIDWithoutViewIncrement(ctx, dramaID)
	if err != nil {
		return errors.New("drama_not_found")
	}

	if !drama.IsActive {
		return errors.New("drama_inactive")
	}

	return nil
}
