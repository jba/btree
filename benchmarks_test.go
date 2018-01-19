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

// +build go1.7

package btree

import (
	"fmt"
	"testing"
)

// TODO: write iterator benchmarks and compare them with the original Ascend/Descend benchmarks.

const benchmarkTreeSize = 10000

var degrees = []int{2, 8, 32, 64}

func BenchmarkInsert(b *testing.B) {
	insertP := perm(benchmarkTreeSize)
	for _, d := range degrees {
		b.Run(fmt.Sprintf("degree=%d", d), func(b *testing.B) {
			i := 0
			for i < b.N {
				tr := New(d)
				for _, m := range insertP {
					tr.Set(m.key, m.value)
					i++
					if i >= b.N {
						return
					}
				}
			}
		})
	}
}

func BenchmarkDeleteInsert(b *testing.B) {
	insertP := perm(benchmarkTreeSize)
	for _, d := range degrees {
		b.Run(fmt.Sprintf("degree=%d", d), func(b *testing.B) {
			tr := New(d)
			for _, m := range insertP {
				tr.Set(m.key, m.value)
			}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				m := insertP[i%benchmarkTreeSize]
				tr.Delete(m.key)
				tr.Set(m.key, m.value)
			}
		})
	}
}

func BenchmarkDeleteInsertCloneOnce(b *testing.B) {
	insertP := perm(benchmarkTreeSize)
	for _, d := range degrees {
		b.Run(fmt.Sprintf("degree=%d", d), func(b *testing.B) {
			tr := New(d)
			for _, m := range insertP {
				tr.Set(m.key, m.value)
			}
			tr = tr.Clone()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				m := insertP[i%benchmarkTreeSize]
				tr.Delete(m.key)
				tr.Set(m.key, m.value)
			}
		})
	}
}

func BenchmarkDeleteInsertCloneEachTime(b *testing.B) {
	insertP := perm(benchmarkTreeSize)
	for _, d := range degrees {
		b.Run(fmt.Sprintf("degree=%d", d), func(b *testing.B) {
			tr := New(d)
			for _, m := range insertP {
				tr.Set(m.key, m.value)
			}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				tr = tr.Clone()
				m := insertP[i%benchmarkTreeSize]
				tr.Delete(m.key)
				tr.Set(m.key, m.value)
			}
		})
	}
}

func BenchmarkDelete(b *testing.B) {
	insertP := perm(benchmarkTreeSize)
	removeP := perm(benchmarkTreeSize)
	for _, d := range degrees {
		b.Run(fmt.Sprintf("degree=%d", d), func(b *testing.B) {
			i := 0
			for i < b.N {
				b.StopTimer()
				tr := New(d)
				for _, v := range insertP {
					tr.Set(v.key, v.value)
				}
				b.StartTimer()
				for _, m := range removeP {
					tr.Delete(m.key)
					i++
					if i >= b.N {
						return
					}
				}
				if tr.Len() > 0 {
					panic(tr.Len())
				}
			}
		})
	}
}

func BenchmarkGet(b *testing.B) {
	insertP := perm(benchmarkTreeSize)
	removeP := perm(benchmarkTreeSize)
	for _, d := range degrees {
		b.Run(fmt.Sprintf("degree=%d", d), func(b *testing.B) {
			i := 0
			for i < b.N {
				b.StopTimer()
				tr := New(d)
				for _, v := range insertP {
					tr.Set(v.key, v.value)
				}
				b.StartTimer()
				for _, m := range removeP {
					tr.Get(m.key)
					i++
					if i >= b.N {
						return
					}
				}
			}
		})
	}
}

func BenchmarkGetCloneEachTime(b *testing.B) {
	insertP := perm(benchmarkTreeSize)
	removeP := perm(benchmarkTreeSize)
	for _, d := range degrees {
		b.Run(fmt.Sprintf("degree=%d", d), func(b *testing.B) {
			i := 0
			for i < b.N {
				b.StopTimer()
				tr := New(d)
				for _, m := range insertP {
					tr.Set(m.key, m.value)
				}
				b.StartTimer()
				for _, m := range removeP {
					tr = tr.Clone()
					tr.Get(m.key)
					i++
					if i >= b.N {
						return
					}
				}
			}
		})
	}
}

type byInts []item

func (a byInts) Len() int {
	return len(a)
}

func (a byInts) Less(i, j int) bool {
	return a[i].key.(Int) < a[j].key.(Int)
}

func (a byInts) Swap(i, j int) {
	a[i], a[j] = a[j], a[i]
}
