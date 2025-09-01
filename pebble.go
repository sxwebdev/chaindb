package chaindb

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cockroachdb/pebble/v2"
)

const (
	// minCache is the minimum amount of memory in megabytes to allocate to pebble
	// read and write caching, split half and half.
	minCache = 16

	// minHandles is the minimum number of files handles to allocate to the open
	// database files.
	minHandles = 16
)

// Database is a persistent key-value store based on the pebble storage engine.
// Apart from basic data storage functionality it also supports batch writes and
// iterating over the keyspace in binary-alphabetical order.
type pebbleDB struct {
	fn string     // filename for reporting
	db *pebble.DB // Underlying pebble storage engine

	quitLock sync.RWMutex // Mutex protecting the quit channel and the closed flag
	closed   bool         // keep track of whether we're Closed

	activeComp    int           // Current number of active compactions
	compStartTime time.Time     // The start time of the earliest currently-active compaction
	compTime      atomic.Int64  // Total time spent in compaction in ns
	level0Comp    atomic.Uint32 // Total number of level-zero compactions
	nonLevel0Comp atomic.Uint32 // Total number of non level-zero compactions

	writeStalled        atomic.Bool  // Flag whether the write is stalled
	writeDelayStartTime time.Time    // The start time of the latest write stall
	writeDelayReason    string       // The reason of the latest write stall
	writeDelayCount     atomic.Int64 // Total number of write stall counts
	writeDelayTime      atomic.Int64 // Total time spent in write stalls

	writeOptions *pebble.WriteOptions
}

func (d *pebbleDB) onCompactionBegin(info pebble.CompactionInfo) {
	if d.activeComp == 0 {
		d.compStartTime = time.Now()
	}
	l0 := info.Input[0]
	if l0.Level == 0 {
		d.level0Comp.Add(1)
	} else {
		d.nonLevel0Comp.Add(1)
	}
	d.activeComp++
}

func (d *pebbleDB) onCompactionEnd(info pebble.CompactionInfo) {
	switch d.activeComp {
	case 1:
		d.compTime.Add(int64(time.Since(d.compStartTime)))
	case 0:
		panic("should not happen")
	}
	d.activeComp--
}

func (d *pebbleDB) onWriteStallBegin(b pebble.WriteStallBeginInfo) {
	d.writeDelayStartTime = time.Now()
	d.writeDelayCount.Add(1)
	d.writeStalled.Store(true)

	// Take just the first word of the reason. These are two potential
	// reasons for the write stall:
	// - memtable count limit reached
	// - L0 file count limit exceeded
	reason := b.Reason
	if i := strings.IndexByte(reason, ' '); i != -1 {
		reason = reason[:i]
	}
	if reason == "L0" || reason == "memtable" {
		d.writeDelayReason = reason
	}
}

func (d *pebbleDB) onWriteStallEnd() {
	d.writeDelayTime.Add(int64(time.Since(d.writeDelayStartTime)))
	d.writeStalled.Store(false)

	if d.writeDelayReason != "" {
		d.writeDelayReason = ""
	}
	d.writeDelayStartTime = time.Time{}
}

// newPebbleDB returns a wrapped pebble DB object.
func newPebbleDB(dirname string, opts ...Option) (*pebbleDB, error) {
	o := &options{
		cache:    minCache,
		handles:  minHandles,
		readonly: false,
		logger:   pebble.DefaultLogger,
	}

	for _, opt := range opts {
		opt(o)
	}

	// Ensure we have some minimal caching and file guarantees
	if o.cache < minCache {
		o.cache = minCache
	}
	if o.handles < minHandles {
		o.handles = minHandles
	}

	// The max memtable size is limited by the uint32 offsets stored in
	// internal/arenaskl.node, DeferredBatchOp, and flushableBatchEntry.
	//
	// - MaxUint32 on 64-bit platforms;
	// - MaxInt on 32-bit platforms.
	//
	// It is used when slices are limited to Uint32 on 64-bit platforms (the
	// length limit for slices is naturally MaxInt on 32-bit platforms).
	//
	// Taken from https://github.com/cockroachdb/pebble/blob/master/internal/constants/constants.go
	maxMemTableSize := (1<<31)<<(^uint(0)>>63) - 1

	// Two memory tables is configured which is identical to leveldb,
	// including a frozen memory table and another live one.
	memTableLimit := 2
	memTableSize := o.cache * 1024 * 1024 / 2 / memTableLimit

	// The memory table size is currently capped at maxMemTableSize-1 due to a
	// known bug in the pebble where maxMemTableSize is not recognized as a
	// valid size.
	//
	// TODO use the maxMemTableSize as the maximum table size once the issue
	// in pebble is fixed.
	if memTableSize >= maxMemTableSize {
		memTableSize = maxMemTableSize - 1
	}
	db := &pebbleDB{
		fn: dirname,
	}

	if o.noSync {
		// Use asynchronous write mode by default. Otherwise, the overhead of frequent fsync
		// operations can be significant, especially on platforms with slow fsync performance
		// (e.g., macOS) or less capable SSDs.
		//
		// Note that enabling async writes means recent data may be lost in the event of an
		// application-level panic (writes will also be lost on a machine-level failure,
		// of course). Geth is expected to handle recovery from an unclean shutdown.
		db.writeOptions = pebble.NoSync
	} else {
		db.writeOptions = pebble.Sync
	}

	opt := &pebble.Options{
		// Pebble has a single combined cache area and the write
		// buffers are taken from this too. Assign all available
		// memory allowance for cache.
		Cache:        pebble.NewCache(int64(o.cache * 1024 * 1024)),
		MaxOpenFiles: o.handles,

		// The size of memory table(as well as the write buffer).
		// Note, there may have more than two memory tables in the system.
		MemTableSize: uint64(memTableSize),

		// MemTableStopWritesThreshold places a hard limit on the size
		// of the existent MemTables(including the frozen one).
		// Note, this must be the number of tables not the size of all memtables
		// according to https://github.com/cockroachdb/pebble/blob/master/options.go#L738-L742
		// and to https://github.com/cockroachdb/pebble/blob/master/db.go#L1892-L1903.
		MemTableStopWritesThreshold: memTableLimit,

		// Per-level options. Options for at least one level must be specified. The
		// options for the last level are used for all subsequent levels.
		Levels:   o.pebbleLevels,
		ReadOnly: o.readonly,
		EventListener: &pebble.EventListener{
			CompactionBegin: db.onCompactionBegin,
			CompactionEnd:   db.onCompactionEnd,
			WriteStallBegin: db.onWriteStallBegin,
			WriteStallEnd:   db.onWriteStallEnd,
		},
		Logger: o.logger,

		WALBytesPerSync: o.walBytesPerSync,
	}
	// Disable seek compaction explicitly. Check https://github.com/ethereum/go-ethereum/pull/20130
	// for more details.
	opt.Experimental.ReadSamplingMultiplier = -1

	// Open the db and recover any potential corruptions
	innerDB, err := pebble.Open(dirname, opt)
	if err != nil {
		return nil, err
	}
	db.db = innerDB

	return db, nil
}

func (d *pebbleDB) Pebble() *pebble.DB { return d.db }

// Close stops the metrics collection, flushes any pending data to disk and closes
// all io accesses to the underlying key-value store.
func (d *pebbleDB) Close() error {
	d.quitLock.Lock()
	defer d.quitLock.Unlock()
	// Allow double closing, simplifies things
	if d.closed {
		return nil
	}

	if err := d.db.Flush(); err != nil {
		return err
	}

	d.closed = true

	return d.db.Close()
}

// Has retrieves if a key is present in the key-value store.
func (d *pebbleDB) Has(key []byte) (bool, error) {
	if len(key) == 0 {
		return false, ErrEmptyKey
	}

	d.quitLock.RLock()
	defer d.quitLock.RUnlock()
	if d.closed {
		return false, pebble.ErrClosed
	}
	_, closer, err := d.db.Get(key)
	if errors.Is(err, pebble.ErrNotFound) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	if err = closer.Close(); err != nil {
		return false, err
	}
	return true, nil
}

// Get retrieves the given key if it's present in the key-value store.
func (d *pebbleDB) Get(key []byte) ([]byte, error) {
	if len(key) == 0 {
		return nil, ErrEmptyKey
	}

	d.quitLock.RLock()
	defer d.quitLock.RUnlock()
	if d.closed {
		return nil, pebble.ErrClosed
	}
	dat, closer, err := d.db.Get(key)
	if err != nil {
		return nil, err
	}
	ret := make([]byte, len(dat))
	copy(ret, dat)
	if err = closer.Close(); err != nil {
		return nil, err
	}
	return ret, nil
}

// Put inserts the given value into the key-value store.
func (d *pebbleDB) Put(key []byte, value []byte) error {
	if len(key) == 0 {
		return ErrEmptyKey
	}

	d.quitLock.RLock()
	defer d.quitLock.RUnlock()
	if d.closed {
		return pebble.ErrClosed
	}
	return d.db.Set(key, value, d.writeOptions)
}

// Delete removes the key from the key-value store.
func (d *pebbleDB) Delete(key []byte) error {
	if len(key) == 0 {
		return ErrEmptyKey
	}

	d.quitLock.RLock()
	defer d.quitLock.RUnlock()
	if d.closed {
		return pebble.ErrClosed
	}
	return d.db.Delete(key, d.writeOptions)
}

// DeleteRange deletes all of the keys (and values) in the range [start,end)
// (inclusive on start, exclusive on end).
func (d *pebbleDB) DeleteRange(start, end []byte) error {
	d.quitLock.RLock()
	defer d.quitLock.RUnlock()
	if d.closed {
		return pebble.ErrClosed
	}
	return d.db.DeleteRange(start, end, d.writeOptions)
}

// NewBatch creates a write-only key-value store that buffers changes to its host
// database until a final write is called.
func (d *pebbleDB) NewBatch() Batch {
	return &batch{
		b:    d.db.NewBatch(),
		db:   d,
		lock: sync.RWMutex{},
	}
}

// NewBatchWithSize creates a write-only database batch with pre-allocated buffer.
func (d *pebbleDB) NewBatchWithSize(size int) Batch {
	return &batch{
		b:    d.db.NewBatchWithSize(size),
		db:   d,
		lock: sync.RWMutex{},
	}
}

// NewBatchFrom creates a new batch that will write to this store but using the provided batch
func (d *pebbleDB) NewBatchFrom(batch Batch) Batch {
	return batch
}

// Stat returns the internal metrics of Pebble in a text format. It's a developer
// method to read everything there is to read, independent of Pebble version.
func (d *pebbleDB) Stat() (string, error) {
	return d.db.Metrics().String(), nil
}

// Compact flattens the underlying data store for the given key range. In essence,
// deleted and overwritten versions are discarded, and the data is rearranged to
// reduce the cost of operations needed to access them.
//
// A nil start is treated as a key before all keys in the data store; a nil limit
// is treated as a key after all keys in the data store. If both is nil then it
// will compact entire data store.
func (d *pebbleDB) Compact(ctx context.Context, start []byte, limit []byte) error {
	// There is no special flag to represent the end of key range
	// in pebble(nil in leveldb). Use an ugly hack to construct a
	// large key to represent it.
	// Note any prefixed database entry will be smaller than this
	// flag, as for trie nodes we need the 32 byte 0xff because
	// there might be a shared prefix starting with a number of
	// 0xff-s, so 32 ensures than only a hash collision could touch it.
	// https://github.com/cockroachdb/pebble/issues/2359#issuecomment-1443995833
	if limit == nil {
		limit = bytes.Repeat([]byte{0xff}, 32)
	}
	return d.db.Compact(ctx, start, limit, true) // Parallelization is preferred
}

// Path returns the path to the database directory.
func (d *pebbleDB) Path() string {
	return d.fn
}

// SyncKeyValue flushes all pending writes in the write-ahead-log to disk,
// ensuring data durability up to that point.
func (d *pebbleDB) SyncKeyValue() error {
	// The entry (value=nil) is not written to the database; it is only
	// added to the WAL. Writing this special log entry in sync mode
	// automatically flushes all previous writes, ensuring database
	// durability up to this point.
	b := d.db.NewBatch()
	b.LogData(nil, nil)
	return d.db.Apply(b, pebble.Sync)
}

// batch is a write-only batch that commits changes to its host database
// when Write is called. This implementation is thread-safe.
type batch struct {
	b      *pebble.Batch
	db     *pebbleDB
	size   int
	lock   sync.RWMutex
	closed bool
}

// Put inserts the given value into the batch for later committing.
func (b *batch) Put(key, value []byte) error {
	b.lock.Lock()
	defer b.lock.Unlock()

	if err := b.b.Set(key, value, nil); err != nil {
		return err
	}
	b.size += len(key) + len(value)
	return nil
}

// Delete inserts the key removal into the batch for later committing.
func (b *batch) Delete(key []byte) error {
	b.lock.Lock()
	defer b.lock.Unlock()

	if err := b.b.Delete(key, nil); err != nil {
		return err
	}
	return nil
}

// DeleteRange inserts the key range removal into the batch for later committing.
func (b *batch) DeleteRange(start, end []byte) error {
	b.lock.Lock()
	defer b.lock.Unlock()

	if err := b.b.DeleteRange(start, end, nil); err != nil {
		return err
	}
	return nil
}

// ValueSize retrieves the amount of data queued up for writing.
func (b *batch) ValueSize() int {
	b.lock.RLock()
	defer b.lock.RUnlock()

	return b.size
}

// Write flushes any accumulated data to disk.
func (b *batch) Write() error {
	b.lock.Lock()
	defer b.lock.Unlock()

	b.db.quitLock.RLock()
	defer b.db.quitLock.RUnlock()
	if b.db.closed {
		return pebble.ErrClosed
	}
	return b.b.Commit(b.db.writeOptions)
}

// Reset resets the batch for reuse.
func (b *batch) Reset() {
	b.lock.Lock()
	defer b.lock.Unlock()

	b.b.Reset()
	b.size = 0
}

// Close closes the batch and releases any resources associated with it.
func (b *batch) Close() error {
	b.lock.Lock()
	defer b.lock.Unlock()
	if b.b == nil {
		return nil
	}

	if b.closed { // Already closed
		return nil
	}

	if err := b.b.Close(); err != nil {
		return err
	}

	b.closed = true

	return nil
}

// Replay replays the batch contents.
func (b *batch) Replay(w KeyValueWriter) error {
	b.lock.RLock()
	defer b.lock.RUnlock()

	reader := b.b.Reader()
	for {
		kind, k, v, ok, err := reader.Next()
		if !ok || err != nil {
			return err
		}
		// The (k,v) slices might be overwritten if the batch is reset/reused,
		// and the receiver should copy them if they are to be retained long-term.
		switch kind {
		case pebble.InternalKeyKindSet:
			if err = w.Put(k, v); err != nil {
				return err
			}
		case pebble.InternalKeyKindDelete:
			if err = w.Delete(k); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unhandled operation, keytype: %v", kind)
		}
	}
}

// pebbleIterator is a wrapper of underlying iterator in storage engine.
// The purpose of this structure is to implement the missing APIs.
//
// The pebble iterator is not thread-safe.
type pebbleIterator struct {
	iter     *pebble.Iterator
	released bool
}

// NewIterator creates a binary-alphabetical iterator over a subset
// of database content with a particular key prefix, starting at a particular
// initial key (or after, if it does not exist).
func (d *pebbleDB) NewIterator(ctx context.Context, iterOptions *pebble.IterOptions) (Iterator, error) {
	iter, err := d.db.NewIterWithContext(ctx, iterOptions)
	if err != nil {
		return nil, err
	}
	return &pebbleIterator{iter: iter, released: false}, nil
}

// First moves the iterator to the first key/value pair. It returns whether the
// iterator is exhausted.
func (iter *pebbleIterator) First() bool {
	return iter.iter.First()
}

// Last moves the iterator to the last key/value pair. It returns whether the
// iterator is exhausted.
func (iter *pebbleIterator) Last() bool {
	return iter.iter.Last()
}

// Next moves the iterator to the next key/value pair. It returns whether the
// iterator is exhausted.
func (iter *pebbleIterator) Next() bool {
	return iter.iter.Next()
}

// Prev moves the iterator to the previous key/value pair. It returns whether the
// iterator is exhausted.
func (iter *pebbleIterator) Prev() bool {
	return iter.iter.Prev()
}

// Error returns any accumulated error. Exhausting all the key/value pairs
// is not considered to be an error.
func (iter *pebbleIterator) Error() error {
	return iter.iter.Error()
}

// Key returns the key of the current key/value pair, or nil if done. The caller
// should not modify the contents of the returned slice, and its contents may
// change on the next call to Next.
func (iter *pebbleIterator) Key() []byte {
	return iter.iter.Key()
}

// Value returns the value of the current key/value pair, or nil if done. The
// caller should not modify the contents of the returned slice, and its contents
// may change on the next call to Next.
func (iter *pebbleIterator) Value() []byte {
	return iter.iter.Value()
}

// Release releases associated resources. Release should always succeed and can
// be called multiple times without causing error.
func (iter *pebbleIterator) Release() error {
	if iter.released {
		return nil
	}
	if err := iter.iter.Close(); err != nil {
		return err
	}
	iter.released = true
	return nil
}
