# BTree implementation for Go

![Travis CI Build Status](https://api.travis-ci.org/google/btree.svg?branch=master)

This package provides an in-memory B-Tree implementation for Go, useful as
an ordered, mutable data structure.

This is a fork of github.com/google/btree with an improved API:

- Separate keys and values. With the single type `Item` it could be awkward to
  construct an item solely for use as a key.
  
- Instead of eight separate methods for iterating, two methods, `After` and
  `Before`, return a `Cursor` that can be used to iterate in either
  direction:
  ```
  c := t.Before(k)
  for c.Next() {
      // First key will be k, if it's present.
      fmt.Println(c.Key, c.Value)
  }
  ```

