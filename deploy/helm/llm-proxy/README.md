# LLM Proxy Helm Chart

This Helm chart deploys the LLM Proxy server to Kubernetes.

## Prerequisites

- Kubernetes 1.19+
- Helm 3.0+

## Installation

### Required: Secret Management

The application requires a `MANAGEMENT_TOKEN` environment variable for admin operations. This chart supports two approaches:

#### Option 1: Reference an Existing Secret (RECOMMENDED for production)

Create a Kubernetes Secret separately:

```bash
kubectl create secret generic llm-proxy-secrets \
  --from-literal=MANAGEMENT_TOKEN="your-secure-random-token"
```

Then install the chart referencing this secret:

```bash
helm install llm-proxy deploy/helm/llm-proxy \
  --set image.repository=your-registry/llm-proxy \
  --set image.tag=v1.0.0 \
  --set secrets.managementToken.existingSecret.name=llm-proxy-secrets
```

#### Option 2: Chart-Managed Secret (NOT recommended for production)

For development/testing only, you can have the chart create the secret:

```bash
helm install llm-proxy deploy/helm/llm-proxy \
  --set image.repository=your-registry/llm-proxy \
  --set image.tag=v1.0.0 \
  --set secrets.create=true \
  --set-string secrets.data.managementToken="your-token"
```

**WARNING:** This approach stores the secret in Helm release history. Use only for development.

### Database Configuration

The chart supports two database backends:

#### SQLite (Default - Single Instance Only)

SQLite is the default and suitable for development or single-instance deployments:

```bash
helm install llm-proxy deploy/helm/llm-proxy \
  --set image.repository=your-registry/llm-proxy \
  --set image.tag=v1.0.0 \
  --set secrets.managementToken.existingSecret.name=llm-proxy-secrets \
  --set env.DB_DRIVER=sqlite
```

**Note:** SQLite does not support multi-replica deployments.

#### PostgreSQL

PostgreSQL is recommended for production and multi-replica deployments. Two modes are supported:

##### Option 1: External PostgreSQL (RECOMMENDED for production)

Use an existing PostgreSQL database:

1. Create a secret with your database connection string:

```bash
kubectl create secret generic llm-proxy-db \
  --from-literal=DATABASE_URL="postgres://user:password@host:5432/dbname?sslmode=require"
```

2. Install the chart with PostgreSQL configuration:

```bash
helm install llm-proxy deploy/helm/llm-proxy \
  --set image.repository=your-registry/llm-proxy \
  --set image.tag=v1.0.0 \
  --set secrets.managementToken.existingSecret.name=llm-proxy-secrets \
  --set secrets.databaseUrl.existingSecret.name=llm-proxy-db \
  --set env.DB_DRIVER=postgres
```

##### Option 2: In-Cluster PostgreSQL (Development/Testing Only)

Deploy PostgreSQL as part of the Helm release:

1. First, update chart dependencies to download the PostgreSQL subchart:

```bash
helm dependency update deploy/helm/llm-proxy
```

2. Install with in-cluster PostgreSQL:

```bash
helm install llm-proxy deploy/helm/llm-proxy \
  --set image.repository=your-registry/llm-proxy \
  --set image.tag=v1.0.0 \
  --set secrets.managementToken.existingSecret.name=llm-proxy-secrets \
  --set env.DB_DRIVER=postgres \
  --set postgresql.enabled=true \
  --set-string postgresql.auth.password="$(openssl rand -base64 32)"
```

**IMPORTANT:**
- In-cluster PostgreSQL is for development/testing only, NOT recommended for production
- Ensure your Docker image is built with PostgreSQL support using the `postgres` build tag
- Default images are built with: `docker build --build-arg POSTGRES_SUPPORT=true`
- You cannot use both in-cluster and external PostgreSQL simultaneously
- When using in-cluster PostgreSQL, data persists via a PersistentVolumeClaim (default 8Gi)

### Using External Redis

LLM Proxy uses Redis for:
- Event bus backend (redis-streams or redis)
- Optional HTTP cache backend
- Optional distributed rate limiting

**Important:** To use Redis, you must:
1. Set `redis.external.addr` to your Redis server address
2. Set `env.LLM_PROXY_EVENT_BUS` to `redis-streams` or `redis` (the chart defaults to `in-memory` for single-instance deployments)

To use an external Redis instance:

```bash
helm install llm-proxy deploy/helm/llm-proxy \
  --set image.repository=your-registry/llm-proxy \
  --set image.tag=v1.0.0 \
  --set secrets.managementToken.existingSecret.name=llm-proxy-secrets \
  --set redis.external.addr="redis.example.com:6379" \
  --set redis.external.db=0 \
  --set env.LLM_PROXY_EVENT_BUS="redis-streams"
```

**Note:** If your Redis instance requires authentication, create a secret with the password:

```bash
# Create password file with restricted permissions (more secure than command-line)
umask 077  # Ensure file is only readable by current user
echo -n "your-redis-password" > /tmp/redis-password.txt

kubectl create secret generic redis-password \
  --from-file=REDIS_PASSWORD=/tmp/redis-password.txt

# Clean up the temporary file
rm /tmp/redis-password.txt

helm install llm-proxy deploy/helm/llm-proxy \
  --set image.repository=your-registry/llm-proxy \
  --set image.tag=v1.0.0 \
  --set secrets.managementToken.existingSecret.name=llm-proxy-secrets \
  --set redis.external.addr="redis.example.com:6379" \
  --set redis.external.password.existingSecret.name=redis-password
```

### Using In-Memory Event Bus (Single Instance Only)

For development or single-instance deployments without Redis:

```bash
helm install llm-proxy deploy/helm/llm-proxy \
  --set image.repository=your-registry/llm-proxy \
  --set image.tag=v1.0.0 \
  --set secrets.managementToken.existingSecret.name=llm-proxy-secrets \
  --set env.LLM_PROXY_EVENT_BUS="in-memory"
```

**WARNING:** The in-memory event bus does not support multi-instance deployments. Use Redis for production environments with multiple replicas.

## Configuration

### Image Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `image.repository` | Container image repository | `llm-proxy` |
| `image.tag` | Container image tag | `""` (uses chart appVersion) |
| `image.pullPolicy` | Image pull policy | `IfNotPresent` |

### Service Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `service.type` | Kubernetes Service type | `ClusterIP` |
| `service.port` | Service port | `8080` |
| `service.targetPort` | Container port | `8080` |

### Resource Limits

| Parameter | Description | Default |
|-----------|-------------|---------|
| `resources.limits.cpu` | CPU limit | `1000m` |
| `resources.limits.memory` | Memory limit | `512Mi` |
| `resources.requests.cpu` | CPU request | `100m` |
| `resources.requests.memory` | Memory request | `128Mi` |

### Environment Variables

Configure non-sensitive environment variables via the `env` map in values.yaml:

```yaml
env:
  LOG_LEVEL: "info"
  LOG_FORMAT: "json"
  ENABLE_METRICS: "true"
  DB_DRIVER: "sqlite"  # or "postgres" for external PostgreSQL
  LLM_PROXY_EVENT_BUS: "in-memory"  # Default. Use "redis-streams" or "redis" when Redis is configured
```

### PostgreSQL Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `postgresql.enabled` | Enable in-cluster PostgreSQL (development/testing only) | `false` |
| `postgresql.auth.username` | PostgreSQL username | `llmproxy` |
| `postgresql.auth.database` | PostgreSQL database name | `llmproxy` |
| `postgresql.auth.password` | PostgreSQL password (use --set-string for security) | `""` |
| `postgresql.auth.existingSecret` | Existing Secret containing PostgreSQL password | `""` |
| `postgresql.primary.persistence.enabled` | Enable persistence for PostgreSQL | `true` |
| `postgresql.primary.persistence.size` | Size of PostgreSQL PersistentVolumeClaim | `8Gi` |
| `postgresql.primary.resources.limits.memory` | PostgreSQL memory limit | `256Mi` |
| `postgresql.primary.resources.limits.cpu` | PostgreSQL CPU limit | `500m` |
| `postgresql.primary.resources.requests.memory` | PostgreSQL memory request | `128Mi` |
| `postgresql.primary.resources.requests.cpu` | PostgreSQL CPU request | `100m` |

The chart supports the following PostgreSQL configurations:

#### External PostgreSQL (Recommended for Production)

```yaml
env:
  DB_DRIVER: "postgres"
secrets:
  databaseUrl:
    existingSecret:
      name: "llm-proxy-db"
      key: "DATABASE_URL"
```

#### In-Cluster PostgreSQL (Development/Testing Only)

**IMPORTANT:** Run `helm dependency update deploy/helm/llm-proxy` first to download the PostgreSQL subchart.

```yaml
env:
  DB_DRIVER: "postgres"
postgresql:
  enabled: true
  auth:
    username: llmproxy
    database: llmproxy
    password: "your-secure-password"  # Use --set-string in practice
  primary:
    persistence:
      enabled: true
      size: 8Gi
```

**Note:** Ensure your Docker image is built with PostgreSQL support (postgres build tag).

### Redis Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `redis.enabled` | Reserved for future use; currently has no effect and does not deploy Redis | `false` |
| `redis.external.addr` | External Redis server address (e.g., `redis.example.com:6379`) | `""` |
| `redis.external.db` | Redis database number | `0` |
| `redis.external.password.existingSecret.name` | Name of existing Secret containing Redis password | `""` |
| `redis.external.password.existingSecret.key` | Key within the Secret for Redis password | `"REDIS_PASSWORD"` |

The chart supports the following Redis configurations:

#### External Redis (Recommended for Production)

```yaml
redis:
  external:
    addr: "redis.example.com:6379"
    db: 0
    password:
      existingSecret:
        name: "redis-password"
        key: "REDIS_PASSWORD"
env:
  LLM_PROXY_EVENT_BUS: "redis-streams"
```

#### In-Memory (Single Instance Only)

```yaml
env:
  LLM_PROXY_EVENT_BUS: "in-memory"
# No redis configuration needed
```

### Secret Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `secrets.create` | Create a Secret managed by the chart (NOT recommended for production) | `false` |
| `secrets.data.managementToken` | Management token value (only if `secrets.create=true`) | `""` |
| `secrets.data.databaseUrl` | Database URL value (only if `secrets.create=true`) | `""` |
| `secrets.managementToken.existingSecret.name` | Name of existing Secret containing MANAGEMENT_TOKEN | `""` |
| `secrets.managementToken.existingSecret.key` | Key within the Secret for management token | `"MANAGEMENT_TOKEN"` |
| `secrets.databaseUrl.existingSecret.name` | Name of existing Secret containing DATABASE_URL (for external PostgreSQL) | `""` |
| `secrets.databaseUrl.existingSecret.key` | Key within the Secret for database URL | `"DATABASE_URL"` |

## Health Checks

The chart configures health probes with dedicated endpoints:
- **Liveness probe** (`/live`): Checks if the application is running
- **Readiness probe** (`/ready`): Checks if the application is ready to serve traffic

Both probes can be customized via `livenessProbe` and `readinessProbe` in values.yaml.

## Security Best Practices

### Secret Management

- **Never store secrets in `values.yaml` or version control**
- Always use existing Kubernetes Secrets and reference them in the chart configuration
- For production, use external secret management tools like:
  - [External Secrets Operator](https://external-secrets.io/)
  - [Sealed Secrets](https://github.com/bitnami-labs/sealed-secrets)
  - Cloud provider secret managers (AWS Secrets Manager, Azure Key Vault, GCP Secret Manager)

### Generating Secure Tokens

Generate a strong management token using:

```bash
# Using openssl (Linux/macOS)
openssl rand -base64 32

# Using /dev/urandom (Linux)
head -c 32 /dev/urandom | base64
```

### PostgreSQL Connection Security

When using external PostgreSQL:
- Always use `sslmode=require` or `sslmode=verify-full` in the connection string
- Use strong, unique passwords
- Restrict database access to specific IP ranges or use private networking
- Enable connection pooling with appropriate limits

Example secure connection string:
```
postgres://username:password@postgres.example.com:5432/llmproxy?sslmode=verify-full
```

## Validation

The chart includes validation that runs in CI to ensure it lints and renders correctly:

```bash
# Run validation locally
./scripts/validate-helm-chart.sh
```

This script performs:
- `helm lint` to check for chart issues
- `helm template` with various value overrides to validate rendering

## Uninstalling

```bash
helm uninstall llm-proxy
```
