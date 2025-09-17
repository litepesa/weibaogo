// ===============================
// internal/models/contact.go - Contact Models
// ===============================

package models

import (
	"strings"
	"time"
)

// Contact represents a user's contact
type Contact struct {
	ID            string    `json:"id" db:"id"`
	UserID        string    `json:"userId" db:"user_id"`
	ContactUserID string    `json:"contactUserId" db:"contact_user_id"`
	ContactName   string    `json:"contactName" db:"contact_name"`
	ContactPhone  string    `json:"contactPhone" db:"contact_phone"`
	IsBlocked     bool      `json:"isBlocked" db:"is_blocked"`
	CreatedAt     time.Time `json:"createdAt" db:"created_at"`
	UpdatedAt     time.Time `json:"updatedAt" db:"updated_at"`
}

// BlockedContact represents a blocked user relationship
type BlockedContact struct {
	ID            string    `json:"id" db:"id"`
	UserID        string    `json:"userId" db:"user_id"`
	BlockedUserID string    `json:"blockedUserId" db:"blocked_user_id"`
	BlockedAt     time.Time `json:"blockedAt" db:"blocked_at"`
}

// ContactSyncMetadata represents metadata for contact synchronization
type ContactSyncMetadata struct {
	UserID             string    `json:"userId" db:"user_id"`
	LastSyncTime       time.Time `json:"lastSyncTime" db:"last_sync_time"`
	SyncVersion        string    `json:"syncVersion" db:"sync_version"`
	DeviceContactsHash string    `json:"deviceContactsHash" db:"device_contacts_hash"`
	SyncCount          int       `json:"syncCount" db:"sync_count"`
}

// Request/Response models for contacts
type ContactSearchRequest struct {
	PhoneNumbers []string `json:"phoneNumbers" binding:"required"`
}

type ContactSearchResponse struct {
	Users []User `json:"users"`
}

type AddContactRequest struct {
	ContactUserID string `json:"contactUserId" binding:"required"`
	ContactName   string `json:"contactName" binding:"required"`
	ContactPhone  string `json:"contactPhone" binding:"required"`
}

type BlockContactRequest struct {
	ContactUserID string `json:"contactUserId" binding:"required"`
}

type ContactSyncRequest struct {
	DeviceContacts []DeviceContact `json:"deviceContacts" binding:"required"`
	SyncVersion    string          `json:"syncVersion,omitempty"`
	ForceSync      bool            `json:"forceSync,omitempty"`
}

type DeviceContact struct {
	ID           string        `json:"id" binding:"required"`
	DisplayName  string        `json:"displayName" binding:"required"`
	PhoneNumbers []PhoneNumber `json:"phoneNumbers" binding:"required"`
}

type PhoneNumber struct {
	Number string `json:"number" binding:"required"`
	Label  string `json:"label,omitempty"`
}

type ContactSyncResponse struct {
	RegisteredContacts   []User          `json:"registeredContacts"`
	UnregisteredContacts []DeviceContact `json:"unregisteredContacts"`
	SyncTime             time.Time       `json:"syncTime"`
	SyncVersion          string          `json:"syncVersion"`
}

type ContactListResponse struct {
	Contacts []ContactResponse `json:"contacts"`
	Total    int               `json:"total"`
}

type ContactResponse struct {
	Contact
	User *User `json:"user,omitempty"` // Full user details if contact is registered
}

type BlockedContactListResponse struct {
	BlockedContacts []User `json:"blockedContacts"`
	Total           int    `json:"total"`
}

// Helper methods for Contact
func (c *Contact) IsRegisteredUser() bool {
	return c.ContactUserID != ""
}

// Validation methods
func (c *Contact) ValidateForCreation() []string {
	var errors []string

	if c.UserID == "" {
		errors = append(errors, "User ID is required")
	}

	if c.ContactName == "" {
		errors = append(errors, "Contact name is required")
	}

	if len(c.ContactName) < 1 || len(c.ContactName) > 100 {
		errors = append(errors, "Contact name must be between 1 and 100 characters")
	}

	if c.ContactPhone == "" {
		errors = append(errors, "Contact phone is required")
	}

	if len(c.ContactPhone) < 10 || len(c.ContactPhone) > 20 {
		errors = append(errors, "Contact phone must be between 10 and 20 characters")
	}

	if c.ContactUserID != "" && c.ContactUserID == c.UserID {
		errors = append(errors, "Cannot add yourself as a contact")
	}

	return errors
}

func (c *Contact) IsValidForCreation() bool {
	return len(c.ValidateForCreation()) == 0
}

// Helper functions
func StandardizePhoneNumber(phoneNumber string) string {
	// Remove all non-digit characters except +
	// This is a simplified version - you might want to use a proper phone number library
	// For now, just ensure it starts with + and contains only digits after that
	if phoneNumber == "" {
		return phoneNumber
	}

	// Simple standardization - in production, use a proper phone number library
	if !strings.HasPrefix(phoneNumber, "+") {
		// Assume US number for simplicity
		if len(phoneNumber) == 10 {
			phoneNumber = "+1" + phoneNumber
		} else if len(phoneNumber) == 11 && strings.HasPrefix(phoneNumber, "1") {
			phoneNumber = "+" + phoneNumber
		}
	}

	return phoneNumber
}

// Constants for contacts
const (
	MaxContactsPerUser       = 5000
	MaxSyncFrequencyMinutes  = 5
	MaxDeviceContactsPerSync = 2000
)
