// ===============================
// VIDEO CRUD OPERATIONS
// ===============================

// ===============================
// internal/services/video.go - Video Social Media Service
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

// GetVideos returns paginated list of videos
func (s *VideoService) GetVideos(ctx context.Context, params models.VideoSearchParams) ([]models.Video, error) {
	query := `
		SELECT * FROM videos 
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

	var videos []models.Video
	err := s.db.SelectContext(ctx, &videos, query, args...)
	return videos, err
}

// GetFeaturedVideos returns featured videos
func (s *VideoService) GetFeaturedVideos(ctx context.Context, limit int) ([]models.Video, error) {
	query := `
		SELECT * FROM videos 
		WHERE is_active = true AND is_featured = true 
		ORDER BY created_at DESC 
		LIMIT $1`

	var videos []models.Video
	err := s.db.SelectContext(ctx, &videos, query, limit)
	return videos, err
}

// GetTrendingVideos returns trending videos based on engagement
func (s *VideoService) GetTrendingVideos(ctx context.Context, limit int) ([]models.Video, error) {
	query := `
		SELECT * FROM videos 
		WHERE is_active = true 
		ORDER BY (likes_count * 2 + comments_count * 3 + shares_count * 5 + views_count) 
		/ GREATEST(1, EXTRACT(EPOCH FROM (NOW() - created_at))/3600) DESC,
		created_at DESC 
		LIMIT $1`

	var videos []models.Video
	err := s.db.SelectContext(ctx, &videos, query, limit)
	return videos, err
}

// GetVideo returns a single video by ID
func (s *VideoService) GetVideo(ctx context.Context, videoID string) (*models.Video, error) {
	query := `SELECT * FROM videos WHERE id = $1 AND is_active = true`

	var video models.Video
	err := s.db.GetContext(ctx, &video, query, videoID)
	if err != nil {
		return nil, err
	}

	// Increment view count asynchronously
	go func() {
		s.incrementViewCount(videoID)
	}()

	return &video, nil
}

// GetUserVideos returns videos by a specific user
func (s *VideoService) GetUserVideos(ctx context.Context, userID string, limit, offset int) ([]models.Video, error) {
	query := `
		SELECT * FROM videos 
		WHERE user_id = $1 AND is_active = true 
		ORDER BY created_at DESC 
		LIMIT $2 OFFSET $3`

	var videos []models.Video
	err := s.db.SelectContext(ctx, &videos, query, userID, limit, offset)
	return videos, err
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

// LikeVideo adds a like to a video
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

	// Add like
	_, err = s.db.ExecContext(ctx, "INSERT INTO video_likes (video_id, user_id) VALUES ($1, $2)", videoID, userID)
	return err
}

// UnlikeVideo removes a like from a video
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
func (s *VideoService) GetUserLikedVideos(ctx context.Context, userID string, limit, offset int) ([]models.Video, error) {
	query := `
		SELECT v.* FROM videos v
		JOIN video_likes vl ON v.id = vl.video_id
		WHERE vl.user_id = $1 AND v.is_active = true
		ORDER BY vl.created_at DESC
		LIMIT $2 OFFSET $3`

	var videos []models.Video
	err := s.db.SelectContext(ctx, &videos, query, userID, limit, offset)
	return videos, err
}

// IncrementVideoViews increments the view count for a video
func (s *VideoService) IncrementVideoViews(ctx context.Context, videoID string) error {
	query := `
		UPDATE videos 
		SET views_count = views_count + 1, updated_at = $1 
		WHERE id = $2 AND is_active = true`

	_, err := s.db.ExecContext(ctx, query, time.Now(), videoID)
	return err
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

// Helper method for async view counting
func (s *VideoService) incrementViewCount(videoID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `UPDATE videos SET views_count = views_count + 1, updated_at = $1 WHERE id = $2 AND is_active = true`
	s.db.ExecContext(ctx, query, time.Now(), videoID)
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
	_, err = s.db.ExecContext(ctx, "INSERT INTO comment_likes (comment_id, user_id) VALUES ($1, $2)", commentID, userID)
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
	_, err = s.db.ExecContext(ctx, "INSERT INTO user_follows (follower_id, following_id) VALUES ($1, $2)", followerID, followingID)
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
func (s *VideoService) GetFollowingVideoFeed(ctx context.Context, userID string, limit, offset int) ([]models.Video, error) {
	query := `
		SELECT v.* FROM videos v
		JOIN user_follows uf ON v.user_id = uf.following_id
		WHERE uf.follower_id = $1 AND v.is_active = true
		ORDER BY v.created_at DESC
		LIMIT $2 OFFSET $3`

	var videos []models.Video
	err := s.db.SelectContext(ctx, &videos, query, userID, limit, offset)
	return videos, err
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
