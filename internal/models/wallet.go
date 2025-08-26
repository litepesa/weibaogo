// ===============================
// internal/models/wallet.go
// ===============================

package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"
)

type Wallet struct {
	WalletID        string    `json:"walletId" db:"wallet_id"`
	UserID          string    `json:"userId" db:"user_id"`
	UserPhoneNumber string    `json:"userPhoneNumber" db:"user_phone_number"`
	UserName        string    `json:"userName" db:"user_name"`
	CoinsBalance    int       `json:"coinsBalance" db:"coins_balance"`
	CreatedAt       time.Time `json:"createdAt" db:"created_at"`
	UpdatedAt       time.Time `json:"updatedAt" db:"updated_at"`
}

type WalletTransaction struct {
	TransactionID    string      `json:"transactionId" db:"transaction_id"`
	WalletID         string      `json:"walletId" db:"wallet_id"`
	UserID           string      `json:"userId" db:"user_id"`
	UserPhoneNumber  string      `json:"userPhoneNumber" db:"user_phone_number"`
	UserName         string      `json:"userName" db:"user_name"`
	Type             string      `json:"type" db:"type"`
	CoinAmount       int         `json:"coinAmount" db:"coin_amount"`
	BalanceBefore    int         `json:"balanceBefore" db:"balance_before"`
	BalanceAfter     int         `json:"balanceAfter" db:"balance_after"`
	Description      string      `json:"description" db:"description"`
	ReferenceID      *string     `json:"referenceId" db:"reference_id"`
	AdminNote        *string     `json:"adminNote" db:"admin_note"`
	PaymentMethod    *string     `json:"paymentMethod" db:"payment_method"`
	PaymentReference *string     `json:"paymentReference" db:"payment_reference"`
	PackageID        *string     `json:"packageId" db:"package_id"`
	PaidAmount       *float64    `json:"paidAmount" db:"paid_amount"`
	Metadata         MetadataMap `json:"metadata" db:"metadata"`
	CreatedAt        time.Time   `json:"createdAt" db:"created_at"`
}

type MetadataMap map[string]interface{}

func (m MetadataMap) Value() (driver.Value, error) {
	return json.Marshal(m)
}

func (m *MetadataMap) Scan(value interface{}) error {
	if value == nil {
		*m = MetadataMap{}
		return nil
	}
	return json.Unmarshal(value.([]byte), m)
}

type CoinPurchaseRequest struct {
	ID               string     `json:"id" db:"id"`
	UserID           string     `json:"userId" db:"user_id"`
	PackageID        string     `json:"packageId" db:"package_id"`
	CoinAmount       int        `json:"coinAmount" db:"coin_amount"`
	PaidAmount       float64    `json:"paidAmount" db:"paid_amount"`
	PaymentReference string     `json:"paymentReference" db:"payment_reference"`
	PaymentMethod    string     `json:"paymentMethod" db:"payment_method"`
	Status           string     `json:"status" db:"status"`
	RequestedAt      time.Time  `json:"requestedAt" db:"requested_at"`
	ProcessedAt      *time.Time `json:"processedAt" db:"processed_at"`
	AdminNote        *string    `json:"adminNote" db:"admin_note"`
}

// Constants for coin packages
const (
	StarterPackCoins = 99
	StarterPackPrice = 100.0
	PopularPackCoins = 495
	PopularPackPrice = 500.0
	ValuePackCoins   = 990
	ValuePackPrice   = 1000.0
	DramaUnlockCost  = 99
)

var CoinPackages = map[string]struct {
	Coins int
	Price float64
	Name  string
}{
	"coins_99":  {Coins: StarterPackCoins, Price: StarterPackPrice, Name: "Starter Pack"},
	"coins_495": {Coins: PopularPackCoins, Price: PopularPackPrice, Name: "Popular Pack"},
	"coins_990": {Coins: ValuePackCoins, Price: ValuePackPrice, Name: "Value Pack"},
}
