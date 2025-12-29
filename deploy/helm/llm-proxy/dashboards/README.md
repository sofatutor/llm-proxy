# Grafana Dashboards for LLM Proxy

This directory contains ready-to-import Grafana dashboard JSON files for monitoring the LLM Proxy.

## Available Dashboards

### llm-proxy.json

Operational dashboard for LLM Proxy metrics, designed for use with a Prometheus datasource.

**Panels include:**

1. **Overview Section**
   - Uptime (in seconds since server start)
   - Request rate (requests per second)
   - Error rate (percentage of requests that failed)
   - Cache hit ratio (percentage of cache hits vs total cache-eligible requests)

2. **Request Metrics Section**
   - Request and error rate over time
   - Cumulative requests and errors

3. **Cache Performance Section**
   - Cache operations rate (hits, misses, bypass, stores)
   - Cache hit ratio over time
   - Cumulative cache operations

4. **Runtime & Resources Section**
   - Memory usage (heap allocated, in use, system)
   - Goroutines count
   - Garbage collection frequency
   - GC pause time rate
   - Memory allocation rate

## Prerequisites

- Grafana instance (version 8.0 or later recommended)
- Prometheus datasource configured in Grafana
- LLM Proxy instance exposing Prometheus metrics at `/metrics/prometheus`
- Prometheus scraping the LLM Proxy metrics endpoint

## Importing the Dashboard

### Method 1: Manual Import via Grafana UI

1. Open your Grafana instance
2. Navigate to **Dashboards** → **Import**
3. Click **Upload JSON file** and select `llm-proxy.json`
4. Select your Prometheus datasource when prompted
5. Click **Import**

### Method 2: Provisioning via Grafana Configuration

Add the dashboard JSON to your Grafana provisioning directory:

```yaml
# grafana-provisioning.yaml
apiVersion: 1

providers:
  - name: 'LLM Proxy Dashboards'
    orgId: 1
    folder: 'LLM Proxy'
    type: file
    disableDeletion: false
    updateIntervalSeconds: 30
    options:
      path: /etc/grafana/provisioning/dashboards
```

Copy the JSON file to the provisioning path and restart Grafana.

### Method 3: Kubernetes/Helm with Grafana Sidecar

If using the Grafana Helm chart with sidecar discovery enabled, you can automatically provision the dashboard using the LLM Proxy Helm chart:

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

This creates a ConfigMap with the dashboard JSON and the `grafana_dashboard: "1"` label, which the Grafana sidecar will automatically discover and import.

**Manual ConfigMap creation:**

If not using the Helm chart, you can create the ConfigMap manually:

```bash
kubectl create configmap llm-proxy-dashboard \
  --from-file=llm-proxy.json=./llm-proxy.json \
  --namespace=monitoring

kubectl label configmap llm-proxy-dashboard \
  grafana_dashboard=1 \
  --namespace=monitoring
```

The Grafana sidecar will automatically discover and load the dashboard.

## Prometheus Configuration

Ensure Prometheus is scraping the LLM Proxy metrics endpoint. Add the following to your `prometheus.yml`:

```yaml
scrape_configs:
  - job_name: 'llm-proxy'
    static_configs:
      - targets: ['llm-proxy:8080']  # Adjust hostname/port as needed
    metrics_path: '/metrics/prometheus'
    scrape_interval: 15s
```

For Kubernetes deployments using the LLM Proxy Helm chart with ServiceMonitor enabled:

```yaml
# values.yaml
metrics:
  enabled: true
  serviceMonitor:
    enabled: true
    interval: 30s
```

## Dashboard Variables

The dashboard includes template variables for easy filtering:

- **Datasource**: Select the Prometheus datasource to use
- **Job**: Filter metrics by Prometheus job name (auto-populated from `llm_proxy_uptime_seconds` metric)

## Metrics Reference

The dashboard uses the following Prometheus metrics exposed by LLM Proxy:

| Metric | Type | Description |
|--------|------|-------------|
| `llm_proxy_uptime_seconds` | gauge | Time since the server started |
| `llm_proxy_requests_total` | counter | Total number of proxy requests |
| `llm_proxy_errors_total` | counter | Total number of proxy errors |
| `llm_proxy_cache_hits_total` | counter | Total number of cache hits |
| `llm_proxy_cache_misses_total` | counter | Total number of cache misses |
| `llm_proxy_cache_bypass_total` | counter | Total number of cache bypasses |
| `llm_proxy_cache_stores_total` | counter | Total number of cache stores |
| `llm_proxy_goroutines` | gauge | Number of goroutines currently running |
| `llm_proxy_memory_heap_alloc_bytes` | gauge | Heap bytes allocated and in use |
| `llm_proxy_memory_heap_inuse_bytes` | gauge | Heap bytes that are in use |
| `llm_proxy_memory_heap_sys_bytes` | gauge | Heap bytes obtained from OS |
| `llm_proxy_memory_total_alloc_bytes` | counter | Total bytes allocated (cumulative) |
| `llm_proxy_gc_runs_total` | counter | Total number of GC runs |
| `llm_proxy_gc_pause_total_seconds` | counter | Total GC pause time in seconds |

For complete metrics documentation, see [docs/observability/instrumentation.md](../../../docs/observability/instrumentation.md).

## Example Queries

### Request Rate
```promql
rate(llm_proxy_requests_total[5m])
```

### Error Rate Percentage
```promql
rate(llm_proxy_errors_total[5m]) / rate(llm_proxy_requests_total[5m]) * 100
```

### Cache Hit Ratio
```promql
llm_proxy_cache_hits_total / (llm_proxy_cache_hits_total + llm_proxy_cache_misses_total)
```

### Memory Usage Trend
```promql
rate(llm_proxy_memory_total_alloc_bytes[5m])
```

## Troubleshooting

### No Data in Dashboard

1. **Check Prometheus is scraping**: Visit Prometheus UI → Targets, verify the `llm-proxy` job is up
2. **Verify metrics endpoint**: `curl http://llm-proxy:8080/metrics/prometheus`
3. **Check Grafana datasource**: Grafana → Configuration → Data Sources → Test
4. **Verify job label**: Ensure the `job` template variable matches your Prometheus job name

### Incorrect or Missing Metrics

1. **Enable metrics in LLM Proxy**: Set `ENABLE_METRICS=true` (default is enabled)
2. **Check metric names**: Query Prometheus directly to verify metric names match
3. **Verify scrape interval**: Ensure Prometheus scrape interval is not too long (recommended: 15-30s)

### Dashboard UID Conflict

If you get a UID conflict error when importing:
1. Edit the JSON file
2. Change the `uid` field to a unique value (or remove it to auto-generate)
3. Re-import the dashboard

## Customization

The dashboard is designed to be customizable. Common modifications:

- **Add panels**: Use existing panels as templates for new visualizations
- **Adjust time ranges**: Modify default time range in dashboard settings
- **Add alerts**: Configure alert rules on panels for proactive monitoring
- **Change refresh rate**: Default is 10s, adjust in dashboard settings
- **Modify thresholds**: Update color thresholds on stat panels for your operational needs

## Support

For issues or questions:
- GitHub Issues: https://github.com/sofatutor/llm-proxy/issues
- Documentation: https://github.com/sofatutor/llm-proxy/tree/main/docs

## License

Same license as the LLM Proxy project (see repository LICENSE file).
