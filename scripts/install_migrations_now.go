// Run this to complete migration system installation:
// go run scripts/install_migrations_now.go
package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

func main() {
	projectRoot := "/home/runner/work/llm-proxy/llm-proxy"
	migrationsDir := filepath.Join(projectRoot, "internal", "database", "migrations")
	sqlDir := filepath.Join(migrationsDir, "sql")

	// Step 1: Create directories
	fmt.Println("Step 1: Creating directories...")
	if err := os.MkdirAll(sqlDir, 0755); err != nil {
		fmt.Printf("❌ Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✓ Created %s\n", migrationsDir)
	fmt.Printf("✓ Created %s\n\n", sqlDir)

	// Step 2: Copy files
	fmt.Println("Step 2: Installing files...")
	files := map[string]string{
		"/tmp/runner.go":            filepath.Join(migrationsDir, "runner.go"),
		"/tmp/runner_test.go":       filepath.Join(migrationsDir, "runner_test.go"),
		"/tmp/migrations_README.md": filepath.Join(migrationsDir, "README.md"),
	}

	for src, dst := range files {
		if err := copyFile(src, dst); err != nil {
			fmt.Printf("❌ Error copying %s: %v\n", src, err)
			os.Exit(1)
		}
		fmt.Printf("✓ Installed %s\n", filepath.Base(dst))
	}
	fmt.Println()

	// Step 3: Add dependency
	fmt.Println("Step 3: Adding goose dependency...")
	cmd := exec.Command("go", "get", "-u", "github.com/pressly/goose/v3")
	cmd.Dir = projectRoot
	if output, err := cmd.CombinedOutput(); err != nil {
		fmt.Printf("❌ Error: %v\n%s\n", err, output)
		os.Exit(1)
	}
	fmt.Println("✓ Added github.com/pressly/goose/v3\n")

	// Step 4: Tidy
	fmt.Println("Step 4: Tidying dependencies...")
	cmd = exec.Command("go", "mod", "tidy")
	cmd.Dir = projectRoot
	if output, err := cmd.CombinedOutput(); err != nil {
		fmt.Printf("❌ Error: %v\n%s\n", err, output)
		os.Exit(1)
	}
	fmt.Println("✓ Dependencies tidied\n")

	// Step 5: Test
	fmt.Println("Step 5: Running tests...")
	cmd = exec.Command("go", "test", "./internal/database/migrations/...", "-v")
	cmd.Dir = projectRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("\n⚠️  Tests failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\n✅ Installation complete!")
	fmt.Println("\nNext: Run 'make test' to verify full test suite")
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err = io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}
