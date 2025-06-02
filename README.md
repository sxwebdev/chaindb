# ChainDB

ChainDB is a high-performance key-value database library for Go, built on top of Pebble DB. It provides a simple and efficient interface for storing and retrieving data, with support for atomic batch operations, range queries, and more.

## Features

- 🔥 High performance: Built on top of Pebble DB, which is optimized for modern hardware
- 🔒 ACID transactions: Support for atomic batch operations
- 📊 Range queries: Efficient iteration over key ranges
- 🔄 Compaction: Built-in support for database optimization
- 🛠️ Simple API: Easy to use interface for common database operations
- 📑 Table support: Namespace your data with prefixed tables
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
    "log"

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
    batch.Put([]byte("alice"), []byte("Alice Smith"))
    batch.Put([]byte("bob"), []byte("Bob Johnson"))
    batch.Write()

    // Range operations on tables
    iter := usersTable.NewIterator(nil, nil)
    defer iter.Release()
    for iter.Next() {
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
batch.Put([]byte("user2"), []byte("data"))
batch.Write()

// Iterate over table contents
iter := usersTable.NewIterator(nil, nil)
defer iter.Release()
```

### Batch Operations

ChainDB supports two types of batch operations with tables:

#### 1. Separate batches for each table

Each table creates its own batch. Such batches are committed independently.

```go
usersBatch := usersTable.NewBatch()
settingsBatch := settingsTable.NewBatch()

usersBatch.Put([]byte("user1"), []byte("John"))
settingsBatch.Put([]byte("user1:theme"), []byte("dark"))

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
batch := db.NewBatch()
usersBatch := usersTable.NewBatchFrom(batch)
settingsBatch := settingsTable.NewBatchFrom(batch)

usersBatch.Put([]byte("user1"), []byte("John"))
settingsBatch.Put([]byte("user1:theme"), []byte("dark"))

// All changes will be applied atomically
if err := batch.Write(); err != nil {
    log.Fatal(err)
}
```

**Difference:**

- Variant 1 — independent batches, committed separately.
- Variant 2 — all changes from different tables go into one batch and are committed atomically.

### Range Queries

You can iterate over a range of keys:

```go
// Iterate over all keys
iter := db.NewIterator(nil, nil)
defer iter.Release()

// Iterate over keys with a specific prefix
prefix := []byte("user:")
iter := db.NewIterator(prefix, nil)
defer iter.Release()

for iter.Next() {
    key := iter.Key()
    value := iter.Value()
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
- Implement proper error handling
- Close iterators after use
- Use appropriate cache sizes for your workload
- Consider using compression for large values

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

This project is licensed under the MIT License - see the LICENSE file for details.
