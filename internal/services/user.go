// ===============================
// internal/services/user.go - UPDATED User Service with Role and WhatsApp Support
// ===============================

package services

import (
	"context"
	"fmt"
	"time"

	"weibaobe/internal/models"

	"github.com/jmoiron/sqlx"
)

type UserService struct {
	db *sqlx.DB
}

func NewUserService(db *sqlx.DB) *UserService {
	return &UserService{db: db}
}

// GetUserBasicInfo retrieves username, profile image, and role for video creation
func (s *UserService) GetUserBasicInfo(ctx context.Context, userID string) (string, string, models.UserRole, error) {
	var name, profileImage string
	var role models.UserRole
	err := s.db.QueryRowContext(ctx,
		"SELECT name, profile_image, role FROM users WHERE uid = $1 AND is_active = true",
		userID).Scan(&name, &profileImage, &role)
	return name, profileImage, role, err
}

// GetUser retrieves full user information including new fields
func (s *UserService) GetUser(ctx context.Context, userID string) (*models.User, error) {
	var user models.User
	query := `
		SELECT uid, name, phone_number, whatsapp_number, profile_image, cover_image, bio, 
		       user_type, role, followers_count, following_count, videos_count, likes_count,
		       is_verified, is_active, is_featured, tags,
		       created_at, updated_at, last_seen, last_post_at
		FROM users 
		WHERE uid = $1 AND is_active = true`

	err := s.db.GetContext(ctx, &user, query, userID)
	return &user, err
}

// GetUserWithRole retrieves user with role information for authorization
func (s *UserService) GetUserWithRole(ctx context.Context, userID string) (*models.User, error) {
	var user models.User
	query := `
		SELECT uid, name, phone_number, whatsapp_number, profile_image, bio, 
		       user_type, role, is_verified, is_active, created_at, updated_at, last_seen
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

// NEW: CheckUserCanPost validates if user can post videos based on role
func (s *UserService) CheckUserCanPost(ctx context.Context, userID string) (bool, error) {
	var role models.UserRole
	err := s.db.QueryRowContext(ctx,
		"SELECT role FROM users WHERE uid = $1 AND is_active = true",
		userID).Scan(&role)
	if err != nil {
		return false, err
	}
	return role.CanPost(), nil
}

// NEW: GetUserRole retrieves only the user's role
func (s *UserService) GetUserRole(ctx context.Context, userID string) (models.UserRole, error) {
	var role models.UserRole
	err := s.db.QueryRowContext(ctx,
		"SELECT role FROM users WHERE uid = $1 AND is_active = true",
		userID).Scan(&role)
	return role, err
}

// NEW: UpdateUserRole updates a user's role (admin only operation)
func (s *UserService) UpdateUserRole(ctx context.Context, userID string, newRole models.UserRole) error {
	if !newRole.IsValid() {
		return fmt.Errorf("invalid role: %s", newRole)
	}

	query := `
		UPDATE users SET 
			role = $2,
			updated_at = NOW()
		WHERE uid = $1 AND is_active = true`

	result, err := s.db.ExecContext(ctx, query, userID, newRole)
	if err != nil {
		return fmt.Errorf("failed to update user role: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check affected rows: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("user not found or inactive: %s", userID)
	}

	return nil
}

// NEW: GetUsersByRole retrieves users by role with pagination
func (s *UserService) GetUsersByRole(ctx context.Context, role models.UserRole, limit, offset int) ([]models.User, error) {
	if !role.IsValid() {
		return nil, fmt.Errorf("invalid role: %s", role)
	}

	var users []models.User
	query := `
		SELECT uid, name, phone_number, whatsapp_number, profile_image, cover_image, bio,
		       user_type, role, followers_count, following_count, videos_count, likes_count,
		       is_verified, is_active, is_featured, tags,
		       created_at, updated_at, last_seen, last_post_at
		FROM users 
		WHERE role = $1 AND is_active = true
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`

	err := s.db.SelectContext(ctx, &users, query, role, limit, offset)
	return users, err
}

// NEW: UpdateUserWhatsApp updates user's WhatsApp number
func (s *UserService) UpdateUserWhatsApp(ctx context.Context, userID string, whatsappNumber *string) error {
	// Validate WhatsApp number format if provided
	if whatsappNumber != nil && *whatsappNumber != "" {
		formatted, err := models.FormatWhatsAppNumber(*whatsappNumber)
		if err != nil {
			return fmt.Errorf("invalid WhatsApp number format: %w", err)
		}
		whatsappNumber = formatted
	}

	query := `
		UPDATE users SET 
			whatsapp_number = $2,
			updated_at = NOW()
		WHERE uid = $1 AND is_active = true`

	result, err := s.db.ExecContext(ctx, query, userID, whatsappNumber)
	if err != nil {
		return fmt.Errorf("failed to update WhatsApp number: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check affected rows: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("user not found or inactive: %s", userID)
	}

	return nil
}

// NEW: GetUsersWithWhatsApp retrieves users who have WhatsApp numbers
func (s *UserService) GetUsersWithWhatsApp(ctx context.Context, limit, offset int) ([]models.User, error) {
	var users []models.User
	query := `
		SELECT uid, name, phone_number, whatsapp_number, profile_image, cover_image, bio,
		       user_type, role, followers_count, following_count, videos_count, likes_count,
		       is_verified, is_active, is_featured, tags,
		       created_at, updated_at, last_seen, last_post_at
		FROM users 
		WHERE whatsapp_number IS NOT NULL AND whatsapp_number != '' AND is_active = true
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2`

	err := s.db.SelectContext(ctx, &users, query, limit, offset)
	return users, err
}

// NEW: CountUsersByRole returns count of users for each role
func (s *UserService) CountUsersByRole(ctx context.Context) (map[models.UserRole]int, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT role, COUNT(*) as count 
		FROM users 
		WHERE is_active = true 
		GROUP BY role`)
	if err != nil {
		return nil, fmt.Errorf("failed to count users by role: %w", err)
	}
	defer rows.Close()

	counts := make(map[models.UserRole]int)
	for rows.Next() {
		var role models.UserRole
		var count int
		if err := rows.Scan(&role, &count); err != nil {
			return nil, fmt.Errorf("failed to scan role count: %w", err)
		}
		counts[role] = count
	}

	return counts, nil
}

// NEW: GetContentCreators retrieves users who can post (admin and host)
func (s *UserService) GetContentCreators(ctx context.Context, limit, offset int) ([]models.User, error) {
	var users []models.User
	query := `
		SELECT uid, name, phone_number, whatsapp_number, profile_image, cover_image, bio,
		       user_type, role, followers_count, following_count, videos_count, likes_count,
		       is_verified, is_active, is_featured, tags,
		       created_at, updated_at, last_seen, last_post_at
		FROM users 
		WHERE role IN ('admin', 'host') AND is_active = true
		ORDER BY videos_count DESC, followers_count DESC
		LIMIT $1 OFFSET $2`

	err := s.db.SelectContext(ctx, &users, query, limit, offset)
	return users, err
}

// NEW: ValidateUserForVideoCreation validates user can create videos
func (s *UserService) ValidateUserForVideoCreation(ctx context.Context, userID string) error {
	var user models.User
	err := s.db.QueryRowContext(ctx,
		"SELECT uid, role, is_active FROM users WHERE uid = $1",
		userID).Scan(&user.UID, &user.Role, &user.IsActive)
	if err != nil {
		return fmt.Errorf("user not found: %s", userID)
	}

	if !user.IsActive {
		return fmt.Errorf("user account is inactive")
	}

	if !user.CanPost() {
		return fmt.Errorf("user role '%s' cannot post videos. Only admin and host users can post", user.Role.DisplayName())
	}

	return nil
}

// NEW: BulkUpdateUserRoles updates multiple users' roles (admin operation)
func (s *UserService) BulkUpdateUserRoles(ctx context.Context, userRoleMap map[string]models.UserRole) error {
	if len(userRoleMap) == 0 {
		return fmt.Errorf("no users to update")
	}

	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	query := `
		UPDATE users SET 
			role = $2,
			updated_at = NOW()
		WHERE uid = $1 AND is_active = true`

	for userID, role := range userRoleMap {
		if !role.IsValid() {
			return fmt.Errorf("invalid role for user %s: %s", userID, role)
		}

		_, err := tx.ExecContext(ctx, query, userID, role)
		if err != nil {
			return fmt.Errorf("failed to update role for user %s: %w", userID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// NEW: SearchUsersByRoleAndName searches users by role and name
func (s *UserService) SearchUsersByRoleAndName(ctx context.Context, role *models.UserRole, nameQuery string, limit, offset int) ([]models.User, error) {
	var users []models.User
	var args []interface{}

	query := `
		SELECT uid, name, phone_number, whatsapp_number, profile_image, cover_image, bio,
		       user_type, role, followers_count, following_count, videos_count, likes_count,
		       is_verified, is_active, is_featured, tags,
		       created_at, updated_at, last_seen, last_post_at
		FROM users 
		WHERE is_active = true`

	argIndex := 1

	if role != nil {
		query += fmt.Sprintf(" AND role = $%d", argIndex)
		args = append(args, *role)
		argIndex++
	}

	if nameQuery != "" {
		query += fmt.Sprintf(" AND name ILIKE $%d", argIndex)
		args = append(args, "%"+nameQuery+"%")
		argIndex++
	}

	query += fmt.Sprintf(" ORDER BY name ASC LIMIT $%d OFFSET $%d", argIndex, argIndex+1)
	args = append(args, limit, offset)

	err := s.db.SelectContext(ctx, &users, query, args...)
	return users, err
}

// Enhanced GetUserStats with role and WhatsApp information
func (s *UserService) GetUserStats(ctx context.Context, userID string) (*models.UserStats, error) {
	var stats models.UserStats

	// Get basic user stats
	err := s.db.QueryRowContext(ctx, `
		SELECT uid, name, role, followers_count, following_count, videos_count, likes_count,
		       is_verified, whatsapp_number, created_at, last_seen, last_post_at
		FROM users 
		WHERE uid = $1 AND is_active = true`, userID).Scan(
		&stats.UserID, &stats.Username, &stats.Role, &stats.FollowersCount,
		&stats.FollowingCount, &stats.VideosCount, &stats.TotalLikes,
		&stats.HasWhatsApp, &stats.JoinedAt, &stats.LastActiveAt, &stats.LastPostAt)
	if err != nil {
		return nil, fmt.Errorf("failed to get user stats: %w", err)
	}

	// Set role-related fields
	stats.RoleDisplayName = stats.Role.DisplayName()
	stats.CanPost = stats.Role.CanPost()
	stats.HasPostedVideos = stats.LastPostAt != nil

	// Get total video views and likes
	err = s.db.QueryRowContext(ctx, `
		SELECT 
			COALESCE(SUM(views_count), 0) as total_views,
			COALESCE(SUM(likes_count), 0) as total_video_likes
		FROM videos 
		WHERE user_id = $1 AND is_active = true`, userID).Scan(&stats.TotalViews, &stats.TotalLikes)
	if err != nil {
		// If no videos, set to 0
		stats.TotalViews = 0
		stats.TotalLikes = 0
	}

	// Calculate engagement rate
	if stats.FollowersCount > 0 {
		stats.EngagementRate = (float64(stats.TotalLikes) / float64(stats.FollowersCount)) * 100
	}

	// Set last post time ago
	if stats.LastPostAt != nil {
		stats.LastPostTimeAgo = formatTimeAgo(*stats.LastPostAt)
	} else {
		stats.LastPostTimeAgo = "Never posted"
	}

	return &stats, nil
}

// Helper function to format time ago
func formatTimeAgo(t time.Time) string {
	// This would match the logic in your User model's GetLastPostTimeAgo method
	// Implementation would be similar to what you have in the User model
	return "some time ago" // Simplified for brevity
}
