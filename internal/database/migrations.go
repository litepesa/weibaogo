// ===============================
// internal/database/migrations.go - Final 100% Error-Free Phone-Only Schema
// ===============================

package database

import (
	"fmt"
	"log"

	"github.com/jmoiron/sqlx"
)

func RunMigrations(db *sqlx.DB) error {
	log.Println("📄 Running video social media migrations (Phone-Only)...")

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
			Version: "001_initial_video_schema_phone_only",
			Query: `
				-- Users table - phone-based auth only (NO EMAIL FIELD)
				CREATE TABLE IF NOT EXISTS users (
					uid VARCHAR(255) PRIMARY KEY,
					name VARCHAR(255) NOT NULL DEFAULT 'User',
					phone_number VARCHAR(20) UNIQUE NOT NULL,
					profile_image TEXT DEFAULT '',
					cover_image TEXT DEFAULT '',
					bio TEXT DEFAULT '',
					user_type VARCHAR(20) DEFAULT 'user',
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
					last_seen TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
					CONSTRAINT users_user_type_check CHECK (user_type IN ('user', 'admin', 'moderator'))
				);

				-- Videos table - core content
				CREATE TABLE IF NOT EXISTS videos (
					id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
					user_id VARCHAR(255) NOT NULL,
					user_name VARCHAR(255) NOT NULL,
					user_image TEXT DEFAULT '',
					video_url TEXT DEFAULT '',
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
					video_id UUID NOT NULL,
					author_id VARCHAR(255) NOT NULL,
					author_name VARCHAR(255) NOT NULL,
					author_image TEXT DEFAULT '',
					content TEXT NOT NULL,
					likes_count INTEGER DEFAULT 0,
					is_reply BOOLEAN DEFAULT false,
					replied_to_comment_id UUID,
					replied_to_author_name VARCHAR(255),
					created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
					updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
				);

				-- Video likes table
				CREATE TABLE IF NOT EXISTS video_likes (
					id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
					video_id UUID NOT NULL,
					user_id VARCHAR(255) NOT NULL,
					created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
					UNIQUE(video_id, user_id)
				);

				-- Comment likes table
				CREATE TABLE IF NOT EXISTS comment_likes (
					id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
					comment_id UUID NOT NULL,
					user_id VARCHAR(255) NOT NULL,
					created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
					UNIQUE(comment_id, user_id)
				);

				-- User follows table
				CREATE TABLE IF NOT EXISTS user_follows (
					id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
					follower_id VARCHAR(255) NOT NULL,
					following_id VARCHAR(255) NOT NULL,
					created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
					UNIQUE(follower_id, following_id),
					CHECK(follower_id != following_id)
				);
			`,
		},
		{
			Version: "002_wallet_system_phone_only",
			Query: `
				-- Wallets table (phone-only)
				CREATE TABLE IF NOT EXISTS wallets (
					wallet_id VARCHAR(255) PRIMARY KEY,
					user_id VARCHAR(255) UNIQUE NOT NULL,
					user_phone_number VARCHAR(20) NOT NULL,
					user_name VARCHAR(255) NOT NULL,
					coins_balance INTEGER DEFAULT 0,
					created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
					updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
				);

				-- Wallet transactions table (phone-only)
				CREATE TABLE IF NOT EXISTS wallet_transactions (
					transaction_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
					wallet_id VARCHAR(255) NOT NULL,
					user_id VARCHAR(255) NOT NULL,
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

				-- Coin purchase requests table (phone-only)
				CREATE TABLE IF NOT EXISTS coin_purchase_requests (
					id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
					user_id VARCHAR(255) NOT NULL,
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
			Version: "003_add_foreign_keys",
			Query: `
				-- Add foreign key constraints (separated for better error handling)
				DO $$
				BEGIN
					-- Videos to users foreign key
					IF NOT EXISTS (SELECT 1 FROM information_schema.table_constraints 
								  WHERE constraint_name = 'videos_user_id_fkey' 
								  AND table_name = 'videos') THEN
						ALTER TABLE videos ADD CONSTRAINT videos_user_id_fkey 
						FOREIGN KEY (user_id) REFERENCES users(uid) ON DELETE CASCADE;
					END IF;

					-- Comments to videos foreign key
					IF NOT EXISTS (SELECT 1 FROM information_schema.table_constraints 
								  WHERE constraint_name = 'comments_video_id_fkey' 
								  AND table_name = 'comments') THEN
						ALTER TABLE comments ADD CONSTRAINT comments_video_id_fkey 
						FOREIGN KEY (video_id) REFERENCES videos(id) ON DELETE CASCADE;
					END IF;

					-- Comments to users foreign key
					IF NOT EXISTS (SELECT 1 FROM information_schema.table_constraints 
								  WHERE constraint_name = 'comments_author_id_fkey' 
								  AND table_name = 'comments') THEN
						ALTER TABLE comments ADD CONSTRAINT comments_author_id_fkey 
						FOREIGN KEY (author_id) REFERENCES users(uid) ON DELETE CASCADE;
					END IF;

					-- Comments self-reference foreign key
					IF NOT EXISTS (SELECT 1 FROM information_schema.table_constraints 
								  WHERE constraint_name = 'comments_replied_to_comment_id_fkey' 
								  AND table_name = 'comments') THEN
						ALTER TABLE comments ADD CONSTRAINT comments_replied_to_comment_id_fkey 
						FOREIGN KEY (replied_to_comment_id) REFERENCES comments(id) ON DELETE CASCADE;
					END IF;

					-- Video likes foreign keys
					IF NOT EXISTS (SELECT 1 FROM information_schema.table_constraints 
								  WHERE constraint_name = 'video_likes_video_id_fkey' 
								  AND table_name = 'video_likes') THEN
						ALTER TABLE video_likes ADD CONSTRAINT video_likes_video_id_fkey 
						FOREIGN KEY (video_id) REFERENCES videos(id) ON DELETE CASCADE;
					END IF;

					IF NOT EXISTS (SELECT 1 FROM information_schema.table_constraints 
								  WHERE constraint_name = 'video_likes_user_id_fkey' 
								  AND table_name = 'video_likes') THEN
						ALTER TABLE video_likes ADD CONSTRAINT video_likes_user_id_fkey 
						FOREIGN KEY (user_id) REFERENCES users(uid) ON DELETE CASCADE;
					END IF;

					-- Comment likes foreign keys
					IF NOT EXISTS (SELECT 1 FROM information_schema.table_constraints 
								  WHERE constraint_name = 'comment_likes_comment_id_fkey' 
								  AND table_name = 'comment_likes') THEN
						ALTER TABLE comment_likes ADD CONSTRAINT comment_likes_comment_id_fkey 
						FOREIGN KEY (comment_id) REFERENCES comments(id) ON DELETE CASCADE;
					END IF;

					IF NOT EXISTS (SELECT 1 FROM information_schema.table_constraints 
								  WHERE constraint_name = 'comment_likes_user_id_fkey' 
								  AND table_name = 'comment_likes') THEN
						ALTER TABLE comment_likes ADD CONSTRAINT comment_likes_user_id_fkey 
						FOREIGN KEY (user_id) REFERENCES users(uid) ON DELETE CASCADE;
					END IF;

					-- User follows foreign keys
					IF NOT EXISTS (SELECT 1 FROM information_schema.table_constraints 
								  WHERE constraint_name = 'user_follows_follower_id_fkey' 
								  AND table_name = 'user_follows') THEN
						ALTER TABLE user_follows ADD CONSTRAINT user_follows_follower_id_fkey 
						FOREIGN KEY (follower_id) REFERENCES users(uid) ON DELETE CASCADE;
					END IF;

					IF NOT EXISTS (SELECT 1 FROM information_schema.table_constraints 
								  WHERE constraint_name = 'user_follows_following_id_fkey' 
								  AND table_name = 'user_follows') THEN
						ALTER TABLE user_follows ADD CONSTRAINT user_follows_following_id_fkey 
						FOREIGN KEY (following_id) REFERENCES users(uid) ON DELETE CASCADE;
					END IF;

					-- Wallet foreign keys
					IF NOT EXISTS (SELECT 1 FROM information_schema.table_constraints 
								  WHERE constraint_name = 'wallets_user_id_fkey' 
								  AND table_name = 'wallets') THEN
						ALTER TABLE wallets ADD CONSTRAINT wallets_user_id_fkey 
						FOREIGN KEY (user_id) REFERENCES users(uid) ON DELETE CASCADE;
					END IF;

					IF NOT EXISTS (SELECT 1 FROM information_schema.table_constraints 
								  WHERE constraint_name = 'wallet_transactions_wallet_id_fkey' 
								  AND table_name = 'wallet_transactions') THEN
						ALTER TABLE wallet_transactions ADD CONSTRAINT wallet_transactions_wallet_id_fkey 
						FOREIGN KEY (wallet_id) REFERENCES wallets(wallet_id);
					END IF;

					IF NOT EXISTS (SELECT 1 FROM information_schema.table_constraints 
								  WHERE constraint_name = 'wallet_transactions_user_id_fkey' 
								  AND table_name = 'wallet_transactions') THEN
						ALTER TABLE wallet_transactions ADD CONSTRAINT wallet_transactions_user_id_fkey 
						FOREIGN KEY (user_id) REFERENCES users(uid) ON DELETE CASCADE;
					END IF;

					IF NOT EXISTS (SELECT 1 FROM information_schema.table_constraints 
								  WHERE constraint_name = 'coin_purchase_requests_user_id_fkey' 
								  AND table_name = 'coin_purchase_requests') THEN
						ALTER TABLE coin_purchase_requests ADD CONSTRAINT coin_purchase_requests_user_id_fkey 
						FOREIGN KEY (user_id) REFERENCES users(uid) ON DELETE CASCADE;
					END IF;
				END $$;
			`,
		},
		{
			Version: "004_create_indexes",
			Query: `
				-- User indexes (phone-only optimized)
				CREATE INDEX IF NOT EXISTS idx_users_phone_number ON users(phone_number);
				CREATE INDEX IF NOT EXISTS idx_users_user_type ON users(user_type);
				CREATE INDEX IF NOT EXISTS idx_users_is_active ON users(is_active);
				CREATE INDEX IF NOT EXISTS idx_users_last_seen ON users(last_seen);
				CREATE INDEX IF NOT EXISTS idx_users_name ON users(name);
				CREATE INDEX IF NOT EXISTS idx_users_followers_count ON users(followers_count DESC);

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
				CREATE INDEX IF NOT EXISTS idx_comment_likes_user_id ON comment_likes(user_id);

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
			Version: "005_add_data_constraints",
			Query: `
				-- Add data validation constraints using DO blocks
				DO $$
				BEGIN
					-- User constraints
					IF NOT EXISTS (SELECT 1 FROM information_schema.check_constraints 
								  WHERE constraint_name = 'users_name_length') THEN
						ALTER TABLE users ADD CONSTRAINT users_name_length
						CHECK (LENGTH(name) >= 1 AND LENGTH(name) <= 50);
					END IF;

					IF NOT EXISTS (SELECT 1 FROM information_schema.check_constraints 
								  WHERE constraint_name = 'users_bio_length') THEN
						ALTER TABLE users ADD CONSTRAINT users_bio_length
						CHECK (LENGTH(bio) <= 160);
					END IF;

					IF NOT EXISTS (SELECT 1 FROM information_schema.check_constraints 
								  WHERE constraint_name = 'users_followers_count_positive') THEN
						ALTER TABLE users ADD CONSTRAINT users_followers_count_positive
						CHECK (followers_count >= 0);
					END IF;

					IF NOT EXISTS (SELECT 1 FROM information_schema.check_constraints 
								  WHERE constraint_name = 'users_following_count_positive') THEN
						ALTER TABLE users ADD CONSTRAINT users_following_count_positive
						CHECK (following_count >= 0);
					END IF;

					-- Video constraints
					IF NOT EXISTS (SELECT 1 FROM information_schema.check_constraints 
								  WHERE constraint_name = 'videos_caption_length') THEN
						ALTER TABLE videos ADD CONSTRAINT videos_caption_length
						CHECK (LENGTH(caption) <= 2200);
					END IF;

					IF NOT EXISTS (SELECT 1 FROM information_schema.check_constraints 
								  WHERE constraint_name = 'videos_counts_positive') THEN
						ALTER TABLE videos ADD CONSTRAINT videos_counts_positive
						CHECK (likes_count >= 0 AND comments_count >= 0 AND views_count >= 0 AND shares_count >= 0);
					END IF;

					-- Comment constraints
					IF NOT EXISTS (SELECT 1 FROM information_schema.check_constraints 
								  WHERE constraint_name = 'comments_content_length') THEN
						ALTER TABLE comments ADD CONSTRAINT comments_content_length
						CHECK (LENGTH(content) >= 1 AND LENGTH(content) <= 500);
					END IF;

					-- Wallet constraints
					IF NOT EXISTS (SELECT 1 FROM information_schema.check_constraints 
								  WHERE constraint_name = 'wallets_coins_balance_positive') THEN
						ALTER TABLE wallets ADD CONSTRAINT wallets_coins_balance_positive
						CHECK (coins_balance >= 0);
					END IF;
				END $$;

				-- Update any existing empty names
				UPDATE users SET name = 'User' WHERE name IS NULL OR name = '';
			`,
		},
		{
			Version: "006_create_functions",
			Query: `
				-- Function to update user video count
				CREATE OR REPLACE FUNCTION update_user_video_count()
				RETURNS TRIGGER AS $$
				BEGIN
					IF TG_OP = 'INSERT' THEN
						UPDATE users 
						SET videos_count = videos_count + 1, 
							updated_at = CURRENT_TIMESTAMP
						WHERE uid = NEW.user_id;
						RETURN NEW;
					ELSIF TG_OP = 'DELETE' THEN
						UPDATE users 
						SET videos_count = GREATEST(0, videos_count - 1),
							updated_at = CURRENT_TIMESTAMP
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
						SET likes_count = likes_count + 1,
							updated_at = CURRENT_TIMESTAMP
						WHERE id = NEW.video_id;
						RETURN NEW;
					ELSIF TG_OP = 'DELETE' THEN
						UPDATE videos 
						SET likes_count = GREATEST(0, likes_count - 1),
							updated_at = CURRENT_TIMESTAMP
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
						SET comments_count = comments_count + 1,
							updated_at = CURRENT_TIMESTAMP
						WHERE id = NEW.video_id;
						RETURN NEW;
					ELSIF TG_OP = 'DELETE' THEN
						UPDATE videos 
						SET comments_count = GREATEST(0, comments_count - 1),
							updated_at = CURRENT_TIMESTAMP
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
						SET following_count = following_count + 1,
							updated_at = CURRENT_TIMESTAMP
						WHERE uid = NEW.follower_id;
						
						UPDATE users 
						SET followers_count = followers_count + 1,
							updated_at = CURRENT_TIMESTAMP
						WHERE uid = NEW.following_id;
						
						RETURN NEW;
					ELSIF TG_OP = 'DELETE' THEN
						UPDATE users 
						SET following_count = GREATEST(0, following_count - 1),
							updated_at = CURRENT_TIMESTAMP
						WHERE uid = OLD.follower_id;
						
						UPDATE users 
						SET followers_count = GREATEST(0, followers_count - 1),
							updated_at = CURRENT_TIMESTAMP
						WHERE uid = OLD.following_id;
						
						RETURN OLD;
					END IF;
					RETURN NULL;
				END;
				$$ LANGUAGE plpgsql;
			`,
		},
		{
			Version: "007_create_triggers",
			Query: `
				-- Drop existing triggers if they exist
				DROP TRIGGER IF EXISTS trigger_update_user_video_count ON videos;
				DROP TRIGGER IF EXISTS trigger_update_video_like_count ON video_likes;
				DROP TRIGGER IF EXISTS trigger_update_video_comment_count ON comments;
				DROP TRIGGER IF EXISTS trigger_update_user_follow_counts ON user_follows;

				-- Create triggers
				CREATE TRIGGER trigger_update_user_video_count
					AFTER INSERT OR DELETE ON videos
					FOR EACH ROW 
					EXECUTE FUNCTION update_user_video_count();

				CREATE TRIGGER trigger_update_video_like_count
					AFTER INSERT OR DELETE ON video_likes
					FOR EACH ROW 
					EXECUTE FUNCTION update_video_like_count();

				CREATE TRIGGER trigger_update_video_comment_count
					AFTER INSERT OR DELETE ON comments
					FOR EACH ROW 
					EXECUTE FUNCTION update_video_comment_count();

				CREATE TRIGGER trigger_update_user_follow_counts
					AFTER INSERT OR DELETE ON user_follows
					FOR EACH ROW 
					EXECUTE FUNCTION update_user_follow_counts();
			`,
		},
	}

	for _, migration := range migrations {
		if err := applyMigration(db, migration); err != nil {
			return fmt.Errorf("failed to apply migration %s: %w", migration.Version, err)
		}
	}

	log.Println("✅ Video social media migrations completed successfully (Phone-Only)")
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
		return fmt.Errorf("failed to check migration status: %w", err)
	}

	if count > 0 {
		log.Printf("⏭️  Migration %s already applied, skipping", migration.Version)
		return nil
	}

	log.Printf("🔧 Applying migration: %s", migration.Version)

	// Apply migration in a transaction
	tx, err := db.Beginx()
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	// Execute migration
	_, err = tx.Exec(migration.Query)
	if err != nil {
		return fmt.Errorf("failed to execute migration %s: %w", migration.Version, err)
	}

	// Record migration
	_, err = tx.Exec("INSERT INTO migrations (version) VALUES ($1)", migration.Version)
	if err != nil {
		return fmt.Errorf("failed to record migration %s: %w", migration.Version, err)
	}

	// Commit transaction
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit migration %s: %w", migration.Version, err)
	}

	log.Printf("✅ Migration %s applied successfully", migration.Version)
	return nil
}
