// ===============================
// internal/services/gift.go - Virtual Gift Service Implementation
// ===============================

package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"weibaobe/internal/models"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type GiftService struct {
	db            *sqlx.DB
	walletService *WalletService
}

func NewGiftService(db *sqlx.DB, walletService *WalletService) *GiftService {
	return &GiftService{
		db:            db,
		walletService: walletService,
	}
}

// ===============================
// Send Gift Transaction
// ===============================

// SendGift processes a gift transaction from sender to recipient
func (s *GiftService) SendGift(
	ctx context.Context,
	senderID string,
	request models.SendGiftRequest,
	giftPrice int,
	giftName string,
	giftEmoji string,
	giftRarity models.GiftRarity,
) (*models.SendGiftResponse, error) {
	// Start a database transaction
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// 1. Validate sender and recipient exist and are different
	if senderID == request.RecipientID {
		return nil, fmt.Errorf("cannot send gift to yourself")
	}

	// 2. Get sender information
	var sender struct {
		UID         string `db:"uid"`
		Name        string `db:"name"`
		PhoneNumber string `db:"phone_number"`
	}
	err = tx.GetContext(ctx, &sender,
		"SELECT uid, name, phone_number FROM users WHERE uid = $1 AND is_active = true",
		senderID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("sender not found or inactive")
		}
		return nil, fmt.Errorf("failed to get sender: %w", err)
	}

	// 3. Get recipient information
	var recipient struct {
		UID         string `db:"uid"`
		Name        string `db:"name"`
		PhoneNumber string `db:"phone_number"`
	}
	err = tx.GetContext(ctx, &recipient,
		"SELECT uid, name, phone_number FROM users WHERE uid = $1 AND is_active = true",
		request.RecipientID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("recipient not found or inactive")
		}
		return nil, fmt.Errorf("failed to get recipient: %w", err)
	}

	// 4. Calculate commission
	recipientAmount, platformCommission := models.CalculateCommission(giftPrice, models.DefaultCommissionRate)

	// 5. Get sender's wallet
	var senderWallet struct {
		WalletID     string `db:"wallet_id"`
		CoinsBalance int    `db:"coins_balance"`
	}
	err = tx.GetContext(ctx, &senderWallet,
		"SELECT wallet_id, coins_balance FROM wallets WHERE user_id = $1",
		senderID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("sender wallet not found")
		}
		return nil, fmt.Errorf("failed to get sender wallet: %w", err)
	}

	// 6. Check if sender has sufficient balance
	if senderWallet.CoinsBalance < giftPrice {
		return nil, fmt.Errorf("insufficient balance: have %d coins, need %d coins",
			senderWallet.CoinsBalance, giftPrice)
	}

	// 7. Get recipient's wallet
	var recipientWallet struct {
		WalletID     string `db:"wallet_id"`
		CoinsBalance int    `db:"coins_balance"`
	}
	err = tx.GetContext(ctx, &recipientWallet,
		"SELECT wallet_id, coins_balance FROM wallets WHERE user_id = $1",
		request.RecipientID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("recipient wallet not found")
		}
		return nil, fmt.Errorf("failed to get recipient wallet: %w", err)
	}

	// 8. Deduct coins from sender's wallet
	senderBalanceBefore := senderWallet.CoinsBalance
	senderBalanceAfter := senderBalanceBefore - giftPrice

	_, err = tx.ExecContext(ctx, `
		UPDATE wallets 
		SET coins_balance = $1, updated_at = CURRENT_TIMESTAMP 
		WHERE user_id = $2
	`, senderBalanceAfter, senderID)
	if err != nil {
		return nil, fmt.Errorf("failed to update sender wallet: %w", err)
	}

	// 9. Add coins to recipient's wallet (after commission)
	recipientBalanceBefore := recipientWallet.CoinsBalance
	recipientBalanceAfter := recipientBalanceBefore + recipientAmount

	_, err = tx.ExecContext(ctx, `
		UPDATE wallets 
		SET coins_balance = $1, updated_at = CURRENT_TIMESTAMP 
		WHERE user_id = $2
	`, recipientBalanceAfter, request.RecipientID)
	if err != nil {
		return nil, fmt.Errorf("failed to update recipient wallet: %w", err)
	}

	// 10. Create gift transaction record
	transactionID := uuid.New().String()
	senderTxID := uuid.New().String()
	recipientTxID := uuid.New().String()

	metadata := models.GiftMetadataMap{
		"gift_rarity": string(giftRarity),
	}
	if request.Message != nil {
		metadata["message"] = *request.Message
	}
	if request.Context != nil {
		metadata["context"] = *request.Context
	}

	var createdAt time.Time
	err = tx.QueryRowContext(ctx, `
		INSERT INTO gift_transactions (
			id, sender_id, sender_name, sender_phone,
			recipient_id, recipient_name, recipient_phone,
			gift_id, gift_name, gift_emoji, gift_rarity,
			gift_price, sender_paid, recipient_received, platform_commission,
			sender_balance_before, sender_balance_after,
			recipient_balance_before, recipient_balance_after,
			status, message, metadata
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22)
		RETURNING created_at
	`, transactionID, sender.UID, sender.Name, sender.PhoneNumber,
		recipient.UID, recipient.Name, recipient.PhoneNumber,
		request.GiftID, giftName, giftEmoji, string(giftRarity),
		giftPrice, giftPrice, recipientAmount, platformCommission,
		senderBalanceBefore, senderBalanceAfter,
		recipientBalanceBefore, recipientBalanceAfter,
		"completed", request.Message, metadata,
	).Scan(&createdAt)
	if err != nil {
		return nil, fmt.Errorf("failed to create gift transaction: %w", err)
	}

	// 11. Create platform commission record
	commissionID := uuid.New().String()
	commissionMetadata := models.GiftMetadataMap{
		"gift_rarity":    string(giftRarity),
		"transaction_id": transactionID,
	}
	commissionMetadataJSON, _ := json.Marshal(commissionMetadata)

	_, err = tx.ExecContext(ctx, `
		INSERT INTO platform_commissions (
			id, gift_transaction_id, commission_amount, original_gift_price,
			commission_rate, sender_id, recipient_id, gift_name, metadata
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, commissionID, transactionID, platformCommission, giftPrice,
		models.DefaultCommissionRate, sender.UID, recipient.UID, giftName, commissionMetadataJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to create commission record: %w", err)
	}

	// 12. Create wallet transaction for sender (debit)
	walletMetadata := models.GiftMetadataMap{
		"gift_name":      giftName,
		"gift_emoji":     giftEmoji,
		"recipient_name": recipient.Name,
	}
	walletMetadataJSON, _ := json.Marshal(walletMetadata)

	_, err = tx.ExecContext(ctx, `
		INSERT INTO wallet_transactions (
			transaction_id, wallet_id, user_id, user_phone_number, user_name,
			type, coin_amount, balance_before, balance_after,
			description, reference_id, gift_id, recipient_id, metadata
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`, senderTxID, senderWallet.WalletID, sender.UID, sender.PhoneNumber, sender.Name,
		"gift_sent", -giftPrice, senderBalanceBefore, senderBalanceAfter,
		fmt.Sprintf("Sent %s to %s", giftName, recipient.Name),
		transactionID, request.GiftID, recipient.UID, walletMetadataJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to create sender wallet transaction: %w", err)
	}

	// 13. Create wallet transaction for recipient (credit)
	recipientWalletMetadata := models.GiftMetadataMap{
		"gift_name":   giftName,
		"gift_emoji":  giftEmoji,
		"sender_name": sender.Name,
		"commission":  platformCommission,
	}
	recipientWalletMetadataJSON, _ := json.Marshal(recipientWalletMetadata)

	_, err = tx.ExecContext(ctx, `
		INSERT INTO wallet_transactions (
			transaction_id, wallet_id, user_id, user_phone_number, user_name,
			type, coin_amount, balance_before, balance_after,
			description, reference_id, gift_id, sender_id, metadata
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`, recipientTxID, recipientWallet.WalletID, recipient.UID, recipient.PhoneNumber, recipient.Name,
		"gift_received", recipientAmount, recipientBalanceBefore, recipientBalanceAfter,
		fmt.Sprintf("Received %s from %s", giftName, sender.Name),
		transactionID, request.GiftID, sender.UID, recipientWalletMetadataJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to create recipient wallet transaction: %w", err)
	}

	// 14. Update user gift statistics for sender
	_, err = tx.ExecContext(ctx, `
		UPDATE users 
		SET 
			gifts_sent_count = gifts_sent_count + 1,
			total_coins_spent_on_gifts = total_coins_spent_on_gifts + $1
		WHERE uid = $2
	`, giftPrice, sender.UID)
	if err != nil {
		return nil, fmt.Errorf("failed to update sender statistics: %w", err)
	}

	// 15. Update user gift statistics for recipient
	_, err = tx.ExecContext(ctx, `
		UPDATE users 
		SET 
			gifts_received_count = gifts_received_count + 1,
			total_coins_earned_from_gifts = total_coins_earned_from_gifts + $1
		WHERE uid = $2
	`, recipientAmount, recipient.UID)
	if err != nil {
		return nil, fmt.Errorf("failed to update recipient statistics: %w", err)
	}

	// 16. Commit the transaction
	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	log.Printf("✅ Gift sent: %s -> %s | %s (%d coins) | Recipient: %d, Commission: %d",
		sender.Name, recipient.Name, giftName, giftPrice, recipientAmount, platformCommission)

	// 17. Build the gift transaction object for response
	giftTransaction := &models.GiftTransaction{
		ID:                     transactionID,
		SenderID:               sender.UID,
		SenderName:             sender.Name,
		SenderPhone:            sender.PhoneNumber,
		RecipientID:            recipient.UID,
		RecipientName:          recipient.Name,
		RecipientPhone:         recipient.PhoneNumber,
		GiftID:                 request.GiftID,
		GiftName:               giftName,
		GiftEmoji:              giftEmoji,
		GiftRarity:             giftRarity,
		GiftPrice:              giftPrice,
		RecipientAmount:        recipientAmount,
		PlatformCommission:     platformCommission,
		CommissionRate:         models.DefaultCommissionRate,
		SenderTransactionID:    &senderTxID,
		RecipientTransactionID: &recipientTxID,
		Message:                request.Message,
		Context:                request.Context,
		Metadata:               metadata,
		CreatedAt:              createdAt,
	}

	// 18. Build response
	response := &models.SendGiftResponse{
		Success:             true,
		GiftTransaction:     giftTransaction,
		SenderNewBalance:    senderBalanceAfter,
		RecipientNewBalance: recipientBalanceAfter,
		PlatformCommission:  platformCommission,
		Message:             fmt.Sprintf("Successfully sent %s to %s", giftName, recipient.Name),
	}

	return response, nil
}

// ===============================
// Gift History & Statistics
// ===============================

// GetUserGiftHistory retrieves gift transaction history for a user
func (s *GiftService) GetUserGiftHistory(ctx context.Context, userID string, limit, offset int) ([]models.GiftHistoryItem, error) {
	var transactions []models.GiftTransaction

	query := `
		SELECT 
			id, sender_id, sender_name, sender_phone,
			recipient_id, recipient_name, recipient_phone,
			gift_id, gift_name, gift_emoji, gift_rarity,
			gift_price, recipient_received as recipient_amount, 
			platform_commission, commission_rate,
			sender_transaction_id, recipient_transaction_id,
			message, context, metadata, created_at
		FROM gift_transactions
		WHERE sender_id = $1 OR recipient_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	err := s.db.SelectContext(ctx, &transactions, query, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get gift history: %w", err)
	}

	// Convert to history items with type
	history := make([]models.GiftHistoryItem, len(transactions))
	for i, tx := range transactions {
		historyType := "received"
		if tx.SenderID == userID {
			historyType = "sent"
		}

		history[i] = models.GiftHistoryItem{
			GiftTransaction: tx,
			Type:            historyType,
		}
	}

	return history, nil
}

// GetUserGiftStats retrieves gift statistics for a user
func (s *GiftService) GetUserGiftStats(ctx context.Context, userID string) (*models.GiftStats, error) {
	stats := &models.GiftStats{
		UserID: userID,
	}

	// Get basic stats from users table
	err := s.db.GetContext(ctx, stats, `
		SELECT 
			uid as user_id,
			name as user_name,
			COALESCE(gifts_sent_count, 0) as gifts_sent,
			COALESCE(gifts_received_count, 0) as gifts_received,
			COALESCE(total_coins_spent_on_gifts, 0) as total_coins_spent_on_gifts,
			COALESCE(total_coins_earned_from_gifts, 0) as total_coins_earned_from_gifts
		FROM users
		WHERE uid = $1
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user gift stats: %w", err)
	}

	// Get most sent gift
	var mostSentGift sql.NullString
	err = s.db.GetContext(ctx, &mostSentGift, `
		SELECT gift_name 
		FROM gift_transactions 
		WHERE sender_id = $1 
		GROUP BY gift_name 
		ORDER BY COUNT(*) DESC 
		LIMIT 1
	`, userID)
	if err == nil && mostSentGift.Valid {
		stats.MostSentGift = &mostSentGift.String
	}

	// Get most received gift
	var mostReceivedGift sql.NullString
	err = s.db.GetContext(ctx, &mostReceivedGift, `
		SELECT gift_name 
		FROM gift_transactions 
		WHERE recipient_id = $1 
		GROUP BY gift_name 
		ORDER BY COUNT(*) DESC 
		LIMIT 1
	`, userID)
	if err == nil && mostReceivedGift.Valid {
		stats.MostReceivedGift = &mostReceivedGift.String
	}

	// Get last gift sent timestamp
	var lastSent sql.NullTime
	err = s.db.GetContext(ctx, &lastSent, `
		SELECT created_at
		FROM gift_transactions 
		WHERE sender_id = $1 
		ORDER BY created_at DESC 
		LIMIT 1
	`, userID)
	if err == nil && lastSent.Valid {
		stats.LastGiftSentAt = &lastSent.Time
	}

	// Get last gift received timestamp
	var lastReceived sql.NullTime
	err = s.db.GetContext(ctx, &lastReceived, `
		SELECT created_at
		FROM gift_transactions 
		WHERE recipient_id = $1 
		ORDER BY created_at DESC 
		LIMIT 1
	`, userID)
	if err == nil && lastReceived.Valid {
		stats.LastGiftReceivedAt = &lastReceived.Time
	}

	return stats, nil
}

// GetGiftTransaction retrieves a specific gift transaction
func (s *GiftService) GetGiftTransaction(ctx context.Context, transactionID string) (*models.GiftTransaction, error) {
	var transaction models.GiftTransaction

	query := `
		SELECT 
			id, sender_id, sender_name, sender_phone,
			recipient_id, recipient_name, recipient_phone,
			gift_id, gift_name, gift_emoji, gift_rarity,
			gift_price, recipient_received as recipient_amount, 
			platform_commission, commission_rate,
			sender_transaction_id, recipient_transaction_id,
			message, context, metadata, created_at
		FROM gift_transactions
		WHERE id = $1
	`

	err := s.db.GetContext(ctx, &transaction, query, transactionID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("transaction not found")
		}
		return nil, fmt.Errorf("failed to get transaction: %w", err)
	}

	return &transaction, nil
}

// ===============================
// Leaderboards
// ===============================

// GetTopGiftSenders retrieves the leaderboard of top gift senders
func (s *GiftService) GetTopGiftSenders(ctx context.Context, limit int) ([]models.TopGiftSender, error) {
	var leaderboard []models.TopGiftSender

	query := `
		SELECT 
			sender_id as user_id,
			sender_name as user_name,
			COUNT(*) as gifts_sent,
			SUM(sender_paid) as total_spent,
			(
				SELECT gift_name 
				FROM gift_transactions gt2 
				WHERE gt2.sender_id = gt.sender_id 
				GROUP BY gift_name 
				ORDER BY COUNT(*) DESC 
				LIMIT 1
			) as most_sent_gift
		FROM gift_transactions gt
		WHERE created_at >= NOW() - INTERVAL '30 days'
		GROUP BY sender_id, sender_name
		ORDER BY total_spent DESC
		LIMIT $1
	`

	err := s.db.SelectContext(ctx, &leaderboard, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get top gift senders: %w", err)
	}

	return leaderboard, nil
}

// GetTopGiftReceivers retrieves the leaderboard of top gift receivers
func (s *GiftService) GetTopGiftReceivers(ctx context.Context, limit int) ([]models.TopGiftReceiver, error) {
	var leaderboard []models.TopGiftReceiver

	query := `
		SELECT 
			recipient_id as user_id,
			recipient_name as user_name,
			COUNT(*) as gifts_received,
			SUM(recipient_received) as total_earned,
			(
				SELECT gift_name 
				FROM gift_transactions gt2 
				WHERE gt2.recipient_id = gt.recipient_id 
				GROUP BY gift_name 
				ORDER BY COUNT(*) DESC 
				LIMIT 1
			) as most_received_gift
		FROM gift_transactions gt
		WHERE created_at >= NOW() - INTERVAL '30 days'
		GROUP BY recipient_id, recipient_name
		ORDER BY total_earned DESC
		LIMIT $1
	`

	err := s.db.SelectContext(ctx, &leaderboard, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get top gift receivers: %w", err)
	}

	return leaderboard, nil
}

// ===============================
// Platform Analytics (Admin)
// ===============================

// GetPlatformCommissionSummary retrieves platform commission statistics
func (s *GiftService) GetPlatformCommissionSummary(ctx context.Context) (*models.PlatformCommissionSummary, error) {
	summary := &models.PlatformCommissionSummary{}

	// Get overall statistics
	err := s.db.GetContext(ctx, summary, `
		SELECT 
			COALESCE(SUM(commission_amount), 0) as total_commissions,
			COUNT(*) as total_gifts_processed,
			COALESCE(SUM(original_gift_price), 0) as total_original_amount,
			COALESCE(AVG(commission_amount), 0) as average_commission
		FROM platform_commissions
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to get commission summary: %w", err)
	}

	// Get today's commission
	var todayCommission sql.NullInt64
	err = s.db.GetContext(ctx, &todayCommission, `
		SELECT COALESCE(SUM(commission_amount), 0) as commission_today
		FROM platform_commissions
		WHERE DATE(created_at) = CURRENT_DATE
	`)
	if err == nil && todayCommission.Valid {
		summary.CommissionToday = todayCommission.Int64
	}

	// Get this week's commission
	var weekCommission sql.NullInt64
	err = s.db.GetContext(ctx, &weekCommission, `
		SELECT COALESCE(SUM(commission_amount), 0) as commission_this_week
		FROM platform_commissions
		WHERE created_at >= DATE_TRUNC('week', CURRENT_DATE)
	`)
	if err == nil && weekCommission.Valid {
		summary.CommissionThisWeek = weekCommission.Int64
	}

	// Get this month's commission
	var monthCommission sql.NullInt64
	err = s.db.GetContext(ctx, &monthCommission, `
		SELECT COALESCE(SUM(commission_amount), 0) as commission_this_month
		FROM platform_commissions
		WHERE created_at >= DATE_TRUNC('month', CURRENT_DATE)
	`)
	if err == nil && monthCommission.Valid {
		summary.CommissionThisMonth = monthCommission.Int64
	}

	return summary, nil
}

// RefreshLeaderboards refreshes the materialized views for leaderboards
func (s *GiftService) RefreshLeaderboards(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, "SELECT refresh_gift_leaderboards()")
	if err != nil {
		return fmt.Errorf("failed to refresh leaderboards: %w", err)
	}
	log.Println("✅ Gift leaderboards refreshed successfully")
	return nil
}
