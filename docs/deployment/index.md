---
title: Deployment
nav_order: 7
has_children: true
---

# Deployment

Production deployment guides for LLM Proxy.

## What's in this section

- **[AWS ECS Architecture](aws-ecs-cdk.md)** - Production deployment on AWS ECS with CDK
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

## Alternative Deployments

- **Docker Compose** - Good for development and testing
- **Kubernetes/Helm** - For existing K8s infrastructure
  - Install from OCI: `helm install llm-proxy oci://ghcr.io/sofatutor/llm-proxy --version <version>`
  - Chart source: `deploy/helm/llm-proxy`
  - See [Helm Chart README](../../deploy/helm/llm-proxy/README.md) for full documentation

