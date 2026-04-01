package main

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"go.etcd.io/bbolt"
)

func main() {
	dbPath := flag.String("db", ".sync_state.db", "Path to the bbolt database file")
	listOnly := flag.Bool("list", false, "List all buckets and keys without deleting")
	flag.Parse()

	if _, err := os.Stat(*dbPath); os.IsNotExist(err) {
		fmt.Printf("Database file %s not found. Nothing to clean.\n", *dbPath)
		return
	}

	db, err := bbolt.Open(*dbPath, 0600, nil)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	err = db.Update(func(tx *bbolt.Tx) error {
		return tx.ForEach(func(name []byte, b *bbolt.Bucket) error {
			bucketName := string(name)
			
			// 1. Check if the bucket itself is a state bucket for an AutoTest account
			// SyncState_AccountID_Type
			if strings.Contains(bucketName, "AutoTest") {
				if *listOnly {
					fmt.Printf("Bucket (AutoTest): %s\n", bucketName)
					return nil
				}
				fmt.Printf("Deleting entire AutoTest bucket: %s\n", bucketName)
				return tx.DeleteBucket(name)
			}

			// 2. Iterate keys within standard buckets (State, Meta, Errors)
			// and remove any that reference "AutoTest" paths
			c := b.Cursor()
			for k, v := c.Seek(nil); k != nil; k, v = c.Next() {
				keyStr := string(k)
				if strings.Contains(keyStr, "AutoTest") {
					if *listOnly {
						fmt.Printf("  Key (AutoTest) in [%s]: %s\n", bucketName, keyStr)
						continue
					}
					fmt.Printf("Deleting AutoTest key from bucket [%s]: %s\n", bucketName, keyStr)
					if err := b.Delete(k); err != nil {
						return err
					}
					continue
				}

				// Also check value if it's a JSON containing AutoTest (e.g. FailureRecord)
				if v != nil && bytes.Contains(v, []byte("AutoTest")) {
					if *listOnly {
						fmt.Printf("  Value-Ref (AutoTest) in [%s]: %s\n", bucketName, keyStr)
						continue
					}
					fmt.Printf("Deleting AutoTest-referencing value from bucket [%s]: %s\n", bucketName, keyStr)
					if err := b.Delete(k); err != nil {
						return err
					}
				}

				if *listOnly {
					fmt.Printf("  Key: %s\n", keyStr)
				}
			}

			return nil
		})
	})

	if err != nil {
		log.Fatalf("Cleanup failed: %v", err)
	}

	fmt.Println("Database cleanup completed successfully.")
}
