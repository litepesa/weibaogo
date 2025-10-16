// ===============================
// internal/services/video.go - SIMPLIFIED with Fuzzy Search + History
// ===============================

package services

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"weibaobe/internal/models"
	"weibaobe/internal/storage"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type VideoService struct {
	db       *sqlx.DB
	r2Client *storage.R2Client
}

func NewVideoService(db *sqlx.DB, r2Client *storage.R2Client) *VideoService {
	return &VideoService{
		db:       db,
		r2Client: r2Client,
	}
}

// ===============================
// URL OPTIMIZATION HELPERS
// ===============================

func (s *VideoService) optimizeVideoURL(url string) string {
	if url == "" {
		return url
	}

	if strings.Contains(url, "cloudflare.com") || strings.Contains(url, "r2.cloudflarestorage.com") {
		if !strings.Contains(url, "?") {
			return url + "?cf_optimize=true"
		}
		return url + "&cf_optimize=true"
	}

	if !strings.Contains(url, "?") {
		return url + "?stream=true"
	}
	return url + "&stream=true"
}

func (s *VideoService) optimizeThumbnailURL(url string) string {
	if url == "" {
		return url
	}

	if strings.Contains(url, "cloudflare.com") {
		if !strings.Contains(url, "?") {
			return url + "?format=webp&quality=85&width=640"
		}
		return url + "&format=webp&quality=85&width=640"
	}

	return url
}

func (s *VideoService) applyURLOptimizations(video *models.VideoResponse) {
	video.VideoURL = s.optimizeVideoURL(video.VideoURL)
	video.ThumbnailURL = s.optimizeThumbnailURL(video.ThumbnailURL)
	video.UserImage = s.optimizeThumbnailURL(video.UserImage)
	video.UserProfileImage = s.optimizeThumbnailURL(video.UserProfileImage)

	for i, imageURL := range video.ImageUrls {
		video.ImageUrls[i] = s.optimizeThumbnailURL(imageURL)
	}
}

// ===============================
// ROLE-BASED VALIDATION HELPERS
// ===============================

func (s *VideoService) ValidateUserCanCreateVideo(ctx context.Context, userID string) (*models.User, error) {
	var user models.User
	query := `
		SELECT uid, name, phone_number, whatsapp_number, profile_image, bio, 
		       user_type, role, is_verified, is_active, created_at
		FROM users 
		WHERE uid = $1`

	err := s.db.QueryRowContext(ctx, query, userID).Scan(
		&user.UID, &user.Name, &user.PhoneNumber, &user.WhatsappNumber,
		&user.ProfileImage, &user.Bio, &user.UserType, &user.Role,
		&user.IsVerified, &user.IsActive, &user.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("user not found: %s", userID)
	}

	if !user.IsActive {
		return nil, fmt.Errorf("user account is inactive")
	}

	return &user, nil
}

// ===============================
// SIMPLIFIED FUZZY SEARCH
// ===============================

// FuzzySearch - Simple fuzzy search across username, caption, and tags
func (s *VideoService) FuzzySearch(ctx context.Context, query string, usernameOnly bool, limit, offset int) ([]models.VideoResponse, int, error) {
	startTime := time.Now()

	// Sanitize query
	cleanQuery := strings.TrimSpace(query)
	if cleanQuery == "" {
		return []models.VideoResponse{}, 0, nil
	}

	log.Printf("Fuzzy search: query='%s', usernameOnly=%v", cleanQuery, usernameOnly)

	// Build search pattern for fuzzy matching
	searchPattern := "%" + strings.ToLower(cleanQuery) + "%"

	var searchQuery string
	var args []interface{}

	if usernameOnly {
		// Search ONLY in username
		searchQuery = `
			SELECT v.id, v.user_id, v.user_name, v.user_image, v.video_url, v.thumbnail_url,
			       v.caption, v.price, v.likes_count, v.comments_count, v.views_count, v.shares_count,
			       v.tags, v.is_active, v.is_featured, v.is_verified, v.is_multiple_images, v.image_urls,
			       v.created_at, v.updated_at,
			       similarity(v.user_name, $1) as relevance
			FROM videos v
			WHERE v.is_active = true
			  AND (LOWER(v.user_name) LIKE $2 OR v.user_name % $1)
			ORDER BY relevance DESC, v.created_at DESC
			LIMIT $3 OFFSET $4`

		args = []interface{}{cleanQuery, searchPattern, limit, offset}
	} else {
		// Search in username, caption, AND tags (fuzzy matching)
		searchQuery = `
			SELECT v.id, v.user_id, v.user_name, v.user_image, v.video_url, v.thumbnail_url,
			       v.caption, v.price, v.likes_count, v.comments_count, v.views_count, v.shares_count,
			       v.tags, v.is_active, v.is_featured, v.is_verified, v.is_multiple_images, v.image_urls,
			       v.created_at, v.updated_at,
			       GREATEST(
			         similarity(v.user_name, $1),
			         similarity(v.caption, $1),
			         CASE 
			           WHEN array_to_string(v.tags, ' ') % $1 THEN 0.7
			           ELSE 0.0
			         END
			       ) as relevance
			FROM videos v
			WHERE v.is_active = true
			  AND (
			    LOWER(v.user_name) LIKE $2 OR v.user_name % $1 OR
			    LOWER(v.caption) LIKE $2 OR v.caption % $1 OR
			    LOWER(array_to_string(v.tags, ' ')) LIKE $2
			  )
			ORDER BY relevance DESC, v.created_at DESC
			LIMIT $3 OFFSET $4`

		args = []interface{}{cleanQuery, searchPattern, limit, offset}
	}

	log.Printf("Executing query with pattern: %s", searchPattern)

	rows, err := s.db.QueryContext(ctx, searchQuery, args...)
	if err != nil {
		log.Printf("Search query failed: %v", err)
		return nil, 0, err
	}
	defer rows.Close()

	var videos []models.VideoResponse
	for rows.Next() {
		var video models.VideoResponse
		var relevance float64

		err := rows.Scan(
			&video.ID, &video.UserID, &video.UserName, &video.UserImage,
			&video.VideoURL, &video.ThumbnailURL, &video.Caption, &video.Price,
			&video.LikesCount, &video.CommentsCount, &video.ViewsCount, &video.SharesCount,
			&video.Tags, &video.IsActive, &video.IsFeatured, &video.IsVerified,
			&video.IsMultipleImages, &video.ImageUrls, &video.CreatedAt, &video.UpdatedAt,
			&relevance,
		)
		if err != nil {
			log.Printf("Row scan error: %v", err)
			continue
		}

		// Apply URL optimizations
		s.applyURLOptimizations(&video)
		video.UserProfileImage = video.UserImage

		videos = append(videos, video)
	}

	if err = rows.Err(); err != nil {
		log.Printf("Rows iteration error: %v", err)
		return videos, len(videos), err
	}

	duration := time.Since(startTime).Milliseconds()
	log.Printf("Fuzzy search completed: %d results in %dms", len(videos), duration)

	return videos, len(videos), nil
}

// ===============================
// SEARCH HISTORY MANAGEMENT
// ===============================

// GetSearchHistory retrieves user's search history
func (s *VideoService) GetSearchHistory(ctx context.Context, userID string, limit int) ([]string, error) {
	query := `
		SELECT DISTINCT query
		FROM search_history
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2`

	rows, err := s.db.QueryContext(ctx, query, userID, limit)
	if err != nil {
		log.Printf("Failed to get search history: %v", err)
		return []string{}, nil // Return empty array instead of error
	}
	defer rows.Close()

	var history []string
	for rows.Next() {
		var query string
		if err := rows.Scan(&query); err != nil {
			continue
		}
		history = append(history, query)
	}

	log.Printf("Retrieved %d search history items for user %s", len(history), userID)
	return history, nil
}

// AddSearchHistory adds a search query to user's history
func (s *VideoService) AddSearchHistory(ctx context.Context, userID, query string) error {
	cleanQuery := strings.TrimSpace(query)
	if cleanQuery == "" {
		return fmt.Errorf("query cannot be empty")
	}

	// Check if table exists, if not create it
	_, err := s.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS search_history (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id VARCHAR(255) NOT NULL,
			query TEXT NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			CONSTRAINT fk_user FOREIGN KEY (user_id) REFERENCES users(uid) ON DELETE CASCADE
		)`)
	if err != nil {
		log.Printf("Failed to create search_history table: %v", err)
	}

	// Create index if not exists
	_, err = s.db.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_search_history_user_id ON search_history(user_id, created_at DESC)`)
	if err != nil {
		log.Printf("Failed to create search_history index: %v", err)
	}

	// Remove duplicate if exists
	_, err = s.db.ExecContext(ctx, `
		DELETE FROM search_history 
		WHERE user_id = $1 AND LOWER(query) = LOWER($2)`,
		userID, cleanQuery)
	if err != nil {
		log.Printf("Warning: Failed to remove duplicate: %v", err)
	}

	// Insert new search
	insertQuery := `
		INSERT INTO search_history (user_id, query, created_at)
		VALUES ($1, $2, $3)`

	_, err = s.db.ExecContext(ctx, insertQuery, userID, cleanQuery, time.Now())
	if err != nil {
		log.Printf("Failed to add search history: %v", err)
		return err
	}

	// Keep only last 50 searches per user
	_, err = s.db.ExecContext(ctx, `
		DELETE FROM search_history
		WHERE id IN (
			SELECT id FROM search_history
			WHERE user_id = $1
			ORDER BY created_at DESC
			OFFSET 50
		)`, userID)
	if err != nil {
		log.Printf("Warning: Failed to cleanup old history: %v", err)
	}

	log.Printf("Added search '%s' to history for user %s", cleanQuery, userID)
	return nil
}

// ClearSearchHistory removes all search history for a user
func (s *VideoService) ClearSearchHistory(ctx context.Context, userID string) error {
	query := `DELETE FROM search_history WHERE user_id = $1`

	result, err := s.db.ExecContext(ctx, query, userID)
	if err != nil {
		log.Printf("Failed to clear search history: %v", err)
		return err
	}

	rowsAffected, _ := result.RowsAffected()
	log.Printf("Cleared %d search history items for user %s", rowsAffected, userID)
	return nil
}

// RemoveSearchHistory removes a specific search query from user's history
func (s *VideoService) RemoveSearchHistory(ctx context.Context, userID, searchQuery string) error {
	query := `DELETE FROM search_history WHERE user_id = $1 AND LOWER(query) = LOWER($2)`

	result, err := s.db.ExecContext(ctx, query, userID, searchQuery)
	if err != nil {
		log.Printf("Failed to remove search history: %v", err)
		return err
	}

	rowsAffected, _ := result.RowsAffected()
	log.Printf("Removed %d search history item(s) for user %s", rowsAffected, userID)
	return nil
}

// ===============================
// POPULAR SEARCH TERMS
// ===============================

func (s *VideoService) GetPopularSearchTerms(ctx context.Context, limit int) ([]string, error) {
	// Try to get from materialized view first
	query := `
		SELECT word 
		FROM popular_search_terms 
		ORDER BY frequency DESC 
		LIMIT $1`

	rows, err := s.db.QueryContext(ctx, query, limit)
	if err != nil {
		log.Printf("Materialized view not available, using fallback: %v", err)
		// Fallback to real-time query
		return s.getPopularSearchTermsFallback(ctx, limit)
	}
	defer rows.Close()

	var terms []string
	for rows.Next() {
		var term string
		if err := rows.Scan(&term); err != nil {
			continue
		}
		terms = append(terms, term)
	}

	log.Printf("Retrieved %d popular search terms", len(terms))
	return terms, nil
}

// Fallback method for popular terms
func (s *VideoService) getPopularSearchTermsFallback(ctx context.Context, limit int) ([]string, error) {
	// Get most common words from recent searches
	query := `
		SELECT query, COUNT(*) as frequency
		FROM search_history
		WHERE created_at >= NOW() - INTERVAL '7 days'
		GROUP BY query
		ORDER BY frequency DESC
		LIMIT $1`

	rows, err := s.db.QueryContext(ctx, query, limit)
	if err != nil {
		log.Printf("Fallback popular terms query failed: %v", err)
		return []string{}, nil
	}
	defer rows.Close()

	var terms []string
	for rows.Next() {
		var term string
		var frequency int
		if err := rows.Scan(&term, &frequency); err != nil {
			continue
		}
		terms = append(terms, term)
	}

	return terms, nil
}

// ===============================
// OPTIMIZED VIDEO CRUD OPERATIONS
// ===============================

func (s *VideoService) GetVideosOptimized(ctx context.Context, params models.VideoSearchParams) ([]models.VideoResponse, error) {
	query := `
		SELECT 
			v.id, v.user_id, v.user_name, v.user_image, v.video_url, v.thumbnail_url,
			v.caption, v.price, v.likes_count, v.comments_count, v.views_count, v.shares_count,
			v.tags, v.is_active, v.is_featured, v.is_verified, v.is_multiple_images, v.image_urls,
			v.created_at, v.updated_at
		FROM videos v
		WHERE v.is_active = true`

	args := []interface{}{}
	argIndex := 1

	if params.UserID != "" {
		query += fmt.Sprintf(" AND v.user_id = $%d", argIndex)
		args = append(args, params.UserID)
		argIndex++
	}

	if params.Featured != nil {
		query += fmt.Sprintf(" AND v.is_featured = $%d", argIndex)
		args = append(args, *params.Featured)
		argIndex++
	}

	if params.MediaType != "" && params.MediaType != "all" {
		if params.MediaType == "image" {
			query += " AND v.is_multiple_images = true"
		} else if params.MediaType == "video" {
			query += " AND v.is_multiple_images = false"
		}
	}

	if params.Query != "" {
		query += fmt.Sprintf(" AND (v.caption ILIKE $%d OR v.user_name ILIKE $%d)", argIndex, argIndex)
		searchPattern := "%" + params.Query + "%"
		args = append(args, searchPattern)
		argIndex++
	}

	// Sorting
	switch params.SortBy {
	case "popular":
		query += " ORDER BY v.likes_count DESC, v.views_count DESC, v.created_at DESC"
	case "trending":
		query += ` ORDER BY (
			CASE 
				WHEN EXTRACT(EPOCH FROM (NOW() - v.created_at)) > 0 THEN
					(v.likes_count * 2.5 + v.comments_count * 3.5 + v.shares_count * 5.0 + v.views_count * 0.1) 
					/ POWER(EXTRACT(EPOCH FROM (NOW() - v.created_at))/3600 + 1, 1.8)
				ELSE v.likes_count * 2.5 + v.comments_count * 3.5 + v.shares_count * 5.0 
			END
		) DESC, v.created_at DESC`
	case "views":
		query += " ORDER BY v.views_count DESC, v.created_at DESC"
	case "likes":
		query += " ORDER BY v.likes_count DESC, v.created_at DESC"
	default:
		query += " ORDER BY v.created_at DESC"
	}

	if params.Limit <= 0 {
		params.Limit = 20
	}
	if params.Limit > 100 {
		params.Limit = 100
	}

	query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", argIndex, argIndex+1)
	args = append(args, params.Limit, params.Offset)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var videos []models.VideoResponse
	for rows.Next() {
		var video models.VideoResponse

		err := rows.Scan(
			&video.ID, &video.UserID, &video.UserName, &video.UserImage,
			&video.VideoURL, &video.ThumbnailURL, &video.Caption, &video.Price,
			&video.LikesCount, &video.CommentsCount, &video.ViewsCount, &video.SharesCount,
			&video.Tags, &video.IsActive, &video.IsFeatured, &video.IsVerified,
			&video.IsMultipleImages, &video.ImageUrls, &video.CreatedAt, &video.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		s.applyURLOptimizations(&video)
		video.UserProfileImage = video.UserImage
		video.IsLiked = false
		video.IsFollowing = false

		videos = append(videos, video)
	}

	return videos, nil
}

func (s *VideoService) GetVideosBulk(ctx context.Context, videoIDs []string, includeInactive bool) ([]models.VideoResponse, error) {
	if len(videoIDs) == 0 {
		return []models.VideoResponse{}, nil
	}

	query := `
		SELECT 
			v.id, v.user_id, v.user_name, v.user_image, v.video_url, v.thumbnail_url,
			v.caption, v.price, v.likes_count, v.comments_count, v.views_count, v.shares_count,
			v.tags, v.is_active, v.is_featured, v.is_verified, v.is_multiple_images, v.image_urls,
			v.created_at, v.updated_at
		FROM videos v
		WHERE v.id = ANY($1::text[])`

	if !includeInactive {
		query += " AND v.is_active = true"
	}

	query += " ORDER BY v.created_at DESC"

	rows, err := s.db.QueryContext(ctx, query, videoIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var videos []models.VideoResponse
	for rows.Next() {
		var video models.VideoResponse

		err := rows.Scan(
			&video.ID, &video.UserID, &video.UserName, &video.UserImage,
			&video.VideoURL, &video.ThumbnailURL, &video.Caption, &video.Price,
			&video.LikesCount, &video.CommentsCount, &video.ViewsCount, &video.SharesCount,
			&video.Tags, &video.IsActive, &video.IsFeatured, &video.IsVerified,
			&video.IsMultipleImages, &video.ImageUrls, &video.CreatedAt, &video.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		s.applyURLOptimizations(&video)
		video.UserProfileImage = video.UserImage

		videos = append(videos, video)
	}

	return videos, nil
}

func (s *VideoService) GetFeaturedVideosOptimized(ctx context.Context, limit int) ([]models.VideoResponse, error) {
	query := `
		SELECT 
			v.id, v.user_id, v.user_name, v.user_image, v.video_url, v.thumbnail_url,
			v.caption, v.price, v.likes_count, v.comments_count, v.views_count, v.shares_count,
			v.tags, v.is_active, v.is_featured, v.is_verified, v.is_multiple_images, v.image_urls,
			v.created_at, v.updated_at
		FROM videos v
		WHERE v.is_active = true AND v.is_featured = true
		ORDER BY v.created_at DESC 
		LIMIT $1`

	rows, err := s.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var videos []models.VideoResponse
	for rows.Next() {
		var video models.VideoResponse

		err := rows.Scan(
			&video.ID, &video.UserID, &video.UserName, &video.UserImage,
			&video.VideoURL, &video.ThumbnailURL, &video.Caption, &video.Price,
			&video.LikesCount, &video.CommentsCount, &video.ViewsCount, &video.SharesCount,
			&video.Tags, &video.IsActive, &video.IsFeatured, &video.IsVerified,
			&video.IsMultipleImages, &video.ImageUrls, &video.CreatedAt, &video.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		s.applyURLOptimizations(&video)
		video.UserProfileImage = video.UserImage

		videos = append(videos, video)
	}

	return videos, nil
}

func (s *VideoService) GetTrendingVideosOptimized(ctx context.Context, limit int) ([]models.VideoResponse, error) {
	query := `
		SELECT 
			v.id, v.user_id, v.user_name, v.user_image, v.video_url, v.thumbnail_url,
			v.caption, v.price, v.likes_count, v.comments_count, v.views_count, v.shares_count,
			v.tags, v.is_active, v.is_featured, v.is_verified, v.is_multiple_images, v.image_urls,
			v.created_at, v.updated_at,
			CASE 
				WHEN EXTRACT(EPOCH FROM (NOW() - v.created_at)) > 0 THEN
					(v.likes_count * 2.5 + v.comments_count * 3.5 + v.shares_count * 5.0 + v.views_count * 0.1) 
					/ POWER(EXTRACT(EPOCH FROM (NOW() - v.created_at))/3600 + 1, 1.8)
				ELSE v.likes_count * 2.5 + v.comments_count * 3.5 + v.shares_count * 5.0 
			END as trending_score
		FROM videos v
		WHERE v.is_active = true
		ORDER BY trending_score DESC, v.created_at DESC 
		LIMIT $1`

	rows, err := s.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var videos []models.VideoResponse
	for rows.Next() {
		var video models.VideoResponse
		var trendingScore float64

		err := rows.Scan(
			&video.ID, &video.UserID, &video.UserName, &video.UserImage,
			&video.VideoURL, &video.ThumbnailURL, &video.Caption, &video.Price,
			&video.LikesCount, &video.CommentsCount, &video.ViewsCount, &video.SharesCount,
			&video.Tags, &video.IsActive, &video.IsFeatured, &video.IsVerified,
			&video.IsMultipleImages, &video.ImageUrls, &video.CreatedAt, &video.UpdatedAt,
			&trendingScore,
		)
		if err != nil {
			return nil, err
		}

		s.applyURLOptimizations(&video)
		video.UserProfileImage = video.UserImage

		videos = append(videos, video)
	}

	return videos, nil
}

func (s *VideoService) GetVideoOptimized(ctx context.Context, videoID string) (*models.VideoResponse, error) {
	query := `
		SELECT 
			v.id, v.user_id, v.user_name, v.user_image, v.video_url, v.thumbnail_url,
			v.caption, v.price, v.likes_count, v.comments_count, v.views_count, v.shares_count,
			v.tags, v.is_active, v.is_featured, v.is_verified, v.is_multiple_images, v.image_urls,
			v.created_at, v.updated_at
		FROM videos v
		WHERE v.id = $1 AND v.is_active = true`

	var video models.VideoResponse

	err := s.db.QueryRowContext(ctx, query, videoID).Scan(
		&video.ID, &video.UserID, &video.UserName, &video.UserImage,
		&video.VideoURL, &video.ThumbnailURL, &video.Caption, &video.Price,
		&video.LikesCount, &video.CommentsCount, &video.ViewsCount, &video.SharesCount,
		&video.Tags, &video.IsActive, &video.IsFeatured, &video.IsVerified,
		&video.IsMultipleImages, &video.ImageUrls, &video.CreatedAt, &video.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	s.applyURLOptimizations(&video)
	video.UserProfileImage = video.UserImage

	// Async view increment
	go func() {
		s.incrementViewCountOptimized(videoID)
	}()

	video.ViewsCount++

	return &video, nil
}

func (s *VideoService) GetUserVideosOptimized(ctx context.Context, userID string, limit, offset int) ([]models.VideoResponse, error) {
	query := `
		SELECT 
			v.id, v.user_id, v.user_name, v.user_image, v.video_url, v.thumbnail_url,
			v.caption, v.price, v.likes_count, v.comments_count, v.views_count, v.shares_count,
			v.tags, v.is_active, v.is_featured, v.is_verified, v.is_multiple_images, v.image_urls,
			v.created_at, v.updated_at
		FROM videos v
		WHERE v.user_id = $1 AND v.is_active = true
		ORDER BY v.created_at DESC 
		LIMIT $2 OFFSET $3`

	rows, err := s.db.QueryContext(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var videos []models.VideoResponse
	for rows.Next() {
		var video models.VideoResponse

		err := rows.Scan(
			&video.ID, &video.UserID, &video.UserName, &video.UserImage,
			&video.VideoURL, &video.ThumbnailURL, &video.Caption, &video.Price,
			&video.LikesCount, &video.CommentsCount, &video.ViewsCount, &video.SharesCount,
			&video.Tags, &video.IsActive, &video.IsFeatured, &video.IsVerified,
			&video.IsMultipleImages, &video.ImageUrls, &video.CreatedAt, &video.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		s.applyURLOptimizations(&video)
		video.UserProfileImage = video.UserImage

		videos = append(videos, video)
	}

	return videos, nil
}

func (s *VideoService) GetUserLikedVideosOptimized(ctx context.Context, userID string, limit, offset int) ([]models.VideoResponse, error) {
	query := `
		SELECT v.id, v.user_id, v.user_name, v.user_image, v.video_url, v.thumbnail_url,
		       v.caption, v.price, v.likes_count, v.comments_count, v.views_count, v.shares_count,
		       v.tags, v.is_active, v.is_featured, v.is_verified, v.is_multiple_images, v.image_urls,
		       v.created_at, v.updated_at
		FROM videos v
		JOIN video_likes vl ON v.id = vl.video_id
		WHERE vl.user_id = $1 AND v.is_active = true
		ORDER BY vl.created_at DESC
		LIMIT $2 OFFSET $3`

	rows, err := s.db.QueryContext(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var videos []models.VideoResponse
	for rows.Next() {
		var video models.VideoResponse

		err := rows.Scan(
			&video.ID, &video.UserID, &video.UserName, &video.UserImage,
			&video.VideoURL, &video.ThumbnailURL, &video.Caption, &video.Price,
			&video.LikesCount, &video.CommentsCount, &video.ViewsCount, &video.SharesCount,
			&video.Tags, &video.IsActive, &video.IsFeatured, &video.IsVerified,
			&video.IsMultipleImages, &video.ImageUrls, &video.CreatedAt, &video.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		s.applyURLOptimizations(&video)
		video.UserProfileImage = video.UserImage
		video.IsLiked = true

		videos = append(videos, video)
	}

	return videos, nil
}

func (s *VideoService) CreateVideoOptimized(ctx context.Context, video *models.Video) (string, error) {
	user, err := s.ValidateUserCanCreateVideo(ctx, video.UserID)
	if err != nil {
		return "", fmt.Errorf("video creation validation failed: %w", err)
	}

	if !video.IsValidForCreation() {
		errors := video.ValidateForCreation()
		return "", fmt.Errorf("validation failed: %v", errors)
	}

	video.ID = uuid.New().String()
	video.CreatedAt = time.Now()
	video.UpdatedAt = time.Now()
	video.IsActive = true
	video.LikesCount = 0
	video.CommentsCount = 0
	video.ViewsCount = 0
	video.SharesCount = 0

	if video.Price < 0 {
		video.Price = 0
	}

	video.UserName = user.Name
	video.UserImage = user.ProfileImage

	video.VideoURL = s.optimizeVideoURL(video.VideoURL)
	video.ThumbnailURL = s.optimizeThumbnailURL(video.ThumbnailURL)

	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	// 🔧 FIXED: Using positional parameters instead of named parameters
	query := `
		INSERT INTO videos (
			id, user_id, user_name, user_image, video_url, thumbnail_url,
			caption, price, likes_count, comments_count, views_count, shares_count,
			tags, is_active, is_featured, is_verified, is_multiple_images, image_urls,
			created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9, $10, $11, $12,
			$13, $14, $15, $16, $17, $18,
			$19, $20
		)`

	log.Printf("🔍 ATTEMPTING VIDEO INSERT:")
	log.Printf("   Video ID: %s", video.ID)
	log.Printf("   User ID: %s", video.UserID)
	log.Printf("   User Name: %s", video.UserName)
	log.Printf("   Caption: %s", video.Caption)
	log.Printf("   Price: %f", video.Price)
	log.Printf("   Tags: %v (type: %T)", video.Tags, video.Tags)
	log.Printf("   ImageUrls: %v (type: %T)", video.ImageUrls, video.ImageUrls)
	log.Printf("   IsMultipleImages: %v", video.IsMultipleImages)
	log.Printf("   VideoURL: %s", video.VideoURL)
	log.Printf("   ThumbnailURL: %s", video.ThumbnailURL)

	_, err = tx.ExecContext(ctx, query,
		video.ID,
		video.UserID,
		video.UserName,
		video.UserImage,
		video.VideoURL,
		video.ThumbnailURL,
		video.Caption,
		video.Price,
		video.LikesCount,
		video.CommentsCount,
		video.ViewsCount,
		video.SharesCount,
		video.Tags,
		video.IsActive,
		video.IsFeatured,
		video.IsVerified,
		video.IsMultipleImages,
		video.ImageUrls,
		video.CreatedAt,
		video.UpdatedAt,
	)
	if err != nil {
		log.Printf("❌ DATABASE INSERT ERROR: %v", err)
		log.Printf("❌ Error Type: %T", err)
		log.Printf("❌ Full Error Details: %+v", err)
		log.Printf("📄 FAILED Video ID: %s", video.ID)
		log.Printf("📄 FAILED User ID: %s", video.UserID)
		log.Printf("📄 FAILED Tags: %v (type: %T)", video.Tags, video.Tags)
		log.Printf("📄 FAILED ImageUrls: %v (type: %T)", video.ImageUrls, video.ImageUrls)
		log.Printf("📄 Full Video Data: %+v", video)
		return "", fmt.Errorf("failed to insert video: %w", err)
	}

	log.Printf("✅ VIDEO INSERTED SUCCESSFULLY: %s", video.ID)

	log.Printf("🔄 UPDATING USER LAST_POST_AT for user: %s", video.UserID)
	updateTime := time.Now()
	_, err = tx.ExecContext(ctx, `
		UPDATE users 
		SET last_post_at = $1::timestamp, updated_at = $2::timestamp 
		WHERE uid = $3`,
		updateTime, updateTime, video.UserID)
	if err != nil {
		log.Printf("❌ USER UPDATE ERROR: %v", err)
		log.Printf("❌ Failed to update last_post_at for user: %s", video.UserID)
		return "", fmt.Errorf("failed to update user last post: %w", err)
	}
	log.Printf("✅ USER LAST_POST_AT UPDATED SUCCESSFULLY")

	log.Printf("🔄 COMMITTING TRANSACTION...")
	if err = tx.Commit(); err != nil {
		log.Printf("❌ TRANSACTION COMMIT ERROR: %v", err)
		return "", fmt.Errorf("failed to commit transaction: %w", err)
	}
	log.Printf("✅ TRANSACTION COMMITTED SUCCESSFULLY")

	log.Printf("🎉 VIDEO CREATION COMPLETED: %s", video.ID)
	return video.ID, nil
}

// ===============================
// VIDEO INTERACTION OPERATIONS
// ===============================

func (s *VideoService) incrementViewCountOptimized(videoID string) {
	maxRetries := 3
	for i := 0; i < maxRetries; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)

		query := `
			UPDATE videos 
			SET views_count = views_count + 1, updated_at = $1 
			WHERE id = $2 AND is_active = true 
			RETURNING views_count`

		var newCount int
		err := s.db.QueryRowContext(ctx, query, time.Now(), videoID).Scan(&newCount)
		cancel()

		if err == nil {
			return
		}

		time.Sleep(time.Duration(i+1) * 500 * time.Millisecond)
	}

	log.Printf("Failed to increment view count for video %s after %d retries", videoID, maxRetries)
}

func (s *VideoService) IncrementVideoViews(ctx context.Context, videoID string) error {
	go s.incrementViewCountOptimized(videoID)
	return nil
}

func (s *VideoService) LikeVideo(ctx context.Context, videoID, userID string) error {
	var exists int
	err := s.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM video_likes WHERE video_id = $1 AND user_id = $2",
		videoID, userID).Scan(&exists)
	if err != nil {
		return err
	}
	if exists > 0 {
		return errors.New("already_liked")
	}

	_, err = s.db.ExecContext(ctx,
		"INSERT INTO video_likes (id, video_id, user_id, created_at) VALUES ($1, $2, $3, $4)",
		uuid.New().String(), videoID, userID, time.Now())
	return err
}

func (s *VideoService) UnlikeVideo(ctx context.Context, videoID, userID string) error {
	result, err := s.db.ExecContext(ctx,
		"DELETE FROM video_likes WHERE video_id = $1 AND user_id = $2",
		videoID, userID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return errors.New("not_liked")
	}

	return nil
}

func (s *VideoService) GetVideoCountsSummary(ctx context.Context, videoID string) (*models.VideoCountsSummary, error) {
	query := `
		SELECT 
			id, views_count, likes_count, comments_count, shares_count, updated_at
		FROM videos 
		WHERE id = $1 AND is_active = true`

	var summary models.VideoCountsSummary
	err := s.db.QueryRowContext(ctx, query, videoID).Scan(
		&summary.VideoID,
		&summary.ViewsCount,
		&summary.LikesCount,
		&summary.CommentsCount,
		&summary.SharesCount,
		&summary.UpdatedAt,
	)

	return &summary, err
}

func (s *VideoService) BatchUpdateViewCounts(ctx context.Context) error {
	query := `
		WITH updated_counts AS (
			UPDATE videos SET 
				likes_count = (
					SELECT COUNT(*) 
					FROM video_likes 
					WHERE video_likes.video_id = videos.id
				),
				comments_count = (
					SELECT COUNT(*) 
					FROM comments 
					WHERE comments.video_id = videos.id
				),
				updated_at = NOW()
			WHERE is_active = true
			RETURNING id
		)
		SELECT COUNT(*) FROM updated_counts`

	var updatedCount int
	err := s.db.QueryRowContext(ctx, query).Scan(&updatedCount)
	if err != nil {
		return err
	}

	log.Printf("Updated counts for %d videos", updatedCount)
	return nil
}

func (s *VideoService) UpdateVideo(ctx context.Context, video *models.Video) error {
	video.UpdatedAt = time.Now()

	video.VideoURL = s.optimizeVideoURL(video.VideoURL)
	video.ThumbnailURL = s.optimizeThumbnailURL(video.ThumbnailURL)

	query := `
		UPDATE videos SET 
			caption = :caption,
			price = :price,
			video_url = :video_url,        
            thumbnail_url = :thumbnail_url,
			tags = :tags,
			is_featured = :is_featured,
			is_verified = :is_verified,
			is_active = :is_active,
			updated_at = :updated_at
		WHERE id = :id AND user_id = :user_id`

	result, err := s.db.NamedExecContext(ctx, query, video)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return errors.New("video_not_found_or_no_access")
	}

	return nil
}

func (s *VideoService) DeleteVideo(ctx context.Context, videoID, userID string) error {
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var exists int
	err = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM videos WHERE id = $1 AND user_id = $2", videoID, userID).Scan(&exists)
	if err != nil {
		return err
	}
	if exists == 0 {
		return errors.New("video_not_found_or_no_access")
	}

	queries := []string{
		"DELETE FROM video_likes WHERE video_id = $1",
		"DELETE FROM comment_likes WHERE comment_id IN (SELECT id FROM comments WHERE video_id = $1)",
		"DELETE FROM comments WHERE video_id = $1",
		"DELETE FROM videos WHERE id = $1",
	}

	for _, query := range queries {
		_, err = tx.ExecContext(ctx, query, videoID)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *VideoService) CheckVideoLiked(ctx context.Context, videoID, userID string) (bool, error) {
	var count int
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM video_likes WHERE video_id = $1 AND user_id = $2", videoID, userID).Scan(&count)
	return count > 0, err
}

func (s *VideoService) IncrementVideoShares(ctx context.Context, videoID string) error {
	query := `
		UPDATE videos 
		SET shares_count = shares_count + 1, updated_at = $1 
		WHERE id = $2 AND is_active = true`

	_, err := s.db.ExecContext(ctx, query, time.Now(), videoID)
	return err
}

// ===============================
// COMMENT OPERATIONS
// ===============================

func (s *VideoService) CreateComment(ctx context.Context, comment *models.Comment) (string, error) {
	if errors := comment.ValidateForCreation(); len(errors) > 0 {
		return "", fmt.Errorf("validation failed: %v", errors)
	}

	comment.ID = uuid.New().String()
	comment.CreatedAt = time.Now()
	comment.UpdatedAt = time.Now()
	comment.LikesCount = 0

	query := `
		INSERT INTO comments (
			id, video_id, author_id, author_name, author_image, content,
			likes_count, is_reply, replied_to_comment_id, replied_to_author_name,
			created_at, updated_at
		) VALUES (
			:id, :video_id, :author_id, :author_name, :author_image, :content,
			:likes_count, :is_reply, :replied_to_comment_id, :replied_to_author_name,
			:created_at, :updated_at
		)`

	_, err := s.db.NamedExecContext(ctx, query, comment)
	return comment.ID, err
}

func (s *VideoService) GetVideoComments(ctx context.Context, videoID string, limit, offset int) ([]models.Comment, error) {
	query := `
		SELECT * FROM comments 
		WHERE video_id = $1 
		ORDER BY created_at DESC 
		LIMIT $2 OFFSET $3`

	var comments []models.Comment
	err := s.db.SelectContext(ctx, &comments, query, videoID, limit, offset)
	return comments, err
}

func (s *VideoService) DeleteComment(ctx context.Context, commentID, userID string) error {
	var authorID string
	err := s.db.QueryRowContext(ctx, "SELECT author_id FROM comments WHERE id = $1", commentID).Scan(&authorID)
	if err != nil {
		return err
	}

	if authorID != userID {
		var userType string
		var userRole models.UserRole
		err = s.db.QueryRowContext(ctx, "SELECT user_type, role FROM users WHERE uid = $1", userID).Scan(&userType, &userRole)
		if err != nil {
			return err
		}
		if userType != "admin" && userType != "moderator" && userRole != models.UserRoleAdmin {
			return errors.New("access_denied")
		}
	}

	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	queries := []string{
		"DELETE FROM comment_likes WHERE comment_id = $1",
		"DELETE FROM comments WHERE replied_to_comment_id = $1",
		"DELETE FROM comments WHERE id = $1",
	}

	for _, query := range queries {
		_, err = tx.ExecContext(ctx, query, commentID)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *VideoService) LikeComment(ctx context.Context, commentID, userID string) error {
	var exists int
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM comment_likes WHERE comment_id = $1 AND user_id = $2", commentID, userID).Scan(&exists)
	if err != nil {
		return err
	}
	if exists > 0 {
		return errors.New("already_liked")
	}

	_, err = s.db.ExecContext(ctx, "INSERT INTO comment_likes (id, comment_id, user_id, created_at) VALUES ($1, $2, $3, $4)",
		uuid.New().String(), commentID, userID, time.Now())
	return err
}

func (s *VideoService) UnlikeComment(ctx context.Context, commentID, userID string) error {
	result, err := s.db.ExecContext(ctx, "DELETE FROM comment_likes WHERE comment_id = $1 AND user_id = $2", commentID, userID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return errors.New("not_liked")
	}

	return nil
}

// ===============================
// SOCIAL OPERATIONS
// ===============================

func (s *VideoService) FollowUser(ctx context.Context, followerID, followingID string) error {
	if followerID == followingID {
		return errors.New("cannot_follow_self")
	}

	var exists int
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM user_follows WHERE follower_id = $1 AND following_id = $2", followerID, followingID).Scan(&exists)
	if err != nil {
		return err
	}
	if exists > 0 {
		return errors.New("already_following")
	}

	_, err = s.db.ExecContext(ctx, "INSERT INTO user_follows (id, follower_id, following_id, created_at) VALUES ($1, $2, $3, $4)",
		uuid.New().String(), followerID, followingID, time.Now())
	return err
}

func (s *VideoService) UnfollowUser(ctx context.Context, followerID, followingID string) error {
	result, err := s.db.ExecContext(ctx, "DELETE FROM user_follows WHERE follower_id = $1 AND following_id = $2", followerID, followingID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return errors.New("not_following")
	}

	return nil
}

func (s *VideoService) CheckUserFollowing(ctx context.Context, followerID, followingID string) (bool, error) {
	var count int
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM user_follows WHERE follower_id = $1 AND following_id = $2", followerID, followingID).Scan(&count)
	return count > 0, err
}

func (s *VideoService) GetUserFollowers(ctx context.Context, userID string, limit, offset int) ([]models.User, error) {
	query := `
		SELECT u.uid, u.name, u.phone_number, u.whatsapp_number, u.profile_image, u.cover_image, u.bio,
		       u.user_type, u.role, u.followers_count, u.following_count, u.videos_count, u.likes_count,
		       u.is_verified, u.is_active, u.is_featured, u.tags,
		       u.created_at, u.updated_at, u.last_seen, u.last_post_at
		FROM users u
		JOIN user_follows uf ON u.uid = uf.follower_id
		WHERE uf.following_id = $1 AND u.is_active = true
		ORDER BY uf.created_at DESC
		LIMIT $2 OFFSET $3`

	var users []models.User
	err := s.db.SelectContext(ctx, &users, query, userID, limit, offset)
	return users, err
}

func (s *VideoService) GetUserFollowing(ctx context.Context, userID string, limit, offset int) ([]models.User, error) {
	query := `
		SELECT u.uid, u.name, u.phone_number, u.whatsapp_number, u.profile_image, u.cover_image, u.bio,
		       u.user_type, u.role, u.followers_count, u.following_count, u.videos_count, u.likes_count,
		       u.is_verified, u.is_active, u.is_featured, u.tags,
		       u.created_at, u.updated_at, u.last_seen, u.last_post_at
		FROM users u
		JOIN user_follows uf ON u.uid = uf.following_id
		WHERE uf.follower_id = $1 AND u.is_active = true
		ORDER BY uf.created_at DESC
		LIMIT $2 OFFSET $3`

	var users []models.User
	err := s.db.SelectContext(ctx, &users, query, userID, limit, offset)
	return users, err
}

func (s *VideoService) GetFollowingVideoFeed(ctx context.Context, userID string, limit, offset int) ([]models.VideoResponse, error) {
	query := `
		SELECT v.id, v.user_id, v.user_name, v.user_image, v.video_url, v.thumbnail_url,
		       v.caption, v.price, v.likes_count, v.comments_count, v.views_count, v.shares_count,
		       v.tags, v.is_active, v.is_featured, v.is_verified, v.is_multiple_images, v.image_urls,
		       v.created_at, v.updated_at
		FROM videos v
		JOIN user_follows uf ON v.user_id = uf.following_id
		WHERE uf.follower_id = $1 AND v.is_active = true
		ORDER BY v.created_at DESC
		LIMIT $2 OFFSET $3`

	rows, err := s.db.QueryContext(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var videos []models.VideoResponse
	for rows.Next() {
		var video models.VideoResponse

		err := rows.Scan(
			&video.ID, &video.UserID, &video.UserName, &video.UserImage,
			&video.VideoURL, &video.ThumbnailURL, &video.Caption, &video.Price,
			&video.LikesCount, &video.CommentsCount, &video.ViewsCount, &video.SharesCount,
			&video.Tags, &video.IsActive, &video.IsFeatured, &video.IsVerified,
			&video.IsMultipleImages, &video.ImageUrls, &video.CreatedAt, &video.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		s.applyURLOptimizations(&video)
		video.UserProfileImage = video.UserImage
		video.IsFollowing = true

		videos = append(videos, video)
	}

	return videos, nil
}

// ===============================
// ADMIN OPERATIONS
// ===============================

func (s *VideoService) ToggleFeatured(ctx context.Context, videoID string, isFeatured bool) error {
	query := `
		UPDATE videos 
		SET is_featured = $1, updated_at = $2 
		WHERE id = $3 AND is_active = true`

	result, err := s.db.ExecContext(ctx, query, isFeatured, time.Now(), videoID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return errors.New("video_not_found")
	}

	return nil
}

func (s *VideoService) ToggleActive(ctx context.Context, videoID string, isActive bool) error {
	query := `
		UPDATE videos 
		SET is_active = $1, updated_at = $2 
		WHERE id = $3`

	result, err := s.db.ExecContext(ctx, query, isActive, time.Now(), videoID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return errors.New("video_not_found")
	}

	return nil
}

func (s *VideoService) GetVideoStats(ctx context.Context, userID string) ([]models.VideoPerformance, error) {
	query := `
		SELECT id as video_id, caption as title, likes_count, comments_count, 
		       views_count, shares_count, created_at
		FROM videos 
		WHERE user_id = $1 AND is_active = true 
		ORDER BY created_at DESC`

	rows, err := s.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []models.VideoPerformance
	for rows.Next() {
		var stat models.VideoPerformance
		err := rows.Scan(
			&stat.VideoID, &stat.Title, &stat.LikesCount,
			&stat.CommentsCount, &stat.ViewsCount, &stat.SharesCount,
			&stat.CreatedAt,
		)
		if err != nil {
			return nil, err
		}

		stat.CalculateEngagementRate()
		stats = append(stats, stat)
	}

	return stats, nil
}
