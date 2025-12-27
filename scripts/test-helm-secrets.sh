#!/usr/bin/env bash
set -euo pipefail

# Test script for secret handling in Helm chart
# This validates different secret configuration scenarios

CHART_DIR="deploy/helm/llm-proxy"
FAILED=0

echo "=== Helm Secret Handling Validation ==="
echo ""

# Test 1: Existing secret for MANAGEMENT_TOKEN
echo "Test 1: Existing MANAGEMENT_TOKEN secret reference"
OUTPUT=$(helm template test "${CHART_DIR}" \
  --show-only templates/deployment.yaml \
  --set image.repository=test --set image.tag=test \
  --set secrets.managementToken.existingSecret.name=my-mgmt-secret)
if echo "$OUTPUT" | grep -A 3 "name: MANAGEMENT_TOKEN" | grep -q "name: my-mgmt-secret"; then
  echo "✓ PASSED"
else
  echo "✗ FAILED - MANAGEMENT_TOKEN secretKeyRef not found"
  FAILED=1
fi
echo ""

# Test 2: Existing DATABASE_URL secret
echo "Test 2: Existing DATABASE_URL secret reference"
OUTPUT=$(helm template test "${CHART_DIR}" \
  --show-only templates/deployment.yaml \
  --set image.repository=test --set image.tag=test \
  --set secrets.databaseUrl.existingSecret.name=db-secret)
if echo "$OUTPUT" | grep -A 3 "name: DATABASE_URL" | grep -q "name: db-secret"; then
  echo "✓ PASSED"
else
  echo "✗ FAILED - DATABASE_URL secretKeyRef not found"
  FAILED=1
fi
echo ""

# Test 3: Chart-managed secret
echo "Test 3: Chart-managed secret creation"
OUTPUT=$(helm template test "${CHART_DIR}" \
  --show-only templates/secret.yaml \
  --set image.repository=test --set image.tag=test \
  --set secrets.create=true \
  --set-string secrets.data.managementToken=test123)
if echo "$OUTPUT" | grep -q "kind: Secret"; then
  echo "✓ PASSED"
else
  echo "✗ FAILED"
  FAILED=1
fi
echo ""

# Test 4: PostgreSQL configuration
echo "Test 4: PostgreSQL DB_DRIVER configuration"
OUTPUT=$(helm template test "${CHART_DIR}" \
  --show-only templates/deployment.yaml \
  --set image.repository=test --set image.tag=test \
  --set env.DB_DRIVER=postgres)
if echo "$OUTPUT" | grep -q 'value: "postgres"'; then
  echo "✓ PASSED"
else
  echo "✗ FAILED"
  FAILED=1
fi
echo ""

# Test 5: Custom secret key
echo "Test 5: Custom secret key names"
OUTPUT=$(helm template test "${CHART_DIR}" \
  --show-only templates/deployment.yaml \
  --set image.repository=test --set image.tag=test \
  --set secrets.managementToken.existingSecret.name=secret \
  --set secrets.managementToken.existingSecret.key=CUSTOM_KEY)
if echo "$OUTPUT" | grep -A 5 "name: MANAGEMENT_TOKEN" | grep -q "key: CUSTOM_KEY"; then
  echo "✓ PASSED"
else
  echo "✗ FAILED"
  FAILED=1
fi
echo ""

# Test 6: Both secrets at once
echo "Test 6: Both secrets configured simultaneously"
OUTPUT=$(helm template test "${CHART_DIR}" \
  --show-only templates/deployment.yaml \
  --set image.repository=test --set image.tag=test \
  --set secrets.managementToken.existingSecret.name=mgmt \
  --set secrets.databaseUrl.existingSecret.name=db)
if echo "$OUTPUT" | grep -A 3 "name: MANAGEMENT_TOKEN" | grep -q "name: mgmt" && \
   echo "$OUTPUT" | grep -A 3 "name: DATABASE_URL" | grep -q "name: db"; then
  echo "✓ PASSED"
else
  echo "✗ FAILED"
  FAILED=1
fi
echo ""

# Test 7: No secrets by default
echo "Test 7: No secrets created by default"
OUTPUT=$(helm template test "${CHART_DIR}" \
  --show-only templates/secret.yaml \
  --set image.repository=test --set image.tag=test 2>&1 || true)
if ! echo "$OUTPUT" | grep -q "kind: Secret"; then
  echo "✓ PASSED"
else
  echo "✗ FAILED"
  FAILED=1
fi
echo ""

# Test 8: Chart-managed secret with both values
echo "Test 8: Chart-managed secret with both MANAGEMENT_TOKEN and DATABASE_URL"
SECRET_OUTPUT=$(helm template test "${CHART_DIR}" \
  --show-only templates/secret.yaml \
  --set image.repository=test --set image.tag=test \
  --set secrets.create=true \
  --set-string secrets.data.managementToken=token123 \
  --set-string secrets.data.databaseUrl=postgres://test)
if echo "$SECRET_OUTPUT" | grep -q "MANAGEMENT_TOKEN:" && echo "$SECRET_OUTPUT" | grep -q "DATABASE_URL:"; then
  echo "✓ PASSED"
else
  echo "✗ FAILED"
  FAILED=1
fi
echo ""

# Test 9: Chart-managed secret with only MANAGEMENT_TOKEN (edge case)
echo "Test 9: Chart-managed secret with only MANAGEMENT_TOKEN"
SECRET_OUTPUT=$(helm template test "${CHART_DIR}" \
  --show-only templates/secret.yaml \
  --set image.repository=test --set image.tag=test \
  --set secrets.create=true \
  --set-string secrets.data.managementToken=token123)
DEPLOY_OUTPUT=$(helm template test "${CHART_DIR}" \
  --show-only templates/deployment.yaml \
  --set image.repository=test --set image.tag=test \
  --set secrets.create=true \
  --set-string secrets.data.managementToken=token123)
# Verify Secret contains only MANAGEMENT_TOKEN
if echo "$SECRET_OUTPUT" | grep -q "MANAGEMENT_TOKEN:" && ! echo "$SECRET_OUTPUT" | grep -q "DATABASE_URL:"; then
  echo "✓ PASSED - Secret contains only MANAGEMENT_TOKEN"
else
  echo "✗ FAILED - Secret should contain only MANAGEMENT_TOKEN"
  FAILED=1
fi
# Verify deployment references only MANAGEMENT_TOKEN env var (no DATABASE_URL)
if echo "$DEPLOY_OUTPUT" | grep -q "name: MANAGEMENT_TOKEN" && ! echo "$DEPLOY_OUTPUT" | grep -q "name: DATABASE_URL"; then
  echo "✓ PASSED - Deployment references only MANAGEMENT_TOKEN"
else
  echo "✗ FAILED - Deployment should reference only MANAGEMENT_TOKEN"
  FAILED=1
fi
echo ""

# Test 10: Chart-managed secret with only DATABASE_URL (edge case)
echo "Test 10: Chart-managed secret with only DATABASE_URL"
SECRET_OUTPUT=$(helm template test "${CHART_DIR}" \
  --show-only templates/secret.yaml \
  --set image.repository=test --set image.tag=test \
  --set secrets.create=true \
  --set-string secrets.data.databaseUrl=postgres://test)
DEPLOY_OUTPUT=$(helm template test "${CHART_DIR}" \
  --show-only templates/deployment.yaml \
  --set image.repository=test --set image.tag=test \
  --set secrets.create=true \
  --set-string secrets.data.databaseUrl=postgres://test)
# Verify Secret contains only DATABASE_URL
if echo "$SECRET_OUTPUT" | grep -q "DATABASE_URL:" && ! echo "$SECRET_OUTPUT" | grep -q "MANAGEMENT_TOKEN:"; then
  echo "✓ PASSED - Secret contains only DATABASE_URL"
else
  echo "✗ FAILED - Secret should contain only DATABASE_URL"
  FAILED=1
fi
# Verify deployment references only DATABASE_URL env var (no MANAGEMENT_TOKEN)
if echo "$DEPLOY_OUTPUT" | grep -q "name: DATABASE_URL" && ! echo "$DEPLOY_OUTPUT" | grep -q "name: MANAGEMENT_TOKEN"; then
  echo "✓ PASSED - Deployment references only DATABASE_URL"
else
  echo "✗ FAILED - Deployment should reference only DATABASE_URL"
  FAILED=1
fi
echo ""

echo "==================================="
if [ $FAILED -eq 0 ]; then
  echo "✅ All validation tests passed!"
  exit 0
else
  echo "❌ Some tests failed"
  exit 1
fi
