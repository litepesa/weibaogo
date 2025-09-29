// ===============================
// internal/models/user.go - UPDATED User Model with Gender, Location, and Language
// ===============================

package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// UserRole represents the user role enum
type UserRole string

const (
	UserRoleAdmin UserRole = "admin"
	UserRoleHost  UserRole = "host"
	UserRoleGuest UserRole = "guest"
)

// String returns the string representation of UserRole
func (r UserRole) String() string {
	return string(r)
}

// IsValid checks if the user role is valid
func (r UserRole) IsValid() bool {
	switch r {
	case UserRoleAdmin, UserRoleHost, UserRoleGuest:
		return true
	}
	return false
}

// CanPost returns true if the user role can post videos
func (r UserRole) CanPost() bool {
	return r == UserRoleAdmin || r == UserRoleHost || r == UserRoleGuest
}

// DisplayName returns the display name for the role
func (r UserRole) DisplayName() string {
	switch r {
	case UserRoleAdmin:
		return "Admin"
	case UserRoleHost:
		return "Host"
	case UserRoleGuest:
		return "Guest"
	default:
		return "Guest"
	}
}

// ParseUserRole parses a string to UserRole
func ParseUserRole(s string) UserRole {
	switch s {
	case "admin":
		return UserRoleAdmin
	case "host":
		return UserRoleHost
	case "guest":
		return UserRoleGuest
	default:
		return UserRoleGuest
	}
}

// UserGender represents the user gender enum
type UserGender string

const (
	UserGenderMale   UserGender = "male"
	UserGenderFemale UserGender = "female"
)

// String returns the string representation of UserGender
func (g UserGender) String() string {
	return string(g)
}

// IsValid checks if the user gender is valid
func (g UserGender) IsValid() bool {
	return g == UserGenderMale || g == UserGenderFemale
}

// DisplayName returns the display name for the gender
func (g UserGender) DisplayName() string {
	switch g {
	case UserGenderMale:
		return "Male"
	case UserGenderFemale:
		return "Female"
	default:
		return ""
	}
}

// ParseUserGender parses a string to UserGender
func ParseUserGender(s string) *UserGender {
	if s == "" {
		return nil
	}
	switch strings.ToLower(s) {
	case "male":
		gender := UserGenderMale
		return &gender
	case "female":
		gender := UserGenderFemale
		return &gender
	default:
		return nil
	}
}

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
	WhatsappNumber *string     `json:"whatsappNumber" db:"whatsapp_number"`
	ProfileImage   string      `json:"profileImage" db:"profile_image"`
	CoverImage     string      `json:"coverImage" db:"cover_image"`
	Bio            string      `json:"bio" db:"bio"`
	UserType       string      `json:"userType" db:"user_type"` // Keep for backward compatibility
	Role           UserRole    `json:"role" db:"role"`
	Gender         *string     `json:"gender" db:"gender"`     // NEW: User gender (male/female)
	Location       *string     `json:"location" db:"location"` // NEW: User location (e.g., "Nairobi, Kenya")
	Language       *string     `json:"language" db:"language"` // NEW: User native language (e.g., "English", "Swahili")
	FollowersCount int         `json:"followersCount" db:"followers_count"`
	FollowingCount int         `json:"followingCount" db:"following_count"`
	VideosCount    int         `json:"videosCount" db:"videos_count"`
	LikesCount     int         `json:"likesCount" db:"likes_count"`
	IsVerified     bool        `json:"isVerified" db:"is_verified"`
	IsActive       bool        `json:"isActive" db:"is_active"`
	IsFeatured     bool        `json:"isFeatured" db:"is_featured"`
	Tags           StringSlice `json:"tags" db:"tags"`

	// Drama-related fields (keeping for compatibility)
	FavoriteDramas StringSlice `json:"favoriteDramas" db:"favorite_dramas"`
	UnlockedDramas StringSlice `json:"unlockedDramas" db:"unlocked_dramas"`
	DramaProgress  IntMap      `json:"dramaProgress" db:"drama_progress"`

	CreatedAt  time.Time  `json:"createdAt" db:"created_at"`
	UpdatedAt  time.Time  `json:"updatedAt" db:"updated_at"`
	LastSeen   time.Time  `json:"lastSeen" db:"last_seen"`
	LastPostAt *time.Time `json:"lastPostAt" db:"last_post_at"`

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
	return u.Role == UserRoleAdmin || u.UserType == "admin"
}

func (u *User) IsHost() bool {
	return u.Role == UserRoleHost
}

func (u *User) IsGuest() bool {
	return u.Role == UserRoleGuest
}

func (u *User) IsModerator() bool {
	return u.UserType == "moderator" || u.UserType == "admin" || u.Role == UserRoleAdmin
}

func (u *User) CanModerate() bool {
	return u.IsModerator()
}

func (u *User) CanManageUsers() bool {
	return u.IsAdmin()
}

func (u *User) CanPost() bool {
	return u.Role.CanPost()
}

// NEW: Gender helper methods
func (u *User) HasGender() bool {
	return u.Gender != nil && *u.Gender != ""
}

func (u *User) GetGender() *UserGender {
	if !u.HasGender() {
		return nil
	}
	return ParseUserGender(*u.Gender)
}

func (u *User) GetGenderDisplay() string {
	gender := u.GetGender()
	if gender == nil {
		return "Not specified"
	}
	return gender.DisplayName()
}

func (u *User) IsMale() bool {
	gender := u.GetGender()
	return gender != nil && *gender == UserGenderMale
}

func (u *User) IsFemale() bool {
	gender := u.GetGender()
	return gender != nil && *gender == UserGenderFemale
}

// NEW: Location helper methods
func (u *User) HasLocation() bool {
	return u.Location != nil && *u.Location != ""
}

func (u *User) GetLocation() string {
	if !u.HasLocation() {
		return ""
	}
	return *u.Location
}

func (u *User) GetLocationDisplay() string {
	if !u.HasLocation() {
		return "Location not set"
	}
	return *u.Location
}

// NEW: Language helper methods
func (u *User) HasLanguage() bool {
	return u.Language != nil && *u.Language != ""
}

func (u *User) GetLanguage() string {
	if !u.HasLanguage() {
		return ""
	}
	return *u.Language
}

func (u *User) GetLanguageDisplay() string {
	if !u.HasLanguage() {
		return "Language not set"
	}
	return *u.Language
}

// WhatsApp helper methods
func (u *User) HasWhatsApp() bool {
	return u.WhatsappNumber != nil && *u.WhatsappNumber != ""
}

func (u *User) GetWhatsAppLink() *string {
	if !u.HasWhatsApp() {
		return nil
	}
	link := fmt.Sprintf("https://wa.me/%s", *u.WhatsappNumber)
	return &link
}

func (u *User) GetWhatsAppLinkWithMessage() *string {
	if !u.HasWhatsApp() {
		return nil
	}
	message := fmt.Sprintf("Hi %s! I found your profile on the app.", u.Name)
	encodedMessage := strings.ReplaceAll(message, " ", "%20")
	encodedMessage = strings.ReplaceAll(encodedMessage, "!", "%21")
	link := fmt.Sprintf("https://wa.me/%s?text=%s", *u.WhatsappNumber, encodedMessage)
	return &link
}

func (u *User) ValidateWhatsAppNumber() error {
	if u.WhatsappNumber == nil || *u.WhatsappNumber == "" {
		return nil
	}

	matched, err := regexp.MatchString(`^254[0-9]{9}$`, *u.WhatsappNumber)
	if err != nil {
		return fmt.Errorf("error validating WhatsApp number: %w", err)
	}
	if !matched {
		return fmt.Errorf("WhatsApp number must be in format 254XXXXXXXXX")
	}

	return nil
}

func FormatWhatsAppNumber(input string) (*string, error) {
	if input == "" {
		return nil, nil
	}

	re := regexp.MustCompile(`\D`)
	cleaned := re.ReplaceAllString(input, "")

	switch {
	case len(cleaned) == 12 && cleaned[:3] == "254":
		return &cleaned, nil
	case len(cleaned) == 10 && cleaned[0] == '0':
		formatted := "254" + cleaned[1:]
		return &formatted, nil
	case len(cleaned) == 9:
		formatted := "254" + cleaned
		return &formatted, nil
	default:
		return nil, fmt.Errorf("invalid phone number format: %s", input)
	}
}

func (u *User) CanCreateDramas() bool {
	return u.IsVerified && u.IsActive
}

func (u *User) HasUnlockedDrama(dramaID string) bool {
	for _, id := range u.UnlockedDramas {
		if id == dramaID {
			return true
		}
	}
	return false
}

func (u *User) HasFavoritedDrama(dramaID string) bool {
	for _, id := range u.FavoriteDramas {
		if id == dramaID {
			return true
		}
	}
	return false
}

func (u *User) GetDramaProgress(dramaID string) int {
	if u.DramaProgress == nil {
		return 0
	}
	return u.DramaProgress[dramaID]
}

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

	if difference.Hours() > 8760 {
		years := int(difference.Hours() / 8760)
		return fmt.Sprintf("%dy ago", years)
	} else if difference.Hours() > 720 {
		months := int(difference.Hours() / 720)
		return fmt.Sprintf("%dmo ago", months)
	} else if difference.Hours() > 24 {
		days := int(difference.Hours() / 24)
		return fmt.Sprintf("%dd ago", days)
	} else if difference.Hours() > 1 {
		hours := int(difference.Hours())
		return fmt.Sprintf("%dh ago", hours)
	} else if difference.Minutes() > 1 {
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

	if !u.Role.IsValid() {
		errors = append(errors, "Invalid user role")
	}

	// Validate WhatsApp number
	if err := u.ValidateWhatsAppNumber(); err != nil {
		errors = append(errors, err.Error())
	}

	// NEW: Validate gender
	if u.Gender != nil && *u.Gender != "" {
		gender := ParseUserGender(*u.Gender)
		if gender == nil {
			errors = append(errors, "Gender must be either 'male' or 'female'")
		}
	}

	// NEW: Validate location length
	if u.Location != nil && len(*u.Location) > 255 {
		errors = append(errors, "Location cannot exceed 255 characters")
	}

	// NEW: Validate language length
	if u.Language != nil && len(*u.Language) > 100 {
		errors = append(errors, "Language cannot exceed 100 characters")
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
	Name           string  `json:"name" binding:"required"`
	PhoneNumber    string  `json:"phoneNumber" binding:"required"`
	WhatsappNumber *string `json:"whatsappNumber"`
	ProfileImage   string  `json:"profileImage"`
	Bio            string  `json:"bio"`
	Role           *string `json:"role"`
	Gender         *string `json:"gender"`   // NEW: Optional gender
	Location       *string `json:"location"` // NEW: Optional location
	Language       *string `json:"language"` // NEW: Optional language
}

type UpdateUserRequest struct {
	Name           string   `json:"name"`
	ProfileImage   string   `json:"profileImage"`
	CoverImage     string   `json:"coverImage"`
	Bio            string   `json:"bio"`
	Tags           []string `json:"tags"`
	WhatsappNumber *string  `json:"whatsappNumber"`
	Gender         *string  `json:"gender"`   // NEW: Optional gender
	Location       *string  `json:"location"` // NEW: Optional location
	Language       *string  `json:"language"` // NEW: Optional language
}

// User response models
type UserResponse struct {
	User
	MutualFollowersCount    int     `json:"mutualFollowersCount"`
	RecentVideos            []Video `json:"recentVideos,omitempty"`
	RoleDisplayName         string  `json:"roleDisplayName"`
	CanPost                 bool    `json:"canPost"`
	HasWhatsApp             bool    `json:"hasWhatsApp"`
	WhatsAppLink            *string `json:"whatsAppLink,omitempty"`
	WhatsAppLinkWithMessage *string `json:"whatsAppLinkWithMessage,omitempty"`
	// NEW: Profile fields
	GenderDisplay   string `json:"genderDisplay"`
	LocationDisplay string `json:"locationDisplay"`
	LanguageDisplay string `json:"languageDisplay"`
	HasGender       bool   `json:"hasGender"`
	HasLocation     bool   `json:"hasLocation"`
	HasLanguage     bool   `json:"hasLanguage"`
	// Drama-related
	CreatedDramasCount  int    `json:"createdDramasCount,omitempty"`
	FavoriteDramasCount int    `json:"favoriteDramasCount"`
	UnlockedDramasCount int    `json:"unlockedDramasCount"`
	HasPostedVideos     bool   `json:"hasPostedVideos"`
	LastPostTimeAgo     string `json:"lastPostTimeAgo"`
}

type UserListResponse struct {
	Users   []UserResponse `json:"users"`
	HasMore bool           `json:"hasMore"`
	Total   int            `json:"total"`
}

type FollowRequest struct {
	TargetUserID string `json:"targetUserId" binding:"required"`
}

type UserSearchParams struct {
	Query    string    `json:"query"`
	UserType string    `json:"userType"`
	Role     *UserRole `json:"role"`
	Gender   *string   `json:"gender"`   // NEW: Filter by gender
	Location *string   `json:"location"` // NEW: Filter by location
	Language *string   `json:"language"` // NEW: Filter by language
	Verified *bool     `json:"verified"`
	Featured *bool     `json:"featured"`
	Limit    int       `json:"limit"`
	Offset   int       `json:"offset"`
}

type UserStats struct {
	UserID         string  `json:"userId"`
	Username       string  `json:"username"`
	FollowersCount int     `json:"followersCount"`
	FollowingCount int     `json:"followingCount"`
	VideosCount    int     `json:"videosCount"`
	TotalLikes     int     `json:"totalLikes"`
	TotalViews     int     `json:"totalViews"`
	EngagementRate float64 `json:"engagementRate"`

	Role            UserRole `json:"role"`
	RoleDisplayName string   `json:"roleDisplayName"`
	CanPost         bool     `json:"canPost"`

	HasWhatsApp bool `json:"hasWhatsApp"`

	// NEW: Profile stats
	Gender        *string `json:"gender,omitempty"`
	GenderDisplay string  `json:"genderDisplay"`
	Location      *string `json:"location,omitempty"`
	Language      *string `json:"language,omitempty"`

	CreatedDramasCount  int `json:"createdDramasCount"`
	DramaRevenue        int `json:"dramaRevenue,omitempty"`
	FavoriteDramasCount int `json:"favoriteDramasCount"`
	UnlockedDramasCount int `json:"unlockedDramasCount"`

	HasPostedVideos bool       `json:"hasPostedVideos"`
	LastPostAt      *time.Time `json:"lastPostAt"`
	LastPostTimeAgo string     `json:"lastPostTimeAgo"`

	JoinedAt     time.Time `json:"joinedAt"`
	LastActiveAt time.Time `json:"lastActiveAt"`
}

type UserActivity struct {
	UserID       string    `json:"userId"`
	ActivityType string    `json:"activityType"`
	TargetID     string    `json:"targetId"`
	TargetType   string    `json:"targetType"`
	CreatedAt    time.Time `json:"createdAt"`
}

const (
	MaxNameLength       = 50
	MaxBioLength        = 160
	MaxTagsPerUser      = 10
	MaxFollowingLimit   = 7500
	MinUsernameLength   = 2
	MaxProfileImageSize = 10 * 1024 * 1024
	MaxCoverImageSize   = 15 * 1024 * 1024
	MaxLocationLength   = 255 // NEW
	MaxLanguageLength   = 100 // NEW
)

const (
	UserTypeUser      = "user"
	UserTypeAdmin     = "admin"
	UserTypeModerator = "moderator"
)
