// ===============================
// internal/services/wallet.go
// ===============================

package services

import (
	"context"
	"time"

	"weibaobe/internal/models"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type WalletService struct {
	db *sqlx.DB
}

func NewWalletService(db *sqlx.DB) *WalletService {
	return &WalletService{db: db}
}

func (s *WalletService) GetWallet(ctx context.Context, userID string) (*models.Wallet, error) {
	var wallet models.Wallet
	query := `SELECT * FROM wallets WHERE user_id = $1`
	err := s.db.GetContext(ctx, &wallet, query, userID)

	if err != nil {
		// Create wallet if it doesn't exist
		wallet, err = s.createWallet(ctx, userID)
		if err != nil {
			return nil, err
		}
	}

	return &wallet, nil
}

func (s *WalletService) createWallet(ctx context.Context, userID string) (models.Wallet, error) {
	// Get user info
	var user models.User
	err := s.db.GetContext(ctx, &user, "SELECT name, phone_number FROM users WHERE uid = $1", userID)
	if err != nil {
		return models.Wallet{}, err
	}

	wallet := models.Wallet{
		WalletID:        userID,
		UserID:          userID,
		UserPhoneNumber: user.PhoneNumber,
		UserName:        user.Name,
		CoinsBalance:    0,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	query := `
		INSERT INTO wallets (wallet_id, user_id, user_phone_number, user_name, coins_balance, created_at, updated_at)
		VALUES (:wallet_id, :user_id, :user_phone_number, :user_name, :coins_balance, :created_at, :updated_at)`

	_, err = s.db.NamedExecContext(ctx, query, wallet)
	return wallet, err
}

func (s *WalletService) GetTransactions(ctx context.Context, userID string, limit int) ([]models.WalletTransaction, error) {
	query := `
		SELECT * FROM wallet_transactions 
		WHERE user_id = $1 
		ORDER BY created_at DESC 
		LIMIT $2`

	var transactions []models.WalletTransaction
	err := s.db.SelectContext(ctx, &transactions, query, userID, limit)
	return transactions, err
}

func (s *WalletService) CreatePurchaseRequest(ctx context.Context, request *models.CoinPurchaseRequest) (string, error) {
	request.ID = uuid.New().String()
	request.RequestedAt = time.Now()

	query := `
		INSERT INTO coin_purchase_requests (
			id, user_id, package_id, coin_amount, paid_amount,
			payment_reference, payment_method, status, requested_at
		) VALUES (
			:id, :user_id, :package_id, :coin_amount, :paid_amount,
			:payment_reference, :payment_method, :status, :requested_at
		)`

	_, err := s.db.NamedExecContext(ctx, query, request)
	return request.ID, err
}

func (s *WalletService) AddCoins(ctx context.Context, userID string, coinAmount int, description, adminNote string) (int, error) {
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	// Get current balance
	var currentBalance int
	err = tx.QueryRowContext(ctx, "SELECT coins_balance FROM users WHERE uid = $1", userID).Scan(&currentBalance)
	if err != nil {
		return 0, err
	}

	newBalance := currentBalance + coinAmount

	// Update user balance
	_, err = tx.ExecContext(ctx,
		"UPDATE users SET coins_balance = $1, updated_at = $2 WHERE uid = $3",
		newBalance, time.Now(), userID)
	if err != nil {
		return 0, err
	}

	// Update wallet
	_, err = tx.ExecContext(ctx,
		"UPDATE wallets SET coins_balance = $1, updated_at = $2 WHERE user_id = $3",
		newBalance, time.Now(), userID)
	if err != nil {
		return 0, err
	}

	// Create transaction record
	transactionID := uuid.New().String()
	if description == "" {
		description = "Admin added coins"
	}

	transaction := models.WalletTransaction{
		TransactionID: transactionID,
		WalletID:      userID,
		UserID:        userID,
		Type:          "admin_credit",
		CoinAmount:    coinAmount,
		BalanceBefore: currentBalance,
		BalanceAfter:  newBalance,
		Description:   description,
		AdminNote:     &adminNote,
		CreatedAt:     time.Now(),
	}

	query := `
		INSERT INTO wallet_transactions (
			transaction_id, wallet_id, user_id, type, coin_amount,
			balance_before, balance_after, description, admin_note, created_at
		) VALUES (
			:transaction_id, :wallet_id, :user_id, :type, :coin_amount,
			:balance_before, :balance_after, :description, :admin_note, :created_at
		)`

	_, err = tx.NamedExecContext(ctx, query, transaction)
	if err != nil {
		return 0, err
	}

	if err = tx.Commit(); err != nil {
		return 0, err
	}

	return newBalance, nil
}

func (s *WalletService) GetPendingPurchases(ctx context.Context, limit int) ([]models.CoinPurchaseRequest, error) {
	query := `
		SELECT * FROM coin_purchase_requests 
		WHERE status = 'pending_admin_verification' 
		ORDER BY requested_at DESC 
		LIMIT $1`

	var requests []models.CoinPurchaseRequest
	err := s.db.SelectContext(ctx, &requests, query, limit)
	return requests, err
}

func (s *WalletService) ProcessPurchaseRequest(ctx context.Context, requestID, status, adminNote string) error {
	if status == "approved" {
		return s.approvePurchaseRequest(ctx, requestID, adminNote)
	} else {
		return s.rejectPurchaseRequest(ctx, requestID, adminNote)
	}
}

func (s *WalletService) approvePurchaseRequest(ctx context.Context, requestID, adminNote string) error {
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Get purchase request
	var request models.CoinPurchaseRequest
	err = tx.GetContext(ctx, &request, "SELECT * FROM coin_purchase_requests WHERE id = $1", requestID)
	if err != nil {
		return err
	}

	// Add coins to user account
	_, err = s.AddCoins(ctx, request.UserID, request.CoinAmount,
		"Coin purchase approved", adminNote)
	if err != nil {
		return err
	}

	// Update request status
	now := time.Now()
	_, err = tx.ExecContext(ctx, `
		UPDATE coin_purchase_requests 
		SET status = 'approved', processed_at = $1, admin_note = $2 
		WHERE id = $3`, now, adminNote, requestID)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (s *WalletService) rejectPurchaseRequest(ctx context.Context, requestID, adminNote string) error {
	now := time.Now()
	_, err := s.db.ExecContext(ctx, `
		UPDATE coin_purchase_requests 
		SET status = 'rejected', processed_at = $1, admin_note = $2 
		WHERE id = $3`, now, adminNote, requestID)

	return err
}
