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

## Example 2: Development Deployment with In-Cluster PostgreSQL

**WARNING:** In-cluster PostgreSQL is for development/testing only. Use external PostgreSQL for production.

### Step 1: Update Chart Dependencies

```bash
# Download the PostgreSQL subchart
helm dependency update deploy/helm/llm-proxy
```

### Step 2: Create Management Token Secret

```bash
# Create management token secret
kubectl create secret generic llm-proxy-secrets \
  --from-literal=MANAGEMENT_TOKEN="$(openssl rand -base64 32)"
```

### Step 3: Deploy with In-Cluster PostgreSQL

```bash
# Deploy with in-cluster PostgreSQL
helm install llm-proxy-dev deploy/helm/llm-proxy \
  --set image.repository=llm-proxy \
  --set image.tag=latest \
  --set secrets.managementToken.existingSecret.name=llm-proxy-secrets \
  --set env.DB_DRIVER=postgres \
  --set postgresql.enabled=true \
  --set-string postgresql.auth.password="$(openssl rand -base64 32)"
```

### Step 4: Verify Deployment

```bash
# Check pod status (should see both llm-proxy and postgresql pods)
kubectl get pods -l app.kubernetes.io/instance=llm-proxy-dev

# Check PostgreSQL pod
kubectl get pods -l app.kubernetes.io/name=postgresql

# Check PostgreSQL service
kubectl get svc -l app.kubernetes.io/name=postgresql

# View logs
kubectl logs -l app.kubernetes.io/name=llm-proxy
kubectl logs -l app.kubernetes.io/name=postgresql

# Check PersistentVolumeClaim for PostgreSQL
kubectl get pvc
```

**IMPORTANT:**
- Ensure your Docker image is built with PostgreSQL support (postgres build tag)
- Default images are built with: `docker build --build-arg POSTGRES_SUPPORT=true`
- Data persists via PersistentVolumeClaim (default 8Gi)

## Example 3: Development Deployment with Chart-Managed Secret

**WARNING:** This approach stores secrets in Helm release history. Use only for development/testing.

```bash
# Generate a random token
MGMT_TOKEN=$(openssl rand -base64 32)

# Deploy with chart-managed secret
helm install llm-proxy-simple deploy/helm/llm-proxy \
  --set image.repository=llm-proxy \
  --set image.tag=latest \
  --set secrets.create=true \
  --set-string secrets.data.managementToken="${MGMT_TOKEN}"
```

## Example 4: Using External Secret Operator

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

## Example 5: SQLite with Single Secret (Simple Install)

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

## Example 6: Using Different Secret Keys

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

## Example 7: Production Values File

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

## Example 8: External Redis for Event Bus and Caching

For production deployments using Redis for event bus and optional caching:

### Step 1: Create Secrets

```bash
# Create management token secret using file-based approach
openssl rand -base64 32 > /tmp/mgmt-token.txt
kubectl create secret generic llm-proxy-secrets \
  --from-file=MANAGEMENT_TOKEN=/tmp/mgmt-token.txt
rm /tmp/mgmt-token.txt

# Create Redis password secret (if your Redis requires authentication)
# Use file-based approach to avoid exposing password in shell history
echo -n "your-redis-password" > /tmp/redis-password.txt
kubectl create secret generic redis-password \
  --from-file=REDIS_PASSWORD=/tmp/redis-password.txt
rm /tmp/redis-password.txt
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

# Verify environment variables are set correctly (requires jq)
kubectl get deployment llm-proxy -o jsonpath='{.spec.template.spec.containers[0].env}' | jq '.[] | select(.name | startswith("REDIS"))'

# Alternative without jq:
kubectl get deployment llm-proxy -o yaml | grep -A 2 "REDIS"
```

## Example 9: Multi-Instance Deployment with Redis

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

## Example 10: Development with In-Memory Event Bus (Single Instance)

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

**SECURITY WARNING:** 
- This configuration uses chart-managed secrets stored in Helm release history (insecure)
- The `--set-string` approach passes the token via command line (may appear in shell history and process listings)
- In-memory event bus does not support multiple replicas
- **Use only for development/testing environments**
- For production, always use existing Kubernetes Secrets created via file-based approach

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

## Example 11: Dispatcher with File Backend

Deploy the dispatcher to store events in a file:

### Step 1: Create Secrets

```bash
# Create management token secret
kubectl create secret generic llm-proxy-secrets \
  --from-literal=MANAGEMENT_TOKEN="$(openssl rand -base64 32)"
```

### Step 2: Deploy with Dispatcher

```bash
helm install llm-proxy deploy/helm/llm-proxy \
  --set image.repository=your-registry/llm-proxy \
  --set image.tag=v1.0.0 \
  --set secrets.managementToken.existingSecret.name=llm-proxy-secrets \
  --set redis.external.addr="redis.example.com:6379" \
  --set env.LLM_PROXY_EVENT_BUS="redis-streams" \
  --set dispatcher.enabled=true \
  --set dispatcher.service="file" \
  --set dispatcher.persistence.size=20Gi
```

### Step 3: Verify Dispatcher

```bash
# Check dispatcher pod status
kubectl get pods -l app.kubernetes.io/component=dispatcher

# Check dispatcher logs
kubectl logs -l app.kubernetes.io/component=dispatcher

# Verify PVC was created
kubectl get pvc
```

## Example 12: Dispatcher with Lunary Integration

Deploy the dispatcher to forward events to Lunary for LLM observability:

### Step 1: Create Secrets

```bash
# Create management token secret
kubectl create secret generic llm-proxy-secrets \
  --from-literal=MANAGEMENT_TOKEN="$(openssl rand -base64 32)"

# Create dispatcher API key secret (file-based for security)
umask 077
echo -n "your-lunary-api-key" > /tmp/lunary-api-key.txt
kubectl create secret generic dispatcher-secrets \
  --from-file=LUNARY_API_KEY=/tmp/lunary-api-key.txt
rm /tmp/lunary-api-key.txt
```

### Step 2: Deploy with Lunary Dispatcher

```bash
helm install llm-proxy deploy/helm/llm-proxy \
  --set image.repository=your-registry/llm-proxy \
  --set image.tag=v1.0.0 \
  --set secrets.managementToken.existingSecret.name=llm-proxy-secrets \
  --set redis.external.addr="redis.example.com:6379" \
  --set env.LLM_PROXY_EVENT_BUS="redis-streams" \
  --set dispatcher.enabled=true \
  --set dispatcher.service="lunary" \
  --set dispatcher.apiKey.existingSecret.name="dispatcher-secrets" \
  --set dispatcher.apiKey.existingSecret.key="LUNARY_API_KEY"
```

### Step 3: Verify Dispatcher

```bash
# Check dispatcher status
kubectl get pods -l app.kubernetes.io/component=dispatcher

# View dispatcher logs
kubectl logs -l app.kubernetes.io/component=dispatcher -f

# Check environment variables (structure only, no values)
kubectl get deployment -o yaml | grep -A 10 "name: dispatcher"
```

## Example 13: Dispatcher with Helicone Integration

Deploy the dispatcher to forward events to Helicone for LLM analytics:

### Step 1: Create Secrets

```bash
# Create management token secret
kubectl create secret generic llm-proxy-secrets \
  --from-literal=MANAGEMENT_TOKEN="$(openssl rand -base64 32)"

# Create dispatcher API key secret
umask 077
echo -n "your-helicone-api-key" > /tmp/helicone-api-key.txt
kubectl create secret generic dispatcher-secrets \
  --from-file=DISPATCHER_API_KEY=/tmp/helicone-api-key.txt
rm /tmp/helicone-api-key.txt
```

### Step 2: Deploy with Helicone Dispatcher

```bash
helm install llm-proxy deploy/helm/llm-proxy \
  --set image.repository=your-registry/llm-proxy \
  --set image.tag=v1.0.0 \
  --set secrets.managementToken.existingSecret.name=llm-proxy-secrets \
  --set redis.external.addr="redis.example.com:6379" \
  --set env.LLM_PROXY_EVENT_BUS="redis-streams" \
  --set dispatcher.enabled=true \
  --set dispatcher.service="helicone" \
  --set dispatcher.apiKey.existingSecret.name="dispatcher-secrets"
```

## Example 14: Production Deployment with Multi-Replica Dispatcher

Deploy with PostgreSQL, Redis, and dispatcher for full observability:

### Step 1: Create Secrets

```bash
# Create management token
kubectl create secret generic llm-proxy-secrets \
  --from-literal=MANAGEMENT_TOKEN="$(openssl rand -base64 32)"

# Create database URL
kubectl create secret generic llm-proxy-db \
  --from-literal=DATABASE_URL="postgres://user:pass@postgres.example.com:5432/llmproxy?sslmode=require"

# Create Redis password
echo -n "your-redis-password" > /tmp/redis-password.txt
kubectl create secret generic redis-password \
  --from-file=REDIS_PASSWORD=/tmp/redis-password.txt
rm /tmp/redis-password.txt

# Create dispatcher API key for Lunary
echo -n "your-lunary-api-key" > /tmp/lunary-api-key.txt
kubectl create secret generic dispatcher-secrets \
  --from-file=LUNARY_API_KEY=/tmp/lunary-api-key.txt
rm /tmp/lunary-api-key.txt
```

### Step 2: Create Production Values File

```yaml
# production-with-dispatcher.yaml
image:
  repository: your-registry/llm-proxy
  tag: v1.0.0
  pullPolicy: IfNotPresent

replicaCount: 3

secrets:
  managementToken:
    existingSecret:
      name: llm-proxy-secrets
  databaseUrl:
    existingSecret:
      name: llm-proxy-db

env:
  DB_DRIVER: "postgres"
  LOG_LEVEL: "info"
  LOG_FORMAT: "json"
  ENABLE_METRICS: "true"
  LLM_PROXY_EVENT_BUS: "redis-streams"

redis:
  external:
    addr: "redis.example.com:6379"
    db: 0
    password:
      existingSecret:
        name: redis-password

dispatcher:
  enabled: true
  replicaCount: 2
  service: "lunary"
  apiKey:
    existingSecret:
      name: "dispatcher-secrets"
      key: "LUNARY_API_KEY"
  config:
    bufferSize: 2000
    batchSize: 200
  resources:
    limits:
      cpu: 1000m
      memory: 512Mi
    requests:
      cpu: 200m
      memory: 256Mi

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

### Step 3: Deploy

```bash
helm install llm-proxy deploy/helm/llm-proxy -f production-with-dispatcher.yaml
```

### Step 4: Verify Deployment

```bash
# Check all pods
kubectl get pods -l app.kubernetes.io/instance=llm-proxy

# Check main proxy pods
kubectl get pods -l app.kubernetes.io/name=llm-proxy

# Check dispatcher pods
kubectl get pods -l app.kubernetes.io/component=dispatcher

# View dispatcher logs
kubectl logs -l app.kubernetes.io/component=dispatcher --tail=50

# Check services
kubectl get svc
```

## Uninstalling

```bash
# Uninstall the Helm release
helm uninstall llm-proxy

# Optionally, delete the secrets (if they were created for this deployment)
kubectl delete secret llm-proxy-secrets llm-proxy-db dispatcher-secrets redis-password
```
