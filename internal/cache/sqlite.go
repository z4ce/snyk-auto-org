package cache

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/z4ce/snyk-auto-org/internal/api"
)

const (
	createOrgsTableSQL = `
CREATE TABLE IF NOT EXISTS organizations (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	slug TEXT NOT NULL
);`

	createMetadataTableSQL = `
CREATE TABLE IF NOT EXISTS metadata (
	key TEXT PRIMARY KEY,
	value TEXT NOT NULL
);`

	insertOrgSQL = `
INSERT OR REPLACE INTO organizations (id, name, slug)
VALUES (?, ?, ?);`

	insertMetadataSQL = `
INSERT OR REPLACE INTO metadata (key, value)
VALUES (?, ?);`

	selectOrgsSQL = `
SELECT id, name, slug
FROM organizations;`

	selectMetadataSQL = `
SELECT value
FROM metadata
WHERE key = ?;`
)

// SQLiteCache implements caching of Snyk organizations using SQLite
type SQLiteCache struct {
	db *sqlx.DB
}

// NewSQLiteCache creates a new SQLite cache
func NewSQLiteCache() (*SQLiteCache, error) {
	// Get user's home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get user home directory: %w", err)
	}

	// Create the cache directory if it doesn't exist
	cacheDir := filepath.Join(homeDir, ".config", "snyk-auto-org")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	// Connect to the SQLite database
	dbPath := filepath.Join(cacheDir, "cache.db")
	db, err := sqlx.Connect("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to SQLite database: %w", err)
	}

	// Create the tables if they don't exist
	if _, err := db.Exec(createOrgsTableSQL); err != nil {
		return nil, fmt.Errorf("failed to create organizations table: %w", err)
	}

	if _, err := db.Exec(createMetadataTableSQL); err != nil {
		return nil, fmt.Errorf("failed to create metadata table: %w", err)
	}

	return &SQLiteCache{
		db: db,
	}, nil
}

// Close closes the database connection
func (c *SQLiteCache) Close() error {
	return c.db.Close()
}

// StoreOrganizations stores the organizations in the cache
func (c *SQLiteCache) StoreOrganizations(orgs []api.Organization) error {
	// Begin a transaction
	tx, err := c.db.Beginx()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Insert each organization
	for _, org := range orgs {
		if _, err := tx.Exec(insertOrgSQL, org.ID, org.Name, org.Slug); err != nil {
			return fmt.Errorf("failed to insert organization: %w", err)
		}
	}

	// Store the update timestamp
	if _, err := tx.Exec(insertMetadataSQL, "last_update", time.Now().Format(time.RFC3339)); err != nil {
		return fmt.Errorf("failed to update timestamp: %w", err)
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// GetOrganizations retrieves the organizations from the cache
func (c *SQLiteCache) GetOrganizations() ([]api.Organization, error) {
	var orgs []api.Organization
	if err := c.db.Select(&orgs, selectOrgsSQL); err != nil {
		return nil, fmt.Errorf("failed to select organizations: %w", err)
	}

	return orgs, nil
}

// IsExpired checks if the cache has expired
func (c *SQLiteCache) IsExpired(ttl time.Duration) (bool, error) {
	var lastUpdateStr string
	err := c.db.Get(&lastUpdateStr, selectMetadataSQL, "last_update")
	if err != nil {
		// If the key doesn't exist, the cache is expired
		return true, nil
	}

	lastUpdate, err := time.Parse(time.RFC3339, lastUpdateStr)
	if err != nil {
		return true, fmt.Errorf("failed to parse last update timestamp: %w", err)
	}

	return time.Since(lastUpdate) > ttl, nil
}

// ResetCache clears all cached data
func (c *SQLiteCache) ResetCache() error {
	_, err := c.db.Exec("DELETE FROM organizations")
	if err != nil {
		return fmt.Errorf("failed to delete organizations: %w", err)
	}

	_, err = c.db.Exec("DELETE FROM metadata")
	if err != nil {
		return fmt.Errorf("failed to delete metadata: %w", err)
	}

	return nil
}
