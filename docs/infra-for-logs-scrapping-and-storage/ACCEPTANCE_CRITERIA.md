# Container Log Shipping Infrastructure

## Acceptance Criteria

### AC1: OpenTelemetry Collector Deployment
**Given:** Kubernetes cluster is running
**When:** OTel Collector DaemonSet is deployed
**Then:** One collector pod runs on each node and mounts `/var/log/containers`
**Affected Files:** 
- `deploy/k8s/otel-collector.yaml`
- `deploy/pulumi/index.ts`
**Test Type:** CI TESTABLE
**Test Implementation:** Add test in `tests/deploy_test.go` to verify DaemonSet exists with correct mount paths using k8s client

### AC2: Log Collection and Shipping
**Given:** OTel Collector is running
**When:** Application containers produce logs
**Then:** Logs are batched and shipped to Loki endpoint within 10 seconds
**Affected Files:**
- `deploy/k8s/otel-collector.yaml` (config section)
**Test Type:** CI TESTABLE
**Test Implementation:** Integration test in `tests/logging_test.go` - deploy test pod, write logs, verify arrival in Loki API

### AC3: Loki Storage Deployment
**Given:** Kubernetes cluster has storage provisioner
**When:** Loki StatefulSet is deployed
**Then:** Loki accepts logs on port 3100 with 10GB PVC attached
**Affected Files:**
- `deploy/k8s/loki.yaml`
- `deploy/pulumi/index.ts`
**Test Type:** CI TESTABLE
**Test Implementation:** Test in `tests/deploy_test.go` to verify StatefulSet, Service, and PVC creation

### AC4: Grafana Integration
**Given:** Loki is running and contains logs
**When:** Grafana datasource is configured
**Then:** Logs are queryable in Grafana with LogQL
**Affected Files:**
- `deploy/k8s/grafana-datasource.yaml`
**Test Type:** MANUAL
**Test Implementation:** Manual verification - query `{namespace="default"}` returns logs

### AC5: Resource Metadata Enrichment
**Given:** Logs are being collected
**When:** OTel processor enriches logs
**Then:** Each log entry contains pod name, namespace, and node labels
**Affected Files:**
- `deploy/k8s/otel-collector.yaml` (processors section)
**Test Type:** CI TESTABLE
**Test Implementation:** Test in `tests/logging_test.go` - verify log entries contain expected k8s metadata fields

## Error Scenarios

### ES1: Loki Unavailable
**Scenario:** Loki endpoint is down
**Expected Behavior:** OTel Collector retries with exponential backoff, logs buffered for up to 60s
**Affected Files:**
- `deploy/k8s/otel-collector.yaml`
**Test Type:** CI TESTABLE