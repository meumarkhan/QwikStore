package datastructures

import (
	"math"
	"math/rand"
)

const (
	skiplistMaxLevel = 32
	skiplistP        = 0.25
)

// ZSetEntry is returned from range queries.
type ZSetEntry struct {
	Member string
	Score  float64
}

type skiplistNode struct {
	member  string
	score   float64
	backward *skiplistNode
	levels  []skiplistLevel
}

type skiplistLevel struct {
	forward *skiplistNode
	span    int // number of nodes skipped
}

// Skiplist is a probabilistic data structure backing sorted sets.
// It maintains elements ordered by score, with member as tiebreaker.
type Skiplist struct {
	header *skiplistNode
	tail   *skiplistNode
	length int
	level  int
}

func newSkiplistNode(level int, score float64, member string) *skiplistNode {
	return &skiplistNode{
		score:  score,
		member: member,
		levels: make([]skiplistLevel, level),
	}
}

// Member returns the node's member string.
func (n *skiplistNode) Member() string { return n.member }

// ScoreVal returns the node's score.
func (n *skiplistNode) ScoreVal() float64 { return n.score }

func NewSkiplist() *Skiplist {
	header := newSkiplistNode(skiplistMaxLevel, -math.MaxFloat64, "")
	return &Skiplist{
		header: header,
		level:  1,
	}
}

func (sl *Skiplist) Len() int {
	return sl.length
}

func (sl *Skiplist) randomLevel() int {
	level := 1
	for level < skiplistMaxLevel && rand.Float64() < skiplistP {
		level++
	}
	return level
}

// Insert adds a (score, member) pair. Returns the new node.
func (sl *Skiplist) Insert(score float64, member string) *skiplistNode {
	update := make([]*skiplistNode, skiplistMaxLevel)
	rank := make([]int, skiplistMaxLevel)

	x := sl.header
	for i := sl.level - 1; i >= 0; i-- {
		if i == sl.level-1 {
			rank[i] = 0
		} else {
			rank[i] = rank[i+1]
		}
		for x.levels[i].forward != nil &&
			(x.levels[i].forward.score < score ||
				(x.levels[i].forward.score == score && x.levels[i].forward.member < member)) {
			rank[i] += x.levels[i].span
			x = x.levels[i].forward
		}
		update[i] = x
	}

	level := sl.randomLevel()
	if level > sl.level {
		for i := sl.level; i < level; i++ {
			rank[i] = 0
			update[i] = sl.header
			update[i].levels[i].span = sl.length
		}
		sl.level = level
	}

	x = newSkiplistNode(level, score, member)
	for i := 0; i < level; i++ {
		x.levels[i].forward = update[i].levels[i].forward
		update[i].levels[i].forward = x
		x.levels[i].span = update[i].levels[i].span - (rank[0] - rank[i])
		update[i].levels[i].span = (rank[0] - rank[i]) + 1
	}
	for i := level; i < sl.level; i++ {
		update[i].levels[i].span++
	}

	if update[0] == sl.header {
		x.backward = nil
	} else {
		x.backward = update[0]
	}
	if x.levels[0].forward != nil {
		x.levels[0].forward.backward = x
	} else {
		sl.tail = x
	}
	sl.length++
	return x
}

// Delete removes a (score, member) pair. Returns true if found.
func (sl *Skiplist) Delete(score float64, member string) bool {
	update := make([]*skiplistNode, skiplistMaxLevel)
	x := sl.header
	for i := sl.level - 1; i >= 0; i-- {
		for x.levels[i].forward != nil &&
			(x.levels[i].forward.score < score ||
				(x.levels[i].forward.score == score && x.levels[i].forward.member < member)) {
			x = x.levels[i].forward
		}
		update[i] = x
	}

	x = x.levels[0].forward
	if x == nil || x.score != score || x.member != member {
		return false
	}
	sl.deleteNode(x, update)
	return true
}

func (sl *Skiplist) deleteNode(x *skiplistNode, update []*skiplistNode) {
	for i := 0; i < sl.level; i++ {
		if update[i].levels[i].forward == x {
			update[i].levels[i].span += x.levels[i].span - 1
			update[i].levels[i].forward = x.levels[i].forward
		} else {
			update[i].levels[i].span--
		}
	}
	if x.levels[0].forward != nil {
		x.levels[0].forward.backward = x.backward
	} else {
		sl.tail = x.backward
	}
	for sl.level > 1 && sl.header.levels[sl.level-1].forward == nil {
		sl.level--
	}
	sl.length--
}

// GetRank returns the 1-based rank of (score, member), or 0 if not found.
func (sl *Skiplist) GetRank(score float64, member string) int {
	rank := 0
	x := sl.header
	for i := sl.level - 1; i >= 0; i-- {
		for x.levels[i].forward != nil &&
			(x.levels[i].forward.score < score ||
				(x.levels[i].forward.score == score && x.levels[i].forward.member <= member)) {
			rank += x.levels[i].span
			x = x.levels[i].forward
		}
		if x.member == member {
			return rank
		}
	}
	return 0
}

// GetByRank returns the node at 1-based rank, or nil.
func (sl *Skiplist) GetByRank(rank int) *skiplistNode {
	traversed := 0
	x := sl.header
	for i := sl.level - 1; i >= 0; i-- {
		for x.levels[i].forward != nil && traversed+x.levels[i].span <= rank {
			traversed += x.levels[i].span
			x = x.levels[i].forward
		}
		if traversed == rank {
			return x
		}
	}
	return nil
}

// RangeByRank returns entries from rank start to stop (1-based, inclusive).
func (sl *Skiplist) RangeByRank(start, stop int) []ZSetEntry {
	if start < 1 {
		start = 1
	}
	if stop > sl.length {
		stop = sl.length
	}
	if start > stop {
		return nil
	}
	node := sl.GetByRank(start)
	var result []ZSetEntry
	for node != nil && start <= stop {
		result = append(result, ZSetEntry{Member: node.member, Score: node.score})
		node = node.levels[0].forward
		start++
	}
	return result
}

// RangeByScore returns entries with min <= score <= max.
func (sl *Skiplist) RangeByScore(min, max float64, offset, count int) []ZSetEntry {
	var result []ZSetEntry
	x := sl.header
	for i := sl.level - 1; i >= 0; i-- {
		for x.levels[i].forward != nil && x.levels[i].forward.score < min {
			x = x.levels[i].forward
		}
	}
	x = x.levels[0].forward
	skipped := 0
	for x != nil && x.score <= max {
		if skipped < offset {
			skipped++
			x = x.levels[0].forward
			continue
		}
		result = append(result, ZSetEntry{Member: x.member, Score: x.score})
		if count > 0 && len(result) >= count {
			break
		}
		x = x.levels[0].forward
	}
	return result
}

// RevRangeByScore returns entries with max >= score >= min (reverse order).
func (sl *Skiplist) RevRangeByScore(max, min float64, offset, count int) []ZSetEntry {
	var result []ZSetEntry
	x := sl.header
	for i := sl.level - 1; i >= 0; i-- {
		for x.levels[i].forward != nil && x.levels[i].forward.score <= max {
			x = x.levels[i].forward
		}
	}
	skipped := 0
	for x != nil && x != sl.header && x.score >= min {
		if skipped < offset {
			skipped++
			x = x.backward
			continue
		}
		result = append(result, ZSetEntry{Member: x.member, Score: x.score})
		if count > 0 && len(result) >= count {
			break
		}
		x = x.backward
	}
	return result
}

// CountByScore returns the number of elements with min <= score <= max.
func (sl *Skiplist) CountByScore(min, max float64) int {
	count := 0
	x := sl.header
	for i := sl.level - 1; i >= 0; i-- {
		for x.levels[i].forward != nil && x.levels[i].forward.score < min {
			x = x.levels[i].forward
		}
	}
	x = x.levels[0].forward
	for x != nil && x.score <= max {
		count++
		x = x.levels[0].forward
	}
	return count
}

// First returns the node with the lowest score, or nil.
func (sl *Skiplist) First() *skiplistNode {
	return sl.header.levels[0].forward
}

// Last returns the node with the highest score, or nil.
func (sl *Skiplist) Last() *skiplistNode {
	return sl.tail
}
