// ===============================
// internal/models/user.go - UPDATED User Model with Drama Fields + LastPostAt
// ===============================

package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

// IntMap represents a map[string]int that can be stored in PostgreSQL as JSONB
type IntMap map[string]int

// Value implements driver.Valuer for database storage
func (m IntMap) Value() (driver.Value, error) {
	if m == nil {
		return nil, nil
	}
	return json.Marshal(m)
}

// Scan implements sql.Scanner for database retrieval
func (m *IntMap) Scan(value interface{}) error {
	if value == nil {
		*m = make(IntMap)
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("cannot scan %T into IntMap", value)
	}

	return json.Unmarshal(bytes, m)
}

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

	// NEW: Drama-related fields
	FavoriteDramas StringSlice `json:"favoriteDramas" db:"favorite_dramas"`
	UnlockedDramas StringSlice `json:"unlockedDramas" db:"unlocked_dramas"`
	DramaProgress  IntMap      `json:"dramaProgress" db:"drama_progress"`

	CreatedAt  time.Time  `json:"createdAt" db:"created_at"`
	UpdatedAt  time.Time  `json:"updatedAt" db:"updated_at"`
	LastSeen   time.Time  `json:"lastSeen" db:"last_seen"`
	LastPostAt *time.Time `json:"lastPostAt" db:"last_post_at"` // NEW: Last video post timestamp

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

// NEW: Check if user can create dramas (verified users only)
func (u *User) CanCreateDramas() bool {
	return u.IsVerified && u.IsActive
}

// NEW: Check if user has unlocked a specific drama
func (u *User) HasUnlockedDrama(dramaID string) bool {
	for _, id := range u.UnlockedDramas {
		if id == dramaID {
			return true
		}
	}
	return false
}

// NEW: Check if user has favorited a specific drama
func (u *User) HasFavoritedDrama(dramaID string) bool {
	for _, id := range u.FavoriteDramas {
		if id == dramaID {
			return true
		}
	}
	return false
}

// NEW: Get drama progress for a specific drama
func (u *User) GetDramaProgress(dramaID string) int {
	if u.DramaProgress == nil {
		return 0
	}
	return u.DramaProgress[dramaID]
}

// NEW: LastPostAt helper methods
func (u *User) HasPostedVideos() bool {
	return u.LastPostAt != nil
}

func (u *User) GetLastPostTime() *time.Time {
	return u.LastPostAt
}

func (u *User) GetTimeSinceLastPost() *time.Duration {
	if u.LastPostAt == nil {
		return nil
	}
	duration := time.Since(*u.LastPostAt)
	return &duration
}

func (u *User) GetLastPostTimeAgo() string {
	if u.LastPostAt == nil {
		return "Never posted"
	}

	now := time.Now()
	difference := now.Sub(*u.LastPostAt)

	if difference.Hours() > 8760 { // More than 1 year
		years := int(difference.Hours() / 8760)
		return fmt.Sprintf("%dy ago", years)
	} else if difference.Hours() > 720 { // More than 1 month
		months := int(difference.Hours() / 720)
		return fmt.Sprintf("%dmo ago", months)
	} else if difference.Hours() > 24 { // More than 1 day
		days := int(difference.Hours() / 24)
		return fmt.Sprintf("%dd ago", days)
	} else if difference.Hours() > 1 { // More than 1 hour
		hours := int(difference.Hours())
		return fmt.Sprintf("%dh ago", hours)
	} else if difference.Minutes() > 1 { // More than 1 minute
		minutes := int(difference.Minutes())
		return fmt.Sprintf("%dm ago", minutes)
	} else {
		return "Just now"
	}
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
	// NEW: Drama-related response fields
	CreatedDramasCount  int `json:"createdDramasCount,omitempty"`
	FavoriteDramasCount int `json:"favoriteDramasCount"`
	UnlockedDramasCount int `json:"unlockedDramasCount"`
	// NEW: Last post related fields
	HasPostedVideos bool   `json:"hasPostedVideos"`
	LastPostTimeAgo string `json:"lastPostTimeAgo"`
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
	UserID         string  `json:"userId"`
	Username       string  `json:"username"`
	FollowersCount int     `json:"followersCount"`
	FollowingCount int     `json:"followingCount"`
	VideosCount    int     `json:"videosCount"`
	TotalLikes     int     `json:"totalLikes"`
	TotalViews     int     `json:"totalViews"`
	EngagementRate float64 `json:"engagementRate"`

	// NEW: Drama-related stats
	CreatedDramasCount  int `json:"createdDramasCount"`
	DramaRevenue        int `json:"dramaRevenue,omitempty"` // Only for verified users
	FavoriteDramasCount int `json:"favoriteDramasCount"`
	UnlockedDramasCount int `json:"unlockedDramasCount"`

	// NEW: Last post related stats
	HasPostedVideos bool       `json:"hasPostedVideos"`
	LastPostAt      *time.Time `json:"lastPostAt"`
	LastPostTimeAgo string     `json:"lastPostTimeAgo"`

	JoinedAt     time.Time `json:"joinedAt"`
	LastActiveAt time.Time `json:"lastActiveAt"`
}

// Activity tracking
type UserActivity struct {
	UserID       string    `json:"userId"`
	ActivityType string    `json:"activityType"` // "video_posted", "drama_created", "drama_unlocked", etc.
	TargetID     string    `json:"targetId"`     // Video ID, Drama ID, Comment ID, etc.
	TargetType   string    `json:"targetType"`   // "video", "drama", "comment", "user"
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
