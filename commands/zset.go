package commands

import (
	"math"
	"strconv"
	"strings"
	"qwikstore/datastructures"
	"qwikstore/resp"
	"qwikstore/store"
)

func getOrCreateZSet(ctx *Context, key string) (*store.ZSet, *resp.Value) {
	obj, ok := ctx.DB.Get(key)
	if !ok {
		z := store.NewZSet()
		ctx.DB.Set(key, &store.Object{Type: store.TypeZSet, Value: z})
		return z, nil
	}
	if obj.Type != store.TypeZSet {
		return nil, wrongTypeErr()
	}
	return obj.Value.(*store.ZSet), nil
}

func getZSet(ctx *Context, key string) (*store.ZSet, *resp.Value) {
	obj, ok := ctx.DB.Get(key)
	if !ok {
		return nil, nil
	}
	if obj.Type != store.TypeZSet {
		return nil, wrongTypeErr()
	}
	return obj.Value.(*store.ZSet), nil
}

func parseScore(s string) (float64, error) {
	s = strings.ToLower(s)
	if s == "+inf" || s == "inf" {
		return math.Inf(1), nil
	}
	if s == "-inf" {
		return math.Inf(-1), nil
	}
	return strconv.ParseFloat(s, 64)
}

func parseScoreRange(minStr, maxStr string) (float64, float64, error) {
	min, err := parseScore(minStr)
	if err != nil {
		return 0, 0, err
	}
	max, err := parseScore(maxStr)
	if err != nil {
		return 0, 0, err
	}
	return min, max, nil
}

func cmdZAdd(ctx *Context) *resp.Value {
	if ctx.NArgs() < 3 {
		return wrongArgCount("ZADD")
	}
	key := ctx.ArgStr(0)
	z, errV := getOrCreateZSet(ctx, key)
	if errV != nil {
		return errV
	}

	// Parse options
	nx, xx, gt, lt, ch := false, false, false, false, false
	i := 1
	for ; i < ctx.NArgs(); i++ {
		switch strings.ToUpper(ctx.ArgStr(i)) {
		case "NX":
			nx = true
		case "XX":
			xx = true
		case "GT":
			gt = true
		case "LT":
			lt = true
		case "CH":
			ch = true
		default:
			goto parseMembers
		}
	}
parseMembers:
	if (ctx.NArgs()-i)%2 != 0 {
		return errResp("ERR syntax error")
	}

	added := 0
	changed := 0
	for ; i < ctx.NArgs(); i += 2 {
		score, err := parseScore(ctx.ArgStr(i))
		if err != nil {
			return errResp("ERR value is not a valid float")
		}
		member := ctx.ArgStr(i + 1)
		oldScore, exists := z.Score(member)
		if nx && exists {
			continue
		}
		if xx && !exists {
			continue
		}
		if gt && exists && score <= oldScore {
			continue
		}
		if lt && exists && score >= oldScore {
			continue
		}
		wasNew := z.Add(score, member)
		if wasNew {
			added++
		} else {
			changed++
		}
	}
	_ = lt
	if ch {
		return intResp(int64(added + changed))
	}
	return intResp(int64(added))
}

func cmdZRem(ctx *Context) *resp.Value {
	if ctx.NArgs() < 2 {
		return wrongArgCount("ZREM")
	}
	z, errV := getZSet(ctx, ctx.ArgStr(0))
	if errV != nil {
		return errV
	}
	if z == nil {
		return intResp(0)
	}
	removed := 0
	for i := 1; i < ctx.NArgs(); i++ {
		if z.Remove(ctx.ArgStr(i)) {
			removed++
		}
	}
	if z.Len() == 0 {
		ctx.DB.Del(ctx.ArgStr(0))
	}
	return intResp(int64(removed))
}

func cmdZScore(ctx *Context) *resp.Value {
	if ctx.NArgs() != 2 {
		return wrongArgCount("ZSCORE")
	}
	z, errV := getZSet(ctx, ctx.ArgStr(0))
	if errV != nil {
		return errV
	}
	if z == nil {
		return nullBulkResp()
	}
	score, ok := z.Score(ctx.ArgStr(1))
	if !ok {
		return nullBulkResp()
	}
	return bulkResp(strconv.FormatFloat(score, 'f', -1, 64))
}

func cmdZIncrBy(ctx *Context) *resp.Value {
	if ctx.NArgs() != 3 {
		return wrongArgCount("ZINCRBY")
	}
	delta, err := parseScore(ctx.ArgStr(1))
	if err != nil {
		return errResp("ERR value is not a valid float")
	}
	z, errV := getOrCreateZSet(ctx, ctx.ArgStr(0))
	if errV != nil {
		return errV
	}
	member := ctx.ArgStr(2)
	cur, _ := z.Score(member)
	newScore := cur + delta
	z.Add(newScore, member)
	return bulkResp(strconv.FormatFloat(newScore, 'f', -1, 64))
}

func cmdZRank(ctx *Context) *resp.Value {
	if ctx.NArgs() != 2 {
		return wrongArgCount("ZRANK")
	}
	z, errV := getZSet(ctx, ctx.ArgStr(0))
	if errV != nil {
		return errV
	}
	if z == nil {
		return nullBulkResp()
	}
	rank, ok := z.Rank(ctx.ArgStr(1))
	if !ok {
		return nullBulkResp()
	}
	return intResp(int64(rank))
}

func cmdZRevRank(ctx *Context) *resp.Value {
	if ctx.NArgs() != 2 {
		return wrongArgCount("ZREVRANK")
	}
	z, errV := getZSet(ctx, ctx.ArgStr(0))
	if errV != nil {
		return errV
	}
	if z == nil {
		return nullBulkResp()
	}
	rank, ok := z.RevRank(ctx.ArgStr(1))
	if !ok {
		return nullBulkResp()
	}
	return intResp(int64(rank))
}

func cmdZRange(ctx *Context) *resp.Value {
	if ctx.NArgs() < 3 {
		return wrongArgCount("ZRANGE")
	}
	z, errV := getZSet(ctx, ctx.ArgStr(0))
	if errV != nil {
		return errV
	}
	if z == nil {
		return &resp.Value{Type: resp.TypeArray, Array: []*resp.Value{}}
	}
	start, err1 := strconv.Atoi(ctx.ArgStr(1))
	stop, err2 := strconv.Atoi(ctx.ArgStr(2))
	if err1 != nil || err2 != nil {
		return errResp("ERR value is not an integer or out of range")
	}
	withScores := ctx.NArgs() >= 4 && strings.ToUpper(ctx.ArgStr(3)) == "WITHSCORES"
	n := z.Len()
	if start < 0 {
		start = n + start
	}
	if stop < 0 {
		stop = n + stop
	}
	if start < 0 {
		start = 0
	}
	if stop >= n {
		stop = n - 1
	}
	entries := z.Skiplist().RangeByRank(start+1, stop+1)
	return zsetEntriesToResp(entries, withScores)
}

func cmdZRevRange(ctx *Context) *resp.Value {
	if ctx.NArgs() < 3 {
		return wrongArgCount("ZREVRANGE")
	}
	z, errV := getZSet(ctx, ctx.ArgStr(0))
	if errV != nil {
		return errV
	}
	if z == nil {
		return &resp.Value{Type: resp.TypeArray, Array: []*resp.Value{}}
	}
	start, err1 := strconv.Atoi(ctx.ArgStr(1))
	stop, err2 := strconv.Atoi(ctx.ArgStr(2))
	if err1 != nil || err2 != nil {
		return errResp("ERR value is not an integer or out of range")
	}
	withScores := ctx.NArgs() >= 4 && strings.ToUpper(ctx.ArgStr(3)) == "WITHSCORES"
	n := z.Len()
	if start < 0 {
		start = n + start
	}
	if stop < 0 {
		stop = n + stop
	}
	// Reverse: rank from end
	revStart := n - 1 - stop
	revStop := n - 1 - start
	if revStart < 0 {
		revStart = 0
	}
	if revStop >= n {
		revStop = n - 1
	}
	entries := z.Skiplist().RangeByRank(revStart+1, revStop+1)
	// Reverse the slice
	for i, j := 0, len(entries)-1; i < j; i, j = i+1, j-1 {
		entries[i], entries[j] = entries[j], entries[i]
	}
	return zsetEntriesToResp(entries, withScores)
}

func parseLimitClause(ctx *Context, startIdx int) (offset, count int, parseErr bool) {
	count = -1
	for i := startIdx; i < ctx.NArgs(); i++ {
		if strings.ToUpper(ctx.ArgStr(i)) == "LIMIT" {
			if i+2 >= ctx.NArgs() {
				return 0, 0, true
			}
			var err error
			offset, err = strconv.Atoi(ctx.ArgStr(i + 1))
			if err != nil {
				return 0, 0, true
			}
			count, err = strconv.Atoi(ctx.ArgStr(i + 2))
			if err != nil {
				return 0, 0, true
			}
			return
		}
	}
	return
}

func cmdZRangeByScore(ctx *Context) *resp.Value {
	if ctx.NArgs() < 3 {
		return wrongArgCount("ZRANGEBYSCORE")
	}
	z, errV := getZSet(ctx, ctx.ArgStr(0))
	if errV != nil {
		return errV
	}
	if z == nil {
		return &resp.Value{Type: resp.TypeArray, Array: []*resp.Value{}}
	}
	min, max, err := parseScoreRange(ctx.ArgStr(1), ctx.ArgStr(2))
	if err != nil {
		return errResp("ERR min or max is not a float")
	}
	withScores := false
	for i := 3; i < ctx.NArgs(); i++ {
		if strings.ToUpper(ctx.ArgStr(i)) == "WITHSCORES" {
			withScores = true
		}
	}
	offset, count, _ := parseLimitClause(ctx, 3)
	entries := z.Skiplist().RangeByScore(min, max, offset, count)
	return zsetEntriesToResp(entries, withScores)
}

func cmdZRevRangeByScore(ctx *Context) *resp.Value {
	if ctx.NArgs() < 3 {
		return wrongArgCount("ZREVRANGEBYSCORE")
	}
	z, errV := getZSet(ctx, ctx.ArgStr(0))
	if errV != nil {
		return errV
	}
	if z == nil {
		return &resp.Value{Type: resp.TypeArray, Array: []*resp.Value{}}
	}
	max, min, err := parseScoreRange(ctx.ArgStr(1), ctx.ArgStr(2))
	if err != nil {
		return errResp("ERR min or max is not a float")
	}
	withScores := false
	for i := 3; i < ctx.NArgs(); i++ {
		if strings.ToUpper(ctx.ArgStr(i)) == "WITHSCORES" {
			withScores = true
		}
	}
	offset, count, _ := parseLimitClause(ctx, 3)
	entries := z.Skiplist().RevRangeByScore(max, min, offset, count)
	return zsetEntriesToResp(entries, withScores)
}

func cmdZCard(ctx *Context) *resp.Value {
	if ctx.NArgs() != 1 {
		return wrongArgCount("ZCARD")
	}
	z, errV := getZSet(ctx, ctx.ArgStr(0))
	if errV != nil {
		return errV
	}
	if z == nil {
		return intResp(0)
	}
	return intResp(int64(z.Len()))
}

func cmdZCount(ctx *Context) *resp.Value {
	if ctx.NArgs() != 3 {
		return wrongArgCount("ZCOUNT")
	}
	z, errV := getZSet(ctx, ctx.ArgStr(0))
	if errV != nil {
		return errV
	}
	if z == nil {
		return intResp(0)
	}
	min, max, err := parseScoreRange(ctx.ArgStr(1), ctx.ArgStr(2))
	if err != nil {
		return errResp("ERR min or max is not a float")
	}
	return intResp(int64(z.Skiplist().CountByScore(min, max)))
}

func cmdZPopMin(ctx *Context) *resp.Value {
	if ctx.NArgs() < 1 {
		return wrongArgCount("ZPOPMIN")
	}
	z, errV := getZSet(ctx, ctx.ArgStr(0))
	if errV != nil {
		return errV
	}
	if z == nil {
		return &resp.Value{Type: resp.TypeArray, Array: []*resp.Value{}}
	}
	count := 1
	if ctx.NArgs() >= 2 {
		n, err := strconv.Atoi(ctx.ArgStr(1))
		if err != nil || n < 0 {
			return errResp("ERR value is not an integer or out of range")
		}
		count = n
	}
	var result []datastructures.ZSetEntry
	for i := 0; i < count; i++ {
		first := z.Skiplist().First()
		if first == nil {
			break
		}
		result = append(result, datastructures.ZSetEntry{Member: first.Member(), Score: first.ScoreVal()})
		z.Remove(first.Member())
	}
	if z.Len() == 0 {
		ctx.DB.Del(ctx.ArgStr(0))
	}
	return zsetEntriesToResp(result, true)
}

func cmdZPopMax(ctx *Context) *resp.Value {
	if ctx.NArgs() < 1 {
		return wrongArgCount("ZPOPMAX")
	}
	z, errV := getZSet(ctx, ctx.ArgStr(0))
	if errV != nil {
		return errV
	}
	if z == nil {
		return &resp.Value{Type: resp.TypeArray, Array: []*resp.Value{}}
	}
	count := 1
	if ctx.NArgs() >= 2 {
		n, err := strconv.Atoi(ctx.ArgStr(1))
		if err != nil || n < 0 {
			return errResp("ERR value is not an integer or out of range")
		}
		count = n
	}
	var result []datastructures.ZSetEntry
	for i := 0; i < count; i++ {
		last := z.Skiplist().Last()
		if last == nil {
			break
		}
		result = append(result, datastructures.ZSetEntry{Member: last.Member(), Score: last.ScoreVal()})
		z.Remove(last.Member())
	}
	if z.Len() == 0 {
		ctx.DB.Del(ctx.ArgStr(0))
	}
	return zsetEntriesToResp(result, true)
}

func cmdZRangeByLex(ctx *Context) *resp.Value {
	// Simplified: treat as ZRANGE with no scores filter
	if ctx.NArgs() < 3 {
		return wrongArgCount("ZRANGEBYLEX")
	}
	z, errV := getZSet(ctx, ctx.ArgStr(0))
	if errV != nil {
		return errV
	}
	if z == nil {
		return &resp.Value{Type: resp.TypeArray, Array: []*resp.Value{}}
	}
	// Return all members in lex order between [min, [max
	minStr := ctx.ArgStr(1)
	maxStr := ctx.ArgStr(2)
	members := z.Members()
	var result []string
	for m := range members {
		if lexInRange(m, minStr, maxStr) {
			result = append(result, m)
		}
	}
	// sort lexicographically
	sortStrings(result)
	return stringsToArray(result)
}

func lexInRange(member, minStr, maxStr string) bool {
	if minStr == "-" && maxStr == "+" {
		return true
	}
	minExcl := strings.HasPrefix(minStr, "(")
	maxExcl := strings.HasPrefix(maxStr, "(")
	if minExcl {
		minStr = minStr[1:]
	} else if strings.HasPrefix(minStr, "[") {
		minStr = minStr[1:]
	}
	if maxExcl {
		maxStr = maxStr[1:]
	} else if strings.HasPrefix(maxStr, "[") {
		maxStr = maxStr[1:]
	}

	if minStr != "-" {
		if minExcl && member <= minStr {
			return false
		}
		if !minExcl && member < minStr {
			return false
		}
	}
	if maxStr != "+" {
		if maxExcl && member >= maxStr {
			return false
		}
		if !maxExcl && member > maxStr {
			return false
		}
	}
	return true
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

func zsetEntriesToResp(entries []datastructures.ZSetEntry, withScores bool) *resp.Value {
	if !withScores {
		arr := make([]*resp.Value, len(entries))
		for i, e := range entries {
			arr[i] = bulkResp(e.Member)
		}
		return &resp.Value{Type: resp.TypeArray, Array: arr}
	}
	arr := make([]*resp.Value, len(entries)*2)
	for i, e := range entries {
		arr[i*2] = bulkResp(e.Member)
		arr[i*2+1] = bulkResp(strconv.FormatFloat(e.Score, 'f', -1, 64))
	}
	return &resp.Value{Type: resp.TypeArray, Array: arr}
}
