// ===============================
// internal/database/migrations.go - Video Social Media Schema
// ===============================

package database

import (
	"fmt"
	"log"

	"github.com/jmoiron/sqlx"
)

func RunMigrations(db *sqlx.DB) error {
	log.Println("📄 Running video social media migrations...")

	// Check if migrations table exists
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS migrations (
			id SERIAL PRIMARY KEY,
			version VARCHAR(255) UNIQUE NOT NULL,
			applied_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	migrations := []Migration{
		{
			Version: "001_initial_video_schema",
			Query: `
				-- Users table - no email, phone-based auth
				CREATE TABLE IF NOT EXISTS users (
					uid VARCHAR(255) PRIMARY KEY,
					name VARCHAR(255) NOT NULL,
					phone_number VARCHAR(20) UNIQUE NOT NULL,
					profile_image TEXT DEFAULT '',
					cover_image TEXT DEFAULT '',
					bio TEXT DEFAULT '',
					user_type VARCHAR(20) DEFAULT 'user' CHECK (user_type IN ('user', 'admin', 'moderator')),
					followers_count INTEGER DEFAULT 0,
					following_count INTEGER DEFAULT 0,
					videos_count INTEGER DEFAULT 0,
					likes_count INTEGER DEFAULT 0,
					is_verified BOOLEAN DEFAULT false,
					is_active BOOLEAN DEFAULT true,
					is_featured BOOLEAN DEFAULT false,
					tags TEXT[] DEFAULT '{}',
					created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
					updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
					last_seen TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
				);

				-- Videos table - core content
				CREATE TABLE IF NOT EXISTS videos (
					id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
					user_id VARCHAR(255) NOT NULL REFERENCES users(uid) ON DELETE CASCADE,
					user_name VARCHAR(255) NOT NULL,
					user_image TEXT DEFAULT '',
					video_url TEXT NOT NULL,
					thumbnail_url TEXT DEFAULT '',
					caption TEXT DEFAULT '',
					likes_count INTEGER DEFAULT 0,
					comments_count INTEGER DEFAULT 0,
					views_count INTEGER DEFAULT 0,
					shares_count INTEGER DEFAULT 0,
					tags TEXT[] DEFAULT '{}',
					is_active BOOLEAN DEFAULT true,
					is_featured BOOLEAN DEFAULT false,
					is_multiple_images BOOLEAN DEFAULT false,
					image_urls TEXT[] DEFAULT '{}',
					created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
					updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
				);

				-- Comments table
				CREATE TABLE IF NOT EXISTS comments (
					id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
					video_id UUID NOT NULL REFERENCES videos(id) ON DELETE CASCADE,
					author_id VARCHAR(255) NOT NULL REFERENCES users(uid) ON DELETE CASCADE,
					author_name VARCHAR(255) NOT NULL,
					author_image TEXT DEFAULT '',
					content TEXT NOT NULL,
					likes_count INTEGER DEFAULT 0,
					is_reply BOOLEAN DEFAULT false,
					replied_to_comment_id UUID REFERENCES comments(id) ON DELETE CASCADE,
					replied_to_author_name VARCHAR(255),
					created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
					updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
				);

				-- Video likes table
				CREATE TABLE IF NOT EXISTS video_likes (
					id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
					video_id UUID NOT NULL REFERENCES videos(id) ON DELETE CASCADE,
					user_id VARCHAR(255) NOT NULL REFERENCES users(uid) ON DELETE CASCADE,
					created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
					UNIQUE(video_id, user_id)
				);

				-- Comment likes table
				CREATE TABLE IF NOT EXISTS comment_likes (
					id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
					comment_id UUID NOT NULL REFERENCES comments(id) ON DELETE CASCADE,
					user_id VARCHAR(255) NOT NULL REFERENCES users(uid) ON DELETE CASCADE,
					created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
					UNIQUE(comment_id, user_id)
				);

				-- User follows table
				CREATE TABLE IF NOT EXISTS user_follows (
					id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
					follower_id VARCHAR(255) NOT NULL REFERENCES users(uid) ON DELETE CASCADE,
					following_id VARCHAR(255) NOT NULL REFERENCES users(uid) ON DELETE CASCADE,
					created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
					UNIQUE(follower_id, following_id),
					CHECK(follower_id != following_id)
				);
			`,
		},
		{
			Version: "002_wallet_system",
			Query: `
				-- Wallets table
				CREATE TABLE IF NOT EXISTS wallets (
					wallet_id VARCHAR(255) PRIMARY KEY,
					user_id VARCHAR(255) UNIQUE NOT NULL REFERENCES users(uid) ON DELETE CASCADE,
					user_phone_number VARCHAR(20) NOT NULL,
					user_name VARCHAR(255) NOT NULL,
					coins_balance INTEGER DEFAULT 0,
					created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
					updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
				);

				-- Wallet transactions table
				CREATE TABLE IF NOT EXISTS wallet_transactions (
					transaction_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
					wallet_id VARCHAR(255) NOT NULL REFERENCES wallets(wallet_id),
					user_id VARCHAR(255) NOT NULL REFERENCES users(uid) ON DELETE CASCADE,
					user_phone_number VARCHAR(20) NOT NULL,
					user_name VARCHAR(255) NOT NULL,
					type VARCHAR(50) NOT NULL,
					coin_amount INTEGER NOT NULL,
					balance_before INTEGER NOT NULL,
					balance_after INTEGER NOT NULL,
					description TEXT DEFAULT '',
					reference_id VARCHAR(255),
					admin_note TEXT,
					payment_method VARCHAR(50),
					payment_reference VARCHAR(255),
					package_id VARCHAR(50),
					paid_amount DECIMAL(10,2),
					metadata JSONB DEFAULT '{}',
					created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
				);

				-- Coin purchase requests table
				CREATE TABLE IF NOT EXISTS coin_purchase_requests (
					id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
					user_id VARCHAR(255) NOT NULL REFERENCES users(uid) ON DELETE CASCADE,
					package_id VARCHAR(50) NOT NULL,
					coin_amount INTEGER NOT NULL,
					paid_amount DECIMAL(10,2) NOT NULL,
					payment_reference VARCHAR(255) NOT NULL,
					payment_method VARCHAR(50) NOT NULL,
					status VARCHAR(50) NOT NULL DEFAULT 'pending_admin_verification',
					requested_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
					processed_at TIMESTAMP WITH TIME ZONE,
					admin_note TEXT
				);
			`,
		},
		{
			Version: "003_create_indexes",
			Query: `
				-- User indexes
				CREATE INDEX IF NOT EXISTS idx_users_phone_number ON users(phone_number);
				CREATE INDEX IF NOT EXISTS idx_users_user_type ON users(user_type);
				CREATE INDEX IF NOT EXISTS idx_users_is_active ON users(is_active);
				CREATE INDEX IF NOT EXISTS idx_users_last_seen ON users(last_seen);

				-- Video indexes
				CREATE INDEX IF NOT EXISTS idx_videos_user_id ON videos(user_id);
				CREATE INDEX IF NOT EXISTS idx_videos_created_at ON videos(created_at DESC);
				CREATE INDEX IF NOT EXISTS idx_videos_is_active ON videos(is_active);
				CREATE INDEX IF NOT EXISTS idx_videos_is_featured ON videos(is_featured);
				CREATE INDEX IF NOT EXISTS idx_videos_likes_count ON videos(likes_count DESC);
				CREATE INDEX IF NOT EXISTS idx_videos_views_count ON videos(views_count DESC);
				CREATE INDEX IF NOT EXISTS idx_videos_tags ON videos USING GIN(tags);

				-- Comment indexes
				CREATE INDEX IF NOT EXISTS idx_comments_video_id ON comments(video_id);
				CREATE INDEX IF NOT EXISTS idx_comments_author_id ON comments(author_id);
				CREATE INDEX IF NOT EXISTS idx_comments_created_at ON comments(created_at DESC);

				-- Like indexes
				CREATE INDEX IF NOT EXISTS idx_video_likes_video_id ON video_likes(video_id);
				CREATE INDEX IF NOT EXISTS idx_video_likes_user_id ON video_likes(user_id);
				CREATE INDEX IF NOT EXISTS idx_comment_likes_comment_id ON comment_likes(comment_id);

				-- Follow indexes
				CREATE INDEX IF NOT EXISTS idx_user_follows_follower_id ON user_follows(follower_id);
				CREATE INDEX IF NOT EXISTS idx_user_follows_following_id ON user_follows(following_id);

				-- Wallet indexes
				CREATE INDEX IF NOT EXISTS idx_wallets_user_id ON wallets(user_id);
				CREATE INDEX IF NOT EXISTS idx_wallet_transactions_user_id ON wallet_transactions(user_id);
				CREATE INDEX IF NOT EXISTS idx_wallet_transactions_type ON wallet_transactions(type);
				CREATE INDEX IF NOT EXISTS idx_coin_purchase_requests_user_id ON coin_purchase_requests(user_id);
				CREATE INDEX IF NOT EXISTS idx_coin_purchase_requests_status ON coin_purchase_requests(status);
			`,
		},
		{
			Version: "004_create_triggers",
			Query: `
				-- Function to update user video count
				CREATE OR REPLACE FUNCTION update_user_video_count()
				RETURNS TRIGGER AS $$
				BEGIN
					IF TG_OP = 'INSERT' THEN
						UPDATE users 
						SET videos_count = videos_count + 1, updated_at = CURRENT_TIMESTAMP
						WHERE uid = NEW.user_id;
						RETURN NEW;
					ELSIF TG_OP = 'DELETE' THEN
						UPDATE users 
						SET videos_count = GREATEST(0, videos_count - 1), updated_at = CURRENT_TIMESTAMP
						WHERE uid = OLD.user_id;
						RETURN OLD;
					END IF;
					RETURN NULL;
				END;
				$$ LANGUAGE plpgsql;

				-- Function to update video like count
				CREATE OR REPLACE FUNCTION update_video_like_count()
				RETURNS TRIGGER AS $$
				BEGIN
					IF TG_OP = 'INSERT' THEN
						UPDATE videos 
						SET likes_count = likes_count + 1, updated_at = CURRENT_TIMESTAMP
						WHERE id = NEW.video_id;
						RETURN NEW;
					ELSIF TG_OP = 'DELETE' THEN
						UPDATE videos 
						SET likes_count = GREATEST(0, likes_count - 1), updated_at = CURRENT_TIMESTAMP
						WHERE id = OLD.video_id;
						RETURN OLD;
					END IF;
					RETURN NULL;
				END;
				$$ LANGUAGE plpgsql;

				-- Function to update comment count
				CREATE OR REPLACE FUNCTION update_video_comment_count()
				RETURNS TRIGGER AS $$
				BEGIN
					IF TG_OP = 'INSERT' THEN
						UPDATE videos 
						SET comments_count = comments_count + 1, updated_at = CURRENT_TIMESTAMP
						WHERE id = NEW.video_id;
						RETURN NEW;
					ELSIF TG_OP = 'DELETE' THEN
						UPDATE videos 
						SET comments_count = GREATEST(0, comments_count - 1), updated_at = CURRENT_TIMESTAMP
						WHERE id = OLD.video_id;
						RETURN OLD;
					END IF;
					RETURN NULL;
				END;
				$$ LANGUAGE plpgsql;

				-- Function to update follow counts
				CREATE OR REPLACE FUNCTION update_user_follow_counts()
				RETURNS TRIGGER AS $$
				BEGIN
					IF TG_OP = 'INSERT' THEN
						UPDATE users 
						SET following_count = following_count + 1, updated_at = CURRENT_TIMESTAMP
						WHERE uid = NEW.follower_id;
						
						UPDATE users 
						SET followers_count = followers_count + 1, updated_at = CURRENT_TIMESTAMP
						WHERE uid = NEW.following_id;
						
						RETURN NEW;
					ELSIF TG_OP = 'DELETE' THEN
						UPDATE users 
						SET following_count = GREATEST(0, following_count - 1), updated_at = CURRENT_TIMESTAMP
						WHERE uid = OLD.follower_id;
						
						UPDATE users 
						SET followers_count = GREATEST(0, followers_count - 1), updated_at = CURRENT_TIMESTAMP
						WHERE uid = OLD.following_id;
						
						RETURN OLD;
					END IF;
					RETURN NULL;
				END;
				$$ LANGUAGE plpgsql;

				-- Create triggers
				DROP TRIGGER IF EXISTS trigger_update_user_video_count ON videos;
				CREATE TRIGGER trigger_update_user_video_count
					AFTER INSERT OR DELETE ON videos
					FOR EACH ROW EXECUTE FUNCTION update_user_video_count();

				DROP TRIGGER IF EXISTS trigger_update_video_like_count ON video_likes;
				CREATE TRIGGER trigger_update_video_like_count
					AFTER INSERT OR DELETE ON video_likes
					FOR EACH ROW EXECUTE FUNCTION update_video_like_count();

				DROP TRIGGER IF EXISTS trigger_update_video_comment_count ON comments;
				CREATE TRIGGER trigger_update_video_comment_count
					AFTER INSERT OR DELETE ON comments
					FOR EACH ROW EXECUTE FUNCTION update_video_comment_count();

				DROP TRIGGER IF EXISTS trigger_update_user_follow_counts ON user_follows;
				CREATE TRIGGER trigger_update_user_follow_counts
					AFTER INSERT OR DELETE ON user_follows
					FOR EACH ROW EXECUTE FUNCTION update_user_follow_counts();
			`,
		},
	}

	for _, migration := range migrations {
		if err := applyMigration(db, migration); err != nil {
			return fmt.Errorf("failed to apply migration %s: %w", migration.Version, err)
		}
	}

	log.Println("✅ Video social media migrations completed successfully")
	return nil
}

type Migration struct {
	Version string
	Query   string
}

func applyMigration(db *sqlx.DB, migration Migration) error {
	// Check if migration already applied
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM migrations WHERE version = $1", migration.Version).Scan(&count)
	if err != nil {
		return err
	}

	if count > 0 {
		return nil // Migration already applied
	}

	// Apply migration
	_, err = db.Exec(migration.Query)
	if err != nil {
		return err
	}

	// Record migration
	_, err = db.Exec("INSERT INTO migrations (version) VALUES ($1)", migration.Version)
	return err
}
