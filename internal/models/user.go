// ===============================
// internal/models/user.go - Video Social Media User Model
// ===============================

package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

type User struct {
	UID            string      `json:"uid" db:"uid"`
	Name           string      `json:"name" db:"name" binding:"required"`
	PhoneNumber    string      `json:"phoneNumber" db:"phone_number" binding:"required"`
	ProfileImage   string      `json:"profileImage" db:"profile_image"`
	CoverImage     string      `json:"coverImage" db:"cover_image"`
	Bio            string      `json:"bio" db:"bio"`
	UserType       string      `json:"userType" db:"user_type"`
	FollowersCount int         `json:"followersCount" db:"followers_count"`
	FollowingCount int         `json:"followingCount" db:"following_count"`
	VideosCount    int         `json:"videosCount" db:"videos_count"`
	LikesCount     int         `json:"likesCount" db:"likes_count"`
	IsVerified     bool        `json:"isVerified" db:"is_verified"`
	IsActive       bool        `json:"isActive" db:"is_active"`
	IsFeatured     bool        `json:"isFeatured" db:"is_featured"`
	Tags           StringSlice `json:"tags" db:"tags"`
	CreatedAt      time.Time   `json:"createdAt" db:"created_at"`
	UpdatedAt      time.Time   `json:"updatedAt" db:"updated_at"`
	LastSeen       time.Time   `json:"lastSeen" db:"last_seen"`

	// Runtime fields (not stored in DB)
	IsFollowing   bool `json:"isFollowing" db:"-"`
	IsCurrentUser bool `json:"isCurrentUser" db:"-"`
}

type UserPreferences struct {
	AutoPlay             bool   `json:"autoPlay"`
	ReceiveNotifications bool   `json:"receiveNotifications"`
	DarkMode             bool   `json:"darkMode"`
	Language             string `json:"language"`
	ShowPhoneNumber      bool   `json:"showPhoneNumber"`
}

func (p UserPreferences) Value() (driver.Value, error) {
	return json.Marshal(p)
}

func (p *UserPreferences) Scan(value interface{}) error {
	if value == nil {
		*p = UserPreferences{
			AutoPlay:             true,
			ReceiveNotifications: true,
			DarkMode:             false,
			Language:             "en",
			ShowPhoneNumber:      false,
		}
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("cannot scan %T into UserPreferences", value)
	}

	return json.Unmarshal(bytes, p)
}

// Helper methods
func (u *User) IsAdmin() bool {
	return u.UserType == "admin"
}

func (u *User) IsModerator() bool {
	return u.UserType == "moderator" || u.UserType == "admin"
}

func (u *User) CanModerate() bool {
	return u.IsModerator()
}

func (u *User) CanManageUsers() bool {
	return u.IsAdmin()
}

func (u *User) HasMinimumFollowers(min int) bool {
	return u.FollowersCount >= min
}

func (u *User) GetEngagementRate() float64 {
	if u.FollowersCount == 0 {
		return 0.0
	}
	// Simple engagement calculation based on videos and followers
	return (float64(u.VideosCount) / float64(u.FollowersCount)) * 100
}

func (u *User) GetDisplayName() string {
	if u.Name != "" {
		return u.Name
	}
	return u.PhoneNumber
}

func (u *User) GetProfileImageOrDefault() string {
	if u.ProfileImage != "" {
		return u.ProfileImage
	}
	// Return a default avatar URL or placeholder
	return "/assets/default-avatar.png"
}

// Validation methods
func (u *User) ValidateForCreation() []string {
	var errors []string

	if u.UID == "" {
		errors = append(errors, "UID is required")
	}

	if u.Name == "" {
		errors = append(errors, "Name is required")
	}

	if len(u.Name) < 2 {
		errors = append(errors, "Name must be at least 2 characters")
	}

	if len(u.Name) > 50 {
		errors = append(errors, "Name cannot exceed 50 characters")
	}

	if u.PhoneNumber == "" {
		errors = append(errors, "Phone number is required")
	}

	if len(u.Bio) > 160 {
		errors = append(errors, "Bio cannot exceed 160 characters")
	}

	if u.UserType != "" && !isValidUserType(u.UserType) {
		errors = append(errors, "Invalid user type")
	}

	return errors
}

func (u *User) IsValidForCreation() bool {
	return len(u.ValidateForCreation()) == 0
}

func isValidUserType(userType string) bool {
	validTypes := []string{"user", "admin", "moderator"}
	for _, vt := range validTypes {
		if userType == vt {
			return true
		}
	}
	return false
}

// User creation request models
type CreateUserRequest struct {
	Name         string `json:"name" binding:"required"`
	PhoneNumber  string `json:"phoneNumber" binding:"required"`
	ProfileImage string `json:"profileImage"`
	Bio          string `json:"bio"`
}

type UpdateUserRequest struct {
	Name         string   `json:"name"`
	ProfileImage string   `json:"profileImage"`
	CoverImage   string   `json:"coverImage"`
	Bio          string   `json:"bio"`
	Tags         []string `json:"tags"`
}

// User response models
type UserResponse struct {
	User
	MutualFollowersCount int     `json:"mutualFollowersCount"`
	RecentVideos         []Video `json:"recentVideos,omitempty"`
}

type UserListResponse struct {
	Users   []UserResponse `json:"users"`
	HasMore bool           `json:"hasMore"`
	Total   int            `json:"total"`
}

// Social interaction models
type FollowRequest struct {
	TargetUserID string `json:"targetUserId" binding:"required"`
}

type UserSearchParams struct {
	Query    string `json:"query"`
	UserType string `json:"userType"`
	Verified *bool  `json:"verified"`
	Featured *bool  `json:"featured"`
	Limit    int    `json:"limit"`
	Offset   int    `json:"offset"`
}

// User statistics
type UserStats struct {
	UserID         string    `json:"userId"`
	Username       string    `json:"username"`
	FollowersCount int       `json:"followersCount"`
	FollowingCount int       `json:"followingCount"`
	VideosCount    int       `json:"videosCount"`
	TotalLikes     int       `json:"totalLikes"`
	TotalViews     int       `json:"totalViews"`
	EngagementRate float64   `json:"engagementRate"`
	JoinedAt       time.Time `json:"joinedAt"`
	LastActiveAt   time.Time `json:"lastActiveAt"`
}

// Activity tracking
type UserActivity struct {
	UserID       string    `json:"userId"`
	ActivityType string    `json:"activityType"` // "video_posted", "comment_added", "like_given", etc.
	TargetID     string    `json:"targetId"`     // Video ID, Comment ID, etc.
	TargetType   string    `json:"targetType"`   // "video", "comment", "user"
	CreatedAt    time.Time `json:"createdAt"`
}

// Constants for user limits
const (
	MaxNameLength       = 50
	MaxBioLength        = 160
	MaxTagsPerUser      = 10
	MaxFollowingLimit   = 7500 // TikTok-style following limit
	MinUsernameLength   = 2
	MaxProfileImageSize = 10 * 1024 * 1024 // 10MB
	MaxCoverImageSize   = 15 * 1024 * 1024 // 15MB
)

// User types
const (
	UserTypeUser      = "user"
	UserTypeAdmin     = "admin"
	UserTypeModerator = "moderator"
)
