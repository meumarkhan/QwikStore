package datastructures

// ListNode is a node in a doubly linked list.
type ListNode struct {
	Value string
	prev  *ListNode
	next  *ListNode
}

// List is a doubly linked list used to back Redis List objects.
type List struct {
	head   *ListNode
	tail   *ListNode
	length int
}

func NewList() *List {
	return &List{}
}

func (l *List) Len() int {
	return l.length
}

func (l *List) LPush(values ...string) {
	for _, v := range values {
		node := &ListNode{Value: v}
		if l.head == nil {
			l.head = node
			l.tail = node
		} else {
			node.next = l.head
			l.head.prev = node
			l.head = node
		}
		l.length++
	}
}

func (l *List) RPush(values ...string) {
	for _, v := range values {
		node := &ListNode{Value: v}
		if l.tail == nil {
			l.head = node
			l.tail = node
		} else {
			node.prev = l.tail
			l.tail.next = node
			l.tail = node
		}
		l.length++
	}
}

func (l *List) LPop() (string, bool) {
	if l.head == nil {
		return "", false
	}
	val := l.head.Value
	l.head = l.head.next
	if l.head != nil {
		l.head.prev = nil
	} else {
		l.tail = nil
	}
	l.length--
	return val, true
}

func (l *List) RPop() (string, bool) {
	if l.tail == nil {
		return "", false
	}
	val := l.tail.Value
	l.tail = l.tail.prev
	if l.tail != nil {
		l.tail.next = nil
	} else {
		l.head = nil
	}
	l.length--
	return val, true
}

// Index returns the element at the given index (0-based, negative counts from end).
func (l *List) Index(idx int) (string, bool) {
	node := l.nodeAt(idx)
	if node == nil {
		return "", false
	}
	return node.Value, true
}

// Set replaces the element at the given index.
func (l *List) Set(idx int, value string) bool {
	node := l.nodeAt(idx)
	if node == nil {
		return false
	}
	node.Value = value
	return true
}

// Range returns a slice of values between start and stop (inclusive, negative ok).
func (l *List) Range(start, stop int) []string {
	start, stop = l.normalizeRange(start, stop)
	if start > stop {
		return nil
	}
	var result []string
	node := l.nodeAt(start)
	for i := start; i <= stop && node != nil; i++ {
		result = append(result, node.Value)
		node = node.next
	}
	return result
}

// Trim removes elements outside [start, stop].
func (l *List) Trim(start, stop int) {
	start, stop = l.normalizeRange(start, stop)
	if start > stop {
		l.head = nil
		l.tail = nil
		l.length = 0
		return
	}
	newHead := l.nodeAt(start)
	newTail := l.nodeAt(stop)
	if newHead != nil {
		newHead.prev = nil
	}
	if newTail != nil {
		newTail.next = nil
	}
	l.head = newHead
	l.tail = newTail
	l.length = stop - start + 1
}

// InsertBefore inserts newVal before the first occurrence of pivot. Returns new length or -1 if pivot not found.
func (l *List) InsertBefore(pivot, newVal string) int {
	for node := l.head; node != nil; node = node.next {
		if node.Value == pivot {
			l.insertBefore(node, newVal)
			return l.length
		}
	}
	return -1
}

// InsertAfter inserts newVal after the first occurrence of pivot.
func (l *List) InsertAfter(pivot, newVal string) int {
	for node := l.head; node != nil; node = node.next {
		if node.Value == pivot {
			l.insertAfter(node, newVal)
			return l.length
		}
	}
	return -1
}

func (l *List) insertBefore(node *ListNode, val string) {
	n := &ListNode{Value: val, next: node, prev: node.prev}
	if node.prev != nil {
		node.prev.next = n
	} else {
		l.head = n
	}
	node.prev = n
	l.length++
}

func (l *List) insertAfter(node *ListNode, val string) {
	n := &ListNode{Value: val, prev: node, next: node.next}
	if node.next != nil {
		node.next.prev = n
	} else {
		l.tail = n
	}
	node.next = n
	l.length++
}

// Remove removes up to count occurrences of element. count>0: from head; count<0: from tail; count=0: all.
func (l *List) Remove(count int, element string) int {
	removed := 0
	fromTail := count < 0
	if count < 0 {
		count = -count
	}
	var node *ListNode
	if fromTail {
		node = l.tail
	} else {
		node = l.head
	}
	for node != nil && (count == 0 || removed < count) {
		var next *ListNode
		if fromTail {
			next = node.prev
		} else {
			next = node.next
		}
		if node.Value == element {
			l.removeNode(node)
			removed++
		}
		node = next
	}
	return removed
}

func (l *List) removeNode(node *ListNode) {
	if node.prev != nil {
		node.prev.next = node.next
	} else {
		l.head = node.next
	}
	if node.next != nil {
		node.next.prev = node.prev
	} else {
		l.tail = node.prev
	}
	l.length--
}

func (l *List) nodeAt(idx int) *ListNode {
	if idx < 0 {
		idx = l.length + idx
	}
	if idx < 0 || idx >= l.length {
		return nil
	}
	if idx < l.length/2 {
		node := l.head
		for i := 0; i < idx; i++ {
			node = node.next
		}
		return node
	}
	node := l.tail
	for i := l.length - 1; i > idx; i-- {
		node = node.prev
	}
	return node
}

func (l *List) normalizeRange(start, stop int) (int, int) {
	if start < 0 {
		start = l.length + start
	}
	if stop < 0 {
		stop = l.length + stop
	}
	if start < 0 {
		start = 0
	}
	if stop >= l.length {
		stop = l.length - 1
	}
	return start, stop
}
