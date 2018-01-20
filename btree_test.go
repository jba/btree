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

package btree

import (
	"flag"
	"fmt"
	"math/rand"
	"reflect"
	"sync"
	"testing"
	"time"
)

func init() {
	seed := time.Now().Unix()
	fmt.Println(seed)
	rand.Seed(seed)
}

// perm returns a random permutation of n Int items in the range [0, n).
func perm(n int) (out []item) {
	for _, v := range rand.Perm(n) {
		i := Int(v)
		out = append(out, item{i, i})
	}
	return
}

// rang returns an ordered list of Int items in the range [0, n).
func rang(n int) (out []item) {
	for i := 0; i < n; i++ {
		out = append(out, item{Int(i), Int(i)})
	}
	return
}

// all extracts all items from an iterator.
func all(it *Iterator) (out []item) {
	for it.Next() {
		out = append(out, item{it.Key, it.Value})
	}
	return
}

// rangerev returns a reversed ordered list of Int items in the range [0, n).
func rangrev(n int) (out []item) {
	for i := n - 1; i >= 0; i-- {
		out = append(out, item{Int(i), Int(i)})
	}
	return
}

func reverse(s []item) {
	for i := 0; i < len(s)/2; i++ {
		s[i], s[len(s)-i-1] = s[len(s)-i-1], s[i]
	}
}

var btreeDegree = flag.Int("degree", 32, "B-Tree degree")

func TestBTree(t *testing.T) {
	tr := New(*btreeDegree)
	const treeSize = 10000
	for i := 0; i < 10; i++ {
		if min, _ := tr.Min(); min != nil {
			t.Fatalf("empty min, got %+v", min)
		}
		if max, _ := tr.Max(); max != nil {
			t.Fatalf("empty max, got %+v", max)
		}
		for _, m := range perm(treeSize) {
			if _, ok := tr.Set(m.key, m.value); ok {
				t.Fatal("set found item", m)
			}
		}
		for _, m := range perm(treeSize) {
			if _, ok := tr.Set(m.key, m.value); !ok {
				t.Fatal("set didn't find item", m)
			}
		}
		mink, minv := tr.Min()
		if want := Int(0); mink != want || minv != want {
			t.Fatalf("min: want %+v, got %+v, %+v", want, mink, minv)
		}
		maxk, maxv := tr.Max()
		if want := Int(treeSize - 1); maxk != want || maxv != want {
			t.Fatalf("max: want %+v, got %+v, %+v", want, maxk, maxv)
		}
		got := all(tr.BeforeMin())
		want := rang(treeSize)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("mismatch:\n got: %v\nwant: %v", got, want)
		}

		for _, m := range perm(treeSize) {
			if _, removed := tr.Delete(m.key); !removed {
				t.Fatalf("didn't find %v", m)
			}
		}
		if got = all(tr.BeforeMin()); len(got) > 0 {
			t.Fatalf("some left!: %v", got)
		}
	}
}

func TestAt(t *testing.T) {
	tr := New(*btreeDegree)
	for _, m := range perm(100) {
		tr.Set(m.key, m.value)
	}
	for i := 0; i < tr.Len(); i++ {
		gotk, gotv := tr.At(i)
		if want := Int(i); gotk != want || gotv != want {
			t.Fatalf("At(%d) = (%v, %v), want (%v, %v)", i, gotk, gotv, want, want)
		}
	}
}

func TestGetWithIndex(t *testing.T) {
	tr := New(*btreeDegree)
	for _, m := range perm(100) {
		tr.Set(m.key, m.value)
	}
	for i := 0; i < tr.Len(); i++ {
		gotv, goti := tr.GetWithIndex(Int(i))
		wantv, wanti := Int(i), i
		if gotv != wantv || goti != wanti {
			t.Errorf("GetWithIndex(%d) = (%v, %v), want (%v, %v)",
				i, gotv, goti, wantv, wanti)
		}
	}
	_, got := tr.GetWithIndex(Int(100))
	if want := -1; got != want {
		t.Errorf("got %d, want %d", got, want)
	}
}

func ExampleBTree() {
	tr := New(*btreeDegree)
	for i := 0; i < 10; i++ {
		tr.Set(Int(i), i)
	}
	fmt.Println("len:       ", tr.Len())
	fmt.Println("get3:      ", tr.Get(Int(3)))
	fmt.Println("get100:    ", tr.Get(Int(100)))
	k, v := tr.At(7)
	fmt.Println("at7:       ", k, v)
	d, ok := tr.Delete(Int(4))
	fmt.Println("del4:      ", d, ok)
	d, ok = tr.Delete(Int(100))
	fmt.Println("del100:    ", d, ok)
	old, ok := tr.Set(Int(5), 11)
	fmt.Println("set5:      ", old, ok)
	old, ok = tr.Set(Int(100), 100)
	fmt.Println("set100:    ", old, ok)
	k, v = tr.Min()
	fmt.Println("min:       ", k, v)
	k, v = tr.DeleteMin()
	fmt.Println("delmin:    ", k, v)
	k, v = tr.Max()
	fmt.Println("max:       ", k, v)
	k, v = tr.DeleteMax()
	fmt.Println("delmax:    ", k, v)
	fmt.Println("len:       ", tr.Len())
	// Output:
	// len:        10
	// get3:       3
	// get100:     <nil>
	// at7:        7 7
	// del4:       4 true
	// del100:     <nil> false
	// set5:       5 true
	// set100:     <nil> false
	// min:        0 0
	// delmin:     0 0
	// max:        100 100
	// delmax:     100 100
	// len:        8
}

func TestDeleteMin(t *testing.T) {
	tr := New(3)
	for _, m := range perm(100) {
		tr.Set(m.key, m.value)
	}
	var got []item
	for tr.Len() > 0 {
		k, v := tr.DeleteMin()
		got = append(got, item{k, v})
	}
	if want := rang(100); !reflect.DeepEqual(got, want) {
		t.Fatalf("got: %v\nwant: %v", got, want)
	}
}

func TestDeleteMax(t *testing.T) {
	tr := New(3)
	for _, m := range perm(100) {
		tr.Set(m.key, m.value)
	}
	var got []item
	for tr.Len() > 0 {
		k, v := tr.DeleteMax()
		got = append(got, item{k, v})
	}
	reverse(got)
	if want := rang(100); !reflect.DeepEqual(got, want) {
		t.Fatalf("got: %v\nwant: %v", got, want)
	}
}

func TestIterator(t *testing.T) {
	const size = 100

	tr := New(2)
	// Empty tree.
	for i, it := range []*Iterator{
		tr.BeforeMin(),
		tr.Before(Int(3)),
		tr.After(Int(3)),
	} {
		if got, want := it.Next(), false; got != want {
			t.Errorf("empty, #%d: got %t, want %t", i, got, want)
		}
	}

	// Root with zero children.
	tr.Set(Int(1), nil)
	tr.Delete(Int(1))
	if !(tr.root != nil && len(tr.root.children) == 0 && len(tr.root.items) == 0) {
		t.Fatal("wrong shape tree")
	}
	for i, it := range []*Iterator{
		tr.BeforeMin(),
		tr.Before(Int(3)),
		tr.After(Int(3)),
	} {
		if got, want := it.Next(), false; got != want {
			t.Errorf("zero root, #%d: got %t, want %t", i, got, want)
		}
	}

	// Tree with size elements.
	p := perm(size)
	for _, v := range p {
		tr.Set(v.key, v.value)
	}

	it := tr.BeforeMin()
	got := all(it)
	want := rang(size)
	// TODO: text Index
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v\nwant %+v\n", got, want)
	}

	for _, w := range want {
		it := tr.Before(w.key)
		got = all(it)
		// TODO: test it.Index
		wn := want[w.key.(Int):]
		if !reflect.DeepEqual(got, wn) {
			t.Fatalf("got %+v\nwant %+v\n", got, wn)
		}

		it = tr.After(w.key)
		got = all(it)
		wn = append([]item(nil), want[:w.key.(Int)+1]...)
		reverse(wn)
		if !reflect.DeepEqual(got, wn) {
			t.Fatalf("got %+v\nwant %+v\n", got, wn)
		}
	}

	// Non-existent keys.
	tr = New(2)
	for _, v := range p {
		tr.Set(Int(v.key.(Int)*2), v.value)
	}
	// tr has only even keys: 0, 2, 4, ... Iterate from odd keys.
	for i := -1; i <= size+1; i += 2 {
		it := tr.Before(Int(i))
		got := all(it)
		var want []item
		for j := (i + 1) / 2; j < size; j++ {
			want = append(want, item{Int(j) * 2, Int(j)})
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("%d: got %+v\nwant %+v\n", i, got, want)
		}

		it = tr.After(Int(i))
		got = all(it)
		want = nil
		for j := (i - 1) / 2; j >= 0; j-- {
			want = append(want, item{Int(j) * 2, Int(j)})
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("%d: got %+v\nwant %+v\n", i, got, want)
		}
	}
}

const cloneTestSize = 10000

func cloneTest(t *testing.T, b *BTree, start int, p []item, wg *sync.WaitGroup, treec chan<- *BTree) {
	treec <- b
	for i := start; i < cloneTestSize; i++ {
		b.Set(p[i].key, p[i].value)
		if i%(cloneTestSize/5) == 0 {
			wg.Add(1)
			go cloneTest(t, b.Clone(), i+1, p, wg, treec)
		}
	}
	wg.Done()
}

func TestCloneConcurrentOperations(t *testing.T) {
	b := New(*btreeDegree)
	treec := make(chan *BTree)
	p := perm(cloneTestSize)
	var wg sync.WaitGroup
	wg.Add(1)
	go cloneTest(t, b, 0, p, &wg, treec)
	var trees []*BTree
	donec := make(chan struct{})
	go func() {
		for t := range treec {
			trees = append(trees, t)
		}
		close(donec)
	}()
	wg.Wait()
	close(treec)
	<-donec
	want := rang(cloneTestSize)
	for i, tree := range trees {
		if !reflect.DeepEqual(want, all(tree.BeforeMin())) {
			t.Errorf("tree %v mismatch", i)
		}
	}
	toRemove := rang(cloneTestSize)[cloneTestSize/2:]
	for i := 0; i < len(trees)/2; i++ {
		tree := trees[i]
		wg.Add(1)
		go func() {
			for _, m := range toRemove {
				tree.Delete(m.key)
			}
			wg.Done()
		}()
	}
	wg.Wait()
	for i, tree := range trees {
		var wantpart []item
		if i < len(trees)/2 {
			wantpart = want[:cloneTestSize/2]
		} else {
			wantpart = want
		}
		if got := all(tree.BeforeMin()); !reflect.DeepEqual(wantpart, got) {
			t.Errorf("tree %v mismatch, want %v got %v", i, len(want), len(got))
		}
	}
}

// Int implements the Key interface for integers.
type Int int

// Less returns true if int(a) < int(b).
func (a Int) Less(b Key) bool {
	return a < b.(Int)
}
