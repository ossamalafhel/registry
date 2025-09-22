# Technical Architecture: Centralized Container Log Shipping

## Architecture Overview
Deploy OpenTelemetry Collector as Kubernetes DaemonSet to collect container logs, store in Loki, and visualize through existing Grafana instance using Pulumi infrastructure-as-code.

## Components

### Component 1: Logging Infrastructure Module
**Type:** Pulumi Infrastructure Module  
**File Path:** `deploy/pkg/k8s/logging.go`  
**Purpose:** Deploy and configure OpenTelemetry Collector DaemonSet and Loki storage  
**Implementation Details:**
- DaemonSet with one collector per node
- Mounts `/var/log/containers` read-only
- Resource limits: 100m CPU, 128Mi memory
- 10s batch timeout for log shipping

#### Interface
```go
func DeployLoggingStack(
    ctx *pulumi.Context,
    cluster *providers.ProviderInfo,
    environment string,
    monitoringNS *corev1.Namespace,
) error
```

### Component 2: Loki Storage Configuration
**Type:** Kubernetes StatefulSet  
**File Path:** `deploy/pkg/k8s/logging.go` (same file)  
**Purpose:** Single-binary Loki instance for log storage  
**Implementation Details:**
- 30GB PVC for 7-day retention
- Gzip compression enabled
- Resource limits: 500m CPU, 512Mi memory

### Component 3: OTel Collector Configuration
**Type:** ConfigMap  
**File Path:** `deploy/configs/otel-collector-config.yaml`  
**Purpose:** Define log collection pipeline  
**Implementation Details:**
```yaml
receivers:
  filelog:
    include: ["/var/log/containers/*.log"]
    exclude: ["*health*", "*ping*"]
processors:
  k8s_attributes:
    passthrough: false
  batch:
    timeout: 10s
    send_batch_size: 100
exporters:
  loki:
    endpoint: "http://loki:3100/loki/api/v1/push"
```

## Data Models

### Loki Configuration
**File Path:** `deploy/configs/loki-config.yaml`  
**Fields:**
- `retention_period: 168h` (7 days)
- `chunk_idle_period: 30m`
- `max_chunk_age: 1h`
- `ingester.max_chunk_size: 2MB`

## Implementation Files

| File Path | Change Type | Description |
|-----------|-------------|-------------|
| `deploy/pkg/k8s/logging.go` | Create | Main logging infrastructure module |
| `deploy/configs/otel-collector-config.yaml` | Create | OTel Collector configuration |
| `deploy/configs/loki-config.yaml` | Create | Loki storage configuration |
| `deploy/cmd/main.go` | Modify | Add logging stack deployment call |
| `tests/logging_test.go` | Create | Integration tests for log pipeline |
| `.env.example` | Modify | Add ENABLE_LOGGING=true flag |

## Technical Challenges & Solutions
1. **High log volume:** Filter healthchecks/pings at collection time, implement 7-day retention
2. **Resource usage:** Strict DaemonSet limits (100m/128Mi), monitor with existing VictoriaMetrics

## Deployment Strategy
1. Set `ENABLE_LOGGING=true` in staging environment
2. Run `make dev-compose` to deploy with logging enabled
3. Verify logs appear in Grafana (`http://localhost:3000`)
4. Deploy to production with same flag

### Rollback Procedure
1. Set `ENABLE_LOGGING=false`
2. Redeploy with `make dev-compose`