package chaindb

import "github.com/cockroachdb/pebble/v2"

type options struct {
	cache           int
	handles         int
	readonly        bool
	noSync          bool
	walBytesPerSync int
	pebbleLevels    []pebble.LevelOptions
}

type Option func(*options)

func WithCache(cache int) Option {
	return func(o *options) {
		o.cache = cache
	}
}

func WithHandles(handles int) Option {
	return func(o *options) {
		o.handles = handles
	}
}

func WithReadonly(readonly bool) Option {
	return func(o *options) {
		o.readonly = readonly
	}
}

func WithNoSync(noSync bool) Option {
	return func(o *options) {
		o.noSync = noSync
	}
}

func WithWALBytesPerSync(walBytesPerSync int) Option {
	return func(o *options) {
		o.walBytesPerSync = walBytesPerSync
	}
}

// WithIdealWALBytesPerSync sets the WALBytesPerSync to 5 times the IdealBatchSize.
//
// Pebble is configured to use asynchronous write mode, meaning write operations
// return as soon as the data is cached in memory, without waiting for the WAL
// to be written. This mode offers better write performance but risks losing
// recent writes if the application crashes or a power failure/system crash occurs.
//
// By setting the WALBytesPerSync, the cached WAL writes will be periodically
// flushed at the background if the accumulated size exceeds this threshold.
func WithIdealWALBytesPerSync() Option {
	return func(o *options) {
		o.walBytesPerSync = IdealBatchSize * 5
	}
}

func WithPebbleLevels(levels []pebble.LevelOptions) Option {
	return func(o *options) {
		o.pebbleLevels = levels
	}
}
