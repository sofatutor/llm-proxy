package main

import (
	"context"
	"fmt"
	"os"

	"github.com/sofatutor/llm-proxy/internal/database"
	"github.com/sofatutor/llm-proxy/internal/encryption"
	"github.com/spf13/cobra"
)

// migrateEncryptCmd encrypts existing plaintext data
var migrateEncryptCmd = &cobra.Command{
	Use:   "encrypt",
	Short: "Encrypt existing plaintext sensitive data",
	Long: `Encrypt existing plaintext API keys and hash existing tokens.

This command reads all existing data from the database and:
- Encrypts plaintext API keys using AES-256-GCM
- Hashes plaintext tokens using SHA-256

IMPORTANT: 
- Set ENCRYPTION_KEY environment variable before running
- This operation is idempotent - already encrypted/hashed data is skipped
- Back up your database before running this command

Example:
  export ENCRYPTION_KEY=$(openssl rand -base64 32)
  llm-proxy migrate encrypt --db ./data/llm-proxy.db`,
	RunE: runMigrateEncrypt,
}

// migrateEncryptStatsCmd shows encryption status of the database
var migrateEncryptStatsCmd = &cobra.Command{
	Use:   "encrypt-status",
	Short: "Show encryption status of database",
	Long:  `Display statistics about encrypted vs plaintext data in the database.`,
	RunE:  runMigrateEncryptStats,
}

func init() {
	// Add encrypt commands to migrate
	migrateCmd.AddCommand(migrateEncryptCmd)
	migrateCmd.AddCommand(migrateEncryptStatsCmd)
}

func runMigrateEncrypt(cmd *cobra.Command, args []string) error {
	// Check for encryption key
	encryptionKey := os.Getenv("ENCRYPTION_KEY")
	if encryptionKey == "" {
		return fmt.Errorf("ENCRYPTION_KEY environment variable is required\n" +
			"Generate one with: openssl rand -base64 32")
	}

	// Create encryptor
	encryptor, err := encryption.NewEncryptorFromBase64Key(encryptionKey)
	if err != nil {
		return fmt.Errorf("failed to create encryptor: %w\n"+
			"Make sure ENCRYPTION_KEY is a valid base64-encoded 32-byte key", err)
	}

	// Create hasher for tokens
	hasher := encryption.NewTokenHasher()

	// Open database
	db, err := openDatabaseForEncryption()
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			fmt.Printf("Warning: Failed to close database: %v\n", closeErr)
		}
	}()

	ctx := context.Background()

	// Encrypt API keys
	fmt.Println("Encrypting API keys...")
	projectsEncrypted, projectsSkipped, err := encryptProjectAPIKeys(ctx, db, encryptor)
	if err != nil {
		return fmt.Errorf("failed to encrypt API keys: %w", err)
	}
	fmt.Printf("  API keys: %d encrypted, %d already encrypted (skipped)\n", projectsEncrypted, projectsSkipped)

	// Hash tokens
	fmt.Println("Hashing tokens...")
	tokensHashed, tokensSkipped, err := hashTokens(ctx, db, hasher)
	if err != nil {
		return fmt.Errorf("failed to hash tokens: %w", err)
	}
	fmt.Printf("  Tokens: %d hashed, %d already hashed (skipped)\n", tokensHashed, tokensSkipped)

	fmt.Println("\nEncryption migration completed successfully!")
	return nil
}

func runMigrateEncryptStats(cmd *cobra.Command, args []string) error {
	// Open database
	db, err := openDatabaseForEncryption()
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			fmt.Printf("Warning: Failed to close database: %v\n", closeErr)
		}
	}()

	ctx := context.Background()

	// Count encrypted vs plaintext API keys
	encryptedKeys, plaintextKeys, err := countProjectEncryptionStatus(ctx, db)
	if err != nil {
		return fmt.Errorf("failed to count API key encryption status: %w", err)
	}

	// Count hashed vs plaintext tokens
	hashedTokens, plaintextTokens, err := countTokenHashStatus(ctx, db)
	if err != nil {
		return fmt.Errorf("failed to count token hash status: %w", err)
	}

	fmt.Println("Database Encryption Status")
	fmt.Println("==========================")
	fmt.Printf("\nAPI Keys:\n")
	fmt.Printf("  Encrypted: %d\n", encryptedKeys)
	fmt.Printf("  Plaintext: %d\n", plaintextKeys)
	if plaintextKeys > 0 {
		fmt.Printf("  ⚠️  Warning: %d API keys are stored in plaintext\n", plaintextKeys)
	} else if encryptedKeys > 0 {
		fmt.Printf("  ✓ All API keys are encrypted\n")
	}

	fmt.Printf("\nTokens:\n")
	fmt.Printf("  Hashed: %d\n", hashedTokens)
	fmt.Printf("  Plaintext: %d\n", plaintextTokens)
	if plaintextTokens > 0 {
		fmt.Printf("  ⚠️  Warning: %d tokens are stored in plaintext\n", plaintextTokens)
	} else if hashedTokens > 0 {
		fmt.Printf("  ✓ All tokens are hashed\n")
	}

	return nil
}

func openDatabaseForEncryption() (*database.DB, error) {
	dbPath := databasePath
	if dbPath == "" {
		dbPath = os.Getenv("DATABASE_PATH")
		if dbPath == "" {
			dbPath = "data/llm-proxy.db"
		}
	}

	dbConfig := database.Config{Path: dbPath}
	db, err := database.New(dbConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to open database at %s: %w", dbPath, err)
	}

	return db, nil
}

func encryptProjectAPIKeys(ctx context.Context, db *database.DB, encryptor *encryption.Encryptor) (encrypted, skipped int, err error) {
	// List all projects
	projects, err := db.ListProjects(ctx)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to list projects: %w", err)
	}

	for _, project := range projects {
		// Skip if already encrypted
		if encryption.IsEncrypted(project.OpenAIAPIKey) {
			skipped++
			continue
		}

		// Skip empty API keys
		if project.OpenAIAPIKey == "" {
			skipped++
			continue
		}

		// Encrypt the API key
		encryptedKey, err := encryptor.Encrypt(project.OpenAIAPIKey)
		if err != nil {
			return encrypted, skipped, fmt.Errorf("failed to encrypt API key for project %s: %w", project.ID, err)
		}

		// Update the project with encrypted key
		project.OpenAIAPIKey = encryptedKey
		if err := db.UpdateProject(ctx, project); err != nil {
			return encrypted, skipped, fmt.Errorf("failed to update project %s: %w", project.ID, err)
		}

		encrypted++
	}

	return encrypted, skipped, nil
}

func hashTokens(ctx context.Context, db *database.DB, hasher *encryption.TokenHasher) (hashed, skipped int, err error) {
	// Get underlying database connection for direct SQL access
	sqlDB := db.DB()
	if sqlDB == nil {
		return 0, 0, fmt.Errorf("failed to get database connection")
	}

	// Query all tokens
	rows, err := sqlDB.QueryContext(ctx, "SELECT token, project_id FROM tokens")
	if err != nil {
		return 0, 0, fmt.Errorf("failed to query tokens: %w", err)
	}
	defer func() { _ = rows.Close() }()

	type tokenRecord struct {
		token     string
		projectID string
	}
	var tokens []tokenRecord

	for rows.Next() {
		var t tokenRecord
		if err := rows.Scan(&t.token, &t.projectID); err != nil {
			return 0, 0, fmt.Errorf("failed to scan token: %w", err)
		}
		tokens = append(tokens, t)
	}
	if err := rows.Err(); err != nil {
		return 0, 0, fmt.Errorf("error iterating tokens: %w", err)
	}

	for _, t := range tokens {
		// Skip if already hashed (SHA-256 produces 64 hex characters)
		if len(t.token) == 64 {
			skipped++
			continue
		}

		// Hash the token
		hashedToken := hasher.CreateLookupKey(t.token)

		// Update the token with hashed value
		_, err := sqlDB.ExecContext(ctx,
			"UPDATE tokens SET token = ? WHERE token = ?",
			hashedToken, t.token)
		if err != nil {
			return hashed, skipped, fmt.Errorf("failed to update token: %w", err)
		}

		hashed++
	}

	return hashed, skipped, nil
}

func countProjectEncryptionStatus(ctx context.Context, db *database.DB) (encrypted, plaintext int, err error) {
	projects, err := db.ListProjects(ctx)
	if err != nil {
		return 0, 0, err
	}

	for _, project := range projects {
		if encryption.IsEncrypted(project.OpenAIAPIKey) {
			encrypted++
		} else if project.OpenAIAPIKey != "" {
			plaintext++
		}
	}

	return encrypted, plaintext, nil
}

func countTokenHashStatus(ctx context.Context, db *database.DB) (hashed, plaintext int, err error) {
	sqlDB := db.DB()
	if sqlDB == nil {
		return 0, 0, fmt.Errorf("failed to get database connection")
	}

	rows, err := sqlDB.QueryContext(ctx, "SELECT token FROM tokens")
	if err != nil {
		return 0, 0, err
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var token string
		if err := rows.Scan(&token); err != nil {
			return 0, 0, err
		}

		// SHA-256 produces 64 hex characters
		if len(token) == 64 {
			hashed++
		} else {
			plaintext++
		}
	}

	return hashed, plaintext, rows.Err()
}
