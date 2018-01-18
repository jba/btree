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
	"sort"
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
func perm(n int) (out []Item) {
	for _, v := range rand.Perm(n) {
		i := Int(v)
		out = append(out, Item{i, i})
	}
	return
}

// rang returns an ordered list of Int items in the range [0, n).
func rang(n int) (out []Item) {
	for i := 0; i < n; i++ {
		out = append(out, Item{Int(i), Int(i)})
	}
	return
}

// all extracts all items from a tree in order as a slice.
func all(t *BTree) (out []Item) {
	t.Ascend(func(a Item) bool {
		out = append(out, a)
		return true
	})
	return
}

// rangerev returns a reversed ordered list of Int items in the range [0, n).
func rangrev(n int) (out []Item) {
	for i := n - 1; i >= 0; i-- {
		out = append(out, Item{Int(i), Int(i)})
	}
	return
}

// allrev extracts all items from a tree in reverse order as a slice.
func allrev(t *BTree) (out []Item) {
	t.Descend(func(a Item) bool {
		out = append(out, a)
		return true
	})
	return
}

var btreeDegree = flag.Int("degree", 32, "B-Tree degree")

func TestBTree(t *testing.T) {
	tr := New(*btreeDegree)
	const treeSize = 10000
	for i := 0; i < 10; i++ {
		if min := tr.Min(); min != (Item{}) {
			t.Fatalf("empty min, got %+v", min)
		}
		if max := tr.Max(); max != (Item{}) {
			t.Fatalf("empty max, got %+v", max)
		}
		for _, item := range perm(treeSize) {
			if _, ok := tr.Set(item.Key, item.Value); ok {
				t.Fatal("insert found item", item)
			}
		}
		for _, item := range perm(treeSize) {
			if _, ok := tr.Set(item.Key, item.Value); !ok {
				t.Fatal("insert didn't find item", item)
			}
		}
		if min, want := tr.Min(), (Item{Int(0), Int(0)}); min != want {
			t.Fatalf("min: want %+v, got %+v", want, min)
		}
		if max, want := tr.Max(), (Item{Int(treeSize - 1), Int(treeSize - 1)}); max != want {
			t.Fatalf("max: want %+v, got %+v", want, max)
		}
		got := all(tr)
		want := rang(treeSize)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("mismatch:\n got: %v\nwant: %v", got, want)
		}

		gotrev := allrev(tr)
		wantrev := rangrev(treeSize)
		if !reflect.DeepEqual(gotrev, wantrev) {
			t.Fatalf("mismatch:\n got: %v\nwant: %v", got, want)
		}

		for _, item := range perm(treeSize) {
			if x := tr.Delete(item.Key); x == (Item{}) {
				t.Fatalf("didn't find %v", item)
			}
		}
		if got = all(tr); len(got) > 0 {
			t.Fatalf("some left!: %v", got)
		}
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
	fmt.Println("del4:      ", tr.Delete(Int(4)))
	fmt.Println("del100:    ", tr.Delete(Int(100)))
	old, ok := tr.Set(Int(5), 11)
	fmt.Println("set5:      ", old, ok)
	old, ok = tr.Set(Int(100), 100)
	fmt.Println("set100:    ", old, ok)
	fmt.Println("min:       ", tr.Min())
	fmt.Println("delmin:    ", tr.DeleteMin())
	fmt.Println("max:       ", tr.Max())
	fmt.Println("delmax:    ", tr.DeleteMax())
	fmt.Println("len:       ", tr.Len())
	// Output:
	// len:        10
	// get3:       3
	// get100:     <nil>
	// del4:       {4 4}
	// del100:     {<nil> <nil>}
	// set5:       5 true
	// set100:     <nil> false
	// min:        {0 0}
	// delmin:     {0 0}
	// max:        {100 100}
	// delmax:     {100 100}
	// len:        8
}

func TestDeleteMin(t *testing.T) {
	tr := New(3)
	for _, v := range perm(100) {
		tr.Set(v.Key, v.Value)
	}
	var got []Item
	for v := tr.DeleteMin(); v != (Item{}); v = tr.DeleteMin() {
		got = append(got, v)
	}
	if want := rang(100); !reflect.DeepEqual(got, want) {
		t.Fatalf("ascendrange:\n got: %v\nwant: %v", got, want)
	}
}

func TestDeleteMax(t *testing.T) {
	tr := New(3)
	for _, v := range perm(100) {
		tr.Set(v.Key, v.Value)
	}
	var got []Item
	for v := tr.DeleteMax(); v != (Item{}); v = tr.DeleteMax() {
		got = append(got, v)
	}
	// Reverse our list.
	for i := 0; i < len(got)/2; i++ {
		got[i], got[len(got)-i-1] = got[len(got)-i-1], got[i]
	}
	if want := rang(100); !reflect.DeepEqual(got, want) {
		t.Fatalf("ascendrange:\n got: %v\nwant: %v", got, want)
	}
}

func TestAscendRange(t *testing.T) {
	tr := New(2)
	for _, v := range perm(100) {
		tr.Set(v.Key, v.Value)
	}
	var got []Item
	tr.AscendRange(Int(40), Int(60), func(a Item) bool {
		got = append(got, a)
		return true
	})
	if want := rang(100)[40:60]; !reflect.DeepEqual(got, want) {
		t.Fatalf("ascendrange:\n got: %v\nwant: %v", got, want)
	}
	got = got[:0]
	tr.AscendRange(Int(40), Int(60), func(a Item) bool {
		if a.Key.(Int) > 50 {
			return false
		}
		got = append(got, a)
		return true
	})
	if want := rang(100)[40:51]; !reflect.DeepEqual(got, want) {
		t.Fatalf("ascendrange:\n got: %v\nwant: %v", got, want)
	}
}

func TestDescendRange(t *testing.T) {
	tr := New(2)
	for _, v := range perm(100) {
		tr.Set(v.Key, v.Value)
	}
	var got []Item
	tr.DescendRange(Int(60), Int(40), func(a Item) bool {
		got = append(got, a)
		return true
	})
	if want := rangrev(100)[39:59]; !reflect.DeepEqual(got, want) {
		t.Fatalf("descendrange:\n got: %v\nwant: %v", got, want)
	}
	got = got[:0]
	tr.DescendRange(Int(60), Int(40), func(a Item) bool {
		if a.Key.(Int) < 50 {
			return false
		}
		got = append(got, a)
		return true
	})
	if want := rangrev(100)[39:50]; !reflect.DeepEqual(got, want) {
		t.Fatalf("descendrange:\n got: %v\nwant: %v", got, want)
	}
}
func TestAscendLessThan(t *testing.T) {
	tr := New(*btreeDegree)
	for _, v := range perm(100) {
		tr.Set(v.Key, v.Value)
	}
	var got []Item
	tr.AscendLessThan(Int(60), func(a Item) bool {
		got = append(got, a)
		return true
	})
	if want := rang(100)[:60]; !reflect.DeepEqual(got, want) {
		t.Fatalf("ascendrange:\n got: %v\nwant: %v", got, want)
	}
	got = got[:0]
	tr.AscendLessThan(Int(60), func(a Item) bool {
		if a.Key.(Int) > 50 {
			return false
		}
		got = append(got, a)
		return true
	})
	if want := rang(100)[:51]; !reflect.DeepEqual(got, want) {
		t.Fatalf("ascendrange:\n got: %v\nwant: %v", got, want)
	}
}

func TestDescendLessOrEqual(t *testing.T) {
	tr := New(*btreeDegree)
	for _, v := range perm(100) {
		tr.Set(v.Key, v.Value)
	}
	var got []Item
	tr.DescendLessOrEqual(Int(40), func(a Item) bool {
		got = append(got, a)
		return true
	})
	if want := rangrev(100)[59:]; !reflect.DeepEqual(got, want) {
		t.Fatalf("descendlessorequal:\n got: %v\nwant: %v", got, want)
	}
	got = got[:0]
	tr.DescendLessOrEqual(Int(60), func(a Item) bool {
		if a.Key.(Int) < 50 {
			return false
		}
		got = append(got, a)
		return true
	})
	if want := rangrev(100)[39:50]; !reflect.DeepEqual(got, want) {
		t.Fatalf("descendlessorequal:\n got: %v\nwant: %v", got, want)
	}
}
func TestAscendGreaterOrEqual(t *testing.T) {
	tr := New(*btreeDegree)
	for _, v := range perm(100) {
		tr.Set(v.Key, v.Value)
	}
	var got []Item
	tr.AscendGreaterOrEqual(Int(40), func(a Item) bool {
		got = append(got, a)
		return true
	})
	if want := rang(100)[40:]; !reflect.DeepEqual(got, want) {
		t.Fatalf("ascendrange:\n got: %v\nwant: %v", got, want)
	}
	got = got[:0]
	tr.AscendGreaterOrEqual(Int(40), func(a Item) bool {
		if a.Key.(Int) > 50 {
			return false
		}
		got = append(got, a)
		return true
	})
	if want := rang(100)[40:51]; !reflect.DeepEqual(got, want) {
		t.Fatalf("ascendrange:\n got: %v\nwant: %v", got, want)
	}
}

func TestDescendGreaterThan(t *testing.T) {
	tr := New(*btreeDegree)
	for _, v := range perm(100) {
		tr.Set(v.Key, v.Value)
	}
	var got []Item
	tr.DescendGreaterThan(Int(40), func(a Item) bool {
		got = append(got, a)
		return true
	})
	if want := rangrev(100)[:59]; !reflect.DeepEqual(got, want) {
		t.Fatalf("descendgreaterthan:\n got: %v\nwant: %v", got, want)
	}
	got = got[:0]
	tr.DescendGreaterThan(Int(40), func(a Item) bool {
		if a.Key.(Int) < 50 {
			return false
		}
		got = append(got, a)
		return true
	})
	if want := rangrev(100)[:50]; !reflect.DeepEqual(got, want) {
		t.Fatalf("descendgreaterthan:\n got: %v\nwant: %v", got, want)
	}
}

const benchmarkTreeSize = 10000

func BenchmarkInsert(b *testing.B) {
	b.StopTimer()
	insertP := perm(benchmarkTreeSize)
	b.StartTimer()
	i := 0
	for i < b.N {
		tr := New(*btreeDegree)
		for _, item := range insertP {
			tr.Set(item.Key, item.Value)
			i++
			if i >= b.N {
				return
			}
		}
	}
}

func BenchmarkDeleteInsert(b *testing.B) {
	b.StopTimer()
	insertP := perm(benchmarkTreeSize)
	tr := New(*btreeDegree)
	for _, item := range insertP {
		tr.Set(item.Key, item.Value)
	}
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		item := insertP[i%benchmarkTreeSize]
		tr.Delete(item.Key)
		tr.Set(item.Key, item.Value)
	}
}

func BenchmarkDeleteInsertCloneOnce(b *testing.B) {
	b.StopTimer()
	insertP := perm(benchmarkTreeSize)
	tr := New(*btreeDegree)
	for _, item := range insertP {
		tr.Set(item.Key, item.Value)
	}
	tr = tr.Clone()
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		item := insertP[i%benchmarkTreeSize]
		tr.Delete(item.Key)
		tr.Set(item.Key, item.Value)
	}
}

func BenchmarkDeleteInsertCloneEachTime(b *testing.B) {
	b.StopTimer()
	insertP := perm(benchmarkTreeSize)
	tr := New(*btreeDegree)
	for _, item := range insertP {
		tr.Set(item.Key, item.Value)
	}
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		tr = tr.Clone()
		item := insertP[i%benchmarkTreeSize]
		tr.Delete(item.Key)
		tr.Set(item.Key, item.Value)
	}
}

func BenchmarkDelete(b *testing.B) {
	b.StopTimer()
	insertP := perm(benchmarkTreeSize)
	removeP := perm(benchmarkTreeSize)
	b.StartTimer()
	i := 0
	for i < b.N {
		b.StopTimer()
		tr := New(*btreeDegree)
		for _, v := range insertP {
			tr.Set(v.Key, v.Value)
		}
		b.StartTimer()
		for _, item := range removeP {
			tr.Delete(item.Key)
			i++
			if i >= b.N {
				return
			}
		}
		if tr.Len() > 0 {
			panic(tr.Len())
		}
	}
}

func BenchmarkGet(b *testing.B) {
	b.StopTimer()
	insertP := perm(benchmarkTreeSize)
	removeP := perm(benchmarkTreeSize)
	b.StartTimer()
	i := 0
	for i < b.N {
		b.StopTimer()
		tr := New(*btreeDegree)
		for _, v := range insertP {
			tr.Set(v.Key, v.Value)
		}
		b.StartTimer()
		for _, item := range removeP {
			tr.Get(item.Key)
			i++
			if i >= b.N {
				return
			}
		}
	}
}

func BenchmarkGetCloneEachTime(b *testing.B) {
	b.StopTimer()
	insertP := perm(benchmarkTreeSize)
	removeP := perm(benchmarkTreeSize)
	b.StartTimer()
	i := 0
	for i < b.N {
		b.StopTimer()
		tr := New(*btreeDegree)
		for _, v := range insertP {
			tr.Set(v.Key, v.Value)
		}
		b.StartTimer()
		for _, item := range removeP {
			tr = tr.Clone()
			tr.Get(item.Key)
			i++
			if i >= b.N {
				return
			}
		}
	}
}

type byInts []Item

func (a byInts) Len() int {
	return len(a)
}

func (a byInts) Less(i, j int) bool {
	return a[i].Key.(Int) < a[j].Key.(Int)
}

func (a byInts) Swap(i, j int) {
	a[i], a[j] = a[j], a[i]
}

func BenchmarkAscend(b *testing.B) {
	arr := perm(benchmarkTreeSize)
	tr := New(*btreeDegree)
	for _, v := range arr {
		tr.Set(v.Key, v.Value)
	}
	sort.Sort(byInts(arr))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		j := 0
		tr.Ascend(func(item Item) bool {
			if item.Key.(Int) != arr[j].Key.(Int) {
				b.Fatalf("mismatch: expected: %v, got %v", arr[j].Key.(Int), item.Key.(Int))
			}
			j++
			return true
		})
	}
}

func BenchmarkDescend(b *testing.B) {
	arr := perm(benchmarkTreeSize)
	tr := New(*btreeDegree)
	for _, v := range arr {
		tr.Set(v.Key, v.Value)
	}
	sort.Sort(byInts(arr))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		j := len(arr) - 1
		tr.Descend(func(item Item) bool {
			if item.Key.(Int) != arr[j].Key.(Int) {
				b.Fatalf("mismatch: expected: %v, got %v", arr[j].Key.(Int), item.Key.(Int))
			}
			j--
			return true
		})
	}
}
func BenchmarkAscendRange(b *testing.B) {
	arr := perm(benchmarkTreeSize)
	tr := New(*btreeDegree)
	for _, v := range arr {
		tr.Set(v.Key, v.Value)
	}
	sort.Sort(byInts(arr))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		j := 100
		tr.AscendRange(Int(100), arr[len(arr)-100].Key, func(item Item) bool {
			if item.Key.(Int) != arr[j].Key.(Int) {
				b.Fatalf("mismatch: expected: %v, got %v", arr[j].Key.(Int), item.Key.(Int))
			}
			j++
			return true
		})
		if j != len(arr)-100 {
			b.Fatalf("expected: %v, got %v", len(arr)-100, j)
		}
	}
}

// func BenchmarkDescendRange(b *testing.B) {
// 	arr := perm(benchmarkTreeSize)
// 	tr := New(*btreeDegree)
// 	for _, v := range arr {
// 		tr.Set(v.Key, v.Value)
// 	}
// 	sort.Sort(byInts(arr))
// 	b.ResetTimer()
// 	for i := 0; i < b.N; i++ {
// 		j := len(arr) - 100
// 		tr.DescendRange(arr[len(arr)-100], Int(100), func(item Item) bool {
// 			if item.(Int) != arr[j].(Int) {
// 				b.Fatalf("mismatch: expected: %v, got %v", arr[j].(Int), item.(Int))
// 			}
// 			j--
// 			return true
// 		})
// 		if j != 100 {
// 			b.Fatalf("expected: %v, got %v", len(arr)-100, j)
// 		}
// 	}
// }
// func BenchmarkAscendGreaterOrEqual(b *testing.B) {
// 	arr := perm(benchmarkTreeSize)
// 	tr := New(*btreeDegree)
// 	for _, v := range arr {
// 		tr.Set(v)
// 	}
// 	sort.Sort(byInts(arr))
// 	b.ResetTimer()
// 	for i := 0; i < b.N; i++ {
// 		j := 100
// 		k := 0
// 		tr.AscendGreaterOrEqual(Int(100), func(item Item) bool {
// 			if item.(Int) != arr[j].(Int) {
// 				b.Fatalf("mismatch: expected: %v, got %v", arr[j].(Int), item.(Int))
// 			}
// 			j++
// 			k++
// 			return true
// 		})
// 		if j != len(arr) {
// 			b.Fatalf("expected: %v, got %v", len(arr), j)
// 		}
// 		if k != len(arr)-100 {
// 			b.Fatalf("expected: %v, got %v", len(arr)-100, k)
// 		}
// 	}
// }
// func BenchmarkDescendLessOrEqual(b *testing.B) {
// 	arr := perm(benchmarkTreeSize)
// 	tr := New(*btreeDegree)
// 	for _, v := range arr {
// 		tr.Set(v)
// 	}
// 	sort.Sort(byInts(arr))
// 	b.ResetTimer()
// 	for i := 0; i < b.N; i++ {
// 		j := len(arr) - 100
// 		k := len(arr)
// 		tr.DescendLessOrEqual(arr[len(arr)-100], func(item Item) bool {
// 			if item.(Int) != arr[j].(Int) {
// 				b.Fatalf("mismatch: expected: %v, got %v", arr[j].(Int), item.(Int))
// 			}
// 			j--
// 			k--
// 			return true
// 		})
// 		if j != -1 {
// 			b.Fatalf("expected: %v, got %v", -1, j)
// 		}
// 		if k != 99 {
// 			b.Fatalf("expected: %v, got %v", 99, k)
// 		}
// 	}
// }

const cloneTestSize = 10000

func cloneTest(t *testing.T, b *BTree, start int, p []Item, wg *sync.WaitGroup, treec chan<- *BTree) {
	treec <- b
	for i := start; i < cloneTestSize; i++ {
		b.Set(p[i].Key, p[i].Value)
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
		if !reflect.DeepEqual(want, all(tree)) {
			t.Errorf("tree %v mismatch", i)
		}
	}
	toRemove := rang(cloneTestSize)[cloneTestSize/2:]
	for i := 0; i < len(trees)/2; i++ {
		tree := trees[i]
		wg.Add(1)
		go func() {
			for _, item := range toRemove {
				tree.Delete(item.Key)
			}
			wg.Done()
		}()
	}
	wg.Wait()
	for i, tree := range trees {
		var wantpart []Item
		if i < len(trees)/2 {
			wantpart = want[:cloneTestSize/2]
		} else {
			wantpart = want
		}
		if got := all(tree); !reflect.DeepEqual(wantpart, got) {
			t.Errorf("tree %v mismatch, want %v got %v", i, len(want), len(got))
		}
	}
}
