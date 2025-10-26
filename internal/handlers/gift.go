// ===============================
// internal/handlers/gift.go - Virtual Gift Handler
// ===============================

package handlers

import (
	"net/http"
	"strconv"

	"weibaobe/internal/models"
	"weibaobe/internal/services"

	"github.com/gin-gonic/gin"
)

type GiftHandler struct {
	giftService *services.GiftService
}

func NewGiftHandler(giftService *services.GiftService) *GiftHandler {
	return &GiftHandler{giftService: giftService}
}

// Available gift catalog (matches Flutter app)
var giftCatalog = map[string]struct {
	Price  int
	Name   string
	Emoji  string
	Rarity models.GiftRarity
}{
	// Popular
	"heart":     {10, "Heart", "❤️", models.GiftRarityCommon},
	"thumbs_up": {15, "Thumbs Up", "👍", models.GiftRarityCommon},
	"clap":      {25, "Applause", "👏", models.GiftRarityUncommon},
	"fire":      {50, "Fire", "🔥", models.GiftRarityRare},
	"star":      {75, "Star", "⭐", models.GiftRarityRare},
	"crown":     {150, "Crown", "👑", models.GiftRarityEpic},
	"kiss":      {35, "Kiss", "💋", models.GiftRarityUncommon},
	"muscle":    {40, "Strong", "💪", models.GiftRarityUncommon},

	// Emotions
	"love_eyes":    {20, "Love Eyes", "😍", models.GiftRarityCommon},
	"laugh":        {15, "Laughing", "😂", models.GiftRarityCommon},
	"cool":         {30, "Cool", "😎", models.GiftRarityUncommon},
	"shocked":      {25, "Shocked", "😱", models.GiftRarityUncommon},
	"party":        {40, "Party", "🥳", models.GiftRarityRare},
	"mind_blown":   {60, "Mind Blown", "🤯", models.GiftRarityRare},
	"crying_laugh": {35, "Crying Laugh", "🤣", models.GiftRarityUncommon},
	"wink":         {20, "Wink", "😉", models.GiftRarityCommon},
	"angel":        {45, "Angel", "😇", models.GiftRarityRare},
	"devil":        {45, "Devil", "😈", models.GiftRarityRare},

	// Animals
	"cat":      {25, "Cat", "🐱", models.GiftRarityCommon},
	"dog":      {25, "Dog", "🐶", models.GiftRarityCommon},
	"bear":     {40, "Bear", "🐻", models.GiftRarityUncommon},
	"lion":     {65, "Lion", "🦁", models.GiftRarityRare},
	"elephant": {55, "Elephant", "🐘", models.GiftRarityRare},
	"eagle":    {70, "Eagle", "🦅", models.GiftRarityRare},
	"dragon":   {180, "Dragon", "🐉", models.GiftRarityEpic},
	"phoenix":  {350, "Phoenix", "🔥🦅", models.GiftRarityLegendary},

	// Luxury
	"diamond":     {200, "Diamond", "💎", models.GiftRarityEpic},
	"trophy":      {100, "Trophy", "🏆", models.GiftRarityRare},
	"rocket":      {120, "Rocket", "🚀", models.GiftRarityEpic},
	"money_bag":   {250, "Money Bag", "💰", models.GiftRarityEpic},
	"unicorn":     {300, "Unicorn", "🦄", models.GiftRarityLegendary},
	"rainbow":     {500, "Rainbow", "🌈", models.GiftRarityLegendary},
	"sports_car":  {800, "Sports Car", "🏎️", models.GiftRarityLegendary},
	"mansion":     {1200, "Mansion", "🏰", models.GiftRarityLegendary},
	"yacht":       {2500, "Yacht", "🛥️", models.GiftRarityMythic},
	"private_jet": {5000, "Private Jet", "🛩️", models.GiftRarityMythic},

	// Food
	"coffee":    {35, "Coffee", "☕", models.GiftRarityUncommon},
	"pizza":     {45, "Pizza", "🍕", models.GiftRarityUncommon},
	"cake":      {55, "Birthday Cake", "🎂", models.GiftRarityRare},
	"champagne": {80, "Champagne", "🍾", models.GiftRarityRare},
	"donut":     {25, "Donut", "🍩", models.GiftRarityCommon},
	"ice_cream": {30, "Ice Cream", "🍦", models.GiftRarityUncommon},
	"burger":    {40, "Burger", "🍔", models.GiftRarityUncommon},
	"sushi":     {60, "Sushi", "🍣", models.GiftRarityRare},
	"lobster":   {120, "Lobster", "🦞", models.GiftRarityEpic},
	"caviar":    {300, "Caviar", "🥄", models.GiftRarityLegendary},

	// Travel
	"beach":      {400, "Beach Vacation", "🏖️", models.GiftRarityLegendary},
	"mountain":   {350, "Mountain Trip", "🏔️", models.GiftRarityLegendary},
	"city_break": {300, "City Break", "🏙️", models.GiftRarityLegendary},
	"safari":     {800, "Safari Adventure", "🦓", models.GiftRarityLegendary},
	"cruise":     {1500, "Luxury Cruise", "🛳️", models.GiftRarityMythic},
	"space_trip": {15000, "Space Trip", "🚀🌌", models.GiftRarityUltimate},

	// Ultra Premium
	"golden_crown":   {1000, "Golden Crown", "👑✨", models.GiftRarityMythic},
	"diamond_ring":   {2000, "Diamond Ring", "💍", models.GiftRarityMythic},
	"golden_statue":  {3500, "Golden Statue", "🗿✨", models.GiftRarityMythic},
	"treasure_chest": {5000, "Treasure Chest", "💰⭐", models.GiftRarityUltimate},
	"palace":         {8000, "Royal Palace", "🏰👑", models.GiftRarityUltimate},
	"island":         {12000, "Private Island", "🏝️🌴", models.GiftRarityUltimate},
	"galaxy":         {25000, "Own a Galaxy", "🌌⭐", models.GiftRarityUltimate},
	"universe":       {50000, "The Universe", "🌌✨🪐", models.GiftRarityUltimate},

	// Nature
	"flower":  {20, "Flower", "🌸", models.GiftRarityCommon},
	"rose":    {45, "Rose", "🌹", models.GiftRarityUncommon},
	"bouquet": {85, "Bouquet", "💐", models.GiftRarityRare},
	"tree":    {60, "Tree", "🌳", models.GiftRarityRare},
	"forest":  {200, "Forest", "🌲🌳", models.GiftRarityEpic},
	"garden":  {400, "Garden Paradise", "🌺🌸🌼", models.GiftRarityLegendary},
	"aurora":  {800, "Aurora Borealis", "🌌💚", models.GiftRarityLegendary},

	// Sports
	"soccer_ball":  {30, "Soccer Ball", "⚽", models.GiftRarityUncommon},
	"basketball":   {35, "Basketball", "🏀", models.GiftRarityUncommon},
	"volleyball":   {25, "Volleyball", "🏐", models.GiftRarityCommon},
	"tennis":       {40, "Tennis", "🎾", models.GiftRarityUncommon},
	"medal":        {150, "Gold Medal", "🥇", models.GiftRarityEpic},
	"championship": {500, "Championship", "🏆⭐", models.GiftRarityLegendary},
	"olympics":     {1000, "Olympic Victory", "🥇🌟", models.GiftRarityMythic},

	// Celestial
	"moon":          {100, "Moon", "🌙", models.GiftRarityRare},
	"sun":           {150, "Sun", "☀️", models.GiftRarityEpic},
	"shooting_star": {250, "Shooting Star", "💫", models.GiftRarityEpic},
	"constellation": {600, "Constellation", "✨⭐✨", models.GiftRarityLegendary},
	"supernova":     {1500, "Supernova", "💥⭐", models.GiftRarityMythic},
	"black_hole":    {3000, "Black Hole", "🕳️✨", models.GiftRarityMythic},
	"big_bang":      {10000, "Big Bang", "💥🌌", models.GiftRarityUltimate},
}

// SendGift handles sending a virtual gift
func (h *GiftHandler) SendGift(c *gin.Context) {
	senderID := c.GetString("userID")
	if senderID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var request models.SendGiftRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate sender is not sending gift to themselves
	if senderID == request.RecipientID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot send gift to yourself"})
		return
	}

	// Get gift details from catalog
	gift, exists := giftCatalog[request.GiftID]
	if !exists {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid gift ID"})
		return
	}

	// Validate gift price
	if !models.ValidateGiftPrice(gift.Price) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid gift price"})
		return
	}

	// Send gift
	response, err := h.giftService.SendGift(
		c.Request.Context(),
		senderID,
		request,
		gift.Price,
		gift.Name,
		gift.Emoji,
		gift.Rarity,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, response)
}

// GetGiftHistory retrieves gift history for a user
func (h *GiftHandler) GetGiftHistory(c *gin.Context) {
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

	offset := 0
	if o := c.Query("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	history, err := h.giftService.GetUserGiftHistory(c.Request.Context(), userID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch gift history"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"history": history,
		"total":   len(history),
	})
}

// GetGiftStats retrieves gift statistics for a user
func (h *GiftHandler) GetGiftStats(c *gin.Context) {
	userID := c.Param("userId")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "User ID required"})
		return
	}

	stats, err := h.giftService.GetUserGiftStats(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// GetPlatformCommissionSummary retrieves platform commission statistics (admin only)
func (h *GiftHandler) GetPlatformCommissionSummary(c *gin.Context) {
	summary, err := h.giftService.GetPlatformCommissionSummary(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch commission summary"})
		return
	}

	c.JSON(http.StatusOK, summary)
}

// GetTopGiftSenders retrieves top gift senders (admin only)
func (h *GiftHandler) GetTopGiftSenders(c *gin.Context) {
	limit := 10
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}

	senders, err := h.giftService.GetTopGiftSenders(c.Request.Context(), limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch top senders"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"topSenders": senders,
		"total":      len(senders),
	})
}

// GetTopGiftReceivers retrieves top gift receivers (admin only)
func (h *GiftHandler) GetTopGiftReceivers(c *gin.Context) {
	limit := 10
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}

	receivers, err := h.giftService.GetTopGiftReceivers(c.Request.Context(), limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch top receivers"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"topReceivers": receivers,
		"total":        len(receivers),
	})
}

// GetGiftCatalog returns available gifts
func (h *GiftHandler) GetGiftCatalog(c *gin.Context) {
	catalog := make([]gin.H, 0, len(giftCatalog))

	for id, gift := range giftCatalog {
		recipientAmount, platformCommission := models.CalculateCommission(gift.Price, models.DefaultCommissionRate)

		catalog = append(catalog, gin.H{
			"id":                 id,
			"name":               gift.Name,
			"emoji":              gift.Emoji,
			"price":              gift.Price,
			"rarity":             gift.Rarity,
			"recipientAmount":    recipientAmount,
			"platformCommission": platformCommission,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"gifts":          catalog,
		"total":          len(catalog),
		"commissionRate": models.DefaultCommissionRate,
	})
}

// GetGiftTransaction retrieves a specific gift transaction
func (h *GiftHandler) GetGiftTransaction(c *gin.Context) {
	transactionID := c.Param("transactionId")
	if transactionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Transaction ID required"})
		return
	}

	transaction, err := h.giftService.GetGiftTransaction(c.Request.Context(), transactionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Transaction not found"})
		return
	}

	c.JSON(http.StatusOK, transaction)
}
