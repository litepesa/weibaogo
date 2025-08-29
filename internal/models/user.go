// ===============================
// internal/models/user.go - Minimal Update (Only Remove CoinsBalance)
// ===============================

package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"
)

type User struct {
	UID          string `json:"uid" db:"uid"`
	Name         string `json:"name" db:"name"`
	Email        string `json:"email" db:"email"`
	PhoneNumber  string `json:"phoneNumber" db:"phone_number"`
	ProfileImage string `json:"profileImage" db:"profile_image"`
	Bio          string `json:"bio" db:"bio"`
	UserType     string `json:"userType" db:"user_type"`
	// REMOVED: CoinsBalance   int             `json:"coinsBalance" db:"coins_balance"`
	FavoriteDramas StringSlice     `json:"favoriteDramas" db:"favorite_dramas"`
	WatchHistory   StringSlice     `json:"watchHistory" db:"watch_history"`
	DramaProgress  IntMap          `json:"dramaProgress" db:"drama_progress"`
	UnlockedDramas StringSlice     `json:"unlockedDramas" db:"unlocked_dramas"`
	Preferences    UserPreferences `json:"preferences" db:"preferences"`
	CreatedAt      time.Time       `json:"createdAt" db:"created_at"`
	UpdatedAt      time.Time       `json:"updatedAt" db:"updated_at"`
	LastSeen       time.Time       `json:"lastSeen" db:"last_seen"`
}

type UserPreferences struct {
	AutoPlay             bool `json:"autoPlay"`
	ReceiveNotifications bool `json:"receiveNotifications"`
	DarkMode             bool `json:"darkMode"`
}

// REMOVE THIS DUPLICATE DEFINITION:
// type StringSlice []string
//
// func (s StringSlice) Value() (driver.Value, error) {
// 	return json.Marshal(s)
// }
//
// func (s *StringSlice) Scan(value interface{}) error {
// 	if value == nil {
// 		*s = StringSlice{}
// 		return nil
// 	}
// 	return json.Unmarshal(value.([]byte), s)
// }

type IntMap map[string]int

func (m IntMap) Value() (driver.Value, error) {
	return json.Marshal(m)
}

func (m *IntMap) Scan(value interface{}) error {
	if value == nil {
		*m = IntMap{}
		return nil
	}
	return json.Unmarshal(value.([]byte), m)
}

func (p UserPreferences) Value() (driver.Value, error) {
	return json.Marshal(p)
}

func (p *UserPreferences) Scan(value interface{}) error {
	if value == nil {
		*p = UserPreferences{}
		return nil
	}
	return json.Unmarshal(value.([]byte), p)
}

// Helper methods
func (u *User) IsAdmin() bool {
	return u.UserType == "admin"
}

func (u *User) HasFavorited(dramaID string) bool {
	for _, id := range u.FavoriteDramas {
		if id == dramaID {
			return true
		}
	}
	return false
}

func (u *User) HasUnlocked(dramaID string) bool {
	for _, id := range u.UnlockedDramas {
		if id == dramaID {
			return true
		}
	}
	return false
}

func (u *User) GetDramaProgress(dramaID string) int {
	return u.DramaProgress[dramaID]
}
