package chaindb_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/sxwebdev/chaindb"
	"golang.org/x/sync/errgroup"
)

func TestConcurrentBatchOperations(t *testing.T) {
	// Create a temporary directory for the test database
	dbPath := filepath.Join(os.TempDir(), fmt.Sprintf("chaindb_concurrent_test_%d", time.Now().UnixNano()))
	defer os.RemoveAll(dbPath)

	// Open the database
	db, err := chaindb.NewDatabase(dbPath)
	require.NoError(t, err)
	defer db.Close()

	// Create a table for our test data
	dataTable := chaindb.NewTable(db, []byte("test_data:"))

	// Test parameters
	numWorkers := 8
	totalItems := 1000
	itemsPerWorker := totalItems / numWorkers

	// Create an error group with context
	g, ctx := errgroup.WithContext(context.Background())

	// Test case 1: Each worker has its own batch
	t.Run("EachWorkerOwnBatch", func(t *testing.T) {
		// Launch workers
		for workerID := 0; workerID < numWorkers; workerID++ {
			id := workerID // Capture for goroutine
			g.Go(func() error {
				// Create a new batch for this worker
				batch := dataTable.NewBatch()
				defer batch.Reset()

				// Calculate start and end indices for this worker
				startIdx := id * itemsPerWorker
				endIdx := startIdx + itemsPerWorker
				if id == numWorkers-1 {
					endIdx = totalItems // Ensure we process all items
				}

				// Write items to the batch
				for i := startIdx; i < endIdx; i++ {
					select {
					case <-ctx.Done():
						return ctx.Err()
					default:
						key := []byte(fmt.Sprintf("own_batch_key_%d", i))
						value := []byte(fmt.Sprintf("value_%d", i))
						if err := batch.Put(key, value); err != nil {
							return fmt.Errorf("worker %d failed to put item %d: %v", id, i, err)
						}
					}
				}

				// Commit the batch
				if err := batch.Write(); err != nil {
					return fmt.Errorf("worker %d failed to write batch: %v", id, err)
				}

				return nil
			})
		}

		// Wait for all workers to complete and check for errors
		require.NoError(t, g.Wait())

		// Verify that all data was written correctly
		verifyData(t, dataTable, "own_batch_key_", totalItems)
	})

	// Test case 2: Shared batch with lock
	t.Run("SharedBatchWithLock", func(t *testing.T) {
		// Create a shared batch and mutex
		sharedBatch := dataTable.NewBatch()
		defer sharedBatch.Reset()

		var batchMutex sync.Mutex
		wg := sync.WaitGroup{}
		wg.Add(numWorkers)

		// Launch workers that share the same batch with proper locking
		for workerID := 0; workerID < numWorkers; workerID++ {
			id := workerID // Capture for goroutine
			go func() {
				defer wg.Done()

				// Calculate start and end indices for this worker
				startIdx := id * itemsPerWorker
				endIdx := startIdx + itemsPerWorker
				if id == numWorkers-1 {
					endIdx = totalItems // Ensure we process all items
				}

				// Write items to the shared batch with lock
				for i := startIdx; i < endIdx; i++ {
					key := []byte(fmt.Sprintf("shared_batch_key_%d", i))
					value := []byte(fmt.Sprintf("value_%d", i))

					batchMutex.Lock()
					err := sharedBatch.Put(key, value)
					batchMutex.Unlock()

					assert.NoError(t, err, "worker %d failed to put item %d", id, i)
				}
			}()
		}

		// Wait for all workers to finish adding data
		wg.Wait()

		// Commit the batch
		require.NoError(t, sharedBatch.Write(), "Failed to write shared batch")

		// Verify that all data was written correctly
		verifyData(t, dataTable, "shared_batch_key_", totalItems)
	})

	// Test case 3: Shared batch WITHOUT lock (showing race condition issues)
	t.Run("SharedBatchWithoutLock", func(t *testing.T) {
		// Create a shared batch
		sharedBatch := dataTable.NewBatch()
		defer sharedBatch.Reset()

		// Record initial memory state
		runtime.GC()
		var mBefore runtime.MemStats
		runtime.ReadMemStats(&mBefore)
		t.Logf("Memory usage before shared batch without locks: Alloc = %v MiB, TotalAlloc = %v MiB",
			mBefore.Alloc/1024/1024, mBefore.TotalAlloc/1024/1024)

		var errCount int32
		wg := sync.WaitGroup{}
		wg.Add(numWorkers)

		// Launch workers that share the same batch WITHOUT locking
		for workerID := 0; workerID < numWorkers; workerID++ {
			id := workerID // Capture for goroutine
			go func() {
				defer wg.Done()

				// Calculate start and end indices for this worker
				startIdx := id * itemsPerWorker
				endIdx := startIdx + itemsPerWorker
				if id == numWorkers-1 {
					endIdx = totalItems // Ensure we process all items
				}

				// Write items to the shared batch WITHOUT lock
				for i := startIdx; i < endIdx; i++ {
					key := []byte(fmt.Sprintf("no_lock_key_%d", i))
					value := []byte(fmt.Sprintf("value_%d", i))

					// No lock here, which might cause race conditions
					err := sharedBatch.Put(key, value)
					// We don't assert errors here as the race condition might cause problems
					// that we're intentionally showing in this test case
					if err != nil {
						t.Logf("Race condition detected in worker %d: %v", id, err)
						atomic.AddInt32(&errCount, 1)
					}
				}
			}()
		}

		// Wait for all workers to finish adding data
		wg.Wait()

		// Try to commit the batch - this might fail due to race conditions
		err := sharedBatch.Write()
		if err != nil {
			t.Logf("Expected error when writing batch with race conditions: %v", err)
		} else {
			// If it somehow succeeded, verify the data
			// Note: We can't rely on the count here due to race conditions
			t.Log("Warning: Batch without locks succeeded writing. This doesn't mean it's safe!")

			// Check if we had any errors during Put operations
			if atomic.LoadInt32(&errCount) > 0 {
				t.Logf("Detected %d errors during concurrent Put operations without locks", errCount)
			} else {
				t.Log("No errors detected during concurrent operations, but race conditions may still exist")
			}

			// Verify the data was written - we expect all items to be there if Write() succeeded
			verifyData(t, dataTable, "no_lock_key_", totalItems)
		}

		// Force garbage collection to clean up any unreferenced objects
		runtime.GC()

		// Check for memory leaks after batch operations
		var mAfter runtime.MemStats
		runtime.ReadMemStats(&mAfter)

		t.Logf("Memory usage after shared batch without locks: Alloc = %v MiB, TotalAlloc = %v MiB",
			mAfter.Alloc/1024/1024, mAfter.TotalAlloc/1024/1024)

		// Calculate memory change in a safe way that avoids overflow
		var memChange int64
		if mAfter.Alloc > mBefore.Alloc {
			memChange = int64(mAfter.Alloc-mBefore.Alloc) / 1024 / 1024
		} else {
			memChange = -int64(mBefore.Alloc-mAfter.Alloc) / 1024 / 1024
		}
		t.Logf("Memory change: %v MiB", memChange)
	})

	// Test case 4: Multiple batches written concurrently
	t.Run("MultipleBatchesConcurrently", func(t *testing.T) {
		g, ctx := errgroup.WithContext(context.Background())

		// Number of batches (each batch will contain itemsPerBatch items)
		numBatches := numWorkers
		itemsPerBatch := totalItems / numBatches

		// Launch a goroutine for each batch
		for batchID := 0; batchID < numBatches; batchID++ {
			id := batchID // Capture for goroutine
			g.Go(func() error {
				// Create a new batch
				batch := dataTable.NewBatch()
				defer batch.Reset()

				// Calculate start and end indices for this batch
				startIdx := id * itemsPerBatch
				endIdx := startIdx + itemsPerBatch
				if id == numBatches-1 {
					endIdx = totalItems // Ensure we process all items
				}

				// Add items to batch
				for i := startIdx; i < endIdx; i++ {
					select {
					case <-ctx.Done():
						return ctx.Err()
					default:
						key := []byte(fmt.Sprintf("multi_batch_key_%d", i))
						value := []byte(fmt.Sprintf("value_%d", i))
						if err := batch.Put(key, value); err != nil {
							return fmt.Errorf("batch %d failed to put item %d: %v", id, i, err)
						}
					}
				}

				// Write the batch
				if err := batch.Write(); err != nil {
					return fmt.Errorf("batch %d failed to write: %v", id, err)
				}

				return nil
			})
		}

		// Wait for all batches to be written
		require.NoError(t, g.Wait())

		// Verify that all data was written correctly
		verifyData(t, dataTable, "multi_batch_key_", totalItems)
	})

	// Test case 5: High contention batch operations
	t.Run("HighContentionBatch", func(t *testing.T) {
		// Create a new table for this test to avoid interference with other tests
		highContentionTable := chaindb.NewTable(db, []byte("high_contention:"))

		// Higher number of workers for more contention
		highWorkers := numWorkers * 4  // 32 workers
		highOperationsPerWorker := 100 // Each worker does fewer operations but more frequently

		// Create shared batch and mutexes for comparison
		sharedBatchWithLock := highContentionTable.NewBatch()
		sharedBatchWithoutLock := highContentionTable.NewBatch()
		defer sharedBatchWithLock.Reset()
		defer sharedBatchWithoutLock.Reset()

		var batchMutex sync.Mutex
		var errCountWithLock int32
		var errCountWithoutLock int32

		// Record initial memory state
		runtime.GC()
		var mBefore runtime.MemStats
		runtime.ReadMemStats(&mBefore)
		t.Logf("Memory usage before high contention test: Alloc = %v MiB, TotalAlloc = %v MiB",
			mBefore.Alloc/1024/1024, mBefore.TotalAlloc/1024/1024)

		t.Log("Starting high contention test with and without locks...")

		// WaitGroups to coordinate the workers
		var wgWithLock, wgWithoutLock sync.WaitGroup
		wgWithLock.Add(highWorkers)
		wgWithoutLock.Add(highWorkers)

		// Start time for performance measurement
		startWithLock := time.Now()

		// Launch workers with lock
		for workerID := 0; workerID < highWorkers; workerID++ {
			id := workerID
			go func() {
				defer wgWithLock.Done()

				startIdx := id * highOperationsPerWorker
				endIdx := startIdx + highOperationsPerWorker

				for i := startIdx; i < endIdx; i++ {
					key := []byte(fmt.Sprintf("high_contention_lock_key_%d", i))
					value := []byte(fmt.Sprintf("value_%d", i))

					batchMutex.Lock()
					err := sharedBatchWithLock.Put(key, value)
					batchMutex.Unlock()

					if err != nil {
						t.Logf("Worker %d with lock: error putting key %s: %v", id, string(key), err)
						atomic.AddInt32(&errCountWithLock, 1)
					}

					// Add a small sleep to simulate some processing between operations
					time.Sleep(time.Microsecond)
				}
			}()
		}

		// Wait for locked workers to complete
		wgWithLock.Wait()
		durationWithLock := time.Since(startWithLock)

		// Write the batch with lock
		errWithLock := sharedBatchWithLock.Write()
		if errWithLock != nil {
			t.Logf("Error writing batch with lock: %v", errWithLock)
		} else {
			t.Logf("Successfully wrote batch with lock in %v", durationWithLock)
			if atomic.LoadInt32(&errCountWithLock) > 0 {
				t.Logf("Detected %d errors during Put operations WITH locks", errCountWithLock)
			}
		}

		// Start time for unlocked operations
		startWithoutLock := time.Now()

		// Launch workers without lock
		for workerID := 0; workerID < highWorkers; workerID++ {
			id := workerID
			go func() {
				defer wgWithoutLock.Done()

				startIdx := id * highOperationsPerWorker
				endIdx := startIdx + highOperationsPerWorker

				for i := startIdx; i < endIdx; i++ {
					key := []byte(fmt.Sprintf("high_contention_no_lock_key_%d", i))
					value := []byte(fmt.Sprintf("value_%d", i))

					// No lock here - potential for race conditions
					err := sharedBatchWithoutLock.Put(key, value)
					if err != nil {
						t.Logf("Worker %d without lock: error putting key %s: %v", id, string(key), err)
						atomic.AddInt32(&errCountWithoutLock, 1)
					}

					// Add a small sleep to simulate some processing between operations
					time.Sleep(time.Microsecond)
				}
			}()
		}

		// Wait for unlocked workers to complete
		wgWithoutLock.Wait()
		durationWithoutLock := time.Since(startWithoutLock)

		// Try to write the batch without lock
		errWithoutLock := sharedBatchWithoutLock.Write()
		if errWithoutLock != nil {
			t.Logf("Error writing batch without lock: %v", errWithoutLock)
		} else {
			t.Logf("Successfully wrote batch without lock in %v", durationWithoutLock)
			if atomic.LoadInt32(&errCountWithoutLock) > 0 {
				t.Logf("Detected %d errors during Put operations WITHOUT locks", errCountWithoutLock)
			}
		}

		// Performance comparison
		t.Logf("Performance comparison - With Lock: %v, Without Lock: %v",
			durationWithLock, durationWithoutLock)

		// Verify the data if writes succeeded
		if errWithLock == nil {
			verifyData(t, highContentionTable, "high_contention_lock_key_", highWorkers*highOperationsPerWorker)
		}

		if errWithoutLock == nil {
			t.Log("Warning: Batch without locks succeeded, but this doesn't mean it's safe or correct!")
			// Try to verify the data, but the count might be wrong due to race conditions
			count := countItems(t, highContentionTable, "high_contention_no_lock_key_")
			t.Logf("Found %d items with prefix 'high_contention_no_lock_key_', expected %d",
				count, highWorkers*highOperationsPerWorker)
		}

		// Check for memory leaks
		runtime.GC()
		var mAfter runtime.MemStats
		runtime.ReadMemStats(&mAfter)

		t.Logf("Memory usage after high contention test: Alloc = %v MiB, TotalAlloc = %v MiB",
			mAfter.Alloc/1024/1024, mAfter.TotalAlloc/1024/1024)

		// Calculate memory change
		var memChange int64
		if mAfter.Alloc > mBefore.Alloc {
			memChange = int64(mAfter.Alloc-mBefore.Alloc) / 1024 / 1024
		} else {
			memChange = -int64(mBefore.Alloc-mAfter.Alloc) / 1024 / 1024
		}
		t.Logf("Memory change: %v MiB", memChange)
	})
}

// Helper function to verify that all expected data exists in the database
func verifyData(t *testing.T, table chaindb.Table, keyPrefix string, expectedCount int) {
	t.Helper()

	// Create an iterator to scan all data
	iter, err := table.NewIterator(context.Background(), nil)
	require.NoError(t, err, "Failed to create iterator")
	defer iter.Release()

	count := 0
	// Use the correct iterator pattern
	for valid := iter.First(); valid && iter.Error() == nil; valid = iter.Next() {
		key := iter.Key()
		value := iter.Value()

		if key == nil || value == nil {
			continue
		}

		keyStr := string(key)
		// Check if the key starts with our prefix
		if len(keyPrefix) > 0 && len(keyStr) >= len(keyPrefix) && keyStr[:len(keyPrefix)] == keyPrefix {
			count++
			// Extract the index from the key (format: "prefix_N")
			var idx int
			_, err := fmt.Sscanf(keyStr, keyPrefix+"%d", &idx)
			require.NoError(t, err, "Failed to parse key index")

			// Check value format
			expectedValue := fmt.Sprintf("value_%d", idx)
			assert.Equal(t, expectedValue, string(value), "Value mismatch for key %s", keyStr)
		}
	}

	require.NoError(t, iter.Error(), "Iterator error")
	assert.Equal(t, expectedCount, count, "Expected to find %d items with prefix %s, but found %d",
		expectedCount, keyPrefix, count)
}

// Helper function to count items with a specific key prefix
func countItems(t *testing.T, table chaindb.Table, keyPrefix string) int {
	t.Helper()

	// Create an iterator to scan all data
	iter, err := table.NewIterator(context.Background(), nil)
	require.NoError(t, err, "Failed to create iterator")
	defer iter.Release()

	count := 0
	// Use the correct iterator pattern
	for valid := iter.First(); valid && iter.Error() == nil; valid = iter.Next() {
		key := iter.Key()

		if key == nil {
			continue
		}

		keyStr := string(key)
		// Check if the key starts with our prefix
		if len(keyPrefix) > 0 && len(keyStr) >= len(keyPrefix) && keyStr[:len(keyPrefix)] == keyPrefix {
			count++
		}
	}

	require.NoError(t, iter.Error(), "Iterator error")
	return count
}
