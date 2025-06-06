package chaindb_test

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"testing"

	"github.com/cockroachdb/pebble/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/sxwebdev/chaindb"
)

type testDB struct {
	db   chaindb.Database
	path string
}

// close
func (t *testDB) Close() {
	if t.db != nil {
		if err := t.db.Close(); err != nil {
			log.Printf("Failed to close database: %v", err)
		}
	}

	if err := os.RemoveAll(t.path); err != nil {
		log.Printf("Failed to remove database path: %v", err)
	}
}

func NewMemDB() (*testDB, error) {
	// Create a temporary directory for the database
	dbPath := filepath.Join(os.TempDir(), "chaindb_test")

	// Open the database
	db, err := chaindb.NewDatabase(dbPath)
	if err != nil {
		return nil, err
	}

	return &testDB{
		db:   db,
		path: dbPath,
	}, nil
}

func TestTable_Prefix(t *testing.T) {
	instance, err := NewMemDB()
	defer instance.Close()
	require.NoError(t, err)

	prefix := []byte("test")
	table := chaindb.NewTable(instance.db, prefix)

	assert.Equal(t, prefix, table.Prefix())
}

func TestTable_BasicOperations(t *testing.T) {
	instance, err := NewMemDB()
	defer instance.Close()
	require.NoError(t, err)

	prefix := []byte("prefix:")
	table := chaindb.NewTable(instance.db, prefix)

	// Test Put and Get
	key := []byte("key")
	value := []byte("value")

	err = table.Put(key, value)
	require.NoError(t, err)

	// Check if we can get it back from the table
	gotValue, err := table.Get(key)
	require.NoError(t, err)
	assert.Equal(t, value, gotValue)

	// Check if the key has the prefix in the underlying DB
	prefixedKey := append(prefix, key...)
	exists, err := instance.db.Has(prefixedKey)
	require.NoError(t, err)
	assert.True(t, exists)

	// Test Has
	exists, err = table.Has(key)
	require.NoError(t, err)
	assert.True(t, exists)

	nonExistentKey := []byte("non-existent")
	exists, err = table.Has(nonExistentKey)
	require.NoError(t, err)
	assert.False(t, exists)

	// Test Delete
	err = table.Delete(key)
	require.NoError(t, err)

	exists, err = table.Has(key)
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestTable_DeleteRange(t *testing.T) {
	instance, err := NewMemDB()
	defer instance.Close()
	require.NoError(t, err)

	prefix := []byte("prefix:")
	table := chaindb.NewTable(instance.db, prefix)

	// Insert several keys
	for i := 0; i < 10; i++ {
		key := []byte{byte(i)}
		value := []byte{byte(i * 10)}
		err := table.Put(key, value)
		require.NoError(t, err)
	}

	// Delete range [3, 7)
	err = table.DeleteRange([]byte{3}, []byte{7})
	require.NoError(t, err)

	// Verify keys 0-2 and 7-9 exist, but 3-6 don't
	for i := 0; i < 10; i++ {
		key := []byte{byte(i)}
		exists, err := table.Has(key)
		require.NoError(t, err)

		if i < 3 || i >= 7 {
			assert.True(t, exists, "Key %d should exist", i)
		} else {
			assert.False(t, exists, "Key %d shouldn't exist", i)
		}
	}
}

func TestTable_Iterator(t *testing.T) {
	instance, err := NewMemDB()
	defer instance.Close()
	require.NoError(t, err)

	prefix := []byte("prefix:")
	table := chaindb.NewTable(instance.db, prefix)

	// Insert some data
	testData := map[string]string{
		"a": "value-a",
		"b": "value-b",
		"c": "value-c",
		"d": "value-d",
	}

	for k, v := range testData {
		err := table.Put([]byte(k), []byte(v))
		require.NoError(t, err)
	}

	// Also insert some data with different prefixes to ensure isolation
	err = instance.db.Put([]byte("other:x"), []byte("should-not-see"))
	require.NoError(t, err)

	// Test iterator with no bounds
	iter, err := table.NewIterator(context.Background(), nil)
	require.NoError(t, err)
	defer iter.Release()

	count := 0
	for valid := iter.First(); valid && iter.Error() == nil; valid = iter.Next() {
		key := string(iter.Key())
		value := string(iter.Value())

		expectedValue, exists := testData[key]
		assert.True(t, exists, "Unexpected key: %s", key)
		assert.Equal(t, expectedValue, value)
		count++
	}
	assert.Equal(t, len(testData), count)

	// Test iterator with bounds
	iterOptions := &pebble.IterOptions{
		LowerBound: []byte("b"),
		UpperBound: []byte("d"),
	}

	boundedIter, err := table.NewIterator(context.Background(), iterOptions)
	require.NoError(t, err)
	defer boundedIter.Release()

	expectedKeys := []string{"b", "c"}
	actualKeys := []string{}

	for valid := boundedIter.First(); valid && boundedIter.Error() == nil; valid = boundedIter.Next() {
		actualKeys = append(actualKeys, string(boundedIter.Key()))
	}

	assert.Equal(t, expectedKeys, actualKeys)
}

func TestTable_Batch(t *testing.T) {
	instance, err := NewMemDB()
	defer instance.Close()
	require.NoError(t, err)

	prefix := []byte("prefix:")
	table := chaindb.NewTable(instance.db, prefix)

	batch := table.NewBatch()

	// Add operations to batch
	keys := [][]byte{[]byte("key1"), []byte("key2"), []byte("key3")}
	values := [][]byte{[]byte("value1"), []byte("value2"), []byte("value3")}

	for i := range keys {
		err := batch.Put(keys[i], values[i])
		require.NoError(t, err)
	}

	// Check batch size
	assert.Greater(t, batch.ValueSize(), 0)

	// Write batch
	err = batch.Write()
	require.NoError(t, err)

	// Verify keys were written
	for i := range keys {
		val, err := table.Get(keys[i])
		require.NoError(t, err)
		assert.Equal(t, values[i], val)

		// Also verify keys were prefixed correctly in the underlying DB
		prefixedKey := append(prefix, keys[i]...)
		dbVal, err := instance.db.Get(prefixedKey)
		require.NoError(t, err)
		assert.Equal(t, values[i], dbVal)
	}

	// Test batch reset
	batch.Reset()
	assert.Equal(t, 0, batch.ValueSize())

	// Test batch delete
	batch = table.NewBatch()
	err = batch.Delete(keys[0])
	require.NoError(t, err)
	err = batch.Write()
	require.NoError(t, err)

	exists, err := table.Has(keys[0])
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestTableBatch_Replay(t *testing.T) {
	instance, err := NewMemDB()
	defer instance.Close()
	require.NoError(t, err)
	prefix := []byte("prefix:")
	table := chaindb.NewTable(instance.db, prefix)

	// Create a batch with operations
	batch := table.NewBatch()
	err = batch.Put([]byte("key1"), []byte("value1"))
	require.NoError(t, err)
	err = batch.Put([]byte("key2"), []byte("value2"))
	require.NoError(t, err)
	err = batch.Delete([]byte("key3"))
	require.NoError(t, err)

	// Create a mock writer to replay into
	mockWriter := &mockKeyValueWriter{}

	// Replay the batch
	err = batch.Replay(mockWriter)
	require.NoError(t, err)

	// Verify the mock writer received the correct calls
	assert.Len(t, mockWriter.puts, 2)
	assert.Equal(t, []byte("key1"), mockWriter.puts[0].key)
	assert.Equal(t, []byte("value1"), mockWriter.puts[0].value)
	assert.Equal(t, []byte("key2"), mockWriter.puts[1].key)
	assert.Equal(t, []byte("value2"), mockWriter.puts[1].value)

	assert.Len(t, mockWriter.deletes, 1)
	assert.Equal(t, []byte("key3"), mockWriter.deletes[0])
}

// Helper mock for testing Replay functionality
type mockKeyValueWriter struct {
	puts    []struct{ key, value []byte }
	deletes [][]byte
}

func (m *mockKeyValueWriter) Put(key, value []byte) error {
	m.puts = append(m.puts, struct{ key, value []byte }{
		key:   append([]byte{}, key...),   // Make a copy to avoid later modification
		value: append([]byte{}, value...), // Make a copy to avoid later modification
	})
	return nil
}

func (m *mockKeyValueWriter) Delete(key []byte) error {
	m.deletes = append(m.deletes, append([]byte{}, key...)) // Make a copy
	return nil
}

func TestTableIterator(t *testing.T) {
	instance, err := NewMemDB()
	defer instance.Close()
	require.NoError(t, err)

	prefix := []byte("prefix:")
	table := chaindb.NewTable(instance.db, prefix)

	// Insert data
	data := []struct {
		key   string
		value string
	}{
		{"a", "value-a"},
		{"b", "value-b"},
		{"c", "value-c"},
		{"d", "value-d"},
		{"aa", "value-aa"},
		{"aaa", "value-aaa"},
	}

	for _, item := range data {
		err := table.Put([]byte(item.key), []byte(item.value))
		require.NoError(t, err)
	}

	iter, err := table.NewIterator(context.Background(), nil)
	require.NoError(t, err)
	defer iter.Release()

	// Test First
	assert.True(t, iter.First())
	assert.Equal(t, "a", string(iter.Key()))
	assert.Equal(t, "value-a", string(iter.Value()))

	// Test Next
	assert.True(t, iter.Next())
	assert.Equal(t, "aa", string(iter.Key()))

	// Test Next
	assert.True(t, iter.Next())
	assert.Equal(t, "aaa", string(iter.Key()))

	// Test Next
	assert.True(t, iter.Next())
	assert.Equal(t, "b", string(iter.Key()))

	assert.True(t, iter.Next())
	assert.Equal(t, "c", string(iter.Key()))

	// Test Last
	assert.True(t, iter.Last())
	assert.Equal(t, "d", string(iter.Key()))

	// Test Prev
	assert.True(t, iter.Prev())
	assert.Equal(t, "c", string(iter.Key()))

	iterOpts := &pebble.IterOptions{
		LowerBound: []byte("a"),
		UpperBound: []byte("b"),
	}

	iter2, err := table.NewIterator(context.Background(), iterOpts)
	require.NoError(t, err)
	defer iter2.Release()

	count := 0
	for valid := iter2.First(); valid && iter2.Error() == nil; valid = iter2.Next() {
		count++
	}

	assert.Equal(t, 3, count)
}

func TestTable_BatchWithSize(t *testing.T) {
	instance, err := NewMemDB()
	defer instance.Close()
	require.NoError(t, err)
	prefix := []byte("prefix:")
	table := chaindb.NewTable(instance.db, prefix)

	// Create batch with predefined size
	batch := table.NewBatchWithSize(1024)

	// Add operations
	err = batch.Put([]byte("key1"), []byte("value1"))
	require.NoError(t, err)

	// Write batch
	err = batch.Write()
	require.NoError(t, err)

	// Verify key was written
	val, err := table.Get([]byte("key1"))
	require.NoError(t, err)
	assert.Equal(t, []byte("value1"), val)
}

func TestTable_NewBatchFrom(t *testing.T) {
	instance, err := NewMemDB()
	defer instance.Close()
	require.NoError(t, err)
	prefix := []byte("prefix:")
	table := chaindb.NewTable(instance.db, prefix)

	// Create original batch
	originalBatch := instance.db.NewBatch()

	// Create table batch from original batch
	tableBatch := table.NewBatchFrom(originalBatch)

	// Add operations to table batch (should go into original batch with prefix)
	err = tableBatch.Put([]byte("key1"), []byte("value1"))
	require.NoError(t, err)

	// Write the batch
	err = tableBatch.Write()
	require.NoError(t, err)

	// Verify key was written with prefix
	val, err := instance.db.Get(append(prefix, []byte("key1")...))
	require.NoError(t, err)
	assert.Equal(t, []byte("value1"), val)

	// And also accessible through the table
	val, err = table.Get([]byte("key1"))
	require.NoError(t, err)
	assert.Equal(t, []byte("value1"), val)
}

func TestUpperBound(t *testing.T) {
	// This is to test the private upperBound function indirectly
	instance, err := NewMemDB()
	defer instance.Close()
	require.NoError(t, err)

	prefix := []byte{0x01, 0xFF, 0xFF}
	table := chaindb.NewTable(instance.db, prefix)

	// Put some keys
	err = table.Put([]byte("key"), []byte("value"))
	require.NoError(t, err)

	// Put a key that would be outside the upper bound
	outsideKey := []byte{0x02, 0x00, 0x00}
	err = instance.db.Put(outsideKey, []byte("outside"))
	require.NoError(t, err)

	// Create iterator and verify it only sees keys within the prefix
	iter, err := table.NewIterator(context.Background(), nil)
	require.NoError(t, err)
	defer iter.Release()

	count := 0
	for valid := iter.First(); valid && iter.Error() == nil; valid = iter.Next() {
		count++
		assert.Equal(t, "key", string(iter.Key()))
	}
	assert.Equal(t, 1, count)
}

func TestSimpleIterator(t *testing.T) {
	// Create a new test database
	db, err := chaindb.NewDatabase(t.TempDir())
	require.NoError(t, err)
	defer db.Close()

	// Put a few keys in the database
	err = db.Put([]byte("key1"), []byte("value1"))
	require.NoError(t, err)
	err = db.Put([]byte("key2"), []byte("value2"))
	require.NoError(t, err)

	// Create an iterator
	iter, err := db.NewIterator(context.Background(), nil)
	require.NoError(t, err)
	defer iter.Release()

	// Count the keys by iterating
	keys := []string{}

	// Approach 1: Simple iteration
	ok := iter.First()
	for ; ok; ok = iter.Next() {
		keys = append(keys, string(iter.Key()))
		// Safety check
		if len(keys) > 10 {
			t.Fatal("Too many iterations, possible infinite loop")
		}
	}

	// Verify results
	assert.Len(t, keys, 2, "Should find exactly 2 keys")
	assert.Contains(t, keys, "key1")
	assert.Contains(t, keys, "key2")
}

func TestUpperBoundFunction(t *testing.T) {
	testCases := []struct {
		prefix      []byte
		expected    []byte
		description string
	}{
		{
			prefix:      []byte{0x01, 0xFF, 0xFF},
			expected:    []byte{0x02},
			description: "Test with prefix [0x01, 0xFF, 0xFF]",
		},
		{
			prefix:      []byte{0x01, 0x02},
			expected:    []byte{0x01, 0x03},
			description: "Test with prefix [0x01, 0x02]",
		},
		{
			prefix:      []byte{0xFF, 0xFF},
			expected:    nil,
			description: "Test with all 0xFF bytes",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			result := chaindb.UpperBound(tc.prefix)
			// Debug output
			fmt.Printf("Prefix: %v, Result: %v\n", tc.prefix, result)
			assert.Equal(t, tc.expected, result)
		})
	}
}
