#!/bin/bash
# Test script for Helm chart validation
# This script tests various deployment scenarios for the llm-proxy Helm chart

set -e

CHART_PATH="deploy/helm/llm-proxy"
echo "Testing Helm Chart: ${CHART_PATH}"
echo "=========================================="

# Test 1: Lint the chart
echo ""
echo "Test 1: Helm Lint"
echo "-------------------"
helm lint "${CHART_PATH}"
echo "✓ Lint passed"

# Test 2: Template rendering with default values
echo ""
echo "Test 2: Default Values Rendering"
echo "-------------------"
helm template test-release "${CHART_PATH}" > /dev/null
echo "✓ Default rendering succeeded"

# Test 3: Template with existing secret (production pattern)
echo ""
echo "Test 3: Production Pattern - Existing Secret"
echo "-------------------"
OUTPUT=$(helm template test-release "${CHART_PATH}" \
  --set secrets.managementToken.existingSecret.name=llm-proxy-secrets \
  --set secrets.managementToken.existingSecret.key=MANAGEMENT_TOKEN)
  
if echo "$OUTPUT" | grep -q "secretKeyRef"; then
  echo "✓ Secret reference found in deployment"
else
  echo "✗ Secret reference NOT found"
  exit 1
fi

if echo "$OUTPUT" | grep -q "name: llm-proxy-secrets"; then
  echo "✓ Correct secret name referenced"
else
  echo "✗ Wrong secret name"
  exit 1
fi

# Test 4: PostgreSQL with DATABASE_URL
echo ""
echo "Test 4: PostgreSQL Configuration"
echo "-------------------"
OUTPUT=$(helm template test-release "${CHART_PATH}" \
  --set secrets.managementToken.existingSecret.name=llm-secrets \
  --set env.DB_DRIVER=postgres \
  --set secrets.databaseUrl.existingSecret.name=db-secrets \
  --set secrets.databaseUrl.existingSecret.key=DATABASE_URL)

if echo "$OUTPUT" | grep -q "DATABASE_URL"; then
  echo "✓ DATABASE_URL environment variable configured"
else
  echo "✗ DATABASE_URL not found"
  exit 1
fi

if echo "$OUTPUT" | grep -q 'DB_DRIVER: "postgres"'; then
  echo "✓ DB_DRIVER set to postgres"
else
  echo "✗ DB_DRIVER not set correctly"
  exit 1
fi

# Test 5: secrets.create mode (development)
echo ""
echo "Test 5: Development Pattern - Chart-Managed Secret"
echo "-------------------"
OUTPUT=$(helm template test-release "${CHART_PATH}" \
  --set secrets.create=true \
  --set secrets.data.managementToken=test-token)

if echo "$OUTPUT" | grep -q "kind: Secret"; then
  echo "✓ Secret resource created"
else
  echo "✗ Secret resource NOT created"
  exit 1
fi

if echo "$OUTPUT" | grep -q "MANAGEMENT_TOKEN:"; then
  echo "✓ MANAGEMENT_TOKEN in secret data"
else
  echo "✗ MANAGEMENT_TOKEN not in secret"
  exit 1
fi

# Test 6: Autoscaling configuration
echo ""
echo "Test 6: Autoscaling Configuration"
echo "-------------------"
OUTPUT=$(helm template test-release "${CHART_PATH}" \
  --set autoscaling.enabled=true \
  --set autoscaling.minReplicas=2 \
  --set autoscaling.maxReplicas=10)

if echo "$OUTPUT" | grep -q "kind: HorizontalPodAutoscaler"; then
  echo "✓ HPA resource created"
else
  echo "✗ HPA resource NOT created"
  exit 1
fi

if echo "$OUTPUT" | grep -q "minReplicas: 2"; then
  echo "✓ minReplicas set correctly"
else
  echo "✗ minReplicas not set"
  exit 1
fi

# Test 7: Ingress configuration
echo ""
echo "Test 7: Ingress Configuration"
echo "-------------------"
OUTPUT=$(helm template test-release "${CHART_PATH}" \
  --set ingress.enabled=true \
  --set ingress.hosts[0].host=llm-proxy.example.com)

if echo "$OUTPUT" | grep -q "kind: Ingress"; then
  echo "✓ Ingress resource created"
else
  echo "✗ Ingress resource NOT created"
  exit 1
fi

if echo "$OUTPUT" | grep -q "llm-proxy.example.com"; then
  echo "✓ Ingress host configured"
else
  echo "✗ Ingress host not configured"
  exit 1
fi

# Test 8: Example values files
echo ""
echo "Test 8: Example Values Files"
echo "-------------------"
helm template test-release "${CHART_PATH}" \
  -f "${CHART_PATH}/values-sqlite-example.yaml" > /dev/null
echo "✓ SQLite example values render successfully"

helm template test-release "${CHART_PATH}" \
  -f "${CHART_PATH}/values-postgres-example.yaml" > /dev/null
echo "✓ PostgreSQL example values render successfully"

echo ""
echo "=========================================="
echo "✓ All tests passed!"
echo "=========================================="
