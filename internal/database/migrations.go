// ===============================
// internal/database/migrations.go
// ===============================

package database

import (
	"fmt"

	"github.com/jmoiron/sqlx"
)

func RunMigrations(db *sqlx.DB) error {
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
			Version: "001_create_coin_purchase_requests",
			Query: `
				CREATE TABLE IF NOT EXISTS coin_purchase_requests (
					id VARCHAR(255) PRIMARY KEY,
					user_id VARCHAR(255) NOT NULL,
					package_id VARCHAR(255) NOT NULL,
					coin_amount INTEGER NOT NULL,
					paid_amount DECIMAL(10,2) NOT NULL,
					payment_reference VARCHAR(255) NOT NULL,
					payment_method VARCHAR(50) NOT NULL,
					status VARCHAR(50) NOT NULL DEFAULT 'pending_admin_verification',
					requested_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
					processed_at TIMESTAMP WITH TIME ZONE,
					admin_note TEXT,
					FOREIGN KEY (user_id) REFERENCES users(uid) ON DELETE CASCADE
				);
				CREATE INDEX IF NOT EXISTS idx_coin_purchase_requests_user_id ON coin_purchase_requests(user_id);
				CREATE INDEX IF NOT EXISTS idx_coin_purchase_requests_status ON coin_purchase_requests(status);
			`,
		},
		{
			Version: "002_add_missing_indexes",
			Query: `
				CREATE INDEX IF NOT EXISTS idx_episodes_drama_episode ON episodes(drama_id, episode_number);
				CREATE INDEX IF NOT EXISTS idx_wallet_transactions_user_type ON wallet_transactions(user_id, type);
				CREATE INDEX IF NOT EXISTS idx_dramas_premium_active ON dramas(is_premium, is_active);
			`,
		},
		{
			Version: "003_update_user_constraints",
			Query: `
				-- Update phone number constraint to be more flexible
				ALTER TABLE users DROP CONSTRAINT IF EXISTS valid_phone_number;
				ALTER TABLE users ADD CONSTRAINT valid_phone_number 
					CHECK (phone_number ~ '^\+?[1-9]\d{1,14} OR phone_number = '');
			`,
		},
	}

	for _, migration := range migrations {
		if err := applyMigration(db, migration); err != nil {
			return fmt.Errorf("failed to apply migration %s: %w", migration.Version, err)
		}
	}

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
