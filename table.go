package chaindb

import (
	"context"
	"fmt"
	"slices"
	"sync"

	"github.com/cockroachdb/pebble/v2"
)

type Table interface {
	Database
	Prefix() []byte
}

// table is a wrapper around a database that prefixes each key access with a pre-
// configured string.
type table struct {
	db     Database
	prefix []byte
}

// NewTable returns a database object that prefixes all keys with a given string.
func NewTable(db Database, prefix []byte) Table {
	return &table{
		db:     db,
		prefix: prefix,
	}
}

// Prefix returns the prefix of the table.
func (t *table) Prefix() []byte {
	return slices.Clone(t.prefix)
}

// Pebble returns the underlying pebble database.
func (t *table) Pebble() *pebble.DB {
	return t.db.Pebble()
}

// Close is a noop to implement the Database interface.
func (t *table) Close() error {
	return nil
}

// Has retrieves if a prefixed version of a key is present in the database.
func (t *table) Has(key []byte) (bool, error) {
	return t.db.Has(append(t.prefix, key...))
}

// Get retrieves the given prefixed key if it's present in the database.
func (t *table) Get(key []byte) ([]byte, error) {
	return t.db.Get(append(t.prefix, key...))
}

// Put inserts the given value into the database at a prefixed version of the
// provided key.
func (t *table) Put(key []byte, value []byte) error {
	return t.db.Put(append(t.prefix, key...), value)
}

// Delete removes the given prefixed key from the database.
func (t *table) Delete(key []byte) error {
	return t.db.Delete(append(t.prefix, key...))
}

// DeleteRange deletes all of the keys (and values) in the range [start,end)
// (inclusive on start, exclusive on end).
func (t *table) DeleteRange(start, end []byte) error {
	prefixedStart := slices.Clone(t.prefix)
	prefixedStart = append(prefixedStart, start...)

	prefixedEnd := slices.Clone(t.prefix)
	prefixedEnd = append(prefixedEnd, end...)

	return t.db.DeleteRange(prefixedStart, prefixedEnd)
}

// NewIterator creates a binary-alphabetical iterator over a subset
// of database content with a particular key prefix, starting at a particular
// initial key (or after, if it does not exist).
func (t *table) NewIterator(ctx context.Context, iterOptions *pebble.IterOptions) (Iterator, error) {
	if iterOptions != nil {
		clonedLower := slices.Clone(iterOptions.LowerBound)
		clonedUpper := slices.Clone(iterOptions.UpperBound)

		iterOptions.LowerBound = append(t.prefix, clonedLower...)
		iterOptions.UpperBound = append(t.prefix, clonedUpper...)
	} else {
		iterOptions = &pebble.IterOptions{
			LowerBound: t.prefix,
			UpperBound: UpperBound(t.prefix),
		}
	}

	iter, err := t.db.NewIterator(ctx, iterOptions)
	if err != nil {
		return nil, err
	}

	return &tableIterator{
		iter:   iter,
		prefix: t.prefix,
	}, nil
}

// Stat returns the statistic data of the database.
func (t *table) Stat() (string, error) {
	return t.db.Stat()
}

// Compact flattens the underlying data store for the given key range. In essence,
// deleted and overwritten versions are discarded, and the data is rearranged to
// reduce the cost of operations needed to access them.
//
// A nil start is treated as a key before all keys in the data store; a nil limit
// is treated as a key after all keys in the data store. If both is nil then it
// will compact entire data store.
func (t *table) Compact(start []byte, limit []byte) error {
	// If no start was specified, use the table prefix as the first value
	if start == nil {
		start = t.prefix
	} else {
		start = append(t.prefix, start...)
	}
	// If no limit was specified, use the first element not matching the prefix
	// as the limit
	if limit == nil {
		limit = t.prefix
		for i := len(limit) - 1; i >= 0; i-- {
			// Bump the current character, stopping if it doesn't overflow
			limit[i]++
			if limit[i] > 0 {
				break
			}
			// Character overflown, proceed to the next or nil if the last
			if i == 0 {
				limit = nil
			}
		}
	} else {
		limit = append(t.prefix, limit...)
	}
	// Range correctly calculated based on table prefix, delegate down
	return t.db.Compact(start, limit)
}

// SyncKeyValue ensures that all pending writes are flushed to disk,
// guaranteeing data durability up to the point.
func (t *table) SyncKeyValue() error {
	return t.db.SyncKeyValue()
}

// NewBatch creates a write-only database that buffers changes to its host db
// until a final write is called, each operation prefixing all keys with the
// pre-configured string.
func (t *table) NewBatch() Batch {
	return &tableBatch{t.db.NewBatch(), t.prefix, sync.RWMutex{}}
}

// NewBatchWithSize creates a write-only database batch with pre-allocated buffer.
func (t *table) NewBatchWithSize(size int) Batch {
	return &tableBatch{t.db.NewBatchWithSize(size), t.prefix, sync.RWMutex{}}
}

// NewBatchFrom creates a new batch that will write to this table using the provided batch
func (t *table) NewBatchFrom(batch Batch) Batch {
	return &tableBatch{
		batch:  batch,
		prefix: t.prefix,
		lock:   sync.RWMutex{},
	}
}

// tableBatch is a write-only database that commits changes to its host database
// when Write is called. A batch can be used concurrently.
type tableBatch struct {
	batch  Batch
	prefix []byte

	lock sync.RWMutex
}

// Put inserts the given value into the batch for key.
func (b *tableBatch) Put(key, value []byte) error {
	b.lock.Lock()
	defer b.lock.Unlock()
	return b.batch.Put(append(b.prefix, key...), value)
}

// Delete removes the key from the batch.
func (b *tableBatch) Delete(key []byte) error {
	b.lock.Lock()
	defer b.lock.Unlock()
	return b.batch.Delete(append(b.prefix, key...))
}

// ValueSize retrieves the amount of data queued up for writing.
func (b *tableBatch) ValueSize() int {
	b.lock.RLock()
	defer b.lock.RUnlock()
	return b.batch.ValueSize()
}

// Write flushes any accumulated data to disk.
func (b *tableBatch) Write() error {
	b.lock.Lock()
	defer b.lock.Unlock()
	return b.batch.Write()
}

// Reset resets the batch for reuse.
func (b *tableBatch) Reset() {
	b.lock.Lock()
	defer b.lock.Unlock()
	b.batch.Reset()
}

// Close closes the batch and releases any resources it holds.
func (b *tableBatch) Close() error {
	b.lock.Lock()
	defer b.lock.Unlock()
	return b.batch.Close()
}

// tableReplayer is a wrapper around a batch replayer which truncates
// the added prefix.
type tableReplayer struct {
	w      KeyValueWriter
	prefix []byte
}

// Put implements the interface KeyValueWriter.
func (r *tableReplayer) Put(key []byte, value []byte) error {
	if key == nil || len(key) < len(r.prefix) {
		return fmt.Errorf("key too short or nil")
	}
	trimmed := key[len(r.prefix):]
	return r.w.Put(trimmed, value)
}

// Delete implements the interface KeyValueWriter.
func (r *tableReplayer) Delete(key []byte) error {
	if key == nil || len(key) < len(r.prefix) {
		return fmt.Errorf("key too short or nil")
	}
	trimmed := key[len(r.prefix):]
	return r.w.Delete(trimmed)
}

// Replay replays the batch contents.
func (b *tableBatch) Replay(w KeyValueWriter) error {
	return b.batch.Replay(&tableReplayer{w: w, prefix: b.prefix})
}

// tableIterator is a wrapper around a database iterator that prefixes each key access
// with a pre-configured string.
type tableIterator struct {
	iter   Iterator
	prefix []byte
}

// First moves the iterator to the first key/value pair. It returns whether the
// iterator is exhausted.
func (iter *tableIterator) First() bool {
	return iter.iter.First()
}

// Last moves the iterator to the last key/value pair. It returns whether the
// iterator is exhausted.
func (iter *tableIterator) Last() bool {
	return iter.iter.Last()
}

// Next moves the iterator to the next key/value pair. It returns whether the
// iterator is exhausted.
func (iter *tableIterator) Next() bool {
	return iter.iter.Next()
}

// Prev moves the iterator to the previous key/value pair. It returns whether the
// iterator is exhausted.
func (iter *tableIterator) Prev() bool {
	return iter.iter.Prev()
}

// Error returns any accumulated error. Exhausting all the key/value pairs
// is not considered to be an error.
func (iter *tableIterator) Error() error {
	return iter.iter.Error()
}

// Key returns the key of the current key/value pair, or nil if done. The caller
// should not modify the contents of the returned slice, and its contents may
// change on the next call to Next.
func (iter *tableIterator) Key() []byte {
	key := iter.iter.Key()
	if key == nil || len(key) < len(iter.prefix) {
		return nil
	}
	// Мы уже задали границы в опциях, так что префикс должен совпадать
	return key[len(iter.prefix):]
}

// Value returns the value of the current key/value pair, or nil if done. The
// caller should not modify the contents of the returned slice, and its contents
// may change on the next call to Next.
func (iter *tableIterator) Value() []byte {
	return iter.iter.Value()
}

// Release releases associated resources. Release should always succeed and can
// be called multiple times without causing error.
func (iter *tableIterator) Release() error {
	return iter.iter.Release()
}
