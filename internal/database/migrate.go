package database

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
)

// RunMigrations executes all SQL migration files in the migrations directory
func (db *DB) RunMigrations() error {
	log.Println("Running database migrations...")

	// Get the migrations directory path
	migrationsDir := "migrations"

	// Check if migrations directory exists
	if _, err := os.Stat(migrationsDir); os.IsNotExist(err) {
		// Try alternate path (for Docker container)
		migrationsDir = "./migrations"
		if _, err := os.Stat(migrationsDir); os.IsNotExist(err) {
			return fmt.Errorf("migrations directory not found")
		}
	}

	// Read all migration files
	files, err := os.ReadDir(migrationsDir)
	if err != nil {
		return fmt.Errorf("failed to read migrations directory: %w", err)
	}

	// Filter and sort SQL files
	var sqlFiles []string
	for _, file := range files {
		if !file.IsDir() && filepath.Ext(file.Name()) == ".sql" {
			sqlFiles = append(sqlFiles, file.Name())
		}
	}
	sort.Strings(sqlFiles)

	if len(sqlFiles) == 0 {
		log.Println("No migration files found")
		return nil
	}

	// Execute each migration file
	ctx := context.Background()
	for _, filename := range sqlFiles {
		log.Printf("Executing migration: %s", filename)

		filePath := filepath.Join(migrationsDir, filename)
		content, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("failed to read migration file %s: %w", filename, err)
		}

		// Execute the SQL
		_, err = db.Pool.Exec(ctx, string(content))
		if err != nil {
			// Check if error is about objects already existing
			if isAlreadyExistsError(err) {
				log.Printf("Migration %s: objects already exist (skipping)", filename)
				continue
			}
			return fmt.Errorf("failed to execute migration %s: %w", filename, err)
		}

		log.Printf("Successfully executed migration: %s", filename)
	}

	log.Println("All migrations completed successfully!")
	return nil
}

// isAlreadyExistsError checks if the error is about objects already existing
func isAlreadyExistsError(err error) bool {
	if err == nil {
		return false
	}
	errMsg := err.Error()
	return containsAny(errMsg, []string{
		"already exists",
		"duplicate key",
		"relation already exists",
	})
}

// containsAny checks if a string contains any of the substrings
func containsAny(s string, substrs []string) bool {
	for _, substr := range substrs {
		if len(s) >= len(substr) {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
		}
	}
	return false
}
