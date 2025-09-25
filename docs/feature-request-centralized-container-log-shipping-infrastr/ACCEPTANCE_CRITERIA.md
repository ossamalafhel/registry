# Centralized Container Log Shipping Infrastructure

## Acceptance Criteria

### AC1: OpenTelemetry Collector DaemonSet Deployment
**Given:** Kubernetes cluster with logging enabled via ENABLE_LOGGING=true  
**When:** Deploy infrastructure with `make dev-compose`  
**Then:** OTel Collector DaemonSet runs on every node with correct resource limits (100m CPU, 128Mi memory)  
**Affected Files:** 
- `deploy/pkg/k8s/logging.go`
- `deploy/cmd/main.go`
**Test Type:** CI TESTABLE  
**Test Implementation:** `tests/logging_test.go` - Verify DaemonSet exists, check pod count equals node count, assert resource limits

### AC2: Loki Storage Deployment
**Given:** Logging infrastructure deployed  
**When:** Loki StatefulSet is created  
**Then:** Loki pod runs with 30GB PVC attached and accepts logs on port 3100  
**Affected Files:**
- `deploy/pkg/k8s/logging.go`
- `deploy/configs/loki-config.yaml`
**Test Type:** CI TESTABLE  
**Test Implementation:** `tests/logging_test.go` - Check StatefulSet exists, verify PVC size, test port connectivity

### AC3: Container Log Collection
**Given:** OTel Collector and Loki running  
**When:** Application containers generate logs  
**Then:** Logs appear in Loki within 15 seconds, excluding healthcheck/ping logs  
**Affected Files:**
- `deploy/configs/otel-collector-config.yaml`
**Test Type:** CI TESTABLE  
**Test Implementation:** `tests/logging_test.go` - Generate test log, query Loki API, verify log presence and filtering

### AC4: Grafana Integration
**Given:** Loki storing logs  
**When:** Access Grafana at http://localhost:3000  
**Then:** Loki datasource configured and container logs queryable via LogQL  
**Affected Files:**
- `deploy/pkg/k8s/logging.go`
**Test Type:** MANUAL  
**Test Implementation:** Manual verification in Grafana UI with sample LogQL query

### AC5: Feature Flag Control
**Given:** Environment variable ENABLE_LOGGING exists  
**When:** Set to false and redeploy  
**Then:** No logging components deployed (OTel, Loki)  
**Affected Files:**
- `.env.example`
- `deploy/cmd/main.go`
**Test Type:** CI TESTABLE  
**Test Implementation:** `tests/logging_test.go` - Deploy with flag=false, verify no logging resources exist

## Error Scenarios

### ES1: Loki Unavailable
**Scenario:** Loki pod crashes or becomes unavailable  
**Expected Behavior:** OTel Collector buffers logs for 60s and retries with exponential backoff  
**Affected Files:**
- `deploy/configs/otel-collector-config.yaml`
**Test Type:** CI TESTABLE