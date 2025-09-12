// ===============================
// internal/services/video.go - Complete Video Social Media Service for PostgreSQL
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
// VIDEO CRUD OPERATIONS
// ===============================

// 🔧 FIXED: GetVideos with proper field mapping for frontend
func (s *VideoService) GetVideos(ctx context.Context, params models.VideoSearchParams) ([]models.VideoResponse, error) {
	query := `
		SELECT 
			id,
			user_id,
			user_name,
			user_image,
			video_url,
			thumbnail_url,
			caption,
			likes_count,
			comments_count,
			views_count,
			shares_count,
			tags,
			is_active,
			is_featured,
			is_multiple_images,
			image_urls,
			created_at,
			updated_at
		FROM videos 
		WHERE is_active = true`

	args := []interface{}{}
	argIndex := 1

	// Add filters
	if params.UserID != "" {
		query += fmt.Sprintf(" AND user_id = $%d", argIndex)
		args = append(args, params.UserID)
		argIndex++
	}

	if params.Featured != nil {
		query += fmt.Sprintf(" AND is_featured = $%d", argIndex)
		args = append(args, *params.Featured)
		argIndex++
	}

	if params.MediaType != "" && params.MediaType != "all" {
		if params.MediaType == "image" {
			query += " AND is_multiple_images = true"
		} else if params.MediaType == "video" {
			query += " AND is_multiple_images = false"
		}
	}

	if params.Query != "" {
		query += fmt.Sprintf(" AND (caption ILIKE $%d OR user_name ILIKE $%d)", argIndex, argIndex)
		searchPattern := "%" + params.Query + "%"
		args = append(args, searchPattern)
		argIndex++
	}

	// Add sorting
	switch params.SortBy {
	case "popular":
		query += " ORDER BY likes_count DESC, views_count DESC, created_at DESC"
	case "trending":
		query += " ORDER BY (likes_count * 2 + comments_count * 3 + shares_count * 5) / GREATEST(1, EXTRACT(EPOCH FROM (NOW() - created_at))/3600) DESC"
	case "views":
		query += " ORDER BY views_count DESC, created_at DESC"
	case "likes":
		query += " ORDER BY likes_count DESC, created_at DESC"
	default: // "latest"
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

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var videos []models.VideoResponse
	for rows.Next() {
		var video models.VideoResponse
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
		)
		if err != nil {
			return nil, err
		}

		// Set additional fields
		video.UserProfileImage = video.UserImage
		video.IsLiked = false     // Will be set by handler if user is authenticated
		video.IsFollowing = false // Will be set by handler if user is authenticated

		videos = append(videos, video)
	}

	return videos, nil
}

// 🔧 ENHANCED: GetFeaturedVideos with proper response structure
func (s *VideoService) GetFeaturedVideos(ctx context.Context, limit int) ([]models.VideoResponse, error) {
	query := `
		SELECT 
			id,
			user_id,
			user_name,
			user_image,
			video_url,
			thumbnail_url,
			caption,
			likes_count,
			comments_count,
			views_count,
			shares_count,
			tags,
			is_active,
			is_featured,
			is_multiple_images,
			image_urls,
			created_at,
			updated_at
		FROM videos 
		WHERE is_active = true AND is_featured = true 
		ORDER BY created_at DESC 
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
		)
		if err != nil {
			return nil, err
		}

		video.UserProfileImage = video.UserImage
		videos = append(videos, video)
	}

	return videos, nil
}

// 🔧 ENHANCED: GetTrendingVideos with proper response structure
func (s *VideoService) GetTrendingVideos(ctx context.Context, limit int) ([]models.VideoResponse, error) {
	query := `
		SELECT 
			id,
			user_id,
			user_name,
			user_image,
			video_url,
			thumbnail_url,
			caption,
			likes_count,
			comments_count,
			views_count,
			shares_count,
			tags,
			is_active,
			is_featured,
			is_multiple_images,
			image_urls,
			created_at,
			updated_at
		FROM videos 
		WHERE is_active = true 
		ORDER BY (likes_count * 2 + comments_count * 3 + shares_count * 5 + views_count) 
		/ GREATEST(1, EXTRACT(EPOCH FROM (NOW() - created_at))/3600) DESC,
		created_at DESC 
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
		)
		if err != nil {
			return nil, err
		}

		video.UserProfileImage = video.UserImage
		videos = append(videos, video)
	}

	return videos, nil
}

// 🔧 FIXED: GetVideo with proper field mapping
func (s *VideoService) GetVideo(ctx context.Context, videoID string) (*models.VideoResponse, error) {
	query := `
		SELECT 
			id,
			user_id,
			user_name,
			user_image,
			video_url,
			thumbnail_url,
			caption,
			likes_count,
			comments_count,
			views_count,
			shares_count,
			tags,
			is_active,
			is_featured,
			is_multiple_images,
			image_urls,
			created_at,
			updated_at
		FROM videos 
		WHERE id = $1 AND is_active = true`

	var video models.VideoResponse
	err := s.db.QueryRowContext(ctx, query, videoID).Scan(
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
	)
	if err != nil {
		return nil, err
	}

	// Set additional fields
	video.UserProfileImage = video.UserImage

	// 🔧 FIXED: Increment view count asynchronously
	go func() {
		s.incrementViewCount(videoID)
	}()

	// Increment view count for immediate display
	video.ViewsCount++

	return &video, nil
}

// 🔧 FIXED: GetUserVideos with proper response structure
func (s *VideoService) GetUserVideos(ctx context.Context, userID string, limit, offset int) ([]models.VideoResponse, error) {
	query := `
		SELECT 
			id,
			user_id,
			user_name,
			user_image,
			video_url,
			thumbnail_url,
			caption,
			likes_count,
			comments_count,
			views_count,
			shares_count,
			tags,
			is_active,
			is_featured,
			is_multiple_images,
			image_urls,
			created_at,
			updated_at
		FROM videos 
		WHERE user_id = $1 AND is_active = true 
		ORDER BY created_at DESC 
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
		)
		if err != nil {
			return nil, err
		}

		video.UserProfileImage = video.UserImage
		videos = append(videos, video)
	}

	return videos, nil
}

// CreateVideo creates a new video post
func (s *VideoService) CreateVideo(ctx context.Context, video *models.Video) (string, error) {
	// Validate video
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

	_, err := s.db.NamedExecContext(ctx, query, video)
	return video.ID, err
}

// UpdateVideo updates an existing video
func (s *VideoService) UpdateVideo(ctx context.Context, video *models.Video) error {
	video.UpdatedAt = time.Now()

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

// DeleteVideo deletes a video
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

	// Delete video likes
	_, err = tx.ExecContext(ctx, "DELETE FROM video_likes WHERE video_id = $1", videoID)
	if err != nil {
		return err
	}

	// Delete comment likes for this video's comments
	_, err = tx.ExecContext(ctx, `
		DELETE FROM comment_likes WHERE comment_id IN (
			SELECT id FROM comments WHERE video_id = $1
		)`, videoID)
	if err != nil {
		return err
	}

	// Delete comments
	_, err = tx.ExecContext(ctx, "DELETE FROM comments WHERE video_id = $1", videoID)
	if err != nil {
		return err
	}

	// Delete the video
	_, err = tx.ExecContext(ctx, "DELETE FROM videos WHERE id = $1", videoID)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// ===============================
// VIDEO INTERACTION OPERATIONS
// ===============================

// 🔧 FIXED: LikeVideo with trigger support
func (s *VideoService) LikeVideo(ctx context.Context, videoID, userID string) error {
	// Check if already liked
	var exists int
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM video_likes WHERE video_id = $1 AND user_id = $2", videoID, userID).Scan(&exists)
	if err != nil {
		return err
	}
	if exists > 0 {
		return errors.New("already_liked")
	}

	// Add like - trigger will auto-update the count
	_, err = s.db.ExecContext(ctx, "INSERT INTO video_likes (id, video_id, user_id, created_at) VALUES ($1, $2, $3, $4)",
		uuid.New().String(), videoID, userID, time.Now())
	return err
}

// 🔧 FIXED: UnlikeVideo with trigger support
func (s *VideoService) UnlikeVideo(ctx context.Context, videoID, userID string) error {
	result, err := s.db.ExecContext(ctx, "DELETE FROM video_likes WHERE video_id = $1 AND user_id = $2", videoID, userID)
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

// CheckVideoLiked checks if a user has liked a video
func (s *VideoService) CheckVideoLiked(ctx context.Context, videoID, userID string) (bool, error) {
	var count int
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM video_likes WHERE video_id = $1 AND user_id = $2", videoID, userID).Scan(&count)
	return count > 0, err
}

// GetUserLikedVideos returns videos liked by a user
func (s *VideoService) GetUserLikedVideos(ctx context.Context, userID string, limit, offset int) ([]models.VideoResponse, error) {
	query := `
		SELECT v.id, v.user_id, v.user_name, v.user_image, v.video_url, v.thumbnail_url,
		       v.caption, v.likes_count, v.comments_count, v.views_count, v.shares_count,
		       v.tags, v.is_active, v.is_featured, v.is_multiple_images, v.image_urls,
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
			&video.VideoURL, &video.ThumbnailURL, &video.Caption,
			&video.LikesCount, &video.CommentsCount, &video.ViewsCount, &video.SharesCount,
			&video.Tags, &video.IsActive, &video.IsFeatured, &video.IsMultipleImages,
			&video.ImageUrls, &video.CreatedAt, &video.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		video.UserProfileImage = video.UserImage
		video.IsLiked = true // These are liked videos
		videos = append(videos, video)
	}

	return videos, nil
}

// 🔧 FIXED: IncrementVideoViews with proper error handling
func (s *VideoService) IncrementVideoViews(ctx context.Context, videoID string) error {
	query := `
		UPDATE videos 
		SET views_count = views_count + 1, updated_at = $1 
		WHERE id = $2 AND is_active = true`

	result, err := s.db.ExecContext(ctx, query, time.Now(), videoID)
	if err != nil {
		// Log error but don't fail the request
		fmt.Printf("Warning: Failed to increment view count for video %s: %v\n", videoID, err)
		return nil // Don't return error for view counting failures
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		fmt.Printf("Warning: Could not check affected rows for video %s: %v\n", videoID, err)
		return nil
	}

	if rowsAffected == 0 {
		fmt.Printf("Warning: No rows affected when incrementing views for video %s\n", videoID)
	}

	return nil
}

// IncrementVideoShares increments the share count for a video
func (s *VideoService) IncrementVideoShares(ctx context.Context, videoID string) error {
	query := `
		UPDATE videos 
		SET shares_count = shares_count + 1, updated_at = $1 
		WHERE id = $2 AND is_active = true`

	_, err := s.db.ExecContext(ctx, query, time.Now(), videoID)
	return err
}

// 🔧 ENHANCED: Helper method for async view counting with retry logic
func (s *VideoService) incrementViewCount(videoID string) {
	maxRetries := 3
	for i := 0; i < maxRetries; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

		query := `UPDATE videos SET views_count = views_count + 1, updated_at = $1 WHERE id = $2 AND is_active = true`
		result, err := s.db.ExecContext(ctx, query, time.Now(), videoID)

		cancel()

		if err == nil {
			if rowsAffected, _ := result.RowsAffected(); rowsAffected > 0 {
				// Success
				return
			}
		}

		// Wait before retry (exponential backoff)
		time.Sleep(time.Duration(i+1) * time.Second)
	}

	// Log final failure
	fmt.Printf("Error: Failed to increment view count for video %s after %d retries\n", videoID, maxRetries)
}

// 🔧 NEW: Get video counts summary
func (s *VideoService) GetVideoCountsSummary(ctx context.Context, videoID string) (*models.VideoCountsSummary, error) {
	query := `
		SELECT 
			id,
			views_count,
			likes_count,
			comments_count,
			shares_count,
			updated_at
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
// COMMENT OPERATIONS
// ===============================

// CreateComment creates a new comment on a video
func (s *VideoService) CreateComment(ctx context.Context, comment *models.Comment) (string, error) {
	// Validate comment
	if errors := comment.ValidateForCreation(); len(errors) > 0 {
		return "", fmt.Errorf("validation failed: %v", errors)
	}

	// Set metadata
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

// GetVideoComments returns comments for a video
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

// DeleteComment deletes a comment
func (s *VideoService) DeleteComment(ctx context.Context, commentID, userID string) error {
	// Check if user owns the comment or is admin/moderator
	var authorID string
	err := s.db.QueryRowContext(ctx, "SELECT author_id FROM comments WHERE id = $1", commentID).Scan(&authorID)
	if err != nil {
		return err
	}

	if authorID != userID {
		// Check if user is admin/moderator
		var userType string
		err = s.db.QueryRowContext(ctx, "SELECT user_type FROM users WHERE uid = $1", userID).Scan(&userType)
		if err != nil {
			return err
		}
		if userType != "admin" && userType != "moderator" {
			return errors.New("access_denied")
		}
	}

	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Delete comment likes
	_, err = tx.ExecContext(ctx, "DELETE FROM comment_likes WHERE comment_id = $1", commentID)
	if err != nil {
		return err
	}

	// Delete replies to this comment
	_, err = tx.ExecContext(ctx, "DELETE FROM comments WHERE replied_to_comment_id = $1", commentID)
	if err != nil {
		return err
	}

	// Delete the comment
	_, err = tx.ExecContext(ctx, "DELETE FROM comments WHERE id = $1", commentID)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// LikeComment adds a like to a comment
func (s *VideoService) LikeComment(ctx context.Context, commentID, userID string) error {
	// Check if already liked
	var exists int
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM comment_likes WHERE comment_id = $1 AND user_id = $2", commentID, userID).Scan(&exists)
	if err != nil {
		return err
	}
	if exists > 0 {
		return errors.New("already_liked")
	}

	// Add like
	_, err = s.db.ExecContext(ctx, "INSERT INTO comment_likes (id, comment_id, user_id, created_at) VALUES ($1, $2, $3, $4)",
		uuid.New().String(), commentID, userID, time.Now())
	return err
}

// UnlikeComment removes a like from a comment
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

// FollowUser creates a follow relationship
func (s *VideoService) FollowUser(ctx context.Context, followerID, followingID string) error {
	if followerID == followingID {
		return errors.New("cannot_follow_self")
	}

	// Check if already following
	var exists int
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM user_follows WHERE follower_id = $1 AND following_id = $2", followerID, followingID).Scan(&exists)
	if err != nil {
		return err
	}
	if exists > 0 {
		return errors.New("already_following")
	}

	// Add follow
	_, err = s.db.ExecContext(ctx, "INSERT INTO user_follows (id, follower_id, following_id, created_at) VALUES ($1, $2, $3, $4)",
		uuid.New().String(), followerID, followingID, time.Now())
	return err
}

// UnfollowUser removes a follow relationship
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

// CheckUserFollowing checks if a user is following another user
func (s *VideoService) CheckUserFollowing(ctx context.Context, followerID, followingID string) (bool, error) {
	var count int
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM user_follows WHERE follower_id = $1 AND following_id = $2", followerID, followingID).Scan(&count)
	return count > 0, err
}

// GetUserFollowers returns users following a user
func (s *VideoService) GetUserFollowers(ctx context.Context, userID string, limit, offset int) ([]models.User, error) {
	query := `
		SELECT u.* FROM users u
		JOIN user_follows uf ON u.uid = uf.follower_id
		WHERE uf.following_id = $1 AND u.is_active = true
		ORDER BY uf.created_at DESC
		LIMIT $2 OFFSET $3`

	var users []models.User
	err := s.db.SelectContext(ctx, &users, query, userID, limit, offset)
	return users, err
}

// GetUserFollowing returns users that a user is following
func (s *VideoService) GetUserFollowing(ctx context.Context, userID string, limit, offset int) ([]models.User, error) {
	query := `
		SELECT u.* FROM users u
		JOIN user_follows uf ON u.uid = uf.following_id
		WHERE uf.follower_id = $1 AND u.is_active = true
		ORDER BY uf.created_at DESC
		LIMIT $2 OFFSET $3`

	var users []models.User
	err := s.db.SelectContext(ctx, &users, query, userID, limit, offset)
	return users, err
}

// GetFollowingVideoFeed returns videos from users that the current user follows
func (s *VideoService) GetFollowingVideoFeed(ctx context.Context, userID string, limit, offset int) ([]models.VideoResponse, error) {
	query := `
		SELECT v.id, v.user_id, v.user_name, v.user_image, v.video_url, v.thumbnail_url,
		       v.caption, v.likes_count, v.comments_count, v.views_count, v.shares_count,
		       v.tags, v.is_active, v.is_featured, v.is_multiple_images, v.image_urls,
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
			&video.VideoURL, &video.ThumbnailURL, &video.Caption,
			&video.LikesCount, &video.CommentsCount, &video.ViewsCount, &video.SharesCount,
			&video.Tags, &video.IsActive, &video.IsFeatured, &video.IsMultipleImages,
			&video.ImageUrls, &video.CreatedAt, &video.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		video.UserProfileImage = video.UserImage
		video.IsFollowing = true // These are from followed users
		videos = append(videos, video)
	}

	return videos, nil
}

// ===============================
// ADMIN OPERATIONS
// ===============================

// ToggleFeatured toggles the featured status of a video
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

// ToggleActive toggles the active status of a video
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

// GetVideoStats returns engagement statistics for videos
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

		// Calculate engagement rate
		stat.CalculateEngagementRate()
		stats = append(stats, stat)
	}

	return stats, nil
}

// ===============================
// ANALYTICS AND PERFORMANCE
// ===============================

// 🔧 NEW: Batch update view counts (for admin use)
func (s *VideoService) BatchUpdateViewCounts(ctx context.Context) error {
	// This can be called periodically to ensure consistency
	query := `
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
		WHERE id IN (
			SELECT DISTINCT id FROM videos WHERE is_active = true
		)`

	_, err := s.db.ExecContext(ctx, query)
	return err
}

// 🔧 NEW: Get trending videos with advanced scoring
func (s *VideoService) GetTrendingVideosAdvanced(ctx context.Context, limit int, timeWindow string) ([]models.VideoResponse, error) {
	var timeCondition string
	switch timeWindow {
	case "hour":
		timeCondition = "created_at >= NOW() - INTERVAL '1 hour'"
	case "day":
		timeCondition = "created_at >= NOW() - INTERVAL '1 day'"
	case "week":
		timeCondition = "created_at >= NOW() - INTERVAL '1 week'"
	default:
		timeCondition = "created_at >= NOW() - INTERVAL '1 day'"
	}

	query := fmt.Sprintf(`
		SELECT 
			id, user_id, user_name, user_image, video_url, thumbnail_url,
			caption, likes_count, comments_count, views_count, shares_count,
			tags, is_active, is_featured, is_multiple_images, image_urls,
			created_at, updated_at,
			-- Advanced trending score
			CASE 
				WHEN EXTRACT(EPOCH FROM (NOW() - created_at)) > 0 THEN
					(likes_count * 2.0 + comments_count * 3.0 + shares_count * 5.0 + views_count * 0.1) 
					/ POWER(EXTRACT(EPOCH FROM (NOW() - created_at))/3600 + 1, 1.5)
				ELSE 0 
			END as trending_score
		FROM videos 
		WHERE is_active = true AND %s
		ORDER BY trending_score DESC, created_at DESC 
		LIMIT $1`, timeCondition)

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
			&video.VideoURL, &video.ThumbnailURL, &video.Caption,
			&video.LikesCount, &video.CommentsCount, &video.ViewsCount, &video.SharesCount,
			&video.Tags, &video.IsActive, &video.IsFeatured, &video.IsMultipleImages,
			&video.ImageUrls, &video.CreatedAt, &video.UpdatedAt, &trendingScore,
		)
		if err != nil {
			return nil, err
		}

		video.UserProfileImage = video.UserImage
		videos = append(videos, video)
	}

	return videos, nil
}

// 🔧 NEW: Get video performance metrics
func (s *VideoService) GetVideoPerformanceMetrics(ctx context.Context, videoID string) (*models.VideoStatsResponse, error) {
	query := `
		SELECT 
			id,
			views_count,
			likes_count,
			comments_count,
			shares_count,
			created_at
		FROM videos 
		WHERE id = $1 AND is_active = true`

	var stats models.VideoStatsResponse
	var createdAt time.Time

	err := s.db.QueryRowContext(ctx, query, videoID).Scan(
		&stats.VideoID,
		&stats.ViewsCount,
		&stats.LikesCount,
		&stats.CommentsCount,
		&stats.SharesCount,
		&createdAt,
	)
	if err != nil {
		return nil, err
	}

	// Calculate engagement rate
	if stats.ViewsCount > 0 {
		totalEngagement := stats.LikesCount + stats.CommentsCount + stats.SharesCount
		stats.EngagementRate = (float64(totalEngagement) / float64(stats.ViewsCount)) * 100
	}

	// Calculate trending score
	hoursOld := time.Since(createdAt).Hours()
	if hoursOld > 0 {
		engagementScore := float64(stats.LikesCount*2 + stats.CommentsCount*3 + stats.SharesCount*5 + stats.ViewsCount)
		timeDecay := 1.0 / (1.0 + hoursOld/24.0)
		stats.TrendingScore = engagementScore * timeDecay
	}

	return &stats, nil
}

// 🔧 NEW: Get user engagement summary
func (s *VideoService) GetUserEngagementSummary(ctx context.Context, userID string) (map[string]interface{}, error) {
	query := `
		SELECT 
			COUNT(*) as total_videos,
			COALESCE(SUM(views_count), 0) as total_views,
			COALESCE(SUM(likes_count), 0) as total_likes,
			COALESCE(SUM(comments_count), 0) as total_comments,
			COALESCE(SUM(shares_count), 0) as total_shares,
			COALESCE(AVG(views_count), 0) as avg_views,
			COALESCE(AVG(likes_count), 0) as avg_likes,
			MAX(views_count) as max_views,
			MIN(CASE WHEN views_count > 0 THEN views_count END) as min_views
		FROM videos 
		WHERE user_id = $1 AND is_active = true`

	var summary struct {
		TotalVideos   int     `db:"total_videos"`
		TotalViews    int     `db:"total_views"`
		TotalLikes    int     `db:"total_likes"`
		TotalComments int     `db:"total_comments"`
		TotalShares   int     `db:"total_shares"`
		AvgViews      float64 `db:"avg_views"`
		AvgLikes      float64 `db:"avg_likes"`
		MaxViews      int     `db:"max_views"`
		MinViews      *int    `db:"min_views"`
	}

	err := s.db.QueryRowContext(ctx, query, userID).Scan(
		&summary.TotalVideos, &summary.TotalViews, &summary.TotalLikes,
		&summary.TotalComments, &summary.TotalShares, &summary.AvgViews,
		&summary.AvgLikes, &summary.MaxViews, &summary.MinViews,
	)
	if err != nil {
		return nil, err
	}

	// Calculate overall engagement rate
	var engagementRate float64
	if summary.TotalViews > 0 {
		totalEngagement := summary.TotalLikes + summary.TotalComments + summary.TotalShares
		engagementRate = (float64(totalEngagement) / float64(summary.TotalViews)) * 100
	}

	result := map[string]interface{}{
		"totalVideos":    summary.TotalVideos,
		"totalViews":     summary.TotalViews,
		"totalLikes":     summary.TotalLikes,
		"totalComments":  summary.TotalComments,
		"totalShares":    summary.TotalShares,
		"avgViews":       summary.AvgViews,
		"avgLikes":       summary.AvgLikes,
		"maxViews":       summary.MaxViews,
		"minViews":       summary.MinViews,
		"engagementRate": engagementRate,
	}

	return result, nil
}

// 🔧 NEW: Search videos with advanced filters
func (s *VideoService) SearchVideosAdvanced(ctx context.Context, params models.VideoSearchParams) ([]models.VideoResponse, error) {
	baseQuery := `
		SELECT 
			id, user_id, user_name, user_image, video_url, thumbnail_url,
			caption, likes_count, comments_count, views_count, shares_count,
			tags, is_active, is_featured, is_multiple_images, image_urls,
			created_at, updated_at
		FROM videos 
		WHERE is_active = true`

	var conditions []string
	var args []interface{}
	argIndex := 1

	// Text search in caption and username
	if params.Query != "" {
		conditions = append(conditions, fmt.Sprintf("(caption ILIKE $%d OR user_name ILIKE $%d)", argIndex, argIndex))
		searchPattern := "%" + params.Query + "%"
		args = append(args, searchPattern)
		argIndex++
	}

	// User filter
	if params.UserID != "" {
		conditions = append(conditions, fmt.Sprintf("user_id = $%d", argIndex))
		args = append(args, params.UserID)
		argIndex++
	}

	// Featured filter
	if params.Featured != nil {
		conditions = append(conditions, fmt.Sprintf("is_featured = $%d", argIndex))
		args = append(args, *params.Featured)
		argIndex++
	}

	// Media type filter
	if params.MediaType != "" && params.MediaType != "all" {
		if params.MediaType == "image" {
			conditions = append(conditions, "is_multiple_images = true")
		} else if params.MediaType == "video" {
			conditions = append(conditions, "is_multiple_images = false")
		}
	}

	// Tags filter
	if len(params.Tags) > 0 {
		for i := range params.Tags {
			conditions = append(conditions, fmt.Sprintf("$%d = ANY(tags)", argIndex+i))
		}
		for _, tag := range params.Tags {
			args = append(args, tag)
		}
		argIndex += len(params.Tags)
	}

	// Build final query
	query := baseQuery
	if len(conditions) > 0 {
		query += " AND " + fmt.Sprintf("(%s)", fmt.Sprintf(conditions[0]))
		for i := 1; i < len(conditions); i++ {
			query += " AND " + fmt.Sprintf("(%s)", conditions[i])
		}
	}

	// Add sorting
	switch params.SortBy {
	case "popular":
		query += " ORDER BY likes_count DESC, views_count DESC, created_at DESC"
	case "trending":
		query += " ORDER BY (likes_count * 2 + comments_count * 3 + shares_count * 5) / GREATEST(1, EXTRACT(EPOCH FROM (NOW() - created_at))/3600) DESC"
	case "views":
		query += " ORDER BY views_count DESC, created_at DESC"
	case "likes":
		query += " ORDER BY likes_count DESC, created_at DESC"
	default:
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
			&video.VideoURL, &video.ThumbnailURL, &video.Caption,
			&video.LikesCount, &video.CommentsCount, &video.ViewsCount, &video.SharesCount,
			&video.Tags, &video.IsActive, &video.IsFeatured, &video.IsMultipleImages,
			&video.ImageUrls, &video.CreatedAt, &video.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		video.UserProfileImage = video.UserImage
		videos = append(videos, video)
	}

	return videos, nil
}
