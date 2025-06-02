package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/sxwebdev/chaindb"
)

func main() {
	// Create a temporary directory for the database
	dbPath := filepath.Join(os.TempDir(), "chaindb_concurrent_batch_test")
	defer os.RemoveAll(dbPath)

	// Open the database
	db, err := chaindb.NewDatabase(dbPath)
	if err != nil {
		log.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Create a table for our data
	dataTable := chaindb.NewTable(db, []byte("data:"))

	// Number of concurrent workers and total items
	numWorkers := 10
	totalItems := 1000
	itemsPerWorker := totalItems / numWorkers

	fmt.Printf("Starting concurrent batch operations with %d workers and %d total items...\n", numWorkers, totalItems)

	// Create a wait group to wait for all workers to complete
	var wg sync.WaitGroup
	wg.Add(numWorkers)

	// Create a channel to collect errors from workers
	errChan := make(chan error, numWorkers)

	// Start time for performance measurement
	startTime := time.Now()

	// Launch workers
	for workerID := 0; workerID < numWorkers; workerID++ {
		go func(id int) {
			defer wg.Done()

			// Create a new batch for this worker
			batch := db.NewBatch()
			defer batch.Reset()

			// Create a table batch that uses the same underlying batch
			tableBatch := dataTable.NewBatchFrom(batch)

			// Calculate start and end indices for this worker
			startIdx := id * itemsPerWorker
			endIdx := startIdx + itemsPerWorker
			if id == numWorkers-1 {
				endIdx = totalItems // Ensure we process all items
			}

			// Write items to the batch
			for i := startIdx; i < endIdx; i++ {
				key := []byte(fmt.Sprintf("item_%d", i))
				value := []byte(fmt.Sprintf("value_%d", i))
				tableBatch.Put(key, value)
			}

			// Commit the batch
			if err := batch.Write(); err != nil {
				errChan <- fmt.Errorf("worker %d failed to write batch: %v", id, err)
				return
			}
		}(workerID)
	}

	// Wait for all workers to complete
	wg.Wait()
	close(errChan)

	// Check for any errors
	for err := range errChan {
		if err != nil {
			log.Fatalf("Error during batch operations: %v", err)
		}
	}

	// Calculate and print performance metrics
	duration := time.Since(startTime)
	fmt.Printf("\nBatch operations completed in %v\n", duration)
	fmt.Printf("Average throughput: %.2f operations/second\n", float64(totalItems)/duration.Seconds())

	// Verify the data
	fmt.Println("\nVerifying data...")
	verified := 0
	iter := dataTable.NewIterator(nil, nil)
	defer iter.Release()

	for iter.Next() {
		verified++
	}
	iter.Release()

	if verified != totalItems {
		log.Fatalf("Data verification failed: expected %d items, found %d", totalItems, verified)
	}

	fmt.Printf("Successfully verified %d items\n", verified)
}
