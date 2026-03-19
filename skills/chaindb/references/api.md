# ChainDB API Reference

## Table of Contents

- [Interfaces](#interfaces)
- [Factory Functions](#factory-functions)
- [Configuration Options](#configuration-options)
- [Helper Functions](#helper-functions)
- [Constants](#constants)
- [Errors](#errors)
- [Type Relationships](#type-relationships)

---

## Interfaces

### KeyValueReader

```go
type KeyValueReader interface {
    Has(key []byte) (bool, error)
    Get(key []byte) ([]byte, error)
}
```

Basic read operations. Returns `pebble.ErrNotFound` from Get when key doesn't
exist. Returns `ErrEmptyKey` for nil/empty keys.

### KeyValueWriter

```go
type KeyValueWriter interface {
    Put(key []byte, value []byte) error
    Delete(key []byte) error
}
```

Basic write operations. Returns `ErrEmptyKey` for nil/empty keys.

### KeyValueRangeDeleter

```go
type KeyValueRangeDeleter interface {
    DeleteRange(start, end []byte) error
}
```

Deletes all keys in range `[start, end)`. The range is half-open — start is
inclusive, end is exclusive.

### KeyValueStater

```go
type KeyValueStater interface {
    Stat() (string, error)
}
```

Returns Pebble metrics as a formatted string.

### KeyValueSyncer

```go
type KeyValueSyncer interface {
    SyncKeyValue() error
}
```

Flushes the WAL to stable storage.

### Compacter

```go
type Compacter interface {
    Compact(ctx context.Context, start []byte, limit []byte) error
}
```

Compacts the key range. Respects context cancellation. Implementation
parallelizes compaction by splitting the range.

### Database

```go
type Database interface {
    KeyValueReader
    KeyValueWriter
    KeyValueStater
    KeyValueSyncer
    KeyValueRangeDeleter
    Batcher
    Iteratee
    Compacter
    io.Closer
    Pebble() *pebble.DB
}
```

Composite interface — the main entry point. `Pebble()` exposes the underlying
engine for advanced operations not covered by the wrapper.

### Batch

```go
type Batch interface {
    KeyValueWriter
    ValueSize() int
    Write() error
    DeleteRange(start, end []byte) error
    Reset()
    Close() error
    Replay(w KeyValueWriter) error
}
```

Atomic operation group. `ValueSize()` returns accumulated data size.
`Replay()` re-applies all operations to another writer.

### Batcher

```go
type Batcher interface {
    NewBatch() Batch
    NewBatchWithSize(size int) Batch
    NewBatchFrom(batch Batch) Batch
}
```

Factory for batches. `NewBatchWithSize` pre-allocates.
`NewBatchFrom` wraps an existing batch with prefix handling (used by Tables).

### Iterator

```go
type Iterator interface {
    First() bool
    Last() bool
    Next() bool
    Prev() bool
    Error() error
    Key() []byte
    Value() []byte
    Release() error
}
```

Bidirectional key scanner. Check `Error()` after iteration loop ends.
Always call `Release()` when done.

### Iteratee

```go
type Iteratee interface {
    NewIterator(ctx context.Context, iterOptions *pebble.IterOptions) (Iterator, error)
}
```

Factory for iterators. Pass `nil` for default options (full range).

### Table

```go
type Table interface {
    Database
    Prefix() []byte
}
```

Prefix-isolated namespace. Implements full Database interface with automatic
key prefixing/stripping.

---

## Factory Functions

### NewDatabase

```go
func NewDatabase(dirname string, opts ...Option) (Database, error)
```

Opens or creates a Pebble database at `dirname`. Accepts functional options.

### NewTable

```go
func NewTable(db Database, prefix []byte) Table
```

Creates a prefix-isolated table over an existing database.

---

## Configuration Options

```go
type Option func(*options)

func WithCache(cache int) Option           // Cache size in MB (min 16)
func WithHandles(handles int) Option       // Max file handles (min 16)
func WithReadonly(readonly bool) Option     // Read-only mode
func WithNoSync(noSync bool) Option        // Disable fsync (faster, less durable)
func WithWALBytesPerSync(n int) Option     // WAL sync threshold in bytes
func WithIdealWALBytesPerSync() Option     // Auto: 5 * IdealBatchSize
func WithPebbleLevels(l [7]pebble.LevelOptions) Option  // LSM level config
func WithLogger(logger pebble.Logger) Option             // Custom logger
```

---

## Helper Functions

### NextPrefix

```go
func NextPrefix(prefix []byte) []byte
```

Returns the lexicographically next prefix. Used for iteration bounds.

### UpperBound

```go
func UpperBound(prefix []byte) (limit []byte)
```

Calculates upper bound for prefix scanning. Increments the last non-0xFF byte.
Returns empty slice if all bytes are 0xFF.

Example: `[0x01, 0x02]` → `[0x01, 0x03]`

---

## Constants

```go
const IdealBatchSize = 100 * 1024  // 100 KB — recommended batch flush threshold
```

---

## Errors

```go
var ErrEmptyKey    = errors.New("empty key")
var ErrTooManyKeys = errors.New("too many keys in deleted range")
```

Also uses `pebble.ErrNotFound` and `pebble.ErrClosed` from the Pebble package.

---

## Type Relationships

```text
Database (interface)
├── pebbleDB (implementation)     — pebble.go
└── table (implementation)        — table.go
    └── uses Database internally

Batch (interface)
├── batch (implementation)        — pebble.go
└── tableBatch (implementation)   — table.go
    └── wraps Batch with prefix

Iterator (interface)
├── pebbleIterator (implementation) — pebble.go
└── tableIterator (implementation)  — table.go
    └── wraps Iterator, strips prefix
```
