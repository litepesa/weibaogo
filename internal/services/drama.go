// ===============================
// internal/services/drama.go - Drama Service for Video Social Media App
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
// OWNERSHIP VERIFICATION (Verified Users Only)
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

// CheckUserCanCreateDrama verifies if user is verified and can create dramas
func (s *DramaService) CheckUserCanCreateDrama(ctx context.Context, userID string) (bool, error) {
	var isVerified bool
	var isActive bool
	query := `SELECT is_verified, is_active FROM users WHERE uid = $1`
	err := s.db.QueryRowContext(ctx, query, userID).Scan(&isVerified, &isActive)
	if err != nil {
		return false, err
	}
	return isVerified && isActive, nil
}

// ===============================
// CORE DRAMA OPERATIONS
// ===============================

func (s *DramaService) GetDramas(ctx context.Context, params models.DramaSearchParams) ([]models.Drama, error) {
	query := `
		SELECT * FROM dramas 
		WHERE is_active = true`
	args := []interface{}{}
	argIndex := 1

	// Add filters
	if params.UserID != "" {
		query += fmt.Sprintf(" AND created_by = $%d", argIndex)
		args = append(args, params.UserID)
		argIndex++
	}

	if params.Premium != nil {
		query += fmt.Sprintf(" AND is_premium = $%d", argIndex)
		args = append(args, *params.Premium)
		argIndex++
	}

	if params.Featured != nil {
		query += fmt.Sprintf(" AND is_featured = $%d", argIndex)
		args = append(args, *params.Featured)
		argIndex++
	}

	if params.Query != "" {
		query += fmt.Sprintf(" AND (title ILIKE $%d OR description ILIKE $%d)", argIndex, argIndex)
		searchPattern := "%" + params.Query + "%"
		args = append(args, searchPattern)
		argIndex++
	}

	// Add sorting
	switch params.SortBy {
	case models.DramaSortByPopular:
		query += " ORDER BY view_count DESC, favorite_count DESC, created_at DESC"
	case models.DramaSortByTrending:
		query += " ORDER BY (view_count + favorite_count * 2 + unlock_count * 5) DESC, created_at DESC"
	case models.DramaSortByRevenue:
		query += " ORDER BY unlock_count DESC, view_count DESC, created_at DESC"
	default: // latest
		query += " ORDER BY created_at DESC"
	}

	// Add pagination
	if params.Limit <= 0 {
		params.Limit = 20
	}
	if params.Limit > 100 {
		params.Limit = 100
	}

	query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", argIndex, argIndex+1)
	args = append(args, params.Limit, params.Offset)

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
		ORDER BY (view_count + favorite_count * 2 + unlock_count * 5) DESC, 
		created_at DESC 
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

// Get dramas created by verified users (for admin dashboard)
func (s *DramaService) GetDramasByVerifiedUser(ctx context.Context, userID string) ([]models.Drama, error) {
	// First verify user is verified
	canCreate, err := s.CheckUserCanCreateDrama(ctx, userID)
	if err != nil {
		return nil, err
	}
	if !canCreate {
		return nil, errors.New("user_not_verified")
	}

	query := `
		SELECT * FROM dramas 
		WHERE created_by = $1 
		ORDER BY created_at DESC`

	var dramas []models.Drama
	err = s.db.SelectContext(ctx, &dramas, query, userID)
	return dramas, err
}

// ===============================
// DRAMA CREATION (Verified Users Only)
// ===============================

func (s *DramaService) CreateDramaWithEpisodes(ctx context.Context, drama *models.Drama) (string, error) {
	// Verify user can create dramas (is verified)
	canCreate, err := s.CheckUserCanCreateDrama(ctx, drama.CreatedBy)
	if err != nil {
		return "", err
	}
	if !canCreate {
		return "", errors.New("user_not_verified_to_create_dramas")
	}

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
	drama.UnlockCount = 0

	query := `
		INSERT INTO dramas (
			drama_id, title, description, banner_image, episode_videos,
			is_premium, free_episodes_count, view_count, favorite_count, unlock_count,
			is_featured, is_active, created_by, created_at, updated_at
		) VALUES (
			:drama_id, :title, :description, :banner_image, :episode_videos,
			:is_premium, :free_episodes_count, :view_count, :favorite_count, :unlock_count,
			:is_featured, :is_active, :created_by, :created_at, :updated_at
		)`

	_, err = s.db.NamedExecContext(ctx, query, drama)
	return drama.DramaID, err
}

// ===============================
// DRAMA UPDATE AND DELETE - WITH OWNERSHIP VERIFICATION
// ===============================

func (s *DramaService) UpdateDrama(ctx context.Context, drama *models.Drama) error {
	// Check ownership
	hasAccess, err := s.CheckDramaOwnership(ctx, drama.DramaID, drama.CreatedBy)
	if err != nil {
		return err
	}
	if !hasAccess {
		return errors.New("drama_not_found_or_no_access")
	}

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

func (s *DramaService) DeleteDrama(ctx context.Context, dramaID, userID string) error {
	// Check ownership
	hasAccess, err := s.CheckDramaOwnership(ctx, dramaID, userID)
	if err != nil {
		return err
	}
	if !hasAccess {
		return errors.New("drama_not_found_or_no_access")
	}

	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Delete from user favorites, unlocked dramas, and drama progress
	_, err = tx.ExecContext(ctx, `
		UPDATE users 
		SET favorite_dramas = array_remove(favorite_dramas, $1),
		    unlocked_dramas = array_remove(unlocked_dramas, $1),
		    drama_progress = drama_progress - $1,
		    updated_at = $2
		WHERE $1 = ANY(favorite_dramas) OR $1 = ANY(unlocked_dramas) OR drama_progress ? $1`,
		dramaID, time.Now())
	if err != nil {
		return err
	}

	// Delete drama progress records
	_, err = tx.ExecContext(ctx, "DELETE FROM user_drama_progress WHERE drama_id = $1", dramaID)
	if err != nil {
		return err
	}

	// Delete drama analytics
	_, err = tx.ExecContext(ctx, "DELETE FROM drama_analytics WHERE drama_id = $1", dramaID)
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

func (s *DramaService) ToggleFeatured(ctx context.Context, dramaID, userID string, isFeatured bool) error {
	// Check ownership
	hasAccess, err := s.CheckDramaOwnership(ctx, dramaID, userID)
	if err != nil {
		return err
	}
	if !hasAccess {
		return errors.New("drama_not_found_or_no_access")
	}

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

func (s *DramaService) ToggleActive(ctx context.Context, dramaID, userID string, isActive bool) error {
	// Check ownership
	hasAccess, err := s.CheckDramaOwnership(ctx, dramaID, userID)
	if err != nil {
		return err
	}
	if !hasAccess {
		return errors.New("drama_not_found_or_no_access")
	}

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

	query := `UPDATE dramas SET view_count = view_count + 1, updated_at = $1 WHERE drama_id = $2 AND is_active = true`
	s.db.ExecContext(ctx, query, time.Now(), dramaID)
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

// GetEpisodeWithUserContext returns episode with user unlock context
func (s *DramaService) GetEpisodeWithUserContext(ctx context.Context, dramaID string, episodeNumber int, userID string) (*models.Episode, error) {
	drama, err := s.GetDrama(ctx, dramaID)
	if err != nil {
		return nil, err
	}

	// Check if user has unlocked the drama
	hasUnlocked, err := s.CheckUserHasUnlockedDrama(ctx, userID, dramaID)
	if err != nil {
		return nil, err
	}

	episode := drama.GetEpisodeWithUserContext(episodeNumber, hasUnlocked)
	if episode == nil {
		return nil, errors.New("episode_not_found")
	}

	return episode, nil
}

// ===============================
// USER DRAMA INTERACTIONS
// ===============================

// CheckUserHasUnlockedDrama checks if user has unlocked a premium drama
func (s *DramaService) CheckUserHasUnlockedDrama(ctx context.Context, userID, dramaID string) (bool, error) {
	var hasUnlocked bool
	query := `SELECT $1 = ANY(unlocked_dramas) FROM users WHERE uid = $2`
	err := s.db.QueryRowContext(ctx, query, dramaID, userID).Scan(&hasUnlocked)
	return hasUnlocked, err
}

// ToggleDramaFavorite adds/removes drama from user favorites
func (s *DramaService) ToggleDramaFavorite(ctx context.Context, userID, dramaID string, isAdding bool) error {
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if isAdding {
		// Add to favorites
		_, err = tx.ExecContext(ctx, `
			UPDATE users 
			SET favorite_dramas = array_append(favorite_dramas, $1),
				updated_at = $2
			WHERE uid = $3 AND NOT ($1 = ANY(favorite_dramas))`,
			dramaID, time.Now(), userID)
	} else {
		// Remove from favorites
		_, err = tx.ExecContext(ctx, `
			UPDATE users 
			SET favorite_dramas = array_remove(favorite_dramas, $1),
				updated_at = $2
			WHERE uid = $3`,
			dramaID, time.Now(), userID)
	}

	if err != nil {
		return err
	}

	// Update drama favorite count
	err = s.IncrementDramaFavorites(ctx, dramaID, isAdding)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// UpdateDramaProgress updates user's progress through a drama
func (s *DramaService) UpdateDramaProgress(ctx context.Context, userID, dramaID string, episodeNumber int) error {
	// First update the user's drama_progress JSONB field
	_, err := s.db.ExecContext(ctx, `
		UPDATE users 
		SET drama_progress = jsonb_set(
			COALESCE(drama_progress, '{}'),
			$1,
			$2::jsonb,
			true
		),
		updated_at = $3
		WHERE uid = $4`,
		fmt.Sprintf(`{"%s"}`, dramaID), fmt.Sprintf(`%d`, episodeNumber), time.Now(), userID)

	if err != nil {
		return err
	}

	// Then upsert the detailed progress record
	query := `
		INSERT INTO user_drama_progress (user_id, drama_id, current_episode, last_watched_at, updated_at)
		VALUES ($1, $2, $3, $4, $4)
		ON CONFLICT (user_id, drama_id) 
		DO UPDATE SET 
			current_episode = GREATEST(user_drama_progress.current_episode, $3),
			last_watched_at = $4,
			updated_at = $4`

	_, err = s.db.ExecContext(ctx, query, userID, dramaID, episodeNumber, time.Now())
	return err
}

// GetUserDramaProgress gets user's progress for a specific drama
func (s *DramaService) GetUserDramaProgress(ctx context.Context, userID, dramaID string) (*models.UserDramaProgress, error) {
	var progress models.UserDramaProgress
	query := `SELECT * FROM user_drama_progress WHERE user_id = $1 AND drama_id = $2`
	err := s.db.GetContext(ctx, &progress, query, userID, dramaID)
	return &progress, err
}

// GetUserFavoriteDramas gets user's favorite dramas
func (s *DramaService) GetUserFavoriteDramas(ctx context.Context, userID string, limit, offset int) ([]models.Drama, error) {
	query := `
		SELECT d.* FROM dramas d
		JOIN users u ON u.uid = $1
		WHERE d.drama_id = ANY(
			SELECT unnest(u.favorite_dramas)
		) AND d.is_active = true
		ORDER BY d.created_at DESC
		LIMIT $2 OFFSET $3`

	var dramas []models.Drama
	err := s.db.SelectContext(ctx, &dramas, query, userID, limit, offset)
	return dramas, err
}

// GetUserContinueWatchingDramas gets dramas user has progress on
func (s *DramaService) GetUserContinueWatchingDramas(ctx context.Context, userID string, limit int) ([]models.Drama, error) {
	query := `
		SELECT d.* FROM dramas d
		JOIN user_drama_progress udp ON d.drama_id = udp.drama_id
		WHERE udp.user_id = $1 AND d.is_active = true AND udp.completed = false
		ORDER BY udp.last_watched_at DESC
		LIMIT $2`

	var dramas []models.Drama
	err := s.db.SelectContext(ctx, &dramas, query, userID, limit)
	return dramas, err
}

// ===============================
// DRAMA UNLOCK OPERATION - WITH UNLOCK COUNT TRACKING
// ===============================

func (s *DramaService) UnlockDrama(ctx context.Context, userID, dramaID string) (bool, int, error) {
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return false, 0, err
	}
	defer tx.Rollback()

	// Get user unlocked dramas
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

	// Increment unlock count in dramas table for revenue tracking
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
		"unlockCost":   unlockCost,
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
			transaction_id, wallet_id, user_id, type, coin_amount, 
			balance_before, balance_after, description, reference_id, metadata, created_at
		) VALUES (
			:transaction_id, :wallet_id, :user_id, :type, :coin_amount,
			:balance_before, :balance_after, :description, :reference_id, :metadata, :created_at
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
// REVENUE ANALYTICS - BASED ON ACTUAL UNLOCK COUNT
// ===============================

// GetDramaRevenue returns exact revenue for a specific drama
func (s *DramaService) GetDramaRevenue(ctx context.Context, dramaID string) (int, error) {
	var unlockCount int
	query := `SELECT unlock_count FROM dramas WHERE drama_id = $1`
	err := s.db.QueryRowContext(ctx, query, dramaID).Scan(&unlockCount)
	if err != nil {
		return 0, err
	}
	return unlockCount * models.DramaUnlockCost, nil
}

// GetVerifiedUserDramasWithRevenue returns dramas with revenue data for verified user
func (s *DramaService) GetVerifiedUserDramasWithRevenue(ctx context.Context, userID string) ([]models.DramaPerformance, error) {
	query := `
		SELECT drama_id, title, array_length(episode_videos, 1) as total_episodes, 
		       view_count, favorite_count, unlock_count, is_premium, created_at
		FROM dramas 
		WHERE created_by = $1 
		ORDER BY created_at DESC`

	rows, err := s.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var performances []models.DramaPerformance
	for rows.Next() {
		var p models.DramaPerformance
		var totalEpisodes *int

		err := rows.Scan(
			&p.DramaID, &p.Title, &totalEpisodes,
			&p.ViewCount, &p.FavoriteCount, &p.UnlockCount,
			&p.IsPremium, &p.CreatedAt,
		)
		if err != nil {
			return nil, err
		}

		if totalEpisodes != nil {
			p.TotalEpisodes = *totalEpisodes
		}

		// Calculate exact revenue from unlock count
		p.Revenue = p.UnlockCount * models.DramaUnlockCost

		// Calculate conversion rate (views to unlocks)
		if p.ViewCount > 0 {
			p.ConversionRate = (float64(p.UnlockCount) / float64(p.ViewCount)) * 100.0
		}

		performances = append(performances, p)
	}

	return performances, nil
}

// GetTotalVerifiedUserRevenue returns total revenue for all user's dramas
func (s *DramaService) GetTotalVerifiedUserRevenue(ctx context.Context, userID string) (int, error) {
	var totalUnlocks int
	query := `
		SELECT COALESCE(SUM(unlock_count), 0) 
		FROM dramas 
		WHERE created_by = $1 AND is_premium = true`

	err := s.db.QueryRowContext(ctx, query, userID).Scan(&totalUnlocks)
	if err != nil {
		return 0, err
	}

	return totalUnlocks * models.DramaUnlockCost, nil
}
