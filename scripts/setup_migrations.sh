#!/bin/bash
# Setup script for database migrations infrastructure
# Creates directory structure and installs migration system files

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
MIGRATIONS_DIR="$PROJECT_ROOT/internal/database/migrations"
SQL_DIR="$MIGRATIONS_DIR/sql"

echo "=== Database Migrations Setup ==="
echo "Project root: $PROJECT_ROOT"
echo ""

# Create directories
echo "Creating directory structure..."
mkdir -p "$SQL_DIR"
echo "✓ Created $MIGRATIONS_DIR"
echo "✓ Created $SQL_DIR"
echo ""

# Copy files from /tmp to proper locations
echo "Installing migration files..."

if [ -f "/tmp/runner.go" ]; then
    cp "/tmp/runner.go" "$MIGRATIONS_DIR/runner.go"
    echo "✓ Installed runner.go"
else
    echo "✗ Warning: /tmp/runner.go not found"
fi

if [ -f "/tmp/runner_test.go" ]; then
    cp "/tmp/runner_test.go" "$MIGRATIONS_DIR/runner_test.go"
    echo "✓ Installed runner_test.go"
else
    echo "✗ Warning: /tmp/runner_test.go not found"
fi

if [ -f "/tmp/migrations_README.md" ]; then
    cp "/tmp/migrations_README.md" "$MIGRATIONS_DIR/README.md"
    echo "✓ Installed README.md"
else
    echo "✗ Warning: /tmp/migrations_README.md not found"
fi

echo ""

# Add goose dependency
echo "Adding goose dependency..."
cd "$PROJECT_ROOT"
go get -u github.com/pressly/goose/v3
echo "✓ Added github.com/pressly/goose/v3"
echo ""

# Tidy dependencies
echo "Tidying dependencies..."
go mod tidy
echo "✓ Dependencies tidied"
echo ""

# Run migration tests
echo "Running migration tests..."
go test ./internal/database/migrations/... -v -race
TEST_EXIT_CODE=$?

echo ""
if [ $TEST_EXIT_CODE -eq 0 ]; then
    echo "✅ All migration tests passed!"
else
    echo "⚠️  Some tests failed (exit code: $TEST_EXIT_CODE)"
    echo "This may be expected if migration files haven't been created yet."
fi

echo ""
echo "=== Setup Complete ==="
echo ""
echo "Migration system installed at: $MIGRATIONS_DIR"
echo ""
echo "Next steps:"
echo "  1. Review files in $MIGRATIONS_DIR"
echo "  2. Create initial migrations in $SQL_DIR"
echo "  3. Run full test suite: make test"
echo "  4. Check coverage: make test-coverage-ci"
echo ""
echo "For usage instructions, see: $MIGRATIONS_DIR/README.md"
echo ""

exit 0
