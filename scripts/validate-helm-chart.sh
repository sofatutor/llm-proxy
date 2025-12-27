#!/usr/bin/env bash
set -euo pipefail

# Validate Helm chart
# This script runs helm lint and helm template to ensure the chart is valid

CHART_DIR="deploy/helm/llm-proxy"

echo "Validating Helm chart at ${CHART_DIR}..."
echo ""

# Check if helm is installed
if ! command -v helm &> /dev/null; then
    echo "ERROR: helm is not installed" >&2
    echo "Install helm from https://helm.sh/docs/intro/install/" >&2
    exit 1
fi

echo "Using helm version:"
helm version --short
echo ""

# Create a minimal mock postgresql subchart for testing if charts/ doesn't exist
# This allows validation to run without network access to download the real subchart
MOCK_CHART_DIR="${CHART_DIR}/charts/postgresql"
MOCK_CREATED=false

if [ ! -d "${CHART_DIR}/charts/postgresql" ]; then
    echo "Creating mock postgresql subchart for validation..."
    mkdir -p "${MOCK_CHART_DIR}/templates"
    
    cat > "${MOCK_CHART_DIR}/Chart.yaml" << 'EOF'
apiVersion: v2
name: postgresql
version: 15.5.38
description: Mock PostgreSQL chart for validation
type: application
EOF
    
    cat > "${MOCK_CHART_DIR}/values.yaml" << 'EOF'
auth:
  username: postgres
  database: postgres
  password: ""
  existingSecret: ""
  secretKeys:
    adminPasswordKey: postgres-password
    userPasswordKey: password
primary:
  service:
    ports:
      postgresql: 5432
  persistence:
    enabled: true
    size: 8Gi
  resources:
    limits:
      memory: 256Mi
      cpu: 500m
    requests:
      memory: 128Mi
      cpu: 100m
EOF
    
    cat > "${MOCK_CHART_DIR}/templates/_helpers.tpl" << 'EOF'
{{- define "postgresql.fullname" -}}
{{- printf "%s-postgresql" .Release.Name -}}
{{- end -}}
EOF
    
    MOCK_CREATED=true
    echo "✓ Mock postgresql subchart created"
fi
echo ""

# Run helm lint
echo "Running helm lint..."
if helm lint "${CHART_DIR}"; then
    echo "✓ helm lint passed"
else
    echo "✗ helm lint failed" >&2
    exit 1
fi
echo ""

# Run helm template with minimal overrides to validate rendering
echo "Running helm template with minimal overrides..."
if TEMPLATE_OUTPUT=$(helm template test-release "${CHART_DIR}" \
    --set image.repository=test-repo \
    --set image.tag=test-tag \
    --set env.LOG_LEVEL=debug 2>&1); then
    echo "✓ helm template rendered successfully"
else
    echo "✗ helm template failed" >&2
    echo "$TEMPLATE_OUTPUT" >&2
    exit 1
fi
echo ""

# Run helm template with minimal required values only (no env block)
echo "Running helm template with minimal required values..."
if TEMPLATE_OUTPUT=$(helm template test-release "${CHART_DIR}" \
    --set image.repository=test-repo \
    --set image.tag=test-tag 2>&1); then
    echo "✓ helm template with minimal values rendered successfully"
else
    echo "✗ helm template with minimal values failed" >&2
    echo "$TEMPLATE_OUTPUT" >&2
    exit 1
fi
echo ""

# Run helm template with custom values to test more complex scenarios
echo "Running helm template with additional overrides..."
if TEMPLATE_OUTPUT=$(helm template test-release "${CHART_DIR}" \
    --set replicaCount=3 \
    --set image.repository=custom-repo \
    --set image.tag=v1.0.0 \
    --set service.type=NodePort \
    --set service.port=9090 \
    --set resources.limits.cpu=2000m \
    --set resources.limits.memory=1Gi \
    --set env.LOG_LEVEL=trace \
    --set env.ENABLE_METRICS=false 2>&1); then
    echo "✓ helm template with custom values rendered successfully"
else
    echo "✗ helm template with custom values failed" >&2
    echo "$TEMPLATE_OUTPUT" >&2
    exit 1
fi
echo ""

# Test external PostgreSQL configuration
echo "Running helm template with external PostgreSQL..."
if TEMPLATE_OUTPUT=$(helm template test-release "${CHART_DIR}" \
    --set image.repository=test-repo \
    --set image.tag=test-tag \
    --set secrets.managementToken.existingSecret.name=test-secret \
    --set secrets.databaseUrl.existingSecret.name=test-db-secret \
    --set env.DB_DRIVER=postgres 2>&1); then
    echo "✓ helm template with external PostgreSQL rendered successfully"
else
    echo "✗ helm template with external PostgreSQL failed" >&2
    echo "$TEMPLATE_OUTPUT" >&2
    exit 1
fi
echo ""

# Test in-cluster PostgreSQL configuration
echo "Running helm template with in-cluster PostgreSQL..."
if TEMPLATE_OUTPUT=$(helm template test-release "${CHART_DIR}" \
    --set image.repository=test-repo \
    --set image.tag=test-tag \
    --set secrets.managementToken.existingSecret.name=test-secret \
    --set env.DB_DRIVER=postgres \
    --set postgresql.enabled=true \
    --set-string postgresql.auth.password=test-password 2>&1); then
    echo "✓ helm template with in-cluster PostgreSQL rendered successfully"
else
    echo "✗ helm template with in-cluster PostgreSQL failed" >&2
    echo "$TEMPLATE_OUTPUT" >&2
    exit 1
fi
echo ""

# Test validation: both in-cluster and external PostgreSQL (should fail)
echo "Testing validation: both in-cluster and external PostgreSQL..."
if TEMPLATE_OUTPUT=$(helm template test-release "${CHART_DIR}" \
    --set image.repository=test-repo \
    --set image.tag=test-tag \
    --set secrets.managementToken.existingSecret.name=test-secret \
    --set secrets.databaseUrl.existingSecret.name=test-db-secret \
    --set env.DB_DRIVER=postgres \
    --set postgresql.enabled=true \
    --set-string postgresql.auth.password=test-password 2>&1); then
    echo "✗ Validation should have failed for conflicting PostgreSQL configuration" >&2
    exit 1
else
    if echo "$TEMPLATE_OUTPUT" | grep -q "Cannot use both in-cluster PostgreSQL"; then
        echo "✓ Validation correctly rejected conflicting PostgreSQL configuration"
    else
        echo "✗ Unexpected error message" >&2
        echo "$TEMPLATE_OUTPUT" >&2
        exit 1
    fi
fi
echo ""

# Test validation: postgres driver without database (should fail)
echo "Testing validation: postgres driver without database configuration..."
if TEMPLATE_OUTPUT=$(helm template test-release "${CHART_DIR}" \
    --set image.repository=test-repo \
    --set image.tag=test-tag \
    --set secrets.managementToken.existingSecret.name=test-secret \
    --set env.DB_DRIVER=postgres 2>&1); then
    echo "✗ Validation should have failed for missing database configuration" >&2
    exit 1
else
    if echo "$TEMPLATE_OUTPUT" | grep -q "DB_DRIVER is set to 'postgres' but no database configuration found"; then
        echo "✓ Validation correctly rejected missing database configuration"
    else
        echo "✗ Unexpected error message" >&2
        echo "$TEMPLATE_OUTPUT" >&2
        exit 1
    fi
fi
echo ""

# Test dispatcher disabled (default)
echo "Running helm template with dispatcher disabled (default)..."
if TEMPLATE_OUTPUT=$(helm template test-release "${CHART_DIR}" \
    --set image.repository=test-repo \
    --set image.tag=test-tag \
    --set secrets.managementToken.existingSecret.name=test-secret 2>&1); then
    if echo "$TEMPLATE_OUTPUT" | grep -q 'name: .*llm-proxy-dispatcher'; then
        echo "✗ Dispatcher resources should not be created when disabled" >&2
        exit 1
    fi
    echo "✓ helm template with dispatcher disabled rendered successfully"
else
    echo "✗ helm template with dispatcher disabled failed" >&2
    echo "$TEMPLATE_OUTPUT" >&2
    exit 1
fi
echo ""

# Test dispatcher enabled with file backend
echo "Running helm template with dispatcher enabled (file backend)..."
if TEMPLATE_OUTPUT=$(helm template test-release "${CHART_DIR}" \
    --set image.repository=test-repo \
    --set image.tag=test-tag \
    --set secrets.managementToken.existingSecret.name=test-secret \
    --set dispatcher.enabled=true \
    --set redis.external.addr="redis.example.com:6379" \
    --set env.LLM_PROXY_EVENT_BUS="redis-streams" 2>&1); then
    if ! echo "$TEMPLATE_OUTPUT" | grep -q 'name: .*llm-proxy-dispatcher'; then
        echo "✗ Dispatcher deployment should be created when enabled" >&2
        exit 1
    fi
    if ! echo "$TEMPLATE_OUTPUT" | grep -q 'kind: PersistentVolumeClaim'; then
        echo "✗ PVC should be created for file backend" >&2
        exit 1
    fi
    echo "✓ helm template with dispatcher (file backend) rendered successfully"
else
    echo "✗ helm template with dispatcher (file backend) failed" >&2
    echo "$TEMPLATE_OUTPUT" >&2
    exit 1
fi
echo ""

# Test dispatcher with Lunary backend and existing secret
echo "Running helm template with dispatcher (Lunary + existing secret)..."
if TEMPLATE_OUTPUT=$(helm template test-release "${CHART_DIR}" \
    --set image.repository=test-repo \
    --set image.tag=test-tag \
    --set secrets.managementToken.existingSecret.name=test-secret \
    --set dispatcher.enabled=true \
    --set dispatcher.service="lunary" \
    --set dispatcher.apiKey.existingSecret.name="dispatcher-secrets" \
    --set redis.external.addr="redis.example.com:6379" \
    --set env.LLM_PROXY_EVENT_BUS="redis-streams" 2>&1); then
    if ! echo "$TEMPLATE_OUTPUT" | grep -q "llm-proxy-dispatcher"; then
        echo "✗ Dispatcher deployment should be created when enabled" >&2
        exit 1
    fi
    if echo "$TEMPLATE_OUTPUT" | grep -q 'kind: PersistentVolumeClaim'; then
        echo "✗ PVC should not be created for Lunary backend" >&2
        exit 1
    fi
    if ! echo "$TEMPLATE_OUTPUT" | grep -q 'name: dispatcher-secrets'; then
        echo "✗ Secret reference should be included for Lunary backend" >&2
        exit 1
    fi
    echo "✓ helm template with dispatcher (Lunary + secret) rendered successfully"
else
    echo "✗ helm template with dispatcher (Lunary + secret) failed" >&2
    echo "$TEMPLATE_OUTPUT" >&2
    exit 1
fi
echo ""

# Test validation: dispatcher with in-memory event bus (should fail)
echo "Testing validation: dispatcher with in-memory event bus..."
if TEMPLATE_OUTPUT=$(helm template test-release "${CHART_DIR}" \
    --set image.repository=test-repo \
    --set image.tag=test-tag \
    --set secrets.managementToken.existingSecret.name=test-secret \
    --set dispatcher.enabled=true \
    --set env.LLM_PROXY_EVENT_BUS="in-memory" 2>&1); then
    echo "✗ Validation should have failed for in-memory event bus with dispatcher" >&2
    exit 1
else
    if echo "$TEMPLATE_OUTPUT" | grep -q "Dispatcher requires a durable event bus"; then
        echo "✓ Validation correctly rejected in-memory event bus with dispatcher"
    else
        echo "✗ Unexpected error message" >&2
        echo "$TEMPLATE_OUTPUT" >&2
        exit 1
    fi
fi
echo ""

# Test validation: dispatcher without event bus configured (defaults to in-memory, should fail)
echo "Testing validation: dispatcher without event bus configured (defaults to in-memory)..."
if TEMPLATE_OUTPUT=$(helm template test-release "${CHART_DIR}" \
    --set image.repository=test-repo \
    --set image.tag=test-tag \
    --set secrets.managementToken.existingSecret.name=test-secret \
    --set dispatcher.enabled=true 2>&1); then
    echo "✗ Validation should have failed for default in-memory event bus" >&2
    exit 1
else
    if echo "$TEMPLATE_OUTPUT" | grep -q "Dispatcher requires a durable event bus"; then
        echo "✓ Validation correctly rejected default in-memory event bus"
    else
        echo "✗ Unexpected error message" >&2
        echo "$TEMPLATE_OUTPUT" >&2
        exit 1
    fi
fi
echo ""

# Test validation: dispatcher with invalid event bus type (should fail)
echo "Testing validation: dispatcher with invalid event bus type..."
if TEMPLATE_OUTPUT=$(helm template test-release "${CHART_DIR}" \
    --set image.repository=test-repo \
    --set image.tag=test-tag \
    --set secrets.managementToken.existingSecret.name=test-secret \
    --set dispatcher.enabled=true \
    --set env.LLM_PROXY_EVENT_BUS="invalid-bus" 2>&1); then
    echo "✗ Validation should have failed for invalid event bus type" >&2
    exit 1
else
    if echo "$TEMPLATE_OUTPUT" | grep -q "Dispatcher requires LLM_PROXY_EVENT_BUS to be 'redis' or 'redis-streams'"; then
        echo "✓ Validation correctly rejected invalid event bus type"
    else
        echo "✗ Unexpected error message" >&2
        echo "$TEMPLATE_OUTPUT" >&2
        exit 1
    fi
fi
echo ""

# Test validation: dispatcher without API key for non-file service (should fail)
echo "Testing validation: dispatcher without API key for non-file service..."
if TEMPLATE_OUTPUT=$(helm template test-release "${CHART_DIR}" \
    --set image.repository=test-repo \
    --set image.tag=test-tag \
    --set secrets.managementToken.existingSecret.name=test-secret \
    --set dispatcher.enabled=true \
    --set dispatcher.service="helicone" \
    --set redis.external.addr="redis.example.com:6379" \
    --set env.LLM_PROXY_EVENT_BUS="redis-streams" 2>&1); then
    echo "✗ Validation should have failed for missing API key" >&2
    exit 1
else
    if echo "$TEMPLATE_OUTPUT" | grep -q "requires an API key"; then
        echo "✓ Validation correctly rejected missing API key"
    else
        echo "✗ Unexpected error message" >&2
        echo "$TEMPLATE_OUTPUT" >&2
        exit 1
    fi
fi
echo ""

# Clean up mock chart if we created it
if [ "$MOCK_CREATED" = true ]; then
    echo "Cleaning up mock postgresql subchart..."
    rm -rf "${MOCK_CHART_DIR}"
    echo "✓ Mock postgresql subchart removed"
    echo ""
fi

echo "✅ All Helm chart validations passed!"
