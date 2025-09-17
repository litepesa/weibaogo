// ===============================
// internal/services/contact.go - Contact Service
// ===============================

package services

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"weibaobe/internal/models"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type ContactService struct {
	db *sqlx.DB
}

func NewContactService(db *sqlx.DB) *ContactService {
	return &ContactService{db: db}
}

// SearchUsersByPhoneNumbers searches for registered users by phone numbers
func (s *ContactService) SearchUsersByPhoneNumbers(phoneNumbers []string) ([]models.User, error) {
	if len(phoneNumbers) == 0 {
		return []models.User{}, nil
	}

	// Standardize phone numbers
	standardized := make([]string, len(phoneNumbers))
	for i, phone := range phoneNumbers {
		standardized[i] = models.StandardizePhoneNumber(phone)
	}

	query, args, err := sqlx.In(`
		SELECT uid, name, phone_number, profile_image, cover_image, bio, user_type,
		       followers_count, following_count, videos_count, likes_count,
		       is_verified, is_active, is_featured, tags,
		       created_at, updated_at, last_seen, last_post_at
		FROM users 
		WHERE phone_number IN (?) AND is_active = true`,
		standardized)

	if err != nil {
		return nil, fmt.Errorf("failed to build query: %w", err)
	}

	query = s.db.Rebind(query)

	var users []models.User
	err = s.db.Select(&users, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to search users: %w", err)
	}

	return users, nil
}

// SearchUserByPhoneNumber searches for a specific user by phone number
func (s *ContactService) SearchUserByPhoneNumber(phoneNumber string) (*models.User, error) {
	standardized := models.StandardizePhoneNumber(phoneNumber)

	var user models.User
	err := s.db.Get(&user, `
		SELECT uid, name, phone_number, profile_image, cover_image, bio, user_type,
		       followers_count, following_count, videos_count, likes_count,
		       is_verified, is_active, is_featured, tags,
		       created_at, updated_at, last_seen, last_post_at
		FROM users 
		WHERE phone_number = $1 AND is_active = true`,
		standardized)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // User not found
		}
		return nil, fmt.Errorf("failed to search user: %w", err)
	}

	return &user, nil
}

// SyncContacts synchronizes device contacts with backend
func (s *ContactService) SyncContacts(userID string, request models.ContactSyncRequest) (*models.ContactSyncResponse, error) {
	// Extract all phone numbers from device contacts
	var allPhoneNumbers []string
	phoneToContactMap := make(map[string]models.DeviceContact)

	for _, contact := range request.DeviceContacts {
		for _, phone := range contact.PhoneNumbers {
			standardized := models.StandardizePhoneNumber(phone.Number)
			allPhoneNumbers = append(allPhoneNumbers, standardized)
			phoneToContactMap[standardized] = contact
		}
	}

	// Find registered users
	registeredUsers, err := s.SearchUsersByPhoneNumbers(allPhoneNumbers)
	if err != nil {
		return nil, fmt.Errorf("failed to search registered users: %w", err)
	}

	// Filter out current user
	filteredUsers := make([]models.User, 0, len(registeredUsers))
	for _, user := range registeredUsers {
		if user.UID != userID {
			filteredUsers = append(filteredUsers, user)
		}
	}

	// Create unregistered contacts list
	registeredPhones := make(map[string]bool)
	for _, user := range filteredUsers {
		registeredPhones[user.PhoneNumber] = true
	}

	processedContactIDs := make(map[string]bool)
	var unregisteredContacts []models.DeviceContact

	for phone, contact := range phoneToContactMap {
		if !registeredPhones[phone] && !processedContactIDs[contact.ID] {
			unregisteredContacts = append(unregisteredContacts, contact)
			processedContactIDs[contact.ID] = true
		}
	}

	// Update sync metadata
	syncVersion := request.SyncVersion
	if syncVersion == "" {
		syncVersion = fmt.Sprintf("%d", time.Now().Unix())
	}

	_, err = s.db.Exec(`
		INSERT INTO contact_sync_metadata (user_id, last_sync_time, sync_version, sync_count)
		VALUES ($1, $2, $3, 1)
		ON CONFLICT (user_id) DO UPDATE SET
			last_sync_time = $2,
			sync_version = $3,
			sync_count = contact_sync_metadata.sync_count + 1`,
		userID, time.Now(), syncVersion)

	if err != nil {
		return nil, fmt.Errorf("failed to update sync metadata: %w", err)
	}

	return &models.ContactSyncResponse{
		RegisteredContacts:   filteredUsers,
		UnregisteredContacts: unregisteredContacts,
		SyncTime:             time.Now(),
		SyncVersion:          syncVersion,
	}, nil
}

// GetUserContacts gets user's contacts
func (s *ContactService) GetUserContacts(userID string) ([]models.Contact, error) {
	var contacts []models.Contact
	err := s.db.Select(&contacts, `
		SELECT id, user_id, contact_user_id, contact_name, contact_phone,
		       is_blocked, created_at, updated_at
		FROM contacts 
		WHERE user_id = $1 AND is_blocked = false
		ORDER BY contact_name ASC`,
		userID)

	if err != nil {
		return nil, fmt.Errorf("failed to get contacts: %w", err)
	}

	return contacts, nil
}

// AddContact adds a new contact
func (s *ContactService) AddContact(contact *models.Contact) error {
	query := `
		INSERT INTO contacts (id, user_id, contact_user_id, contact_name, 
		                     contact_phone, is_blocked, created_at, updated_at)
		VALUES (:id, :user_id, :contact_user_id, :contact_name,
		        :contact_phone, :is_blocked, :created_at, :updated_at)
		ON CONFLICT (user_id, contact_user_id) DO UPDATE SET
			contact_name = EXCLUDED.contact_name,
			updated_at = EXCLUDED.updated_at`

	_, err := s.db.NamedExec(query, contact)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate") {
			return fmt.Errorf("contact already exists")
		}
		return fmt.Errorf("failed to add contact: %w", err)
	}

	return nil
}

// RemoveContact removes a contact
func (s *ContactService) RemoveContact(userID, contactID string) error {
	result, err := s.db.Exec(`
		DELETE FROM contacts 
		WHERE id = $1 AND user_id = $2`,
		contactID, userID)

	if err != nil {
		return fmt.Errorf("failed to remove contact: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check remove result: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("contact not found")
	}

	return nil
}

// BlockContact blocks a contact
func (s *ContactService) BlockContact(userID, contactUserID string) error {
	tx, err := s.db.Beginx()
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	// Check if already blocked
	var count int
	err = tx.QueryRow(`
		SELECT COUNT(*) FROM blocked_contacts 
		WHERE user_id = $1 AND blocked_user_id = $2`,
		userID, contactUserID).Scan(&count)

	if err != nil {
		return fmt.Errorf("failed to check existing block: %w", err)
	}

	if count > 0 {
		return fmt.Errorf("user already blocked")
	}

	// Add to blocked contacts
	_, err = tx.Exec(`
		INSERT INTO blocked_contacts (id, user_id, blocked_user_id, blocked_at)
		VALUES ($1, $2, $3, $4)`,
		uuid.New().String(), userID, contactUserID, time.Now())

	if err != nil {
		return fmt.Errorf("failed to block contact: %w", err)
	}

	// Update contact record if exists
	_, err = tx.Exec(`
		UPDATE contacts 
		SET is_blocked = true, updated_at = $1
		WHERE user_id = $2 AND contact_user_id = $3`,
		time.Now(), userID, contactUserID)

	// Don't fail if contact record doesn't exist

	return tx.Commit()
}

// UnblockContact unblocks a contact
func (s *ContactService) UnblockContact(userID, contactUserID string) error {
	tx, err := s.db.Beginx()
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	// Remove from blocked contacts
	result, err := tx.Exec(`
		DELETE FROM blocked_contacts 
		WHERE user_id = $1 AND blocked_user_id = $2`,
		userID, contactUserID)

	if err != nil {
		return fmt.Errorf("failed to unblock contact: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check unblock result: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("user not blocked")
	}

	// Update contact record if exists
	_, err = tx.Exec(`
		UPDATE contacts 
		SET is_blocked = false, updated_at = $1
		WHERE user_id = $2 AND contact_user_id = $3`,
		time.Now(), userID, contactUserID)

	// Don't fail if contact record doesn't exist

	return tx.Commit()
}

// GetBlockedContacts gets user's blocked contacts
func (s *ContactService) GetBlockedContacts(userID string) ([]models.User, error) {
	var users []models.User
	err := s.db.Select(&users, `
		SELECT u.uid, u.name, u.phone_number, u.profile_image, u.cover_image, u.bio, u.user_type,
		       u.followers_count, u.following_count, u.videos_count, u.likes_count,
		       u.is_verified, u.is_active, u.is_featured, u.tags,
		       u.created_at, u.updated_at, u.last_seen, u.last_post_at
		FROM users u
		INNER JOIN blocked_contacts bc ON u.uid = bc.blocked_user_id
		WHERE bc.user_id = $1
		ORDER BY bc.blocked_at DESC`,
		userID)

	if err != nil {
		return nil, fmt.Errorf("failed to get blocked contacts: %w", err)
	}

	return users, nil
}
