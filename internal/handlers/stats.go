// ===============================
// internal/handlers/stats.go - Admin Analytics
// ===============================

package handlers

import (
	"net/http"

	"weibaobe/internal/database"

	"github.com/gin-gonic/gin"
)

type Stats struct {
	TotalUsers    int `json:"totalUsers" db:"total_users"`
	TotalDramas   int `json:"totalDramas" db:"total_dramas"`
	TotalEpisodes int `json:"totalEpisodes" db:"total_episodes"`
	TotalCoins    int `json:"totalCoins" db:"total_coins"`

	ActiveUsers    int `json:"activeUsers" db:"active_users"`
	PremiumDramas  int `json:"premiumDramas" db:"premium_dramas"`
	FeaturedDramas int `json:"featuredDramas" db:"featured_dramas"`

	PendingPurchases int     `json:"pendingPurchases" db:"pending_purchases"`
	TotalRevenue     float64 `json:"totalRevenue" db:"total_revenue"`
}

func GetStats(c *gin.Context) {
	db := database.GetDB()

	query := `
		SELECT 
			(SELECT COUNT(*) FROM users) as total_users,
			(SELECT COUNT(*) FROM dramas WHERE is_active = true) as total_dramas,
			(SELECT COUNT(*) FROM episodes) as total_episodes,
			(SELECT COALESCE(SUM(coins_balance), 0) FROM users) as total_coins,
			(SELECT COUNT(*) FROM users WHERE last_seen > NOW() - INTERVAL '24 hours') as active_users,
			(SELECT COUNT(*) FROM dramas WHERE is_premium = true AND is_active = true) as premium_dramas,
			(SELECT COUNT(*) FROM dramas WHERE is_featured = true AND is_active = true) as featured_dramas,
			(SELECT COUNT(*) FROM coin_purchase_requests WHERE status = 'pending_admin_verification') as pending_purchases,
			(SELECT COALESCE(SUM(paid_amount), 0) FROM coin_purchase_requests WHERE status = 'approved') as total_revenue
	`

	var stats Stats
	err := db.Get(&stats, query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch stats"})
		return
	}

	c.JSON(http.StatusOK, stats)
}
