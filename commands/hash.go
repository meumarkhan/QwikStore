package commands

import (
	"strconv"
	"qwikstore/resp"
	"qwikstore/store"
)

func getOrCreateHash(ctx *Context, key string) (map[string]string, *resp.Value) {
	obj, ok := ctx.DB.Get(key)
	if !ok {
		h := make(map[string]string)
		ctx.DB.Set(key, &store.Object{Type: store.TypeHash, Value: h})
		return h, nil
	}
	if obj.Type != store.TypeHash {
		return nil, wrongTypeErr()
	}
	return obj.Value.(map[string]string), nil
}

func getHash(ctx *Context, key string) (map[string]string, *resp.Value) {
	obj, ok := ctx.DB.Get(key)
	if !ok {
		return nil, nil
	}
	if obj.Type != store.TypeHash {
		return nil, wrongTypeErr()
	}
	return obj.Value.(map[string]string), nil
}

func cmdHSet(ctx *Context) *resp.Value {
	if ctx.NArgs() < 3 || ctx.NArgs()%2 == 0 {
		return wrongArgCount("HSET")
	}
	h, errV := getOrCreateHash(ctx, ctx.ArgStr(0))
	if errV != nil {
		return errV
	}
	added := 0
	for i := 1; i < ctx.NArgs(); i += 2 {
		field := ctx.ArgStr(i)
		if _, exists := h[field]; !exists {
			added++
		}
		h[field] = ctx.ArgStr(i + 1)
	}
	return intResp(int64(added))
}

func cmdHGet(ctx *Context) *resp.Value {
	if ctx.NArgs() != 2 {
		return wrongArgCount("HGET")
	}
	h, errV := getHash(ctx, ctx.ArgStr(0))
	if errV != nil {
		return errV
	}
	if h == nil {
		return nullBulkResp()
	}
	v, ok := h[ctx.ArgStr(1)]
	if !ok {
		return nullBulkResp()
	}
	return bulkResp(v)
}

func cmdHMSet(ctx *Context) *resp.Value {
	if ctx.NArgs() < 3 || ctx.NArgs()%2 == 0 {
		return wrongArgCount("HMSET")
	}
	h, errV := getOrCreateHash(ctx, ctx.ArgStr(0))
	if errV != nil {
		return errV
	}
	for i := 1; i < ctx.NArgs(); i += 2 {
		h[ctx.ArgStr(i)] = ctx.ArgStr(i + 1)
	}
	return okResp()
}

func cmdHMGet(ctx *Context) *resp.Value {
	if ctx.NArgs() < 2 {
		return wrongArgCount("HMGET")
	}
	h, errV := getHash(ctx, ctx.ArgStr(0))
	if errV != nil {
		return errV
	}
	arr := make([]*resp.Value, ctx.NArgs()-1)
	for i := 1; i < ctx.NArgs(); i++ {
		if h == nil {
			arr[i-1] = nullBulkResp()
			continue
		}
		v, ok := h[ctx.ArgStr(i)]
		if !ok {
			arr[i-1] = nullBulkResp()
		} else {
			arr[i-1] = bulkResp(v)
		}
	}
	return &resp.Value{Type: resp.TypeArray, Array: arr}
}

func cmdHDel(ctx *Context) *resp.Value {
	if ctx.NArgs() < 2 {
		return wrongArgCount("HDEL")
	}
	h, errV := getHash(ctx, ctx.ArgStr(0))
	if errV != nil {
		return errV
	}
	if h == nil {
		return intResp(0)
	}
	deleted := 0
	for i := 1; i < ctx.NArgs(); i++ {
		if _, ok := h[ctx.ArgStr(i)]; ok {
			delete(h, ctx.ArgStr(i))
			deleted++
		}
	}
	if len(h) == 0 {
		ctx.DB.Del(ctx.ArgStr(0))
	}
	return intResp(int64(deleted))
}

func cmdHExists(ctx *Context) *resp.Value {
	if ctx.NArgs() != 2 {
		return wrongArgCount("HEXISTS")
	}
	h, errV := getHash(ctx, ctx.ArgStr(0))
	if errV != nil {
		return errV
	}
	if h == nil {
		return intResp(0)
	}
	_, ok := h[ctx.ArgStr(1)]
	if ok {
		return intResp(1)
	}
	return intResp(0)
}

func cmdHGetAll(ctx *Context) *resp.Value {
	if ctx.NArgs() != 1 {
		return wrongArgCount("HGETALL")
	}
	h, errV := getHash(ctx, ctx.ArgStr(0))
	if errV != nil {
		return errV
	}
	if h == nil {
		return &resp.Value{Type: resp.TypeArray, Array: []*resp.Value{}}
	}
	arr := make([]*resp.Value, 0, len(h)*2)
	for k, v := range h {
		arr = append(arr, bulkResp(k), bulkResp(v))
	}
	return &resp.Value{Type: resp.TypeArray, Array: arr}
}

func cmdHKeys(ctx *Context) *resp.Value {
	if ctx.NArgs() != 1 {
		return wrongArgCount("HKEYS")
	}
	h, errV := getHash(ctx, ctx.ArgStr(0))
	if errV != nil {
		return errV
	}
	if h == nil {
		return &resp.Value{Type: resp.TypeArray, Array: []*resp.Value{}}
	}
	arr := make([]*resp.Value, 0, len(h))
	for k := range h {
		arr = append(arr, bulkResp(k))
	}
	return &resp.Value{Type: resp.TypeArray, Array: arr}
}

func cmdHVals(ctx *Context) *resp.Value {
	if ctx.NArgs() != 1 {
		return wrongArgCount("HVALS")
	}
	h, errV := getHash(ctx, ctx.ArgStr(0))
	if errV != nil {
		return errV
	}
	if h == nil {
		return &resp.Value{Type: resp.TypeArray, Array: []*resp.Value{}}
	}
	arr := make([]*resp.Value, 0, len(h))
	for _, v := range h {
		arr = append(arr, bulkResp(v))
	}
	return &resp.Value{Type: resp.TypeArray, Array: arr}
}

func cmdHLen(ctx *Context) *resp.Value {
	if ctx.NArgs() != 1 {
		return wrongArgCount("HLEN")
	}
	h, errV := getHash(ctx, ctx.ArgStr(0))
	if errV != nil {
		return errV
	}
	if h == nil {
		return intResp(0)
	}
	return intResp(int64(len(h)))
}

func cmdHIncrBy(ctx *Context) *resp.Value {
	if ctx.NArgs() != 3 {
		return wrongArgCount("HINCRBY")
	}
	delta, err := strconv.ParseInt(ctx.ArgStr(2), 10, 64)
	if err != nil {
		return errResp("ERR value is not an integer or out of range")
	}
	h, errV := getOrCreateHash(ctx, ctx.ArgStr(0))
	if errV != nil {
		return errV
	}
	field := ctx.ArgStr(1)
	var cur int64
	if v, ok := h[field]; ok {
		cur, err = strconv.ParseInt(v, 10, 64)
		if err != nil {
			return errResp("ERR hash value is not an integer")
		}
	}
	cur += delta
	h[field] = strconv.FormatInt(cur, 10)
	return intResp(cur)
}

func cmdHIncrByFloat(ctx *Context) *resp.Value {
	if ctx.NArgs() != 3 {
		return wrongArgCount("HINCRBYFLOAT")
	}
	delta, err := strconv.ParseFloat(ctx.ArgStr(2), 64)
	if err != nil {
		return errResp("ERR value is not a valid float")
	}
	h, errV := getOrCreateHash(ctx, ctx.ArgStr(0))
	if errV != nil {
		return errV
	}
	field := ctx.ArgStr(1)
	var cur float64
	if v, ok := h[field]; ok {
		cur, err = strconv.ParseFloat(v, 64)
		if err != nil {
			return errResp("ERR hash value is not a float")
		}
	}
	cur += delta
	s := strconv.FormatFloat(cur, 'f', -1, 64)
	h[field] = s
	return bulkResp(s)
}

func cmdHSetNX(ctx *Context) *resp.Value {
	if ctx.NArgs() != 3 {
		return wrongArgCount("HSETNX")
	}
	h, errV := getOrCreateHash(ctx, ctx.ArgStr(0))
	if errV != nil {
		return errV
	}
	field := ctx.ArgStr(1)
	if _, ok := h[field]; ok {
		return intResp(0)
	}
	h[field] = ctx.ArgStr(2)
	return intResp(1)
}
