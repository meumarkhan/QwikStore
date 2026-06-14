package server

import (
	"fmt"
	"io"
	"log"
	"runtime"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"

	"qwikstore/commands"
	"qwikstore/config"
	"qwikstore/persistence"
	"qwikstore/resp"
	"qwikstore/store"
)

// Server is the main qwikstore server.
//
// Concurrency model: exactly ONE goroutine (pinned to one OS thread via
// runtime.LockOSThread) runs the epoll event loop. That goroutine:
//   - accepts new connections
//   - reads and parses RESP commands
//   - executes commands against the store
//   - writes responses
//
// Because no other goroutine touches the store or the client map, no mutex
// is required for either. The only background goroutine is AOF bgSync, which
// holds its own internal mutex and only does OS-level file I/O.
type Server struct {
	cfg      *config.Config
	store    *store.Store
	registry *commands.Registry
	aof      *persistence.AOF
	epoll    *EPoll
	serverFd int
	clients  map[int]*Client // fd → client; only accessed from the event loop
	done     chan struct{}
}

func New(cfg *config.Config) (*Server, error) {
	s := &Server{
		cfg:      cfg,
		store:    store.New(cfg.Databases),
		registry: commands.NewRegistry(),
		clients:  make(map[int]*Client),
		done:     make(chan struct{}),
	}

	if cfg.AOFEnabled {
		if err := s.replayAOF(); err != nil {
			return nil, fmt.Errorf("AOF replay: %w", err)
		}
		aof, err := persistence.NewAOF(cfg.AOFFilename, cfg.AOFSync)
		if err != nil {
			return nil, fmt.Errorf("AOF open: %w", err)
		}
		s.aof = aof
	}
	return s, nil
}

func (s *Server) replayAOF() error {
	log.Printf("replaying AOF from %s", s.cfg.AOFFilename)
	db := s.store.DB(0)
	return persistence.Replay(s.cfg.AOFFilename, func(cmd string, args []string) error {
		vals := make([]*resp.Value, len(args))
		for i, a := range args {
			vals[i] = &resp.Value{Type: resp.TypeBulkString, Str: a}
		}
		fn, ok := s.registry.Get(cmd)
		if !ok {
			return nil
		}
		fn(&commands.Context{DB: db, Args: vals})
		return nil
	})
}

// Listen creates the server socket, registers it with epoll, locks the current
// goroutine to its OS thread, then runs the event loop forever.
func (s *Server) Listen() error {
	fd, err := createServerSocket(s.cfg.Host, s.cfg.Port)
	if err != nil {
		return err
	}
	s.serverFd = fd

	ep, err := NewEPoll()
	if err != nil {
		unix.Close(fd)
		return err
	}
	s.epoll = ep

	if err := ep.Add(fd); err != nil {
		ep.Close()
		unix.Close(fd)
		return fmt.Errorf("epoll add server fd: %w", err)
	}

	log.Printf("qwikstore listening on %s (single-threaded epoll)", s.cfg.Addr())

	// Pin this goroutine to a single OS thread. From this point on, no other
	// goroutine runs on this thread, and we never spawn any that touch the store.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	s.eventLoop()
	return nil
}

func (s *Server) eventLoop() {
	for {
		fds, err := s.epoll.Wait(256)
		if err != nil {
			log.Printf("epoll wait: %v", err)
			return
		}
		for _, fd := range fds {
			if fd == s.serverFd {
				s.acceptConn()
			} else {
				if err := s.handleClient(fd); err != nil {
					s.removeClient(fd)
				}
			}
		}
	}
}

func (s *Server) acceptConn() {
	// Accept as many connections as are pending (server fd is non-blocking).
	for {
		connFd, _, err := unix.Accept4(s.serverFd, unix.SOCK_NONBLOCK|unix.SOCK_CLOEXEC)
		if err != nil {
			if err == unix.EAGAIN || err == unix.EWOULDBLOCK {
				break // no more pending connections
			}
			log.Printf("accept: %v", err)
			break
		}
		if err := s.epoll.Add(connFd); err != nil {
			log.Printf("epoll add client fd %d: %v", connFd, err)
			unix.Close(connFd)
			continue
		}
		s.clients[connFd] = NewClient(connFd)
	}
}

func (s *Server) removeClient(fd int) {
	s.epoll.Remove(fd)
	if c, ok := s.clients[fd]; ok {
		c.Close()
		delete(s.clients, fd)
	}
}

func (s *Server) handleClient(fd int) error {
	c, ok := s.clients[fd]
	if !ok {
		return fmt.Errorf("unknown fd %d", fd)
	}

	value, err := c.Reader().ReadValue()
	if err != nil {
		if err == io.EOF {
			return io.EOF
		}
		return err
	}

	cmdName, args, err := value.ToCommand()
	if err != nil {
		c.Writer().WriteError("ERR Protocol error")
		c.Writer().Flush()
		return nil
	}
	cmdName = strings.ToUpper(cmdName)

	switch cmdName {
	case "MULTI":
		if c.InMulti() {
			c.Writer().WriteError("ERR MULTI calls can not be nested")
		} else {
			c.StartMulti()
			c.Writer().WriteSimpleString("OK")
		}
		c.Writer().Flush()
		return nil
	case "DISCARD":
		if !c.InMulti() {
			c.Writer().WriteError("ERR DISCARD without MULTI")
		} else {
			c.DiscardMulti()
			c.Writer().WriteSimpleString("OK")
		}
		c.Writer().Flush()
		return nil
	case "EXEC":
		if !c.InMulti() {
			c.Writer().WriteError("ERR EXEC without MULTI")
			c.Writer().Flush()
			return nil
		}
		return s.execTransaction(c)
	case "QUIT":
		c.Writer().WriteSimpleString("OK")
		c.Writer().Flush()
		return io.EOF
	}

	if c.InMulti() {
		c.Enqueue(cmdName, args)
		c.Writer().WriteSimpleString("QUEUED")
		c.Writer().Flush()
		return nil
	}

	result := s.execute(c, cmdName, args)
	if err := c.Writer().WriteValue(result); err != nil {
		return err
	}
	return c.Writer().Flush()
}

func (s *Server) execute(c *Client, cmdName string, args []*resp.Value) *resp.Value {
	if cmdName == "SELECT" {
		if len(args) != 1 {
			return &resp.Value{Type: resp.TypeError, Str: "ERR wrong number of arguments for 'select' command"}
		}
		idx, err := strconv.Atoi(args[0].Str)
		if err != nil || idx < 0 || idx >= s.cfg.Databases {
			return &resp.Value{Type: resp.TypeError, Str: "ERR DB index is out of range"}
		}
		c.SetDBIndex(idx)
		return &resp.Value{Type: resp.TypeSimpleString, Str: "OK"}
	}

	if cmdName == "FLUSHALL" {
		for i := 0; i < s.store.NumDBs(); i++ {
			s.store.DB(i).Flush()
		}
		return &resp.Value{Type: resp.TypeSimpleString, Str: "OK"}
	}

	fn, ok := s.registry.Get(cmdName)
	if !ok {
		return &resp.Value{Type: resp.TypeError, Str: fmt.Sprintf("ERR unknown command '%s'", strings.ToLower(cmdName))}
	}

	ctx := &commands.Context{
		DB:   s.store.DB(c.DBIndex()),
		Args: args,
		AOF:  s.aof,
	}
	result := fn(ctx)

	if s.aof != nil && isWriteCommand(cmdName) {
		all := make([]string, 0, len(args)+1)
		all = append(all, cmdName)
		for _, a := range args {
			all = append(all, a.Str)
		}
		s.aof.Write(all)
	}
	return result
}

func (s *Server) execTransaction(c *Client) error {
	queue := c.FlushQueue()
	c.Writer().WriteArrayLen(len(queue))
	for _, entry := range queue {
		result := s.execute(c, entry.Name(), entry.Args())
		c.Writer().WriteValue(result)
	}
	return c.Writer().Flush()
}

func (s *Server) Close() {
	if s.epoll != nil {
		s.epoll.Close()
	}
	unix.Close(s.serverFd)
	if s.aof != nil {
		s.aof.Close()
	}
}

func isWriteCommand(cmd string) bool {
	switch cmd {
	case "SET", "SETNX", "SETEX", "PSETEX", "MSET", "MSETNX", "GETSET", "GETDEL",
		"INCR", "INCRBY", "DECR", "DECRBY", "INCRBYFLOAT", "APPEND", "SETRANGE",
		"DEL", "UNLINK", "EXPIRE", "EXPIREAT", "PEXPIRE", "PEXPIREAT", "PERSIST",
		"RENAME", "RENAMENX", "COPY",
		"LPUSH", "RPUSH", "LPUSHX", "RPUSHX", "LPOP", "RPOP", "LSET", "LINSERT",
		"LREM", "LTRIM", "RPOPLPUSH", "LMOVE",
		"HSET", "HMSET", "HDEL", "HINCRBY", "HINCRBYFLOAT", "HSETNX",
		"SADD", "SREM", "SMOVE", "SPOP", "SUNIONSTORE", "SINTERSTORE", "SDIFFSTORE",
		"ZADD", "ZREM", "ZINCRBY", "ZPOPMIN", "ZPOPMAX",
		"FLUSHDB", "FLUSHALL", "SELECT":
		return true
	}
	return false
}
