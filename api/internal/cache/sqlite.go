package cache

import (
	"database/sql"
	"time"

	_ "modernc.org/sqlite"
)

// Cacher is the interface that both Cache and MemoryCache implement.
type Cacher interface {
	Get(key string, ttl time.Duration) (data []byte, fresh bool, err error)
	Set(key string, value []byte, ttl time.Duration) error
	Delete(key string) error
	Close() error
}

// Cache is a SQLite-backed key/value cache with TTL-based freshness.
type Cache struct {
	db *sql.DB
}

// New opens (or creates) a SQLite database at dsn and initialises the cache
// table. Use ":memory:" for an ephemeral in-process cache.
func New(dsn string) (*Cache, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}

	if _, err := db.Exec(`PRAGMA journal_mode=WAL`); err != nil {
		_ = db.Close()
		return nil, err
	}
	if _, err := db.Exec(`PRAGMA synchronous=NORMAL`); err != nil {
		_ = db.Close()
		return nil, err
	}
	if _, err := db.Exec(`PRAGMA busy_timeout=5000`); err != nil {
		_ = db.Close()
		return nil, err
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS cache (
			key        TEXT    PRIMARY KEY,
			value      BLOB    NOT NULL,
			created_at INTEGER NOT NULL
		)
	`)
	if err != nil {
		_ = db.Close()
		return nil, err
	}

	return &Cache{db: db}, nil
}

// Set stores value under key with the current timestamp. Any existing entry for the same key is replaced.
// The ttl parameter is accepted for interface compatibility but ignored; TTL is enforced
// only during Get calls.
func (c *Cache) Set(key string, value []byte, _ time.Duration) error {
	_, err := c.db.Exec(
		`INSERT OR REPLACE INTO cache (key, value, created_at) VALUES (?, ?, ?)`,
		key, value, time.Now().Unix(),
	)
	return err
}

// getWithTime retrieves the value stored under key along with its original
// created_at timestamp in a single atomic row scan. A zero createdAt means
// the key was not found.
func (c *Cache) getWithTime(key string, ttl time.Duration) (data []byte, fresh bool, createdAt time.Time, err error) {
	var value []byte
	var tsUnix int64

	row := c.db.QueryRow(
		`SELECT value, created_at FROM cache WHERE key = ?`, key,
	)
	if err = row.Scan(&value, &tsUnix); err != nil {
		if err == sql.ErrNoRows {
			return nil, false, time.Time{}, nil
		}
		return nil, false, time.Time{}, err
	}

	t := time.Unix(tsUnix, 0)
	age := time.Since(t)
	return value, age < ttl, t, nil
}

// Get retrieves the value stored under key.
//
// If the key does not exist, data is nil and err is nil.
// If the key exists but is older than ttl, data is returned with fresh=false
// (stale-while-revalidate: the caller can serve the stale data and refresh in
// the background).
// If the key exists and is within ttl, fresh=true.
func (c *Cache) Get(key string, ttl time.Duration) (data []byte, fresh bool, err error) {
	data, fresh, _, err = c.getWithTime(key, ttl)
	return
}

// Delete removes a single cache entry. Missing keys are not an error.
func (c *Cache) Delete(key string) error {
	_, err := c.db.Exec(`DELETE FROM cache WHERE key = ?`, key)
	return err
}

// Close releases the underlying database connection.
func (c *Cache) Close() error {
	return c.db.Close()
}

// ListPackageNames returns the name portion of all cache keys matching
// the "pkg:{name}" pattern, i.e. every package that has been fetched and
// cached at least once.
func (c *Cache) ListPackageNames() ([]string, error) {
	rows, err := c.db.Query(
		`SELECT SUBSTR(key, 5) FROM cache WHERE key LIKE 'pkg:%'`,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	return names, rows.Err()
}
