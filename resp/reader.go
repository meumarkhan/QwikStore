package resp

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strconv"
)

// RESP data types
const (
	TypeSimpleString = '+'
	TypeError        = '-'
	TypeInteger      = ':'
	TypeBulkString   = '$'
	TypeArray        = '*'
)

var (
	ErrInvalidSyntax = errors.New("invalid RESP syntax")
	ErrUnexpectedEOF = errors.New("unexpected EOF")
)

// Value represents a parsed RESP value.
type Value struct {
	Type    byte
	Str     string
	Integer int64
	Array   []*Value
	IsNull  bool
}

type Reader struct {
	rd *bufio.Reader
}

func NewReader(r io.Reader) *Reader {
	return &Reader{rd: bufio.NewReaderSize(r, 64*1024)}
}

func (r *Reader) ReadValue() (*Value, error) {
	b, err := r.rd.ReadByte()
	if err != nil {
		return nil, err
	}
	switch b {
	case TypeSimpleString:
		return r.readSimpleString()
	case TypeError:
		return r.readError()
	case TypeInteger:
		return r.readInteger()
	case TypeBulkString:
		return r.readBulkString()
	case TypeArray:
		return r.readArray()
	default:
		// Inline command (e.g. plain text like "PING\r\n")
		r.rd.UnreadByte()
		return r.readInline()
	}
}

func (r *Reader) readLine() (string, error) {
	line, err := r.rd.ReadString('\n')
	if err != nil {
		return "", err
	}
	if len(line) < 2 || line[len(line)-2] != '\r' {
		return "", ErrInvalidSyntax
	}
	return line[:len(line)-2], nil
}

func (r *Reader) readSimpleString() (*Value, error) {
	line, err := r.readLine()
	if err != nil {
		return nil, err
	}
	return &Value{Type: TypeSimpleString, Str: line}, nil
}

func (r *Reader) readError() (*Value, error) {
	line, err := r.readLine()
	if err != nil {
		return nil, err
	}
	return &Value{Type: TypeError, Str: line}, nil
}

func (r *Reader) readInteger() (*Value, error) {
	line, err := r.readLine()
	if err != nil {
		return nil, err
	}
	n, err := strconv.ParseInt(line, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid integer: %s", line)
	}
	return &Value{Type: TypeInteger, Integer: n}, nil
}

func (r *Reader) readBulkString() (*Value, error) {
	line, err := r.readLine()
	if err != nil {
		return nil, err
	}
	length, err := strconv.ParseInt(line, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid bulk string length: %s", line)
	}
	if length == -1 {
		return &Value{Type: TypeBulkString, IsNull: true}, nil
	}
	if length < 0 {
		return nil, ErrInvalidSyntax
	}
	buf := make([]byte, length+2) // +2 for \r\n
	if _, err := io.ReadFull(r.rd, buf); err != nil {
		return nil, ErrUnexpectedEOF
	}
	if buf[length] != '\r' || buf[length+1] != '\n' {
		return nil, ErrInvalidSyntax
	}
	return &Value{Type: TypeBulkString, Str: string(buf[:length])}, nil
}

func (r *Reader) readArray() (*Value, error) {
	line, err := r.readLine()
	if err != nil {
		return nil, err
	}
	count, err := strconv.ParseInt(line, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid array length: %s", line)
	}
	if count == -1 {
		return &Value{Type: TypeArray, IsNull: true}, nil
	}
	if count < 0 {
		return nil, ErrInvalidSyntax
	}
	arr := make([]*Value, count)
	for i := int64(0); i < count; i++ {
		v, err := r.ReadValue()
		if err != nil {
			return nil, err
		}
		arr[i] = v
	}
	return &Value{Type: TypeArray, Array: arr}, nil
}

// readInline parses inline commands like "PING" or "SET foo bar"
func (r *Reader) readInline() (*Value, error) {
	line, err := r.readLine()
	if err != nil {
		return nil, err
	}
	parts := splitInline(line)
	if len(parts) == 0 {
		return nil, ErrInvalidSyntax
	}
	arr := make([]*Value, len(parts))
	for i, p := range parts {
		arr[i] = &Value{Type: TypeBulkString, Str: p}
	}
	return &Value{Type: TypeArray, Array: arr}, nil
}

func splitInline(line string) []string {
	var parts []string
	var current []byte
	inQuote := false
	for i := 0; i < len(line); i++ {
		c := line[i]
		if c == '"' {
			inQuote = !inQuote
		} else if c == ' ' && !inQuote {
			if len(current) > 0 {
				parts = append(parts, string(current))
				current = current[:0]
			}
		} else {
			current = append(current, c)
		}
	}
	if len(current) > 0 {
		parts = append(parts, string(current))
	}
	return parts
}

// ToCommand extracts command name and args from an array Value.
func (v *Value) ToCommand() (string, []*Value, error) {
	if v.Type != TypeArray || len(v.Array) == 0 {
		return "", nil, ErrInvalidSyntax
	}
	cmd := v.Array[0]
	if cmd.Type != TypeBulkString {
		return "", nil, ErrInvalidSyntax
	}
	return cmd.Str, v.Array[1:], nil
}
