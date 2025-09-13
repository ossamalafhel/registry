# OpenTelemetry Collector Deployment

This directory contains the implementation for deploying OpenTelemetry Collector as a DaemonSet in Kubernetes for container log collection.

## Overview

The OpenTelemetry Collector is deployed as a DaemonSet to ensure one collector pod runs on each node in the Kubernetes cluster. It collects container logs from `/var/log/containers` and enriches them with Kubernetes metadata.

## Components

### 1. Pulumi Deployment (`pkg/k8s/otel_collector.go`)
- Programmatic deployment using Pulumi Go SDK
- Integrated with the main deployment pipeline
- Automatically deployed when running `pulumi up`

### 2. Kubernetes Manifest (`otel-collector-daemonset.yaml`)
- Standalone YAML manifest for manual deployment
- Can be used for testing or direct kubectl apply

## Features

### Log Collection
- **Container Logs**: Collects logs from all containers via `/var/log/containers/*.log`
- **Self-exclusion**: Excludes OTel Collector's own logs to prevent infinite loops
- **JSON Parsing**: Parses Docker/CRI-O JSON log format
- **Metadata Extraction**: Extracts pod name, namespace, container name from file paths

### Kubernetes Integration
- **ServiceAccount**: Proper RBAC permissions for accessing Kubernetes API
- **ClusterRole**: Read-only access to pods, namespaces, nodes, and workloads
- **Node Tolerations**: Runs on master/control-plane nodes as well
- **DaemonSet**: Ensures coverage on all nodes

### Volume Mounts
- `/var/log`: Read-only access to container logs
- `/var/lib/docker/containers`: Access to Docker container logs
- ConfigMap: OTel Collector configuration

### Resource Management
- **Requests**: 100m CPU, 256Mi Memory
- **Limits**: 500m CPU, 512Mi Memory
- **Memory Limiter**: Prevents OOM with 75% limit threshold
- **GOMEMLIMIT**: Set to 400MiB for Go runtime

### Security
- **ReadOnlyRootFilesystem**: Enabled for security
- **Non-root User**: Runs as nobody (UID 65534)
- **No Privilege Escalation**: Prevented
- **Dropped Capabilities**: All capabilities dropped

### Observability
- **Health Checks**: Liveness and readiness probes on port 13133
- **Prometheus Metrics**: Exposed on port 8888
- **Internal Metrics**: Exposed on port 8889
- **pprof**: Performance profiling on port 1777

## Configuration

The collector is configured with:

### Receivers
1. **filelog**: Collects container logs from filesystem
2. **k8s_cluster**: Collects Kubernetes cluster metrics
3. **hostmetrics**: Collects host-level metrics (CPU, memory, disk, etc.)

### Processors
1. **k8sattributes**: Enriches logs with Kubernetes metadata
2. **resource**: Adds environment and cluster information
3. **memory_limiter**: Prevents OOM conditions
4. **batch**: Batches data for efficient export

### Exporters
1. **debug**: For troubleshooting (logs sample of data)
2. **otlp**: Sends data to OTLP-compatible backend
3. **prometheus**: Exposes metrics in Prometheus format

## Deployment

### Using Pulumi (Recommended)

The OTel Collector is automatically deployed as part of the main deployment:

```bash
cd deploy
pulumi up
```

### Manual Deployment

To deploy manually using kubectl:

```bash
kubectl apply -f otel-collector-daemonset.yaml
```

### Verification

Check that the DaemonSet is running:

```bash
# Check DaemonSet status
kubectl get daemonset -n otel-system

# Check pods (should be one per node)
kubectl get pods -n otel-system -o wide

# Check logs
kubectl logs -n otel-system -l app=otel-collector

# Check metrics endpoint
kubectl port-forward -n otel-system daemonset/otel-collector 8889:8889
curl http://localhost:8889/metrics
```

## Integration with Monitoring Stack

The OTel Collector integrates with the existing monitoring stack:

1. **Metrics**: Prometheus-compatible metrics can be scraped by VictoriaMetrics
2. **Logs**: Can be forwarded to a log aggregation system via OTLP
3. **Traces**: Ready to collect distributed traces when instrumentation is added

## Customization

### Backend Configuration

Update the OTLP exporter endpoint in the configuration:

```yaml
exporters:
  otlp:
    endpoint: "your-backend:4317"  # Replace with actual endpoint
    tls:
      insecure: false
      cert_file: /path/to/cert
      key_file: /path/to/key
```

### Resource Limits

Adjust resource limits based on your cluster size:

```yaml
resources:
  requests:
    cpu: 200m      # Increase for larger clusters
    memory: 512Mi  # Increase for higher log volumes
  limits:
    cpu: 1000m
    memory: 1Gi
```

### Log Filtering

Add additional exclude patterns if needed:

```yaml
filelog:
  exclude:
    - /var/log/containers/*otel-collector*.log
    - /var/log/containers/*kube-system*.log  # Example: exclude system logs
```

## Troubleshooting

### No Logs Collected

1. Check volume mounts exist:
   ```bash
   kubectl exec -n otel-system -it <pod-name> -- ls -la /var/log/containers/
   ```

2. Check RBAC permissions:
   ```bash
   kubectl auth can-i get pods --as=system:serviceaccount:otel-system:otel-collector
   ```

### High Memory Usage

1. Adjust memory_limiter settings
2. Reduce batch size
3. Increase resource limits

### Connection Issues

1. Verify backend endpoint is reachable
2. Check TLS configuration
3. Review firewall/network policies

## Future Enhancements

- [ ] Add trace collection when applications are instrumented
- [ ] Implement log sampling for high-volume environments
- [ ] Add custom resource detectors for cloud providers
- [ ] Implement tail sampling for traces
- [ ] Add metric filtering and aggregation rules