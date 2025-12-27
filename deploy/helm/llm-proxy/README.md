# LLM Proxy Helm Chart

This Helm chart deploys the LLM Proxy server to Kubernetes.

## Prerequisites

- Kubernetes 1.19+
- Helm 3.0+

## Installation

**Important:** The application requires a `MANAGEMENT_TOKEN` environment variable to be configured. This minimal chart does not yet include secret management. You must manually provide `MANAGEMENT_TOKEN` (for example, via a Kubernetes Secret and corresponding pod configuration) for the deployment to function correctly. Secret handling support will be added in issue #203.

```bash
helm install llm-proxy deploy/helm/llm-proxy \
  --set image.repository=your-registry/llm-proxy \
  --set image.tag=v1.0.0
```

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
```

## Health Checks

The chart configures health probes with dedicated endpoints:
- **Liveness probe** (`/live`): Checks if the application is running
- **Readiness probe** (`/ready`): Checks if the application is ready to serve traffic

Both probes can be customized via `livenessProbe` and `readinessProbe` in values.yaml.

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
