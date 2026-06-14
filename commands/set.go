package commands

import (
	"math/rand"
	"strconv"
	"qwikstore/resp"
	"qwikstore/store"
)

func getOrCreateSet(ctx *Context, key string) (map[string]struct{}, *resp.Value) {
	obj, ok := ctx.DB.Get(key)
	if !ok {
		s := make(map[string]struct{})
		ctx.DB.Set(key, &store.Object{Type: store.TypeSet, Value: s})
		return s, nil
	}
	if obj.Type != store.TypeSet {
		return nil, wrongTypeErr()
	}
	return obj.Value.(map[string]struct{}), nil
}

func getSet(ctx *Context, key string) (map[string]struct{}, *resp.Value) {
	obj, ok := ctx.DB.Get(key)
	if !ok {
		return nil, nil
	}
	if obj.Type != store.TypeSet {
		return nil, wrongTypeErr()
	}
	return obj.Value.(map[string]struct{}), nil
}

func cmdSAdd(ctx *Context) *resp.Value {
	if ctx.NArgs() < 2 {
		return wrongArgCount("SADD")
	}
	s, errV := getOrCreateSet(ctx, ctx.ArgStr(0))
	if errV != nil {
		return errV
	}
	added := 0
	for i := 1; i < ctx.NArgs(); i++ {
		m := ctx.ArgStr(i)
		if _, ok := s[m]; !ok {
			s[m] = struct{}{}
			added++
		}
	}
	return intResp(int64(added))
}

func cmdSRem(ctx *Context) *resp.Value {
	if ctx.NArgs() < 2 {
		return wrongArgCount("SREM")
	}
	s, errV := getSet(ctx, ctx.ArgStr(0))
	if errV != nil {
		return errV
	}
	if s == nil {
		return intResp(0)
	}
	removed := 0
	for i := 1; i < ctx.NArgs(); i++ {
		m := ctx.ArgStr(i)
		if _, ok := s[m]; ok {
			delete(s, m)
			removed++
		}
	}
	if len(s) == 0 {
		ctx.DB.Del(ctx.ArgStr(0))
	}
	return intResp(int64(removed))
}

func cmdSMembers(ctx *Context) *resp.Value {
	if ctx.NArgs() != 1 {
		return wrongArgCount("SMEMBERS")
	}
	s, errV := getSet(ctx, ctx.ArgStr(0))
	if errV != nil {
		return errV
	}
	if s == nil {
		return &resp.Value{Type: resp.TypeArray, Array: []*resp.Value{}}
	}
	arr := make([]*resp.Value, 0, len(s))
	for m := range s {
		arr = append(arr, bulkResp(m))
	}
	return &resp.Value{Type: resp.TypeArray, Array: arr}
}

func cmdSIsMember(ctx *Context) *resp.Value {
	if ctx.NArgs() != 2 {
		return wrongArgCount("SISMEMBER")
	}
	s, errV := getSet(ctx, ctx.ArgStr(0))
	if errV != nil {
		return errV
	}
	if s == nil {
		return intResp(0)
	}
	if _, ok := s[ctx.ArgStr(1)]; ok {
		return intResp(1)
	}
	return intResp(0)
}

func cmdSMIsMember(ctx *Context) *resp.Value {
	if ctx.NArgs() < 2 {
		return wrongArgCount("SMISMEMBER")
	}
	s, errV := getSet(ctx, ctx.ArgStr(0))
	if errV != nil {
		return errV
	}
	arr := make([]*resp.Value, ctx.NArgs()-1)
	for i := 1; i < ctx.NArgs(); i++ {
		if s != nil {
			if _, ok := s[ctx.ArgStr(i)]; ok {
				arr[i-1] = intResp(1)
				continue
			}
		}
		arr[i-1] = intResp(0)
	}
	return &resp.Value{Type: resp.TypeArray, Array: arr}
}

func cmdSCard(ctx *Context) *resp.Value {
	if ctx.NArgs() != 1 {
		return wrongArgCount("SCARD")
	}
	s, errV := getSet(ctx, ctx.ArgStr(0))
	if errV != nil {
		return errV
	}
	if s == nil {
		return intResp(0)
	}
	return intResp(int64(len(s)))
}

func collectSets(ctx *Context, startIdx int) ([]map[string]struct{}, *resp.Value) {
	var sets []map[string]struct{}
	for i := startIdx; i < ctx.NArgs(); i++ {
		s, errV := getSet(ctx, ctx.ArgStr(i))
		if errV != nil {
			return nil, errV
		}
		if s == nil {
			s = make(map[string]struct{})
		}
		sets = append(sets, s)
	}
	return sets, nil
}

func setUnion(sets []map[string]struct{}) map[string]struct{} {
	result := make(map[string]struct{})
	for _, s := range sets {
		for m := range s {
			result[m] = struct{}{}
		}
	}
	return result
}

func setInter(sets []map[string]struct{}) map[string]struct{} {
	if len(sets) == 0 {
		return make(map[string]struct{})
	}
	result := make(map[string]struct{})
	for m := range sets[0] {
		in := true
		for _, s := range sets[1:] {
			if _, ok := s[m]; !ok {
				in = false
				break
			}
		}
		if in {
			result[m] = struct{}{}
		}
	}
	return result
}

func setDiff(sets []map[string]struct{}) map[string]struct{} {
	if len(sets) == 0 {
		return make(map[string]struct{})
	}
	result := make(map[string]struct{})
	for m := range sets[0] {
		in := false
		for _, s := range sets[1:] {
			if _, ok := s[m]; ok {
				in = true
				break
			}
		}
		if !in {
			result[m] = struct{}{}
		}
	}
	return result
}

func setToArray(s map[string]struct{}) *resp.Value {
	arr := make([]*resp.Value, 0, len(s))
	for m := range s {
		arr = append(arr, bulkResp(m))
	}
	return &resp.Value{Type: resp.TypeArray, Array: arr}
}

func cmdSUnion(ctx *Context) *resp.Value {
	if ctx.NArgs() < 1 {
		return wrongArgCount("SUNION")
	}
	sets, errV := collectSets(ctx, 0)
	if errV != nil {
		return errV
	}
	return setToArray(setUnion(sets))
}

func cmdSInter(ctx *Context) *resp.Value {
	if ctx.NArgs() < 1 {
		return wrongArgCount("SINTER")
	}
	sets, errV := collectSets(ctx, 0)
	if errV != nil {
		return errV
	}
	return setToArray(setInter(sets))
}

func cmdSDiff(ctx *Context) *resp.Value {
	if ctx.NArgs() < 1 {
		return wrongArgCount("SDIFF")
	}
	sets, errV := collectSets(ctx, 0)
	if errV != nil {
		return errV
	}
	return setToArray(setDiff(sets))
}

func storeSetResult(ctx *Context, dst string, s map[string]struct{}) *resp.Value {
	ctx.DB.Del(dst)
	if len(s) > 0 {
		ctx.DB.Set(dst, &store.Object{Type: store.TypeSet, Value: s})
	}
	return intResp(int64(len(s)))
}

func cmdSUnionStore(ctx *Context) *resp.Value {
	if ctx.NArgs() < 2 {
		return wrongArgCount("SUNIONSTORE")
	}
	sets, errV := collectSets(ctx, 1)
	if errV != nil {
		return errV
	}
	return storeSetResult(ctx, ctx.ArgStr(0), setUnion(sets))
}

func cmdSInterStore(ctx *Context) *resp.Value {
	if ctx.NArgs() < 2 {
		return wrongArgCount("SINTERSTORE")
	}
	sets, errV := collectSets(ctx, 1)
	if errV != nil {
		return errV
	}
	return storeSetResult(ctx, ctx.ArgStr(0), setInter(sets))
}

func cmdSDiffStore(ctx *Context) *resp.Value {
	if ctx.NArgs() < 2 {
		return wrongArgCount("SDIFFSTORE")
	}
	sets, errV := collectSets(ctx, 1)
	if errV != nil {
		return errV
	}
	return storeSetResult(ctx, ctx.ArgStr(0), setDiff(sets))
}

func cmdSMove(ctx *Context) *resp.Value {
	if ctx.NArgs() != 3 {
		return wrongArgCount("SMOVE")
	}
	src, errV := getSet(ctx, ctx.ArgStr(0))
	if errV != nil {
		return errV
	}
	if src == nil {
		return intResp(0)
	}
	member := ctx.ArgStr(2)
	if _, ok := src[member]; !ok {
		return intResp(0)
	}
	delete(src, member)
	if len(src) == 0 {
		ctx.DB.Del(ctx.ArgStr(0))
	}
	dst, errV := getOrCreateSet(ctx, ctx.ArgStr(1))
	if errV != nil {
		return errV
	}
	dst[member] = struct{}{}
	return intResp(1)
}

func cmdSPop(ctx *Context) *resp.Value {
	if ctx.NArgs() < 1 {
		return wrongArgCount("SPOP")
	}
	s, errV := getSet(ctx, ctx.ArgStr(0))
	if errV != nil {
		return errV
	}
	if s == nil {
		return nullBulkResp()
	}
	count := 1
	returnArr := false
	if ctx.NArgs() >= 2 {
		n, err := strconv.Atoi(ctx.ArgStr(1))
		if err != nil || n < 0 {
			return errResp("ERR value is not an integer or out of range")
		}
		count = n
		returnArr = true
	}
	keys := make([]string, 0, len(s))
	for k := range s {
		keys = append(keys, k)
	}
	rand.Shuffle(len(keys), func(i, j int) { keys[i], keys[j] = keys[j], keys[i] })
	if count > len(keys) {
		count = len(keys)
	}
	popped := keys[:count]
	for _, k := range popped {
		delete(s, k)
	}
	if len(s) == 0 {
		ctx.DB.Del(ctx.ArgStr(0))
	}
	if !returnArr {
		if len(popped) == 0 {
			return nullBulkResp()
		}
		return bulkResp(popped[0])
	}
	return stringsToArray(popped)
}

func cmdSRandMember(ctx *Context) *resp.Value {
	if ctx.NArgs() < 1 {
		return wrongArgCount("SRANDMEMBER")
	}
	s, errV := getSet(ctx, ctx.ArgStr(0))
	if errV != nil {
		return errV
	}
	if s == nil {
		if ctx.NArgs() >= 2 {
			return &resp.Value{Type: resp.TypeArray, Array: []*resp.Value{}}
		}
		return nullBulkResp()
	}
	keys := make([]string, 0, len(s))
	for k := range s {
		keys = append(keys, k)
	}
	if ctx.NArgs() < 2 {
		return bulkResp(keys[rand.Intn(len(keys))])
	}
	count, err := strconv.Atoi(ctx.ArgStr(1))
	if err != nil {
		return errResp("ERR value is not an integer or out of range")
	}
	if count >= 0 {
		if count > len(keys) {
			count = len(keys)
		}
		rand.Shuffle(len(keys), func(i, j int) { keys[i], keys[j] = keys[j], keys[i] })
		return stringsToArray(keys[:count])
	}
	// negative count: allow repeats
	count = -count
	result := make([]string, count)
	for i := 0; i < count; i++ {
		result[i] = keys[rand.Intn(len(keys))]
	}
	return stringsToArray(result)
}
