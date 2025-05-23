# Admin Package

This package provides the admin interface functionality:

- Admin UI handlers and templates
- Project and token management interface
- Authentication and authorization
- Dashboard and statistics views

# Admin Module

## Testing Strategy

### Goals
- Achieve and maintain **90%+ code coverage** for all admin server logic.
- Cover all HTTP handlers, error branches, and template rendering code paths.
- Ensure tests are robust to changes in working directory and template locations.

### Test Data
- Minimal HTML templates for all routes are in `internal/admin/testdata/`.
- Tests use a helper (`testTemplateDir()`) to resolve the correct path for templates, making tests CWD-agnostic.

### Adding/Updating Tests
- For each new handler or feature, add a corresponding test in `server_test.go`.
- If a handler renders a new template, add a minimal HTML file to `testdata/`.
- Use table-driven tests for logic with multiple input/output cases.
- Use mock API clients to isolate handler logic from external dependencies.

### Running Tests
- Run all tests and check coverage with:
  ```sh
  make test-coverage
  # or
  go test -cover ./internal/admin/...
  ```

### Coverage Enforcement
- PRs are not merged unless coverage is **90%+**.
- Coverage is checked in CI and locally.

### Special Conventions
- Template path resolution is handled by a helper to ensure tests work regardless of CWD.
- Minimal templates are used to satisfy the loader and cover template parsing code paths.

### Troubleshooting
- If tests fail due to missing templates, ensure all required files exist in `testdata/`.
- If coverage drops, check for untested error branches or new code.
