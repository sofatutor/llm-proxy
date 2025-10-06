# Kubernetes Deployment with Helm

This document provides comprehensive instructions for deploying LLM Proxy to Kubernetes using Helm charts.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Quick Start](#quick-start)
- [Configuration](#configuration)
- [Installation Examples](#installation-examples)
- [Upgrade Guide](#upgrade-guide)
- [Security Considerations](#security-considerations)
- [Monitoring and Observability](#monitoring-and-observability)
- [Troubleshooting](#troubleshooting)

## Prerequisites

### Required Tools

- **Kubernetes cluster** (v1.19+)
- **Helm** (v3.8+)
- **kubectl** configured for your cluster

### Required Kubernetes Resources

- **Namespace** (recommended: dedicated namespace)
- **Storage class** for persistent volumes
- **Ingress controller** (for external access)
- **Secrets management** (for production deployments)

### Install Helm

```bash
# Install Helm (if not already installed)
curl https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash

# Verify installation
helm version
```

## Quick Start

### 1. Add Helm Repository Dependencies

```bash
# Add Bitnami repository for Redis dependency
helm repo add bitnami https://charts.bitnami.com/bitnami
helm repo update
```

### 2. Create Namespace

```bash
kubectl create namespace llm-proxy
```

### 3. Basic Installation

```bash
# Navigate to the chart directory
cd deploy/helm/llm-proxy

# Update dependencies
helm dependency update

# Install with minimal configuration
helm install llm-proxy . \
  --namespace llm-proxy \
  --set config.managementToken="$(openssl rand -base64 32)" \
  --wait
```

### 4. Verify Installation

```bash
# Check deployment status
kubectl get pods -n llm-proxy

# Test health endpoint
kubectl port-forward -n llm-proxy svc/llm-proxy 8080:8080
curl http://localhost:8080/health
```

## Configuration

### Core Configuration Options

| Parameter | Description | Default |
|-----------|-------------|---------|
| `image.repository` | Docker image repository | `ghcr.io/sofatutor/llm-proxy` |
| `image.tag` | Docker image tag | `latest` |
| `config.managementToken` | Management API token | `""` (required) |
| `config.logLevel` | Log level | `info` |
| `redis.enabled` | Enable Redis dependency | `true` |
| `dispatcher.enabled` | Enable event dispatcher | `true` |
| `autoscaling.enabled` | Enable horizontal pod autoscaler | `false` |
| `ingress.enabled` | Enable ingress | `false` |

### Environment-Specific Values

The chart includes example values files for different environments:

- **Development**: `examples/values-development.yaml`
- **Production**: `examples/values-production.yaml`

## Installation Examples

### Development Environment

```bash
helm install llm-proxy . \
  --namespace llm-proxy \
  --values examples/values-development.yaml \
  --set config.managementToken="dev-management-token" \
  --wait
```

### Production Environment

```bash
# Create production secrets first
kubectl create secret generic llm-proxy-secrets \
  --namespace llm-proxy \
  --from-literal=management-token="$(openssl rand -base64 32)" \
  --from-literal=openai-api-key="sk-your-openai-key"

# Install with production configuration
helm install llm-proxy . \
  --namespace llm-proxy \
  --values examples/values-production.yaml \
  --set secrets.external=true \
  --set ingress.hosts[0].host=llm-proxy.yourdomain.com \
  --wait
```

### External Redis Configuration

```bash
helm install llm-proxy . \
  --namespace llm-proxy \
  --set redis.enabled=false \
  --set redis.external.host=redis.example.com \
  --set redis.external.port=6379 \
  --set config.managementToken="$(openssl rand -base64 32)" \
  --wait
```

### PostgreSQL Database Configuration

```bash
helm install llm-proxy . \
  --namespace llm-proxy \
  --set config.database.type=postgresql \
  --set config.database.postgresql.host=postgres.example.com \
  --set config.database.postgresql.user=llmproxy \
  --set config.database.postgresql.database=llmproxy \
  --set config.managementToken="$(openssl rand -base64 32)" \
  --wait
```

## Upgrade Guide

### Standard Upgrade

```bash
# Update Helm dependencies
helm dependency update

# Upgrade deployment
helm upgrade llm-proxy . \
  --namespace llm-proxy \
  --values your-values.yaml \
  --wait
```

### Rolling Back

```bash
# View release history
helm history llm-proxy -n llm-proxy

# Rollback to previous version
helm rollback llm-proxy 1 -n llm-proxy
```

### Zero-Downtime Upgrades

The chart supports zero-downtime upgrades through:
- **Pod Disruption Budget**: Ensures minimum replicas during upgrades
- **Rolling Update Strategy**: Gradual pod replacement
- **Health Checks**: Ensures new pods are healthy before proceeding

## Security Considerations

### Secrets Management

#### Option 1: Kubernetes Secrets (Development)

```bash
kubectl create secret generic llm-proxy-secrets \
  --namespace llm-proxy \
  --from-literal=management-token="$(openssl rand -base64 32)" \
  --from-literal=openai-api-key="sk-your-key"
```

#### Option 2: External Secret Operator (Production)

```yaml
# Install External Secrets Operator first
helm repo add external-secrets https://charts.external-secrets.io
helm install external-secrets external-secrets/external-secrets -n external-secrets-system --create-namespace

# Configure SecretStore (example for AWS Secrets Manager)
apiVersion: external-secrets.io/v1beta1
kind: SecretStore
metadata:
  name: llm-proxy-secrets
  namespace: llm-proxy
spec:
  provider:
    aws:
      service: SecretsManager
      region: us-west-2
      auth:
        secretRef:
          accessKeyId:
            name: aws-secret
            key: access-key-id
          secretAccessKey:
            name: aws-secret
            key: secret-access-key
```

### Network Security

#### Network Policies

Enable network policies for production deployments:

```yaml
networkPolicy:
  enabled: true
  policyTypes:
    - Ingress
    - Egress
```

#### Pod Security Standards

The chart implements security best practices:
- Non-root containers
- Read-only root filesystem
- Dropped capabilities
- Seccomp profiles

### RBAC

The chart creates a minimal service account with no additional permissions by default. For production:

```yaml
serviceAccount:
  create: true
  annotations:
    # AWS IRSA example
    eks.amazonaws.com/role-arn: arn:aws:iam::ACCOUNT:role/llm-proxy-role
```

## Monitoring and Observability

### Prometheus Integration

Enable Prometheus monitoring:

```yaml
serviceMonitor:
  enabled: true
  labels:
    release: prometheus-operator

podMonitor:
  enabled: true
  labels:
    release: prometheus-operator
```

### Health Checks

The chart configures comprehensive health checks:

- **Liveness Probe**: `/health` - Detects unhealthy containers
- **Readiness Probe**: `/ready` - Ensures pod is ready for traffic  
- **Startup Probe**: `/health` - Handles slow-starting containers

### Event Dispatchers

Configure event dispatchers for observability:

```yaml
dispatcher:
  enabled: true
  services:
    file:
      enabled: true
      endpoint: "/app/logs/events.jsonl"
    helicone:
      enabled: true
      apiKey: "your-helicone-key"
```

## Troubleshooting

### Common Issues

#### 1. Pod Stuck in Pending State

```bash
# Check pod events
kubectl describe pod -n llm-proxy -l app.kubernetes.io/name=llm-proxy

# Check node resources
kubectl top nodes

# Check storage
kubectl get pv,pvc -n llm-proxy
```

#### 2. Health Check Failures

```bash
# Check pod logs
kubectl logs -n llm-proxy -l app.kubernetes.io/name=llm-proxy

# Test health endpoint directly
kubectl exec -it -n llm-proxy deployment/llm-proxy -- wget -qO- http://localhost:8080/health
```

#### 3. Redis Connection Issues

```bash
# Check Redis pod status
kubectl get pods -n llm-proxy -l app.kubernetes.io/name=redis

# Test Redis connectivity
kubectl exec -it -n llm-proxy deployment/llm-proxy -- nc -zv redis-host 6379
```

### Debug Commands

```bash
# View all resources
kubectl get all -n llm-proxy

# Check configuration
helm get values llm-proxy -n llm-proxy

# View rendered templates
helm template llm-proxy . --debug

# Run Helm tests
helm test llm-proxy -n llm-proxy

# Check ingress
kubectl describe ingress -n llm-proxy
```

### Log Analysis

```bash
# View application logs
kubectl logs -n llm-proxy -l app.kubernetes.io/name=llm-proxy -f

# View dispatcher logs
kubectl logs -n llm-proxy -l app.kubernetes.io/component=dispatcher -f

# Export logs for analysis
kubectl logs -n llm-proxy deployment/llm-proxy --since=1h > llm-proxy.log
```

## Advanced Configuration

### Custom Resources Limits

```yaml
resources:
  limits:
    cpu: 2000m
    memory: 1Gi
    ephemeral-storage: 2Gi
  requests:
    cpu: 500m
    memory: 512Mi
    ephemeral-storage: 1Gi
```

### Node Affinity and Tolerations

```yaml
nodeSelector:
  kubernetes.io/arch: amd64
  node-type: compute

tolerations:
  - key: "workload"
    operator: "Equal"
    value: "llm-proxy"
    effect: "NoSchedule"

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

### Topology Spread Constraints

```yaml
topologySpreadConstraints:
  - maxSkew: 1
    topologyKey: topology.kubernetes.io/zone
    whenUnsatisfiable: DoNotSchedule
    labelSelector:
      matchLabels:
        app.kubernetes.io/name: llm-proxy
```

## Support and Resources

- **GitHub Repository**: https://github.com/sofatutor/llm-proxy
- **Documentation**: https://github.com/sofatutor/llm-proxy/tree/main/docs
- **Issues**: https://github.com/sofatutor/llm-proxy/issues
- **Security**: See [Security Documentation](../../security.md)