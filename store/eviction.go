package store

import (
	"math"
	"math/rand"
	"time"
	"qwikstore/config"
)

const evictionSampleSize = 16

// Evict removes one key from db according to policy.
// Called from the single event-loop goroutine — no locking needed.
func Evict(db *DB, policy config.EvictionPolicy) bool {
	switch policy {
	case config.EvictAllKeysLRU:
		return evictLRU(db, false)
	case config.EvictVolatileLRU:
		return evictLRU(db, true)
	case config.EvictAllKeysLFU:
		return evictLFU(db, false)
	case config.EvictVolatileLFU:
		return evictLFU(db, true)
	case config.EvictAllKeysRandom:
		return evictRandom(db, false)
	case config.EvictVolatileRandom:
		return evictRandom(db, true)
	case config.EvictVolatileTTL:
		return evictByTTL(db)
	}
	return false
}

func evictLRU(db *DB, volatileOnly bool) bool {
	obj, key := sampleCandidate(db, volatileOnly, func(a, b *Object) bool {
		return a.LastAccess < b.LastAccess
	})
	if obj == nil {
		return false
	}
	delete(db.data, key)
	return true
}

func evictLFU(db *DB, volatileOnly bool) bool {
	obj, key := sampleCandidate(db, volatileOnly, func(a, b *Object) bool {
		return a.AccessFreq < b.AccessFreq
	})
	if obj == nil {
		return false
	}
	delete(db.data, key)
	return true
}

func evictRandom(db *DB, volatileOnly bool) bool {
	var candidates []string
	for k, obj := range db.data {
		if volatileOnly && !obj.HasExpiry {
			continue
		}
		candidates = append(candidates, k)
		if len(candidates) >= evictionSampleSize {
			break
		}
	}
	if len(candidates) == 0 {
		return false
	}
	delete(db.data, candidates[rand.Intn(len(candidates))])
	return true
}

func evictByTTL(db *DB) bool {
	var bestKey string
	bestTTL := time.Duration(math.MaxInt64)
	sampled := 0
	for k, obj := range db.data {
		if !obj.HasExpiry {
			continue
		}
		ttl := time.Until(obj.Expiry)
		if ttl < bestTTL {
			bestTTL = ttl
			bestKey = k
		}
		sampled++
		if sampled >= evictionSampleSize {
			break
		}
	}
	if bestKey == "" {
		return false
	}
	delete(db.data, bestKey)
	return true
}

func sampleCandidate(db *DB, volatileOnly bool, worse func(a, b *Object) bool) (*Object, string) {
	var bestObj *Object
	var bestKey string
	sampled := 0
	for k, obj := range db.data {
		if volatileOnly && !obj.HasExpiry {
			continue
		}
		if bestObj == nil || worse(obj, bestObj) {
			bestObj = obj
			bestKey = k
		}
		sampled++
		if sampled >= evictionSampleSize {
			break
		}
	}
	return bestObj, bestKey
}
