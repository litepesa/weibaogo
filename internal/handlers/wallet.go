// ===============================
// internal/handlers/wallet.go
// ===============================

package handlers

import (
	"net/http"
	"strconv"

	"weibaobe/internal/models"
	"weibaobe/internal/services"

	"github.com/gin-gonic/gin"
)

type WalletHandler struct {
	service *services.WalletService
}

func NewWalletHandler(service *services.WalletService) *WalletHandler {
	return &WalletHandler{service: service}
}

func (h *WalletHandler) GetWallet(c *gin.Context) {
	userID := c.Param("userId")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "User ID required"})
		return
	}

	wallet, err := h.service.GetWallet(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch wallet"})
		return
	}

	c.JSON(http.StatusOK, wallet)
}

func (h *WalletHandler) GetTransactions(c *gin.Context) {
	userID := c.Param("userId")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "User ID required"})
		return
	}

	limit := 50
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 200 {
			limit = parsed
		}
	}

	transactions, err := h.service.GetTransactions(c.Request.Context(), userID, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch transactions"})
		return
	}

	c.JSON(http.StatusOK, transactions)
}

func (h *WalletHandler) CreatePurchaseRequest(c *gin.Context) {
	userID := c.Param("userId")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "User ID required"})
		return
	}

	var request struct {
		PackageID        string `json:"packageId" binding:"required"`
		PaymentReference string `json:"paymentReference" binding:"required"`
		PaymentMethod    string `json:"paymentMethod" binding:"required"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate package exists
	pkg, exists := models.CoinPackages[request.PackageID]
	if !exists {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid package ID"})
		return
	}

	purchaseRequest := &models.CoinPurchaseRequest{
		UserID:           userID,
		PackageID:        request.PackageID,
		CoinAmount:       pkg.Coins,
		PaidAmount:       pkg.Price,
		PaymentReference: request.PaymentReference,
		PaymentMethod:    request.PaymentMethod,
		Status:           "pending_admin_verification",
	}

	requestID, err := h.service.CreatePurchaseRequest(c.Request.Context(), purchaseRequest)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create purchase request"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"requestId": requestID,
		"message":   "Purchase request submitted for admin verification",
		"status":    "pending",
	})
}

func (h *WalletHandler) AddCoins(c *gin.Context) {
	userID := c.Param("userId")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "User ID required"})
		return
	}

	var request struct {
		CoinAmount  int    `json:"coinAmount" binding:"required"`
		Description string `json:"description"`
		AdminNote   string `json:"adminNote"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if request.CoinAmount <= 0 || request.CoinAmount > 10000 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid coin amount"})
		return
	}

	newBalance, err := h.service.AddCoins(c.Request.Context(), userID, request.CoinAmount, request.Description, request.AdminNote)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add coins"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    "Coins added successfully",
		"newBalance": newBalance,
	})
}

func (h *WalletHandler) GetPendingPurchases(c *gin.Context) {
	limit := 50
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 200 {
			limit = parsed
		}
	}

	requests, err := h.service.GetPendingPurchases(c.Request.Context(), limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch pending purchases"})
		return
	}

	c.JSON(http.StatusOK, requests)
}

func (h *WalletHandler) ApprovePurchase(c *gin.Context) {
	requestID := c.Param("requestId")
	if requestID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Request ID required"})
		return
	}

	var request struct {
		AdminNote string `json:"adminNote"`
	}

	c.ShouldBindJSON(&request) // Optional admin note

	err := h.service.ProcessPurchaseRequest(c.Request.Context(), requestID, "approved", request.AdminNote)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to approve purchase"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Purchase request approved and coins added"})
}

func (h *WalletHandler) RejectPurchase(c *gin.Context) {
	requestID := c.Param("requestId")
	if requestID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Request ID required"})
		return
	}

	var request struct {
		AdminNote string `json:"adminNote" binding:"required"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := h.service.ProcessPurchaseRequest(c.Request.Context(), requestID, "rejected", request.AdminNote)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to reject purchase"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Purchase request rejected"})
}
