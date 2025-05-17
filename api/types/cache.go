package types

// Cache defines the interface for cache storage
// Provides key-value based storage and retrieval functionality with expiration support
// Implementation classes must ensure thread safety
type Cache interface {
	// Set stores a key-value pair in cache with optional expiration
	// Parameters:
	//   - key: cache key (string)
	//   - value: value to store (interface{})
	//   - ttl: time-to-live duration string (e.g. "10m", "1h")
	// Returns:
	//   - error: returns error if ttl format is invalid
	// Note: If ttl is 0 or empty string, the item will never expire
	Set(key string, value interface{}, ttl string) error
	// Get retrieves a value from cache by key
	// Parameters:
	//   - key: cache key to lookup (string)
	// Returns:
	//   - interface{}: stored value, nil if not exists or expired
	Get(key string) interface{}
	// Has checks if a key exists in cache
	// Parameters:
	//   - key: cache key to check (string)
	// Returns:
	//   - bool: true if key exists and not expired, false otherwise
	Has(key string) bool
	// Delete removes a cache item by key
	// Parameters:
	//   - key: cache key to delete (string)
	// Returns:
	//   - error: current implementation always returns nil
	Delete(key string) error
	// DeleteByPrefix removes all cache items with the specified prefix
	// Parameters:
	//   - prefix: key prefix to match (string)
	// Returns:
	//   - error: current implementation always returns nil
	DeleteByPrefix(prefix string) error

	// GetByPrefix retrieves all values with keys matching the specified prefix
	// Parameters:
	//   - prefix: key prefix to match (string)
	// Returns:
	//   - map[string]interface{}: map of matching key-value pairs
	GetByPrefix(prefix string) map[string]interface{}
}
