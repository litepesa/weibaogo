// ===============================
// internal/services/drama.go
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

func (s *DramaService) ReorderEpisodes(context context.Context, dramaID string, order []struct {
	EpisodeID     string "json:\"episodeId\" binding:\"required\""
	EpisodeNumber int    "json:\"episodeNumber\" binding:\"required,min=1\""
}) any {
	panic("unimplemented")
}

func (s *DramaService) GetDramaEpisodeStats(context context.Context, dramaID string) (any, any) {
	panic("unimplemented")
}

func (s *DramaService) BatchUpdateEpisodes(context context.Context, episodes []models.Episode) any {
	panic("unimplemented")
}

func (s *DramaService) BatchDeleteEpisodes(context context.Context, ds []string) (any, any) {
	panic("unimplemented")
}

func (s *DramaService) ValidateEpisodeAccess(context context.Context, userID string, episodeID string) (any, any, any) {
	panic("unimplemented")
}

func (s *DramaService) SearchEpisodes(context context.Context, query string, dramaID string, limit int) (any, any) {
	panic("unimplemented")
}

func (s *DramaService) GetEpisodesByDuration(context context.Context, minDuration int, maxDuration int, limit int) (any, any) {
	panic("unimplemented")
}

func NewDramaService(db *sqlx.DB, r2Client *storage.R2Client) *DramaService {
	return &DramaService{
		db:       db,
		r2Client: r2Client,
	}
}

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

// Atomic drama unlock implementation
func (s *DramaService) UnlockDrama(ctx context.Context, userID, dramaID string) (bool, int, error) {
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return false, 0, err
	}
	defer tx.Rollback()

	// Get user and check current balance and unlocked dramas
	var user models.User
	query := `SELECT coins_balance, unlocked_dramas FROM users WHERE uid = $1`
	err = tx.GetContext(ctx, &user, query, userID)
	if err != nil {
		return false, 0, errors.New("user_not_found")
	}

	// Check if drama already unlocked
	for _, id := range user.UnlockedDramas {
		if id == dramaID {
			return false, user.CoinsBalance, errors.New("already_unlocked")
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
		return false, user.CoinsBalance, errors.New("drama_free")
	}

	// Check sufficient balance
	unlockCost := models.DramaUnlockCost
	if user.CoinsBalance < unlockCost {
		return false, user.CoinsBalance, errors.New("insufficient_funds")
	}

	// Update user balance and unlocked dramas
	newUnlocked := append(user.UnlockedDramas, dramaID)
	newBalance := user.CoinsBalance - unlockCost

	query = `
		UPDATE users 
		SET coins_balance = $1, unlocked_dramas = $2, updated_at = $3 
		WHERE uid = $4`
	_, err = tx.ExecContext(ctx, query, newBalance, models.StringSlice(newUnlocked), time.Now(), userID)
	if err != nil {
		return false, 0, err
	}

	// Update wallet
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
		BalanceBefore: user.CoinsBalance,
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

	query := `
		UPDATE episodes SET 
			episode_number = :episode_number, 
			episode_title = :episode_title,
			thumbnail_url = :thumbnail_url, 
			video_url = :video_url, 
			video_duration = :video_duration,
			updated_at = :updated_at
		WHERE episode_id = :episode_id`

	_, err := s.db.NamedExecContext(ctx, query, episode)
	return err
}

func (s *DramaService) DeleteEpisode(ctx context.Context, episodeID string) error {
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Get drama ID first for updating total count
	var dramaID string
	err = tx.QueryRowContext(ctx, "SELECT drama_id FROM episodes WHERE episode_id = $1", episodeID).Scan(&dramaID)
	if err != nil {
		return err
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
