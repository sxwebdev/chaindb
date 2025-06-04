package chaindb

// IdealBatchSize defines the size of the data batches should ideally add in one
// write.
const IdealBatchSize = 100 * 1024

// Batch is a write-only database that commits changes to its host database
// when Write is called. Implementations of this interface can be used concurrently.
type Batch interface {
	KeyValueWriter

	// ValueSize retrieves the amount of data queued up for writing.
	ValueSize() int

	// Write flushes any accumulated data to disk.
	Write() error

	// Reset resets the batch for reuse.
	Reset()

	// Close closes the batch and releases any resources it holds.
	Close() error

	// Replay replays the batch contents.
	Replay(w KeyValueWriter) error
}

// Batcher wraps the NewBatch method of a backing data store.
type Batcher interface {
	// NewBatch creates a write-only database that buffers changes to its host db
	// until a final write is called.
	NewBatch() Batch

	// NewBatchWithSize creates a write-only database batch with pre-allocated buffer.
	NewBatchWithSize(size int) Batch

	// NewBatchFrom creates a new batch that will write to this store but using the provided batch
	NewBatchFrom(batch Batch) Batch
}
