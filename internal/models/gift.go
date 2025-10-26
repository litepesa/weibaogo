// ===============================
// internal/models/gift.go - Virtual Gift Models
// ===============================

package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"
)

// GiftRarity represents gift rarity levels
type GiftRarity string

const (
	GiftRarityCommon    GiftRarity = "common"
	GiftRarityUncommon  GiftRarity = "uncommon"
	GiftRarityRare      GiftRarity = "rare"
	GiftRarityEpic      GiftRarity = "epic"
	GiftRarityLegendary GiftRarity = "legendary"
	GiftRarityMythic    GiftRarity = "mythic"
	GiftRarityUltimate  GiftRarity = "ultimate"
)

// GiftTransaction represents a gift sent from one user to another
type GiftTransaction struct {
	ID                     string          `json:"id" db:"id"`
	SenderID               string          `json:"senderId" db:"sender_id"`
	SenderName             string          `json:"senderName" db:"sender_name"`
	SenderPhone            string          `json:"senderPhone" db:"sender_phone"`
	RecipientID            string          `json:"recipientId" db:"recipient_id"`
	RecipientName          string          `json:"recipientName" db:"recipient_name"`
	RecipientPhone         string          `json:"recipientPhone" db:"recipient_phone"`
	GiftID                 string          `json:"giftId" db:"gift_id"`
	GiftName               string          `json:"giftName" db:"gift_name"`
	GiftEmoji              string          `json:"giftEmoji" db:"gift_emoji"`
	GiftRarity             GiftRarity      `json:"giftRarity" db:"gift_rarity"`
	GiftPrice              int             `json:"giftPrice" db:"gift_price"`
	RecipientAmount        int             `json:"recipientAmount" db:"recipient_amount"`
	PlatformCommission     int             `json:"platformCommission" db:"platform_commission"`
	CommissionRate         float64         `json:"commissionRate" db:"commission_rate"`
	SenderTransactionID    *string         `json:"senderTransactionId" db:"sender_transaction_id"`
	RecipientTransactionID *string         `json:"recipientTransactionId" db:"recipient_transaction_id"`
	Message                *string         `json:"message" db:"message"`
	Context                *string         `json:"context" db:"context"`
	Metadata               GiftMetadataMap `json:"metadata" db:"metadata"`
	CreatedAt              time.Time       `json:"createdAt" db:"created_at"`
}

// GiftMetadataMap for storing additional gift information
type GiftMetadataMap map[string]interface{}

func (m GiftMetadataMap) Value() (driver.Value, error) {
	return json.Marshal(m)
}

func (m *GiftMetadataMap) Scan(value interface{}) error {
	if value == nil {
		*m = GiftMetadataMap{}
		return nil
	}
	return json.Unmarshal(value.([]byte), m)
}

// PlatformCommission represents platform earnings from a gift
type PlatformCommission struct {
	ID                string    `json:"id" db:"id"`
	GiftTransactionID string    `json:"giftTransactionId" db:"gift_transaction_id"`
	CommissionAmount  int       `json:"commissionAmount" db:"commission_amount"`
	OriginalAmount    int       `json:"originalAmount" db:"original_amount"`
	CommissionRate    float64   `json:"commissionRate" db:"commission_rate"`
	SenderID          string    `json:"senderId" db:"sender_id"`
	RecipientID       string    `json:"recipientId" db:"recipient_id"`
	GiftID            string    `json:"giftId" db:"gift_id"`
	GiftName          string    `json:"giftName" db:"gift_name"`
	CreatedAt         time.Time `json:"createdAt" db:"created_at"`
}

// SendGiftRequest represents a request to send a gift
type SendGiftRequest struct {
	RecipientID string  `json:"recipientId" binding:"required"`
	GiftID      string  `json:"giftId" binding:"required"`
	Message     *string `json:"message"`
	Context     *string `json:"context"` // e.g., "video", "profile", "live_stream"
}

// SendGiftResponse represents the response after sending a gift
type SendGiftResponse struct {
	Success             bool             `json:"success"`
	GiftTransaction     *GiftTransaction `json:"giftTransaction"`
	SenderNewBalance    int              `json:"senderNewBalance"`
	RecipientNewBalance int              `json:"recipientNewBalance"`
	PlatformCommission  int              `json:"platformCommission"`
	Message             string           `json:"message"`
}

// GiftStats represents gift statistics for a user
type GiftStats struct {
	UserID                    string     `json:"userId"`
	UserName                  string     `json:"userName"`
	GiftsSent                 int        `json:"giftsSent"`
	GiftsReceived             int        `json:"giftsReceived"`
	TotalCoinsSpentOnGifts    int        `json:"totalCoinsSpentOnGifts"`
	TotalCoinsEarnedFromGifts int        `json:"totalCoinsEarnedFromGifts"`
	MostSentGift              *string    `json:"mostSentGift"`
	MostReceivedGift          *string    `json:"mostReceivedGift"`
	LastGiftSentAt            *time.Time `json:"lastGiftSentAt"`
	LastGiftReceivedAt        *time.Time `json:"lastGiftReceivedAt"`
}

// PlatformCommissionSummary represents overall platform commission stats
type PlatformCommissionSummary struct {
	TotalCommissions    int64   `json:"totalCommissions" db:"total_commissions"`
	TotalGiftsProcessed int64   `json:"totalGiftsProcessed" db:"total_gifts_processed"`
	TotalOriginalAmount int64   `json:"totalOriginalAmount" db:"total_original_amount"`
	AverageCommission   float64 `json:"averageCommission" db:"average_commission"`
	CommissionToday     int64   `json:"commissionToday" db:"commission_today"`
	CommissionThisWeek  int64   `json:"commissionThisWeek" db:"commission_this_week"`
	CommissionThisMonth int64   `json:"commissionThisMonth" db:"commission_this_month"`
}

// TopGiftSender represents a top gift sender
type TopGiftSender struct {
	UserID       string  `json:"userId" db:"user_id"`
	UserName     string  `json:"userName" db:"user_name"`
	GiftsSent    int     `json:"giftsSent" db:"gifts_sent"`
	TotalSpent   int     `json:"totalSpent" db:"total_spent"`
	MostSentGift *string `json:"mostSentGift" db:"most_sent_gift"`
}

// TopGiftReceiver represents a top gift receiver
type TopGiftReceiver struct {
	UserID           string  `json:"userId" db:"user_id"`
	UserName         string  `json:"userName" db:"user_name"`
	GiftsReceived    int     `json:"giftsReceived" db:"gifts_received"`
	TotalEarned      int     `json:"totalEarned" db:"total_earned"`
	MostReceivedGift *string `json:"mostReceivedGift" db:"most_received_gift"`
}

// GiftHistory represents gift transaction history
type GiftHistoryItem struct {
	GiftTransaction
	Type string `json:"type"` // "sent" or "received"
}

// Constants for gift system
const (
	DefaultCommissionRate = 30.0   // 30% platform commission
	MinGiftPrice          = 10     // Minimum gift price in coins
	MaxGiftPrice          = 100000 // Maximum gift price in coins
)

// Helper methods for GiftTransaction
func (gt *GiftTransaction) IsSender(userID string) bool {
	return gt.SenderID == userID
}

func (gt *GiftTransaction) IsRecipient(userID string) bool {
	return gt.RecipientID == userID
}

func (gt *GiftTransaction) GetUserRole(userID string) string {
	if gt.IsSender(userID) {
		return "sender"
	} else if gt.IsRecipient(userID) {
		return "recipient"
	}
	return ""
}

// CalculateCommission calculates platform commission and recipient amount
func CalculateCommission(giftPrice int, commissionRate float64) (recipientAmount int, platformCommission int) {
	platformCommission = int(float64(giftPrice) * (commissionRate / 100.0))
	recipientAmount = giftPrice - platformCommission
	return
}

// ValidateGiftPrice validates if gift price is within acceptable range
func ValidateGiftPrice(price int) bool {
	return price >= MinGiftPrice && price <= MaxGiftPrice
}
