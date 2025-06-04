# ChainDB

ChainDB is a high-performance key-value database library for Go, built on top of Pebble DB. It provides a simple and efficient interface for storing and retrieving data, with support for atomic batch operations, range queries, and more.

## Features

- 🔥 High performance: Built on top of Pebble DB, which is optimized for modern hardware
- 🔒 ACID transactions: Support for atomic batch operations
- 📊 Range queries: Efficient iteration over key ranges
- 🔄 Compaction: Built-in support for database optimization
- 🛠️ Simple API: Easy to use interface for common database operations
- 📑 Table support: Namespace your data with prefixed tables
- 🔄 Thread-safety: All batch operations are thread-safe by default
- ⛓️ Blockchain ready: Optimized for blockchain applications with atomic operations and efficient state management

## Installation

```bash
go get github.com/sxwebdev/chaindb
```

## Quick Start

Here's a simple example of how to use ChainDB:

```go
package main

import (
    "context"
    "log"
    "sync"

    "github.com/cockroachdb/pebble/v2"
    "github.com/sxwebdev/chaindb"
)

func main() {
    db, err := chaindb.NewDatabase("./data")
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()

    // Create a table with a prefix
    usersTable := chaindb.NewTable(db, []byte("users:"))
    settingsTable := chaindb.NewTable(db, []byte("settings:"))

    // Basic operations with tables
    usersTable.Put([]byte("john"), []byte("John Doe"))
    settingsTable.Put([]byte("theme"), []byte("dark"))

    // Read from tables
    userData, _ := usersTable.Get([]byte("john"))
    theme, _ := settingsTable.Get([]byte("theme"))

    // Batch operations with tables
    batch := usersTable.NewBatch()
    defer batch.Close()

    // Safe concurrent usage of batch operations
    var wg sync.WaitGroup
    wg.Add(2)

    go func() {
        defer wg.Done()
        batch.Put([]byte("alice"), []byte("Alice Smith"))
    }()

    go func() {
        defer wg.Done()
        batch.Put([]byte("bob"), []byte("Bob Johnson"))
    }()

    wg.Wait()
    batch.Write()

    // Range operations on tables
    iter, err := usersTable.NewIterator(context.Background(), nil)
    if err != nil {
        log.Fatal(err)
    }
    defer iter.Release()

    // Properly initialize iterator with First() call
    for valid := iter.First(); valid && iter.Error() == nil; valid = iter.Next() {
        key := iter.Key()
        value := iter.Value()
        // Process user data
    }
}
```

## Advanced Usage

### Table Operations

Tables provide a way to namespace your data within the database. Each table automatically prefixes all keys with a specified string:

```go
// Create tables for different data types
usersTable := chaindb.NewTable(db, []byte("users:"))
settingsTable := chaindb.NewTable(db, []byte("settings:"))
logsTable := chaindb.NewTable(db, []byte("logs:"))

// Operations on tables work the same as on the main database
usersTable.Put([]byte("user1"), []byte("data"))
settingsTable.Put([]byte("config"), []byte("value"))

// Tables support all database operations
batch := usersTable.NewBatch()
defer batch.Close()

batch.Put([]byte("user2"), []byte("data"))
batch.Write()

// Iterate over table contents
iter := usersTable.NewIterator(nil, nil)
defer iter.Release()
```

### Batch Operations

ChainDB supports two types of batch operations with tables, and all batch operations are thread-safe by default:

#### 1. Separate batches for each table

Each table creates its own batch. Such batches are committed independently.

```go
usersBatch := usersTable.NewBatch()
defer usersBatch.Close()

settingsBatch := settingsTable.NewBatch()
defer settingsBatch.Close()

usersBatch.Put([]byte("user1"), []byte("John"))
settingsBatch.Put([]byte("user1:theme"), []byte("dark"))

// Safe for concurrent use from multiple goroutines
go func() {
    usersBatch.Put([]byte("user2"), []byte("Alice"))
}()

if err := usersBatch.Write(); err != nil {
    log.Fatal(err)
}
if err := settingsBatch.Write(); err != nil {
    log.Fatal(err)
}
```

#### 2. One common batch for all tables (atomically)

You can create one batch at the database level and use it for all tables through `NewBatchFrom`. All changes will be written atomically in one operation.

```go
import (
    "log"
    "time"

    "github.com/sxwebdev/chaindb"
)

// ...

batch := db.NewBatch()
defer batch.Close()

usersBatch := usersTable.NewBatchFrom(batch)
settingsBatch := settingsTable.NewBatchFrom(batch)

usersBatch.Put([]byte("user1"), []byte("John"))
settingsBatch.Put([]byte("user1:theme"), []byte("dark"))

// Multiple goroutines can safely use the batches concurrently
go func() {
    usersBatch.Put([]byte("user2"), []byte("Alice"))
}()
go func() {
    settingsBatch.Put([]byte("user2:theme"), []byte("light"))
}()

// Allow time for concurrent operations to complete
time.Sleep(100 * time.Millisecond)

// All changes will be applied atomically
if err := batch.Write(); err != nil {
    log.Fatal(err)
}
```

**Difference:**

- Variant 1 — independent batches, committed separately.
- Variant 2 — all changes from different tables go into one batch and are committed atomically.

Both variants are fully thread-safe and can be used concurrently from multiple goroutines.

### Thread-safe Batch Operations

ChainDB provides thread-safe batch operations that can be safely used from multiple goroutines:

```go
package main

import (
    "fmt"
    "log"
    "sync"

    "github.com/sxwebdev/chaindb"
)

func main() {
    db, err := chaindb.NewDatabase("./data")
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()

    userTable := chaindb.NewTable(db, []byte("users:"))

    // Create a shared batch
    batch := userTable.NewBatch()
    defer batch.Close()

    // Use the batch concurrently from multiple goroutines
    var wg sync.WaitGroup
    wg.Add(10)

    for i := 0; i < 10; i++ {
        id := i // Capture loop variable
        go func() {
            defer wg.Done()
            key := []byte(fmt.Sprintf("user_%d", id))
            val := []byte(fmt.Sprintf("User #%d", id))
            batch.Put(key, val)
        }()
    }

    // Wait for all operations to complete
    wg.Wait()

    // Write all changes atomically
    if err := batch.Write(); err != nil {
        log.Fatal(err)
    }

    // Verify the data was written
    count := 0
    iter := userTable.NewIterator(context.Background(), nil)
    defer iter.Release()
    for valid := iter.First(); valid && iter.Error() == nil; valid = iter.Next() {
        count++
        log.Printf("Found: %s = %s", iter.Key(), iter.Value())
    }
    log.Printf("Total users: %d", count)
}
```

### Advantages of Thread-Safe Batches

The thread-safety features of ChainDB provide several advantages:

1. **Simplified concurrent programming**: No need to manually synchronize access to batch operations.
2. **Reduced boilerplate code**: Avoid writing additional synchronization code with mutexes or channels.
3. **Better performance**: Internal locking is optimized for the specific operations.
4. **Prevention of race conditions**: The batch implementation uses fine-grained locking to prevent data corruption.
5. **Easier parallel processing**: Process data in parallel and write it safely to the database.

Here's an example showing how to process data in parallel and write it atomically:

```go
func processDataConcurrently(db chaindb.Database, data []DataItem) error {
    table := chaindb.NewTable(db, []byte("processed:"))
    batch := table.NewBatch()
    defer batch.Close()

    var wg sync.WaitGroup
    errCh := make(chan error, len(data))

    // Process items in parallel
    for _, item := range data {
        wg.Add(1)
        go func(item DataItem) {
            defer wg.Done()

            // Process the item
            processedData, err := processItem(item)
            if err != nil {
                errCh <- err
                return
            }

            // Add to batch - thread-safe operation
            key := []byte(item.ID)
            if err := batch.Put(key, processedData); err != nil {
                errCh <- err
                return
            }
        }(item)
    }

    // Wait for all goroutines to complete
    wg.Wait()
    close(errCh)

    // Check for errors
    for err := range errCh {
        if err != nil {
            return err
        }
    }

    // Write all processed data atomically
    return batch.Write()
}
```

This pattern enables efficient parallel processing while maintaining data consistency.

### Range Queries

You can iterate over a range of keys:

```go
// Iterate over all keys
iter, err := db.NewIterator(context.Background(), nil)
if err != nil {
    log.Fatal(err)
}
defer iter.Release()

// Correctly iterate using a First()-based loop
for valid := iter.First(); valid && iter.Error() == nil; valid = iter.Next() {
    key := iter.Key()
    value := iter.Value()
    // Process key-value pair
}

// Iterate over keys with a specific prefix
iterOptions := &pebble.IterOptions{
    LowerBound: []byte("user:"),
    UpperBound: []byte("user;"), // Next byte after : is ; in ASCII
}
prefixIter, err := db.NewIterator(context.Background(), iterOptions)
if err != nil {
    log.Fatal(err)
}
defer prefixIter.Release()

// Properly initialize the iterator and check for errors
for valid := prefixIter.First(); valid && prefixIter.Error() == nil; valid = prefixIter.Next() {
    key := prefixIter.Key()
    value := prefixIter.Value()
    // Process key-value pair
}
```

### Database Maintenance

ChainDB provides methods for database maintenance:

```go
// Compact the database
db.Compact(nil, nil)

// Get database statistics
stats, _ := db.Stat()

// Sync data to disk
db.SyncKeyValue()
```

## Performance Considerations

- Use tables to organize and namespace your data
- Use batch operations for multiple writes
- Take advantage of built-in thread-safety for concurrent batch operations
- Implement proper error handling
- Close iterators after use
- Use appropriate cache sizes for your workload
- Consider using compression for large values

## Concurrency Support

ChainDB provides safe concurrent access to its APIs:

- **Thread-safe batches**: All batch operations are protected by internal locks and can be safely used from multiple goroutines without external synchronization.
- **Safe iterator usage**: While iterators themselves are not thread-safe and should not be shared between goroutines, multiple iterators can be created and used concurrently.
- **Atomic batch operations**: Use batch operations for atomicity when working with multiple keys.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

This project is licensed under the MIT License - see the LICENSE file for details.
