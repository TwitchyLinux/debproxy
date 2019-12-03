package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/boltdb/bolt"
)

type cache struct {
	store *bolt.DB
}

func (c *cache) Get(url string) []byte {
	var b []byte
	c.store.View(func(tx *bolt.Tx) error {
		v := tx.Bucket([]byte("DebPackages")).Get([]byte(url))
		if len(v) > 0 {
			b = make([]byte, len(v))
			copy(b, v)
		}
		return nil
	})
	return b
}

func (c *cache) Put(url string, data []byte) {
	c.store.Update(func(tx *bolt.Tx) error {
		if err := tx.Bucket([]byte("DebPackages")).Put([]byte(url), []byte(data)); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to commit %q: %v\n", url, err)
		}
		return nil
	})
}

func openCache(path string) *cache {
	db, err := bolt.Open(filepath.Join(path, "deb.cache"), 0644, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open %s: %v\n", filepath.Join(path, "deb.cache"), err)
		os.Exit(1)
	}
	db.Update(func(tx *bolt.Tx) error {
		tx.CreateBucket([]byte("DebPackages"))
		return nil
	})

	return &cache{store: db}
}
