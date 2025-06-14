package chaindb

import (
	"github.com/cockroachdb/pebble/v2"
	"github.com/cockroachdb/pebble/v2/bloom"
)

var DefaultPebbleLevels = []pebble.LevelOptions{
	{TargetFileSize: 2 * 1024 * 1024, FilterPolicy: bloom.FilterPolicy(10)},
	{TargetFileSize: 4 * 1024 * 1024, FilterPolicy: bloom.FilterPolicy(10)},
	{TargetFileSize: 8 * 1024 * 1024, FilterPolicy: bloom.FilterPolicy(10)},
	{TargetFileSize: 16 * 1024 * 1024, FilterPolicy: bloom.FilterPolicy(10)},
	{TargetFileSize: 32 * 1024 * 1024, FilterPolicy: bloom.FilterPolicy(10)},
	{TargetFileSize: 64 * 1024 * 1024, FilterPolicy: bloom.FilterPolicy(10)},
	{TargetFileSize: 128 * 1024 * 1024, FilterPolicy: bloom.FilterPolicy(10)},
}

func DefaultPebbleLevelsWithCompression(compressionType pebble.Compression) []pebble.LevelOptions {
	return []pebble.LevelOptions{
		{
			TargetFileSize: 2 * 1024 * 1024,
			FilterPolicy:   bloom.FilterPolicy(10),
			Compression: func() pebble.Compression {
				return compressionType
			},
		},
		{
			TargetFileSize: 4 * 1024 * 1024,
			FilterPolicy:   bloom.FilterPolicy(10),
			Compression: func() pebble.Compression {
				return compressionType
			},
		},
		{
			TargetFileSize: 8 * 1024 * 1024,
			FilterPolicy:   bloom.FilterPolicy(10),
			Compression: func() pebble.Compression {
				return compressionType
			},
		},
		{
			TargetFileSize: 16 * 1024 * 1024,
			FilterPolicy:   bloom.FilterPolicy(10),
			Compression: func() pebble.Compression {
				return compressionType
			},
		},
		{
			TargetFileSize: 32 * 1024 * 1024,
			FilterPolicy:   bloom.FilterPolicy(10),
			Compression: func() pebble.Compression {
				return compressionType
			},
		},
		{
			TargetFileSize: 64 * 1024 * 1024,
			FilterPolicy:   bloom.FilterPolicy(10),
			Compression: func() pebble.Compression {
				return compressionType
			},
		},
		{
			TargetFileSize: 128 * 1024 * 1024,
			FilterPolicy:   bloom.FilterPolicy(10),
			Compression: func() pebble.Compression {
				return compressionType
			},
		},
	}
}
