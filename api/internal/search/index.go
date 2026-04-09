package search

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

// PackageEntry represents a package in the search index.
type PackageEntry struct {
	Name      string `json:"name"`
	Summary   string `json:"summary"`
	Downloads int64  `json:"downloads"`
}

// Index is a SQLite-backed full-text search index for packages.
type Index struct {
	db *sql.DB
}

// NewIndex opens (or creates) a SQLite database at dsn and initialises the
// FTS5 virtual table and companion meta table.
func NewIndex(dsn string) (*Index, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("search: open db: %w", err)
	}

	// Enable WAL for better concurrent read performance (no-op for :memory:).
	if _, err := db.Exec(`PRAGMA journal_mode=WAL`); err != nil {
		db.Close()
		return nil, fmt.Errorf("search: set WAL: %w", err)
	}

	// packages_meta holds authoritative data (name, summary, downloads).
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS packages_meta (
			name      TEXT PRIMARY KEY,
			summary   TEXT,
			downloads INTEGER
		)
	`); err != nil {
		db.Close()
		return nil, fmt.Errorf("search: create meta table: %w", err)
	}

	// packages_fts is the FTS5 virtual table.  downloads is UNINDEXED so it
	// is stored but not tokenised; sorting happens via the meta JOIN.
	if _, err := db.Exec(`
		CREATE VIRTUAL TABLE IF NOT EXISTS packages_fts USING fts5(
			name,
			summary,
			downloads UNINDEXED,
			tokenize='porter unicode61'
		)
	`); err != nil {
		db.Close()
		return nil, fmt.Errorf("search: create fts table: %w", err)
	}

	return &Index{db: db}, nil
}

// Upsert inserts or replaces a package in both the meta and FTS tables.
func (idx *Index) Upsert(entry PackageEntry) error {
	tx, err := idx.db.Begin()
	if err != nil {
		return fmt.Errorf("search: begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	if _, err := tx.Exec(`
		INSERT OR REPLACE INTO packages_meta (name, summary, downloads)
		VALUES (?, ?, ?)
	`, entry.Name, entry.Summary, entry.Downloads); err != nil {
		return fmt.Errorf("search: upsert meta: %w", err)
	}

	// Remove old FTS row (if any) then insert fresh to avoid duplicates.
	if _, err := tx.Exec(`DELETE FROM packages_fts WHERE name = ?`, entry.Name); err != nil {
		return fmt.Errorf("search: delete fts: %w", err)
	}

	if _, err := tx.Exec(`
		INSERT INTO packages_fts (name, summary, downloads)
		VALUES (?, ?, ?)
	`, entry.Name, entry.Summary, entry.Downloads); err != nil {
		return fmt.Errorf("search: insert fts: %w", err)
	}

	return tx.Commit()
}

// Search performs a prefix-aware full-text search and returns up to limit
// results ordered by exact-name match first, then downloads descending.
func (idx *Index) Search(query string, limit int) ([]PackageEntry, error) {
	if limit <= 0 {
		limit = 10
	}

	// Wrap in quotes and append * for prefix matching (typeahead behaviour).
	// sqlite fts5 treats "foo"* as a prefix query on the phrase "foo".
	ftsQuery := fmt.Sprintf(`"%s"*`, query)

	rows, err := idx.db.Query(`
		SELECT m.name, m.summary, m.downloads
		FROM packages_fts f
		JOIN packages_meta m ON m.name = f.name
		WHERE packages_fts MATCH ?
		ORDER BY
			CASE WHEN lower(m.name) = lower(?) THEN 0 ELSE 1 END,
			m.downloads DESC
		LIMIT ?
	`, ftsQuery, query, limit)
	if err != nil {
		return nil, fmt.Errorf("search: query: %w", err)
	}
	defer rows.Close()

	var results []PackageEntry
	for rows.Next() {
		var e PackageEntry
		if err := rows.Scan(&e.Name, &e.Summary, &e.Downloads); err != nil {
			return nil, fmt.Errorf("search: scan: %w", err)
		}
		results = append(results, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("search: rows: %w", err)
	}

	return results, nil
}

// Close releases the underlying database connection.
func (idx *Index) Close() error {
	return idx.db.Close()
}
