# Failing Test: TestNew_PermissionDenied

The unit test `TestNew_PermissionDenied` in `internal/database/database_test.go` fails when run as root because the expected permission error does not occur. This contradicts the TDD requirements in PLAN.md that mandate all tests pass with 90%+ coverage.

Excerpt from the test file:
```
207:func TestNew_PermissionDenied(t *testing.T) {
```
Test output:
```
--- FAIL: TestNew_PermissionDenied (0.00s)
    database_test.go:220: expected error for permission denied in New
```
**Plan references:**
- TDD and coverage requirement【F:PLAN.md†L3-L10】【F:PLAN.md†L237-L238】

Fix the test or adjust its setup so it reliably detects permission errors.
