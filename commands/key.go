package commands

import (
	"strconv"
	"strings"
	"time"
	"qwikstore/resp"
	"qwikstore/store"
)

func cmdDel(ctx *Context) *resp.Value {
	if ctx.NArgs() < 1 {
		return wrongArgCount("DEL")
	}
	return intResp(int64(ctx.DB.Del(ctx.AllArgStr()...)))
}

func cmdExists(ctx *Context) *resp.Value {
	if ctx.NArgs() < 1 {
		return wrongArgCount("EXISTS")
	}
	return intResp(int64(ctx.DB.Exists(ctx.AllArgStr()...)))
}

func cmdExpire(ctx *Context) *resp.Value {
	if ctx.NArgs() != 2 {
		return wrongArgCount("EXPIRE")
	}
	secs, err := strconv.ParseInt(ctx.ArgStr(1), 10, 64)
	if err != nil {
		return errResp("ERR value is not an integer or out of range")
	}
	ok := ctx.DB.SetExpiry(ctx.ArgStr(0), time.Now().Add(time.Duration(secs)*time.Second))
	if ok {
		return intResp(1)
	}
	return intResp(0)
}

func cmdExpireAt(ctx *Context) *resp.Value {
	if ctx.NArgs() != 2 {
		return wrongArgCount("EXPIREAT")
	}
	ts, err := strconv.ParseInt(ctx.ArgStr(1), 10, 64)
	if err != nil {
		return errResp("ERR value is not an integer or out of range")
	}
	ok := ctx.DB.SetExpiry(ctx.ArgStr(0), time.Unix(ts, 0))
	if ok {
		return intResp(1)
	}
	return intResp(0)
}

func cmdPExpire(ctx *Context) *resp.Value {
	if ctx.NArgs() != 2 {
		return wrongArgCount("PEXPIRE")
	}
	ms, err := strconv.ParseInt(ctx.ArgStr(1), 10, 64)
	if err != nil {
		return errResp("ERR value is not an integer or out of range")
	}
	ok := ctx.DB.SetExpiry(ctx.ArgStr(0), time.Now().Add(time.Duration(ms)*time.Millisecond))
	if ok {
		return intResp(1)
	}
	return intResp(0)
}

func cmdPExpireAt(ctx *Context) *resp.Value {
	if ctx.NArgs() != 2 {
		return wrongArgCount("PEXPIREAT")
	}
	ms, err := strconv.ParseInt(ctx.ArgStr(1), 10, 64)
	if err != nil {
		return errResp("ERR value is not an integer or out of range")
	}
	ok := ctx.DB.SetExpiry(ctx.ArgStr(0), time.UnixMilli(ms))
	if ok {
		return intResp(1)
	}
	return intResp(0)
}

func cmdTTL(ctx *Context) *resp.Value {
	if ctx.NArgs() != 1 {
		return wrongArgCount("TTL")
	}
	d := ctx.DB.TTL(ctx.ArgStr(0))
	if d == -2 {
		return intResp(-2) // key does not exist
	}
	if d == -1 {
		return intResp(-1) // no expiry
	}
	return intResp(int64(d.Seconds()))
}

func cmdPTTL(ctx *Context) *resp.Value {
	if ctx.NArgs() != 1 {
		return wrongArgCount("PTTL")
	}
	d := ctx.DB.TTL(ctx.ArgStr(0))
	if d == -2 {
		return intResp(-2)
	}
	if d == -1 {
		return intResp(-1)
	}
	return intResp(int64(d.Milliseconds()))
}

func cmdPersist(ctx *Context) *resp.Value {
	if ctx.NArgs() != 1 {
		return wrongArgCount("PERSIST")
	}
	if ctx.DB.RemoveExpiry(ctx.ArgStr(0)) {
		return intResp(1)
	}
	return intResp(0)
}

func cmdRename(ctx *Context) *resp.Value {
	if ctx.NArgs() != 2 {
		return wrongArgCount("RENAME")
	}
	if !ctx.DB.Rename(ctx.ArgStr(0), ctx.ArgStr(1)) {
		return errResp("ERR no such key")
	}
	return okResp()
}

func cmdRenameNX(ctx *Context) *resp.Value {
	if ctx.NArgs() != 2 {
		return wrongArgCount("RENAMENX")
	}
	if ctx.DB.Exists(ctx.ArgStr(0)) == 0 {
		return errResp("ERR no such key")
	}
	if ctx.DB.Exists(ctx.ArgStr(1)) > 0 {
		return intResp(0)
	}
	ctx.DB.Rename(ctx.ArgStr(0), ctx.ArgStr(1))
	return intResp(1)
}

func cmdType(ctx *Context) *resp.Value {
	if ctx.NArgs() != 1 {
		return wrongArgCount("TYPE")
	}
	return &resp.Value{Type: resp.TypeSimpleString, Str: ctx.DB.Type(ctx.ArgStr(0))}
}

func cmdKeys(ctx *Context) *resp.Value {
	if ctx.NArgs() != 1 {
		return wrongArgCount("KEYS")
	}
	keys := ctx.DB.Keys(ctx.ArgStr(0))
	return stringsToArray(keys)
}

func cmdScan(ctx *Context) *resp.Value {
	if ctx.NArgs() < 1 {
		return wrongArgCount("SCAN")
	}
	cursor, err := strconv.Atoi(ctx.ArgStr(0))
	if err != nil {
		return errResp("ERR value is not an integer or out of range")
	}
	pattern := ""
	count := 10
	typFilter := ""
	for i := 1; i < ctx.NArgs(); i++ {
		switch strings.ToUpper(ctx.ArgStr(i)) {
		case "MATCH":
			i++
			if i < ctx.NArgs() {
				pattern = ctx.ArgStr(i)
			}
		case "COUNT":
			i++
			if i < ctx.NArgs() {
				count, _ = strconv.Atoi(ctx.ArgStr(i))
			}
		case "TYPE":
			i++
			if i < ctx.NArgs() {
				typFilter = ctx.ArgStr(i)
			}
		}
	}
	nextCursor, keys := ctx.DB.Scan(cursor, count, pattern, typFilter)
	return arrayResp(
		bulkResp(strconv.Itoa(nextCursor)),
		stringsToArray(keys),
	)
}

func cmdRandomKey(ctx *Context) *resp.Value {
	k := ctx.DB.RandomKey()
	if k == "" {
		return nullBulkResp()
	}
	return bulkResp(k)
}

func cmdCopy(ctx *Context) *resp.Value {
	if ctx.NArgs() < 2 {
		return wrongArgCount("COPY")
	}
	src := ctx.ArgStr(0)
	dst := ctx.ArgStr(1)
	replace := false
	for i := 2; i < ctx.NArgs(); i++ {
		if strings.ToUpper(ctx.ArgStr(i)) == "REPLACE" {
			replace = true
		}
	}
	srcObj, ok := ctx.DB.Get(src)
	if !ok {
		return intResp(0)
	}
	if !replace {
		if ctx.DB.Exists(dst) > 0 {
			return intResp(0)
		}
	}
	newObj := &store.Object{
		Type:      srcObj.Type,
		Value:     deepCopyValue(srcObj),
		HasExpiry: srcObj.HasExpiry,
		Expiry:    srcObj.Expiry,
	}
	ctx.DB.Set(dst, newObj)
	return intResp(1)
}

func deepCopyValue(obj *store.Object) interface{} {
	switch obj.Type {
	case store.TypeString:
		return obj.Value.(string)
	case store.TypeHash:
		src := obj.Value.(map[string]string)
		dst := make(map[string]string, len(src))
		for k, v := range src {
			dst[k] = v
		}
		return dst
	case store.TypeSet:
		src := obj.Value.(map[string]struct{})
		dst := make(map[string]struct{}, len(src))
		for k := range src {
			dst[k] = struct{}{}
		}
		return dst
	}
	// For List and ZSet, return as-is (shallow — good enough for a learning project)
	return obj.Value
}

func cmdObject(ctx *Context) *resp.Value {
	if ctx.NArgs() < 2 {
		return wrongArgCount("OBJECT")
	}
	sub := strings.ToUpper(ctx.ArgStr(0))
	key := ctx.ArgStr(1)
	obj, ok := ctx.DB.Get(key)
	if !ok {
		return errResp("ERR no such key")
	}
	switch sub {
	case "REFCOUNT":
		return intResp(1)
	case "IDLETIME":
		idle := time.Since(time.Unix(0, obj.LastAccess)).Seconds()
		return intResp(int64(idle))
	case "FREQ":
		return intResp(int64(obj.AccessFreq))
	case "ENCODING":
		return bulkResp(encodingForType(obj.Type))
	case "HELP":
		return stringsToArray([]string{
			"OBJECT <subcommand> [<arg> [value] [opt] ...]. subcommands are:",
			"ENCODING <key>",
			"FREQ <key>",
			"IDLETIME <key>",
			"REFCOUNT <key>",
		})
	}
	return errResp("ERR unknown subcommand '" + ctx.ArgStr(0) + "'")
}

func encodingForType(t store.ValueType) string {
	switch t {
	case store.TypeString:
		return "embstr"
	case store.TypeList:
		return "linkedlist"
	case store.TypeHash:
		return "hashtable"
	case store.TypeSet:
		return "hashtable"
	case store.TypeZSet:
		return "skiplist"
	}
	return "unknown"
}
