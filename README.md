# QwikStore

A Redis-like in memory database written in Go — built from scratch as a learning project to understand how Redis works under the hood.

QwikStore implements the RESP protocol, a single-threaded epoll event loop, five core data structures, key expiry, eight eviction policies, and AOF persistence. It is fully compatible with `redis-cli` and any standard Redis client.

---

## Architecture

### Single-threaded epoll event loop

QwikStore follows Redis's exact concurrency model: one OS thread handles everything.

```
┌─────────────────────────────────────────────────┐
│              epoll event loop                   │
│         (pinned to one OS thread)               │
│                                                 │
│  EpollWait()                                    │
│      │                                          │
│      ├── server fd ready → Accept4() new conn   │
│      │       └── register client fd with epoll  │
│      │                                          │
│      └── client fd ready → read RESP command    │
│              └── execute against store          │
│                      └── write response         │
└─────────────────────────────────────────────────┘
```

- The server socket is created with raw Linux syscalls (`unix.Socket`, `unix.Bind`, `unix.Listen`) — bypassing Go's runtime network poller entirely.
- New connections are accepted with `unix.Accept4(..., SOCK_NONBLOCK)` and registered with epoll alongside the server socket.
- `runtime.LockOSThread()` pins the event loop goroutine to a single OS thread so the Go scheduler never moves it.
- Because all I/O and all store access happens sequentially on one thread, **no mutex is needed on any data structure**.

### No `net.Conn`, no Go network poller

All reads and writes use raw `unix.Read` / `unix.Write` syscalls wrapped in a thin `rawConn` type that implements `io.Reader` and `io.Writer`. The RESP parser sits on top of a `bufio.Reader` over this `rawConn`.

---

## Project Structure

```
qwikstore/
├── main.go                  # Entry point, flag parsing, signal handling
├── config/
│   └── config.go            # Server configuration (port, eviction policy, AOF, etc.)
├── server/
│   ├── server.go            # Event loop, accept, command dispatch
│   ├── epoll.go             # Thin epoll wrapper (EpollCreate1, EpollCtl, EpollWait)
│   ├── client.go            # Per-client state (db index, MULTI queue)
│   └── rawconn.go           # Raw fd I/O + server socket creation via syscalls
├── resp/
│   ├── reader.go            # RESP2 protocol parser (inline + array commands)
│   └── writer.go            # RESP2 protocol serializer
├── store/
│   ├── store.go             # Multi-database keyspace, Object type, lazy expiry
│   └── eviction.go          # Eight eviction policies (LRU, LFU, random, TTL)
├── datastructures/
│   ├── list.go              # Doubly linked list (backs Redis Lists)
│   └── skiplist.go          # Probabilistic skip list (backs Redis Sorted Sets)
├── commands/
│   ├── registry.go          # Command registry + response helper functions
│   ├── string.go            # String commands
│   ├── list.go              # List commands
│   ├── hash.go              # Hash commands
│   ├── set.go               # Set commands
│   ├── zset.go              # Sorted set commands
│   ├── key.go               # Key/expiry/scan commands
│   └── server_cmd.go        # Server commands (PING, INFO, MULTI, etc.)
└── persistence/
    └── aof.go               # Append-Only File writer and replay
```

---

## Supported Commands

### Strings

| Command                                         | Description                     |
| ----------------------------------------------- | ------------------------------- |
| `SET key value [EX\|PX\|EXAT\|PXAT n] [NX\|XX]` | Set a key                       |
| `GET key`                                       | Get a key                       |
| `GETSET key value`                              | Set and return old value        |
| `GETDEL key`                                    | Get and delete                  |
| `MSET key value [key value ...]`                | Set multiple keys               |
| `MGET key [key ...]`                            | Get multiple keys               |
| `MSETNX key value [key value ...]`              | Set multiple keys if none exist |
| `SETNX key value`                               | Set if not exists               |
| `SETEX key seconds value`                       | Set with TTL (seconds)          |
| `PSETEX key ms value`                           | Set with TTL (milliseconds)     |
| `INCR / DECR key`                               | Increment / decrement by 1      |
| `INCRBY / DECRBY key n`                         | Increment / decrement by n      |
| `INCRBYFLOAT key n`                             | Increment by float              |
| `APPEND key value`                              | Append to string                |
| `STRLEN key`                                    | String length                   |
| `GETRANGE key start end`                        | Substring                       |
| `SETRANGE key offset value`                     | Overwrite substring             |

### Lists

| Command                                 | Description                                 |
| --------------------------------------- | ------------------------------------------- |
| `LPUSH / RPUSH key value [value ...]`   | Push to head / tail                         |
| `LPUSHX / RPUSHX key value [value ...]` | Push only if key exists                     |
| `LPOP / RPOP key [count]`               | Pop from head / tail                        |
| `LLEN key`                              | List length                                 |
| `LRANGE key start stop`                 | Get range of elements                       |
| `LINDEX key index`                      | Get element by index                        |
| `LSET key index value`                  | Set element by index                        |
| `LINSERT key BEFORE\|AFTER pivot value` | Insert relative to pivot                    |
| `LREM key count value`                  | Remove occurrences                          |
| `LTRIM key start stop`                  | Trim to range                               |
| `RPOPLPUSH src dst`                     | Pop from tail, push to head of another list |
| `LMOVE src dst LEFT\|RIGHT LEFT\|RIGHT` | Move element between lists                  |

### Hashes

| Command                                   | Description               |
| ----------------------------------------- | ------------------------- |
| `HSET key field value [field value ...]`  | Set one or more fields    |
| `HGET key field`                          | Get a field               |
| `HMSET key field value [field value ...]` | Set multiple fields       |
| `HMGET key field [field ...]`             | Get multiple fields       |
| `HDEL key field [field ...]`              | Delete fields             |
| `HEXISTS key field`                       | Check if field exists     |
| `HGETALL key`                             | Get all fields and values |
| `HKEYS / HVALS key`                       | Get all fields / values   |
| `HLEN key`                                | Number of fields          |
| `HINCRBY / HINCRBYFLOAT key field n`      | Increment field value     |
| `HSETNX key field value`                  | Set field if not exists   |

### Sets

| Command                                                    | Description                |
| ---------------------------------------------------------- | -------------------------- |
| `SADD key member [member ...]`                             | Add members                |
| `SREM key member [member ...]`                             | Remove members             |
| `SMEMBERS key`                                             | Get all members            |
| `SISMEMBER key member`                                     | Test membership            |
| `SMISMEMBER key member [member ...]`                       | Test multiple memberships  |
| `SCARD key`                                                | Set cardinality            |
| `SUNION / SINTER / SDIFF key [key ...]`                    | Set operations             |
| `SUNIONSTORE / SINTERSTORE / SDIFFSTORE dst key [key ...]` | Store set operation result |
| `SMOVE src dst member`                                     | Move member between sets   |
| `SPOP key [count]`                                         | Pop random members         |
| `SRANDMEMBER key [count]`                                  | Get random members         |

### Sorted Sets

| Command                                                          | Description                          |
| ---------------------------------------------------------------- | ------------------------------------ |
| `ZADD key [NX\|XX] [GT\|LT] [CH] score member [...]`             | Add members                          |
| `ZREM key member [member ...]`                                   | Remove members                       |
| `ZSCORE key member`                                              | Get score                            |
| `ZINCRBY key increment member`                                   | Increment score                      |
| `ZRANK / ZREVRANK key member`                                    | Get rank (asc / desc)                |
| `ZRANGE key start stop [WITHSCORES]`                             | Get range by rank                    |
| `ZREVRANGE key start stop [WITHSCORES]`                          | Get range by rank (reversed)         |
| `ZRANGEBYSCORE key min max [WITHSCORES] [LIMIT offset count]`    | Range by score                       |
| `ZREVRANGEBYSCORE key max min [WITHSCORES] [LIMIT offset count]` | Reverse range by score               |
| `ZRANGEBYLEX key min max`                                        | Range by lexicographic order         |
| `ZCARD key`                                                      | Number of members                    |
| `ZCOUNT key min max`                                             | Count in score range                 |
| `ZPOPMIN / ZPOPMAX key [count]`                                  | Pop lowest / highest scoring members |

### Keys

| Command                                             | Description                      |
| --------------------------------------------------- | -------------------------------- |
| `DEL key [key ...]`                                 | Delete keys                      |
| `EXISTS key [key ...]`                              | Check existence                  |
| `EXPIRE / PEXPIRE key n`                            | Set TTL (seconds / milliseconds) |
| `EXPIREAT / PEXPIREAT key timestamp`                | Set expiry as Unix timestamp     |
| `TTL / PTTL key`                                    | Get remaining TTL                |
| `PERSIST key`                                       | Remove expiry                    |
| `RENAME / RENAMENX key newkey`                      | Rename key                       |
| `TYPE key`                                          | Get value type                   |
| `KEYS pattern`                                      | Find keys by glob pattern        |
| `SCAN cursor [MATCH pattern] [COUNT n] [TYPE type]` | Cursor-based iteration           |
| `RANDOMKEY`                                         | Random key                       |
| `COPY src dst [REPLACE]`                            | Copy a key                       |
| `OBJECT ENCODING\|IDLETIME\|FREQ\|REFCOUNT key`     | Object introspection             |

### Server

| Command                       | Description                                     |
| ----------------------------- | ----------------------------------------------- |
| `PING [message]`              | Ping the server                                 |
| `ECHO message`                | Echo a message                                  |
| `SELECT index`                | Switch database (0–15)                          |
| `DBSIZE`                      | Number of keys in current DB                    |
| `FLUSHDB`                     | Delete all keys in current DB                   |
| `FLUSHALL`                    | Delete all keys in all DBs                      |
| `INFO [section]`              | Server info (server, memory, clients, keyspace) |
| `CONFIG GET\|SET parameter`   | Read/write config                               |
| `COMMAND [COUNT\|INFO\|DOCS]` | Command introspection                           |
| `SAVE / BGSAVE`               | Trigger persistence                             |
| `DEBUG SLEEP seconds`         | Sleep for testing                               |
| `MULTI / EXEC / DISCARD`      | Transactions                                    |
| `QUIT`                        | Close connection                                |

---

## Key Expiry

Expiry is handled **lazily**: when a key is accessed, it is checked and deleted inline if expired. This is exactly how Redis handles it — no background goroutine scans for expired keys.

```
GET mykey  →  check expiry  →  expired? delete + return nil  :  return value
```

---

## Eviction Policies

When `--maxmemory` is set, QwikStore evicts keys according to the configured policy. Eight policies are supported, matching Redis's API exactly:

| Policy            | Description                                         |
| ----------------- | --------------------------------------------------- |
| `noeviction`      | Return error when memory limit is reached (default) |
| `allkeys-lru`     | Evict least recently used key from all keys         |
| `volatile-lru`    | Evict least recently used key from keys with TTL    |
| `allkeys-lfu`     | Evict least frequently used key from all keys       |
| `volatile-lfu`    | Evict least frequently used key from keys with TTL  |
| `allkeys-random`  | Evict a random key from all keys                    |
| `volatile-random` | Evict a random key from keys with TTL               |
| `volatile-ttl`    | Evict the key closest to expiry                     |

Eviction uses **sampled approximation** (16 candidates per eviction call), the same approach Redis uses to avoid scanning the entire keyspace.

---

## AOF Persistence

When enabled, every write command is appended to the AOF file in RESP format. On startup, the file is replayed to restore state.

Three fsync strategies:

| Strategy   | Behaviour                                                |
| ---------- | -------------------------------------------------------- |
| `always`   | fsync after every write command — safest, slowest        |
| `everysec` | fsync once per second in background — balanced (default) |
| `no`       | Let the OS decide — fastest, least durable               |

---

## Getting Started

### Prerequisites

- Go 1.21+
- Linux (epoll is Linux-only)

### Build

```bash
git clone https://github.com/yourname/qwikstore
cd qwikstore
go build -o qwikstore .
```

### Run

```bash
# Default: listens on 127.0.0.1:6379
./qwikstore

# Custom port
./qwikstore --port 6380

# With AOF persistence
./qwikstore --appendonly true --appendfilename mydata.aof

# With memory limit and LRU eviction
./qwikstore --maxmemory 134217728 --maxmemory-policy allkeys-lru
```

### Connect with redis-cli

```bash
redis-cli PING
# PONG

redis-cli SET hello world
redis-cli GET hello
# world

redis-cli ZADD leaderboard 100 alice 200 bob
redis-cli ZRANGE leaderboard 0 -1 WITHSCORES
```

### All flags

| Flag                 | Default          | Description              |
| -------------------- | ---------------- | ------------------------ |
| `--host`             | `127.0.0.1`      | Bind address             |
| `--port`             | `6379`           | Port                     |
| `--maxmemory`        | `0` (unlimited)  | Max memory in bytes      |
| `--maxmemory-policy` | `noeviction`     | Eviction policy          |
| `--appendonly`       | `false`          | Enable AOF persistence   |
| `--appendfilename`   | `appendonly.aof` | AOF file path            |
| `--appendfsync`      | `everysec`       | AOF fsync strategy       |
| `--maxclients`       | `10000`          | Max simultaneous clients |
| `--databases`        | `16`             | Number of databases      |

---

## Data Structures

### Doubly Linked List — `datastructures/list.go`

Backs Redis List objects. O(1) push/pop from both ends. Range queries walk from whichever end is closer to the target index.

### Skip List — `datastructures/skiplist.go`

Backs Redis Sorted Set objects. A probabilistic data structure that maintains elements ordered by score, with O(log N) insert, delete, and rank queries. Each node has a random number of forward pointers (up to 32 levels), allowing the list to be traversed in large jumps. Paired with a `map[string]float64` for O(1) score lookups by member name.

---

## License

MIT
