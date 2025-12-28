---
title: Kubernetes / Helm
parent: Deployment
nav_order: 2
---

# Kubernetes Deployment with Helm

**Status**: Available  
**Chart Location**: [`deploy/helm/llm-proxy`](../../deploy/helm/llm-proxy/)  
**Last Updated**: December 27, 2025

---

## Overview

Deploy LLM Proxy to Kubernetes using the official Helm chart. The chart supports:

- **SQLite** for single-instance deployments (development/testing)
- **PostgreSQL** for production (external or in-cluster)
- **Redis** for event bus and caching (external)
- **Ingress** for external access with TLS
- **Horizontal Pod Autoscaler (HPA)** for automatic scaling
- **Dispatcher** for async event forwarding to observability platforms

### When to Use Helm

Choose Helm deployment when:
- You already have Kubernetes infrastructure
- You need fine-grained control over deployment configuration
- You want to integrate with existing K8s tooling (Ingress, HPA, service mesh)
- You need multi-region or multi-cluster deployments

For AWS-native deployments without existing K8s infrastructure, consider [AWS ECS](aws-ecs-cdk.md) instead.

---

## Prerequisites

- **Kubernetes 1.19+** cluster
- **Helm 3.0+** installed
- **kubectl** configured to access your cluster
- Container registry with the LLM Proxy image

---

## Quick Start Scenarios

### 1. SQLite (Single Instance, Development)

Minimal deployment for development or testing:

```bash
# Create management token secret
kubectl create secret generic llm-proxy-secrets \
  --from-literal=MANAGEMENT_TOKEN="$(openssl rand -base64 32)"

# Deploy with SQLite
helm install llm-proxy deploy/helm/llm-proxy \
  --set image.repository=your-registry/llm-proxy \
  --set image.tag=v1.0.0 \
  --set secrets.managementToken.existingSecret.name=llm-proxy-secrets
```

**Note**: SQLite is the default database. Not suitable for multi-replica deployments.

### 2. PostgreSQL (External, Production)

Production deployment with external PostgreSQL:

```bash
# Create secrets
kubectl create secret generic llm-proxy-secrets \
  --from-literal=MANAGEMENT_TOKEN="$(openssl rand -base64 32)"

# NOTE: Replace USER and PASSWORD with your actual DB credentials; never commit real secrets
kubectl create secret generic llm-proxy-db \
  --from-literal=DATABASE_URL="postgres://USER:PASSWORD@postgres.example.com:5432/llmproxy?sslmode=verify-full"

# Deploy with external PostgreSQL
helm install llm-proxy deploy/helm/llm-proxy \
  --set image.repository=your-registry/llm-proxy \
  --set image.tag=v1.0.0 \
  --set secrets.managementToken.existingSecret.name=llm-proxy-secrets \
  --set secrets.databaseUrl.existingSecret.name=llm-proxy-db \
  --set env.DB_DRIVER=postgres
```

**Important**: Ensure your Docker image is built with PostgreSQL support:
```bash
docker build --build-arg POSTGRES_SUPPORT=true -t your-registry/llm-proxy:v1.0.0 .
```

### 3. External Redis (Multi-Instance)

Deploy with Redis for event bus and caching:

```bash
# Create secrets
kubectl create secret generic llm-proxy-secrets \
  --from-literal=MANAGEMENT_TOKEN="$(openssl rand -base64 32)"

# NOTE: Replace USER and PASSWORD with your actual DB credentials; never commit real secrets
kubectl create secret generic llm-proxy-db \
  --from-literal=DATABASE_URL="postgres://USER:PASSWORD@postgres.example.com:5432/llmproxy?sslmode=verify-full"

# Create Redis password secret (if authentication is enabled)
openssl rand -base64 32 > /tmp/redis-password.txt
kubectl create secret generic redis-password \
  --from-file=REDIS_PASSWORD=/tmp/redis-password.txt
rm /tmp/redis-password.txt

# Deploy with Redis
helm install llm-proxy deploy/helm/llm-proxy \
  --set image.repository=your-registry/llm-proxy \
  --set image.tag=v1.0.0 \
  --set secrets.managementToken.existingSecret.name=llm-proxy-secrets \
  --set secrets.databaseUrl.existingSecret.name=llm-proxy-db \
  --set env.DB_DRIVER=postgres \
  --set redis.external.addr="redis.example.com:6379" \
  --set redis.external.password.existingSecret.name=redis-password \
  --set env.LLM_PROXY_EVENT_BUS="redis-streams" \
  --set replicaCount=3
```

**Note**: Redis is required for multi-instance deployments. The in-memory event bus only works with a single replica.

### 4. Ingress + TLS (External Access)

Expose the service via Ingress with automatic TLS:

```bash
helm install llm-proxy deploy/helm/llm-proxy \
  --set image.repository=your-registry/llm-proxy \
  --set image.tag=v1.0.0 \
  --set secrets.managementToken.existingSecret.name=llm-proxy-secrets \
  --set secrets.databaseUrl.existingSecret.name=llm-proxy-db \
  --set env.DB_DRIVER=postgres \
  --set ingress.enabled=true \
  --set ingress.className=nginx \
  --set 'ingress.annotations.cert-manager\.io/cluster-issuer=letsencrypt-prod' \
  --set ingress.hosts[0].host=api.example.com \
  --set ingress.hosts[0].paths[0].path=/ \
  --set ingress.hosts[0].paths[0].pathType=Prefix \
  --set ingress.tls[0].secretName=llm-proxy-tls \
  --set ingress.tls[0].hosts[0]=api.example.com
```

**Prerequisites**:
- NGINX Ingress Controller (or another Ingress controller) installed
- cert-manager for automatic TLS certificate management (optional)

### 5. Autoscaling (HPA)

Enable Horizontal Pod Autoscaler for automatic scaling:

```bash
helm install llm-proxy deploy/helm/llm-proxy \
  --set image.repository=your-registry/llm-proxy \
  --set image.tag=v1.0.0 \
  --set secrets.managementToken.existingSecret.name=llm-proxy-secrets \
  --set secrets.databaseUrl.existingSecret.name=llm-proxy-db \
  --set env.DB_DRIVER=postgres \
  --set autoscaling.enabled=true \
  --set autoscaling.minReplicas=2 \
  --set autoscaling.maxReplicas=20 \
  --set autoscaling.targetCPUUtilizationPercentage=75
```

**Prerequisites**:
- metrics-server installed in your cluster
- Resource requests properly configured (CPU/memory)
- PostgreSQL database (SQLite does not support multi-replica)

**Note**: When HPA is enabled, the `replicaCount` value is ignored.

---

## Production Values File

For production deployments, use a `values.yaml` file:

```yaml
# production-values.yaml
image:
  repository: your-registry/llm-proxy
  tag: v1.0.0

secrets:
  managementToken:
    existingSecret:
      name: llm-proxy-secrets
  databaseUrl:
    existingSecret:
      name: llm-proxy-db

env:
  DB_DRIVER: postgres
  LOG_LEVEL: info
  LOG_FORMAT: json
  ENABLE_METRICS: "true"
  LLM_PROXY_EVENT_BUS: redis-streams

redis:
  external:
    addr: "redis.example.com:6379"
    db: 0
    password:
      existingSecret:
        name: redis-password

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

autoscaling:
  enabled: true
  minReplicas: 2
  maxReplicas: 20
  targetCPUUtilizationPercentage: 75

resources:
  requests:
    cpu: 200m
    memory: 256Mi
  limits:
    cpu: 2000m
    memory: 1Gi
```

Deploy with the values file:

```bash
helm install llm-proxy deploy/helm/llm-proxy -f production-values.yaml
```

---

## Dispatcher (Event Forwarding)

The optional dispatcher component forwards events to observability platforms:

```bash
# Deploy with dispatcher for Lunary integration
helm install llm-proxy deploy/helm/llm-proxy \
  --set image.repository=your-registry/llm-proxy \
  --set image.tag=v1.0.0 \
  --set secrets.managementToken.existingSecret.name=llm-proxy-secrets \
  --set redis.external.addr="redis.example.com:6379" \
  --set env.LLM_PROXY_EVENT_BUS="redis-streams" \
  --set dispatcher.enabled=true \
  --set dispatcher.service="lunary" \
  --set dispatcher.apiKey.existingSecret.name="dispatcher-secrets"
```

**Supported backends**:
- `file` - Write events to JSONL file (with PersistentVolumeClaim)
- `lunary` - Forward to Lunary.ai for LLM observability
- `helicone` - Forward to Helicone for LLM analytics

**Important**: Dispatcher requires Redis. It cannot be used with the in-memory event bus.

---

## Verification

After deployment, verify the installation:

```bash
# Check pod status
kubectl get pods -l app.kubernetes.io/name=llm-proxy

# Check service
kubectl get svc -l app.kubernetes.io/name=llm-proxy

# View logs
kubectl logs -l app.kubernetes.io/name=llm-proxy

# Test health endpoints
kubectl port-forward svc/llm-proxy 8080:8080
curl http://localhost:8080/live
curl http://localhost:8080/ready
```

For Ingress deployments:

```bash
# Check Ingress status
kubectl get ingress

# Test external access (after DNS is configured)
curl https://api.example.com/live
```

---

## Upgrading

Upgrade an existing deployment:

```bash
# Upgrade with new image version
helm upgrade llm-proxy deploy/helm/llm-proxy \
  --reuse-values \
  --set image.tag=v1.1.0

# Upgrade with new values file
helm upgrade llm-proxy deploy/helm/llm-proxy -f production-values.yaml
```

---

## Uninstalling

```bash
# Uninstall the release
helm uninstall llm-proxy

# Optionally, delete secrets
kubectl delete secret llm-proxy-secrets llm-proxy-db redis-password
```

---

## Complete Documentation

For comprehensive documentation, see:

- **[Helm Chart README](../../deploy/helm/llm-proxy/README.md)** - Full configuration reference
- **[Helm Chart Examples](../../deploy/helm/llm-proxy/EXAMPLES.md)** - Additional deployment examples
- **[values.yaml](../../deploy/helm/llm-proxy/values.yaml)** - All configurable values

The chart-local documentation includes:
- Detailed configuration for all components
- Security best practices
- Secret management strategies
- Health check configuration
- Resource limits and requests
- PostgreSQL subchart configuration (in-cluster development)
- Advanced dispatcher scenarios
- Troubleshooting guides

---

## Comparison: Helm vs AWS ECS

| Factor | Helm / Kubernetes | AWS ECS |
|--------|------------------|---------|
| **Infrastructure** | Requires existing K8s cluster | AWS-native, no K8s needed |
| **Cost** | Depends on cluster setup | ~$130/mo for low traffic |
| **Complexity** | Higher (K8s knowledge required) | Lower (managed service) |
| **Portability** | Multi-cloud, on-premise | AWS only |
| **Tooling** | Rich K8s ecosystem | AWS-native tools |
| **Scaling** | HPA, cluster autoscaler | ECS auto-scaling |
| **Best For** | Existing K8s infrastructure | AWS-first deployments |

**Recommendation**:
- **Choose Helm** if you already have Kubernetes infrastructure or need multi-cloud portability
- **Choose AWS ECS** for AWS-native deployments without existing K8s infrastructure

See the [AWS ECS Architecture Guide](aws-ecs-cdk.md) for AWS-specific deployment.

---

## See Also

- [Performance Tuning](performance.md) - Optimization and resource planning
- [Security Best Practices](security.md) - Production security guidelines
- [Configuration Reference](../getting-started/configuration.md) - Environment variables and settings
- [Token Management](../guides/token-management.md) - API token configuration
