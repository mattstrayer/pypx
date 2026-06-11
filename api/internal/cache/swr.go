package cache

import (
	"log"
	"time"

	"golang.org/x/sync/singleflight"
)

// refreshGroup dedupes concurrent background refreshes per cache key.
var refreshGroup singleflight.Group

// RefreshInBackground runs fetch in a goroutine and stores its result under
// key with ttl. Concurrent calls for the same key while a refresh is in
// flight are coalesced into one. Errors are logged, never returned — SWR
// refreshes are best-effort.
func RefreshInBackground(c Cacher, key string, ttl time.Duration, fetch func() ([]byte, error)) {
	go func() {
		_, _, _ = refreshGroup.Do(key, func() (any, error) {
			data, err := fetch()
			if err != nil {
				log.Printf("background refresh failed for %q: %v", key, err)
				return nil, err
			}
			if err := c.Set(key, data, ttl); err != nil {
				log.Printf("background refresh cache set failed for %q: %v", key, err)
			}
			return nil, nil
		})
	}()
}
