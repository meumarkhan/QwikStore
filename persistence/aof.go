package persistence

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
	"qwikstore/config"
	"qwikstore/resp"
)

// AOF handles Append-Only File persistence.
type AOF struct {
	mu       sync.Mutex
	file     *os.File
	buf      *bufio.Writer
	sync     config.AOFSync
	ticker   *time.Ticker
	stopCh   chan struct{}
}

func NewAOF(filename string, syncPolicy config.AOFSync) (*AOF, error) {
	f, err := os.OpenFile(filename, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("open AOF file: %w", err)
	}
	a := &AOF{
		file:   f,
		buf:    bufio.NewWriterSize(f, 64*1024),
		sync:   syncPolicy,
		stopCh: make(chan struct{}),
	}
	if syncPolicy == config.AOFSyncEverySec {
		a.ticker = time.NewTicker(time.Second)
		go a.bgSync()
	}
	return a, nil
}

// Write appends a command to the AOF in RESP format.
func (a *AOF) Write(args []string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	fmt.Fprintf(a.buf, "*%d\r\n", len(args))
	for _, arg := range args {
		fmt.Fprintf(a.buf, "$%d\r\n%s\r\n", len(arg), arg)
	}

	switch a.sync {
	case config.AOFSyncAlways:
		if err := a.buf.Flush(); err != nil {
			return err
		}
		return a.file.Sync()
	case config.AOFSyncNo:
		// OS decides when to flush
	}
	return nil
}

func (a *AOF) bgSync() {
	for {
		select {
		case <-a.ticker.C:
			a.mu.Lock()
			a.buf.Flush()
			a.file.Sync()
			a.mu.Unlock()
		case <-a.stopCh:
			return
		}
	}
}

// Close flushes and closes the AOF file.
func (a *AOF) Close() error {
	if a.ticker != nil {
		a.ticker.Stop()
	}
	close(a.stopCh)
	a.mu.Lock()
	defer a.mu.Unlock()
	a.buf.Flush()
	a.file.Sync()
	return a.file.Close()
}

// Replay reads the AOF file and calls handler for each command.
// handler receives (cmdName, args []string).
func Replay(filename string, handler func(cmd string, args []string) error) error {
	f, err := os.Open(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer f.Close()

	reader := resp.NewReader(f)
	for {
		val, err := reader.ReadValue()
		if err != nil {
			if err == io.EOF {
				break
			}
			// Skip malformed entries at end of file (crash-safe)
			break
		}
		cmd, args, err := val.ToCommand()
		if err != nil {
			continue
		}
		strArgs := make([]string, len(args))
		for i, a := range args {
			strArgs[i] = a.Str
		}
		if err := handler(strings.ToUpper(cmd), strArgs); err != nil {
			return err
		}
	}
	return nil
}
