package search

import (
	"database/sql"
	"fmt"
	"strings"

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

	// Rebuild the FTS5 table from meta on every startup to clear any dupes
	// (FTS5 has no unique constraint) and ensure a clean 1:1 mapping.
	db.Exec(`DROP TABLE IF EXISTS packages_fts`) //nolint:errcheck

	if _, err := db.Exec(`
		CREATE VIRTUAL TABLE packages_fts USING fts5(
			name,
			summary,
			downloads UNINDEXED,
			tokenize='porter unicode61'
		)
	`); err != nil {
		db.Close()
		return nil, fmt.Errorf("search: create fts table: %w", err)
	}

	// Populate FTS from existing meta data (fast local copy, no network).
	if _, err := db.Exec(`
		INSERT INTO packages_fts (name, summary, downloads)
		SELECT name, summary, downloads FROM packages_meta
	`); err != nil {
		db.Close()
		return nil, fmt.Errorf("search: populate fts from meta: %w", err)
	}

	return &Index{db: db}, nil
}

// Upsert inserts or replaces a package in both the meta and FTS tables.
func (idx *Index) Upsert(entry PackageEntry) error {
	return idx.UpsertBatch([]PackageEntry{entry})
}

// UpsertBatch inserts new packages into both the meta and FTS tables,
// skipping any that already exist. Only inserts into FTS when the meta row
// is new (FTS5 tables have no unique constraint, so we guard against dupes
// by checking the meta INSERT result).
func (idx *Index) UpsertBatch(entries []PackageEntry) error {
	if len(entries) == 0 {
		return nil
	}

	tx, err := idx.db.Begin()
	if err != nil {
		return fmt.Errorf("search: begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	metaStmt, err := tx.Prepare(`INSERT OR IGNORE INTO packages_meta (name, summary, downloads) VALUES (?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("search: prepare meta: %w", err)
	}
	defer metaStmt.Close()

	ftsStmt, err := tx.Prepare(`INSERT INTO packages_fts (name, summary, downloads) VALUES (?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("search: prepare fts: %w", err)
	}
	defer ftsStmt.Close()

	for _, e := range entries {
		res, err := metaStmt.Exec(e.Name, e.Summary, e.Downloads)
		if err != nil {
			return fmt.Errorf("search: upsert meta %q: %w", e.Name, err)
		}
		// Only insert into FTS if the meta row was actually inserted (not ignored).
		if n, _ := res.RowsAffected(); n > 0 {
			if _, err := ftsStmt.Exec(e.Name, e.Summary, e.Downloads); err != nil {
				return fmt.Errorf("search: insert fts %q: %w", e.Name, err)
			}
		}
	}

	return tx.Commit()
}

// sanitizeFTSQuery strips characters and keywords that have special meaning in
// FTS5 MATCH expressions so that raw user input cannot inject query syntax.
//
// Strategy:
//  1. Trim surrounding whitespace.
//  2. Remove double-quotes (they delimit FTS5 phrases; unbalanced quotes cause
//     parse errors).
//  3. Remove the FTS5 boolean/proximity operators OR, AND, NOT and NEAR (case-
//     insensitive, whole-word).  These are always upper-case in FTS5 syntax but
//     we normalise to be safe.
func sanitizeFTSQuery(q string) string {
	q = strings.TrimSpace(q)
	// Strip all double-quote characters to prevent phrase-injection.
	q = strings.ReplaceAll(q, `"`, "")
	// Remove FTS5 operator keywords (whole word, case-insensitive).
	for _, op := range []string{"OR", "AND", "NOT", "NEAR"} {
		// Replace both upper and lower case variants surrounded by word
		// boundaries (spaces or start/end of string).
		q = strings.ReplaceAll(q, " "+op+" ", " ")
		q = strings.ReplaceAll(q, " "+strings.ToLower(op)+" ", " ")
	}
	return strings.TrimSpace(q)
}

// Search performs a prefix-aware full-text search and returns up to limit
// results ordered by exact-name match first, then downloads descending.
func (idx *Index) Search(query string, limit int) ([]PackageEntry, error) {
	if limit <= 0 {
		limit = 10
	}

	// Sanitize user input before embedding in the FTS5 MATCH expression.
	clean := sanitizeFTSQuery(query)
	if clean == "" {
		return nil, nil
	}

	// Wrap in quotes and append * for prefix matching (typeahead behaviour).
	// sqlite fts5 treats "foo"* as a prefix query on the phrase "foo".
	ftsQuery := fmt.Sprintf(`"%s"*`, clean)

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

// UpdateDownloadsBatch updates only the downloads column for existing packages.
func (idx *Index) UpdateDownloadsBatch(entries []PackageEntry) error {
	if len(entries) == 0 {
		return nil
	}

	tx, err := idx.db.Begin()
	if err != nil {
		return fmt.Errorf("search: begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	stmt, err := tx.Prepare(`UPDATE packages_meta SET downloads = ? WHERE lower(name) = lower(?)`)
	if err != nil {
		return fmt.Errorf("search: prepare update downloads: %w", err)
	}
	defer stmt.Close()

	for _, e := range entries {
		if _, err := stmt.Exec(e.Downloads, e.Name); err != nil {
			return fmt.Errorf("search: update downloads %q: %w", e.Name, err)
		}
	}

	return tx.Commit()
}

// Close releases the underlying database connection.
func (idx *Index) Close() error {
	return idx.db.Close()
}
