// Package chaindb provides a high-performance, thread-safe key-value database
// abstraction built on top of Pebble DB (CockroachDB's LSM-based storage engine).
// It is specifically optimized for blockchain applications and other use cases
// requiring atomic batch operations, efficient state management, and safe concurrent access.
//
// # Overview
//
// ChainDB wraps Pebble DB to provide:
//   - Clean, user-friendly API for key-value storage
//   - Thread-safe batch operations without external synchronization
//   - Table-based namespacing with prefix isolation
//   - Atomic multi-table transactions via shared batches
//   - Efficient iteration with automatic prefix handling
//
// # Quick Start
//
// Basic usage:
//
//	db, err := chaindb.NewDatabase("./data")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer db.Close()
//
//	// Simple key-value operations
//	db.Put([]byte("key"), []byte("value"))
//	val, err := db.Get([]byte("key"))
//	exists, err := db.Has([]byte("key"))
//	db.Delete([]byte("key"))
//
// # Tables (Namespacing)
//
// Tables provide logical separation of data using key prefixes.
// All keys in a table are automatically prefixed, and the prefix
// is transparently stripped when reading:
//
//	users := chaindb.NewTable(db, []byte("users:"))
//	settings := chaindb.NewTable(db, []byte("settings:"))
//
//	// Key "alice" is stored as "users:alice" in the underlying database
//	users.Put([]byte("alice"), []byte("user data"))
//
//	// Key "theme" is stored as "settings:theme"
//	settings.Put([]byte("theme"), []byte("dark"))
//
//	// When reading, the prefix is automatically stripped
//	val, _ := users.Get([]byte("alice"))  // Works as expected
//
// # Batch Operations
//
// Batches accumulate multiple operations and apply them atomically.
// All batch types are fully thread-safe - multiple goroutines can
// safely call Put/Delete concurrently without external locking:
//
//	batch := db.NewBatch()
//	defer batch.Close()
//
//	// Safe to call from multiple goroutines
//	var wg sync.WaitGroup
//	for i := 0; i < 100; i++ {
//	    wg.Add(1)
//	    go func(id int) {
//	        defer wg.Done()
//	        key := []byte(fmt.Sprintf("key_%d", id))
//	        batch.Put(key, []byte("value"))
//	    }(i)
//	}
//	wg.Wait()
//
//	// All operations are applied atomically
//	if err := batch.Write(); err != nil {
//	    log.Fatal(err)
//	}
//
// # Atomic Multi-Table Transactions
//
// For atomic updates across multiple tables, use a shared batch:
//
//	// Create a shared batch from the underlying database
//	sharedBatch := db.NewBatch()
//	defer sharedBatch.Close()
//
//	// Create table-specific batch views
//	userBatch := users.NewBatchFrom(sharedBatch)
//	settingBatch := settings.NewBatchFrom(sharedBatch)
//
//	// Operations on different tables
//	userBatch.Put([]byte("alice"), []byte("new user data"))
//	settingBatch.Put([]byte("alice:theme"), []byte("light"))
//
//	// Single atomic commit - both tables updated together
//	if err := sharedBatch.Write(); err != nil {
//	    log.Fatal(err)
//	}
//
// # Iteration
//
// Iterators allow sequential access to key-value pairs.
// For tables, key prefixes are automatically stripped:
//
//	iter, err := db.NewIterator(context.Background(), nil)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer iter.Release()
//
//	for valid := iter.First(); valid && iter.Error() == nil; valid = iter.Next() {
//	    key := iter.Key()     // Read-only, valid until next iteration
//	    value := iter.Value() // Read-only, valid until next iteration
//	    fmt.Printf("%s: %s\n", key, value)
//	}
//
//	if err := iter.Error(); err != nil {
//	    log.Fatal(err)
//	}
//
// Range iteration with bounds:
//
//	opts := &pebble.IterOptions{
//	    LowerBound: []byte("a"),
//	    UpperBound: []byte("z"),
//	}
//	iter, _ := table.NewIterator(context.Background(), opts)
//	defer iter.Release()
//
// Note: Iterators are NOT thread-safe individually, but multiple
// iterators can exist and operate concurrently on the same database.
//
// # Configuration Options
//
// The database can be configured with various options:
//
//	db, err := chaindb.NewDatabase("./data",
//	    chaindb.WithCache(256),           // 256 MB cache
//	    chaindb.WithHandles(512),         // Max 512 open files
//	    chaindb.WithNoSync(true),         // Async writes (faster, less durable)
//	    chaindb.WithLogger(myLogger),     // Custom logger
//	)
//
// Available options:
//   - WithCache(cacheMB int) - Set cache size in MB (minimum 16 MB)
//   - WithHandles(handles int) - Set max open file handles (minimum 16)
//   - WithReadonly(readonly bool) - Open database in read-only mode
//   - WithNoSync(noSync bool) - Disable fsync for faster writes (risk of data loss on crash)
//   - WithLogger(logger pebble.Logger) - Set custom logger
//   - WithWALBytesPerSync(bytes int) - Set WAL sync threshold
//   - WithIdealWALBytesPerSync() - Auto-set WAL sync (5x IdealBatchSize)
//   - WithPebbleLevels(levels [7]LevelOptions) - Fine-grained LSM tree tuning
//
// # Database Maintenance
//
// Compaction flattens the LSM tree, reducing space and improving read performance:
//
//	// Compact entire database
//	err := db.Compact(context.Background(), nil, nil)
//
//	// Compact specific range
//	err := db.Compact(context.Background(), []byte("a"), []byte("z"))
//
// Force WAL flush to disk:
//
//	err := db.SyncKeyValue()
//
// Get database statistics:
//
//	stats, err := db.Stat()
//	fmt.Println(stats)
//
// Access underlying Pebble instance (for advanced use cases):
//
//	pebbleDB := db.Pebble()
//
// # Thread Safety
//
// ChainDB provides the following thread-safety guarantees:
//
//   - Database operations (Get, Put, Delete, Has): Safe for concurrent use
//   - Batch operations (Put, Delete, Write, Reset): Fully thread-safe with internal locking
//   - Iterators: NOT thread-safe individually, but multiple iterators can run concurrently
//   - Table operations: Inherit thread-safety from underlying Database/Batch
//
// # Error Handling
//
// The package defines the following errors:
//
//   - ErrEmptyKey: Returned when an operation is attempted with an empty key
//   - ErrTooManyKeys: Returned when DeleteRange would affect too many keys
//   - pebble.ErrClosed: Returned when operating on a closed database
//   - pebble.ErrNotFound: Returned by Get when the key does not exist
//
// # Performance Considerations
//
// Advantages:
//   - Built on Pebble DB, optimized for modern SSD hardware
//   - Thread-safe batches eliminate need for external synchronization
//   - Efficient prefix-based table isolation
//   - Optional async write mode for high-throughput scenarios
//
// Considerations:
//   - LSM tree write amplification (inherent to the storage model)
//   - Memory overhead for caches and open file handles
//   - Regular compaction recommended for long-running databases
//
// # Constants
//
// The package provides helpful constants:
//
//   - IdealBatchSize (100 KB): Recommended batch size for optimal write performance
//
// Example usage:
//
//	batch := db.NewBatch()
//	for {
//	    batch.Put(key, value)
//	    if batch.ValueSize() >= chaindb.IdealBatchSize {
//	        batch.Write()
//	        batch.Reset()
//	    }
//	}
package chaindb
