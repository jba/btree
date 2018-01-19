// Copyright 2014 Google Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package btree implements in-memory B-Trees of arbitrary degree.
//
// btree implements an in-memory B-Tree for use as an ordered data structure.
// It is not meant for persistent storage solutions.
//
// It has a flatter structure than an equivalent red-black or other binary tree,
// which in some cases yields better memory usage and/or performance.
// See some discussion on the matter here:
//   http://google-opensource.blogspot.com/2013/01/c-containers-that-save-memory-and-time.html
// Note, though, that this project is in no way related to the C++ B-Tree
// implementation written about there.
//
// Within this tree, each node contains a slice of items and a (possibly nil)
// slice of children.  For basic numeric values or raw structs, this can cause
// efficiency differences when compared to equivalent C++ template code that
// stores values in arrays within the node:
//   * Due to the overhead of storing values as interfaces (each
//     value needs to be stored as the value itself, then 2 words for the
//     interface pointing to that value and its type), resulting in higher
//     memory use.
//   * Since interfaces can point to values anywhere in memory, values are
//     most likely not stored in contiguous blocks, resulting in a higher
//     number of cache misses.
// These issues don't tend to matter, though, when working with strings or other
// heap-allocated structures, since C++-equivalent structures also must store
// pointers and also distribute their values across the heap.
//
// This implementation is based on google/btree (http://github.com/google/btree), and much of
// the code is taken from there. But the API has been changed significantly.
package btree

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
)

// Key represents a key into the tree.
type Key interface {
	// Less tests whether the current item is less than the given argument.
	//
	// This must provide a strict weak ordering.
	// If !a.Less(b) && !b.Less(a), we treat this to mean a == b (i.e. we can only
	// hold one of either a or b in the tree).
	Less(than Key) bool
}

type Value interface{}

// Item is a key-value pair.
type Item struct {
	Key   Key
	Value Value
}

// New creates a new B-Tree with the given degree.
//
// New(2), for example, will create a 2-3-4 tree (each node contains 1-3 items
// and 2-4 children).
func New(degree int) *BTree {
	if degree <= 1 {
		panic("bad degree")
	}
	return &BTree{
		degree: degree,
		cow:    &copyOnWriteContext{},
	}
}

// items stores items in a node.
type items []Item

// insertAt inserts a value into the given index, pushing all subsequent values
// forward.
func (s *items) insertAt(index int, item Item) {
	*s = append(*s, Item{})
	if index < len(*s) {
		copy((*s)[index+1:], (*s)[index:])
	}
	(*s)[index] = item
}

// removeAt removes a value at a given index, pulling all subsequent values
// back.
func (s *items) removeAt(index int) Item {
	item := (*s)[index]
	copy((*s)[index:], (*s)[index+1:])
	(*s)[len(*s)-1] = Item{}
	*s = (*s)[:len(*s)-1]
	return item
}

// pop removes and returns the last element in the list.
func (s *items) pop() Item {
	index := len(*s) - 1
	out := (*s)[index]
	(*s)[index] = Item{}
	*s = (*s)[:index]
	return out
}

var nilItems = make(items, 16)

// truncate truncates this instance at index so that it contains only the
// first index items. index must be less than or equal to length.
func (s *items) truncate(index int) {
	var toClear items
	*s, toClear = (*s)[:index], (*s)[index:]
	for len(toClear) > 0 {
		toClear = toClear[copy(toClear, nilItems):]
	}
}

// find returns the index where an item with key should be inserted into this
// list.  'found' is true if the item already exists in the list at the given
// index.
func (s items) find(key Key) (index int, found bool) {
	i := sort.Search(len(s), func(i int) bool { return key.Less(s[i].Key) })
	// i is the smallest index of s for which key.Less(s[i].Key), or len(s).
	if i > 0 && !s[i-1].Key.Less(key) {
		return i - 1, true
	}
	return i, false
}

// children stores child nodes in a node.
type children []*node

// insertAt inserts a value into the given index, pushing all subsequent values
// forward.
func (s *children) insertAt(index int, n *node) {
	*s = append(*s, nil)
	if index < len(*s) {
		copy((*s)[index+1:], (*s)[index:])
	}
	(*s)[index] = n
}

// removeAt removes a value at a given index, pulling all subsequent values
// back.
func (s *children) removeAt(index int) *node {
	n := (*s)[index]
	copy((*s)[index:], (*s)[index+1:])
	(*s)[len(*s)-1] = nil
	*s = (*s)[:len(*s)-1]
	return n
}

// pop removes and returns the last element in the list.
func (s *children) pop() (out *node) {
	index := len(*s) - 1
	out = (*s)[index]
	(*s)[index] = nil
	*s = (*s)[:index]
	return
}

var nilChildren = make(children, 16)

// truncate truncates this instance at index so that it contains only the
// first index children. index must be less than or equal to length.
func (s *children) truncate(index int) {
	var toClear children
	*s, toClear = (*s)[:index], (*s)[index:]
	for len(toClear) > 0 {
		toClear = toClear[copy(toClear, nilChildren):]
	}
}

// node is an internal node in a tree.
//
// It must at all times maintain the invariant that either
//   * len(children) == 0, len(items) unconstrained
//   * len(children) == len(items) + 1
type node struct {
	items    items
	children children
	cow      *copyOnWriteContext
}

func (n *node) mutableFor(cow *copyOnWriteContext) *node {
	if n.cow == cow {
		return n
	}
	out := cow.newNode()
	if cap(out.items) >= len(n.items) {
		out.items = out.items[:len(n.items)]
	} else {
		out.items = make(items, len(n.items), cap(n.items))
	}
	copy(out.items, n.items)
	// Copy children
	if cap(out.children) >= len(n.children) {
		out.children = out.children[:len(n.children)]
	} else {
		out.children = make(children, len(n.children), cap(n.children))
	}
	copy(out.children, n.children)
	return out
}

func (n *node) mutableChild(i int) *node {
	c := n.children[i].mutableFor(n.cow)
	n.children[i] = c
	return c
}

// split splits the given node at the given index.  The current node shrinks,
// and this function returns the item that existed at that index and a new node
// containing all items/children after it.
func (n *node) split(i int) (Item, *node) {
	item := n.items[i]
	next := n.cow.newNode()
	next.items = append(next.items, n.items[i+1:]...)
	n.items.truncate(i)
	if len(n.children) > 0 {
		next.children = append(next.children, n.children[i+1:]...)
		n.children.truncate(i + 1)
	}
	return item, next
}

// maybeSplitChild checks if a child should be split, and if so splits it.
// Returns whether or not a split occurred.
func (n *node) maybeSplitChild(i, maxItems int) bool {
	if len(n.children[i].items) < maxItems {
		return false
	}
	first := n.mutableChild(i)
	item, second := first.split(maxItems / 2)
	n.items.insertAt(i, item)
	n.children.insertAt(i+1, second)
	return true
}

// insert inserts an item into the subtree rooted at this node, making sure
// no nodes in the subtree exceed maxItems items.  Should an equivalent item be
// be found/replaced by insert, its value will be returned.
func (n *node) insert(item Item, maxItems int) (old Value, present bool) {
	i, found := n.items.find(item.Key)
	if found {
		out := n.items[i]
		n.items[i] = item
		return out.Value, true
	}
	if len(n.children) == 0 {
		n.items.insertAt(i, item)
		return old, false
	}
	if n.maybeSplitChild(i, maxItems) {
		inTree := n.items[i]
		switch {
		case item.Key.Less(inTree.Key):
			// no change, we want first split node
		case inTree.Key.Less(item.Key):
			i++ // we want second split node
		default:
			out := n.items[i]
			n.items[i] = item
			return out.Value, true
		}
	}
	return n.mutableChild(i).insert(item, maxItems)
}

// get finds the given key in the subtree and returns the corresponding Item, along with a boolean reporting
// whether it was found.
func (n *node) get(k Key) (Item, bool) {
	i, found := n.items.find(k)
	if found {
		return n.items[i], true
	}
	if len(n.children) > 0 {
		return n.children[i].get(k)
	}
	return Item{}, false
}

// cursorsFor returns a stack of cursors for the key. If the second return value is
// true, the stack points at the first element to be returned from the iterator. If it is false,
// the key is not in the tree, and the stack is positioned conceptually just before
// where the key would be.
func (n *node) cursorsFor(k Key, cstack []cursor) ([]cursor, bool) {
	i, found := n.items.find(k)
	cstack = append(cstack, cursor{n, i})
	if found {
		return cstack, true
	}
	if len(n.children) > 0 {
		return n.children[i].cursorsFor(k, cstack)
	}
	return cstack, i < len(n.items)
}

// toRemove details what item to remove in a node.remove call.
type toRemove int

const (
	removeItem toRemove = iota // removes the given item
	removeMin                  // removes smallest item in the subtree
	removeMax                  // removes largest item in the subtree
)

// remove removes an item from the subtree rooted at this node.
func (n *node) remove(key Key, minItems int, typ toRemove) Item {
	var i int
	var found bool
	switch typ {
	case removeMax:
		if len(n.children) == 0 {
			return n.items.pop()
		}
		i = len(n.items)
	case removeMin:
		if len(n.children) == 0 {
			return n.items.removeAt(0)
		}
		i = 0
	case removeItem:
		i, found = n.items.find(key)
		if len(n.children) == 0 {
			if found {
				return n.items.removeAt(i)
			}
			return Item{}
		}
	default:
		panic("invalid type")
	}
	// If we get to here, we have children.
	if len(n.children[i].items) <= minItems {
		return n.growChildAndRemove(i, key, minItems, typ)
	}
	child := n.mutableChild(i)
	// Either we had enough items to begin with, or we've done some
	// merging/stealing, because we've got enough now and we're ready to return
	// stuff.
	if found {
		// The item exists at index 'i', and the child we've selected can give us a
		// predecessor, since if we've gotten here it's got > minItems items in it.
		out := n.items[i]
		// We use our special-case 'remove' call with typ=maxItem to pull the
		// predecessor of item i (the rightmost leaf of our immediate left child)
		// and set it into where we pulled the item from.
		n.items[i] = child.remove(nil, minItems, removeMax)
		return out
	}
	// Final recursive call.  Once we're here, we know that the item isn't in this
	// node and that the child is big enough to remove from.
	return child.remove(key, minItems, typ)
}

// growChildAndRemove grows child 'i' to make sure it's possible to remove an
// item from it while keeping it at minItems, then calls remove to actually
// remove it.
//
// Most documentation says we have to do two sets of special casing:
//   1) item is in this node
//   2) item is in child
// In both cases, we need to handle the two subcases:
//   A) node has enough values that it can spare one
//   B) node doesn't have enough values
// For the latter, we have to check:
//   a) left sibling has node to spare
//   b) right sibling has node to spare
//   c) we must merge
// To simplify our code here, we handle cases #1 and #2 the same:
// If a node doesn't have enough items, we make sure it does (using a,b,c).
// We then simply redo our remove call, and the second time (regardless of
// whether we're in case 1 or 2), we'll have enough items and can guarantee
// that we hit case A.
func (n *node) growChildAndRemove(i int, key Key, minItems int, typ toRemove) Item {
	if i > 0 && len(n.children[i-1].items) > minItems {
		// Steal from left child
		child := n.mutableChild(i)
		stealFrom := n.mutableChild(i - 1)
		stolenItem := stealFrom.items.pop()
		child.items.insertAt(0, n.items[i-1])
		n.items[i-1] = stolenItem
		if len(stealFrom.children) > 0 {
			child.children.insertAt(0, stealFrom.children.pop())
		}
	} else if i < len(n.items) && len(n.children[i+1].items) > minItems {
		// steal from right child
		child := n.mutableChild(i)
		stealFrom := n.mutableChild(i + 1)
		stolenItem := stealFrom.items.removeAt(0)
		child.items = append(child.items, n.items[i])
		n.items[i] = stolenItem
		if len(stealFrom.children) > 0 {
			child.children = append(child.children, stealFrom.children.removeAt(0))
		}
	} else {
		if i >= len(n.items) {
			i--
		}
		child := n.mutableChild(i)
		// merge with right child
		mergeItem := n.items.removeAt(i)
		mergeChild := n.children.removeAt(i + 1)
		child.items = append(child.items, mergeItem)
		child.items = append(child.items, mergeChild.items...)
		child.children = append(child.children, mergeChild.children...)
		n.cow.freeNode(mergeChild)
	}
	return n.remove(key, minItems, typ)
}

type direction int

const (
	descend = direction(-1)
	ascend  = direction(+1)
)

// ItemIterator allows callers of Ascend* to iterate in-order over portions of
// the tree.  When this function returns false, iteration will stop and the
// associated Ascend* function will immediately return.
type ItemIterator func(i Item) bool

// iterate provides a simple method for iterating over elements in the tree.
//
// When ascending, the 'start' should be less than 'stop' and when descending,
// the 'start' should be greater than 'stop'. Setting 'includeStart' to true
// will force the iterator to include the first item when it equals 'start',
// thus creating a "greaterOrEqual" or "lessThanEqual" rather than just a
// "greaterThan" or "lessThan" queries.
func (n *node) iterate(dir direction, start, stop Key, includeStart bool, hit bool, iter ItemIterator) (bool, bool) {
	var ok bool
	switch dir {
	case ascend:
		for i := 0; i < len(n.items); i++ {
			if start != nil && n.items[i].Key.Less(start) {
				continue
			}
			if len(n.children) > 0 {
				if hit, ok = n.children[i].iterate(dir, start, stop, includeStart, hit, iter); !ok {
					return hit, false
				}
			}
			if !includeStart && !hit && start != nil && !start.Less(n.items[i].Key) {
				hit = true
				continue
			}
			hit = true
			if stop != nil && !n.items[i].Key.Less(stop) {
				return hit, false
			}
			if !iter(n.items[i]) {
				return hit, false
			}
		}
		if len(n.children) > 0 {
			if hit, ok = n.children[len(n.children)-1].iterate(dir, start, stop, includeStart, hit, iter); !ok {
				return hit, false
			}
		}
	case descend:
		for i := len(n.items) - 1; i >= 0; i-- {
			if start != nil && !n.items[i].Key.Less(start) {
				if !includeStart || hit || start.Less(n.items[i].Key) {
					continue
				}
			}
			if len(n.children) > 0 {
				if hit, ok = n.children[i+1].iterate(dir, start, stop, includeStart, hit, iter); !ok {
					return hit, false
				}
			}
			if stop != nil && !stop.Less(n.items[i].Key) {
				return hit, false //	continue
			}
			hit = true
			if !iter(n.items[i]) {
				return hit, false
			}
		}
		if len(n.children) > 0 {
			if hit, ok = n.children[0].iterate(dir, start, stop, includeStart, hit, iter); !ok {
				return hit, false
			}
		}
	}
	return hit, true
}

// Used for testing/debugging purposes.
func (n *node) print(w io.Writer, level int) {
	fmt.Fprintf(w, "%sNODE:%v\n", strings.Repeat("  ", level), n.items)
	for _, c := range n.children {
		c.print(w, level+1)
	}
}

// BTree is an implementation of a B-Tree.
//
// BTree stores Item instances in an ordered structure, allowing easy insertion,
// removal, and iteration.
//
// Write operations are not safe for concurrent mutation by multiple
// goroutines, but Read operations are.
type BTree struct {
	degree int
	length int
	root   *node
	cow    *copyOnWriteContext
}

// copyOnWriteContext pointers determine node ownership... a tree with a write
// context equivalent to a node's write context is allowed to modify that node.
// A tree whose write context does not match a node's is not allowed to modify
// it, and must create a new, writable copy (IE: it's a Clone).
//
// When doing any write operation, we maintain the invariant that the current
// node's context is equal to the context of the tree that requested the write.
// We do this by, before we descend into any node, creating a copy with the
// correct context if the contexts don't match.
//
// Since the node we're currently visiting on any write has the requesting
// tree's context, that node is modifiable in place.  Children of that node may
// not share context, but before we descend into them, we'll make a mutable
// copy.
type copyOnWriteContext struct{ byte } // non-empty, because empty structs may have same addr

// Clone clones the btree, lazily.  Clone should not be called concurrently,
// but the original tree (t) and the new tree (t2) can be used concurrently
// once the Clone call completes.
//
// The internal tree structure of b is marked read-only and shared between t and
// t2.  Writes to both t and t2 use copy-on-write logic, creating new nodes
// whenever one of b's original nodes would have been modified.  Read operations
// should have no performance degredation.  Write operations for both t and t2
// will initially experience minor slow-downs caused by additional allocs and
// copies due to the aforementioned copy-on-write logic, but should converge to
// the original performance characteristics of the original tree.
func (t *BTree) Clone() (t2 *BTree) {
	// Create two entirely new copy-on-write contexts.
	// This operation effectively creates three trees:
	//   the original, shared nodes (old b.cow)
	//   the new b.cow nodes
	//   the new out.cow nodes
	cow1, cow2 := *t.cow, *t.cow
	out := *t
	t.cow = &cow1
	out.cow = &cow2
	return &out
}

// maxItems returns the max number of items to allow per node.
func (t *BTree) maxItems() int {
	return t.degree*2 - 1
}

// minItems returns the min number of items to allow per node (ignored for the
// root node).
func (t *BTree) minItems() int {
	return t.degree - 1
}

var nodePool = sync.Pool{New: func() interface{} { return new(node) }}

func (c *copyOnWriteContext) newNode() *node {
	n := nodePool.Get().(*node)
	n.cow = c
	return n
}

func (c *copyOnWriteContext) freeNode(n *node) {
	if n.cow == c {
		// clear to allow GC
		n.items.truncate(0)
		n.children.truncate(0)
		n.cow = nil
		nodePool.Put(n)
	}
}

// Set sets the given key to the given value in the tree. The key must not be nil.
// If the key does not exist, it is added and the second return value is false. If
// the key exists in the tree, its value is replace and the old value is returned
// and the second return value is true.

func (t *BTree) Set(key Key, value Value) (old Value, present bool) {
	if key == nil {
		panic("btree: nil key")
	}
	if t.root == nil {
		t.root = t.cow.newNode()
		t.root.items = append(t.root.items, Item{key, value})
		t.length++
		return old, false
	}
	t.root = t.root.mutableFor(t.cow)
	if len(t.root.items) >= t.maxItems() {
		item2, second := t.root.split(t.maxItems() / 2)
		oldroot := t.root
		t.root = t.cow.newNode()
		t.root.items = append(t.root.items, item2)
		t.root.children = append(t.root.children, oldroot, second)
	}

	old, present = t.root.insert(Item{key, value}, t.maxItems())
	if !present {
		t.length++
	}
	return old, present
}

// Delete removes the item with key, returning its value. If no such item exists, returns
// nil.
func (t *BTree) Delete(key Key) Value {
	return t.deleteItem(key, removeItem).Value
}

// DeleteMin removes the smallest item in the tree and returns the Key and Value.
// If no such item exists, returns zero values.
func (t *BTree) DeleteMin() (Key, Value) {
	item := t.deleteItem(nil, removeMin)
	return item.Key, item.Value
}

// DeleteMax removes the largest item in the tree and returns it.
// If no such item exists, returns nil.
func (t *BTree) DeleteMax() (Key, Value) {
	item := t.deleteItem(nil, removeMax)
	return item.Key, item.Value
}

func (t *BTree) deleteItem(key Key, typ toRemove) Item {
	if t.root == nil || len(t.root.items) == 0 {
		return Item{}
	}
	t.root = t.root.mutableFor(t.cow)
	out := t.root.remove(key, t.minItems(), typ)
	if len(t.root.items) == 0 && len(t.root.children) > 0 {
		oldroot := t.root
		t.root = t.root.children[0]
		t.cow.freeNode(oldroot)
	}
	if out != (Item{}) {
		t.length--
	}
	return out
}

// AscendRange calls the iterator for every value in the tree within the range
// [greaterOrEqual, lessThan), until iterator returns false.
func (t *BTree) AscendRange(greaterOrEqual, lessThan Key, iterator ItemIterator) {
	if t.root == nil {
		return
	}
	t.root.iterate(ascend, greaterOrEqual, lessThan, true, false, iterator)
}

// AscendLessThan calls the iterator for every value in the tree within the range
// [first, pivot), until iterator returns false.
func (t *BTree) AscendLessThan(pivot Key, iterator ItemIterator) {
	if t.root == nil {
		return
	}
	t.root.iterate(ascend, nil, pivot, false, false, iterator)
}

// AscendGreaterOrEqual calls the iterator for every value in the tree within
// the range [pivot, last], until iterator returns false.
func (t *BTree) AscendGreaterOrEqual(pivot Key, iterator ItemIterator) {
	if t.root == nil {
		return
	}
	t.root.iterate(ascend, pivot, nil, true, false, iterator)
}

// Ascend calls the iterator for every value in the tree within the range
// [first, last], until iterator returns false.
func (t *BTree) Ascend(iterator ItemIterator) {
	if t.root == nil {
		return
	}
	t.root.iterate(ascend, nil, nil, false, false, iterator)
}

// DescendRange calls the iterator for every value in the tree within the range
// [lessOrEqual, greaterThan), until iterator returns false.
func (t *BTree) DescendRange(lessOrEqual, greaterThan Key, iterator ItemIterator) {
	if t.root == nil {
		return
	}
	t.root.iterate(descend, lessOrEqual, greaterThan, true, false, iterator)
}

// DescendLessOrEqual calls the iterator for every value in the tree within the range
// [pivot, first], until iterator returns false.
func (t *BTree) DescendLessOrEqual(pivot Key, iterator ItemIterator) {
	if t.root == nil {
		return
	}
	t.root.iterate(descend, pivot, nil, true, false, iterator)
}

// DescendGreaterThan calls the iterator for every value in the tree within
// the range (pivot, last], until iterator returns false.
func (t *BTree) DescendGreaterThan(pivot Key, iterator ItemIterator) {
	if t.root == nil {
		return
	}
	t.root.iterate(descend, nil, pivot, false, false, iterator)
}

// Descend calls the iterator for every value in the tree within the range
// [last, first], until iterator returns false.
func (t *BTree) Descend(iterator ItemIterator) {
	if t.root == nil {
		return
	}
	t.root.iterate(descend, nil, nil, false, false, iterator)
}

// Get returns the value corresponding to key in the tree, or the zero value if there is none.
func (t *BTree) Get(k Key) Value {
	var z Value
	if t.root == nil {
		return z
	}
	item, ok := t.root.get(k)
	if !ok {
		return z
	}
	return item.Value
}

// Has returns true if the given key is in the tree.
func (t *BTree) Has(k Key) bool {
	if t.root == nil {
		return false
	}
	_, ok := t.root.get(k)
	return ok
}

// Min returns the smallest key in the tree and its value. If the tree is empty, both
// return values are zero values.
func (t *BTree) Min() (Key, Value) {
	var k Key
	var v Value
	if t.root == nil {
		return k, v
	}
	n := t.root
	for len(n.children) > 0 {
		n = n.children[0]
	}
	if len(n.items) == 0 {
		return k, v
	}
	return n.items[0].Key, n.items[0].Value
}

// Max returns the largest key in the tree and its value. If the tree is empty, both
// return values are zero values.
func (t *BTree) Max() (Key, Value) {
	var k Key
	var v Value
	if t.root == nil {
		return k, v
	}
	n := t.root
	for len(n.children) > 0 {
		n = n.children[len(n.children)-1]
	}
	if len(n.items) == 0 {
		return k, v
	}
	it := n.items[len(n.items)-1]
	return it.Key, it.Value
}

// Len returns the number of items currently in the tree.
func (t *BTree) Len() int {
	return t.length
}

func (t *BTree) Before(k Key) *Iterator {
	if t.root == nil {
		return &Iterator{}
	}
	var cs []cursor
	cs, stay := t.root.cursorsFor(k, cs)
	// If we found the key, the cursor stack is pointing to it. Since that is
	// the first element we want, don't advance the iterator on the initial call to Next.
	// If we haven't found the key, then the cursor stack is pointing just before it,
	// so the first call to Next will advance the iterator to the right key.
	return &Iterator{
		cursors: cs,
		stay:    stay,
	}
}

func (t *BTree) BeforeMin() *Iterator {
	if t.root == nil {
		return &Iterator{}
	}
	return &Iterator{
		cursors: []cursor{{t.root, -1}},
		Index:   -1,
	}
}

// func (t *BTree) After(key Key) *Iterator {
// 	// Find item at key, or just after.
// 	item, nodes := t.atOrAfter(key)
// 	if item == nil {
// 		return nil
// 	}
// 	return &Cursor{
// 		Key:   item.key,
// 		Value: item.value,
// 		nodes: nodes,
// 	}
// }

// An Iterator supports traversing the items in the tree.
type Iterator struct {
	Key   Key
	Value Value

	// Index is the position of the item in the tree viewed as a sequence.
	// The minimum item has index zero.
	Index int

	cursors []cursor // stack of nodes with indices; last element is the top
	stay    bool     // don't do anything on the first call to Next.
}

// When inc returns true, the top cursor on the stack refers to the new current item.
func (it *Iterator) inc() bool {
	// Useful invariants for understanding this function:
	// - Leaf nodes have zero children, and zero or more items.
	// - Nonleaf nodes have one more child than item, and children[i] < items[i] < children[i+1].
	// - The current item in the iterator is top.node.items[top.index].
	if len(it.cursors) == 0 {
		return false
	}
	if it.stay {
		it.stay = false
		return true
	}
	// If we are at a non-leaf node, we just saw items[i], so
	// now we want to continue with children[i+1], which must exist
	// by the node invariant. We want the minimum item in that child's subtree.
	last := len(it.cursors) - 1
	it.cursors[last].index++ // inc the original, not a copy
	top := it.cursors[last]
	for len(top.node.children) > 0 {
		top = cursor{top.node.children[top.index], 0}
		it.cursors = append(it.cursors, top)
	}
	// Here, we are at a leaf node, with only items. top.index points to
	// the new current item, if it's within the items slice.
	for top.index >= len(top.node.items) {
		// We've gone through everything in this node. Pop it off the stack.
		it.cursors = it.cursors[:last]
		last--
		// If the stack is now empty,we're past the last item in the tree.
		if len(it.cursors) == 0 {
			return false
		}
		top = it.cursors[last]
		// The new top's index points to a child, which we've just finished
		// exploring. The next item is the one at the same index in the items slice.
	}
	// Here, the top cursor on the stack refers to the next item.
	return true
}

// A cursor is effectively a pointer into a node. A stack of cursors identifies an item in the tree,
// and makes it possible to move to the next or previous item efficiently.
//
// If the cursor is on the top of the stack, its index points into the node's items slice, selecting
// the current item. Otherwise, the index points into the children slice and identifies the child
// that is next in the stack.
type cursor struct {
	node  *node
	index int
}

// Next advances the Iterator to the next item in the tree. If Next returns true,
// the Iterator's Key, Value and Index fields refer to the next item. If Next returns
// false, there are no more items and the values of Key, Value and Index are undefined.
func (it *Iterator) Next() bool {
	if !it.inc() {
		return false
	}
	top := it.cursors[len(it.cursors)-1]
	item := top.node.items[top.index]
	it.Key = item.Key
	it.Value = item.Value
	it.Index++
	return true
}

// // Prev returns the item immediately preceding i, or nil if there is none.
// func (c *Iterator) Prev() *Iterator {
// }
