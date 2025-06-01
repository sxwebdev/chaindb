package chaindb

// NewDatabase creates a new database instance.
//
// The database is created using the pebble storage engine.
//
// The database is created with the given logger, file, cache, handles and readonly flags.
func NewDatabase(logger Logger, file string, cache int, handles int, readonly bool) (Database, error) {
	return newPebbleDB(logger, file, cache, handles, readonly)
}
