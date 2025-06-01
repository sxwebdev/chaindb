# ChainDB

ChainDB is a high-performance key-value database library for Go, built on top of Pebble DB. It provides a simple and efficient interface for storing and retrieving data, with support for atomic batch operations, range queries, and more.

## Features

- 🔥 High performance: Built on top of Pebble DB, which is optimized for modern hardware
- 🔒 ACID transactions: Support for atomic batch operations
- 📊 Range queries: Efficient iteration over key ranges
- 🔄 Compaction: Built-in support for database optimization
- 📝 Logging: Flexible logging interface for monitoring and debugging
- 🛠️ Simple API: Easy to use interface for common database operations
- 📑 Table support: Namespace your data with prefixed tables

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
    "os"
    "path/filepath"

    "github.com/sxwebdev/chaindb"
)

// SimpleLogger implements the chaindb.Logger interface
type SimpleLogger struct {
    *log.Logger
}

func (l *SimpleLogger) Debug(msg string, ctx ...any) {
    l.Printf("[DEBUG] %s %v", msg, ctx)
}

func (l *SimpleLogger) Info(msg string, ctx ...any) {
    l.Printf("[INFO] %s %v", msg, ctx)
}

func (l *SimpleLogger) Warn(msg string, ctx ...any) {
    l.Printf("[WARN] %s %v", msg, ctx)
}

func (l *SimpleLogger) Error(msg string, ctx ...any) {
    l.Printf("[ERROR] %s %v", msg, ctx)
}

func main() {
    // Create a database instance
    dbPath := filepath.Join(os.TempDir(), "chaindb_test")
    logger := &SimpleLogger{log.New(os.Stdout, "", log.LstdFlags)}

    db, err := chaindb.NewDatabase(logger, dbPath, 16, 16, "test", false)
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()

    // Create a table with a prefix
    usersTable := chaindb.NewTable(db, "users:")
    settingsTable := chaindb.NewTable(db, "settings:")

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
usersTable := chaindb.NewTable(db, "users:")
settingsTable := chaindb.NewTable(db, "settings:")
logsTable := chaindb.NewTable(db, "logs:")

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

Batch operations allow you to perform multiple writes atomically:

```go
batch := db.NewBatch()
batch.Put([]byte("key1"), []byte("value1"))
batch.Put([]byte("key2"), []byte("value2"))
batch.Delete([]byte("key3"))
err := batch.Write()
```

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
