---
name: chaindb
description: >-
  Go key-value database library built on Pebble DB (github.com/sxwebdev/chaindb),
  optimized for blockchain workloads. Use this skill whenever working in the chaindb
  codebase — editing database interfaces, batch operations, table prefix isolation,
  iterators, options, or tests. Also triggers when code imports "chaindb",
  "sxwebdev/chaindb", or references chaindb.Database, chaindb.Table, chaindb.Batch,
  chaindb.Iterator, chaindb.NewDatabase, chaindb.NewTable, chaindb.NewBatch,
  KeyValueReader, KeyValueWriter, KeyValueRangeDeleter, Batcher, Iteratee, Compacter,
  IdealBatchSize, NextPrefix, UpperBound, or pebble.IterOptions in a chaindb context.
  Applies when the user mentions Pebble DB wrapper, prefix-isolated tables, atomic
  batch writes, key-value iteration, thread-safe batch operations, or blockchain
  database storage in Go.
user-invocable: false
---

# ChainDB — Pebble DB Wrapper for Go

## Overview

ChainDB is a thread-safe key-value database library wrapping CockroachDB's Pebble
storage engine. It provides prefix-isolated tables, atomic batch operations, and
bidirectional iterators — designed for blockchain and high-throughput Go applications.

The architecture follows a layered decorator pattern:

- **Database** — core Pebble wrapper with CRUD, compaction, and stats
- **Table** — prefix-isolated namespace over a Database (automatic key prefixing)
- **Batch** — atomic multi-operation commit with thread-safe locking
- **Iterator** — bidirectional key scanning with automatic prefix stripping

## Key Files

| File          | Purpose                                                                |
| ------------- | ---------------------------------------------------------------------- |
| `database.go` | Interface definitions (KeyValueReader, KeyValueWriter, Database, etc.) |
| `pebble.go`   | Pebble wrapper implementation, batch, iterator                         |
| `table.go`    | Table with prefix isolation, tableBatch, tableIterator                 |
| `batch.go`    | Batch interface definition                                             |
| `iterator.go` | Iterator interface definition                                          |
| `options.go`  | Functional options (WithCache, WithHandles, etc.)                      |
| `helpers.go`  | NextPrefix(), UpperBound() utilities                                   |
| `errors.go`   | ErrEmptyKey, ErrTooManyKeys                                            |
| `doc.go`      | Package-level documentation                                            |

## Instructions

### Creating or Modifying Interfaces

1. All public interfaces live in `database.go`, `batch.go`, and `iterator.go`.
   Keep interfaces small and composable — the Database interface is a composite
   of KeyValueReader, KeyValueWriter, KeyValueStater, KeyValueSyncer,
   KeyValueRangeDeleter, Batcher, Iteratee, Compacter, and io.Closer.

2. When adding a new capability, define a focused interface first, then compose
   it into Database. This preserves the ability to accept narrow interfaces in
   function signatures.

3. Any new interface method must be implemented on both `pebbleDB` (in `pebble.go`)
   and `table` (in `table.go`) because Table also implements Database.

### Working with Tables (Prefix Isolation)

1. Tables prepend a prefix to every key transparently using `slices.Concat()`.
   When reading back, the prefix is stripped from iterator keys.

2. `NewTable(db, prefix)` creates a table. The prefix byte slice is stored as-is —
   include any delimiter in the prefix itself (e.g., `[]byte("users:")`).

3. Table iterators automatically set `LowerBound` and `UpperBound` on
   `pebble.IterOptions` to scope iteration within the prefix range.
   `UpperBound()` helper increments the last non-0xFF byte.

4. Cross-table atomic writes use `NewBatchFrom()` — multiple table batches sharing
   a single underlying database batch, committed atomically with one `batch.Write()`.

### Working with Batches

1. Batch operations are thread-safe via `sync.RWMutex` — Put, Delete, DeleteRange
   take a write lock; ValueSize takes a read lock.

2. Always `defer batch.Close()` after creating a batch to prevent resource leaks.

3. Use `IdealBatchSize` (100 KB) as a guideline for when to flush batches.
   For large imports, check `batch.ValueSize()` and call `Write()` + `Reset()`
   when approaching this threshold.

4. `Replay(w KeyValueWriter)` re-applies batch operations to another writer —
   useful for migration or replication patterns.

### Working with Iterators

1. Iterators are NOT thread-safe. Each goroutine must create its own iterator.

2. Always `defer iter.Release()` immediately after creation.

3. Standard iteration pattern:

   ```go
   iter, err := table.NewIterator(ctx, nil)
   if err != nil { return err }
   defer iter.Release()

   for valid := iter.First(); valid && iter.Error() == nil; valid = iter.Next() {
       // iter.Key(), iter.Value()
   }
   ```

4. Use `pebble.IterOptions` with `LowerBound`/`UpperBound` to scope iteration.
   For prefix scanning on raw Database (not Table), set bounds using
   `NextPrefix()` or `UpperBound()` helpers.

### Configuration Options

Use the functional options pattern when creating a database:

| Option                              | Purpose                           | Default         |
| ----------------------------------- | --------------------------------- | --------------- |
| `WithCache(mb)`                     | Read/write cache in MB            | 16 MB min       |
| `WithHandles(n)`                    | Max open file descriptors         | 16 min          |
| `WithReadonly(bool)`                | Read-only mode                    | false           |
| `WithNoSync(bool)`                  | Async writes (less durable)       | false           |
| `WithWALBytesPerSync(n)`            | WAL sync threshold                | —               |
| `WithIdealWALBytesPerSync()`        | Auto WAL sync (5x IdealBatchSize) | —               |
| `WithPebbleLevels([7]LevelOptions)` | LSM level config                  | Pebble defaults |
| `WithLogger(logger)`                | Custom Pebble logger              | —               |

### Writing Tests

1. Use `testify` (assert/require) — consistent with existing tests.

2. Create a temp directory with `t.TempDir()` for each test database.

3. Always close the database in cleanup: `defer db.Close()`.

4. For concurrency tests, use `sync.WaitGroup` or `errgroup.WithContext`.
   See `batch_test.go` for patterns including shared-batch and per-worker-batch
   approaches.

5. Verify data with iteration — create helper functions like `verifyData()`
   and `countItems()` from `batch_test.go`.

### Error Handling

- `ErrEmptyKey` — returned by Get, Put, Delete, Has when key is nil or empty.
  Always validate keys at the boundary.
- `pebble.ErrNotFound` — returned by Get when key doesn't exist. Use `Has()`
  first if you need to distinguish "not found" from other errors.
- `pebble.ErrClosed` — operations on a closed database. Guard with the internal
  `quitLock` pattern.

### Thread-Safety Model

- Database CRUD operations: thread-safe (Pebble guarantees)
- Batch operations: thread-safe (internal RWMutex)
- Iterators: NOT thread-safe (one per goroutine)
- Close: protected by quitLock to prevent use-after-close

## Examples

**Example 1: Adding a new Database method**

Input: User asks to add a `Count(prefix []byte) (int, error)` method.

Output:

1. Add `Count` to a new interface in `database.go`
2. Compose the new interface into `Database`
3. Implement on `pebbleDB` in `pebble.go` using an iterator with prefix bounds
4. Implement on `table` in `table.go` (prepend table prefix, delegate to db)
5. Add test in a `_test.go` file using `t.TempDir()` + testify

**Example 2: Atomic multi-table write**

```go
db, _ := chaindb.NewDatabase(dir)
defer db.Close()

users := chaindb.NewTable(db, []byte("users:"))
settings := chaindb.NewTable(db, []byte("settings:"))

// Shared batch for atomic commit
batch := db.NewBatch()
defer batch.Close()

ub := users.NewBatchFrom(batch)
sb := settings.NewBatchFrom(batch)

ub.Put([]byte("alice"), []byte(`{"name":"Alice"}`))
sb.Put([]byte("alice:theme"), []byte("dark"))

batch.Write() // Both writes commit atomically
```

## Key Principles

1. **Compose small interfaces** — narrow function signatures accept only what
   they need (KeyValueReader vs full Database). This makes testing and mocking
   straightforward.

2. **Prefix transparency** — Tables handle all prefix logic internally. Code
   using a Table should never manually prepend/strip prefixes, because that
   breaks the abstraction and leads to key corruption.

3. **Batch lifecycle** — Create, operate, Write, Close. Never reuse a batch
   after Close. Use Reset only to clear and reuse within the same lifecycle.

4. **Iterator scoping** — Always scope iterators with bounds or use Tables.
   Unbounded iteration on a large database is expensive and defeats the purpose
   of prefix isolation.

5. **Thread-safety by design** — Internal locking means callers don't need
   external synchronization for batches. But iterators are intentionally
   unsynchronized for performance — document this in any new iterator code.
