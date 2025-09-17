// ===============================
// internal/database/migrations.go - Updated with Chat and Contacts Support
// ===============================

package database

import (
	"fmt"
	"log"

	"github.com/jmoiron/sqlx"
)

func RunMigrations(db *sqlx.DB) error {
	log.Println("📄 Running video social media migrations with chat and contacts support...")

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
			Version: "003_contacts_and_chat_system",
			Query: `
				-- Contacts table - stores user contact relationships
				CREATE TABLE IF NOT EXISTS contacts (
					id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
					user_id VARCHAR(255) NOT NULL,
					contact_user_id VARCHAR(255) NOT NULL,
					contact_name VARCHAR(255) NOT NULL,
					contact_phone VARCHAR(20) NOT NULL,
					is_blocked BOOLEAN DEFAULT false,
					created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
					updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
					UNIQUE(user_id, contact_user_id),
					CHECK(user_id != contact_user_id)
				);

				-- Blocked contacts table - separate table for blocked relationships
				CREATE TABLE IF NOT EXISTS blocked_contacts (
					id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
					user_id VARCHAR(255) NOT NULL,
					blocked_user_id VARCHAR(255) NOT NULL,
					blocked_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
					UNIQUE(user_id, blocked_user_id),
					CHECK(user_id != blocked_user_id)
				);

				-- Chats table - chat conversations
				CREATE TABLE IF NOT EXISTS chats (
					chat_id VARCHAR(255) PRIMARY KEY,
					participants TEXT[] NOT NULL,
					last_message TEXT DEFAULT '',
					last_message_type VARCHAR(50) DEFAULT 'text',
					last_message_sender VARCHAR(255),
					last_message_time TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
					unread_counts JSONB DEFAULT '{}',
					is_archived JSONB DEFAULT '{}',
					is_pinned JSONB DEFAULT '{}',
					is_muted JSONB DEFAULT '{}',
					chat_wallpapers JSONB DEFAULT '{}',
					font_sizes JSONB DEFAULT '{}',
					created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
					updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
				);

				-- Messages table - chat messages
				CREATE TABLE IF NOT EXISTS messages (
					message_id VARCHAR(255) PRIMARY KEY,
					chat_id VARCHAR(255) NOT NULL,
					sender_id VARCHAR(255) NOT NULL,
					content TEXT NOT NULL DEFAULT '',
					type VARCHAR(50) NOT NULL DEFAULT 'text',
					status VARCHAR(50) NOT NULL DEFAULT 'sent',
					timestamp TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
					media_url TEXT,
					media_metadata JSONB DEFAULT '{}',
					reply_to_message_id VARCHAR(255),
					reply_to_content TEXT,
					reply_to_sender VARCHAR(255),
					reactions JSONB DEFAULT '{}',
					is_edited BOOLEAN DEFAULT false,
					edited_at TIMESTAMP WITH TIME ZONE,
					is_pinned BOOLEAN DEFAULT false,
					read_by JSONB DEFAULT '{}',
					delivered_to JSONB DEFAULT '{}',
					created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
					CONSTRAINT messages_type_check CHECK (type IN ('text', 'image', 'video', 'file', 'audio', 'location', 'contact')),
					CONSTRAINT messages_status_check CHECK (status IN ('sending', 'sent', 'delivered', 'read', 'failed'))
				);

				-- Message reactions table (for easier querying)
				CREATE TABLE IF NOT EXISTS message_reactions (
					id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
					message_id VARCHAR(255) NOT NULL,
					user_id VARCHAR(255) NOT NULL,
					emoji VARCHAR(20) NOT NULL,
					created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
					UNIQUE(message_id, user_id)
				);

				-- Chat participants table (for easier querying)
				CREATE TABLE IF NOT EXISTS chat_participants (
					id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
					chat_id VARCHAR(255) NOT NULL,
					user_id VARCHAR(255) NOT NULL,
					joined_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
					left_at TIMESTAMP WITH TIME ZONE,
					UNIQUE(chat_id, user_id)
				);

				-- Contact sync metadata table
				CREATE TABLE IF NOT EXISTS contact_sync_metadata (
					user_id VARCHAR(255) PRIMARY KEY,
					last_sync_time TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
					sync_version VARCHAR(50) DEFAULT '1.0',
					device_contacts_hash VARCHAR(64),
					sync_count INTEGER DEFAULT 0
				);
			`,
		},
		{
			Version: "004_add_foreign_keys",
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

					-- User follows foreign keys
					IF NOT EXISTS (SELECT 1 FROM information_schema.table_constraints 
								  WHERE constraint_name = 'user_follows_follower_id_fkey' 
								  AND table_name = 'user_follows') THEN
						ALTER TABLE user_follows ADD CONSTRAINT user_follows_follower_id_fkey 
						FOREIGN KEY (follower_id) REFERENCES users(uid) ON DELETE CASCADE;
					END IF;

					-- Contacts foreign keys
					IF NOT EXISTS (SELECT 1 FROM information_schema.table_constraints 
								  WHERE constraint_name = 'contacts_user_id_fkey' 
								  AND table_name = 'contacts') THEN
						ALTER TABLE contacts ADD CONSTRAINT contacts_user_id_fkey 
						FOREIGN KEY (user_id) REFERENCES users(uid) ON DELETE CASCADE;
					END IF;

					IF NOT EXISTS (SELECT 1 FROM information_schema.table_constraints 
								  WHERE constraint_name = 'contacts_contact_user_id_fkey' 
								  AND table_name = 'contacts') THEN
						ALTER TABLE contacts ADD CONSTRAINT contacts_contact_user_id_fkey 
						FOREIGN KEY (contact_user_id) REFERENCES users(uid) ON DELETE CASCADE;
					END IF;

					-- Blocked contacts foreign keys
					IF NOT EXISTS (SELECT 1 FROM information_schema.table_constraints 
								  WHERE constraint_name = 'blocked_contacts_user_id_fkey' 
								  AND table_name = 'blocked_contacts') THEN
						ALTER TABLE blocked_contacts ADD CONSTRAINT blocked_contacts_user_id_fkey 
						FOREIGN KEY (user_id) REFERENCES users(uid) ON DELETE CASCADE;
					END IF;

					-- Messages foreign keys
					IF NOT EXISTS (SELECT 1 FROM information_schema.table_constraints 
								  WHERE constraint_name = 'messages_chat_id_fkey' 
								  AND table_name = 'messages') THEN
						ALTER TABLE messages ADD CONSTRAINT messages_chat_id_fkey 
						FOREIGN KEY (chat_id) REFERENCES chats(chat_id) ON DELETE CASCADE;
					END IF;

					IF NOT EXISTS (SELECT 1 FROM information_schema.table_constraints 
								  WHERE constraint_name = 'messages_sender_id_fkey' 
								  AND table_name = 'messages') THEN
						ALTER TABLE messages ADD CONSTRAINT messages_sender_id_fkey 
						FOREIGN KEY (sender_id) REFERENCES users(uid) ON DELETE CASCADE;
					END IF;

					-- Chat participants foreign keys
					IF NOT EXISTS (SELECT 1 FROM information_schema.table_constraints 
								  WHERE constraint_name = 'chat_participants_chat_id_fkey' 
								  AND table_name = 'chat_participants') THEN
						ALTER TABLE chat_participants ADD CONSTRAINT chat_participants_chat_id_fkey 
						FOREIGN KEY (chat_id) REFERENCES chats(chat_id) ON DELETE CASCADE;
					END IF;

					-- Message reactions foreign keys
					IF NOT EXISTS (SELECT 1 FROM information_schema.table_constraints 
								  WHERE constraint_name = 'message_reactions_message_id_fkey' 
								  AND table_name = 'message_reactions') THEN
						ALTER TABLE message_reactions ADD CONSTRAINT message_reactions_message_id_fkey 
						FOREIGN KEY (message_id) REFERENCES messages(message_id) ON DELETE CASCADE;
					END IF;

					-- Wallet foreign keys (existing)
					IF NOT EXISTS (SELECT 1 FROM information_schema.table_constraints 
								  WHERE constraint_name = 'wallets_user_id_fkey' 
								  AND table_name = 'wallets') THEN
						ALTER TABLE wallets ADD CONSTRAINT wallets_user_id_fkey 
						FOREIGN KEY (user_id) REFERENCES users(uid) ON DELETE CASCADE;
					END IF;

					-- Contact sync metadata foreign key
					IF NOT EXISTS (SELECT 1 FROM information_schema.table_constraints 
								  WHERE constraint_name = 'contact_sync_metadata_user_id_fkey' 
								  AND table_name = 'contact_sync_metadata') THEN
						ALTER TABLE contact_sync_metadata ADD CONSTRAINT contact_sync_metadata_user_id_fkey 
						FOREIGN KEY (user_id) REFERENCES users(uid) ON DELETE CASCADE;
					END IF;
				END $block1$;
			`,
		},
		{
			Version: "005_create_indexes",
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

				-- Chat and message indexes
				CREATE INDEX IF NOT EXISTS idx_chats_participants ON chats USING GIN(participants);
				CREATE INDEX IF NOT EXISTS idx_chats_last_message_time ON chats(last_message_time DESC);
				CREATE INDEX IF NOT EXISTS idx_chats_created_at ON chats(created_at DESC);

				CREATE INDEX IF NOT EXISTS idx_messages_chat_id ON messages(chat_id);
				CREATE INDEX IF NOT EXISTS idx_messages_sender_id ON messages(sender_id);
				CREATE INDEX IF NOT EXISTS idx_messages_timestamp ON messages(timestamp DESC);
				CREATE INDEX IF NOT EXISTS idx_messages_type ON messages(type);
				CREATE INDEX IF NOT EXISTS idx_messages_status ON messages(status);
				CREATE INDEX IF NOT EXISTS idx_messages_is_pinned ON messages(is_pinned) WHERE is_pinned = true;

				-- Contact indexes
				CREATE INDEX IF NOT EXISTS idx_contacts_user_id ON contacts(user_id);
				CREATE INDEX IF NOT EXISTS idx_contacts_contact_user_id ON contacts(contact_user_id);
				CREATE INDEX IF NOT EXISTS idx_contacts_is_blocked ON contacts(is_blocked);
				CREATE INDEX IF NOT EXISTS idx_blocked_contacts_user_id ON blocked_contacts(user_id);
				CREATE INDEX IF NOT EXISTS idx_blocked_contacts_blocked_user_id ON blocked_contacts(blocked_user_id);

				-- Chat participant indexes
				CREATE INDEX IF NOT EXISTS idx_chat_participants_user_id ON chat_participants(user_id);
				CREATE INDEX IF NOT EXISTS idx_chat_participants_chat_id ON chat_participants(chat_id);

				-- Message reaction indexes
				CREATE INDEX IF NOT EXISTS idx_message_reactions_message_id ON message_reactions(message_id);
				CREATE INDEX IF NOT EXISTS idx_message_reactions_user_id ON message_reactions(user_id);

				-- Contact sync metadata indexes
				CREATE INDEX IF NOT EXISTS idx_contact_sync_metadata_last_sync_time ON contact_sync_metadata(last_sync_time);

				-- Like indexes (existing)
				CREATE INDEX IF NOT EXISTS idx_video_likes_video_id ON video_likes(video_id);
				CREATE INDEX IF NOT EXISTS idx_video_likes_user_id ON video_likes(user_id);
				CREATE INDEX IF NOT EXISTS idx_comment_likes_comment_id ON comment_likes(comment_id);
				CREATE INDEX IF NOT EXISTS idx_comment_likes_user_id ON comment_likes(user_id);

				-- Follow indexes (existing)
				CREATE INDEX IF NOT EXISTS idx_user_follows_follower_id ON user_follows(follower_id);
				CREATE INDEX IF NOT EXISTS idx_user_follows_following_id ON user_follows(following_id);

				-- Wallet indexes (existing)
				CREATE INDEX IF NOT EXISTS idx_wallets_user_id ON wallets(user_id);
				CREATE INDEX IF NOT EXISTS idx_wallet_transactions_user_id ON wallet_transactions(user_id);
				CREATE INDEX IF NOT EXISTS idx_wallet_transactions_type ON wallet_transactions(type);
			`,
		},
		{
			Version: "006_add_data_constraints",
			Query: `
				-- Add data validation constraints using DO blocks
				DO $block1$
				BEGIN
					-- User constraints (existing)
					IF NOT EXISTS (SELECT 1 FROM information_schema.check_constraints 
								  WHERE constraint_name = 'users_name_length') THEN
						ALTER TABLE users ADD CONSTRAINT users_name_length
						CHECK (LENGTH(name) >= 1 AND LENGTH(name) <= 50);
					END IF;

					-- Message constraints
					IF NOT EXISTS (SELECT 1 FROM information_schema.check_constraints 
								  WHERE constraint_name = 'messages_content_length') THEN
						ALTER TABLE messages ADD CONSTRAINT messages_content_length
						CHECK (LENGTH(content) <= 4000);
					END IF;

					-- Chat constraints
					IF NOT EXISTS (SELECT 1 FROM information_schema.check_constraints 
								  WHERE constraint_name = 'chats_participants_count') THEN
						ALTER TABLE chats ADD CONSTRAINT chats_participants_count
						CHECK (array_length(participants, 1) >= 2);
					END IF;

					-- Contact constraints
					IF NOT EXISTS (SELECT 1 FROM information_schema.check_constraints 
								  WHERE constraint_name = 'contacts_phone_length') THEN
						ALTER TABLE contacts ADD CONSTRAINT contacts_phone_length
						CHECK (LENGTH(contact_phone) >= 10 AND LENGTH(contact_phone) <= 20);
					END IF;
				END $block1$;
			`,
		},
		{
			Version: "007_create_functions",
			Query: `
				-- Function to update chat last message
				CREATE OR REPLACE FUNCTION update_chat_last_message()
				RETURNS TRIGGER AS $func1$
				BEGIN
					IF TG_OP = 'INSERT' THEN
						UPDATE chats 
						SET last_message = NEW.content,
							last_message_type = NEW.type,
							last_message_sender = NEW.sender_id,
							last_message_time = NEW.timestamp,
							updated_at = CURRENT_TIMESTAMP
						WHERE chat_id = NEW.chat_id;
						RETURN NEW;
					END IF;
					RETURN NULL;
				END;
				$func1$ LANGUAGE plpgsql;

				-- Function to generate chat ID from participants
				CREATE OR REPLACE FUNCTION generate_chat_id(participant1 TEXT, participant2 TEXT)
				RETURNS TEXT AS $func2$
				BEGIN
					-- Sort participants to ensure consistent chat IDs
					IF participant1 < participant2 THEN
						RETURN participant1 || '_' || participant2;
					ELSE
						RETURN participant2 || '_' || participant1;
					END IF;
				END;
				$func2$ LANGUAGE plpgsql;

				-- Function to check if users are not blocked
				CREATE OR REPLACE FUNCTION users_not_blocked(user1_id TEXT, user2_id TEXT)
				RETURNS BOOLEAN AS $func3$
				BEGIN
					-- Check if either user has blocked the other
					RETURN NOT EXISTS (
						SELECT 1 FROM blocked_contacts 
						WHERE (user_id = user1_id AND blocked_user_id = user2_id)
						   OR (user_id = user2_id AND blocked_user_id = user1_id)
					);
				END;
				$func3$ LANGUAGE plpgsql;

				-- Function to update user video count (existing)
				CREATE OR REPLACE FUNCTION update_user_video_count()
				RETURNS TRIGGER AS $func4$
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
				$func4$ LANGUAGE plpgsql;

				-- Function to update video like count (existing)
				CREATE OR REPLACE FUNCTION update_video_like_count()
				RETURNS TRIGGER AS $func5$
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
				$func5$ LANGUAGE plpgsql;

				-- Function to update comment count (existing)
				CREATE OR REPLACE FUNCTION update_video_comment_count()
				RETURNS TRIGGER AS $func6$
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
				$func6$ LANGUAGE plpgsql;

				-- Function to update follow counts (existing)
				CREATE OR REPLACE FUNCTION update_user_follow_counts()
				RETURNS TRIGGER AS $func7$
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
				$func7$ LANGUAGE plpgsql;
			`,
		},
		{
			Version: "008_create_triggers",
			Query: `
				-- Drop existing triggers if they exist
				DROP TRIGGER IF EXISTS trigger_update_chat_last_message ON messages;
				DROP TRIGGER IF EXISTS trigger_update_user_video_count ON videos;
				DROP TRIGGER IF EXISTS trigger_update_video_like_count ON video_likes;
				DROP TRIGGER IF EXISTS trigger_update_video_comment_count ON comments;
				DROP TRIGGER IF EXISTS trigger_update_user_follow_counts ON user_follows;

				-- Create triggers
				CREATE TRIGGER trigger_update_chat_last_message
					AFTER INSERT ON messages
					FOR EACH ROW 
					EXECUTE FUNCTION update_chat_last_message();

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
			Version: "009_add_last_post_at_and_trigger",
			Query: `
				-- Add last_post_at column to users table
				ALTER TABLE users ADD COLUMN IF NOT EXISTS last_post_at TIMESTAMP WITH TIME ZONE;

				-- Create function to update user's last_post_at when video is created
				CREATE OR REPLACE FUNCTION update_user_last_post()
				RETURNS TRIGGER AS $func8$
				BEGIN
					UPDATE users 
					SET last_post_at = NEW.created_at,
						updated_at = CURRENT_TIMESTAMP
					WHERE uid = NEW.user_id;
					RETURN NEW;
				END;
				$func8$ LANGUAGE plpgsql;

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
	}

	for _, migration := range migrations {
		if err := applyMigration(db, migration); err != nil {
			return fmt.Errorf("failed to apply migration %s: %w", migration.Version, err)
		}
	}

	log.Println("✅ Video social media migrations with chat and contacts support completed successfully")
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
