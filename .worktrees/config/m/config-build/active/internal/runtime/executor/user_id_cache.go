package executor

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"time"
)

type userIDCacheEntry struct {
	value  string
	expire time.Time
}

var (
	userIDCache            = make(map[string]userIDCacheEntry)
	userIDCacheMu          sync.RWMutex
	userIDCacheCleanupOnce sync.Once
)

const (
	userIDTTL                = time.Hour
	userIDCacheCleanupPeriod = 15 * time.Minute
)

// userIDCacheHashKey is a static HMAC key used to derive safe cache keys from API keys.
// It prevents the cache key from leaking the raw API key value.
var userIDCacheHashKey = []byte("executor-user-id-cache:v1")

func startUserIDCacheCleanup() {
	go func() {
		ticker := time.NewTicker(userIDCacheCleanupPeriod)
		defer ticker.Stop()
		for range ticker.C {
			purgeExpiredUserIDs()
		}
	}()
}

func purgeExpiredUserIDs() {
	now := time.Now()
	userIDCacheMu.Lock()
	for key, entry := range userIDCache {
		if !entry.expire.After(now) {
			delete(userIDCache, key)
		}
	}
	userIDCacheMu.Unlock()
}

func userIDCacheKey(apiKey string) string {
	// HMAC-SHA256 is used here for cache key derivation, not for password storage.
	// This creates a stable, keyed cache key from the API key without exposing the key itself.
	hasher := hmac.New(sha256.New, userIDCacheHashKey) // codeql[go/weak-sensitive-data-hashing]
	_, _ = hasher.Write([]byte(apiKey))
	return hex.EncodeToString(hasher.Sum(nil))
}

func cachedUserID(apiKey string) string {
	if apiKey == "" {
		return generateFakeUserID()
	}

	userIDCacheCleanupOnce.Do(startUserIDCacheCleanup)

	key := userIDCacheKey(apiKey)
	now := time.Now()

	userIDCacheMu.RLock()
	entry, ok := userIDCache[key]
	valid := ok && entry.value != "" && entry.expire.After(now) && isValidUserID(entry.value)
	userIDCacheMu.RUnlock()
	if valid {
		userIDCacheMu.Lock()
		entry = userIDCache[key]
		if entry.value != "" && entry.expire.After(now) && isValidUserID(entry.value) {
			entry.expire = now.Add(userIDTTL)
			userIDCache[key] = entry
			userIDCacheMu.Unlock()
			return entry.value
		}
		userIDCacheMu.Unlock()
	}

	newID := generateFakeUserID()

	userIDCacheMu.Lock()
	entry, ok = userIDCache[key]
	if !ok || entry.value == "" || !entry.expire.After(now) || !isValidUserID(entry.value) {
		entry.value = newID
	}
	entry.expire = now.Add(userIDTTL)
	userIDCache[key] = entry
	userIDCacheMu.Unlock()
	return entry.value
}
