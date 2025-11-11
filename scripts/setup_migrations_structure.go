// Package main provides a setup utility for creating the migrations package structure.
// This is a Go-based alternative to setup_migrations.sh
//
// Usage:
//   go run scripts/setup_migrations_structure.go
//
package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
)

func main() {
	fmt.Println("=== Database Migrations Setup ===")

	projectRoot := findProjectRoot()
	if projectRoot == "" {
		log.Fatal("❌ Could not find project root (go.mod not found)")
	}
	fmt.Printf("Project root: %s\n\n", projectRoot)

	migrationsDir := filepath.Join(projectRoot, "internal", "database", "migrations")
	sqlDir := filepath.Join(migrationsDir, "sql")

	// Step 1: Create directories
	fmt.Println("Step 1: Creating directory structure...")
	if err := os.MkdirAll(sqlDir, 0755); err != nil {
		log.Fatalf("❌ Failed to create directories: %v", err)
	}
	fmt.Printf("✓ Created %s\n", migrationsDir)
	fmt.Printf("✓ Created %s\n\n", sqlDir)

	// Step 2: Copy files from /tmp
	fmt.Println("Step 2: Installing migration files...")
	files := map[string]string{
		"/tmp/runner.go":            filepath.Join(migrationsDir, "runner.go"),
		"/tmp/runner_test.go":       filepath.Join(migrationsDir, "runner_test.go"),
		"/tmp/migrations_README.md": filepath.Join(migrationsDir, "README.md"),
	}

	for src, dst := range files {
		if err := copyFile(src, dst); err != nil {
			fmt.Printf("⚠️  Warning: Failed to copy %s: %v\n", filepath.Base(src), err)
		} else {
			fmt.Printf("✓ Installed %s\n", filepath.Base(dst))
		}
	}
	fmt.Println()

	// Step 3: Add goose dependency
	fmt.Println("Step 3: Adding goose dependency...")
	cmd := exec.Command("go", "get", "-u", "github.com/pressly/goose/v3")
	cmd.Dir = projectRoot
	if output, err := cmd.CombinedOutput(); err != nil {
		fmt.Printf("⚠️  Warning: go get failed: %v\n%s\n", err, output)
	} else {
		fmt.Println("✓ Added github.com/pressly/goose/v3")
	}
	fmt.Println()

	// Step 4: Tidy dependencies
	fmt.Println("Step 4: Tidying dependencies...")
	cmd = exec.Command("go", "mod", "tidy")
	cmd.Dir = projectRoot
	if output, err := cmd.CombinedOutput(); err != nil {
		fmt.Printf("⚠️  Warning: go mod tidy failed: %v\n%s\n", err, output)
	} else {
		fmt.Println("✓ Dependencies tidied")
	}
	fmt.Println()

	// Step 5: Run tests
	fmt.Println("Step 5: Running migration tests...")
	cmd = exec.Command("go", "test", "./internal/database/migrations/...", "-v", "-race")
	cmd.Dir = projectRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("\n⚠️  Some tests failed: %v\n", err)
		fmt.Println("This may be expected if migration files haven't been created yet.")
	} else {
		fmt.Println("\n✅ All migration tests passed!")
	}
	fmt.Println()

	// Summary
	fmt.Println("=== Setup Complete ===")
	fmt.Println()
	fmt.Printf("Migration system installed at: %s\n", migrationsDir)
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Printf("  1. Review files in %s\n", migrationsDir)
	fmt.Printf("  2. Create initial migrations in %s\n", sqlDir)
	fmt.Println("  3. Run full test suite: make test")
	fmt.Println("  4. Check coverage: make test-coverage-ci")
	fmt.Println()
	fmt.Printf("For usage instructions, see: %s/README.md\n", migrationsDir)
	fmt.Println()
}

// findProjectRoot walks up the directory tree to find go.mod
func findProjectRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached filesystem root
			return ""
		}
		dir = parent
	}
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open source: %w", err)
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("create dest: %w", err)
	}
	defer destFile.Close()

	if _, err := io.Copy(destFile, sourceFile); err != nil {
		return fmt.Errorf("copy: %w", err)
	}

	if err := destFile.Sync(); err != nil {
		return fmt.Errorf("sync: %w", err)
	}

	// Copy permissions
	if info, err := os.Stat(src); err == nil {
		if err := os.Chmod(dst, info.Mode()); err != nil {
			return fmt.Errorf("chmod: %w", err)
		}
	}

	return nil
}
