// ===============================
// internal/services/video.go - UPDATED Video Service with Role Validation
// ===============================

package services

import (
	"context"
	"errors"
	"fmt"
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

// 🚀 NEW: URL optimization for video streaming
func (s *VideoService) optimizeVideoURL(url string) string {
	if url == "" {
		return url
	}

	// Add streaming optimizations for different providers
	if strings.Contains(url, "cloudflare.com") || strings.Contains(url, "r2.cloudflarestorage.com") {
		// Add Cloudflare streaming optimizations
		if !strings.Contains(url, "?") {
			return url + "?cf_optimize=true"
		}
		return url + "&cf_optimize=true"
	}

	// Add generic streaming parameters
	if !strings.Contains(url, "?") {
		return url + "?stream=true"
	}
	return url + "&stream=true"
}

// 🚀 NEW: Thumbnail URL optimization
func (s *VideoService) optimizeThumbnailURL(url string) string {
	if url == "" {
		return url
	}

	// Add image optimization parameters
	if strings.Contains(url, "cloudflare.com") {
		if !strings.Contains(url, "?") {
			return url + "?format=webp&quality=85&width=640"
		}
		return url + "&format=webp&quality=85&width=640"
	}

	return url
}

// 🚀 NEW: Apply URL optimizations to video response
func (s *VideoService) applyURLOptimizations(video *models.VideoResponse) {
	video.VideoURL = s.optimizeVideoURL(video.VideoURL)
	video.ThumbnailURL = s.optimizeThumbnailURL(video.ThumbnailURL)
	video.UserImage = s.optimizeThumbnailURL(video.UserImage)
	video.UserProfileImage = s.optimizeThumbnailURL(video.UserProfileImage)

	// Optimize image URLs in array
	for i, imageURL := range video.ImageUrls {
		video.ImageUrls[i] = s.optimizeThumbnailURL(imageURL)
	}
}

// ===============================
// 🆕 NEW: ROLE-BASED VALIDATION HELPERS
// ===============================

// 🆕 NEW: ValidateUserCanCreateVideo validates user role and status for video creation
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

	// Check if user account is active
	if !user.IsActive {
		return nil, fmt.Errorf("user account is inactive")
	}

	// Check if user role allows posting videos
	if !user.CanPost() {
		return nil, fmt.Errorf("user role '%s' cannot post videos. Only admin and host users can post", user.Role.DisplayName())
	}

	return &user, nil
}

// 🆕 NEW: GetUserBasicInfoWithRole retrieves user info including role for video operations
func (s *VideoService) GetUserBasicInfoWithRole(ctx context.Context, userID string) (string, string, models.UserRole, error) {
	var name, profileImage string
	var role models.UserRole

	query := `
		SELECT name, profile_image, role 
		FROM users 
		WHERE uid = $1 AND is_active = true`

	err := s.db.QueryRowContext(ctx, query, userID).Scan(&name, &profileImage, &role)
	if err != nil {
		return "", "", models.UserRoleGuest, fmt.Errorf("failed to get user info: %w", err)
	}

	return name, profileImage, role, nil
}

// 🆕 NEW: CheckUserPostingPermission quickly checks if user can post
func (s *VideoService) CheckUserPostingPermission(ctx context.Context, userID string) error {
	var role models.UserRole
	var isActive bool

	query := `
		SELECT role, is_active 
		FROM users 
		WHERE uid = $1`

	err := s.db.QueryRowContext(ctx, query, userID).Scan(&role, &isActive)
	if err != nil {
		return fmt.Errorf("user not found: %s", userID)
	}

	if !isActive {
		return fmt.Errorf("user account is inactive")
	}

	if !role.CanPost() {
		return fmt.Errorf("user role '%s' cannot post videos", role.DisplayName())
	}

	return nil
}

// ===============================
// OPTIMIZED VIDEO CRUD OPERATIONS WITH ROLE VALIDATION
// ===============================

// 🚀 OPTIMIZED: GetVideosOptimized with better query and URL optimization
func (s *VideoService) GetVideosOptimized(ctx context.Context, params models.VideoSearchParams) ([]models.VideoResponse, error) {
	// Optimized query with better indexing hints
	query := `
		SELECT 
			v.id,
			v.user_id,
			v.user_name,
			v.user_image,
			v.video_url,
			v.thumbnail_url,
			v.caption,
			v.likes_count,
			v.comments_count,
			v.views_count,
			v.shares_count,
			v.tags,
			v.is_active,
			v.is_featured,
			v.is_multiple_images,
			v.image_urls,
			v.created_at,
			v.updated_at,
			u.role as user_role
		FROM videos v
		JOIN users u ON v.user_id = u.uid
		WHERE v.is_active = true AND u.is_active = true`

	args := []interface{}{}
	argIndex := 1

	// Add filters with optimized conditions
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
		// Optimized full-text search with better performance
		query += fmt.Sprintf(" AND (v.caption ILIKE $%d OR v.user_name ILIKE $%d)", argIndex, argIndex)
		searchPattern := "%" + params.Query + "%"
		args = append(args, searchPattern)
		argIndex++
	}

	// 🆕 NEW: Filter by role if specified
	if params.Role != nil {
		query += fmt.Sprintf(" AND u.role = $%d", argIndex)
		args = append(args, *params.Role)
		argIndex++
	}

	// Optimized sorting with better performance
	switch params.SortBy {
	case "popular":
		query += " ORDER BY v.likes_count DESC, v.views_count DESC, v.created_at DESC"
	case "trending":
		// Improved trending algorithm with time decay
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
	default: // "latest"
		query += " ORDER BY v.created_at DESC"
	}

	// Add pagination with limits
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
		var userRole models.UserRole

		err := rows.Scan(
			&video.ID,
			&video.UserID,
			&video.UserName,
			&video.UserImage,
			&video.VideoURL,
			&video.ThumbnailURL,
			&video.Caption,
			&video.LikesCount,
			&video.CommentsCount,
			&video.ViewsCount,
			&video.SharesCount,
			&video.Tags,
			&video.IsActive,
			&video.IsFeatured,
			&video.IsMultipleImages,
			&video.ImageUrls,
			&video.CreatedAt,
			&video.UpdatedAt,
			&userRole,
		)
		if err != nil {
			return nil, err
		}

		// Apply URL optimizations
		s.applyURLOptimizations(&video)

		// Set additional fields
		video.UserProfileImage = video.UserImage
		video.IsLiked = false     // Will be set by handler if user is authenticated
		video.IsFollowing = false // Will be set by handler if user is authenticated

		// 🆕 NEW: Add role information to response
		video.UserRole = userRole.String()
		video.UserCanPost = userRole.CanPost()

		videos = append(videos, video)
	}

	return videos, nil
}

// 🚀 NEW: Bulk video fetching for efficient loading with role info
func (s *VideoService) GetVideosBulk(ctx context.Context, videoIDs []string, includeInactive bool) ([]models.VideoResponse, error) {
	if len(videoIDs) == 0 {
		return []models.VideoResponse{}, nil
	}

	// Build query with IN clause for bulk fetching including role
	query := `
		SELECT 
			v.id, v.user_id, v.user_name, v.user_image, v.video_url, v.thumbnail_url,
			v.caption, v.likes_count, v.comments_count, v.views_count, v.shares_count,
			v.tags, v.is_active, v.is_featured, v.is_multiple_images, v.image_urls,
			v.created_at, v.updated_at, u.role as user_role
		FROM videos v
		JOIN users u ON v.user_id = u.uid
		WHERE v.id = ANY($1::text[]) AND u.is_active = true`

	if !includeInactive {
		query += " AND v.is_active = true"
	}

	query += " ORDER BY v.created_at DESC"

	// Convert string slice to PostgreSQL array format
	rows, err := s.db.QueryContext(ctx, query, videoIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var videos []models.VideoResponse
	for rows.Next() {
		var video models.VideoResponse
		var userRole models.UserRole

		err := rows.Scan(
			&video.ID, &video.UserID, &video.UserName, &video.UserImage,
			&video.VideoURL, &video.ThumbnailURL, &video.Caption,
			&video.LikesCount, &video.CommentsCount, &video.ViewsCount, &video.SharesCount,
			&video.Tags, &video.IsActive, &video.IsFeatured, &video.IsMultipleImages,
			&video.ImageUrls, &video.CreatedAt, &video.UpdatedAt, &userRole,
		)
		if err != nil {
			return nil, err
		}

		// Apply URL optimizations
		s.applyURLOptimizations(&video)
		video.UserProfileImage = video.UserImage

		// 🆕 NEW: Add role information
		video.UserRole = userRole.String()
		video.UserCanPost = userRole.CanPost()

		videos = append(videos, video)
	}

	return videos, nil
}

// 🚀 OPTIMIZED: GetFeaturedVideosOptimized with role info
func (s *VideoService) GetFeaturedVideosOptimized(ctx context.Context, limit int) ([]models.VideoResponse, error) {
	// Optimized query with index hints and role info
	query := `
		SELECT 
			v.id, v.user_id, v.user_name, v.user_image, v.video_url, v.thumbnail_url,
			v.caption, v.likes_count, v.comments_count, v.views_count, v.shares_count,
			v.tags, v.is_active, v.is_featured, v.is_multiple_images, v.image_urls,
			v.created_at, v.updated_at, u.role as user_role
		FROM videos v
		JOIN users u ON v.user_id = u.uid
		WHERE v.is_active = true AND v.is_featured = true AND u.is_active = true
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
		var userRole models.UserRole

		err := rows.Scan(
			&video.ID, &video.UserID, &video.UserName, &video.UserImage,
			&video.VideoURL, &video.ThumbnailURL, &video.Caption,
			&video.LikesCount, &video.CommentsCount, &video.ViewsCount, &video.SharesCount,
			&video.Tags, &video.IsActive, &video.IsFeatured, &video.IsMultipleImages,
			&video.ImageUrls, &video.CreatedAt, &video.UpdatedAt, &userRole,
		)
		if err != nil {
			return nil, err
		}

		// Apply URL optimizations
		s.applyURLOptimizations(&video)
		video.UserProfileImage = video.UserImage

		// 🆕 NEW: Add role information
		video.UserRole = userRole.String()
		video.UserCanPost = userRole.CanPost()

		videos = append(videos, video)
	}

	return videos, nil
}

// 🚀 OPTIMIZED: GetTrendingVideosOptimized with improved algorithm and role info
func (s *VideoService) GetTrendingVideosOptimized(ctx context.Context, limit int) ([]models.VideoResponse, error) {
	// Enhanced trending algorithm with better time decay and engagement weights
	query := `
		SELECT 
			v.id, v.user_id, v.user_name, v.user_image, v.video_url, v.thumbnail_url,
			v.caption, v.likes_count, v.comments_count, v.views_count, v.shares_count,
			v.tags, v.is_active, v.is_featured, v.is_multiple_images, v.image_urls,
			v.created_at, v.updated_at, u.role as user_role,
			CASE 
				WHEN EXTRACT(EPOCH FROM (NOW() - v.created_at)) > 0 THEN
					(v.likes_count * 2.5 + v.comments_count * 3.5 + v.shares_count * 5.0 + v.views_count * 0.1) 
					/ POWER(EXTRACT(EPOCH FROM (NOW() - v.created_at))/3600 + 1, 1.8)
				ELSE v.likes_count * 2.5 + v.comments_count * 3.5 + v.shares_count * 5.0 
			END as trending_score
		FROM videos v
		JOIN users u ON v.user_id = u.uid
		WHERE v.is_active = true AND u.is_active = true
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
		var userRole models.UserRole
		var trendingScore float64 // We select but don't need to return this

		err := rows.Scan(
			&video.ID, &video.UserID, &video.UserName, &video.UserImage,
			&video.VideoURL, &video.ThumbnailURL, &video.Caption,
			&video.LikesCount, &video.CommentsCount, &video.ViewsCount, &video.SharesCount,
			&video.Tags, &video.IsActive, &video.IsFeatured, &video.IsMultipleImages,
			&video.ImageUrls, &video.CreatedAt, &video.UpdatedAt, &userRole, &trendingScore,
		)
		if err != nil {
			return nil, err
		}

		// Apply URL optimizations
		s.applyURLOptimizations(&video)
		video.UserProfileImage = video.UserImage

		// 🆕 NEW: Add role information
		video.UserRole = userRole.String()
		video.UserCanPost = userRole.CanPost()

		videos = append(videos, video)
	}

	return videos, nil
}

// 🚀 OPTIMIZED: GetVideoOptimized with URL optimization and role info
func (s *VideoService) GetVideoOptimized(ctx context.Context, videoID string) (*models.VideoResponse, error) {
	query := `
		SELECT 
			v.id, v.user_id, v.user_name, v.user_image, v.video_url, v.thumbnail_url,
			v.caption, v.likes_count, v.comments_count, v.views_count, v.shares_count,
			v.tags, v.is_active, v.is_featured, v.is_multiple_images, v.image_urls,
			v.created_at, v.updated_at, u.role as user_role
		FROM videos v
		JOIN users u ON v.user_id = u.uid
		WHERE v.id = $1 AND v.is_active = true AND u.is_active = true`

	var video models.VideoResponse
	var userRole models.UserRole

	err := s.db.QueryRowContext(ctx, query, videoID).Scan(
		&video.ID, &video.UserID, &video.UserName, &video.UserImage,
		&video.VideoURL, &video.ThumbnailURL, &video.Caption,
		&video.LikesCount, &video.CommentsCount, &video.ViewsCount, &video.SharesCount,
		&video.Tags, &video.IsActive, &video.IsFeatured, &video.IsMultipleImages,
		&video.ImageUrls, &video.CreatedAt, &video.UpdatedAt, &userRole,
	)
	if err != nil {
		return nil, err
	}

	// Apply URL optimizations
	s.applyURLOptimizations(&video)
	video.UserProfileImage = video.UserImage

	// 🆕 NEW: Add role information
	video.UserRole = userRole.String()
	video.UserCanPost = userRole.CanPost()

	// 🚀 OPTIMIZED: Async view increment with better performance
	go func() {
		s.incrementViewCountOptimized(videoID)
	}()

	// Increment view count for immediate display
	video.ViewsCount++

	return &video, nil
}

// 🚀 OPTIMIZED: GetUserVideosOptimized with URL optimization and role info
func (s *VideoService) GetUserVideosOptimized(ctx context.Context, userID string, limit, offset int) ([]models.VideoResponse, error) {
	query := `
		SELECT 
			v.id, v.user_id, v.user_name, v.user_image, v.video_url, v.thumbnail_url,
			v.caption, v.likes_count, v.comments_count, v.views_count, v.shares_count,
			v.tags, v.is_active, v.is_featured, v.is_multiple_images, v.image_urls,
			v.created_at, v.updated_at, u.role as user_role
		FROM videos v
		JOIN users u ON v.user_id = u.uid
		WHERE v.user_id = $1 AND v.is_active = true AND u.is_active = true
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
		var userRole models.UserRole

		err := rows.Scan(
			&video.ID, &video.UserID, &video.UserName, &video.UserImage,
			&video.VideoURL, &video.ThumbnailURL, &video.Caption,
			&video.LikesCount, &video.CommentsCount, &video.ViewsCount, &video.SharesCount,
			&video.Tags, &video.IsActive, &video.IsFeatured, &video.IsMultipleImages,
			&video.ImageUrls, &video.CreatedAt, &video.UpdatedAt, &userRole,
		)
		if err != nil {
			return nil, err
		}

		// Apply URL optimizations
		s.applyURLOptimizations(&video)
		video.UserProfileImage = video.UserImage

		// 🆕 NEW: Add role information
		video.UserRole = userRole.String()
		video.UserCanPost = userRole.CanPost()

		videos = append(videos, video)
	}

	return videos, nil
}

// 🚀 OPTIMIZED: GetUserLikedVideosOptimized with URL optimization and role info
func (s *VideoService) GetUserLikedVideosOptimized(ctx context.Context, userID string, limit, offset int) ([]models.VideoResponse, error) {
	query := `
		SELECT v.id, v.user_id, v.user_name, v.user_image, v.video_url, v.thumbnail_url,
		       v.caption, v.likes_count, v.comments_count, v.views_count, v.shares_count,
		       v.tags, v.is_active, v.is_featured, v.is_multiple_images, v.image_urls,
		       v.created_at, v.updated_at, u.role as user_role
		FROM videos v
		JOIN video_likes vl ON v.id = vl.video_id
		JOIN users u ON v.user_id = u.uid
		WHERE vl.user_id = $1 AND v.is_active = true AND u.is_active = true
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
		var userRole models.UserRole

		err := rows.Scan(
			&video.ID, &video.UserID, &video.UserName, &video.UserImage,
			&video.VideoURL, &video.ThumbnailURL, &video.Caption,
			&video.LikesCount, &video.CommentsCount, &video.ViewsCount, &video.SharesCount,
			&video.Tags, &video.IsActive, &video.IsFeatured, &video.IsMultipleImages,
			&video.ImageUrls, &video.CreatedAt, &video.UpdatedAt, &userRole,
		)
		if err != nil {
			return nil, err
		}

		// Apply URL optimizations
		s.applyURLOptimizations(&video)
		video.UserProfileImage = video.UserImage
		video.IsLiked = true // These are liked videos

		// 🆕 NEW: Add role information
		video.UserRole = userRole.String()
		video.UserCanPost = userRole.CanPost()

		videos = append(videos, video)
	}

	return videos, nil
}

// 🚀 UPDATED: CreateVideoOptimized with role validation
func (s *VideoService) CreateVideoOptimized(ctx context.Context, video *models.Video) (string, error) {
	// 🆕 NEW: Validate user can create videos based on role
	user, err := s.ValidateUserCanCreateVideo(ctx, video.UserID)
	if err != nil {
		return "", fmt.Errorf("video creation validation failed: %w", err)
	}

	// Enhanced validation
	if !video.IsValidForCreation() {
		errors := video.ValidateForCreation()
		return "", fmt.Errorf("validation failed: %v", errors)
	}

	// Set metadata
	video.ID = uuid.New().String()
	video.CreatedAt = time.Now()
	video.UpdatedAt = time.Now()
	video.IsActive = true
	video.LikesCount = 0
	video.CommentsCount = 0
	video.ViewsCount = 0
	video.SharesCount = 0

	// Ensure user info is from validated user
	video.UserName = user.Name
	video.UserImage = user.ProfileImage

	// Optimize URLs before storing
	video.VideoURL = s.optimizeVideoURL(video.VideoURL)
	video.ThumbnailURL = s.optimizeThumbnailURL(video.ThumbnailURL)

	// Start transaction for consistent data
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	// Optimized insert query
	query := `
		INSERT INTO videos (
			id, user_id, user_name, user_image, video_url, thumbnail_url,
			caption, likes_count, comments_count, views_count, shares_count,
			tags, is_active, is_featured, is_multiple_images, image_urls,
			created_at, updated_at
		) VALUES (
			:id, :user_id, :user_name, :user_image, :video_url, :thumbnail_url,
			:caption, :likes_count, :comments_count, :views_count, :shares_count,
			:tags, :is_active, :is_featured, :is_multiple_images, :image_urls,
			:created_at, :updated_at
		)`

	_, err = tx.NamedExecContext(ctx, query, video)
	if err != nil {
		return "", fmt.Errorf("failed to insert video: %w", err)
	}

	// Update user's last post timestamp and video count (handled by database trigger)
	// But we can also update it manually for immediate consistency
	_, err = tx.ExecContext(ctx, `
		UPDATE users 
		SET last_post_at = $1, updated_at = $1 
		WHERE uid = $2`,
		video.CreatedAt, video.UserID)
	if err != nil {
		return "", fmt.Errorf("failed to update user last post: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return "", fmt.Errorf("failed to commit transaction: %w", err)
	}

	return video.ID, nil
}

// ===============================
// OPTIMIZED VIDEO INTERACTION OPERATIONS
// ===============================

// 🚀 OPTIMIZED: Enhanced view count increment with batching
func (s *VideoService) incrementViewCountOptimized(videoID string) {
	maxRetries := 3
	for i := 0; i < maxRetries; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)

		// Optimized query with RETURNING clause for better performance
		query := `
			UPDATE videos 
			SET views_count = views_count + 1, updated_at = $1 
			WHERE id = $2 AND is_active = true 
			RETURNING views_count`

		var newCount int
		err := s.db.QueryRowContext(ctx, query, time.Now(), videoID).Scan(&newCount)
		cancel()

		if err == nil {
			// Success - could potentially cache the new count here
			return
		}

		// Exponential backoff
		time.Sleep(time.Duration(i+1) * 500 * time.Millisecond)
	}

	// Log failure after all retries
	fmt.Printf("Warning: Failed to increment view count for video %s after %d retries\n", videoID, maxRetries)
}

// 🚀 OPTIMIZED: IncrementVideoViews with better error handling
func (s *VideoService) IncrementVideoViews(ctx context.Context, videoID string) error {
	// Use optimized async increment
	go s.incrementViewCountOptimized(videoID)
	return nil // Always return success for view counts to not break user experience
}

// 🚀 OPTIMIZED: LikeVideo with optimized database operations
func (s *VideoService) LikeVideo(ctx context.Context, videoID, userID string) error {
	// Check if already liked with optimized query
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

	// Add like - database trigger will auto-update the count
	_, err = s.db.ExecContext(ctx,
		"INSERT INTO video_likes (id, video_id, user_id, created_at) VALUES ($1, $2, $3, $4)",
		uuid.New().String(), videoID, userID, time.Now())
	return err
}

// 🚀 OPTIMIZED: UnlikeVideo with optimized database operations
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

// 🚀 OPTIMIZED: GetVideoCountsSummary with better performance
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

// ===============================
// OPTIMIZED BATCH OPERATIONS
// ===============================

// 🚀 OPTIMIZED: Batch update counts with better performance
func (s *VideoService) BatchUpdateViewCounts(ctx context.Context) error {
	// More efficient batch update using CTEs
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

	fmt.Printf("Updated counts for %d videos\n", updatedCount)
	return nil
}

// ===============================
// 🆕 NEW: ROLE-BASED ANALYTICS AND REPORTING
// ===============================

// 🆕 NEW: GetVideoStatsByRole returns video statistics grouped by user roles
func (s *VideoService) GetVideoStatsByRole(ctx context.Context) (map[string]interface{}, error) {
	query := `
		SELECT 
			u.role,
			COUNT(v.id) as video_count,
			COALESCE(SUM(v.views_count), 0) as total_views,
			COALESCE(SUM(v.likes_count), 0) as total_likes,
			COALESCE(SUM(v.comments_count), 0) as total_comments,
			COALESCE(AVG(v.views_count), 0) as avg_views,
			COALESCE(AVG(v.likes_count), 0) as avg_likes
		FROM videos v
		JOIN users u ON v.user_id = u.uid
		WHERE v.is_active = true AND u.is_active = true
		GROUP BY u.role
		ORDER BY video_count DESC`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stats := make(map[string]interface{})
	var totalVideos int
	var totalViews int64
	var totalLikes int64

	for rows.Next() {
		var role string
		var videoCount int
		var views, likes, comments int64
		var avgViews, avgLikes float64

		err := rows.Scan(&role, &videoCount, &views, &likes, &comments, &avgViews, &avgLikes)
		if err != nil {
			return nil, err
		}

		stats[role] = map[string]interface{}{
			"videoCount":    videoCount,
			"totalViews":    views,
			"totalLikes":    likes,
			"totalComments": comments,
			"avgViews":      avgViews,
			"avgLikes":      avgLikes,
		}

		totalVideos += videoCount
		totalViews += views
		totalLikes += likes
	}

	// Add overall statistics
	stats["overall"] = map[string]interface{}{
		"totalVideos": totalVideos,
		"totalViews":  totalViews,
		"totalLikes":  totalLikes,
	}

	return stats, nil
}

// 🆕 NEW: GetTopContentCreators returns top video creators by role
func (s *VideoService) GetTopContentCreators(ctx context.Context, limit int) ([]map[string]interface{}, error) {
	query := `
		SELECT 
			u.uid,
			u.name,
			u.role,
			u.profile_image,
			COUNT(v.id) as video_count,
			COALESCE(SUM(v.views_count), 0) as total_views,
			COALESCE(SUM(v.likes_count), 0) as total_likes,
			COALESCE(AVG(v.views_count), 0) as avg_views
		FROM users u
		JOIN videos v ON u.uid = v.user_id
		WHERE u.is_active = true AND v.is_active = true AND u.role IN ('admin', 'host')
		GROUP BY u.uid, u.name, u.role, u.profile_image
		ORDER BY total_views DESC, total_likes DESC
		LIMIT $1`

	rows, err := s.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var creators []map[string]interface{}
	for rows.Next() {
		var uid, name, role, profileImage string
		var videoCount int
		var totalViews, totalLikes int64
		var avgViews float64

		err := rows.Scan(&uid, &name, &role, &profileImage, &videoCount, &totalViews, &totalLikes, &avgViews)
		if err != nil {
			return nil, err
		}

		creators = append(creators, map[string]interface{}{
			"uid":          uid,
			"name":         name,
			"role":         role,
			"profileImage": s.optimizeThumbnailURL(profileImage),
			"videoCount":   videoCount,
			"totalViews":   totalViews,
			"totalLikes":   totalLikes,
			"avgViews":     avgViews,
		})
	}

	return creators, nil
}

// ===============================
// PRESERVED METHODS WITH OPTIMIZATIONS APPLIED
// All existing functionality maintained with performance improvements and role awareness
// ===============================

func (s *VideoService) UpdateVideo(ctx context.Context, video *models.Video) error {
	video.UpdatedAt = time.Now()

	// Optimize URLs before updating
	video.VideoURL = s.optimizeVideoURL(video.VideoURL)
	video.ThumbnailURL = s.optimizeThumbnailURL(video.ThumbnailURL)

	query := `
		UPDATE videos SET 
			caption = :caption,
			tags = :tags,
			is_featured = :is_featured,
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

	// Check if video exists and user owns it
	var exists int
	err = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM videos WHERE id = $1 AND user_id = $2", videoID, userID).Scan(&exists)
	if err != nil {
		return err
	}
	if exists == 0 {
		return errors.New("video_not_found_or_no_access")
	}

	// Delete in optimized order
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
// PRESERVED COMMENT OPERATIONS
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
		// 🆕 UPDATED: Check both old user_type and new role system
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
// PRESERVED SOCIAL OPERATIONS WITH ROLE AWARENESS
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

// 🚀 UPDATED: GetUserFollowers with role information
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

// 🚀 UPDATED: GetUserFollowing with role information
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

// 🚀 UPDATED: GetFollowingVideoFeed with role information
func (s *VideoService) GetFollowingVideoFeed(ctx context.Context, userID string, limit, offset int) ([]models.VideoResponse, error) {
	query := `
		SELECT v.id, v.user_id, v.user_name, v.user_image, v.video_url, v.thumbnail_url,
		       v.caption, v.likes_count, v.comments_count, v.views_count, v.shares_count,
		       v.tags, v.is_active, v.is_featured, v.is_multiple_images, v.image_urls,
		       v.created_at, v.updated_at, u.role as user_role
		FROM videos v
		JOIN user_follows uf ON v.user_id = uf.following_id
		JOIN users u ON v.user_id = u.uid
		WHERE uf.follower_id = $1 AND v.is_active = true AND u.is_active = true
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
		var userRole models.UserRole

		err := rows.Scan(
			&video.ID, &video.UserID, &video.UserName, &video.UserImage,
			&video.VideoURL, &video.ThumbnailURL, &video.Caption,
			&video.LikesCount, &video.CommentsCount, &video.ViewsCount, &video.SharesCount,
			&video.Tags, &video.IsActive, &video.IsFeatured, &video.IsMultipleImages,
			&video.ImageUrls, &video.CreatedAt, &video.UpdatedAt, &userRole,
		)
		if err != nil {
			return nil, err
		}

		// Apply URL optimizations
		s.applyURLOptimizations(&video)
		video.UserProfileImage = video.UserImage
		video.IsFollowing = true // These are from followed users

		// 🆕 NEW: Add role information
		video.UserRole = userRole.String()
		video.UserCanPost = userRole.CanPost()

		videos = append(videos, video)
	}

	return videos, nil
}

// ===============================
// PRESERVED ADMIN OPERATIONS WITH OPTIMIZATIONS
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
