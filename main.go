package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

// Database Models
type User struct {
	UID            string          `json:"uid" db:"uid"`
	Name           string          `json:"name" db:"name"`
	Email          string          `json:"email" db:"email"`
	PhoneNumber    string          `json:"phoneNumber" db:"phone_number"`
	ProfileImage   string          `json:"profileImage" db:"profile_image"`
	Bio            string          `json:"bio" db:"bio"`
	UserType       string          `json:"userType" db:"user_type"`
	CoinsBalance   int             `json:"coinsBalance" db:"coins_balance"`
	FavoriteDramas []string        `json:"favoriteDramas" db:"favorite_dramas"`
	WatchHistory   []string        `json:"watchHistory" db:"watch_history"`
	DramaProgress  map[string]int  `json:"dramaProgress" db:"drama_progress"`
	UnlockedDramas []string        `json:"unlockedDramas" db:"unlocked_dramas"`
	Preferences    UserPreferences `json:"preferences" db:"preferences"`
	CreatedAt      time.Time       `json:"createdAt" db:"created_at"`
	UpdatedAt      time.Time       `json:"updatedAt" db:"updated_at"`
	LastSeen       time.Time       `json:"lastSeen" db:"last_seen"`
}

type UserPreferences struct {
	AutoPlay             bool `json:"autoPlay"`
	ReceiveNotifications bool `json:"receiveNotifications"`
	DarkMode             bool `json:"darkMode"`
}

type Drama struct {
	DramaID           string    `json:"dramaId" db:"drama_id"`
	Title             string    `json:"title" db:"title"`
	Description       string    `json:"description" db:"description"`
	BannerImage       string    `json:"bannerImage" db:"banner_image"`
	TotalEpisodes     int       `json:"totalEpisodes" db:"total_episodes"`
	IsPremium         bool      `json:"isPremium" db:"is_premium"`
	FreeEpisodesCount int       `json:"freeEpisodesCount" db:"free_episodes_count"`
	ViewCount         int       `json:"viewCount" db:"view_count"`
	FavoriteCount     int       `json:"favoriteCount" db:"favorite_count"`
	IsFeatured        bool      `json:"isFeatured" db:"is_featured"`
	IsActive          bool      `json:"isActive" db:"is_active"`
	CreatedBy         string    `json:"createdBy" db:"created_by"`
	CreatedAt         time.Time `json:"createdAt" db:"created_at"`
	UpdatedAt         time.Time `json:"updatedAt" db:"updated_at"`
	PublishedAt       time.Time `json:"publishedAt" db:"published_at"`
}

type Episode struct {
	EpisodeID        string    `json:"episodeId" db:"episode_id"`
	DramaID          string    `json:"dramaId" db:"drama_id"`
	EpisodeNumber    int       `json:"episodeNumber" db:"episode_number"`
	EpisodeTitle     string    `json:"episodeTitle" db:"episode_title"`
	ThumbnailURL     string    `json:"thumbnailUrl" db:"thumbnail_url"`
	VideoURL         string    `json:"videoUrl" db:"video_url"`
	VideoDuration    int       `json:"videoDuration" db:"video_duration"`
	EpisodeViewCount int       `json:"episodeViewCount" db:"episode_view_count"`
	UploadedBy       string    `json:"uploadedBy" db:"uploaded_by"`
	CreatedAt        time.Time `json:"createdAt" db:"created_at"`
	UpdatedAt        time.Time `json:"updatedAt" db:"updated_at"`
	ReleasedAt       time.Time `json:"releasedAt" db:"released_at"`
}

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
	TransactionID    string                 `json:"transactionId" db:"transaction_id"`
	WalletID         string                 `json:"walletId" db:"wallet_id"`
	UserID           string                 `json:"userId" db:"user_id"`
	UserPhoneNumber  string                 `json:"userPhoneNumber" db:"user_phone_number"`
	UserName         string                 `json:"userName" db:"user_name"`
	Type             string                 `json:"type" db:"type"`
	CoinAmount       int                    `json:"coinAmount" db:"coin_amount"`
	BalanceBefore    int                    `json:"balanceBefore" db:"balance_before"`
	BalanceAfter     int                    `json:"balanceAfter" db:"balance_after"`
	Description      string                 `json:"description" db:"description"`
	ReferenceID      *string                `json:"referenceId" db:"reference_id"`
	AdminNote        *string                `json:"adminNote" db:"admin_note"`
	PaymentMethod    *string                `json:"paymentMethod" db:"payment_method"`
	PaymentReference *string                `json:"paymentReference" db:"payment_reference"`
	PackageID        *string                `json:"packageId" db:"package_id"`
	PaidAmount       *float64               `json:"paidAmount" db:"paid_amount"`
	Metadata         map[string]interface{} `json:"metadata" db:"metadata"`
	CreatedAt        time.Time              `json:"createdAt" db:"created_at"`
}

// Global variables
var (
	db       *sqlx.DB
	s3Client *s3.S3
)

// Initialize database and services
func initDB() error {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return fmt.Errorf("DATABASE_URL environment variable not set")
	}

	var err error
	db, err = sqlx.Connect("postgres", dbURL)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	// Test connection
	if err := db.Ping(); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	log.Println("Successfully connected to PostgreSQL database")
	return nil
}

func initR2() error {
	accountID := os.Getenv("R2_ACCOUNT_ID")
	accessKey := os.Getenv("R2_ACCESS_KEY")
	secretKey := os.Getenv("R2_SECRET_KEY")

	if accountID == "" || accessKey == "" || secretKey == "" {
		return fmt.Errorf("R2 credentials not set")
	}

	// Create session with R2 endpoint
	sess, err := session.NewSession(&aws.Config{
		Region:           aws.String("auto"),
		Endpoint:         aws.String(fmt.Sprintf("https://%s.r2.cloudflarestorage.com", accountID)),
		Credentials:      credentials.NewStaticCredentials(accessKey, secretKey, ""),
		S3ForcePathStyle: aws.Bool(true),
	})

	if err != nil {
		return fmt.Errorf("failed to create R2 session: %w", err)
	}

	s3Client = s3.New(sess)
	log.Println("Successfully initialized R2 client")
	return nil
}

// Middleware for Firebase Auth verification
func firebaseAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header required"})
			c.Abort()
			return
		}

		// Extract token from "Bearer <token>"
		tokenParts := strings.Split(authHeader, " ")
		if len(tokenParts) != 2 || tokenParts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid authorization header format"})
			c.Abort()
			return
		}

		token := tokenParts[1]

		// Here you would verify the Firebase token
		// For now, we'll extract the UID from the token (simplified)
		// In production, use Firebase Admin SDK to verify

		// Simplified: assume the token contains the UID (you should verify with Firebase)
		// This is just a placeholder - implement proper Firebase token verification
		userID := extractUIDFromToken(token)
		if userID == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			c.Abort()
			return
		}

		c.Set("userID", userID)
		c.Next()
	}
}

// Placeholder function - implement proper Firebase token verification
func extractUIDFromToken(token string) string {
	// This is a simplified implementation
	// In production, use Firebase Admin SDK to verify the token
	// and extract the UID
	return "placeholder_uid"
}

// API Handlers

// User Handlers
func createUser(c *gin.Context) {
	var user User
	if err := c.ShouldBindJSON(&user); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user.CreatedAt = time.Now()
	user.UpdatedAt = time.Now()
	user.LastSeen = time.Now()

	// Convert slices and maps to JSON for storage
	favoriteDramas, _ := json.Marshal(user.FavoriteDramas)
	watchHistory, _ := json.Marshal(user.WatchHistory)
	dramaProgress, _ := json.Marshal(user.DramaProgress)
	unlockedDramas, _ := json.Marshal(user.UnlockedDramas)
	preferences, _ := json.Marshal(user.Preferences)

	query := `
		INSERT INTO users (uid, name, email, phone_number, profile_image, bio, user_type, 
		                   coins_balance, favorite_dramas, watch_history, drama_progress, 
		                   unlocked_dramas, preferences, created_at, updated_at, last_seen)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
		RETURNING uid`

	var returnedUID string
	err := db.QueryRow(query, user.UID, user.Name, user.Email, user.PhoneNumber,
		user.ProfileImage, user.Bio, user.UserType, user.CoinsBalance,
		string(favoriteDramas), string(watchHistory), string(dramaProgress),
		string(unlockedDramas), string(preferences), user.CreatedAt,
		user.UpdatedAt, user.LastSeen).Scan(&returnedUID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"uid": returnedUID, "message": "User created successfully"})
}

func getUser(c *gin.Context) {
	userID := c.Param("uid")

	var user User
	var favoriteDramas, watchHistory, dramaProgress, unlockedDramas, preferences string

	query := `
		SELECT uid, name, email, phone_number, profile_image, bio, user_type, 
		       coins_balance, favorite_dramas, watch_history, drama_progress, 
		       unlocked_dramas, preferences, created_at, updated_at, last_seen
		FROM users WHERE uid = $1`

	err := db.QueryRow(query, userID).Scan(
		&user.UID, &user.Name, &user.Email, &user.PhoneNumber,
		&user.ProfileImage, &user.Bio, &user.UserType, &user.CoinsBalance,
		&favoriteDramas, &watchHistory, &dramaProgress, &unlockedDramas,
		&preferences, &user.CreatedAt, &user.UpdatedAt, &user.LastSeen,
	)

	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Unmarshal JSON fields
	json.Unmarshal([]byte(favoriteDramas), &user.FavoriteDramas)
	json.Unmarshal([]byte(watchHistory), &user.WatchHistory)
	json.Unmarshal([]byte(dramaProgress), &user.DramaProgress)
	json.Unmarshal([]byte(unlockedDramas), &user.UnlockedDramas)
	json.Unmarshal([]byte(preferences), &user.Preferences)

	c.JSON(http.StatusOK, user)
}

func updateUser(c *gin.Context) {
	userID := c.Param("uid")
	var user User
	if err := c.ShouldBindJSON(&user); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user.UpdatedAt = time.Now()

	// Convert slices and maps to JSON
	favoriteDramas, _ := json.Marshal(user.FavoriteDramas)
	watchHistory, _ := json.Marshal(user.WatchHistory)
	dramaProgress, _ := json.Marshal(user.DramaProgress)
	unlockedDramas, _ := json.Marshal(user.UnlockedDramas)
	preferences, _ := json.Marshal(user.Preferences)

	query := `
		UPDATE users SET name = $2, email = $3, phone_number = $4, profile_image = $5,
		                bio = $6, user_type = $7, coins_balance = $8, favorite_dramas = $9,
		                watch_history = $10, drama_progress = $11, unlocked_dramas = $12,
		                preferences = $13, updated_at = $14, last_seen = $15
		WHERE uid = $1`

	_, err := db.Exec(query, userID, user.Name, user.Email, user.PhoneNumber,
		user.ProfileImage, user.Bio, user.UserType, user.CoinsBalance,
		string(favoriteDramas), string(watchHistory), string(dramaProgress),
		string(unlockedDramas), string(preferences), user.UpdatedAt, user.LastSeen)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "User updated successfully"})
}

// Drama Handlers
func getDramas(c *gin.Context) {
	limit := 20
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil {
			limit = parsed
		}
	}

	featured := c.Query("featured") == "true"
	premium := c.Query("premium")
	search := c.Query("search")

	query := "SELECT * FROM dramas WHERE is_active = true"
	args := []interface{}{}
	argCount := 0

	if featured {
		argCount++
		query += fmt.Sprintf(" AND is_featured = $%d", argCount)
		args = append(args, true)
	}

	if premium == "true" {
		argCount++
		query += fmt.Sprintf(" AND is_premium = $%d", argCount)
		args = append(args, true)
	} else if premium == "false" {
		argCount++
		query += fmt.Sprintf(" AND is_premium = $%d", argCount)
		args = append(args, false)
	}

	if search != "" {
		argCount++
		query += fmt.Sprintf(" AND title ILIKE $%d", argCount)
		args = append(args, "%"+search+"%")
	}

	query += " ORDER BY created_at DESC"
	argCount++
	query += fmt.Sprintf(" LIMIT $%d", argCount)
	args = append(args, limit)

	var dramas []Drama
	err := db.Select(&dramas, query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch dramas"})
		return
	}

	c.JSON(http.StatusOK, dramas)
}

func getDrama(c *gin.Context) {
	dramaID := c.Param("dramaId")

	var drama Drama
	query := "SELECT * FROM dramas WHERE drama_id = $1 AND is_active = true"
	err := db.Get(&drama, query, dramaID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Drama not found"})
		return
	}

	// Increment view count
	db.Exec("UPDATE dramas SET view_count = view_count + 1 WHERE drama_id = $1", dramaID)

	c.JSON(http.StatusOK, drama)
}

func createDrama(c *gin.Context) {
	userID := c.GetString("userID")

	var drama Drama
	if err := c.ShouldBindJSON(&drama); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	drama.DramaID = uuid.New().String()
	drama.CreatedBy = userID
	drama.CreatedAt = time.Now()
	drama.UpdatedAt = time.Now()
	drama.PublishedAt = time.Now()
	drama.IsActive = true

	query := `
		INSERT INTO dramas (drama_id, title, description, banner_image, total_episodes,
		                    is_premium, free_episodes_count, view_count, favorite_count,
		                    is_featured, is_active, created_by, created_at, updated_at, published_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)`

	_, err := db.Exec(query, drama.DramaID, drama.Title, drama.Description,
		drama.BannerImage, drama.TotalEpisodes, drama.IsPremium,
		drama.FreeEpisodesCount, 0, 0, drama.IsFeatured, drama.IsActive,
		drama.CreatedBy, drama.CreatedAt, drama.UpdatedAt, drama.PublishedAt)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create drama"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"dramaId": drama.DramaID, "message": "Drama created successfully"})
}

// Episode Handlers
func getDramaEpisodes(c *gin.Context) {
	dramaID := c.Param("dramaId")

	var episodes []Episode
	query := "SELECT * FROM episodes WHERE drama_id = $1 ORDER BY episode_number ASC"
	err := db.Select(&episodes, query, dramaID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch episodes"})
		return
	}

	c.JSON(http.StatusOK, episodes)
}

func getEpisode(c *gin.Context) {
	episodeID := c.Param("episodeId")

	var episode Episode
	query := "SELECT * FROM episodes WHERE episode_id = $1"
	err := db.Get(&episode, query, episodeID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Episode not found"})
		return
	}

	// Increment view count
	db.Exec("UPDATE episodes SET episode_view_count = episode_view_count + 1 WHERE episode_id = $1", episodeID)

	c.JSON(http.StatusOK, episode)
}

// Atomic drama unlock handler
func unlockDrama(c *gin.Context) {
	userID := c.GetString("userID")

	var request struct {
		DramaID    string `json:"dramaId" binding:"required"`
		UnlockCost int    `json:"unlockCost" binding:"required"`
		DramaTitle string `json:"dramaTitle" binding:"required"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Start transaction
	tx, err := db.Beginx()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start transaction"})
		return
	}
	defer tx.Rollback()

	// Get user and check if drama already unlocked
	var user User
	var unlockedDramas string
	err = tx.QueryRow("SELECT coins_balance, unlocked_dramas FROM users WHERE uid = $1", userID).
		Scan(&user.CoinsBalance, &unlockedDramas)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	json.Unmarshal([]byte(unlockedDramas), &user.UnlockedDramas)

	// Check if already unlocked
	for _, dramaID := range user.UnlockedDramas {
		if dramaID == request.DramaID {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Drama already unlocked"})
			return
		}
	}

	// Check sufficient balance
	if user.CoinsBalance < request.UnlockCost {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Insufficient coins"})
		return
	}

	// Verify drama exists and is premium
	var drama Drama
	err = tx.Get(&drama, "SELECT * FROM dramas WHERE drama_id = $1 AND is_active = true", request.DramaID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Drama not found"})
		return
	}

	if !drama.IsPremium {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Drama is free to watch"})
		return
	}

	// Update user balance and unlocked dramas
	user.UnlockedDramas = append(user.UnlockedDramas, request.DramaID)
	newBalance := user.CoinsBalance - request.UnlockCost
	unlockedDramasJSON, _ := json.Marshal(user.UnlockedDramas)

	_, err = tx.Exec("UPDATE users SET coins_balance = $1, unlocked_dramas = $2, updated_at = $3 WHERE uid = $4",
		newBalance, string(unlockedDramasJSON), time.Now(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user"})
		return
	}

	// Create transaction record
	transactionID := uuid.New().String()
	metadata := map[string]interface{}{
		"dramaId":    request.DramaID,
		"dramaTitle": request.DramaTitle,
		"unlockType": "full_drama",
	}
	metadataJSON, _ := json.Marshal(metadata)

	_, err = tx.Exec(`
		INSERT INTO wallet_transactions (transaction_id, wallet_id, user_id, type, coin_amount,
		                                balance_before, balance_after, description, reference_id,
		                                metadata, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		transactionID, userID, userID, "drama_unlock", request.UnlockCost,
		user.CoinsBalance, newBalance, fmt.Sprintf("Unlocked: %s", request.DramaTitle),
		request.DramaID, string(metadataJSON), time.Now())

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create transaction"})
		return
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    "Drama unlocked successfully",
		"newBalance": newBalance,
	})
}

// Wallet Handlers
func getWallet(c *gin.Context) {
	userID := c.Param("userId")

	var wallet Wallet
	query := "SELECT * FROM wallets WHERE user_id = $1"
	err := db.Get(&wallet, query, userID)
	if err != nil {
		// Create wallet if it doesn't exist
		wallet = Wallet{
			WalletID:     userID,
			UserID:       userID,
			CoinsBalance: 0,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}

		insertQuery := `
			INSERT INTO wallets (wallet_id, user_id, user_phone_number, user_name, coins_balance, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7)`

		db.Exec(insertQuery, wallet.WalletID, wallet.UserID, "", "", wallet.CoinsBalance, wallet.CreatedAt, wallet.UpdatedAt)
	}

	c.JSON(http.StatusOK, wallet)
}

func getWalletTransactions(c *gin.Context) {
	userID := c.Param("userId")
	limit := 50
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil {
			limit = parsed
		}
	}

	var transactions []WalletTransaction
	var metadataStrings []string

	query := `
		SELECT transaction_id, wallet_id, user_id, user_phone_number, user_name, type,
		       coin_amount, balance_before, balance_after, description, reference_id,
		       admin_note, payment_method, payment_reference, package_id, paid_amount,
		       metadata, created_at
		FROM wallet_transactions 
		WHERE user_id = $1 
		ORDER BY created_at DESC 
		LIMIT $2`

	rows, err := db.Query(query, userID, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch transactions"})
		return
	}
	defer rows.Close()

	for rows.Next() {
		var tx WalletTransaction
		var metadataStr string

		err := rows.Scan(&tx.TransactionID, &tx.WalletID, &tx.UserID, &tx.UserPhoneNumber,
			&tx.UserName, &tx.Type, &tx.CoinAmount, &tx.BalanceBefore, &tx.BalanceAfter,
			&tx.Description, &tx.ReferenceID, &tx.AdminNote, &tx.PaymentMethod,
			&tx.PaymentReference, &tx.PackageID, &tx.PaidAmount, &metadataStr, &tx.CreatedAt)

		if err != nil {
			continue
		}

		json.Unmarshal([]byte(metadataStr), &tx.Metadata)
		transactions = append(transactions, tx)
		metadataStrings = append(metadataStrings, metadataStr)
	}

	c.JSON(http.StatusOK, transactions)
}

// File upload handler for R2
func uploadFile(c *gin.Context) {
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No file uploaded"})
		return
	}
	defer file.Close()

	fileType := c.PostForm("type") // "banner", "thumbnail", "video"
	bucketName := os.Getenv("R2_BUCKET_NAME")

	// Generate unique filename
	filename := fmt.Sprintf("%s/%s-%s", fileType, uuid.New().String(), header.Filename)

	// Upload to R2
	_, err = s3Client.PutObject(&s3.PutObjectInput{
		Bucket:      aws.String(bucketName),
		Key:         aws.String(filename),
		Body:        file,
		ContentType: aws.String(header.Header.Get("Content-Type")),
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to upload file"})
		return
	}

	// Generate public URL
	publicURL := fmt.Sprintf("https://%s.%s.r2.cloudflarestorage.com/%s",
		bucketName, os.Getenv("R2_ACCOUNT_ID"), filename)

	c.JSON(http.StatusOK, gin.H{"url": publicURL})
}

func main() {
	// Initialize database
	if err := initDB(); err != nil {
		log.Fatal("Failed to initialize database:", err)
	}

	// Initialize R2
	if err := initR2(); err != nil {
		log.Fatal("Failed to initialize R2:", err)
	}

	// Setup Gin router
	r := gin.Default()

	// CORS middleware
	r.Use(cors.New(cors.Config{
		AllowAllOrigins:  true,
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// Health check endpoint
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy"})
	})

	// Public routes (no auth required)
	public := r.Group("/api/v1")
	{
		// Public drama routes
		public.GET("/dramas", getDramas)
		public.GET("/dramas/:dramaId", getDrama)
		public.GET("/dramas/:dramaId/episodes", getDramaEpisodes)
		public.GET("/episodes/:episodeId", getEpisode)
	}

	// Protected routes (Firebase auth required)
	protected := r.Group("/api/v1")
	protected.Use(firebaseAuthMiddleware())
	{
		// User routes
		protected.POST("/users", createUser)
		protected.GET("/users/:uid", getUser)
		protected.PUT("/users/:uid", updateUser)

		// Drama management (admin only in client)
		protected.POST("/dramas", createDrama)
		protected.PUT("/dramas/:dramaId", updateDrama)
		protected.DELETE("/dramas/:dramaId", deleteDrama)

		// Episode management
		protected.POST("/dramas/:dramaId/episodes", createEpisode)
		protected.PUT("/episodes/:episodeId", updateEpisode)
		protected.DELETE("/episodes/:episodeId", deleteEpisode)

		// Drama unlock
		protected.POST("/unlock-drama", unlockDrama)

		// Wallet routes
		protected.GET("/wallet/:userId", getWallet)
		protected.GET("/wallet/:userId/transactions", getWalletTransactions)
		protected.POST("/wallet/:userId/add-coins", addCoins) // Admin only

		// File upload
		protected.POST("/upload", uploadFile)

		// User actions
		protected.POST("/users/:uid/favorites", toggleFavorite)
		protected.POST("/users/:uid/watch-history", addToWatchHistory)
		protected.POST("/users/:uid/drama-progress", updateDramaProgress)
	}

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Server starting on port %s", port)
	log.Fatal(r.Run(":" + port))
}

// Additional handlers

func updateDrama(c *gin.Context) {
	dramaID := c.Param("dramaId")

	var drama Drama
	if err := c.ShouldBindJSON(&drama); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	drama.UpdatedAt = time.Now()

	query := `
		UPDATE dramas SET title = $2, description = $3, banner_image = $4, 
		                  total_episodes = $5, is_premium = $6, free_episodes_count = $7,
		                  is_featured = $8, is_active = $9, updated_at = $10
		WHERE drama_id = $1`

	_, err := db.Exec(query, dramaID, drama.Title, drama.Description, drama.BannerImage,
		drama.TotalEpisodes, drama.IsPremium, drama.FreeEpisodesCount,
		drama.IsFeatured, drama.IsActive, drama.UpdatedAt)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update drama"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Drama updated successfully"})
}

func deleteDrama(c *gin.Context) {
	dramaID := c.Param("dramaId")

	// Start transaction to delete drama and its episodes
	tx, err := db.Beginx()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start transaction"})
		return
	}
	defer tx.Rollback()

	// Delete episodes first
	_, err = tx.Exec("DELETE FROM episodes WHERE drama_id = $1", dramaID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete episodes"})
		return
	}

	// Delete drama
	_, err = tx.Exec("DELETE FROM dramas WHERE drama_id = $1", dramaID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete drama"})
		return
	}

	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Drama deleted successfully"})
}

func createEpisode(c *gin.Context) {
	dramaID := c.Param("dramaId")
	userID := c.GetString("userID")

	var episode Episode
	if err := c.ShouldBindJSON(&episode); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	episode.EpisodeID = uuid.New().String()
	episode.DramaID = dramaID
	episode.UploadedBy = userID
	episode.CreatedAt = time.Now()
	episode.UpdatedAt = time.Now()
	episode.ReleasedAt = time.Now()

	query := `
		INSERT INTO episodes (episode_id, drama_id, episode_number, episode_title,
		                      thumbnail_url, video_url, video_duration, episode_view_count,
		                      uploaded_by, created_at, updated_at, released_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`

	_, err := db.Exec(query, episode.EpisodeID, episode.DramaID, episode.EpisodeNumber,
		episode.EpisodeTitle, episode.ThumbnailURL, episode.VideoURL, episode.VideoDuration,
		0, episode.UploadedBy, episode.CreatedAt, episode.UpdatedAt, episode.ReleasedAt)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create episode"})
		return
	}

	// Update drama's total episodes count
	db.Exec("UPDATE dramas SET total_episodes = (SELECT COUNT(*) FROM episodes WHERE drama_id = $1), updated_at = $2 WHERE drama_id = $1",
		dramaID, time.Now())

	c.JSON(http.StatusCreated, gin.H{"episodeId": episode.EpisodeID, "message": "Episode created successfully"})
}

func updateEpisode(c *gin.Context) {
	episodeID := c.Param("episodeId")

	var episode Episode
	if err := c.ShouldBindJSON(&episode); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	episode.UpdatedAt = time.Now()

	query := `
		UPDATE episodes SET episode_number = $2, episode_title = $3, thumbnail_url = $4,
		                    video_url = $5, video_duration = $6, updated_at = $7
		WHERE episode_id = $1`

	_, err := db.Exec(query, episodeID, episode.EpisodeNumber, episode.EpisodeTitle,
		episode.ThumbnailURL, episode.VideoURL, episode.VideoDuration, episode.UpdatedAt)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update episode"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Episode updated successfully"})
}

func deleteEpisode(c *gin.Context) {
	episodeID := c.Param("episodeId")

	// Get drama ID first for updating total count
	var dramaID string
	err := db.QueryRow("SELECT drama_id FROM episodes WHERE episode_id = $1", episodeID).Scan(&dramaID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Episode not found"})
		return
	}

	// Delete episode
	_, err = db.Exec("DELETE FROM episodes WHERE episode_id = $1", episodeID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete episode"})
		return
	}

	// Update drama's total episodes count
	db.Exec("UPDATE dramas SET total_episodes = (SELECT COUNT(*) FROM episodes WHERE drama_id = $1), updated_at = $2 WHERE drama_id = $1",
		dramaID, time.Now())

	c.JSON(http.StatusOK, gin.H{"message": "Episode deleted successfully"})
}

func addCoins(c *gin.Context) {
	userID := c.Param("userId")

	var request struct {
		CoinAmount  int    `json:"coinAmount" binding:"required"`
		Description string `json:"description"`
		AdminNote   string `json:"adminNote"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Start transaction
	tx, err := db.Beginx()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start transaction"})
		return
	}
	defer tx.Rollback()

	// Get current balance
	var currentBalance int
	err = tx.QueryRow("SELECT coins_balance FROM users WHERE uid = $1", userID).Scan(&currentBalance)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	newBalance := currentBalance + request.CoinAmount

	// Update user balance
	_, err = tx.Exec("UPDATE users SET coins_balance = $1, updated_at = $2 WHERE uid = $3",
		newBalance, time.Now(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update balance"})
		return
	}

	// Create transaction record
	transactionID := uuid.New().String()
	description := request.Description
	if description == "" {
		description = "Admin added coins"
	}

	_, err = tx.Exec(`
		INSERT INTO wallet_transactions (transaction_id, wallet_id, user_id, type, coin_amount,
		                                balance_before, balance_after, description, admin_note, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		transactionID, userID, userID, "admin_credit", request.CoinAmount,
		currentBalance, newBalance, description, request.AdminNote, time.Now())

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create transaction"})
		return
	}

	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    "Coins added successfully",
		"newBalance": newBalance,
	})
}

func toggleFavorite(c *gin.Context) {
	userID := c.Param("uid")

	var request struct {
		DramaID    string `json:"dramaId" binding:"required"`
		IsFavorite bool   `json:"isFavorite"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get current favorites
	var favoritesStr string
	err := db.QueryRow("SELECT favorite_dramas FROM users WHERE uid = $1", userID).Scan(&favoritesStr)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	var favorites []string
	json.Unmarshal([]byte(favoritesStr), &favorites)

	// Update favorites list
	if request.IsFavorite {
		// Add to favorites if not already there
		found := false
		for _, id := range favorites {
			if id == request.DramaID {
				found = true
				break
			}
		}
		if !found {
			favorites = append(favorites, request.DramaID)
		}
	} else {
		// Remove from favorites
		for i, id := range favorites {
			if id == request.DramaID {
				favorites = append(favorites[:i], favorites[i+1:]...)
				break
			}
		}
	}

	// Update user
	favoritesJSON, _ := json.Marshal(favorites)
	_, err = db.Exec("UPDATE users SET favorite_dramas = $1, updated_at = $2 WHERE uid = $3",
		string(favoritesJSON), time.Now(), userID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update favorites"})
		return
	}

	// Update drama favorite count
	if request.IsFavorite {
		db.Exec("UPDATE dramas SET favorite_count = favorite_count + 1 WHERE drama_id = $1", request.DramaID)
	} else {
		db.Exec("UPDATE dramas SET favorite_count = favorite_count - 1 WHERE drama_id = $1 AND favorite_count > 0", request.DramaID)
	}

	c.JSON(http.StatusOK, gin.H{"message": "Favorites updated successfully"})
}

func addToWatchHistory(c *gin.Context) {
	userID := c.Param("uid")

	var request struct {
		EpisodeID string `json:"episodeId" binding:"required"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get current watch history
	var historyStr string
	err := db.QueryRow("SELECT watch_history FROM users WHERE uid = $1", userID).Scan(&historyStr)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	var history []string
	json.Unmarshal([]byte(historyStr), &history)

	// Add to history if not already there
	found := false
	for _, id := range history {
		if id == request.EpisodeID {
			found = true
			break
		}
	}

	if !found {
		history = append(history, request.EpisodeID)

		// Limit history to last 1000 episodes
		if len(history) > 1000 {
			history = history[len(history)-1000:]
		}

		historyJSON, _ := json.Marshal(history)
		_, err = db.Exec("UPDATE users SET watch_history = $1, updated_at = $2 WHERE uid = $3",
			string(historyJSON), time.Now(), userID)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update watch history"})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "Watch history updated successfully"})
}

func updateDramaProgress(c *gin.Context) {
	userID := c.Param("uid")

	var request struct {
		DramaID       string `json:"dramaId" binding:"required"`
		EpisodeNumber int    `json:"episodeNumber" binding:"required"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get current progress
	var progressStr string
	err := db.QueryRow("SELECT drama_progress FROM users WHERE uid = $1", userID).Scan(&progressStr)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	var progress map[string]int
	json.Unmarshal([]byte(progressStr), &progress)

	if progress == nil {
		progress = make(map[string]int)
	}

	// Update progress only if new episode number is higher
	if currentProgress, exists := progress[request.DramaID]; !exists || request.EpisodeNumber > currentProgress {
		progress[request.DramaID] = request.EpisodeNumber

		progressJSON, _ := json.Marshal(progress)
		_, err = db.Exec("UPDATE users SET drama_progress = $1, updated_at = $2 WHERE uid = $3",
			string(progressJSON), time.Now(), userID)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update drama progress"})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "Drama progress updated successfully"})
}
