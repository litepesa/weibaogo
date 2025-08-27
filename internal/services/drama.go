// ===============================
// internal/services/drama.go - Complete Implementation
// ===============================

package services

import (
	"context"
	"errors"
	"fmt"
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
// DRAMA OPERATIONS
// ===============================

func (s *DramaService) GetDramas(ctx context.Context, limit int) ([]models.Drama, error) {
	query := `
		SELECT * FROM dramas 
		WHERE is_active = true 
		ORDER BY created_at DESC 
		LIMIT $1`

	var dramas []models.Drama
	err := s.db.SelectContext(ctx, &dramas, query, limit)
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
		WHERE is_active = true AND title ILIKE $1 
		ORDER BY created_at DESC 
		LIMIT $2`

	var dramas []models.Drama
	err := s.db.SelectContext(ctx, &dramas, query, "%"+searchQuery+"%", limit)
	return dramas, err
}

func (s *DramaService) GetDrama(ctx context.Context, dramaID string) (*models.Drama, error) {
	query := `SELECT * FROM dramas WHERE drama_id = $1 AND is_active = true`

	var drama models.Drama
	err := s.db.GetContext(ctx, &drama, query, dramaID)
	if err != nil {
		return nil, err
	}

	// Increment view count asynchronously
	go func() {
		s.db.Exec("UPDATE dramas SET view_count = view_count + 1 WHERE drama_id = $1", dramaID)
	}()

	return &drama, nil
}

func (s *DramaService) CreateDrama(ctx context.Context, drama *models.Drama) (string, error) {
	drama.DramaID = uuid.New().String()
	drama.CreatedAt = time.Now()
	drama.UpdatedAt = time.Now()
	drama.PublishedAt = time.Now()
	drama.IsActive = true
	drama.ViewCount = 0
	drama.FavoriteCount = 0

	query := `
		INSERT INTO dramas (
			drama_id, title, description, banner_image, total_episodes,
			is_premium, free_episodes_count, view_count, favorite_count,
			is_featured, is_active, created_by, created_at, updated_at, published_at
		) VALUES (
			:drama_id, :title, :description, :banner_image, :total_episodes,
			:is_premium, :free_episodes_count, :view_count, :favorite_count,
			:is_featured, :is_active, :created_by, :created_at, :updated_at, :published_at
		)`

	_, err := s.db.NamedExecContext(ctx, query, drama)
	return drama.DramaID, err
}

func (s *DramaService) UpdateDrama(ctx context.Context, drama *models.Drama) error {
	drama.UpdatedAt = time.Now()

	query := `
		UPDATE dramas SET 
			title = :title, 
			description = :description, 
			banner_image = :banner_image,
			total_episodes = :total_episodes, 
			is_premium = :is_premium, 
			free_episodes_count = :free_episodes_count,
			is_featured = :is_featured, 
			is_active = :is_active, 
			updated_at = :updated_at
		WHERE drama_id = :drama_id`

	_, err := s.db.NamedExecContext(ctx, query, drama)
	return err
}

func (s *DramaService) DeleteDrama(ctx context.Context, dramaID string) error {
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Delete episodes first
	_, err = tx.ExecContext(ctx, "DELETE FROM episodes WHERE drama_id = $1", dramaID)
	if err != nil {
		return err
	}

	// Delete drama
	_, err = tx.ExecContext(ctx, "DELETE FROM dramas WHERE drama_id = $1", dramaID)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (s *DramaService) ToggleFeatured(ctx context.Context, dramaID string, isFeatured bool) error {
	query := `
		UPDATE dramas 
		SET is_featured = $1, updated_at = $2 
		WHERE drama_id = $3`

	_, err := s.db.ExecContext(ctx, query, isFeatured, time.Now(), dramaID)
	return err
}

func (s *DramaService) ToggleActive(ctx context.Context, dramaID string, isActive bool) error {
	query := `
		UPDATE dramas 
		SET is_active = $1, updated_at = $2 
		WHERE drama_id = $3`

	_, err := s.db.ExecContext(ctx, query, isActive, time.Now(), dramaID)
	return err
}

func (s *DramaService) GetDramasByAdmin(ctx context.Context, adminID string) ([]models.Drama, error) {
	query := `
		SELECT * FROM dramas 
		WHERE created_by = $1 
		ORDER BY created_at DESC`

	var dramas []models.Drama
	err := s.db.SelectContext(ctx, &dramas, query, adminID)
	return dramas, err
}

// ===============================
// EPISODE OPERATIONS
// ===============================

func (s *DramaService) GetDramaEpisodes(ctx context.Context, dramaID string) ([]models.Episode, error) {
	query := `
		SELECT * FROM episodes 
		WHERE drama_id = $1 
		ORDER BY episode_number ASC`

	var episodes []models.Episode
	err := s.db.SelectContext(ctx, &episodes, query, dramaID)
	return episodes, err
}

func (s *DramaService) GetEpisode(ctx context.Context, episodeID string) (*models.Episode, error) {
	query := `SELECT * FROM episodes WHERE episode_id = $1`

	var episode models.Episode
	err := s.db.GetContext(ctx, &episode, query, episodeID)
	if err != nil {
		return nil, err
	}

	// Increment view count asynchronously
	go func() {
		s.db.Exec("UPDATE episodes SET episode_view_count = episode_view_count + 1 WHERE episode_id = $1", episodeID)
	}()

	return &episode, nil
}

func (s *DramaService) CreateEpisode(ctx context.Context, episode *models.Episode) (string, error) {
	episode.EpisodeID = uuid.New().String()
	episode.CreatedAt = time.Now()
	episode.UpdatedAt = time.Now()
	episode.ReleasedAt = time.Now()
	episode.EpisodeViewCount = 0

	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	// Check for duplicate episode number
	var existingCount int
	err = tx.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM episodes WHERE drama_id = $1 AND episode_number = $2",
		episode.DramaID, episode.EpisodeNumber).Scan(&existingCount)
	if err != nil {
		return "", err
	}

	if existingCount > 0 {
		return "", errors.New("duplicate_episode_number")
	}

	query := `
		INSERT INTO episodes (
			episode_id, drama_id, episode_number, episode_title,
			thumbnail_url, video_url, video_duration, episode_view_count,
			uploaded_by, created_at, updated_at, released_at
		) VALUES (
			:episode_id, :drama_id, :episode_number, :episode_title,
			:thumbnail_url, :video_url, :video_duration, :episode_view_count,
			:uploaded_by, :created_at, :updated_at, :released_at
		)`

	_, err = tx.NamedExecContext(ctx, query, episode)
	if err != nil {
		return "", err
	}

	// Update drama's total episodes count
	_, err = tx.ExecContext(ctx, `
		UPDATE dramas 
		SET total_episodes = (SELECT COUNT(*) FROM episodes WHERE drama_id = $1),
		    updated_at = $2 
		WHERE drama_id = $1`, episode.DramaID, time.Now())
	if err != nil {
		return "", err
	}

	if err = tx.Commit(); err != nil {
		return "", err
	}

	return episode.EpisodeID, nil
}

func (s *DramaService) UpdateEpisode(ctx context.Context, episode *models.Episode) error {
	episode.UpdatedAt = time.Now()

	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Check if episode exists
	var exists int
	err = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM episodes WHERE episode_id = $1", episode.EpisodeID).Scan(&exists)
	if err != nil {
		return err
	}
	if exists == 0 {
		return errors.New("episode_not_found")
	}

	// Check for duplicate episode number (excluding current episode)
	var duplicateCount int
	err = tx.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM episodes WHERE drama_id = $1 AND episode_number = $2 AND episode_id != $3",
		episode.DramaID, episode.EpisodeNumber, episode.EpisodeID).Scan(&duplicateCount)
	if err != nil {
		return err
	}

	if duplicateCount > 0 {
		return errors.New("duplicate_episode_number")
	}

	query := `
		UPDATE episodes SET 
			episode_number = :episode_number, 
			episode_title = :episode_title,
			thumbnail_url = :thumbnail_url, 
			video_url = :video_url, 
			video_duration = :video_duration,
			updated_at = :updated_at
		WHERE episode_id = :episode_id`

	_, err = tx.NamedExecContext(ctx, query, episode)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (s *DramaService) DeleteEpisode(ctx context.Context, episodeID string) error {
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Check if episode exists and get drama ID
	var dramaID string
	err = tx.QueryRowContext(ctx, "SELECT drama_id FROM episodes WHERE episode_id = $1", episodeID).Scan(&dramaID)
	if err != nil {
		return errors.New("episode_not_found")
	}

	// Delete episode
	_, err = tx.ExecContext(ctx, "DELETE FROM episodes WHERE episode_id = $1", episodeID)
	if err != nil {
		return err
	}

	// Update drama's total episodes count
	_, err = tx.ExecContext(ctx, `
		UPDATE dramas 
		SET total_episodes = (SELECT COUNT(*) FROM episodes WHERE drama_id = $1),
		    updated_at = $2 
		WHERE drama_id = $1`, dramaID, time.Now())
	if err != nil {
		return err
	}

	return tx.Commit()
}

// ===============================
// BULK OPERATIONS
// ===============================

func (s *DramaService) BulkCreateEpisodes(ctx context.Context, episodes []models.Episode) ([]string, error) {
	if len(episodes) == 0 {
		return nil, errors.New("no episodes provided")
	}

	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	dramaID := episodes[0].DramaID
	var episodeIDs []string

	// Check for duplicate episode numbers within the batch and existing episodes
	episodeNumbers := make(map[int]bool)
	for _, episode := range episodes {
		if episodeNumbers[episode.EpisodeNumber] {
			return nil, errors.New("duplicate_episode_numbers")
		}
		episodeNumbers[episode.EpisodeNumber] = true

		// Check if episode number already exists in database
		var existingCount int
		err = tx.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM episodes WHERE drama_id = $1 AND episode_number = $2",
			dramaID, episode.EpisodeNumber).Scan(&existingCount)
		if err != nil {
			return nil, err
		}
		if existingCount > 0 {
			return nil, errors.New("duplicate_episode_numbers")
		}
	}

	// Insert all episodes
	for _, episode := range episodes {
		episode.EpisodeID = uuid.New().String()
		episode.CreatedAt = time.Now()
		episode.UpdatedAt = time.Now()
		episode.ReleasedAt = time.Now()
		episode.EpisodeViewCount = 0

		query := `
			INSERT INTO episodes (
				episode_id, drama_id, episode_number, episode_title,
				thumbnail_url, video_url, video_duration, episode_view_count,
				uploaded_by, created_at, updated_at, released_at
			) VALUES (
				:episode_id, :drama_id, :episode_number, :episode_title,
				:thumbnail_url, :video_url, :video_duration, :episode_view_count,
				:uploaded_by, :created_at, :updated_at, :released_at
			)`

		_, err = tx.NamedExecContext(ctx, query, episode)
		if err != nil {
			return nil, err
		}

		episodeIDs = append(episodeIDs, episode.EpisodeID)
	}

	// Update drama's total episodes count
	_, err = tx.ExecContext(ctx, `
		UPDATE dramas 
		SET total_episodes = (SELECT COUNT(*) FROM episodes WHERE drama_id = $1),
		    updated_at = $2 
		WHERE drama_id = $1`, dramaID, time.Now())
	if err != nil {
		return nil, err
	}

	if err = tx.Commit(); err != nil {
		return nil, err
	}

	return episodeIDs, nil
}

// ===============================
// SEARCH AND FILTER OPERATIONS
// ===============================

func (s *DramaService) SearchEpisodes(ctx context.Context, query, dramaID string, limit int) ([]models.Episode, error) {
	var episodes []models.Episode
	var sqlQuery string
	var args []interface{}

	if dramaID != "" {
		sqlQuery = `
			SELECT * FROM episodes 
			WHERE drama_id = $1 AND episode_title ILIKE $2 
			ORDER BY episode_number ASC 
			LIMIT $3`
		args = []interface{}{dramaID, "%" + query + "%", limit}
	} else {
		sqlQuery = `
			SELECT * FROM episodes 
			WHERE episode_title ILIKE $1 
			ORDER BY created_at DESC 
			LIMIT $2`
		args = []interface{}{"%" + query + "%", limit}
	}

	err := s.db.SelectContext(ctx, &episodes, sqlQuery, args...)
	return episodes, err
}

// ===============================
// DRAMA UNLOCK OPERATION - UPDATED
// ===============================

func (s *DramaService) UnlockDrama(ctx context.Context, userID, dramaID string) (bool, int, error) {
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return false, 0, err
	}
	defer tx.Rollback()

	// UPDATED: Get user unlocked dramas from users table and balance from wallet
	var user models.User
	query := `SELECT unlocked_dramas FROM users WHERE uid = $1`
	err = tx.GetContext(ctx, &user, query, userID)
	if err != nil {
		return false, 0, errors.New("user_not_found")
	}

	// UPDATED: Get current balance from wallet table
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

	// Check sufficient balance
	unlockCost := models.DramaUnlockCost
	if currentBalance < unlockCost {
		return false, currentBalance, errors.New("insufficient_funds")
	}

	// Update user unlocked dramas (only this field)
	newUnlocked := append(user.UnlockedDramas, dramaID)
	newBalance := currentBalance - unlockCost

	query = `
		UPDATE users 
		SET unlocked_dramas = $1, updated_at = $2 
		WHERE uid = $3`
	_, err = tx.ExecContext(ctx, query, models.StringSlice(newUnlocked), time.Now(), userID)
	if err != nil {
		return false, 0, err
	}

	// UPDATED: Update wallet balance (single source of truth)
	query = `
		UPDATE wallets 
		SET coins_balance = $1, updated_at = $2 
		WHERE user_id = $3`
	_, err = tx.ExecContext(ctx, query, newBalance, time.Now(), userID)
	if err != nil {
		return false, 0, err
	}

	// Create transaction record
	transactionID := uuid.New().String()
	metadata := models.MetadataMap{
		"dramaId":    dramaID,
		"dramaTitle": drama.Title,
		"unlockType": "full_drama",
	}

	transaction := models.WalletTransaction{
		TransactionID: transactionID,
		WalletID:      userID,
		UserID:        userID,
		Type:          "drama_unlock",
		CoinAmount:    unlockCost,
		BalanceBefore: currentBalance,
		BalanceAfter:  newBalance,
		Description:   fmt.Sprintf("Unlocked: %s", drama.Title),
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
