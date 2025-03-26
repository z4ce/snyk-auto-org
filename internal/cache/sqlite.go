package cache

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

	createTargetsTableSQL = `
CREATE TABLE IF NOT EXISTS targets (
	id TEXT PRIMARY KEY,
	org_id TEXT NOT NULL,
	display_name TEXT NOT NULL,
	url TEXT NOT NULL,
	FOREIGN KEY (org_id) REFERENCES organizations(id)
);`

	insertOrgSQL = `
INSERT OR REPLACE INTO organizations (id, name, slug)
VALUES (?, ?, ?);`

	insertMetadataSQL = `
INSERT OR REPLACE INTO metadata (key, value)
VALUES (?, ?);`

	insertTargetSQL = `
INSERT OR REPLACE INTO targets (id, org_id, display_name, url)
VALUES (?, ?, ?, ?);`

	selectOrgsSQL = `
SELECT id, name, slug
FROM organizations;`

	selectMetadataSQL = `
SELECT value
FROM metadata
WHERE key = ?;`

	selectTargetsSQL = `
SELECT id, org_id, display_name, url
FROM targets;`

	selectTargetsByOrgIDSQL = `
SELECT id, org_id, display_name, url
FROM targets
WHERE org_id = ?;`

	selectTargetsByURLSQL = `
SELECT t.id, t.org_id, t.display_name, t.url, o.name as org_name
FROM targets t
JOIN organizations o ON t.org_id = o.id
WHERE LOWER(t.url) = LOWER(?) OR LOWER(t.url) = LOWER(?);`
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

	if _, err := db.Exec(createTargetsTableSQL); err != nil {
		return nil, fmt.Errorf("failed to create targets table: %w", err)
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

// StoreTargets stores targets for an organization in the cache
func (c *SQLiteCache) StoreTargets(orgID string, targets []api.Target) error {
	if len(targets) == 0 {
		return nil // Nothing to store
	}

	// Begin a transaction
	tx, err := c.db.Beginx()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Insert each target
	for _, target := range targets {
		if _, err := tx.Exec(insertTargetSQL, target.ID, orgID, target.Attributes.DisplayName, target.Attributes.URL); err != nil {
			return fmt.Errorf("failed to insert target: %w", err)
		}
	}

	// Store the targets update timestamp for this org
	if _, err := tx.Exec(insertMetadataSQL, fmt.Sprintf("targets_update_%s", orgID), time.Now().Format(time.RFC3339)); err != nil {
		return fmt.Errorf("failed to update targets timestamp: %w", err)
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// GetTargets retrieves all targets from the cache
func (c *SQLiteCache) GetTargets() ([]api.Target, error) {
	rows, err := c.db.Query(selectTargetsSQL)
	if err != nil {
		return nil, fmt.Errorf("failed to select targets: %w", err)
	}
	defer rows.Close()

	var targets []api.Target
	for rows.Next() {
		var id, orgID, displayName, url string
		if err := rows.Scan(&id, &orgID, &displayName, &url); err != nil {
			return nil, fmt.Errorf("failed to scan target row: %w", err)
		}

		target := api.Target{
			ID: id,
		}
		target.Attributes.DisplayName = displayName
		target.Attributes.URL = url

		targets = append(targets, target)
	}

	return targets, nil
}

// GetTargetsByOrgID retrieves targets for a specific organization from the cache
func (c *SQLiteCache) GetTargetsByOrgID(orgID string) ([]api.Target, error) {
	rows, err := c.db.Query(selectTargetsByOrgIDSQL, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to select targets for org %s: %w", orgID, err)
	}
	defer rows.Close()

	var targets []api.Target
	for rows.Next() {
		var id, orgID, displayName, url string
		if err := rows.Scan(&id, &orgID, &displayName, &url); err != nil {
			return nil, fmt.Errorf("failed to scan target row: %w", err)
		}

		target := api.Target{
			ID: id,
		}
		target.Attributes.DisplayName = displayName
		target.Attributes.URL = url

		targets = append(targets, target)
	}

	return targets, nil
}

// GetTargetsByURL retrieves targets with a specific URL from the cache
// This function now checks for both HTTP and HTTPS variants of the URL
func (c *SQLiteCache) GetTargetsByURL(url string) ([]api.OrgTarget, error) {
	// Create both HTTP and HTTPS variants of the URL
	httpVariant := url
	httpsVariant := url

	// Make sure we have both variants of the URL
	if strings.HasPrefix(url, "https://") {
		httpVariant = "http://" + strings.TrimPrefix(url, "https://")
	} else if strings.HasPrefix(url, "http://") {
		httpsVariant = "https://" + strings.TrimPrefix(url, "http://")
	} else {
		// If no protocol provided, default to both http:// and https:// prefixes
		httpVariant = "http://" + url
		httpsVariant = "https://" + url
	}

	rows, err := c.db.Query(selectTargetsByURLSQL, httpVariant, httpsVariant)
	if err != nil {
		return nil, fmt.Errorf("failed to select targets for URL %s: %w", url, err)
	}
	defer rows.Close()

	var orgTargets []api.OrgTarget
	for rows.Next() {
		var id, orgID, displayName, url, orgName string
		if err := rows.Scan(&id, &orgID, &displayName, &url, &orgName); err != nil {
			return nil, fmt.Errorf("failed to scan target row: %w", err)
		}

		orgTarget := api.OrgTarget{
			OrgID:      orgID,
			OrgName:    orgName,
			TargetURL:  url,
			TargetName: displayName,
		}

		orgTargets = append(orgTargets, orgTarget)
	}

	return orgTargets, nil
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

// IsTargetsCacheExpired checks if the targets cache for an organization has expired
func (c *SQLiteCache) IsTargetsCacheExpired(orgID string, ttl time.Duration) (bool, error) {
	var lastUpdateStr string
	err := c.db.Get(&lastUpdateStr, selectMetadataSQL, fmt.Sprintf("targets_update_%s", orgID))
	if err != nil {
		// If the key doesn't exist, the cache is expired
		return true, nil
	}

	lastUpdate, err := time.Parse(time.RFC3339, lastUpdateStr)
	if err != nil {
		return true, fmt.Errorf("failed to parse targets last update timestamp: %w", err)
	}

	return time.Since(lastUpdate) > ttl, nil
}

// ResetCache clears all cached data
func (c *SQLiteCache) ResetCache() error {
	_, err := c.db.Exec("DELETE FROM targets")
	if err != nil {
		return fmt.Errorf("failed to delete targets: %w", err)
	}

	_, err = c.db.Exec("DELETE FROM organizations")
	if err != nil {
		return fmt.Errorf("failed to delete organizations: %w", err)
	}

	_, err = c.db.Exec("DELETE FROM metadata")
	if err != nil {
		return fmt.Errorf("failed to delete metadata: %w", err)
	}

	return nil
}
