// Copyright 2014 Google Inc.
// Modified 2018 by Jonathan Amsterdam (jbamsterdam@gmail.com)
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

package btree_test

import (
	"fmt"

	"github.com/jba/btree"
)

func ExampleBTree() {
	tr := btree.New(32)
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

func ExampleIterator_Next() {
	tr := btree.New(16)
	for i := 0; i < 5; i++ {
		tr.Set(Int(i), i)
	}
	it := tr.BeforeIndex(0)
	for it.Next() {
		fmt.Println(it.Key, it.Value, it.Index)
	}
	// Output:
	// 0 0 0
	// 1 1 1
	// 2 2 2
	// 3 3 3
	// 4 4 4
}

// Int implements the Key interface for integers.
type Int int

// Less returns true if int(a) < int(b).
func (a Int) Less(b btree.Key) bool {
	return a < b.(Int)
}
