package chaindb

import (
	"context"

	"github.com/cockroachdb/pebble/v2"
)

// Iterator iterates over a database's key/value pairs in ascending key order.
//
// When it encounters an error any seek will return false and will yield no key/
// value pairs. The error can be queried by calling the Error method. Calling
// Release is still necessary.
//
// An iterator must be released after use, but it is not necessary to read an
// iterator until exhaustion. An iterator is not safe for concurrent use, but it
// is safe to use multiple iterators concurrently.
type Iterator interface {
	// First moves the iterator to the first key/value pair. It returns whether the
	First() bool

	// Last moves the iterator to the last key/value pair. It returns whether the
	// iterator is exhausted.
	Last() bool

	// Next moves the iterator to the next key/value pair. It returns whether the
	// iterator is exhausted.
	Next() bool

	// Prev moves the iterator to the previous key/value pair. It returns whether the
	// iterator is exhausted.
	Prev() bool

	// Error returns any accumulated error. Exhausting all the key/value pairs
	// is not considered to be an error.
	Error() error

	// Key returns the key of the current key/value pair, or nil if done. The caller
	// should not modify the contents of the returned slice, and its contents may
	// change on the next call to Next.
	Key() []byte

	// Value returns the value of the current key/value pair, or nil if done. The
	// caller should not modify the contents of the returned slice, and its contents
	// may change on the next call to Next.
	Value() []byte

	// Release releases associated resources. Release should always succeed and can
	// be called multiple times without causing error.
	Release() error
}

// Iteratee wraps the NewIterator methods of a backing data store.
type Iteratee interface {
	// NewIterator creates a binary-alphabetical iterator over a subset
	// of database content with a particular key prefix, starting at a particular
	// initial key (or after, if it does not exist).
	//
	// Note: This method assumes that the prefix is NOT part of the start, so there's
	// no need for the caller to prepend the prefix to the start
	NewIterator(ctx context.Context, iterOptions *pebble.IterOptions) (Iterator, error)
}
