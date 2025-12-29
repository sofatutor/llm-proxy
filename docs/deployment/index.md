---
title: Deployment
nav_order: 7
has_children: true
---

# Deployment

Production deployment guides for LLM Proxy.

## What's in this section

- **[AWS ECS Architecture](aws-ecs-cdk.md)** - Production deployment on AWS ECS with CDK
- **[Kubernetes / Helm](helm.md)** - Kubernetes deployment with Helm chart
- **[Performance Tuning](performance.md)** - Optimization and performance best practices
- **[Security Best Practices](security.md)** - Production security guidelines

## Recommended: AWS ECS

For production deployments, we recommend **AWS ECS with CDK**:

- Aurora PostgreSQL Serverless v2 for database
- ElastiCache Redis for caching and rate limiting
- ALB with ACM for HTTPS termination
- Auto-scaling based on CPU/request count
- ~$130/month for low-traffic deployments

See the [AWS ECS Architecture Guide](aws-ecs-cdk.md) for details.

## Alternative: Kubernetes/Helm

For organizations with existing Kubernetes infrastructure, we provide a **comprehensive Helm chart**:

- Single-instance deployments with SQLite (development)
- Multi-replica deployments with PostgreSQL and Redis (production)
- Ingress support with TLS (NGINX, Traefik, etc.)
- Horizontal Pod Autoscaler (HPA) for automatic scaling
- Event dispatcher for observability platforms (Lunary, Helicone)

See the **[Kubernetes / Helm Deployment Guide](helm.md)** for complete documentation.

Additional Helm chart references:

- Install from OCI: `helm install llm-proxy oci://ghcr.io/sofatutor/llm-proxy --version <version>`
- Chart source: `deploy/helm/llm-proxy`
- Full chart documentation: [Helm Chart README](../../deploy/helm/llm-proxy/README.md)

**Helm Quick Start (SQLite)**:
```bash
kubectl create secret generic llm-proxy-secrets \
  --from-literal=MANAGEMENT_TOKEN="$(openssl rand -base64 32)"

helm install llm-proxy deploy/helm/llm-proxy \
  --set image.repository=your-registry/llm-proxy \
  --set image.tag=v1.0.0 \
  --set secrets.managementToken.existingSecret.name=llm-proxy-secrets
```

**Helm Quick Start (Production with PostgreSQL)**:

> **Note**: If using a custom-built image, ensure it's built with PostgreSQL support: `docker build --build-arg POSTGRES_SUPPORT=true ...`. See the [full guide](helm.md) for details.

```bash
kubectl create secret generic llm-proxy-secrets \
  --from-literal=MANAGEMENT_TOKEN="$(openssl rand -base64 32)"

# NOTE: Replace USER and PASSWORD with your actual DB credentials; never commit real secrets
kubectl create secret generic llm-proxy-db \
  --from-literal=DATABASE_URL="postgres://USER:PASSWORD@postgres.example.com:5432/llmproxy?sslmode=verify-full"

helm install llm-proxy deploy/helm/llm-proxy \
  --set image.repository=your-registry/llm-proxy \
  --set image.tag=v1.0.0 \
  --set secrets.managementToken.existingSecret.name=llm-proxy-secrets \
  --set secrets.databaseUrl.existingSecret.name=llm-proxy-db \
  --set env.DB_DRIVER=postgres
```

## Other Deployment Options

- **Docker Compose** - Good for local development and testing (see repository root)

