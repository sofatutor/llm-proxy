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
TEMPLATE_OUTPUT=$(helm template test-release "${CHART_DIR}" \
    --set image.repository=test-repo \
    --set image.tag=test-tag \
    --set env.LOG_LEVEL=debug 2>&1)
if [ $? -eq 0 ]; then
    echo "✓ helm template rendered successfully"
else
    echo "✗ helm template failed" >&2
    echo "$TEMPLATE_OUTPUT" >&2
    exit 1
fi
echo ""

# Run helm template with minimal required values only (no env block)
echo "Running helm template with minimal required values..."
TEMPLATE_OUTPUT=$(helm template test-release "${CHART_DIR}" \
    --set image.repository=test-repo \
    --set image.tag=test-tag 2>&1)
if [ $? -eq 0 ]; then
    echo "✓ helm template with minimal values rendered successfully"
else
    echo "✗ helm template with minimal values failed" >&2
    echo "$TEMPLATE_OUTPUT" >&2
    exit 1
fi
echo ""

# Run helm template with custom values to test more complex scenarios
echo "Running helm template with additional overrides..."
TEMPLATE_OUTPUT=$(helm template test-release "${CHART_DIR}" \
    --set replicaCount=3 \
    --set image.repository=custom-repo \
    --set image.tag=v1.0.0 \
    --set service.type=NodePort \
    --set service.port=9090 \
    --set resources.limits.cpu=2000m \
    --set resources.limits.memory=1Gi \
    --set env.LOG_LEVEL=trace \
    --set env.ENABLE_METRICS=false 2>&1)
if [ $? -eq 0 ]; then
    echo "✓ helm template with custom values rendered successfully"
else
    echo "✗ helm template with custom values failed" >&2
    echo "$TEMPLATE_OUTPUT" >&2
    exit 1
fi
echo ""

echo "✅ All Helm chart validations passed!"
