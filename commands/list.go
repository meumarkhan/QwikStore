package commands

import (
	"strconv"
	"strings"
	"qwikstore/datastructures"
	"qwikstore/resp"
	"qwikstore/store"
)

func getOrCreateList(ctx *Context, key string) (*datastructures.List, *resp.Value) {
	obj, ok := ctx.DB.Get(key)
	if !ok {
		l := datastructures.NewList()
		ctx.DB.Set(key, &store.Object{Type: store.TypeList, Value: l})
		return l, nil
	}
	if obj.Type != store.TypeList {
		return nil, wrongTypeErr()
	}
	return obj.Value.(*datastructures.List), nil
}

func getList(ctx *Context, key string) (*datastructures.List, *resp.Value) {
	obj, ok := ctx.DB.Get(key)
	if !ok {
		return nil, nil
	}
	if obj.Type != store.TypeList {
		return nil, wrongTypeErr()
	}
	return obj.Value.(*datastructures.List), nil
}

func cmdLPush(ctx *Context) *resp.Value {
	if ctx.NArgs() < 2 {
		return wrongArgCount("LPUSH")
	}
	l, err := getOrCreateList(ctx, ctx.ArgStr(0))
	if err != nil {
		return err
	}
	for i := 1; i < ctx.NArgs(); i++ {
		l.LPush(ctx.ArgStr(i))
	}
	return intResp(int64(l.Len()))
}

func cmdRPush(ctx *Context) *resp.Value {
	if ctx.NArgs() < 2 {
		return wrongArgCount("RPUSH")
	}
	l, err := getOrCreateList(ctx, ctx.ArgStr(0))
	if err != nil {
		return err
	}
	for i := 1; i < ctx.NArgs(); i++ {
		l.RPush(ctx.ArgStr(i))
	}
	return intResp(int64(l.Len()))
}

func cmdLPushX(ctx *Context) *resp.Value {
	if ctx.NArgs() < 2 {
		return wrongArgCount("LPUSHX")
	}
	l, errV := getList(ctx, ctx.ArgStr(0))
	if errV != nil {
		return errV
	}
	if l == nil {
		return intResp(0)
	}
	for i := 1; i < ctx.NArgs(); i++ {
		l.LPush(ctx.ArgStr(i))
	}
	return intResp(int64(l.Len()))
}

func cmdRPushX(ctx *Context) *resp.Value {
	if ctx.NArgs() < 2 {
		return wrongArgCount("RPUSHX")
	}
	l, errV := getList(ctx, ctx.ArgStr(0))
	if errV != nil {
		return errV
	}
	if l == nil {
		return intResp(0)
	}
	for i := 1; i < ctx.NArgs(); i++ {
		l.RPush(ctx.ArgStr(i))
	}
	return intResp(int64(l.Len()))
}

func cmdLPop(ctx *Context) *resp.Value {
	if ctx.NArgs() < 1 {
		return wrongArgCount("LPOP")
	}
	l, errV := getList(ctx, ctx.ArgStr(0))
	if errV != nil {
		return errV
	}
	if l == nil {
		return nullBulkResp()
	}
	count := 1
	returnArray := false
	if ctx.NArgs() >= 2 {
		n, err := strconv.Atoi(ctx.ArgStr(1))
		if err != nil || n < 0 {
			return errResp("ERR value is not an integer or out of range")
		}
		count = n
		returnArray = true
	}
	if returnArray {
		arr := make([]*resp.Value, 0, count)
		for i := 0; i < count; i++ {
			v, ok := l.LPop()
			if !ok {
				break
			}
			arr = append(arr, bulkResp(v))
		}
		if l.Len() == 0 {
			ctx.DB.Del(ctx.ArgStr(0))
		}
		if len(arr) == 0 {
			return nullArrayResp()
		}
		return &resp.Value{Type: resp.TypeArray, Array: arr}
	}
	v, ok := l.LPop()
	if l.Len() == 0 {
		ctx.DB.Del(ctx.ArgStr(0))
	}
	if !ok {
		return nullBulkResp()
	}
	return bulkResp(v)
}

func cmdRPop(ctx *Context) *resp.Value {
	if ctx.NArgs() < 1 {
		return wrongArgCount("RPOP")
	}
	l, errV := getList(ctx, ctx.ArgStr(0))
	if errV != nil {
		return errV
	}
	if l == nil {
		return nullBulkResp()
	}
	count := 1
	returnArray := false
	if ctx.NArgs() >= 2 {
		n, err := strconv.Atoi(ctx.ArgStr(1))
		if err != nil || n < 0 {
			return errResp("ERR value is not an integer or out of range")
		}
		count = n
		returnArray = true
	}
	if returnArray {
		arr := make([]*resp.Value, 0, count)
		for i := 0; i < count; i++ {
			v, ok := l.RPop()
			if !ok {
				break
			}
			arr = append(arr, bulkResp(v))
		}
		if l.Len() == 0 {
			ctx.DB.Del(ctx.ArgStr(0))
		}
		if len(arr) == 0 {
			return nullArrayResp()
		}
		return &resp.Value{Type: resp.TypeArray, Array: arr}
	}
	v, ok := l.RPop()
	if l.Len() == 0 {
		ctx.DB.Del(ctx.ArgStr(0))
	}
	if !ok {
		return nullBulkResp()
	}
	return bulkResp(v)
}

func cmdLLen(ctx *Context) *resp.Value {
	if ctx.NArgs() != 1 {
		return wrongArgCount("LLEN")
	}
	l, errV := getList(ctx, ctx.ArgStr(0))
	if errV != nil {
		return errV
	}
	if l == nil {
		return intResp(0)
	}
	return intResp(int64(l.Len()))
}

func cmdLRange(ctx *Context) *resp.Value {
	if ctx.NArgs() != 3 {
		return wrongArgCount("LRANGE")
	}
	l, errV := getList(ctx, ctx.ArgStr(0))
	if errV != nil {
		return errV
	}
	if l == nil {
		return &resp.Value{Type: resp.TypeArray, Array: []*resp.Value{}}
	}
	start, err1 := strconv.Atoi(ctx.ArgStr(1))
	stop, err2 := strconv.Atoi(ctx.ArgStr(2))
	if err1 != nil || err2 != nil {
		return errResp("ERR value is not an integer or out of range")
	}
	items := l.Range(start, stop)
	return stringsToArray(items)
}

func cmdLIndex(ctx *Context) *resp.Value {
	if ctx.NArgs() != 2 {
		return wrongArgCount("LINDEX")
	}
	l, errV := getList(ctx, ctx.ArgStr(0))
	if errV != nil {
		return errV
	}
	if l == nil {
		return nullBulkResp()
	}
	idx, err := strconv.Atoi(ctx.ArgStr(1))
	if err != nil {
		return errResp("ERR value is not an integer or out of range")
	}
	v, ok := l.Index(idx)
	if !ok {
		return nullBulkResp()
	}
	return bulkResp(v)
}

func cmdLSet(ctx *Context) *resp.Value {
	if ctx.NArgs() != 3 {
		return wrongArgCount("LSET")
	}
	l, errV := getList(ctx, ctx.ArgStr(0))
	if errV != nil {
		return errV
	}
	if l == nil {
		return errResp("ERR no such key")
	}
	idx, err := strconv.Atoi(ctx.ArgStr(1))
	if err != nil {
		return errResp("ERR value is not an integer or out of range")
	}
	if !l.Set(idx, ctx.ArgStr(2)) {
		return errResp("ERR index out of range")
	}
	return okResp()
}

func cmdLInsert(ctx *Context) *resp.Value {
	if ctx.NArgs() != 4 {
		return wrongArgCount("LINSERT")
	}
	l, errV := getList(ctx, ctx.ArgStr(0))
	if errV != nil {
		return errV
	}
	if l == nil {
		return intResp(0)
	}
	where := strings.ToUpper(ctx.ArgStr(1))
	pivot := ctx.ArgStr(2)
	val := ctx.ArgStr(3)
	var n int
	switch where {
	case "BEFORE":
		n = l.InsertBefore(pivot, val)
	case "AFTER":
		n = l.InsertAfter(pivot, val)
	default:
		return errResp("ERR syntax error")
	}
	if n == -1 {
		return intResp(-1)
	}
	return intResp(int64(n))
}

func cmdLRem(ctx *Context) *resp.Value {
	if ctx.NArgs() != 3 {
		return wrongArgCount("LREM")
	}
	l, errV := getList(ctx, ctx.ArgStr(0))
	if errV != nil {
		return errV
	}
	if l == nil {
		return intResp(0)
	}
	count, err := strconv.Atoi(ctx.ArgStr(1))
	if err != nil {
		return errResp("ERR value is not an integer or out of range")
	}
	n := l.Remove(count, ctx.ArgStr(2))
	if l.Len() == 0 {
		ctx.DB.Del(ctx.ArgStr(0))
	}
	return intResp(int64(n))
}

func cmdLTrim(ctx *Context) *resp.Value {
	if ctx.NArgs() != 3 {
		return wrongArgCount("LTRIM")
	}
	l, errV := getList(ctx, ctx.ArgStr(0))
	if errV != nil {
		return errV
	}
	if l == nil {
		return okResp()
	}
	start, err1 := strconv.Atoi(ctx.ArgStr(1))
	stop, err2 := strconv.Atoi(ctx.ArgStr(2))
	if err1 != nil || err2 != nil {
		return errResp("ERR value is not an integer or out of range")
	}
	l.Trim(start, stop)
	if l.Len() == 0 {
		ctx.DB.Del(ctx.ArgStr(0))
	}
	return okResp()
}

func cmdRPopLPush(ctx *Context) *resp.Value {
	if ctx.NArgs() != 2 {
		return wrongArgCount("RPOPLPUSH")
	}
	src, errV := getList(ctx, ctx.ArgStr(0))
	if errV != nil {
		return errV
	}
	if src == nil {
		return nullBulkResp()
	}
	val, ok := src.RPop()
	if !ok {
		return nullBulkResp()
	}
	if src.Len() == 0 {
		ctx.DB.Del(ctx.ArgStr(0))
	}
	dst, errV := getOrCreateList(ctx, ctx.ArgStr(1))
	if errV != nil {
		return errV
	}
	dst.LPush(val)
	return bulkResp(val)
}

func cmdLMove(ctx *Context) *resp.Value {
	if ctx.NArgs() != 4 {
		return wrongArgCount("LMOVE")
	}
	srcKey := ctx.ArgStr(0)
	dstKey := ctx.ArgStr(1)
	srcDir := strings.ToUpper(ctx.ArgStr(2))
	dstDir := strings.ToUpper(ctx.ArgStr(3))

	src, errV := getList(ctx, srcKey)
	if errV != nil {
		return errV
	}
	if src == nil {
		return nullBulkResp()
	}

	var val string
	var ok bool
	if srcDir == "LEFT" {
		val, ok = src.LPop()
	} else if srcDir == "RIGHT" {
		val, ok = src.RPop()
	} else {
		return errResp("ERR syntax error")
	}
	if !ok {
		return nullBulkResp()
	}
	if src.Len() == 0 {
		ctx.DB.Del(srcKey)
	}

	dst, errV := getOrCreateList(ctx, dstKey)
	if errV != nil {
		return errV
	}
	if dstDir == "LEFT" {
		dst.LPush(val)
	} else if dstDir == "RIGHT" {
		dst.RPush(val)
	} else {
		return errResp("ERR syntax error")
	}
	return bulkResp(val)
}
