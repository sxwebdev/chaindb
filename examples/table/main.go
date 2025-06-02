package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/sxwebdev/chaindb"
)

// User represents a simple user structure
type User struct {
	ID       string
	Name     string
	Email    string
	Settings map[string]string
}

func main() {
	// Create a temporary directory for the database
	dbPath := filepath.Join(os.TempDir(), "chaindb_table_test")
	defer os.RemoveAll(dbPath)

	// Open the database
	db, err := chaindb.NewDatabase(dbPath)
	if err != nil {
		log.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Create tables for different data types
	usersTable := chaindb.NewTable(db, []byte("users:"))
	settingsTable := chaindb.NewTable(db, []byte("settings:"))
	logsTable := chaindb.NewTable(db, []byte("logs:"))

	// Example 1: Store user data
	fmt.Println("\nExample 1: Storing user data")
	user := User{
		ID:    "user1",
		Name:  "John Doe",
		Email: "john@example.com",
		Settings: map[string]string{
			"theme":    "dark",
			"language": "en",
		},
	}

	// Store user data in users table
	if err := usersTable.Put([]byte(user.ID), []byte(user.Name)); err != nil {
		log.Fatalf("Failed to store user: %v", err)
	}

	// Store user email in users table with a different key
	if err := usersTable.Put([]byte(user.ID+":email"), []byte(user.Email)); err != nil {
		log.Fatalf("Failed to store user email: %v", err)
	}

	// Store user settings in settings table
	for key, value := range user.Settings {
		settingsKey := fmt.Sprintf("%s:%s", user.ID, key)
		if err := settingsTable.Put([]byte(settingsKey), []byte(value)); err != nil {
			log.Fatalf("Failed to store setting: %v", err)
		}
	}

	// Example 2: Batch operations with tables
	fmt.Println("\nExample 2: Batch operations")
	batch := usersTable.NewBatch()
	batch.Put([]byte("user2"), []byte("Alice Smith"))
	batch.Put([]byte("user3"), []byte("Bob Johnson"))
	if err := batch.Write(); err != nil {
		log.Fatalf("Failed to write batch: %v", err)
	}

	// Example 3: Reading data from tables
	fmt.Println("\nExample 3: Reading data")

	// Read user data
	userData, err := usersTable.Get([]byte("user1"))
	if err != nil {
		log.Fatalf("Failed to get user: %v", err)
	}
	fmt.Printf("User data: %s\n", string(userData))

	// Read user email
	userEmail, err := usersTable.Get([]byte("user1:email"))
	if err != nil {
		log.Fatalf("Failed to get user email: %v", err)
	}
	fmt.Printf("User email: %s\n", string(userEmail))

	// Read user settings
	theme, err := settingsTable.Get([]byte("user1:theme"))
	if err != nil {
		log.Fatalf("Failed to get theme setting: %v", err)
	}
	fmt.Printf("User theme: %s\n", string(theme))

	// Example 4: Iterating over table contents
	fmt.Println("\nExample 4: Iterating over users")
	iter := usersTable.NewIterator(nil, nil)
	defer iter.Release()

	fmt.Println("All users:")
	for iter.Next() {
		key := string(iter.Key())
		value := string(iter.Value())
		fmt.Printf("  %s: %s\n", key, value)
	}

	// Example 5: Logging operations
	fmt.Println("\nExample 5: Logging operations")
	logs := []string{
		"User logged in",
		"Settings updated",
		"Profile viewed",
	}

	for i, logMsg := range logs {
		logKey := fmt.Sprintf("user1:%d", i)
		if err := logsTable.Put([]byte(logKey), []byte(logMsg)); err != nil {
			log.Fatalf("Failed to store log: %v", err)
		}
	}

	// Read all logs for user1
	fmt.Println("User logs:")
	logIter := logsTable.NewIterator([]byte("user1:"), nil)
	defer logIter.Release()

	for logIter.Next() {
		key := string(logIter.Key())
		value := string(logIter.Value())
		fmt.Printf("  %s: %s\n", key, value)
	}

	// Example 6: Deleting data
	fmt.Println("\nExample 6: Deleting data")

	// Delete a user
	if err := usersTable.Delete([]byte("user2")); err != nil {
		log.Fatalf("Failed to delete user: %v", err)
	}

	// Delete all settings for a user
	settingsIter := settingsTable.NewIterator([]byte("user1:"), nil)
	defer settingsIter.Release()

	for settingsIter.Next() {
		key := settingsIter.Key()
		if err := settingsTable.Delete(key); err != nil {
			log.Fatalf("Failed to delete setting: %v", err)
		}
	}

	// Verify deletion
	exists, err := usersTable.Has([]byte("user2"))
	if err != nil {
		log.Fatalf("Failed to check user existence: %v", err)
	}
	fmt.Printf("User2 exists after deletion: %v\n", exists)

	// Example 7: Database maintenance
	fmt.Println("\nExample 7: Database maintenance")

	// Compact the database
	if err := db.Compact(nil, nil); err != nil {
		log.Fatalf("Failed to compact database: %v", err)
	}

	// Get database statistics
	stats, err := db.Stat()
	if err != nil {
		log.Fatalf("Failed to get database stats: %v", err)
	}
	fmt.Printf("Database stats: %s\n", stats)

	// Sync to disk
	if err := db.SyncKeyValue(); err != nil {
		log.Fatalf("Failed to sync database: %v", err)
	}

	fmt.Println("\nAll examples completed successfully")
}
