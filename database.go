package chaindb

import (
	"context"
	"errors"
	"io"

	"github.com/cockroachdb/pebble/v2"
)

// KeyValueReader wraps the Has and Get method of a backing data store.
type KeyValueReader interface {
	// Has retrieves if a key is present in the key-value data store.
	Has(key []byte) (bool, error)

	// Get retrieves the given key if it's present in the key-value data store.
	Get(key []byte) ([]byte, error)
}

// KeyValueWriter wraps the Put method of a backing data store.
type KeyValueWriter interface {
	// Put inserts the given value into the key-value data store.
	Put(key []byte, value []byte) error

	// Delete removes the key from the key-value data store.
	Delete(key []byte) error
}

var ErrTooManyKeys = errors.New("too many keys in deleted range")

// KeyValueRangeDeleter wraps the DeleteRange method of a backing data store.
type KeyValueRangeDeleter interface {
	// DeleteRange deletes all of the keys (and values) in the range [start,end)
	// (inclusive on start, exclusive on end).
	// Some implementations of DeleteRange may return ErrTooManyKeys after
	// partially deleting entries in the given range.
	DeleteRange(start, end []byte) error
}

// KeyValueStater wraps the Stat method of a backing data store.
type KeyValueStater interface {
	// Stat returns the statistic data of the database.
	Stat() (string, error)
}

// KeyValueSyncer wraps the SyncKeyValue method of a backing data store.
type KeyValueSyncer interface {
	// SyncKeyValue ensures that all pending writes are flushed to disk,
	// guaranteeing data durability up to the point.
	SyncKeyValue() error
}

// Compacter wraps the Compact method of a backing data store.
type Compacter interface {
	// Compact flattens the underlying data store for the given key range. In essence,
	// deleted and overwritten versions are discarded, and the data is rearranged to
	// reduce the cost of operations needed to access them.
	//
	// A nil start is treated as a key before all keys in the data store; a nil limit
	// is treated as a key after all keys in the data store. If both is nil then it
	// will compact entire data store.
	Compact(ctx context.Context, start []byte, limit []byte) error
}

// Database contains all the methods required to allow handling different
// key-value data stores backing the high level database.
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

// NewDatabase creates a new database instance.
//
// The database is created using the pebble storage engine.
//
// The database is created with the given directory name and options.
func NewDatabase(dirname string, opts ...Option) (Database, error) {
	return newPebbleDB(dirname, opts...)
}
