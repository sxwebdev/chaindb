package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/sxwebdev/chaindb"
)

func main() {
	// Create a temporary directory for the database
	dbPath := filepath.Join(os.TempDir(), "chaindb_batch_test")
	defer os.RemoveAll(dbPath)

	// Open the database
	db, err := chaindb.NewDatabase(dbPath)
	if err != nil {
		log.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Create tables for different data types
	usersTable := chaindb.NewTable(db, "users:")
	settingsTable := chaindb.NewTable(db, "settings:")
	logsTable := chaindb.NewTable(db, "logs:")

	fmt.Println("Starting batch operations...")

	// Create a single batch for all operations
	batch := db.NewBatch()
	defer batch.Reset()

	// Create table batches that use the same underlying batch
	usersBatch := usersTable.NewBatchFrom(batch)
	settingsBatch := settingsTable.NewBatchFrom(batch)
	logsBatch := logsTable.NewBatchFrom(batch)

	// Write to users table
	fmt.Println("\nWriting user data...")
	usersBatch.Put([]byte("user1"), []byte("John Doe"))
	usersBatch.Put([]byte("user2"), []byte("Alice Smith"))

	// Write to settings table
	fmt.Println("Writing user settings...")
	settingsBatch.Put([]byte("user1:theme"), []byte("dark"))
	settingsBatch.Put([]byte("user2:theme"), []byte("light"))
	settingsBatch.Put([]byte("user1:language"), []byte("en"))
	settingsBatch.Put([]byte("user2:language"), []byte("fr"))

	// Write to logs table
	fmt.Println("Writing user logs...")
	logsBatch.Put([]byte("user1:login"), []byte("2024-03-20"))
	logsBatch.Put([]byte("user2:login"), []byte("2024-03-21"))
	logsBatch.Put([]byte("user1:action"), []byte("profile_update"))
	logsBatch.Put([]byte("user2:action"), []byte("settings_change"))

	// Commit all changes atomically
	fmt.Println("\nCommitting batch...")
	if err := batch.Write(); err != nil {
		log.Fatalf("Failed to write batch: %v", err)
	}

	// Verify the data
	fmt.Println("\nVerifying data...")

	// Check users
	user1, err := usersTable.Get([]byte("user1"))
	if err != nil {
		log.Fatalf("Failed to get user1: %v", err)
	}
	fmt.Printf("User1: %s\n", string(user1))

	// Check settings
	theme1, err := settingsTable.Get([]byte("user1:theme"))
	if err != nil {
		log.Fatalf("Failed to get user1 theme: %v", err)
	}
	fmt.Printf("User1 theme: %s\n", string(theme1))

	// Check logs
	log1, err := logsTable.Get([]byte("user1:login"))
	if err != nil {
		log.Fatalf("Failed to get user1 login: %v", err)
	}
	fmt.Printf("User1 last login: %s\n", string(log1))

	// Iterate over all users
	fmt.Println("\nAll users:")
	iter := usersTable.NewIterator(nil, nil)
	defer iter.Release()

	for iter.Next() {
		key := string(iter.Key())
		value := string(iter.Value())
		fmt.Printf("  %s: %s\n", key, value)
	}

	fmt.Println("\nBatch operations completed successfully")

	os.Exit(0)
}
