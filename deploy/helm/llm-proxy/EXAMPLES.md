# Helm Chart Examples

This document provides practical examples for deploying the LLM Proxy using the Helm chart with different secret configurations.

## Prerequisites

```bash
# Ensure Helm is installed
helm version

# Ensure kubectl is configured
kubectl cluster-info

# Optional: jq for JSON parsing in troubleshooting commands
jq --version
```

## Example 1: Production Deployment with Existing Secrets (Recommended)

### Step 1: Create Kubernetes Secrets

```bash
# Create management token secret
kubectl create secret generic llm-proxy-secrets \
  --from-literal=MANAGEMENT_TOKEN="$(openssl rand -base64 32)"

# Create database secret for PostgreSQL
kubectl create secret generic llm-proxy-db \
  --from-literal=DATABASE_URL="postgres://llmproxy:STRONG_DB_PASSWORD_HERE@postgres.example.com:5432/llmproxy?sslmode=require"
```

### Step 2: Deploy with Helm

```bash
helm install llm-proxy deploy/helm/llm-proxy \
  --set image.repository=your-registry/llm-proxy \
  --set image.tag=v1.0.0 \
  --set secrets.managementToken.existingSecret.name=llm-proxy-secrets \
  --set secrets.databaseUrl.existingSecret.name=llm-proxy-db \
  --set env.DB_DRIVER=postgres
```

### Step 3: Verify Deployment

```bash
# Check pod status
kubectl get pods -l app.kubernetes.io/name=llm-proxy

# Check pod logs
kubectl logs -l app.kubernetes.io/name=llm-proxy

# Verify secret references
kubectl get deployment -o yaml | grep -A 3 secretKeyRef
```

## Example 2: Development Deployment with Chart-Managed Secret

**WARNING:** This approach stores secrets in Helm release history. Use only for development/testing.

```bash
# Generate a random token
MGMT_TOKEN=$(openssl rand -base64 32)

# Deploy with chart-managed secret
helm install llm-proxy-dev deploy/helm/llm-proxy \
  --set image.repository=llm-proxy \
  --set image.tag=latest \
  --set secrets.create=true \
  --set-string secrets.data.managementToken="${MGMT_TOKEN}"
```

## Example 3: Using External Secret Operator

If you're using [External Secrets Operator](https://external-secrets.io/):

### Step 1: Create ExternalSecret Resource

```yaml
apiVersion: external-secrets.io/v1beta1
kind: ExternalSecret
metadata:
  name: llm-proxy-secrets
spec:
  refreshInterval: 1h
  secretStoreRef:
    name: aws-secrets-manager  # or your secret store
    kind: SecretStore
  target:
    name: llm-proxy-secrets
    creationPolicy: Owner
  data:
    - secretKey: MANAGEMENT_TOKEN
      remoteRef:
        key: llm-proxy/management-token
    - secretKey: DATABASE_URL
      remoteRef:
        key: llm-proxy/database-url
```

### Step 2: Deploy with Helm

```bash
# Wait for ExternalSecret to create the secret
kubectl wait --for=condition=Ready externalsecret/llm-proxy-secrets --timeout=60s

# Deploy the Helm chart
helm install llm-proxy deploy/helm/llm-proxy \
  --set image.repository=your-registry/llm-proxy \
  --set image.tag=v1.0.0 \
  --set secrets.managementToken.existingSecret.name=llm-proxy-secrets \
  --set secrets.databaseUrl.existingSecret.name=llm-proxy-secrets \
  --set env.DB_DRIVER=postgres
```

## Example 4: SQLite with Single Secret (Simple Install)

For single-instance deployments using SQLite:

```bash
# Create only the management token secret
kubectl create secret generic llm-proxy-secrets \
  --from-literal=MANAGEMENT_TOKEN="$(openssl rand -base64 32)"

# Deploy with SQLite (default)
helm install llm-proxy deploy/helm/llm-proxy \
  --set image.repository=your-registry/llm-proxy \
  --set image.tag=v1.0.0 \
  --set secrets.managementToken.existingSecret.name=llm-proxy-secrets \
  --set env.DB_DRIVER=sqlite
```

## Example 5: Using Different Secret Keys

If your existing secret uses different key names:

```bash
# Your existing secret has keys named differently
# For example: TOKEN and DB_CONN_STRING

helm install llm-proxy deploy/helm/llm-proxy \
  --set image.repository=your-registry/llm-proxy \
  --set image.tag=v1.0.0 \
  --set secrets.managementToken.existingSecret.name=my-secrets \
  --set secrets.managementToken.existingSecret.key=TOKEN \
  --set secrets.databaseUrl.existingSecret.name=my-secrets \
  --set secrets.databaseUrl.existingSecret.key=DB_CONN_STRING \
  --set env.DB_DRIVER=postgres
```

## Example 6: Production Values File

Create a `production-values.yaml` file:

```yaml
image:
  repository: your-registry/llm-proxy
  tag: v1.0.0
  pullPolicy: IfNotPresent

replicaCount: 3

secrets:
  managementToken:
    existingSecret:
      name: llm-proxy-secrets
      key: MANAGEMENT_TOKEN
  databaseUrl:
    existingSecret:
      name: llm-proxy-db
      key: DATABASE_URL

env:
  DB_DRIVER: "postgres"
  LOG_LEVEL: "info"
  LOG_FORMAT: "json"
  ENABLE_METRICS: "true"

resources:
  limits:
    cpu: 2000m
    memory: 1Gi
  requests:
    cpu: 500m
    memory: 256Mi

affinity:
  podAntiAffinity:
    preferredDuringSchedulingIgnoredDuringExecution:
    - weight: 100
      podAffinityTerm:
        labelSelector:
          matchExpressions:
          - key: app.kubernetes.io/name
            operator: In
            values:
            - llm-proxy
        topologyKey: kubernetes.io/hostname
```

Deploy with the values file:

```bash
helm install llm-proxy deploy/helm/llm-proxy -f production-values.yaml
```

## Example 7: External Redis for Event Bus and Caching

For production deployments using Redis for event bus and optional caching:

### Step 1: Create Secrets

```bash
# Create management token secret
kubectl create secret generic llm-proxy-secrets \
  --from-literal=MANAGEMENT_TOKEN="$(openssl rand -base64 32)"

# Create Redis password secret (if your Redis requires authentication)
kubectl create secret generic redis-password \
  --from-literal=REDIS_PASSWORD="your-redis-password"
```

### Step 2: Deploy with Redis Configuration

```bash
helm install llm-proxy deploy/helm/llm-proxy \
  --set image.repository=your-registry/llm-proxy \
  --set image.tag=v1.0.0 \
  --set secrets.managementToken.existingSecret.name=llm-proxy-secrets \
  --set redis.external.addr="redis.example.com:6379" \
  --set redis.external.db=0 \
  --set redis.external.password.existingSecret.name=redis-password \
  --set env.LLM_PROXY_EVENT_BUS="redis-streams"
```

### Step 3: Verify Redis Connection

```bash
# Check pod logs for Redis connection messages
kubectl logs -l app.kubernetes.io/name=llm-proxy | grep -i redis

# Verify environment variables are set correctly
kubectl get deployment llm-proxy -o jsonpath='{.spec.template.spec.containers[0].env}' | jq '.[] | select(.name | startswith("REDIS"))'
```

## Example 8: Multi-Instance Deployment with Redis

For scaling with multiple replicas (requires Redis for event bus):

```yaml
# redis-scaling-values.yaml
image:
  repository: your-registry/llm-proxy
  tag: v1.0.0
  pullPolicy: IfNotPresent

replicaCount: 3

secrets:
  managementToken:
    existingSecret:
      name: llm-proxy-secrets

redis:
  external:
    addr: "redis.example.com:6379"
    db: 0
    password:
      existingSecret:
        name: redis-password

env:
  DB_DRIVER: "postgres"
  LOG_LEVEL: "info"
  LOG_FORMAT: "json"
  LLM_PROXY_EVENT_BUS: "redis-streams"  # Required for multi-instance

resources:
  limits:
    cpu: 1000m
    memory: 512Mi
  requests:
    cpu: 200m
    memory: 256Mi
```

Deploy with:

```bash
helm install llm-proxy deploy/helm/llm-proxy -f redis-scaling-values.yaml \
  --set secrets.databaseUrl.existingSecret.name=llm-proxy-db
```

## Example 9: Development with In-Memory Event Bus (Single Instance)

For local development or testing without Redis:

```bash
helm install llm-proxy-dev deploy/helm/llm-proxy \
  --set image.repository=llm-proxy \
  --set image.tag=latest \
  --set secrets.create=true \
  --set-string secrets.data.managementToken="$(openssl rand -base64 32)" \
  --set env.LLM_PROXY_EVENT_BUS="in-memory" \
  --set env.DB_DRIVER="sqlite"
```

**WARNING:** 
- This configuration uses in-memory event bus, which does not support multiple replicas
- Chart-managed secrets are stored in Helm release history
- Use only for development/testing environments

## Upgrading

To upgrade an existing deployment:

```bash
# Upgrade with new image version
helm upgrade llm-proxy deploy/helm/llm-proxy \
  --reuse-values \
  --set image.tag=v1.1.0

# Upgrade with new values file
helm upgrade llm-proxy deploy/helm/llm-proxy -f production-values.yaml
```

## Troubleshooting

### Check if secrets are properly referenced

```bash
# Verify deployment environment variables (structure only, no values)
kubectl get deployment llm-proxy -o jsonpath='{.spec.template.spec.containers[0].env}' | jq

# Check if secret exists
kubectl get secret llm-proxy-secrets -o yaml

# Verify pod environment configuration (shows secretKeyRef, not actual values)
kubectl get deployment llm-proxy -o jsonpath='{.spec.template.spec.containers[0].env[?(@.name=="MANAGEMENT_TOKEN")]}' | jq
```

### Validate secret before deployment

```bash
# Dry-run to see rendered manifests
helm install llm-proxy deploy/helm/llm-proxy \
  --dry-run \
  --debug \
  --set secrets.managementToken.existingSecret.name=llm-proxy-secrets
```

## Uninstalling

```bash
# Uninstall the Helm release
helm uninstall llm-proxy

# Optionally, delete the secrets (if they were created for this deployment)
kubectl delete secret llm-proxy-secrets llm-proxy-db
```
