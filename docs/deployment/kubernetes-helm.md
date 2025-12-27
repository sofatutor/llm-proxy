# Kubernetes Deployment with Helm

This guide covers deploying the LLM Proxy to Kubernetes using Helm charts.

## Overview

The LLM Proxy Helm chart provides a secure, production-ready deployment with:

- **Secure secrets management** via existing Kubernetes Secrets
- **Support for SQLite and PostgreSQL** database backends
- **Horizontal Pod Autoscaling** for high availability
- **Security-first defaults** (non-root user, read-only filesystem, dropped capabilities)
- **Flexible configuration** for various deployment scenarios

## Quick Start

### Prerequisites

- Kubernetes cluster (1.19+)
- Helm 3.0+
- kubectl configured to access your cluster

### 1. Create Secrets

**IMPORTANT**: Never commit secrets to Git. Always create them externally.

```bash
# Generate a secure management token
MANAGEMENT_TOKEN=$(openssl rand -base64 32)

# Create Kubernetes Secret
kubectl create secret generic llm-proxy-secrets \
  --from-literal=MANAGEMENT_TOKEN="${MANAGEMENT_TOKEN}"
```

For PostgreSQL deployments, include DATABASE_URL:

```bash
kubectl create secret generic llm-proxy-secrets \
  --from-literal=MANAGEMENT_TOKEN="${MANAGEMENT_TOKEN}" \
  --from-literal=DATABASE_URL="postgres://user:password@postgres:5432/llmproxy?sslmode=require"
```

### 2. Install the Chart

**SQLite (single instance):**

```bash
helm install llm-proxy deploy/helm/llm-proxy \
  --set secrets.managementToken.existingSecret.name=llm-proxy-secrets \
  --set image.repository=your-registry/llm-proxy \
  --set image.tag=v1.0.0
```

**PostgreSQL (multi-instance):**

```bash
helm install llm-proxy deploy/helm/llm-proxy \
  --set secrets.managementToken.existingSecret.name=llm-proxy-secrets \
  --set env.DB_DRIVER=postgres \
  --set secrets.databaseUrl.existingSecret.name=llm-proxy-secrets \
  --set image.repository=your-registry/llm-proxy \
  --set image.tag=v1.0.0
```

### 3. Verify Installation

```bash
# Check pod status
kubectl get pods -l app.kubernetes.io/name=llm-proxy

# Check logs
kubectl logs -l app.kubernetes.io/name=llm-proxy

# Port-forward to test
kubectl port-forward svc/llm-proxy 8080:8080

# Test health endpoint
curl http://localhost:8080/health
```

## Deployment Scenarios

### Scenario 1: Development/Testing (SQLite)

Best for: Local testing, development environments, proof of concept

```yaml
# values-dev.yaml
replicaCount: 1

secrets:
  managementToken:
    existingSecret:
      name: llm-proxy-secrets

env:
  DB_DRIVER: sqlite
  LOG_LEVEL: debug
```

```bash
helm install llm-proxy deploy/helm/llm-proxy -f values-dev.yaml
```

### Scenario 2: Production (PostgreSQL + Autoscaling)

Best for: Production workloads, high availability, horizontal scaling

```yaml
# values-prod.yaml
replicaCount: 3

secrets:
  managementToken:
    existingSecret:
      name: llm-proxy-secrets
  databaseUrl:
    existingSecret:
      name: llm-proxy-secrets

env:
  DB_DRIVER: postgres
  LLM_PROXY_EVENT_BUS: redis-streams
  REDIS_ADDR: redis:6379

autoscaling:
  enabled: true
  minReplicas: 2
  maxReplicas: 10
  targetCPUUtilizationPercentage: 70

resources:
  limits:
    cpu: 2000m
    memory: 1Gi
  requests:
    cpu: 200m
    memory: 256Mi
```

```bash
helm install llm-proxy deploy/helm/llm-proxy -f values-prod.yaml
```

### Scenario 3: With Ingress and TLS

Best for: External access, HTTPS termination

```yaml
# values-ingress.yaml
ingress:
  enabled: true
  className: nginx
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod
    nginx.ingress.kubernetes.io/ssl-redirect: "true"
  hosts:
    - host: llm-proxy.example.com
      paths:
        - path: /
          pathType: Prefix
  tls:
    - secretName: llm-proxy-tls
      hosts:
        - llm-proxy.example.com

secrets:
  managementToken:
    existingSecret:
      name: llm-proxy-secrets

env:
  CORS_ALLOWED_ORIGINS: "https://app.example.com"
```

```bash
helm install llm-proxy deploy/helm/llm-proxy -f values-ingress.yaml
```

## Security Configuration

### Secret Management

The chart supports two approaches for secrets:

#### 1. Existing Secrets (RECOMMENDED for production)

Create secrets externally and reference them:

```yaml
secrets:
  managementToken:
    existingSecret:
      name: llm-proxy-secrets
      key: MANAGEMENT_TOKEN
  databaseUrl:
    existingSecret:
      name: db-secrets
      key: DATABASE_URL
```

#### 2. Chart-Managed Secrets (Development only)

```yaml
secrets:
  create: true
  data:
    managementToken: "test-token-not-for-prod"
    databaseUrl: "postgres://..."
```

**WARNING**: Never use `secrets.create=true` in production!

### Database Security

For PostgreSQL deployments:

1. **Use SSL/TLS connections:**
   ```
   DATABASE_URL=postgres://user:pass@host/db?sslmode=require
   ```

2. **For certificate verification:**
   ```
   DATABASE_URL=postgres://user:pass@host/db?sslmode=verify-full&sslrootcert=/path/to/ca.crt
   ```

3. **Store DATABASE_URL in a Secret**, never in ConfigMap or values.yaml

### Pod Security

The chart applies security best practices by default:

- Runs as non-root user (UID 1000)
- Read-only root filesystem
- All capabilities dropped
- No privilege escalation
- Security contexts enforced

## Database Configuration

### SQLite

Use SQLite for single-instance deployments:

```yaml
env:
  DB_DRIVER: sqlite
  DATABASE_PATH: /app/data/llm-proxy.db

# Persistent storage for SQLite
volumes:
  - name: data
    persistentVolumeClaim:
      claimName: llm-proxy-data
```

**Note**: SQLite requires persistent storage and cannot be used with autoscaling.

### PostgreSQL

Use PostgreSQL for multi-instance deployments:

```yaml
secrets:
  databaseUrl:
    existingSecret:
      name: llm-proxy-secrets
      key: DATABASE_URL

env:
  DB_DRIVER: postgres
  DATABASE_POOL_SIZE: "20"
  DATABASE_MAX_IDLE_CONNS: "10"
```

## Resource Management

### Resource Requests and Limits

Configure based on your workload:

```yaml
resources:
  limits:
    cpu: 2000m      # Maximum CPU
    memory: 1Gi     # Maximum memory
  requests:
    cpu: 200m       # Guaranteed CPU
    memory: 256Mi   # Guaranteed memory
```

### Horizontal Pod Autoscaling

Enable autoscaling for dynamic workloads:

```yaml
autoscaling:
  enabled: true
  minReplicas: 2
  maxReplicas: 10
  targetCPUUtilizationPercentage: 70
  targetMemoryUtilizationPercentage: 80
```

## Monitoring and Observability

### Metrics Endpoint

The chart exposes a metrics endpoint by default that can be scraped by Prometheus or other monitoring systems:

```yaml
env:
  ENABLE_METRICS: "true"
  METRICS_PATH: "/metrics"
```

Example Prometheus scrape configuration using ServiceMonitor:

```yaml
apiVersion: v1
kind: ServiceMonitor
metadata:
  name: llm-proxy
spec:
  selector:
    matchLabels:
      app.kubernetes.io/name: llm-proxy
  endpoints:
    - port: http
      path: /metrics
```

### Logging

Configure logging for Kubernetes:

```yaml
env:
  LOG_LEVEL: "info"
  LOG_FORMAT: "json"
  LOG_FILE: ""  # Empty = stdout (recommended)
```

Access logs:

```bash
kubectl logs -f deployment/llm-proxy
kubectl logs -f -l app.kubernetes.io/name=llm-proxy --all-containers=true
```

## Upgrading

### Rolling Updates

```bash
# Update image version
helm upgrade llm-proxy deploy/helm/llm-proxy \
  --set image.tag=v1.1.0 \
  --reuse-values

# Update configuration
helm upgrade llm-proxy deploy/helm/llm-proxy \
  -f values-prod.yaml
```

### Zero-Downtime Upgrades

Ensure at least 2 replicas and configure pod disruption budget:

```yaml
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: llm-proxy-pdb
spec:
  minAvailable: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: llm-proxy
```

## Troubleshooting

### Pod Startup Issues

```bash
# Check pod events
kubectl describe pod <pod-name>

# Check logs
kubectl logs <pod-name>

# Check secret references
kubectl get secret llm-proxy-secrets
kubectl describe secret llm-proxy-secrets
```

### Database Connection Issues

```bash
# Test from within pod
kubectl exec -it deployment/llm-proxy -- /bin/sh

# Check environment variables (be careful not to expose secrets)
kubectl exec deployment/llm-proxy -- env | grep DB_DRIVER
```

### Health Check Failures

```bash
# Port-forward and test directly
kubectl port-forward deployment/llm-proxy 8080:8080
curl http://localhost:8080/health

# Check readiness probe
kubectl get pods -o json | jq '.items[].status.conditions'
```

## Production Checklist

Before deploying to production:

- [ ] Secrets created externally (never in Git)
- [ ] Using `existingSecret` references (not `secrets.create=true`)
- [ ] PostgreSQL configured with SSL/TLS (`sslmode=require` minimum)
- [ ] Resource limits configured appropriately
- [ ] Autoscaling enabled (for PostgreSQL deployments)
- [ ] Ingress configured with TLS
- [ ] CORS origins restricted (not `*`)
- [ ] Monitoring and alerting configured
- [ ] Log aggregation configured
- [ ] Backup strategy for database
- [ ] Pod disruption budget configured
- [ ] Network policies applied

## Advanced Configuration

### Using External PostgreSQL (AWS RDS, GCP Cloud SQL, etc.)

```bash
# Create secret with RDS/Cloud SQL connection string
kubectl create secret generic llm-proxy-db \
  --from-literal=DATABASE_URL="postgres://user:pass@rds-endpoint:5432/llmproxy?sslmode=require"

# Install with external DB
helm install llm-proxy deploy/helm/llm-proxy \
  --set secrets.managementToken.existingSecret.name=llm-proxy-secrets \
  --set env.DB_DRIVER=postgres \
  --set secrets.databaseUrl.existingSecret.name=llm-proxy-db
```

### Using External Redis (AWS ElastiCache, etc.)

```yaml
env:
  LLM_PROXY_EVENT_BUS: redis-streams
  REDIS_ADDR: redis.example.com:6379
  REDIS_DB: "0"
```

### Custom Environment Variables

```yaml
extraEnvFrom:
  - configMapRef:
      name: my-custom-config

extraSecretEnvFrom:
  - secretRef:
      name: my-custom-secrets
```

## Further Reading

- [Helm Chart README](../../deploy/helm/llm-proxy/README.md) - Complete chart documentation
- [Security Guide](../security.md) - Security best practices
- [Database Configuration](../database/README.md) - Database setup and migration
- [Configuration Reference](../configuration.md) - All configuration options

## Support

For issues and questions:
- GitHub Issues: https://github.com/sofatutor/llm-proxy/issues
- Chart Repository: https://github.com/sofatutor/llm-proxy/tree/main/deploy/helm/llm-proxy
