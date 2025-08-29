// ===============================
// internal/services/drama.go - UNIFIED ARCHITECTURE
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
			is_featured, is_active, created_by, created_at, updated_at
		) VALUES (
			:drama_id, :title, :description, :banner_image, :episode_videos,
			:is_premium, :free_episodes_count, :view_count, :favorite_count,
			:is_featured, :is_active, :created_by, :created_at, :updated_at
		)`

	_, err := s.db.NamedExecContext(ctx, query, drama)
	return drama.DramaID, err
}

// ===============================
// DRAMA UPDATE AND DELETE
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
		WHERE drama_id = :drama_id`

	result, err := s.db.NamedExecContext(ctx, query, drama)
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
// DRAMA STATUS TOGGLES
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
// EPISODE-LIKE OPERATIONS (for compatibility)
// ===============================

// GetDramaEpisodes returns episodes as Episode structs for frontend compatibility
func (s *DramaService) GetDramaEpisodes(ctx context.Context, dramaID string) ([]models.Episode, error) {
	drama, err := s.GetDrama(ctx, dramaID)
	if err != nil {
		return nil, err
	}

	return drama.GetAllEpisodes(), nil
}

// GetEpisode returns a specific episode by drama ID and episode number
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
// DRAMA UNLOCK OPERATION
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

	// Check sufficient balance
	unlockCost := models.DramaUnlockCost
	if currentBalance < unlockCost {
		return false, currentBalance, errors.New("insufficient_funds")
	}

	// Update user unlocked dramas
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

	// Update wallet balance
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
		"dramaId":      dramaID,
		"dramaTitle":   drama.Title,
		"unlockType":   "full_drama",
		"episodeCount": fmt.Sprintf("%d", len(drama.EpisodeVideos)),
	}

	transaction := models.WalletTransaction{
		TransactionID: transactionID,
		WalletID:      userID,
		UserID:        userID,
		Type:          "drama_unlock",
		CoinAmount:    unlockCost,
		BalanceBefore: currentBalance,
		BalanceAfter:  newBalance,
		Description:   fmt.Sprintf("Unlocked: %s (%d episodes)", drama.Title, len(drama.EpisodeVideos)),
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
