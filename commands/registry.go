package commands

import (
	"strings"
	"qwikstore/persistence"
	"qwikstore/resp"
	"qwikstore/store"
)

// CommandFunc is the signature every command handler must satisfy.
type CommandFunc func(ctx *Context) *resp.Value

// Context carries everything a command needs to execute.
type Context struct {
	DB   *store.DB
	Args []*resp.Value // does NOT include the command name itself
	AOF  *persistence.AOF
	// Raw args as strings (lazily populated by ArgStr)
	rawArgs []string
}

// Arg returns the i-th argument as a resp.Value (0-indexed).
func (c *Context) Arg(i int) *resp.Value {
	if i >= len(c.Args) {
		return nil
	}
	return c.Args[i]
}

// ArgStr returns the i-th argument as a string, or "" if out of range.
func (c *Context) ArgStr(i int) string {
	a := c.Arg(i)
	if a == nil {
		return ""
	}
	return a.Str
}

// NArgs returns the number of arguments (excluding cmd name).
func (c *Context) NArgs() int {
	return len(c.Args)
}

// AllArgStr returns all args as a string slice.
func (c *Context) AllArgStr() []string {
	s := make([]string, len(c.Args))
	for i, a := range c.Args {
		s[i] = a.Str
	}
	return s
}

// Registry maps command names to their handlers.
type Registry struct {
	cmds map[string]CommandFunc
}

func NewRegistry() *Registry {
	r := &Registry{cmds: make(map[string]CommandFunc)}
	r.registerAll()
	return r
}

func (r *Registry) Register(name string, fn CommandFunc) {
	r.cmds[strings.ToUpper(name)] = fn
}

func (r *Registry) Get(name string) (CommandFunc, bool) {
	fn, ok := r.cmds[strings.ToUpper(name)]
	return fn, ok
}

func (r *Registry) Commands() []string {
	names := make([]string, 0, len(r.cmds))
	for n := range r.cmds {
		names = append(names, n)
	}
	return names
}

func (r *Registry) registerAll() {
	// String
	r.Register("SET", cmdSet)
	r.Register("GET", cmdGet)
	r.Register("GETSET", cmdGetSet)
	r.Register("MSET", cmdMSet)
	r.Register("MGET", cmdMGet)
	r.Register("MSETNX", cmdMSetNX)
	r.Register("SETNX", cmdSetNX)
	r.Register("SETEX", cmdSetEX)
	r.Register("PSETEX", cmdPSetEX)
	r.Register("INCR", cmdIncr)
	r.Register("INCRBY", cmdIncrBy)
	r.Register("DECR", cmdDecr)
	r.Register("DECRBY", cmdDecrBy)
	r.Register("INCRBYFLOAT", cmdIncrByFloat)
	r.Register("APPEND", cmdAppend)
	r.Register("STRLEN", cmdStrLen)
	r.Register("GETRANGE", cmdGetRange)
	r.Register("SUBSTR", cmdGetRange) // alias
	r.Register("SETRANGE", cmdSetRange)
	r.Register("GETDEL", cmdGetDel)

	// List
	r.Register("LPUSH", cmdLPush)
	r.Register("RPUSH", cmdRPush)
	r.Register("LPUSHX", cmdLPushX)
	r.Register("RPUSHX", cmdRPushX)
	r.Register("LPOP", cmdLPop)
	r.Register("RPOP", cmdRPop)
	r.Register("LLEN", cmdLLen)
	r.Register("LRANGE", cmdLRange)
	r.Register("LINDEX", cmdLIndex)
	r.Register("LSET", cmdLSet)
	r.Register("LINSERT", cmdLInsert)
	r.Register("LREM", cmdLRem)
	r.Register("LTRIM", cmdLTrim)
	r.Register("RPOPLPUSH", cmdRPopLPush)
	r.Register("LMOVE", cmdLMove)

	// Hash
	r.Register("HSET", cmdHSet)
	r.Register("HGET", cmdHGet)
	r.Register("HMSET", cmdHMSet)
	r.Register("HMGET", cmdHMGet)
	r.Register("HDEL", cmdHDel)
	r.Register("HEXISTS", cmdHExists)
	r.Register("HGETALL", cmdHGetAll)
	r.Register("HKEYS", cmdHKeys)
	r.Register("HVALS", cmdHVals)
	r.Register("HLEN", cmdHLen)
	r.Register("HINCRBY", cmdHIncrBy)
	r.Register("HINCRBYFLOAT", cmdHIncrByFloat)
	r.Register("HSETNX", cmdHSetNX)

	// Set
	r.Register("SADD", cmdSAdd)
	r.Register("SREM", cmdSRem)
	r.Register("SMEMBERS", cmdSMembers)
	r.Register("SISMEMBER", cmdSIsMember)
	r.Register("SMISMEMBER", cmdSMIsMember)
	r.Register("SCARD", cmdSCard)
	r.Register("SUNION", cmdSUnion)
	r.Register("SINTER", cmdSInter)
	r.Register("SDIFF", cmdSDiff)
	r.Register("SUNIONSTORE", cmdSUnionStore)
	r.Register("SINTERSTORE", cmdSInterStore)
	r.Register("SDIFFSTORE", cmdSDiffStore)
	r.Register("SMOVE", cmdSMove)
	r.Register("SPOP", cmdSPop)
	r.Register("SRANDMEMBER", cmdSRandMember)

	// Sorted Set
	r.Register("ZADD", cmdZAdd)
	r.Register("ZREM", cmdZRem)
	r.Register("ZSCORE", cmdZScore)
	r.Register("ZINCRBY", cmdZIncrBy)
	r.Register("ZRANK", cmdZRank)
	r.Register("ZREVRANK", cmdZRevRank)
	r.Register("ZRANGE", cmdZRange)
	r.Register("ZREVRANGE", cmdZRevRange)
	r.Register("ZRANGEBYSCORE", cmdZRangeByScore)
	r.Register("ZREVRANGEBYSCORE", cmdZRevRangeByScore)
	r.Register("ZCARD", cmdZCard)
	r.Register("ZCOUNT", cmdZCount)
	r.Register("ZPOPMIN", cmdZPopMin)
	r.Register("ZPOPMAX", cmdZPopMax)
	r.Register("ZRANGEBYLEX", cmdZRangeByLex)

	// Key
	r.Register("DEL", cmdDel)
	r.Register("UNLINK", cmdDel) // alias (async del not needed for learning project)
	r.Register("EXISTS", cmdExists)
	r.Register("EXPIRE", cmdExpire)
	r.Register("EXPIREAT", cmdExpireAt)
	r.Register("PEXPIRE", cmdPExpire)
	r.Register("PEXPIREAT", cmdPExpireAt)
	r.Register("TTL", cmdTTL)
	r.Register("PTTL", cmdPTTL)
	r.Register("PERSIST", cmdPersist)
	r.Register("RENAME", cmdRename)
	r.Register("RENAMENX", cmdRenameNX)
	r.Register("TYPE", cmdType)
	r.Register("KEYS", cmdKeys)
	r.Register("SCAN", cmdScan)
	r.Register("RANDOMKEY", cmdRandomKey)
	r.Register("COPY", cmdCopy)
	r.Register("OBJECT", cmdObject)

	// Server
	r.Register("PING", cmdPing)
	r.Register("ECHO", cmdEcho)
	r.Register("SELECT", cmdSelect)
	r.Register("DBSIZE", cmdDBSize)
	r.Register("FLUSHDB", cmdFlushDB)
	r.Register("FLUSHALL", cmdFlushAll)
	r.Register("INFO", cmdInfo)
	r.Register("CONFIG", cmdConfig)
	r.Register("COMMAND", cmdCommand)
	r.Register("DEBUG", cmdDebug)
	r.Register("SAVE", cmdSave)
	r.Register("BGSAVE", cmdBgSave)
	r.Register("LASTSAVE", cmdLastSave)
}

// --- Response helpers ---

func okResp() *resp.Value {
	return &resp.Value{Type: resp.TypeSimpleString, Str: "OK"}
}

func errResp(msg string) *resp.Value {
	return &resp.Value{Type: resp.TypeError, Str: msg}
}

func intResp(n int64) *resp.Value {
	return &resp.Value{Type: resp.TypeInteger, Integer: n}
}

func bulkResp(s string) *resp.Value {
	return &resp.Value{Type: resp.TypeBulkString, Str: s}
}

func nullBulkResp() *resp.Value {
	return &resp.Value{Type: resp.TypeBulkString, IsNull: true}
}

func nullArrayResp() *resp.Value {
	return &resp.Value{Type: resp.TypeArray, IsNull: true}
}

func arrayResp(items ...*resp.Value) *resp.Value {
	return &resp.Value{Type: resp.TypeArray, Array: items}
}

func stringsToArray(strs []string) *resp.Value {
	arr := make([]*resp.Value, len(strs))
	for i, s := range strs {
		arr[i] = bulkResp(s)
	}
	return &resp.Value{Type: resp.TypeArray, Array: arr}
}

func wrongArgCount(cmd string) *resp.Value {
	return errResp("ERR wrong number of arguments for '" + strings.ToLower(cmd) + "' command")
}

func wrongTypeErr() *resp.Value {
	return errResp("WRONGTYPE Operation against a key holding the wrong kind of value")
}
