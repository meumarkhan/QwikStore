package store

import (
	"math/rand"
	"path/filepath"
	"strings"
	"time"
	"qwikstore/datastructures"
)

// ValueType enumerates the Redis data types.
type ValueType int

const (
	TypeString ValueType = iota
	TypeList
	TypeHash
	TypeSet
	TypeZSet
)

func (t ValueType) String() string {
	switch t {
	case TypeString:
		return "string"
	case TypeList:
		return "list"
	case TypeHash:
		return "hash"
	case TypeSet:
		return "set"
	case TypeZSet:
		return "zset"
	}
	return "none"
}

// Object holds a Redis value along with metadata.
type Object struct {
	Type       ValueType
	Value      interface{} // string | *datastructures.List | map[string]string | map[string]struct{} | *ZSet
	Expiry     time.Time
	HasExpiry  bool
	LastAccess int64  // unix nano, for LRU
	AccessFreq uint32 // for LFU
}

func (o *Object) Touch() {
	o.LastAccess = time.Now().UnixNano()
	if o.AccessFreq < 1<<24 {
		o.AccessFreq++
	}
}

func (o *Object) IsExpired() bool {
	return o.HasExpiry && time.Now().After(o.Expiry)
}

// ZSet wraps a skiplist + member->score map.
type ZSet struct {
	sl      *datastructures.Skiplist
	members map[string]float64
}

func NewZSet() *ZSet {
	return &ZSet{
		sl:      datastructures.NewSkiplist(),
		members: make(map[string]float64),
	}
}

func (z *ZSet) Add(score float64, member string) bool {
	if old, ok := z.members[member]; ok {
		z.sl.Delete(old, member)
	} else {
		z.members[member] = score
		z.sl.Insert(score, member)
		return true
	}
	z.members[member] = score
	z.sl.Insert(score, member)
	return false
}

func (z *ZSet) Remove(member string) bool {
	score, ok := z.members[member]
	if !ok {
		return false
	}
	z.sl.Delete(score, member)
	delete(z.members, member)
	return true
}

func (z *ZSet) Score(member string) (float64, bool) {
	s, ok := z.members[member]
	return s, ok
}

func (z *ZSet) Rank(member string) (int, bool) {
	score, ok := z.members[member]
	if !ok {
		return 0, false
	}
	r := z.sl.GetRank(score, member)
	return r - 1, true
}

func (z *ZSet) RevRank(member string) (int, bool) {
	rank, ok := z.Rank(member)
	if !ok {
		return 0, false
	}
	return z.sl.Len() - 1 - rank, true
}

func (z *ZSet) Len() int                              { return z.sl.Len() }
func (z *ZSet) Skiplist() *datastructures.Skiplist    { return z.sl }
func (z *ZSet) Members() map[string]float64           { return z.members }

// DB is a single Redis database (keyspace).
// It has NO mutex — all access must come from the single event-loop goroutine.
type DB struct {
	data map[string]*Object
}

func newDB() *DB {
	return &DB{data: make(map[string]*Object)}
}

// Store is the top-level data store holding multiple databases.
type Store struct {
	dbs []*DB
}

func New(numDBs int) *Store {
	dbs := make([]*DB, numDBs)
	for i := range dbs {
		dbs[i] = newDB()
	}
	return &Store{dbs: dbs}
}

func (s *Store) DB(idx int) *DB {
	if idx < 0 || idx >= len(s.dbs) {
		return nil
	}
	return s.dbs[idx]
}

func (s *Store) NumDBs() int { return len(s.dbs) }

// --- DB-level operations (all unsynchronized — safe because single-threaded) ---

func (db *DB) Get(key string) (*Object, bool) {
	obj, ok := db.data[key]
	if !ok {
		return nil, false
	}
	if obj.IsExpired() {
		delete(db.data, key)
		return nil, false
	}
	obj.Touch()
	return obj, true
}

func (db *DB) Set(key string, obj *Object) {
	obj.LastAccess = time.Now().UnixNano()
	db.data[key] = obj
}

func (db *DB) Del(keys ...string) int {
	count := 0
	for _, k := range keys {
		if _, ok := db.data[k]; ok {
			delete(db.data, k)
			count++
		}
	}
	return count
}

func (db *DB) Exists(keys ...string) int {
	count := 0
	for _, k := range keys {
		obj, ok := db.data[k]
		if ok && !obj.IsExpired() {
			count++
		}
	}
	return count
}

func (db *DB) SetExpiry(key string, t time.Time) bool {
	obj, ok := db.data[key]
	if !ok || obj.IsExpired() {
		return false
	}
	obj.HasExpiry = true
	obj.Expiry = t
	return true
}

func (db *DB) RemoveExpiry(key string) bool {
	obj, ok := db.data[key]
	if !ok {
		return false
	}
	obj.HasExpiry = false
	return true
}

func (db *DB) TTL(key string) time.Duration {
	obj, ok := db.data[key]
	if !ok || obj.IsExpired() {
		return -2
	}
	if !obj.HasExpiry {
		return -1
	}
	d := time.Until(obj.Expiry)
	if d < 0 {
		return -2
	}
	return d
}

func (db *DB) Type(key string) string {
	obj, ok := db.Get(key)
	if !ok {
		return "none"
	}
	return obj.Type.String()
}

func (db *DB) Keys(pattern string) []string {
	var keys []string
	for k, obj := range db.data {
		if obj.IsExpired() {
			continue
		}
		if matchGlob(pattern, k) {
			keys = append(keys, k)
		}
	}
	return keys
}

func (db *DB) Size() int {
	count := 0
	for _, obj := range db.data {
		if !obj.IsExpired() {
			count++
		}
	}
	return count
}

func (db *DB) Flush() {
	db.data = make(map[string]*Object)
}

func (db *DB) RandomKey() string {
	var keys []string
	for k, obj := range db.data {
		if !obj.IsExpired() {
			keys = append(keys, k)
		}
	}
	if len(keys) == 0 {
		return ""
	}
	return keys[rand.Intn(len(keys))]
}

func (db *DB) Rename(src, dst string) bool {
	obj, ok := db.data[src]
	if !ok || obj.IsExpired() {
		return false
	}
	delete(db.data, src)
	db.data[dst] = obj
	return true
}

func (db *DB) Scan(cursor, count int, pattern, typFilter string) (int, []string) {
	allKeys := make([]string, 0, len(db.data))
	for k, obj := range db.data {
		if !obj.IsExpired() {
			allKeys = append(allKeys, k)
		}
	}
	if count <= 0 {
		count = 10
	}
	total := len(allKeys)
	if total == 0 {
		return 0, nil
	}
	start := cursor % total
	var result []string
	for i := 0; i < total && len(result) < count; i++ {
		idx := (start + i) % total
		k := allKeys[idx]
		if pattern != "" && !matchGlob(pattern, k) {
			continue
		}
		if typFilter != "" {
			obj := db.data[k]
			if obj == nil || obj.Type.String() != typFilter {
				continue
			}
		}
		result = append(result, k)
	}
	nextCursor := 0
	if len(result) == count {
		nextCursor = (start + count) % total
	}
	return nextCursor, result
}

func matchGlob(pattern, s string) bool {
	if pattern == "*" {
		return true
	}
	matched, err := filepath.Match(strings.ToLower(pattern), strings.ToLower(s))
	if err != nil {
		return false
	}
	return matched
}
