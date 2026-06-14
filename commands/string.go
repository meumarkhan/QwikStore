package commands

import (
	"fmt"
	"strconv"
	"strings"
	"time"
	"qwikstore/resp"
	"qwikstore/store"
)

func cmdSet(ctx *Context) *resp.Value {
	if ctx.NArgs() < 2 {
		return wrongArgCount("SET")
	}
	key := ctx.ArgStr(0)
	val := ctx.ArgStr(1)

	var expiry time.Time
	var hasExpiry bool
	nx, xx := false, false

	for i := 2; i < ctx.NArgs(); i++ {
		opt := strings.ToUpper(ctx.ArgStr(i))
		switch opt {
		case "EX":
			i++
			if i >= ctx.NArgs() {
				return errResp("ERR syntax error")
			}
			secs, err := strconv.ParseInt(ctx.ArgStr(i), 10, 64)
			if err != nil || secs <= 0 {
				return errResp("ERR invalid expire time in 'set' command")
			}
			expiry = time.Now().Add(time.Duration(secs) * time.Second)
			hasExpiry = true
		case "PX":
			i++
			if i >= ctx.NArgs() {
				return errResp("ERR syntax error")
			}
			ms, err := strconv.ParseInt(ctx.ArgStr(i), 10, 64)
			if err != nil || ms <= 0 {
				return errResp("ERR invalid expire time in 'set' command")
			}
			expiry = time.Now().Add(time.Duration(ms) * time.Millisecond)
			hasExpiry = true
		case "EXAT":
			i++
			if i >= ctx.NArgs() {
				return errResp("ERR syntax error")
			}
			ts, err := strconv.ParseInt(ctx.ArgStr(i), 10, 64)
			if err != nil || ts <= 0 {
				return errResp("ERR invalid expire time in 'set' command")
			}
			expiry = time.Unix(ts, 0)
			hasExpiry = true
		case "PXAT":
			i++
			if i >= ctx.NArgs() {
				return errResp("ERR syntax error")
			}
			ts, err := strconv.ParseInt(ctx.ArgStr(i), 10, 64)
			if err != nil || ts <= 0 {
				return errResp("ERR invalid expire time in 'set' command")
			}
			expiry = time.UnixMilli(ts)
			hasExpiry = true
		case "NX":
			nx = true
		case "XX":
			xx = true
		case "KEEPTTL":
			// keep existing TTL — handle below
		default:
			return errResp("ERR syntax error")
		}
	}

	if nx {
		_, exists := ctx.DB.Get(key)
		if exists {
			return nullBulkResp()
		}
	}
	if xx {
		_, exists := ctx.DB.Get(key)
		if !exists {
			return nullBulkResp()
		}
	}

	obj := &store.Object{
		Type:      store.TypeString,
		Value:     val,
		HasExpiry: hasExpiry,
		Expiry:    expiry,
	}
	ctx.DB.Set(key, obj)
	return okResp()
}

func cmdGet(ctx *Context) *resp.Value {
	if ctx.NArgs() != 1 {
		return wrongArgCount("GET")
	}
	obj, ok := ctx.DB.Get(ctx.ArgStr(0))
	if !ok {
		return nullBulkResp()
	}
	if obj.Type != store.TypeString {
		return wrongTypeErr()
	}
	return bulkResp(obj.Value.(string))
}

func cmdGetSet(ctx *Context) *resp.Value {
	if ctx.NArgs() != 2 {
		return wrongArgCount("GETSET")
	}
	key := ctx.ArgStr(0)
	old, ok := ctx.DB.Get(key)
	var oldVal *resp.Value
	if !ok {
		oldVal = nullBulkResp()
	} else if old.Type != store.TypeString {
		return wrongTypeErr()
	} else {
		oldVal = bulkResp(old.Value.(string))
	}
	ctx.DB.Set(key, &store.Object{Type: store.TypeString, Value: ctx.ArgStr(1)})
	return oldVal
}

func cmdGetDel(ctx *Context) *resp.Value {
	if ctx.NArgs() != 1 {
		return wrongArgCount("GETDEL")
	}
	key := ctx.ArgStr(0)
	obj, ok := ctx.DB.Get(key)
	if !ok {
		return nullBulkResp()
	}
	if obj.Type != store.TypeString {
		return wrongTypeErr()
	}
	v := bulkResp(obj.Value.(string))
	ctx.DB.Del(key)
	return v
}

func cmdMSet(ctx *Context) *resp.Value {
	if ctx.NArgs() < 2 || ctx.NArgs()%2 != 0 {
		return wrongArgCount("MSET")
	}
	for i := 0; i < ctx.NArgs(); i += 2 {
		ctx.DB.Set(ctx.ArgStr(i), &store.Object{Type: store.TypeString, Value: ctx.ArgStr(i + 1)})
	}
	return okResp()
}

func cmdMSetNX(ctx *Context) *resp.Value {
	if ctx.NArgs() < 2 || ctx.NArgs()%2 != 0 {
		return wrongArgCount("MSETNX")
	}
	for i := 0; i < ctx.NArgs(); i += 2 {
		if _, ok := ctx.DB.Get(ctx.ArgStr(i)); ok {
			return intResp(0)
		}
	}
	for i := 0; i < ctx.NArgs(); i += 2 {
		ctx.DB.Set(ctx.ArgStr(i), &store.Object{Type: store.TypeString, Value: ctx.ArgStr(i + 1)})
	}
	return intResp(1)
}

func cmdMGet(ctx *Context) *resp.Value {
	if ctx.NArgs() == 0 {
		return wrongArgCount("MGET")
	}
	arr := make([]*resp.Value, ctx.NArgs())
	for i := 0; i < ctx.NArgs(); i++ {
		obj, ok := ctx.DB.Get(ctx.ArgStr(i))
		if !ok || obj.Type != store.TypeString {
			arr[i] = nullBulkResp()
		} else {
			arr[i] = bulkResp(obj.Value.(string))
		}
	}
	return &resp.Value{Type: resp.TypeArray, Array: arr}
}

func cmdSetNX(ctx *Context) *resp.Value {
	if ctx.NArgs() != 2 {
		return wrongArgCount("SETNX")
	}
	_, exists := ctx.DB.Get(ctx.ArgStr(0))
	if exists {
		return intResp(0)
	}
	ctx.DB.Set(ctx.ArgStr(0), &store.Object{Type: store.TypeString, Value: ctx.ArgStr(1)})
	return intResp(1)
}

func cmdSetEX(ctx *Context) *resp.Value {
	if ctx.NArgs() != 3 {
		return wrongArgCount("SETEX")
	}
	secs, err := strconv.ParseInt(ctx.ArgStr(1), 10, 64)
	if err != nil || secs <= 0 {
		return errResp("ERR invalid expire time in 'setex' command")
	}
	ctx.DB.Set(ctx.ArgStr(0), &store.Object{
		Type:      store.TypeString,
		Value:     ctx.ArgStr(2),
		HasExpiry: true,
		Expiry:    time.Now().Add(time.Duration(secs) * time.Second),
	})
	return okResp()
}

func cmdPSetEX(ctx *Context) *resp.Value {
	if ctx.NArgs() != 3 {
		return wrongArgCount("PSETEX")
	}
	ms, err := strconv.ParseInt(ctx.ArgStr(1), 10, 64)
	if err != nil || ms <= 0 {
		return errResp("ERR invalid expire time in 'psetex' command")
	}
	ctx.DB.Set(ctx.ArgStr(0), &store.Object{
		Type:      store.TypeString,
		Value:     ctx.ArgStr(2),
		HasExpiry: true,
		Expiry:    time.Now().Add(time.Duration(ms) * time.Millisecond),
	})
	return okResp()
}

func cmdIncr(ctx *Context) *resp.Value {
	return incrBy(ctx, "INCR", 1)
}

func cmdIncrBy(ctx *Context) *resp.Value {
	if ctx.NArgs() != 2 {
		return wrongArgCount("INCRBY")
	}
	by, err := strconv.ParseInt(ctx.ArgStr(1), 10, 64)
	if err != nil {
		return errResp("ERR value is not an integer or out of range")
	}
	return incrBy(ctx, "INCRBY", by)
}

func cmdDecr(ctx *Context) *resp.Value {
	return incrBy(ctx, "DECR", -1)
}

func cmdDecrBy(ctx *Context) *resp.Value {
	if ctx.NArgs() != 2 {
		return wrongArgCount("DECRBY")
	}
	by, err := strconv.ParseInt(ctx.ArgStr(1), 10, 64)
	if err != nil {
		return errResp("ERR value is not an integer or out of range")
	}
	return incrBy(ctx, "DECRBY", -by)
}

func incrBy(ctx *Context, cmd string, delta int64) *resp.Value {
	key := ctx.ArgStr(0)
	obj, ok := ctx.DB.Get(key)
	var cur int64
	if ok {
		if obj.Type != store.TypeString {
			return wrongTypeErr()
		}
		var err error
		cur, err = strconv.ParseInt(obj.Value.(string), 10, 64)
		if err != nil {
			return errResp("ERR value is not an integer or out of range")
		}
	}
	cur += delta
	ctx.DB.Set(key, &store.Object{Type: store.TypeString, Value: strconv.FormatInt(cur, 10)})
	return intResp(cur)
}

func cmdIncrByFloat(ctx *Context) *resp.Value {
	if ctx.NArgs() != 2 {
		return wrongArgCount("INCRBYFLOAT")
	}
	key := ctx.ArgStr(0)
	delta, err := strconv.ParseFloat(ctx.ArgStr(1), 64)
	if err != nil {
		return errResp("ERR value is not a valid float")
	}
	obj, ok := ctx.DB.Get(key)
	var cur float64
	if ok {
		if obj.Type != store.TypeString {
			return wrongTypeErr()
		}
		cur, err = strconv.ParseFloat(obj.Value.(string), 64)
		if err != nil {
			return errResp("ERR value is not a valid float")
		}
	}
	cur += delta
	s := strconv.FormatFloat(cur, 'f', -1, 64)
	ctx.DB.Set(key, &store.Object{Type: store.TypeString, Value: s})
	return bulkResp(s)
}

func cmdAppend(ctx *Context) *resp.Value {
	if ctx.NArgs() != 2 {
		return wrongArgCount("APPEND")
	}
	key := ctx.ArgStr(0)
	obj, ok := ctx.DB.Get(key)
	var existing string
	if ok {
		if obj.Type != store.TypeString {
			return wrongTypeErr()
		}
		existing = obj.Value.(string)
	}
	newVal := existing + ctx.ArgStr(1)
	ctx.DB.Set(key, &store.Object{Type: store.TypeString, Value: newVal})
	return intResp(int64(len(newVal)))
}

func cmdStrLen(ctx *Context) *resp.Value {
	if ctx.NArgs() != 1 {
		return wrongArgCount("STRLEN")
	}
	obj, ok := ctx.DB.Get(ctx.ArgStr(0))
	if !ok {
		return intResp(0)
	}
	if obj.Type != store.TypeString {
		return wrongTypeErr()
	}
	return intResp(int64(len(obj.Value.(string))))
}

func cmdGetRange(ctx *Context) *resp.Value {
	if ctx.NArgs() != 3 {
		return wrongArgCount("GETRANGE")
	}
	obj, ok := ctx.DB.Get(ctx.ArgStr(0))
	if !ok {
		return bulkResp("")
	}
	if obj.Type != store.TypeString {
		return wrongTypeErr()
	}
	s := obj.Value.(string)
	start, err1 := strconv.Atoi(ctx.ArgStr(1))
	end, err2 := strconv.Atoi(ctx.ArgStr(2))
	if err1 != nil || err2 != nil {
		return errResp("ERR value is not an integer or out of range")
	}
	l := len(s)
	if start < 0 {
		start = l + start
	}
	if end < 0 {
		end = l + end
	}
	if start < 0 {
		start = 0
	}
	if end >= l {
		end = l - 1
	}
	if start > end {
		return bulkResp("")
	}
	return bulkResp(s[start : end+1])
}

func cmdSetRange(ctx *Context) *resp.Value {
	if ctx.NArgs() != 3 {
		return wrongArgCount("SETRANGE")
	}
	key := ctx.ArgStr(0)
	offset, err := strconv.Atoi(ctx.ArgStr(1))
	if err != nil || offset < 0 {
		return errResp("ERR offset is not an integer or out of range")
	}
	repl := ctx.ArgStr(2)

	obj, ok := ctx.DB.Get(key)
	var s string
	if ok {
		if obj.Type != store.TypeString {
			return wrongTypeErr()
		}
		s = obj.Value.(string)
	}

	end := offset + len(repl)
	if end > len(s) {
		s = s + fmt.Sprintf("%*s", end-len(s), "")
		b := []byte(s)
		copy(b[offset:], repl)
		s = string(b)
	} else {
		b := []byte(s)
		copy(b[offset:], repl)
		s = string(b)
	}
	ctx.DB.Set(key, &store.Object{Type: store.TypeString, Value: s})
	return intResp(int64(len(s)))
}
