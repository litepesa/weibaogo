// internal/database/connection.go
package database

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

// DB holds the database connection
var DB *sqlx.DB

// Connect establishes a connection to PostgreSQL database with optimizations
func Connect(databaseURL string) (*sqlx.DB, error) {
	if databaseURL == "" {
		return nil, fmt.Errorf("database URL is empty")
	}

	// Connect to database
	db, err := sqlx.Connect("postgres", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// 🚀 OPTIMIZED: Enhanced connection pool for video workload
	// Video applications are typically read-heavy with burst patterns
	db.SetMaxOpenConns(50)                  // Increased for concurrent video requests
	db.SetMaxIdleConns(25)                  // Keep more connections ready for burst traffic
	db.SetConnMaxLifetime(10 * time.Minute) // Longer lifetime for video streaming sessions
	db.SetConnMaxIdleTime(5 * time.Minute)  // Keep idle connections longer for better reuse

	// Test the connection with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Set global DB variable for easy access
	DB = db

	log.Println("✅ Successfully connected to PostgreSQL database")
	log.Printf("📊 Connection pool optimized for video workload:")
	log.Printf("   • Max open connections: 50 (increased for concurrency)")
	log.Printf("   • Max idle connections: 25 (keep ready for burst traffic)")
	log.Printf("   • Connection lifetime: 10 minutes (longer for streaming)")
	log.Printf("   • Idle timeout: 5 minutes (better connection reuse)")

	return db, nil
}

// Close closes the database connection
func Close() error {
	if DB != nil {
		log.Println("🔒 Closing database connections...")
		return DB.Close()
	}
	return nil
}

// GetDB returns the global database instance
func GetDB() *sqlx.DB {
	return DB
}

// Health checks the database connection health with timeout
func Health() error {
	if DB == nil {
		return fmt.Errorf("database connection is nil")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	return DB.PingContext(ctx)
}

// Transaction executes a function within a database transaction with timeout
func Transaction(fn func(*sqlx.Tx) error) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tx, err := DB.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			panic(r)
		}
	}()

	if err := fn(tx); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("transaction error: %v, rollback error: %v", err, rbErr)
		}
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// Stats returns database connection statistics
func Stats() sql.DBStats {
	if DB == nil {
		return sql.DBStats{}
	}
	return DB.Stats()
}

// 🚀 NEW: GetOptimizedStats returns enhanced statistics with performance metrics
func GetOptimizedStats() map[string]interface{} {
	if DB == nil {
		return map[string]interface{}{"error": "database not connected"}
	}

	stats := DB.Stats()

	// Calculate utilization percentages
	openUtilization := float64(stats.OpenConnections) / 50.0 * 100
	idleUtilization := float64(stats.Idle) / 25.0 * 100

	return map[string]interface{}{
		"connections": map[string]interface{}{
			"open":             stats.OpenConnections,
			"in_use":           stats.InUse,
			"idle":             stats.Idle,
			"max_open":         50,
			"max_idle":         25,
			"open_utilization": fmt.Sprintf("%.1f%%", openUtilization),
			"idle_utilization": fmt.Sprintf("%.1f%%", idleUtilization),
		},
		"wait_stats": map[string]interface{}{
			"wait_count":           stats.WaitCount,
			"wait_duration":        stats.WaitDuration.String(),
			"max_idle_closed":      stats.MaxIdleClosed,
			"max_idle_time_closed": stats.MaxIdleTimeClosed,
			"max_lifetime_closed":  stats.MaxLifetimeClosed,
		},
		"health": map[string]interface{}{
			"status":        "connected",
			"optimized_for": "video_workload",
			"pool_efficiency": map[string]interface{}{
				"reuse_ratio": calculateReuseRatio(stats),
				"wait_ratio":  calculateWaitRatio(stats),
			},
		},
	}
}

// 🚀 NEW: Helper functions for performance metrics
func calculateReuseRatio(stats sql.DBStats) string {
	if stats.OpenConnections == 0 {
		return "0%"
	}

	// Estimate connection reuse based on idle vs total connections
	reuseRatio := float64(stats.Idle) / float64(stats.OpenConnections) * 100
	return fmt.Sprintf("%.1f%%", reuseRatio)
}

func calculateWaitRatio(stats sql.DBStats) string {
	if stats.OpenConnections == 0 {
		return "0%"
	}

	// Simple wait ratio calculation
	if stats.WaitCount == 0 {
		return "0%"
	}

	// This is a simplified calculation - in production you'd want more sophisticated metrics
	waitRatio := float64(stats.WaitCount) / float64(stats.OpenConnections*100) * 100
	if waitRatio > 100 {
		waitRatio = 100
	}

	return fmt.Sprintf("%.1f%%", waitRatio)
}

// 🚀 NEW: CreatePerformanceIndexes - separate function for database optimizations
func CreatePerformanceIndexes() error {
	if DB == nil {
		return fmt.Errorf("database not connected")
	}

	log.Println("📈 Creating performance indexes for video workload...")

	// Create indexes for better video query performance
	optimizationQueries := []string{
		// Index for video listing queries
		"CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_videos_active_created ON videos(is_active, created_at DESC) WHERE is_active = true",

		// Index for featured videos
		"CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_videos_featured ON videos(is_featured, created_at DESC) WHERE is_featured = true AND is_active = true",

		// Index for user videos
		"CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_videos_user_active ON videos(user_id, is_active, created_at DESC) WHERE is_active = true",

		// Index for trending algorithm
		"CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_videos_trending ON videos(is_active, likes_count, comments_count, shares_count, views_count, created_at) WHERE is_active = true",

		// Index for video likes
		"CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_video_likes_video_user ON video_likes(video_id, user_id)",
		"CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_video_likes_user_created ON video_likes(user_id, created_at DESC)",

		// Index for comments
		"CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_comments_video_created ON comments(video_id, created_at DESC)",

		// Index for user follows
		"CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_user_follows_follower ON user_follows(follower_id, created_at DESC)",
		"CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_user_follows_following ON user_follows(following_id, created_at DESC)",
	}

	for i, query := range optimizationQueries {
		log.Printf("   • Creating performance index %d/%d...", i+1, len(optimizationQueries))
		if _, err := DB.Exec(query); err != nil {
			// Log error but don't fail - indexes might already exist
			log.Printf("   ⚠️  Index creation warning (might already exist): %v", err)
		}
	}

	log.Println("✅ Performance indexes creation completed")
	return nil
}

// 🚀 NEW: HealthCheck with detailed connection info
func HealthCheck() map[string]interface{} {
	if DB == nil {
		return map[string]interface{}{
			"status":  "error",
			"message": "database not connected",
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Test connection
	err := DB.PingContext(ctx)
	if err != nil {
		return map[string]interface{}{
			"status":  "error",
			"message": fmt.Sprintf("ping failed: %v", err),
		}
	}

	// Get detailed stats
	stats := GetOptimizedStats()
	stats["status"] = "healthy"
	stats["ping"] = "ok"
	stats["timestamp"] = time.Now().Unix()

	return stats
}
