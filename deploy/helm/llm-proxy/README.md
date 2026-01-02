# LLM Proxy Helm Chart

This Helm chart deploys the LLM Proxy server to Kubernetes.

## Installation from OCI Registry (Recommended)

The LLM Proxy Helm chart is published as an OCI artifact to GitHub Container Registry (GHCR). You can install it directly without cloning the repository:

```bash
# Create required secrets
kubectl create secret generic llm-proxy-secrets \
  --from-literal=MANAGEMENT_TOKEN="$(openssl rand -base64 32)"

# Install from OCI registry (replace <version> with actual release, e.g., 1.0.0)
helm install llm-proxy oci://ghcr.io/sofatutor/charts/llm-proxy \
  --version <version> \
  --set image.repository=ghcr.io/sofatutor/llm-proxy \
  --set image.tag=latest \
  --set secrets.managementToken.existingSecret.name=llm-proxy-secrets
```

**Available versions:** See [GitHub Container Registry](https://github.com/sofatutor/llm-proxy/pkgs/container/llm-proxy) for all published chart versions.

**Note on Dependencies:** The chart includes an optional PostgreSQL dependency (disabled by default). OCI charts are published with dependencies included. If you enable `postgresql.enabled=true`, the PostgreSQL subchart will be available from the packaged chart.

## Installation from Local Chart

If you have cloned the repository, you can install the chart locally:

### SQLite (Default - Single Instance)

```bash
kubectl create secret generic llm-proxy-secrets \
  --from-literal=MANAGEMENT_TOKEN="$(openssl rand -base64 32)"

helm install llm-proxy deploy/helm/llm-proxy \
  --set image.repository=ghcr.io/sofatutor/llm-proxy \
  --set image.tag=latest \
  --set secrets.managementToken.existingSecret.name=llm-proxy-secrets
```

### PostgreSQL (Production - External)

```bash
kubectl create secret generic llm-proxy-secrets \
  --from-literal=MANAGEMENT_TOKEN="$(openssl rand -base64 32)"

kubectl create secret generic llm-proxy-db \
  --from-literal=DATABASE_URL="postgres://user:pass@host:5432/db?sslmode=require"

helm install llm-proxy deploy/helm/llm-proxy \
  --set image.repository=ghcr.io/sofatutor/llm-proxy \
  --set image.tag=latest \
  --set secrets.managementToken.existingSecret.name=llm-proxy-secrets \
  --set secrets.databaseUrl.existingSecret.name=llm-proxy-db \
  --set env.DB_DRIVER=postgres
```

Or using the OCI registry:

```bash
helm install llm-proxy oci://ghcr.io/sofatutor/charts/llm-proxy --version <version> \
  --set image.repository=ghcr.io/sofatutor/llm-proxy \
  --set image.tag=latest \
  --set secrets.managementToken.existingSecret.name=llm-proxy-secrets \
  --set secrets.databaseUrl.existingSecret.name=llm-proxy-db \
  --set env.DB_DRIVER=postgres
```

### PostgreSQL (Development - In-Cluster)

**Note:** When installing from the OCI registry, the chart is published with dependencies included. The `helm dependency update` step below is only required when installing the chart from a local checkout.

```bash
# If using local chart, download PostgreSQL subchart
helm dependency update deploy/helm/llm-proxy

kubectl create secret generic llm-proxy-secrets \
  --from-literal=MANAGEMENT_TOKEN="$(openssl rand -base64 32)"

helm install llm-proxy deploy/helm/llm-proxy \
  --set image.repository=ghcr.io/sofatutor/llm-proxy \
  --set image.tag=latest \
  --set secrets.managementToken.existingSecret.name=llm-proxy-secrets \
  --set env.DB_DRIVER=postgres \
  --set postgresql.enabled=true \
  --set-string postgresql.auth.password="$(openssl rand -base64 32)"
```

**IMPORTANT:** Ensure your Docker image is built with PostgreSQL support:
```bash
docker build --build-arg POSTGRES_SUPPORT=true -t ghcr.io/sofatutor/llm-proxy:latest .
```

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
  --set image.repository=ghcr.io/sofatutor/llm-proxy \
  --set image.tag=latest \
  --set secrets.managementToken.existingSecret.name=llm-proxy-secrets
```

#### Option 2: Chart-Managed Secret (NOT recommended for production)

For development/testing only, you can have the chart create the secret:

```bash
helm install llm-proxy deploy/helm/llm-proxy \
  --set image.repository=ghcr.io/sofatutor/llm-proxy \
  --set image.tag=latest \
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
  --set image.repository=ghcr.io/sofatutor/llm-proxy \
  --set image.tag=latest \
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
  --set image.repository=ghcr.io/sofatutor/llm-proxy \
  --set image.tag=latest \
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
  --set image.repository=ghcr.io/sofatutor/llm-proxy \
  --set image.tag=latest \
  --set secrets.managementToken.existingSecret.name=llm-proxy-secrets \
  --set env.DB_DRIVER=postgres \
  --set postgresql.enabled=true \
  --set-string postgresql.auth.password="$(openssl rand -base64 32)"
```

**IMPORTANT:**
- **⚠️ For production high-availability (HA) deployments, use an external managed PostgreSQL service:**
  - Amazon Aurora PostgreSQL / Amazon RDS for PostgreSQL
  - Google Cloud SQL for PostgreSQL
  - Azure Database for PostgreSQL
  - Self-managed PostgreSQL with replication
- The in-cluster PostgreSQL StatefulSet is hardcoded to `replicas: 1` and is suitable for **development/testing only**
- It does not provide high availability or automatic failover
- Ensure your Docker image is built with PostgreSQL support using the `postgres` build tag
- Default images are built with: `docker build --build-arg POSTGRES_SUPPORT=true`
- You cannot use both in-cluster and external PostgreSQL simultaneously
- When using in-cluster PostgreSQL, data persists via a PersistentVolumeClaim (default 8Gi)

#### MySQL

MySQL is also supported for production and multi-replica deployments. Two modes are supported:

##### Option 1: External MySQL (RECOMMENDED for production)

Use an existing MySQL database:

1. Create a secret with your database connection string:

```bash
kubectl create secret generic llm-proxy-db \
  --from-literal=DATABASE_URL="user:password@tcp(host:3306)/dbname?parseTime=true&tls=true"
```

2. Install the chart with MySQL configuration:

```bash
helm install llm-proxy deploy/helm/llm-proxy \
  --set image.repository=ghcr.io/sofatutor/llm-proxy \
  --set image.tag=latest \
  --set secrets.managementToken.existingSecret.name=llm-proxy-secrets \
  --set secrets.databaseUrl.existingSecret.name=llm-proxy-db \
  --set env.DB_DRIVER=mysql
```

##### Option 2: In-Cluster MySQL (Development/Testing Only)

Deploy MySQL as part of the Helm release:

```bash
helm install llm-proxy deploy/helm/llm-proxy \
  --set image.repository=ghcr.io/sofatutor/llm-proxy \
  --set image.tag=latest \
  --set secrets.managementToken.existingSecret.name=llm-proxy-secrets \
  --set env.DB_DRIVER=mysql \
  --set mysql.enabled=true \
  --set-string mysql.auth.rootPassword="$(openssl rand -base64 32)" \
  --set-string mysql.auth.password="$(openssl rand -base64 32)"
```

Or using the example values file:

```bash
helm install llm-proxy deploy/helm/llm-proxy \
  -f deploy/helm/llm-proxy/examples/values-mysql.yaml \
  --set image.repository=ghcr.io/sofatutor/llm-proxy \
  --set image.tag=latest \
  --set-string mysql.auth.rootPassword="$(openssl rand -base64 32)" \
  --set-string mysql.auth.password="$(openssl rand -base64 32)"
```

**IMPORTANT:**
- **⚠️ For production high-availability (HA) deployments, use an external managed MySQL service:**
  - Amazon Aurora MySQL / Amazon RDS for MySQL
  - Google Cloud SQL for MySQL
  - Azure Database for MySQL
  - Self-managed MySQL Group Replication or InnoDB Cluster
- The in-cluster MySQL StatefulSet is hardcoded to `replicas: 1` and is suitable for **development/testing only**
- It does not provide high availability or automatic failover
- Ensure your Docker image is built with MySQL support using the `mysql` build tag
- Build command: `docker build --build-arg MYSQL_SUPPORT=true -t llm-proxy:mysql .`
- You cannot use both in-cluster and external MySQL simultaneously
- When using in-cluster MySQL, data persists via a PersistentVolumeClaim (default 10Gi)
- MySQL connection string format: `user:password@tcp(host:port)/database?parseTime=true`
- For production, enable TLS with `tls=true` or `tls=skip-verify` (not recommended)

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
  --set image.repository=ghcr.io/sofatutor/llm-proxy \
  --set image.tag=latest \
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
  --set image.repository=ghcr.io/sofatutor/llm-proxy \
  --set image.tag=latest \
  --set secrets.managementToken.existingSecret.name=llm-proxy-secrets \
  --set redis.external.addr="redis.example.com:6379" \
  --set redis.external.password.existingSecret.name=redis-password
```

### Using In-Memory Event Bus (Single Instance Only)

For development or single-instance deployments without Redis:

```bash
helm install llm-proxy deploy/helm/llm-proxy \
  --set image.repository=ghcr.io/sofatutor/llm-proxy \
  --set image.tag=latest \
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

### Ingress Configuration

The chart supports optional Ingress resource for external access with TLS support.

| Parameter | Description | Default |
|-----------|-------------|---------|
| `ingress.enabled` | Enable Ingress resource | `false` |
| `ingress.className` | Ingress class name (e.g., `nginx`, `traefik`) | `""` |
| `ingress.annotations` | Annotations for Ingress resource | `{}` |
| `ingress.hosts` | List of hosts and paths | See values.yaml |
| `ingress.tls` | TLS configuration for Ingress | `[]` |

Example with NGINX Ingress and cert-manager:

```bash
helm install llm-proxy deploy/helm/llm-proxy \
  --set image.repository=ghcr.io/sofatutor/llm-proxy \
  --set image.tag=latest \
  --set secrets.managementToken.existingSecret.name=llm-proxy-secrets \
  --set ingress.enabled=true \
  --set ingress.className=nginx \
  --set 'ingress.annotations.cert-manager\.io/cluster-issuer=letsencrypt-prod' \
  --set ingress.hosts[0].host=api.example.com \
  --set ingress.hosts[0].paths[0].path=/ \
  --set ingress.hosts[0].paths[0].pathType=Prefix \
  --set ingress.tls[0].secretName=llm-proxy-tls \
  --set ingress.tls[0].hosts[0]=api.example.com
```

Or via values.yaml:

```yaml
ingress:
  enabled: true
  className: nginx
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod
  hosts:
    - host: api.example.com
      paths:
        - path: /
          pathType: Prefix
  tls:
    - secretName: llm-proxy-tls
      hosts:
        - api.example.com
```

### Autoscaling Configuration

The chart supports Horizontal Pod Autoscaler (HPA) for automatic scaling based on CPU and memory metrics.

| Parameter | Description | Default |
|-----------|-------------|---------|
| `autoscaling.enabled` | Enable HPA | `false` |
| `autoscaling.minReplicas` | Minimum number of replicas | `1` |
| `autoscaling.maxReplicas` | Maximum number of replicas | `10` |
| `autoscaling.targetCPUUtilizationPercentage` | Target CPU utilization percentage | `80` |
| `autoscaling.targetMemoryUtilizationPercentage` | Target memory utilization percentage (optional) | unset |

**Note:** When HPA is enabled, the `replicaCount` value is ignored as HPA manages the replica count.

**Important:** For autoscaling to work properly:
- Your cluster must have metrics-server installed
- Resource requests must be properly configured (CPU/memory)
- When using SQLite (`DB_DRIVER=sqlite`), autoscaling is not recommended as SQLite doesn't support concurrent writes from multiple replicas
- For production autoscaling, use PostgreSQL (`DB_DRIVER=postgres`) with external or in-cluster database

Example with CPU-based autoscaling:

```bash
helm install llm-proxy deploy/helm/llm-proxy \
  --set image.repository=ghcr.io/sofatutor/llm-proxy \
  --set image.tag=latest \
  --set secrets.managementToken.existingSecret.name=llm-proxy-secrets \
  --set env.DB_DRIVER=postgres \
  --set secrets.databaseUrl.existingSecret.name=llm-proxy-db \
  --set autoscaling.enabled=true \
  --set autoscaling.minReplicas=2 \
  --set autoscaling.maxReplicas=20 \
  --set autoscaling.targetCPUUtilizationPercentage=75
```

Or via values.yaml with both CPU and memory targets:

```yaml
autoscaling:
  enabled: true
  minReplicas: 2
  maxReplicas: 20
  targetCPUUtilizationPercentage: 75
  targetMemoryUtilizationPercentage: 85
```

### Prometheus Metrics Configuration

The chart supports optional Prometheus metrics scraping in two modes:
1. **Vanilla Prometheus**: Scrape via Service annotations
2. **Prometheus Operator**: Scrape via ServiceMonitor CRD

| Parameter | Description | Default |
|-----------|-------------|---------|
| `metrics.enabled` | Enable Prometheus metrics scraping support | `false` |
| `metrics.path` | Metrics endpoint path | `/metrics/prometheus` |
| `metrics.portName` | Port name to scrape (references service port) | `http` |
| `metrics.annotations` | Prometheus scraping annotations for vanilla Prometheus | See values.yaml |
| `metrics.serviceMonitor.enabled` | Enable ServiceMonitor resource (requires Prometheus Operator) | `false` |
| `metrics.serviceMonitor.labels` | Additional labels for ServiceMonitor (e.g., for Prometheus discovery) | `{}` |
| `metrics.serviceMonitor.interval` | Scrape interval | `30s` |
| `metrics.serviceMonitor.scrapeTimeout` | Scrape timeout | `10s` |

**Note:** The application metrics endpoint is enabled by default via `ENABLE_METRICS=true` in the base `values.yaml`. The `metrics.*` values in this chart only configure Prometheus scraping of that existing endpoint.

#### Grafana Dashboard

A ready-to-import Grafana dashboard is available in the `dashboards/` directory:

- **Dashboard JSON**: [`dashboards/llm-proxy.json`](dashboards/llm-proxy.json)
- **Documentation**: See the [dashboards README](dashboards/README.md) for detailed import instructions

The dashboard provides comprehensive operational visibility including:
- Request rate, error rate, and uptime monitoring
- Cache performance metrics (hits, misses, bypass, stores)
- Memory usage and Go runtime metrics
- Garbage collection statistics

**Automatic provisioning via Grafana sidecar:**

If using the Grafana Helm chart with sidecar discovery enabled, you can automatically provision the dashboard:

```bash
helm install llm-proxy deploy/helm/llm-proxy \
  --set image.repository=ghcr.io/sofatutor/llm-proxy \
  --set image.tag=latest \
  --set secrets.managementToken.existingSecret.name=llm-proxy-secrets \
  --set metrics.enabled=true \
  --set metrics.grafanaDashboard.enabled=true
```

Or via values.yaml:

```yaml
metrics:
  enabled: true
  grafanaDashboard:
    enabled: true
    labels:
      grafana_dashboard: "1"  # Default label for Grafana sidecar
```

This creates a ConfigMap with the `grafana_dashboard: "1"` label, which the Grafana sidecar will automatically discover and import.

**Manual import:**

To manually import the dashboard into Grafana, see the [dashboards README](dashboards/README.md) for detailed instructions.

#### Vanilla Prometheus (Service Annotations)

For Prometheus instances without the Prometheus Operator:

```bash
helm install llm-proxy deploy/helm/llm-proxy \
  --set image.repository=ghcr.io/sofatutor/llm-proxy \
  --set image.tag=latest \
  --set secrets.managementToken.existingSecret.name=llm-proxy-secrets \
  --set metrics.enabled=true
```

Or via values.yaml:

```yaml
metrics:
  enabled: true
  path: "/metrics/prometheus"
  annotations:
    prometheus.io/scrape: "true"
    prometheus.io/path: "/metrics/prometheus"
    prometheus.io/port: "8080"
```

This adds standard Prometheus annotations to the Service, allowing Prometheus to auto-discover and scrape the metrics endpoint.

#### Prometheus Operator (ServiceMonitor)

For clusters with Prometheus Operator installed:

```bash
helm install llm-proxy deploy/helm/llm-proxy \
  --set image.repository=ghcr.io/sofatutor/llm-proxy \
  --set image.tag=latest \
  --set secrets.managementToken.existingSecret.name=llm-proxy-secrets \
  --set metrics.enabled=true \
  --set metrics.serviceMonitor.enabled=true \
  --set metrics.serviceMonitor.labels.prometheus=kube-prometheus
```

Or via values.yaml:

```yaml
metrics:
  enabled: true
  path: "/metrics/prometheus"
  serviceMonitor:
    enabled: true
    labels:
      prometheus: kube-prometheus  # Match your Prometheus selector
    interval: 30s
    scrapeTimeout: 10s
```

**Important:** The `labels` must match your Prometheus instance's `serviceMonitorSelector`. Common label examples:
- `prometheus: kube-prometheus` (kube-prometheus-stack)
- `release: prometheus` (Prometheus Operator)

#### Available Metrics

The `/metrics/prometheus` endpoint exposes:

**Application Metrics:**
- `llm_proxy_uptime_seconds` - Server uptime
- `llm_proxy_requests_total` - Total proxy requests
- `llm_proxy_errors_total` - Total proxy errors
- `llm_proxy_cache_hits_total` - Cache hits
- `llm_proxy_cache_misses_total` - Cache misses
- `llm_proxy_cache_bypass_total` - Cache bypasses
- `llm_proxy_cache_stores_total` - Cache stores

**Go Runtime Metrics:**
- `llm_proxy_goroutines` - Number of goroutines
- `llm_proxy_memory_heap_alloc_bytes` - Heap bytes allocated
- `llm_proxy_memory_total_alloc_bytes` - Total bytes allocated (cumulative)
- `llm_proxy_gc_runs_total` - Total GC runs
- And more...

See [Instrumentation Documentation](../../docs/observability/instrumentation.md#prometheus-metrics-endpoint) for complete metric list and example queries.

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
| `secrets.encryptionKey.existingSecret.name` | Name of existing Secret containing ENCRYPTION_KEY (strongly recommended for production) | `""` |
| `secrets.encryptionKey.existingSecret.key` | Key within the Secret for encryption key | `"ENCRYPTION_KEY"` |

#### Encryption Key Configuration (Recommended for Production)

The `ENCRYPTION_KEY` is used to encrypt API keys (AES-256-GCM) and hash tokens (SHA-256) stored in the database. Without this key, sensitive data is stored in plaintext.

**To enable encryption:**

1. Generate a secure encryption key:
```bash
openssl rand -base64 32
```

2. Create a Kubernetes Secret:
```bash
kubectl create secret generic llm-proxy-encryption \
  --from-literal=ENCRYPTION_KEY=$(openssl rand -base64 32)
```

3. Configure the chart to use it:
```bash
helm install llm-proxy deploy/helm/llm-proxy \
  --set image.repository=ghcr.io/sofatutor/llm-proxy \
  --set image.tag=latest \
  --set secrets.managementToken.existingSecret.name=llm-proxy-secrets \
  --set secrets.encryptionKey.existingSecret.name=llm-proxy-encryption
```

Or via values.yaml:
```yaml
secrets:
  managementToken:
    existingSecret:
      name: llm-proxy-secrets
  encryptionKey:
    existingSecret:
      name: llm-proxy-encryption
      key: ENCRYPTION_KEY
```

**Important:** Without `ENCRYPTION_KEY`, the application will still function but will store API keys and tokens in plaintext. A warning will be displayed during installation.


### Dispatcher Configuration

The dispatcher is an optional separate workload that consumes events from the event bus and forwards them to external observability platforms (Lunary, Helicone, or file storage).

**IMPORTANT:** The dispatcher requires a durable event bus (Redis). It cannot be used with the in-memory event bus.

| Parameter | Description | Default |
|-----------|-------------|---------|
| `dispatcher.enabled` | Enable dispatcher deployment | `false` |
| `dispatcher.replicaCount` | Number of dispatcher replicas | `1` |
| `dispatcher.service` | Backend service (`file`, `lunary`, `helicone`) | `file` |
| `dispatcher.endpoint` | Service-specific endpoint URL or file path | `""` (auto-configured based on service) |
| `dispatcher.apiKey.existingSecret.name` | Name of existing Secret containing API key | `""` |
| `dispatcher.apiKey.existingSecret.key` | Key within the Secret for API key | `"DISPATCHER_API_KEY"` |
| `dispatcher.config.bufferSize` | Event bus buffer size | `1000` |
| `dispatcher.config.batchSize` | Events per batch | `100` |
| `dispatcher.persistence.enabled` | Enable PVC for file backend | `true` |
| `dispatcher.persistence.size` | PVC size | `10Gi` |
| `dispatcher.persistence.storageClass` | Storage class for PVC | `""` |
| `dispatcher.resources.limits.cpu` | CPU limit | `500m` |
| `dispatcher.resources.limits.memory` | Memory limit | `256Mi` |
| `dispatcher.resources.requests.cpu` | CPU request | `100m` |
| `dispatcher.resources.requests.memory` | Memory request | `128Mi` |

#### Dispatcher Backend Services

The dispatcher supports three backend services:

**File Backend** (default):
- Writes events to a JSONL file
- Creates PersistentVolumeClaim when `dispatcher.service=file` and `dispatcher.persistence.enabled=true` (enabled by default)
- Default endpoint: `/app/data/events.jsonl`
- No API key required

**Lunary Backend**:
- Forwards events to [Lunary.ai](https://lunary.ai) for LLM observability
- Default endpoint: `https://api.lunary.ai/v1/runs/ingest`
- Requires API key via `dispatcher.apiKey`

**Helicone Backend**:
- Forwards events to [Helicone](https://helicone.ai) for LLM analytics
- Default endpoint: `https://api.worker.helicone.ai/custom/v1/log`
- Requires API key via `dispatcher.apiKey`

#### Dispatcher Configuration Examples

**File Backend (default)**:
```yaml
dispatcher:
  enabled: true
  service: "file"
  persistence:
    enabled: true
    size: 10Gi
```

**Lunary Backend**:
```yaml
dispatcher:
  enabled: true
  service: "lunary"
  apiKey:
    existingSecret:
      name: "dispatcher-secrets"
      key: "LUNARY_API_KEY"
```

**Helicone Backend**:
```yaml
dispatcher:
  enabled: true
  service: "helicone"
  apiKey:
    existingSecret:
      name: "dispatcher-secrets"
      key: "DISPATCHER_API_KEY"
```

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
