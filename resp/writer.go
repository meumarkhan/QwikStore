package resp

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
)

// Writer serializes RESP values to an io.Writer.
type Writer struct {
	wr *bufio.Writer
}

func NewWriter(w io.Writer) *Writer {
	return &Writer{wr: bufio.NewWriterSize(w, 64*1024)}
}

func (w *Writer) Flush() error {
	return w.wr.Flush()
}

func (w *Writer) WriteSimpleString(s string) error {
	_, err := fmt.Fprintf(w.wr, "+%s\r\n", s)
	return err
}

func (w *Writer) WriteError(msg string) error {
	_, err := fmt.Fprintf(w.wr, "-%s\r\n", msg)
	return err
}

func (w *Writer) WriteInteger(n int64) error {
	_, err := fmt.Fprintf(w.wr, ":%d\r\n", n)
	return err
}

func (w *Writer) WriteBulkString(s string) error {
	_, err := fmt.Fprintf(w.wr, "$%d\r\n%s\r\n", len(s), s)
	return err
}

func (w *Writer) WriteNullBulkString() error {
	_, err := w.wr.WriteString("$-1\r\n")
	return err
}

func (w *Writer) WriteNullArray() error {
	_, err := w.wr.WriteString("*-1\r\n")
	return err
}

func (w *Writer) WriteArrayLen(n int) error {
	_, err := fmt.Fprintf(w.wr, "*%d\r\n", n)
	return err
}

func (w *Writer) WriteFloat(f float64) error {
	s := strconv.FormatFloat(f, 'f', -1, 64)
	return w.WriteBulkString(s)
}

// WriteValue serializes a Value recursively.
func (w *Writer) WriteValue(v *Value) error {
	if v == nil || v.IsNull {
		return w.WriteNullBulkString()
	}
	switch v.Type {
	case TypeSimpleString:
		return w.WriteSimpleString(v.Str)
	case TypeError:
		return w.WriteError(v.Str)
	case TypeInteger:
		return w.WriteInteger(v.Integer)
	case TypeBulkString:
		return w.WriteBulkString(v.Str)
	case TypeArray:
		if err := w.WriteArrayLen(len(v.Array)); err != nil {
			return err
		}
		for _, elem := range v.Array {
			if err := w.WriteValue(elem); err != nil {
				return err
			}
		}
		return nil
	}
	return fmt.Errorf("unknown value type: %c", v.Type)
}
