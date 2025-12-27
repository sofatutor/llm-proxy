# LLM Proxy Helm Chart

This Helm chart deploys the LLM Proxy application to a Kubernetes cluster.

## Prerequisites

- Kubernetes 1.19+
- Helm 3.0+
- Kubernetes Secrets for sensitive data (recommended for production)

## Security Best Practices

**IMPORTANT**: This chart is designed with security as the primary concern:

1. **Never commit secrets to Git** - Always use existing Kubernetes Secrets
2. **Use `existingSecret` references** - The recommended approach for production
3. **Optional chart-managed Secret** - Only for development/testing with `secrets.create=true`

## Installation

### Production Installation (Recommended)

1. **Create Kubernetes Secrets externally:**

```bash
# Create secret for management token
kubectl create secret generic llm-proxy-secrets \
  --from-literal=MANAGEMENT_TOKEN="$(openssl rand -base64 32)"

# If using PostgreSQL, add DATABASE_URL to the same secret
kubectl create secret generic llm-proxy-secrets \
  --from-literal=MANAGEMENT_TOKEN="$(openssl rand -base64 32)" \
  --from-literal=DATABASE_URL="postgres://user:password@postgres:5432/llmproxy?sslmode=require"
```

2. **Install the chart referencing existing secrets:**

```bash
helm install llm-proxy deploy/helm/llm-proxy \
  --set secrets.managementToken.existingSecret.name=llm-proxy-secrets \
  --set secrets.managementToken.existingSecret.key=MANAGEMENT_TOKEN \
  --set image.repository=your-registry/llm-proxy \
  --set image.tag=v1.0.0
```

3. **For PostgreSQL deployments:**

```bash
helm install llm-proxy deploy/helm/llm-proxy \
  --set secrets.managementToken.existingSecret.name=llm-proxy-secrets \
  --set secrets.managementToken.existingSecret.key=MANAGEMENT_TOKEN \
  --set env.DB_DRIVER=postgres \
  --set secrets.databaseUrl.existingSecret.name=llm-proxy-secrets \
  --set secrets.databaseUrl.existingSecret.key=DATABASE_URL \
  --set image.repository=your-registry/llm-proxy \
  --set image.tag=v1.0.0
```

### Development/Testing Installation

For development or testing only, you can let the chart create secrets:

```bash
helm install llm-proxy deploy/helm/llm-proxy \
  --set secrets.create=true \
  --set secrets.data.managementToken=test-token-do-not-use-in-prod \
  --set image.repository=llm-proxy \
  --set image.tag=latest
```

**WARNING**: This approach is NOT recommended for production use!

## Configuration

### Required Configuration

The following must be configured for the application to start:

| Parameter | Description | Default | Required |
|-----------|-------------|---------|----------|
| `secrets.managementToken.existingSecret.name` | Name of existing Secret containing MANAGEMENT_TOKEN | `""` | Yes (or use `secrets.create=true`) |
| `secrets.managementToken.existingSecret.key` | Key in Secret for MANAGEMENT_TOKEN | `"MANAGEMENT_TOKEN"` | Yes (when using existingSecret) |

### PostgreSQL Configuration

When using PostgreSQL (`env.DB_DRIVER=postgres`):

| Parameter | Description | Default | Required |
|-----------|-------------|---------|----------|
| `env.DB_DRIVER` | Database driver (sqlite or postgres) | `"sqlite"` | No |
| `secrets.databaseUrl.existingSecret.name` | Name of existing Secret containing DATABASE_URL | `""` | Yes (when DB_DRIVER=postgres) |
| `secrets.databaseUrl.existingSecret.key` | Key in Secret for DATABASE_URL | `"DATABASE_URL"` | Yes (when using existingSecret) |

### Image Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `image.repository` | Container image repository | `llm-proxy` |
| `image.tag` | Container image tag | `""` (uses chart appVersion) |
| `image.pullPolicy` | Image pull policy | `IfNotPresent` |
| `imagePullSecrets` | Image pull secrets | `[]` |

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

### Autoscaling

| Parameter | Description | Default |
|-----------|-------------|---------|
| `autoscaling.enabled` | Enable HPA | `false` |
| `autoscaling.minReplicas` | Minimum replicas | `1` |
| `autoscaling.maxReplicas` | Maximum replicas | `10` |
| `autoscaling.targetCPUUtilizationPercentage` | Target CPU % | `80` |

### Ingress

| Parameter | Description | Default |
|-----------|-------------|---------|
| `ingress.enabled` | Enable Ingress | `false` |
| `ingress.className` | Ingress class name | `""` |
| `ingress.hosts[0].host` | Hostname | `llm-proxy.local` |
| `ingress.tls` | TLS configuration | `[]` |

### Application Environment Variables

All application environment variables can be configured under the `env` key. See `values.yaml` for the complete list.

Important environment variables:

- `env.DB_DRIVER`: Database driver (`sqlite` or `postgres`)
- `env.LOG_LEVEL`: Logging level (`debug`, `info`, `warn`, `error`)
- `env.ENABLE_METRICS`: Enable metrics endpoint
- `env.LLM_PROXY_EVENT_BUS`: Event bus backend (`in-memory`, `redis`, `redis-streams`)

## Examples

### SQLite (Single Instance)

```yaml
# values-sqlite.yaml
image:
  repository: your-registry/llm-proxy
  tag: v1.0.0

secrets:
  managementToken:
    existingSecret:
      name: llm-proxy-secrets
      key: MANAGEMENT_TOKEN

env:
  DB_DRIVER: sqlite
  DATABASE_PATH: /app/data/llm-proxy.db

replicaCount: 1
```

```bash
kubectl create secret generic llm-proxy-secrets \
  --from-literal=MANAGEMENT_TOKEN="$(openssl rand -base64 32)"

helm install llm-proxy deploy/helm/llm-proxy -f values-sqlite.yaml
```

### PostgreSQL with Redis (Multi-Instance)

```yaml
# values-postgres.yaml
image:
  repository: your-registry/llm-proxy
  tag: v1.0.0

replicaCount: 3

secrets:
  managementToken:
    existingSecret:
      name: llm-proxy-secrets
      key: MANAGEMENT_TOKEN
  databaseUrl:
    existingSecret:
      name: llm-proxy-secrets
      key: DATABASE_URL

env:
  DB_DRIVER: postgres
  LLM_PROXY_EVENT_BUS: redis-streams
  REDIS_ADDR: redis:6379

autoscaling:
  enabled: true
  minReplicas: 2
  maxReplicas: 10
  targetCPUUtilizationPercentage: 70
```

```bash
kubectl create secret generic llm-proxy-secrets \
  --from-literal=MANAGEMENT_TOKEN="$(openssl rand -base64 32)" \
  --from-literal=DATABASE_URL="postgres://user:pass@postgres:5432/llmproxy?sslmode=require"

helm install llm-proxy deploy/helm/llm-proxy -f values-postgres.yaml
```

### With Ingress and TLS

```yaml
# values-ingress.yaml
ingress:
  enabled: true
  className: nginx
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod
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
      key: MANAGEMENT_TOKEN
```

## Upgrading

```bash
helm upgrade llm-proxy deploy/helm/llm-proxy \
  --set secrets.managementToken.existingSecret.name=llm-proxy-secrets \
  --set image.tag=v1.1.0 \
  --reuse-values
```

## Uninstalling

```bash
helm uninstall llm-proxy
```

## Security Considerations

1. **Secrets Management**
   - Always use external Kubernetes Secrets for production
   - Rotate secrets regularly
   - Use RBAC to restrict Secret access

2. **Database Security**
   - Use TLS/SSL for PostgreSQL connections (`sslmode=require` or `sslmode=verify-full`)
   - Store DATABASE_URL in a Secret, not ConfigMap
   - Use strong database passwords

3. **Network Security**
   - Use NetworkPolicies to restrict pod communication
   - Enable TLS on Ingress
   - Restrict CORS origins in production

4. **Pod Security**
   - Chart runs as non-root user by default
   - Read-only root filesystem enabled
   - All capabilities dropped
   - Security contexts enforced

## Troubleshooting

### Pods not starting

Check if secrets are configured correctly:

```bash
kubectl describe pod -l app.kubernetes.io/name=llm-proxy
kubectl logs -l app.kubernetes.io/name=llm-proxy
```

### Database connection issues

For PostgreSQL, verify the DATABASE_URL format and connectivity:

```bash
kubectl exec -it deployment/llm-proxy -- /bin/sh
# Try connecting to database from within pod
```

### Secret not found errors

Ensure the Secret exists in the same namespace:

```bash
kubectl get secrets
kubectl describe secret llm-proxy-secrets
```

## Support

For issues and questions:
- GitHub Issues: https://github.com/sofatutor/llm-proxy/issues
- Documentation: https://github.com/sofatutor/llm-proxy/tree/main/docs
