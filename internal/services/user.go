// ===============================
// internal/services/user.go - User Service for Video Social Media
// ===============================

package services

import (
	"context"

	"weibaobe/internal/models"

	"github.com/jmoiron/sqlx"
)

type UserService struct {
	db *sqlx.DB
}

func NewUserService(db *sqlx.DB) *UserService {
	return &UserService{db: db}
}

// GetUserBasicInfo retrieves username and profile image for video creation
func (s *UserService) GetUserBasicInfo(ctx context.Context, userID string) (string, string, error) {
	var name, profileImage string
	err := s.db.QueryRowContext(ctx,
		"SELECT name, profile_image FROM users WHERE uid = $1 AND is_active = true",
		userID).Scan(&name, &profileImage)
	return name, profileImage, err
}

// GetUser retrieves full user information
func (s *UserService) GetUser(ctx context.Context, userID string) (*models.User, error) {
	var user models.User
	query := `
		SELECT uid, name, phone_number, profile_image, cover_image, bio, user_type,
		       followers_count, following_count, videos_count, likes_count,
		       is_verified, is_active, is_featured, tags,
		       created_at, updated_at, last_seen, last_post_at
		FROM users 
		WHERE uid = $1 AND is_active = true`

	err := s.db.GetContext(ctx, &user, query, userID)
	return &user, err
}

// UpdateUserPostCount updates user's videos count and last post timestamp
func (s *UserService) UpdateUserPostCount(ctx context.Context, userID string) error {
	query := `
		UPDATE users SET 
			videos_count = videos_count + 1,
			last_post_at = NOW(),
			updated_at = NOW()
		WHERE uid = $1`

	_, err := s.db.ExecContext(ctx, query, userID)
	return err
}

// CheckUserExists verifies if a user exists and is active
func (s *UserService) CheckUserExists(ctx context.Context, userID string) (bool, error) {
	var count int
	err := s.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM users WHERE uid = $1 AND is_active = true",
		userID).Scan(&count)
	return count > 0, err
}

// IsUserVerified checks if user is verified (needed for drama creation)
func (s *UserService) IsUserVerified(ctx context.Context, userID string) (bool, error) {
	var isVerified bool
	err := s.db.QueryRowContext(ctx,
		"SELECT is_verified FROM users WHERE uid = $1 AND is_active = true",
		userID).Scan(&isVerified)
	return isVerified, err
}
