# Technical Architecture: Container Log Shipping Infrastructure

## Architecture Overview
Deploy OpenTelemetry Collector as Kubernetes DaemonSet to collect container logs, ship to Loki for storage, and visualize through existing Grafana. Minimal application changes required - purely infrastructure addition.

## Components

### Component 1: OpenTelemetry Collector DaemonSet
**Type:** Infrastructure/Kubernetes
**File Path:** `deploy/k8s/otel-collector.yaml`
**Purpose:** Collect logs from all containers via stdout/stderr
**Implementation Details:**
- DaemonSet ensures one collector per node
- Mounts `/var/log/containers` for log access
- Resource processors add pod/namespace/node metadata
- Batch processor for efficient shipping

#### Configuration
```yaml
receivers:
  filelog:
    include: ["/var/log/containers/*.log"]
    operators:
      - type: json_parser
        timestamp:
          parse_from: attributes.time
processors:
  resource:
    attributes:
      - key: k8s.pod.name
        from_attribute: pod_name
      - key: k8s.namespace.name
        from_attribute: namespace
  batch:
    timeout: 10s
    send_batch_size: 100
exporters:
  loki:
    endpoint: http://loki:3100/loki/api/v1/push
```

### Component 2: Loki Storage
**Type:** Infrastructure/StatefulSet
**File Path:** `deploy/k8s/loki.yaml`
**Purpose:** Store and index logs with 7-day retention
**Implementation Details:**
- Single binary mode for simplicity
- Local storage with PVC (10GB initially)
- Retention policy via config

### Component 3: Grafana Datasource
**Type:** Configuration
**File Path:** `deploy/k8s/grafana-datasource.yaml`
**Purpose:** Connect existing Grafana to Loki
**Implementation Details:**
- Add Loki as datasource
- Pre-configured dashboards for container logs

## API Endpoints

No API changes required - this is infrastructure-only.

## Data Models

No database changes required. Logs stored in Loki's internal format.

## Implementation Files

| File Path | Change Type | Description |
|-----------|-------------|-------------|
| deploy/k8s/otel-collector.yaml | Create | OTel Collector DaemonSet + ConfigMap |
| deploy/k8s/loki.yaml | Create | Loki StatefulSet + Service + ConfigMap |
| deploy/k8s/grafana-datasource.yaml | Create | Grafana Loki datasource configuration |
| deploy/pulumi/index.ts | Modify | Add log infrastructure resources |

## Technical Challenges & Solutions
1. **Log Volume:** Use batch processing and 7-day retention to control storage
2. **Performance Impact:** Resource limits on OTel collector (100m CPU, 128Mi memory)

## Deployment Strategy
1. Deploy Loki StatefulSet first
2. Deploy OTel Collector DaemonSet
3. Configure Grafana datasource
4. Verify logs appear in Grafana

### Rollback Procedure
1. Delete OTel Collector DaemonSet
2. Remove Grafana datasource