package server

import (
	"bufio"
	"qwikstore/resp"
)

// Client represents a connected redis-cli session.
// It owns its raw fd and is only ever touched from the single event-loop goroutine.
type Client struct {
	conn    *rawConn
	reader  *resp.Reader
	writer  *resp.Writer
	dbIndex int
	// MULTI/EXEC transaction state
	inMulti bool
	queue   []*cmdEntry
}

type cmdEntry struct {
	name string
	args []*resp.Value
}

func (e *cmdEntry) Name() string        { return e.name }
func (e *cmdEntry) Args() []*resp.Value { return e.args }

func NewClient(fd int) *Client {
	rc := newRawConn(fd)
	return &Client{
		conn:   rc,
		reader: resp.NewReader(bufio.NewReaderSize(rc, 64*1024)),
		writer: resp.NewWriter(bufio.NewWriterSize(rc, 64*1024)),
	}
}

func (c *Client) Close() error        { return c.conn.Close() }
func (c *Client) Reader() *resp.Reader { return c.reader }
func (c *Client) Writer() *resp.Writer { return c.writer }
func (c *Client) DBIndex() int         { return c.dbIndex }
func (c *Client) SetDBIndex(i int)     { c.dbIndex = i }
func (c *Client) InMulti() bool        { return c.inMulti }

func (c *Client) StartMulti()  { c.inMulti = true; c.queue = nil }
func (c *Client) DiscardMulti() { c.inMulti = false; c.queue = nil }

func (c *Client) Enqueue(name string, args []*resp.Value) {
	c.queue = append(c.queue, &cmdEntry{name: name, args: args})
}

func (c *Client) FlushQueue() []*cmdEntry {
	q := c.queue
	c.queue = nil
	c.inMulti = false
	return q
}
