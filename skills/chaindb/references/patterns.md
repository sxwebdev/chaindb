# ChainDB Code Patterns & Conventions

## Table of Contents

- [Database Lifecycle](#database-lifecycle)
- [Table Usage](#table-usage)
- [Batch Patterns](#batch-patterns)
- [Iterator Patterns](#iterator-patterns)
- [Concurrency Patterns](#concurrency-patterns)
- [Testing Patterns](#testing-patterns)
- [Adding New Features Checklist](#adding-new-features-checklist)

---

## Database Lifecycle

### Opening

```go
db, err := chaindb.NewDatabase("./data",
    chaindb.WithCache(256),     // 256 MB cache
    chaindb.WithHandles(512),   // 512 file handles
)
if err != nil {
    return fmt.Errorf("open database: %w", err)
}
defer db.Close()
```

### Closing

Always close via `defer` or explicit cleanup. The database uses `quitLock`
(RWMutex) internally — all operations check `closed` state before proceeding.

---

## Table Usage

### Creating Tables

```go
users    := chaindb.NewTable(db, []byte("users:"))
blocks   := chaindb.NewTable(db, []byte("blocks:"))
metadata := chaindb.NewTable(db, []byte("meta:"))
```

Convention: use a colon `:` suffix in prefixes for readability in hex dumps.

### CRUD on Tables

```go
// Write
err := users.Put([]byte("alice"), []byte(`{"name":"Alice"}`))

// Read
val, err := users.Get([]byte("alice"))
if errors.Is(err, pebble.ErrNotFound) {
    // handle missing key
}

// Check existence
exists, err := users.Has([]byte("alice"))

// Delete
err = users.Delete([]byte("alice"))

// Range delete
err = users.DeleteRange([]byte("a"), []byte("b")) // deletes keys [a, b)
```

---

## Batch Patterns

### Simple Batch

```go
batch := table.NewBatch()
defer batch.Close()

for _, item := range items {
    if err := batch.Put(item.Key, item.Value); err != nil {
        return err
    }
}
return batch.Write()
```

### Large Import with Flush Threshold

```go
batch := table.NewBatch()
defer batch.Close()

for _, item := range items {
    batch.Put(item.Key, item.Value)

    if batch.ValueSize() >= chaindb.IdealBatchSize {
        if err := batch.Write(); err != nil {
            return err
        }
        batch.Reset()
    }
}
// Flush remaining
return batch.Write()
```

### Atomic Multi-Table Write

```go
// One underlying batch, multiple table views
batch := db.NewBatch()
defer batch.Close()

usersBatch    := users.NewBatchFrom(batch)
settingsBatch := settings.NewBatchFrom(batch)

usersBatch.Put([]byte("alice"), userData)
settingsBatch.Put([]byte("alice:theme"), []byte("dark"))

// Single atomic commit
if err := batch.Write(); err != nil {
    return err
}
```

### Pre-Allocated Batch

```go
batch := table.NewBatchWithSize(1024 * 1024) // 1 MB pre-alloc
defer batch.Close()
```

---

## Iterator Patterns

### Forward Scan (All Keys)

```go
iter, err := table.NewIterator(ctx, nil)
if err != nil {
    return err
}
defer iter.Release()

for valid := iter.First(); valid && iter.Error() == nil; valid = iter.Next() {
    key := iter.Key()
    value := iter.Value()
    // process
}
if err := iter.Error(); err != nil {
    return err
}
```

### Reverse Scan

```go
for valid := iter.Last(); valid && iter.Error() == nil; valid = iter.Prev() {
    // process in reverse order
}
```

### Bounded Scan (Raw Database)

```go
prefix := []byte("blocks:")
iter, err := db.NewIterator(ctx, &pebble.IterOptions{
    LowerBound: prefix,
    UpperBound: chaindb.NextPrefix(prefix),
})
```

Note: When using Tables, bounds are set automatically. Only use manual bounds
on the raw Database.

---

## Concurrency Patterns

### Per-Worker Batches (Recommended)

```go
g, ctx := errgroup.WithContext(ctx)

for i := range numWorkers {
    g.Go(func() error {
        batch := table.NewBatch()
        defer batch.Close()

        for j := range itemsPerWorker {
            key := fmt.Appendf(nil, "worker%d:item%d", i, j)
            batch.Put(key, value)
        }
        return batch.Write()
    })
}
return g.Wait()
```

### Shared Batch (Thread-Safe)

Batches have internal locking, so a shared batch works without external
synchronization:

```go
batch := table.NewBatch()
defer batch.Close()

g, _ := errgroup.WithContext(ctx)
for i := range numWorkers {
    g.Go(func() error {
        key := fmt.Appendf(nil, "item%d", i)
        return batch.Put(key, value)
    })
}
g.Wait()
batch.Write()
```

### Iterator Per Goroutine

Iterators are NOT thread-safe. Always create a separate iterator per goroutine:

```go
g.Go(func() error {
    iter, err := table.NewIterator(ctx, nil)
    if err != nil {
        return err
    }
    defer iter.Release()
    // use iter
    return nil
})
```

---

## Testing Patterns

### Test Setup

```go
func TestFeature(t *testing.T) {
    dir := t.TempDir()
    db, err := chaindb.NewDatabase(dir)
    require.NoError(t, err)
    defer db.Close()

    table := chaindb.NewTable(db, []byte("test:"))
    // ... test logic
}
```

### Data Verification Helper

```go
func verifyData(t *testing.T, table chaindb.Table, prefix string, count int) {
    t.Helper()
    for i := range count {
        key := fmt.Appendf(nil, "%s%d", prefix, i)
        has, err := table.Has(key)
        require.NoError(t, err)
        assert.True(t, has, "missing key: %s", key)
    }
}
```

### Counting Keys with Iterator

```go
func countItems(t *testing.T, table chaindb.Table, prefix string) int {
    t.Helper()
    iter, err := table.NewIterator(context.Background(), &pebble.IterOptions{
        LowerBound: []byte(prefix),
        UpperBound: chaindb.NextPrefix([]byte(prefix)),
    })
    require.NoError(t, err)
    defer iter.Release()

    count := 0
    for valid := iter.First(); valid && iter.Error() == nil; valid = iter.Next() {
        count++
    }
    require.NoError(t, iter.Error())
    return count
}
```

---

## Adding New Features Checklist

When adding a new method or capability to chaindb:

1. **Interface** — Define or extend an interface in `database.go`, `batch.go`,
   or `iterator.go`. Keep it minimal.

2. **Compose** — If it's a new interface, compose it into `Database` in
   `database.go`.

3. **Implement on pebbleDB** — Add the method in `pebble.go`. Follow existing
   patterns: check `closed` state with `quitLock.RLock()`, validate keys with
   `ErrEmptyKey`, wrap Pebble errors.

4. **Implement on table** — Add the method in `table.go`. Prepend prefix to
   keys using `slices.Concat(t.prefix, key)`. Delegate to `t.db`.

5. **Implement on batch/iterator** — If the feature involves batch or iterator,
   implement on both `batch`/`tableBatch` or `pebbleIterator`/`tableIterator`.

6. **Test** — Add tests in the appropriate `_test.go` file. Test both direct
   database use and table use. Include concurrency tests if the feature
   involves shared state.

7. **Document** — Update `doc.go` and `README.md` if the feature is
   user-facing.
