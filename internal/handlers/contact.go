// ===============================
// internal/handlers/contact.go - Contact Handler
// ===============================

package handlers

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"weibaobe/internal/models"
	"weibaobe/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type ContactHandler struct {
	contactService *services.ContactService
	userService    *services.UserService
}

func NewContactHandler(contactService *services.ContactService, userService *services.UserService) *ContactHandler {
	return &ContactHandler{
		contactService: contactService,
		userService:    userService,
	}
}

// SearchContacts searches for registered users by phone numbers
func (h *ContactHandler) SearchContacts(c *gin.Context) {
	var request models.ContactSearchRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if len(request.PhoneNumbers) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Phone numbers required"})
		return
	}

	if len(request.PhoneNumbers) > models.MaxDeviceContactsPerSync {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Too many phone numbers (max %d)", models.MaxDeviceContactsPerSync)})
		return
	}

	users, err := h.contactService.SearchUsersByPhoneNumbers(request.PhoneNumbers)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to search contacts"})
		return
	}

	c.JSON(http.StatusOK, models.ContactSearchResponse{Users: users})
}

// SyncContacts synchronizes device contacts with backend
func (h *ContactHandler) SyncContacts(c *gin.Context) {
	userID := c.GetString("userID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	var request models.ContactSyncRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if len(request.DeviceContacts) > models.MaxDeviceContactsPerSync {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Too many contacts (max %d)", models.MaxDeviceContactsPerSync)})
		return
	}

	result, err := h.contactService.SyncContacts(userID, request)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to sync contacts"})
		return
	}

	c.JSON(http.StatusOK, result)
}

// GetContacts gets user's contacts
func (h *ContactHandler) GetContacts(c *gin.Context) {
	userID := c.GetString("userID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	contacts, err := h.contactService.GetUserContacts(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get contacts"})
		return
	}

	// Enhance contacts with user information
	contactResponses := make([]models.ContactResponse, len(contacts))
	for i, contact := range contacts {
		contactResponses[i] = models.ContactResponse{Contact: contact}

		if contact.ContactUserID != "" {
			if user, err := h.userService.GetUser(c, contact.ContactUserID); err == nil {
				contactResponses[i].User = user
			}
		}
	}

	c.JSON(http.StatusOK, models.ContactListResponse{
		Contacts: contactResponses,
		Total:    len(contactResponses),
	})
}

// AddContact adds a new contact
func (h *ContactHandler) AddContact(c *gin.Context) {
	userID := c.GetString("userID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	var request models.AddContactRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if request.ContactUserID == userID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot add yourself as contact"})
		return
	}

	// Check if target user exists
	targetUser, err := h.userService.GetUser(c, request.ContactUserID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	contact := &models.Contact{
		ID:            uuid.New().String(),
		UserID:        userID,
		ContactUserID: request.ContactUserID,
		ContactName:   request.ContactName,
		ContactPhone:  targetUser.PhoneNumber,
		IsBlocked:     false,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	if !contact.IsValidForCreation() {
		errors := contact.ValidateForCreation()
		c.JSON(http.StatusBadRequest, gin.H{"error": "Validation failed", "details": errors})
		return
	}

	err = h.contactService.AddContact(contact)
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			c.JSON(http.StatusConflict, gin.H{"error": "Contact already exists"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add contact"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Contact added successfully",
		"contact": contact,
	})
}

// RemoveContact removes a contact
func (h *ContactHandler) RemoveContact(c *gin.Context) {
	userID := c.GetString("userID")
	contactID := c.Param("contactId")

	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	if contactID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Contact ID required"})
		return
	}

	err := h.contactService.RemoveContact(userID, contactID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, gin.H{"error": "Contact not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove contact"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Contact removed successfully"})
}

// BlockContact blocks a contact
func (h *ContactHandler) BlockContact(c *gin.Context) {
	userID := c.GetString("userID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	var request models.BlockContactRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if request.ContactUserID == userID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot block yourself"})
		return
	}

	err := h.contactService.BlockContact(userID, request.ContactUserID)
	if err != nil {
		if strings.Contains(err.Error(), "already blocked") {
			c.JSON(http.StatusConflict, gin.H{"error": "User already blocked"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to block contact"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Contact blocked successfully"})
}

// UnblockContact unblocks a contact
func (h *ContactHandler) UnblockContact(c *gin.Context) {
	userID := c.GetString("userID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	var request models.BlockContactRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := h.contactService.UnblockContact(userID, request.ContactUserID)
	if err != nil {
		if strings.Contains(err.Error(), "not blocked") {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not blocked"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to unblock contact"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Contact unblocked successfully"})
}

// GetBlockedContacts gets user's blocked contacts
func (h *ContactHandler) GetBlockedContacts(c *gin.Context) {
	userID := c.GetString("userID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	blockedUsers, err := h.contactService.GetBlockedContacts(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get blocked contacts"})
		return
	}

	c.JSON(http.StatusOK, models.BlockedContactListResponse{
		BlockedContacts: blockedUsers,
		Total:           len(blockedUsers),
	})
}

// SearchUserByPhone searches for a specific user by phone number
func (h *ContactHandler) SearchUserByPhone(c *gin.Context) {
	phoneNumber := c.Query("phoneNumber")
	if phoneNumber == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Phone number required"})
		return
	}

	user, err := h.contactService.SearchUserByPhoneNumber(models.StandardizePhoneNumber(phoneNumber))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to search user"})
		return
	}

	if user == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"user": user})
}
