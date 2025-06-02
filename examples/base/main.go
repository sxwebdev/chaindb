package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/sxwebdev/chaindb"
)

func main() {
	// Create a temporary directory for the database
	dbPath := filepath.Join(os.TempDir(), "chaindb_test")
	defer os.RemoveAll(dbPath)

	// Open the database
	db, err := chaindb.NewDatabase(dbPath)
	if err != nil {
		log.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Test Put and Get
	key := []byte("test_key")
	value := []byte("test_value")
	if err := db.Put(key, value); err != nil {
		log.Fatalf("Failed to put value: %v", err)
	}

	retrieved, err := db.Get(key)
	if err != nil {
		log.Fatalf("Failed to get value: %v", err)
	}
	fmt.Printf("Retrieved value: %s\n", string(retrieved))

	// Test Has
	exists, err := db.Has(key)
	if err != nil {
		log.Fatalf("Failed to check key existence: %v", err)
	}
	fmt.Printf("Key exists: %v\n", exists)

	// Test Batch operations
	batch := db.NewBatch()
	batch.Put([]byte("batch_key1"), []byte("batch_value1"))
	batch.Put([]byte("batch_key2"), []byte("batch_value2"))
	if err := batch.Write(); err != nil {
		log.Fatalf("Failed to write batch: %v", err)
	}

	// Test Iterator
	iter := db.NewIterator(context.Background(), nil, nil)
	defer iter.Release()

	fmt.Println("\nIterating through all keys:")
	for iter.Next() {
		fmt.Printf("Key: %s, Value: %s\n", string(iter.Key()), string(iter.Value()))
	}
	if err := iter.Error(); err != nil {
		log.Fatalf("Iterator error: %v", err)
	}

	// Test Delete
	if err := db.Delete(key); err != nil {
		log.Fatalf("Failed to delete key: %v", err)
	}

	// Check if key is deleted
	exists, err = db.Has(key)
	if err != nil {
		log.Fatalf("Failed to check key existence after deletion: %v", err)
	}
	fmt.Printf("\nKey exists after deletion: %v\n", exists)

	// Test DeleteRange
	start := []byte("batch_key1")
	end := []byte("batch_key2")
	if err := db.DeleteRange(start, end); err != nil {
		log.Fatalf("Failed to delete range: %v", err)
	}

	// Test Compact
	if err := db.Compact(nil, nil); err != nil {
		log.Fatalf("Failed to compact database: %v", err)
	}

	// Test Stat
	stats, err := db.Stat()
	if err != nil {
		log.Fatalf("Failed to get database stats: %v", err)
	}
	fmt.Printf("\nDatabase stats: %s\n", stats)

	// Test SyncKeyValue
	if err := db.SyncKeyValue(); err != nil {
		log.Fatalf("Failed to sync database: %v", err)
	}

	fmt.Println("All tests passed successfully")
}
