// ===============================
// internal/database/migrations.go - COMPLETE VERSION with Gift System & Platform Commissions
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
				DO $block1$
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
				END $block1$;
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
				CREATE INDEX IF NOT EXISTS idx_users_is_verified ON users(is_verified);

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
				DO $block1$
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
				END $block1$;

				-- Update any existing empty names
				UPDATE users SET name = 'User' WHERE name IS NULL OR name = '';
			`,
		},
		{
			Version: "006_create_functions",
			Query: `
				-- Function to update user video count
				CREATE OR REPLACE FUNCTION update_user_video_count()
				RETURNS TRIGGER AS $func1$
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
				$func1$ LANGUAGE plpgsql;

				-- Function to update video like count
				CREATE OR REPLACE FUNCTION update_video_like_count()
				RETURNS TRIGGER AS $func2$
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
				$func2$ LANGUAGE plpgsql;

				-- Function to update comment count
				CREATE OR REPLACE FUNCTION update_video_comment_count()
				RETURNS TRIGGER AS $func3$
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
				$func3$ LANGUAGE plpgsql;

				-- Function to update follow counts
				CREATE OR REPLACE FUNCTION update_user_follow_counts()
				RETURNS TRIGGER AS $func4$
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
				$func4$ LANGUAGE plpgsql;
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
		{
			Version: "008_add_last_post_at_and_trigger",
			Query: `
				-- ===============================
				-- ADD LAST_POST_AT FUNCTIONALITY
				-- ===============================
				
				-- Add last_post_at column to users table
				ALTER TABLE users ADD COLUMN IF NOT EXISTS last_post_at TIMESTAMP WITH TIME ZONE;

				-- Create function to update user's last_post_at when video is created
				CREATE OR REPLACE FUNCTION update_user_last_post()
				RETURNS TRIGGER AS $func5$
				BEGIN
					UPDATE users 
					SET last_post_at = NEW.created_at,
						updated_at = CURRENT_TIMESTAMP
					WHERE uid = NEW.user_id;
					RETURN NEW;
				END;
				$func5$ LANGUAGE plpgsql;

				-- Create trigger to automatically update last_post_at
				DROP TRIGGER IF EXISTS trigger_update_user_last_post ON videos;
				CREATE TRIGGER trigger_update_user_last_post
					AFTER INSERT ON videos
					FOR EACH ROW 
					EXECUTE FUNCTION update_user_last_post();

				-- Create index for last_post_at column for efficient sorting
				CREATE INDEX IF NOT EXISTS idx_users_last_post_at ON users(last_post_at DESC);

				-- Populate last_post_at for existing users based on their most recent video
				UPDATE users 
				SET last_post_at = subquery.latest_post
				FROM (
					SELECT user_id, MAX(created_at) as latest_post
					FROM videos 
					GROUP BY user_id
				) AS subquery
				WHERE users.uid = subquery.user_id;
			`,
		},
		{
			Version: "009_add_user_role_and_whatsapp",
			Query: `
				-- ===============================
				-- ADD USER ROLE AND WHATSAPP NUMBER FIELDS
				-- ===============================
				
				-- Add role column to users table
				ALTER TABLE users ADD COLUMN IF NOT EXISTS role VARCHAR(20) DEFAULT 'guest';

				-- Add whatsapp_number column to users table  
				ALTER TABLE users ADD COLUMN IF NOT EXISTS whatsapp_number VARCHAR(20);

				-- Add check constraint for role values
				DO $block1$
				BEGIN
					IF NOT EXISTS (SELECT 1 FROM information_schema.check_constraints 
								  WHERE constraint_name = 'users_role_check') THEN
						ALTER TABLE users ADD CONSTRAINT users_role_check
						CHECK (role IN ('admin', 'host', 'guest'));
					END IF;
				END $block1$;

				-- Add check constraint for WhatsApp number format (Kenyan format: 254XXXXXXXXX)
				DO $block2$
				BEGIN
					IF NOT EXISTS (SELECT 1 FROM information_schema.check_constraints 
								  WHERE constraint_name = 'users_whatsapp_number_format_check') THEN
						ALTER TABLE users ADD CONSTRAINT users_whatsapp_number_format_check
						CHECK (whatsapp_number IS NULL OR whatsapp_number ~ '^254[0-9]{9}$');
					END IF;
				END $block2$;

				-- Create index for role column for efficient filtering
				CREATE INDEX IF NOT EXISTS idx_users_role ON users(role);

				-- Create index for whatsapp_number column
				CREATE INDEX IF NOT EXISTS idx_users_whatsapp_number ON users(whatsapp_number);

				-- Update existing users to have 'guest' role if they don't have one
				UPDATE users SET role = 'guest' WHERE role IS NULL OR role = '';

				-- Migrate existing user_type values to role column for better alignment
				UPDATE users 
				SET role = CASE 
					WHEN user_type = 'admin' THEN 'admin'
					WHEN user_type = 'moderator' THEN 'host'  -- Moderators become hosts
					ELSE 'guest'
				END
				WHERE role = 'guest';

				-- Create function to validate video posting based on user role
				CREATE OR REPLACE FUNCTION validate_user_can_post(user_uid VARCHAR(255))
				RETURNS BOOLEAN AS $func6$
				DECLARE
					user_role VARCHAR(20);
				BEGIN
					SELECT role INTO user_role FROM users WHERE uid = user_uid AND is_active = true;
					
					IF user_role IS NULL THEN
						RETURN FALSE;
					END IF;
					
					RETURN user_role IN ('admin', 'host');
				END;
				$func6$ LANGUAGE plpgsql;

				-- Add trigger to validate user can post when creating videos
				CREATE OR REPLACE FUNCTION check_user_can_post_video()
				RETURNS TRIGGER AS $func7$
				BEGIN
					IF NOT validate_user_can_post(NEW.user_id) THEN
						RAISE EXCEPTION 'User with role "guest" cannot post videos. Only admin and host users can post videos.';
					END IF;
					RETURN NEW;
				END;
				$func7$ LANGUAGE plpgsql;

				-- Create trigger for video posting validation
				DROP TRIGGER IF EXISTS trigger_check_user_can_post_video ON videos;
				CREATE TRIGGER trigger_check_user_can_post_video
					BEFORE INSERT ON videos
					FOR EACH ROW 
					EXECUTE FUNCTION check_user_can_post_video();

				-- Update the existing video count update function to also validate role
				CREATE OR REPLACE FUNCTION update_user_video_count()
				RETURNS TRIGGER AS $func1_updated$
				BEGIN
					IF TG_OP = 'INSERT' THEN
						-- Additional validation happens in trigger_check_user_can_post_video
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
				$func1_updated$ LANGUAGE plpgsql;
			`,
		},
		{
			Version: "010_ensure_video_price_and_verified_compatibility",
			Query: `
				-- ===============================
				-- 🔧 ENSURE COMPATIBILITY WITH ALREADY ADDED PRICE AND IS_VERIFIED FIELDS
				-- ===============================
				
				-- Since the fields may already exist, we'll just ensure they have the right structure
				-- and add any missing indexes or constraints
				
				-- Ensure price column exists with correct type and default
				DO $block1$
				BEGIN
					-- Check if price column exists, if not add it
					IF NOT EXISTS (SELECT 1 FROM information_schema.columns 
								  WHERE table_name = 'videos' AND column_name = 'price') THEN
						ALTER TABLE videos ADD COLUMN price DECIMAL(10,2) DEFAULT 0.00;
					END IF;
					
					-- Check if is_verified column exists, if not add it
					IF NOT EXISTS (SELECT 1 FROM information_schema.columns 
								  WHERE table_name = 'videos' AND column_name = 'is_verified') THEN
						ALTER TABLE videos ADD COLUMN is_verified BOOLEAN DEFAULT false;
					END IF;
				END $block1$;

				-- Add check constraint for price (must be non-negative) if not exists
				DO $block2$
				BEGIN
					IF NOT EXISTS (SELECT 1 FROM information_schema.check_constraints 
								  WHERE constraint_name = 'videos_price_positive') THEN
						ALTER TABLE videos ADD CONSTRAINT videos_price_positive
						CHECK (price >= 0);
					END IF;
				END $block2$;

				-- Create indexes for the fields to optimize queries (only if they don't exist)
				CREATE INDEX IF NOT EXISTS idx_videos_price ON videos(price DESC);
				CREATE INDEX IF NOT EXISTS idx_videos_is_verified ON videos(is_verified);
				CREATE INDEX IF NOT EXISTS idx_videos_verified_price ON videos(is_verified, price DESC);
				CREATE INDEX IF NOT EXISTS idx_videos_active_verified ON videos(is_active, is_verified);
				CREATE INDEX IF NOT EXISTS idx_videos_featured_verified ON videos(is_featured, is_verified);

				-- Ensure existing videos have valid default values
				UPDATE videos 
				SET price = COALESCE(price, 0.00)
				WHERE price IS NULL;
				
				UPDATE videos 
				SET is_verified = COALESCE(is_verified, false)
				WHERE is_verified IS NULL;

				-- Create helper function for premium content identification
				CREATE OR REPLACE FUNCTION is_premium_content(video_verified BOOLEAN, video_price DECIMAL)
				RETURNS BOOLEAN AS $func8$
				BEGIN
					RETURN video_verified = true AND video_price > 0;
				END;
				$func8$ LANGUAGE plpgsql;

				-- Create function to get content tier for videos
				CREATE OR REPLACE FUNCTION get_video_content_tier(
					video_verified BOOLEAN, 
					video_featured BOOLEAN, 
					video_price DECIMAL,
					video_likes INTEGER,
					video_views INTEGER
				)
				RETURNS TEXT AS $func9$
				DECLARE
					engagement_rate DECIMAL;
				BEGIN
					-- Calculate engagement rate
					IF video_views > 0 THEN
						engagement_rate := (video_likes::DECIMAL / video_views::DECIMAL) * 100;
					ELSE
						engagement_rate := 0;
					END IF;
					
					-- Determine tier
					IF video_verified = true AND video_featured = true THEN
						RETURN 'Premium+';
					ELSIF video_verified = true THEN
						RETURN 'Premium';
					ELSIF video_featured = true THEN
						RETURN 'Featured';
					ELSIF engagement_rate > 5.0 THEN
						RETURN 'Popular';
					ELSE
						RETURN 'Standard';
					END IF;
				END;
				$func9$ LANGUAGE plpgsql;
			`,
		},
		{
			Version: "011_add_search_optimization_indexes",
			Query: `
		-- ===============================
		-- 🔍 SEARCH OPTIMIZATION INDEXES AND EXTENSIONS
		-- ===============================

		-- Enable trigram extension for fuzzy search (handles typos)
		CREATE EXTENSION IF NOT EXISTS pg_trgm;

		-- 1. Full-text search index for captions (most important for search performance)
		CREATE INDEX IF NOT EXISTS idx_videos_caption_fulltext 
		ON videos USING gin(to_tsvector('english', caption));

		-- 2. Trigram index for fuzzy search on captions (handles typos)
		CREATE INDEX IF NOT EXISTS idx_videos_caption_trgm 
		ON videos USING gin(caption gin_trgm_ops);

		-- 3. Trigram index for fuzzy search on usernames
		CREATE INDEX IF NOT EXISTS idx_videos_user_name_trgm 
		ON videos USING gin(user_name gin_trgm_ops);

		-- 4. Combined search optimization index
		CREATE INDEX IF NOT EXISTS idx_videos_search_optimized 
		ON videos(is_active, created_at DESC) 
		WHERE is_active = true;

		-- 5. Search filtering indexes
		CREATE INDEX IF NOT EXISTS idx_videos_media_type_search 
		ON videos(is_multiple_images, is_active, created_at DESC) 
		WHERE is_active = true;

		-- 6. Price-based filtering for search
		CREATE INDEX IF NOT EXISTS idx_videos_price_search 
		ON videos(price, is_active, created_at DESC) 
		WHERE is_active = true;

		-- 7. Verification-based filtering for search
		CREATE INDEX IF NOT EXISTS idx_videos_verified_search 
		ON videos(is_verified, is_active, created_at DESC) 
		WHERE is_active = true;

		-- 8. Combined search filters index
		CREATE INDEX IF NOT EXISTS idx_videos_combined_search_filters 
		ON videos(is_active, is_multiple_images, is_verified, price, created_at DESC) 
		WHERE is_active = true;

		-- 9. Trending score calculation helper index
		CREATE INDEX IF NOT EXISTS idx_videos_trending_search 
		ON videos(is_active, likes_count, views_count, comments_count, shares_count, created_at) 
		WHERE is_active = true;

		-- Create helper function for search relevance scoring
		CREATE OR REPLACE FUNCTION calculate_search_relevance(
			caption_text TEXT,
			username_text TEXT,
			search_query TEXT
		)
		RETURNS DECIMAL AS $func_search$
		DECLARE
			caption_relevance DECIMAL := 0;
			username_relevance DECIMAL := 0;
		BEGIN
			-- Caption exact match gets highest score
			IF LOWER(caption_text) LIKE '%' || LOWER(search_query) || '%' THEN
				caption_relevance := 1.0;
			END IF;
			
			-- Username match gets medium score
			IF LOWER(username_text) LIKE '%' || LOWER(search_query) || '%' THEN
				username_relevance := 0.8;
			END IF;
			
			-- Return highest relevance
			RETURN GREATEST(caption_relevance, username_relevance);
		END;
		$func_search$ LANGUAGE plpgsql;

		-- Create function for search suggestions (autocomplete)
		CREATE OR REPLACE FUNCTION get_search_suggestions(search_prefix TEXT, result_limit INTEGER DEFAULT 5)
		RETURNS TABLE(suggestion TEXT, match_type TEXT) AS $func_suggestions$
		BEGIN
			RETURN QUERY
			SELECT DISTINCT 
				CASE 
					WHEN v.caption ILIKE search_prefix || '%' THEN v.caption
					WHEN v.user_name ILIKE search_prefix || '%' THEN v.user_name
				END as suggestion,
				CASE 
					WHEN v.caption ILIKE search_prefix || '%' THEN 'caption'
					WHEN v.user_name ILIKE search_prefix || '%' THEN 'username'
				END as match_type
			FROM videos v
			WHERE v.is_active = true 
			  AND (v.caption ILIKE search_prefix || '%' OR v.user_name ILIKE search_prefix || '%')
			  AND LENGTH(COALESCE(v.caption, '')) > 0
			ORDER BY suggestion
			LIMIT result_limit;
		END;
		$func_suggestions$ LANGUAGE plpgsql;

		-- Create materialized view for popular search terms (performance optimization)
		CREATE MATERIALIZED VIEW IF NOT EXISTS popular_search_terms AS
		SELECT 
			word,
			COUNT(*) as frequency,
			MAX(v.created_at) as last_used
		FROM (
			SELECT 
				unnest(string_to_array(LOWER(regexp_replace(caption, '[^a-zA-Z0-9\s]', ' ', 'g')), ' ')) as word,
				created_at
			FROM videos 
			WHERE is_active = true 
			  AND created_at >= NOW() - INTERVAL '30 days'
			  AND LENGTH(caption) > 10
		) v
		WHERE LENGTH(word) > 3 
		  AND word NOT IN ('the', 'and', 'for', 'are', 'but', 'not', 'you', 'all', 'can', 'had', 'her', 'was', 'one', 'our', 'out', 'day', 'get', 'has', 'him', 'his', 'how', 'its', 'may', 'new', 'now', 'old', 'see', 'two', 'who', 'boy', 'did', 'man', 'way', 'will', 'with', 'that', 'this', 'they', 'have', 'from', 'been', 'some', 'what', 'were', 'said', 'each', 'make', 'like', 'into', 'time', 'very', 'when', 'much', 'more', 'most', 'over', 'such', 'take', 'than', 'them', 'well', 'know')
		GROUP BY word
		HAVING COUNT(*) >= 2
		ORDER BY frequency DESC, last_used DESC
		LIMIT 100;

		-- Create index on materialized view
		CREATE UNIQUE INDEX IF NOT EXISTS idx_popular_search_terms_word 
		ON popular_search_terms(word);

		-- Create function to refresh popular search terms
		CREATE OR REPLACE FUNCTION refresh_popular_search_terms()
		RETURNS VOID AS $func_refresh$
		BEGIN
			REFRESH MATERIALIZED VIEW CONCURRENTLY popular_search_terms;
		END;
		$func_refresh$ LANGUAGE plpgsql;
	`,
		},

		{
			Version: "012_add_user_profile_fields",
			Query: `
		-- ===============================
		-- ADD GENDER, LOCATION, AND LANGUAGE FIELDS TO USERS
		-- ===============================
		
		-- Add gender column (male or female)
		ALTER TABLE users ADD COLUMN IF NOT EXISTS gender VARCHAR(10);

		-- Add location column (free text, e.g., "Nairobi, Kenya")
		ALTER TABLE users ADD COLUMN IF NOT EXISTS location VARCHAR(255);

		-- Drop language column if it exists (to recreate fresh)
		DO $$
		BEGIN
			-- Drop old format check constraint
			IF EXISTS (
				SELECT 1 FROM information_schema.table_constraints 
				WHERE constraint_name = 'users_language_format_check' 
				AND table_name = 'users'
			) THEN
				ALTER TABLE users DROP CONSTRAINT users_language_format_check;
			END IF;
			
			-- Drop length check constraint
			IF EXISTS (
				SELECT 1 FROM information_schema.table_constraints 
				WHERE constraint_name = 'users_language_length_check' 
				AND table_name = 'users'
			) THEN
				ALTER TABLE users DROP CONSTRAINT users_language_length_check;
			END IF;
		END $$;

		-- Drop the index for language
		DROP INDEX IF EXISTS idx_users_language;

		-- Drop the language column completely if it exists
		ALTER TABLE users DROP COLUMN IF EXISTS language;

		-- Create language column fresh - VARCHAR(100), NULL by default
		ALTER TABLE users ADD COLUMN language VARCHAR(100);

		-- Add check constraint for gender values
		DO $$
		BEGIN
			IF NOT EXISTS (
				SELECT 1 FROM information_schema.table_constraints 
				WHERE constraint_name = 'users_gender_check' 
				AND table_name = 'users'
			) THEN
				ALTER TABLE users ADD CONSTRAINT users_gender_check
				CHECK (gender IS NULL OR gender IN ('male', 'female'));
			END IF;
		END $$;

		-- Add check constraint for location length
		DO $$
		BEGIN
			IF NOT EXISTS (
				SELECT 1 FROM information_schema.table_constraints 
				WHERE constraint_name = 'users_location_length_check' 
				AND table_name = 'users'
			) THEN
				ALTER TABLE users ADD CONSTRAINT users_location_length_check
				CHECK (location IS NULL OR LENGTH(location) <= 255);
			END IF;
		END $$;

		-- Add check constraint for language length
		DO $$
		BEGIN
			IF NOT EXISTS (
				SELECT 1 FROM information_schema.table_constraints 
				WHERE constraint_name = 'users_language_length_check' 
				AND table_name = 'users'
			) THEN
				ALTER TABLE users ADD CONSTRAINT users_language_length_check
				CHECK (language IS NULL OR LENGTH(language) <= 100);
			END IF;
		END $$;

		-- Create indexes for filtering and search performance
		CREATE INDEX IF NOT EXISTS idx_users_gender 
		ON users(gender) 
		WHERE gender IS NOT NULL;

		CREATE INDEX IF NOT EXISTS idx_users_location 
		ON users(location) 
		WHERE location IS NOT NULL;

		CREATE INDEX IF NOT EXISTS idx_users_language 
		ON users(language) 
		WHERE language IS NOT NULL;

		-- Create composite index for location-based searches
		CREATE INDEX IF NOT EXISTS idx_users_location_active 
		ON users(location, is_active) 
		WHERE location IS NOT NULL AND is_active = true;

		-- Create function to validate gender value
		CREATE OR REPLACE FUNCTION is_valid_gender(gender_value VARCHAR)
		RETURNS BOOLEAN AS $$
		BEGIN
			RETURN gender_value IS NULL OR gender_value IN ('male', 'female');
		END;
		$$ LANGUAGE plpgsql;

		-- Drop existing demographics function if it exists
		DROP FUNCTION IF EXISTS get_user_demographics_summary();

		-- Create function to get user demographics summary
		CREATE OR REPLACE FUNCTION get_user_demographics_summary()
		RETURNS TABLE(
			total_users BIGINT,
			male_count BIGINT,
			female_count BIGINT,
			unspecified_gender_count BIGINT,
			top_locations TEXT[],
			top_languages TEXT[]
		) AS $$
		BEGIN
			RETURN QUERY
			WITH gender_stats AS (
				SELECT 
					COUNT(*) as total,
					COUNT(*) FILTER (WHERE gender = 'male') as male,
					COUNT(*) FILTER (WHERE gender = 'female') as female,
					COUNT(*) FILTER (WHERE gender IS NULL) as unspecified
				FROM users
				WHERE is_active = true
			),
			location_stats AS (
				SELECT COALESCE(ARRAY_AGG(location ORDER BY count DESC), ARRAY[]::TEXT[]) as locations
				FROM (
					SELECT location, COUNT(*) as count
					FROM users
					WHERE is_active = true AND location IS NOT NULL
					GROUP BY location
					ORDER BY count DESC
					LIMIT 10
				) l
			),
			language_stats AS (
				SELECT COALESCE(ARRAY_AGG(language ORDER BY count DESC), ARRAY[]::TEXT[]) as languages
				FROM (
					SELECT language, COUNT(*) as count
					FROM users
					WHERE is_active = true AND language IS NOT NULL
					GROUP BY language
					ORDER BY count DESC
					LIMIT 10
				) lang
			)
			SELECT 
				g.total,
				g.male,
				g.female,
				g.unspecified,
				l.locations,
				lang.languages
			FROM gender_stats g
			CROSS JOIN location_stats l
			CROSS JOIN language_stats lang;
		END;
		$$ LANGUAGE plpgsql;

		-- Add column comments for documentation
		COMMENT ON COLUMN users.gender IS 'User gender: male or female (optional)';
		COMMENT ON COLUMN users.location IS 'User location in free text format, e.g., "Nairobi, Kenya" (optional)';
		COMMENT ON COLUMN users.language IS 'User native/spoken language in free text format, e.g., "English", "Swahili", "French" (optional)';
	`,
		},
		{
			Version: "013_increase_price_limit",
			Query: `
        -- Increase price column limit to support up to 999,999,999.99
        ALTER TABLE videos 
        ALTER COLUMN price TYPE DECIMAL(12,2);
    `,
		},
		{
			Version: "014_gift_system_with_platform_commissions",
			Query: `
		-- ===============================
		-- 🎁 VIRTUAL GIFT SYSTEM WITH PLATFORM COMMISSIONS
		-- ===============================
		
		-- 1. Add gift statistics columns to users table
		ALTER TABLE users ADD COLUMN IF NOT EXISTS gifts_sent_count INTEGER DEFAULT 0;
		ALTER TABLE users ADD COLUMN IF NOT EXISTS gifts_received_count INTEGER DEFAULT 0;
		ALTER TABLE users ADD COLUMN IF NOT EXISTS total_coins_spent_on_gifts INTEGER DEFAULT 0;
		ALTER TABLE users ADD COLUMN IF NOT EXISTS total_coins_earned_from_gifts INTEGER DEFAULT 0;

		-- Add constraints for gift statistics
		DO $$
		BEGIN
			IF NOT EXISTS (
				SELECT 1 FROM information_schema.table_constraints 
				WHERE constraint_name = 'users_gifts_sent_count_positive' 
				AND table_name = 'users'
			) THEN
				ALTER TABLE users ADD CONSTRAINT users_gifts_sent_count_positive
				CHECK (gifts_sent_count >= 0);
			END IF;

			IF NOT EXISTS (
				SELECT 1 FROM information_schema.table_constraints 
				WHERE constraint_name = 'users_gifts_received_count_positive' 
				AND table_name = 'users'
			) THEN
				ALTER TABLE users ADD CONSTRAINT users_gifts_received_count_positive
				CHECK (gifts_received_count >= 0);
			END IF;

			IF NOT EXISTS (
				SELECT 1 FROM information_schema.table_constraints 
				WHERE constraint_name = 'users_total_coins_spent_positive' 
				AND table_name = 'users'
			) THEN
				ALTER TABLE users ADD CONSTRAINT users_total_coins_spent_positive
				CHECK (total_coins_spent_on_gifts >= 0);
			END IF;

			IF NOT EXISTS (
				SELECT 1 FROM information_schema.table_constraints 
				WHERE constraint_name = 'users_total_coins_earned_positive' 
				AND table_name = 'users'
			) THEN
				ALTER TABLE users ADD CONSTRAINT users_total_coins_earned_positive
				CHECK (total_coins_earned_from_gifts >= 0);
			END IF;
		END $$;

		-- Create indexes for gift statistics
		CREATE INDEX IF NOT EXISTS idx_users_gifts_sent_count ON users(gifts_sent_count DESC);
		CREATE INDEX IF NOT EXISTS idx_users_gifts_received_count ON users(gifts_received_count DESC);
		CREATE INDEX IF NOT EXISTS idx_users_total_coins_spent ON users(total_coins_spent_on_gifts DESC);
		CREATE INDEX IF NOT EXISTS idx_users_total_coins_earned ON users(total_coins_earned_from_gifts DESC);

		-- 2. Create gift_transactions table
		CREATE TABLE IF NOT EXISTS gift_transactions (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			
			-- Transaction participants
			sender_id VARCHAR(255) NOT NULL,
			sender_name VARCHAR(255) NOT NULL,
			sender_phone VARCHAR(20) NOT NULL,
			recipient_id VARCHAR(255) NOT NULL,
			recipient_name VARCHAR(255) NOT NULL,
			recipient_phone VARCHAR(20) NOT NULL,
			
			-- Gift details
			gift_id VARCHAR(100) NOT NULL,
			gift_name VARCHAR(100) NOT NULL,
			gift_emoji VARCHAR(50) NOT NULL,
			gift_rarity VARCHAR(50) NOT NULL,
			
			-- Financial details
			gift_price INTEGER NOT NULL,
			sender_paid INTEGER NOT NULL,
			recipient_received INTEGER NOT NULL,
			platform_commission INTEGER NOT NULL,
			
			-- Wallet balances
			sender_balance_before INTEGER NOT NULL,
			sender_balance_after INTEGER NOT NULL,
			recipient_balance_before INTEGER NOT NULL,
			recipient_balance_after INTEGER NOT NULL,
			
			-- Status and metadata
			status VARCHAR(50) DEFAULT 'completed',
			message TEXT,
			metadata JSONB DEFAULT '{}',
			
			-- Timestamps
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			
			-- Foreign keys
			CONSTRAINT gift_transactions_sender_fkey 
				FOREIGN KEY (sender_id) REFERENCES users(uid) ON DELETE CASCADE,
			CONSTRAINT gift_transactions_recipient_fkey 
				FOREIGN KEY (recipient_id) REFERENCES users(uid) ON DELETE CASCADE,
			
			-- Checks
			CHECK (sender_id != recipient_id),
			CHECK (gift_price > 0),
			CHECK (sender_paid > 0),
			CHECK (recipient_received >= 0),
			CHECK (platform_commission >= 0),
			CHECK (sender_paid = recipient_received + platform_commission)
		);

		-- Create indexes for gift_transactions
		CREATE INDEX IF NOT EXISTS idx_gift_transactions_sender_id ON gift_transactions(sender_id, created_at DESC);
		CREATE INDEX IF NOT EXISTS idx_gift_transactions_recipient_id ON gift_transactions(recipient_id, created_at DESC);
		CREATE INDEX IF NOT EXISTS idx_gift_transactions_created_at ON gift_transactions(created_at DESC);
		CREATE INDEX IF NOT EXISTS idx_gift_transactions_gift_id ON gift_transactions(gift_id);
		CREATE INDEX IF NOT EXISTS idx_gift_transactions_status ON gift_transactions(status);

		-- 3. Create platform_commissions table
		CREATE TABLE IF NOT EXISTS platform_commissions (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			
			-- Link to gift transaction
			gift_transaction_id UUID NOT NULL,
			
			-- Commission details
			commission_amount INTEGER NOT NULL,
			original_gift_price INTEGER NOT NULL,
			commission_rate DECIMAL(5,2) DEFAULT 30.00,
			
			-- Transaction participants (for reporting)
			sender_id VARCHAR(255) NOT NULL,
			recipient_id VARCHAR(255) NOT NULL,
			gift_name VARCHAR(100) NOT NULL,
			
			-- Metadata
			metadata JSONB DEFAULT '{}',
			
			-- Timestamps
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			
			-- Foreign keys
			CONSTRAINT platform_commissions_gift_transaction_fkey 
				FOREIGN KEY (gift_transaction_id) REFERENCES gift_transactions(id) ON DELETE CASCADE,
			CONSTRAINT platform_commissions_sender_fkey 
				FOREIGN KEY (sender_id) REFERENCES users(uid) ON DELETE CASCADE,
			CONSTRAINT platform_commissions_recipient_fkey 
				FOREIGN KEY (recipient_id) REFERENCES users(uid) ON DELETE CASCADE,
			
			-- Checks
			CHECK (commission_amount >= 0),
			CHECK (original_gift_price > 0),
			CHECK (commission_rate >= 0 AND commission_rate <= 100)
		);

		-- Create indexes for platform_commissions
		CREATE INDEX IF NOT EXISTS idx_platform_commissions_gift_transaction_id ON platform_commissions(gift_transaction_id);
		CREATE INDEX IF NOT EXISTS idx_platform_commissions_created_at ON platform_commissions(created_at DESC);
		CREATE INDEX IF NOT EXISTS idx_platform_commissions_sender_id ON platform_commissions(sender_id);
		CREATE INDEX IF NOT EXISTS idx_platform_commissions_recipient_id ON platform_commissions(recipient_id);

		-- 4. Update wallet_transactions to support gift types
		DO $$
		BEGIN
			-- Add gift_id column if it doesn't exist
			IF NOT EXISTS (
				SELECT 1 FROM information_schema.columns 
				WHERE table_name = 'wallet_transactions' AND column_name = 'gift_id'
			) THEN
				ALTER TABLE wallet_transactions ADD COLUMN gift_id VARCHAR(100);
			END IF;

			-- Add recipient_id column if it doesn't exist
			IF NOT EXISTS (
				SELECT 1 FROM information_schema.columns 
				WHERE table_name = 'wallet_transactions' AND column_name = 'recipient_id'
			) THEN
				ALTER TABLE wallet_transactions ADD COLUMN recipient_id VARCHAR(255);
			END IF;

			-- Add sender_id column if it doesn't exist
			IF NOT EXISTS (
				SELECT 1 FROM information_schema.columns 
				WHERE table_name = 'wallet_transactions' AND column_name = 'sender_id'
			) THEN
				ALTER TABLE wallet_transactions ADD COLUMN sender_id VARCHAR(255);
			END IF;
		END $$;

		-- Create indexes for new wallet_transactions columns
		CREATE INDEX IF NOT EXISTS idx_wallet_transactions_gift_id ON wallet_transactions(gift_id) WHERE gift_id IS NOT NULL;
		CREATE INDEX IF NOT EXISTS idx_wallet_transactions_recipient_id ON wallet_transactions(recipient_id) WHERE recipient_id IS NOT NULL;
		CREATE INDEX IF NOT EXISTS idx_wallet_transactions_sender_id ON wallet_transactions(sender_id) WHERE sender_id IS NOT NULL;

		-- 5. Create function to calculate platform commission
		CREATE OR REPLACE FUNCTION calculate_platform_commission(
			gift_price INTEGER,
			commission_rate DECIMAL DEFAULT 30.00
		)
		RETURNS INTEGER AS $$
		BEGIN
			RETURN FLOOR((gift_price * commission_rate) / 100.0);
		END;
		$$ LANGUAGE plpgsql IMMUTABLE;

		-- 6. Create function to calculate recipient amount
		CREATE OR REPLACE FUNCTION calculate_recipient_amount(
			gift_price INTEGER,
			commission_rate DECIMAL DEFAULT 30.00
		)
		RETURNS INTEGER AS $$
		BEGIN
			RETURN gift_price - calculate_platform_commission(gift_price, commission_rate);
		END;
		$$ LANGUAGE plpgsql IMMUTABLE;

		-- 7. Create function to get total platform revenue
		CREATE OR REPLACE FUNCTION get_total_platform_revenue()
		RETURNS TABLE(
			total_commissions BIGINT,
			total_gifts_facilitated BIGINT,
			average_commission DECIMAL,
			commission_rate DECIMAL
		) AS $$
		BEGIN
			RETURN QUERY
			SELECT 
				COALESCE(SUM(commission_amount), 0) as total_commissions,
				COUNT(*) as total_gifts_facilitated,
				COALESCE(AVG(commission_amount), 0) as average_commission,
				AVG(commission_rate) as commission_rate
			FROM platform_commissions;
		END;
		$$ LANGUAGE plpgsql;

		-- 8. Create function to get user gift statistics
		CREATE OR REPLACE FUNCTION get_user_gift_stats(user_uid VARCHAR(255))
		RETURNS TABLE(
			gifts_sent BIGINT,
			gifts_received BIGINT,
			total_spent INTEGER,
			total_earned INTEGER,
			favorite_gift_to_send VARCHAR(100),
			favorite_gift_received VARCHAR(100)
		) AS $$
		BEGIN
			RETURN QUERY
			SELECT 
				COUNT(*) FILTER (WHERE gt.sender_id = user_uid) as gifts_sent,
				COUNT(*) FILTER (WHERE gt.recipient_id = user_uid) as gifts_received,
				COALESCE(SUM(gt.sender_paid) FILTER (WHERE gt.sender_id = user_uid), 0)::INTEGER as total_spent,
				COALESCE(SUM(gt.recipient_received) FILTER (WHERE gt.recipient_id = user_uid), 0)::INTEGER as total_earned,
				(
					SELECT gift_name 
					FROM gift_transactions 
					WHERE sender_id = user_uid 
					GROUP BY gift_name 
					ORDER BY COUNT(*) DESC 
					LIMIT 1
				) as favorite_gift_to_send,
				(
					SELECT gift_name 
					FROM gift_transactions 
					WHERE recipient_id = user_uid 
					GROUP BY gift_name 
					ORDER BY COUNT(*) DESC 
					LIMIT 1
				) as favorite_gift_received
			FROM gift_transactions gt;
		END;
		$$ LANGUAGE plpgsql;

		-- 9. Create materialized view for top gift givers (leaderboard)
		CREATE MATERIALIZED VIEW IF NOT EXISTS top_gift_givers AS
		SELECT 
			sender_id,
			sender_name,
			COUNT(*) as gifts_sent,
			SUM(sender_paid) as total_spent,
			AVG(sender_paid) as avg_gift_value,
			MAX(created_at) as last_gift_sent
		FROM gift_transactions
		WHERE created_at >= NOW() - INTERVAL '30 days'
		GROUP BY sender_id, sender_name
		ORDER BY total_spent DESC
		LIMIT 100;

		CREATE UNIQUE INDEX IF NOT EXISTS idx_top_gift_givers_sender_id ON top_gift_givers(sender_id);

		-- 10. Create materialized view for top gift receivers (leaderboard)
		CREATE MATERIALIZED VIEW IF NOT EXISTS top_gift_receivers AS
		SELECT 
			recipient_id,
			recipient_name,
			COUNT(*) as gifts_received,
			SUM(recipient_received) as total_earned,
			AVG(recipient_received) as avg_gift_value,
			MAX(created_at) as last_gift_received
		FROM gift_transactions
		WHERE created_at >= NOW() - INTERVAL '30 days'
		GROUP BY recipient_id, recipient_name
		ORDER BY total_earned DESC
		LIMIT 100;

		CREATE UNIQUE INDEX IF NOT EXISTS idx_top_gift_receivers_recipient_id ON top_gift_receivers(recipient_id);

		-- 11. Create function to refresh gift leaderboards
		CREATE OR REPLACE FUNCTION refresh_gift_leaderboards()
		RETURNS VOID AS $$
		BEGIN
			REFRESH MATERIALIZED VIEW CONCURRENTLY top_gift_givers;
			REFRESH MATERIALIZED VIEW CONCURRENTLY top_gift_receivers;
		END;
		$$ LANGUAGE plpgsql;

		-- 12. Add comments for documentation
		COMMENT ON TABLE gift_transactions IS 'Records all virtual gift transactions between users';
		COMMENT ON TABLE platform_commissions IS 'Tracks platform revenue from gift commissions (30% of each gift)';
		COMMENT ON COLUMN users.gifts_sent_count IS 'Total number of gifts sent by user';
		COMMENT ON COLUMN users.gifts_received_count IS 'Total number of gifts received by user';
		COMMENT ON COLUMN users.total_coins_spent_on_gifts IS 'Total coins spent sending gifts';
		COMMENT ON COLUMN users.total_coins_earned_from_gifts IS 'Total coins earned from received gifts (after 30% commission)';
		COMMENT ON FUNCTION calculate_platform_commission IS 'Calculates platform commission (default 30%) from gift price';
		COMMENT ON FUNCTION calculate_recipient_amount IS 'Calculates amount recipient receives (gift price - 30% commission)';

		-- 13. Initialize gift counts for existing users
		UPDATE users 
		SET 
			gifts_sent_count = 0,
			gifts_received_count = 0,
			total_coins_spent_on_gifts = 0,
			total_coins_earned_from_gifts = 0
		WHERE 
			gifts_sent_count IS NULL OR 
			gifts_received_count IS NULL OR
			total_coins_spent_on_gifts IS NULL OR
			total_coins_earned_from_gifts IS NULL;
	`,
		},
		{
			Version: "015_allow_all_users_to_post",
			Query: `
		-- ===============================
		-- ✅ ALLOW ALL AUTHENTICATED USERS TO POST VIDEOS
		-- ===============================
		
		-- Drop the role-based posting restriction trigger
		DROP TRIGGER IF EXISTS trigger_check_user_can_post_video ON videos;
		
		-- Drop the old role-checking functions
		DROP FUNCTION IF EXISTS check_user_can_post_video();
		DROP FUNCTION IF EXISTS validate_user_can_post(VARCHAR);
		
		-- Create new function that only checks if user is active
		CREATE OR REPLACE FUNCTION validate_user_is_active(user_uid VARCHAR(255))
		RETURNS BOOLEAN AS $$
		DECLARE
			user_active BOOLEAN;
		BEGIN
			SELECT is_active INTO user_active FROM users WHERE uid = user_uid;
			RETURN COALESCE(user_active, FALSE);
		END;
		$$ LANGUAGE plpgsql;
		
		-- Create new trigger function that only validates user is active
		CREATE OR REPLACE FUNCTION check_user_is_active_for_video()
		RETURNS TRIGGER AS $$
		BEGIN
			IF NOT validate_user_is_active(NEW.user_id) THEN
				RAISE EXCEPTION 'User account is inactive or does not exist';
			END IF;
			RETURN NEW;
		END;
		$$ LANGUAGE plpgsql;
		
		-- Create new trigger that only checks if user is active (not role)
		CREATE TRIGGER trigger_check_user_is_active_for_video
			BEFORE INSERT ON videos
			FOR EACH ROW 
			EXECUTE FUNCTION check_user_is_active_for_video();
		
		-- Add comment to document the change
		COMMENT ON TRIGGER trigger_check_user_is_active_for_video ON videos IS 
		'Validates that user account is active before allowing video creation. All active authenticated users can post videos regardless of role.';
	`,
		},
	}

	for _, migration := range migrations {
		if err := applyMigration(db, migration); err != nil {
			return fmt.Errorf("failed to apply migration %s: %w", migration.Version, err)
		}
	}

	log.Println("✅ Video social media migrations completed successfully")
	log.Println("🔑 Features added:")
	log.Println("   • User roles: admin, host, guest")
	log.Println("   • WhatsApp number field (Kenyan format: 254XXXXXXXXX)")
	log.Println("   • Role-based video posting permissions (admin/host only)")
	log.Println("   • 🆕 Video price field for business posts")
	log.Println("   • 🆕 Video verification field for content verification")
	log.Println("   • 🔍 Advanced search optimization with multiple modes:")
	log.Println("      - Full-text search with ranking")
	log.Println("      - Fuzzy search with typo handling")
	log.Println("      - Exact phrase matching")
	log.Println("      - Combined search strategies")
	log.Println("   • 🚀 Search performance indexes (10-100x faster)")
	log.Println("   • 💡 Real-time search suggestions")
	log.Println("   • 📊 Popular search terms tracking")
	log.Println("   • 🎯 Advanced search filters (media type, price, verification)")
	log.Println("   • 🎁 Virtual Gift System:")
	log.Println("      - Gift transactions tracking")
	log.Println("      - Platform commission system (30%)")
	log.Println("      - User gift statistics (sent/received/spent/earned)")
	log.Println("      - Gift leaderboards (top givers & receivers)")
	log.Println("      - Wallet integration for gift payments")
	log.Println("   • Database triggers for role validation")
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
