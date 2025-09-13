package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

// TestOTelCollectorLokiIntegration verifies AC2: Log Collection and Shipping
// Given OTel Collector is running, When application containers produce logs,
// Then logs are batched and shipped to Loki endpoint within 10 seconds
func TestOTelCollectorLokiIntegration(t *testing.T) {
	// Test configuration
	lokiEndpoint := getEnvOrDefault("LOKI_ENDPOINT", "http://localhost:3100")
	testNamespace := "test-namespace"
	testPod := "test-pod"
	testContainer := "test-container"
	
	// Create test context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	// Test cases
	tests := []struct {
		name           string
		logMessage     string
		expectedLabels map[string]string
		maxLatency     time.Duration
	}{
		{
			name:       "AC2_LogShippingWithin10Seconds",
			logMessage: fmt.Sprintf("TEST_LOG_%d: Testing OTel Collector to Loki pipeline", time.Now().Unix()),
			expectedLabels: map[string]string{
				"namespace": testNamespace,
				"pod":       testPod,
				"container": testContainer,
				"level":     "INFO",
			},
			maxLatency: 10 * time.Second, // AC2 requirement: within 10 seconds
		},
		{
			name:       "AC2_ErrorLogShipping",
			logMessage: fmt.Sprintf("ERROR_LOG_%d: Testing error log shipping", time.Now().Unix()),
			expectedLabels: map[string]string{
				"namespace": testNamespace,
				"pod":       testPod,
				"container": testContainer,
				"level":     "ERROR",
			},
			maxLatency: 10 * time.Second,
		},
		{
			name:       "AC2_MultiLineLogShipping",
			logMessage: fmt.Sprintf("MULTILINE_%d: Line1\nLine2\nLine3", time.Now().Unix()),
			expectedLabels: map[string]string{
				"namespace": testNamespace,
				"pod":       testPod,
				"container": testContainer,
			},
			maxLatency: 10 * time.Second,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Record start time
			startTime := time.Now()
			
			// Generate log entry
			logEntry := generateLogEntry(tt.logMessage, tt.expectedLabels)
			
			// Write log to simulated container log file
			if err := writeContainerLog(logEntry); err != nil {
				t.Fatalf("Failed to write container log: %v", err)
			}
			
			// Poll Loki for the log entry
			found := false
			var latency time.Duration
			
			ticker := time.NewTicker(500 * time.Millisecond)
			defer ticker.Stop()
			
			for {
				select {
				case <-ctx.Done():
					t.Fatalf("Context timeout: log not found in Loki within timeout period")
				case <-ticker.C:
					latency = time.Since(startTime)
					
					// Query Loki for the log
					if foundInLoki(t, lokiEndpoint, tt.logMessage, tt.expectedLabels) {
						found = true
						goto verify
					}
					
					// Check if we've exceeded the maximum allowed latency
					if latency > tt.maxLatency {
						t.Fatalf("AC2 FAILED: Log not shipped within %v (actual: %v)", tt.maxLatency, latency)
					}
				}
			}
			
		verify:
			if !found {
				t.Fatalf("Log entry not found in Loki")
			}
			
			// Verify latency requirement
			if latency > tt.maxLatency {
				t.Errorf("AC2 FAILED: Latency %v exceeds maximum %v", latency, tt.maxLatency)
			} else {
				t.Logf("AC2 PASSED: Log shipped to Loki in %v (requirement: <%v)", latency, tt.maxLatency)
			}
		})
	}
}

// TestOTelCollectorBatchProcessing verifies batch processing behavior
func TestOTelCollectorBatchProcessing(t *testing.T) {
	lokiEndpoint := getEnvOrDefault("LOKI_ENDPOINT", "http://localhost:3100")
	batchSize := 100
	
	// Generate batch of logs
	logs := make([]string, batchSize)
	timestamp := time.Now().Unix()
	for i := 0; i < batchSize; i++ {
		logs[i] = fmt.Sprintf("BATCH_TEST_%d_%d: Log message %d", timestamp, i, i)
	}
	
	// Record start time
	startTime := time.Now()
	
	// Write all logs
	for _, log := range logs {
		logEntry := generateLogEntry(log, map[string]string{
			"namespace": "test-namespace",
			"pod":       "batch-test-pod",
			"container": "batch-test-container",
		})
		if err := writeContainerLog(logEntry); err != nil {
			t.Fatalf("Failed to write log: %v", err)
		}
	}
	
	// Wait for batch timeout (10 seconds as per configuration)
	time.Sleep(11 * time.Second)
	
	// Verify all logs are in Loki
	foundCount := 0
	for _, log := range logs {
		if foundInLoki(t, lokiEndpoint, log, nil) {
			foundCount++
		}
	}
	
	processingTime := time.Since(startTime)
	successRate := float64(foundCount) / float64(batchSize) * 100
	
	t.Logf("Batch processing results: %d/%d logs found (%.1f%%) in %v", 
		foundCount, batchSize, successRate, processingTime)
	
	if successRate < 95.0 {
		t.Errorf("Batch processing failed: only %.1f%% of logs were successfully shipped", successRate)
	}
	
	// Verify batching happened within the 10-second window
	if processingTime > 15*time.Second {
		t.Errorf("Batch processing took too long: %v (expected: ~10s)", processingTime)
	}
}

// TestOTelCollectorKubernetesMetadata verifies Kubernetes metadata enrichment
func TestOTelCollectorKubernetesMetadata(t *testing.T) {
	lokiEndpoint := getEnvOrDefault("LOKI_ENDPOINT", "http://localhost:3100")
	
	// Generate log with minimal info
	logMessage := fmt.Sprintf("METADATA_TEST_%d: Testing K8s metadata enrichment", time.Now().Unix())
	logEntry := generateLogEntry(logMessage, map[string]string{
		"namespace": "test-namespace",
		"pod":       "metadata-test-pod",
	})
	
	// Write log
	if err := writeContainerLog(logEntry); err != nil {
		t.Fatalf("Failed to write log: %v", err)
	}
	
	// Wait for processing
	time.Sleep(5 * time.Second)
	
	// Query Loki and verify metadata
	query := fmt.Sprintf(`{namespace="test-namespace",pod="metadata-test-pod"} |= "%s"`, logMessage)
	resp, err := queryLoki(lokiEndpoint, query)
	if err != nil {
		t.Fatalf("Failed to query Loki: %v", err)
	}
	
	// Verify enriched metadata
	expectedMetadata := []string{
		"container",
		"node",
		"cluster",
		"environment",
		"service",
	}
	
	for _, metadata := range expectedMetadata {
		if !strings.Contains(string(resp), metadata) {
			t.Errorf("Missing expected metadata field: %s", metadata)
		}
	}
	
	t.Log("Kubernetes metadata enrichment verified successfully")
}

// TestOTelCollectorFilterProcessing verifies log filtering
func TestOTelCollectorFilterProcessing(t *testing.T) {
	lokiEndpoint := getEnvOrDefault("LOKI_ENDPOINT", "http://localhost:3100")
	
	// Test filtered logs (should not appear in Loki)
	filteredLogs := []string{
		"health check passed",
		"readiness probe succeeded",
		"liveness probe check",
	}
	
	// Test non-filtered logs (should appear in Loki)
	regularLog := fmt.Sprintf("FILTER_TEST_%d: Regular application log", time.Now().Unix())
	
	// Write all logs
	for _, log := range filteredLogs {
		logEntry := generateLogEntry(log, map[string]string{"namespace": "test-namespace"})
		if err := writeContainerLog(logEntry); err != nil {
			t.Fatalf("Failed to write log: %v", err)
		}
	}
	
	regularEntry := generateLogEntry(regularLog, map[string]string{"namespace": "test-namespace"})
	if err := writeContainerLog(regularEntry); err != nil {
		t.Fatalf("Failed to write regular log: %v", err)
	}
	
	// Wait for processing
	time.Sleep(5 * time.Second)
	
	// Verify filtered logs are not in Loki
	for _, log := range filteredLogs {
		if foundInLoki(t, lokiEndpoint, log, nil) {
			t.Errorf("Filtered log should not be in Loki: %s", log)
		}
	}
	
	// Verify regular log is in Loki
	if !foundInLoki(t, lokiEndpoint, regularLog, nil) {
		t.Error("Regular log should be in Loki")
	}
	
	t.Log("Log filtering verified successfully")
}

// Helper functions

func generateLogEntry(message string, labels map[string]string) string {
	entry := map[string]interface{}{
		"log":    message,
		"stream": "stdout",
		"time":   time.Now().Format(time.RFC3339Nano),
	}
	
	// Add labels as metadata
	for k, v := range labels {
		entry[k] = v
	}
	
	data, _ := json.Marshal(entry)
	return string(data)
}

func writeContainerLog(logEntry string) error {
	// In a real test, this would write to the actual container log location
	// For testing purposes, we'll simulate this
	// The OTel Collector's filelog receiver would pick this up from /var/log/containers/
	
	// Note: In actual implementation, you'd write to a file that the OTel Collector monitors
	fmt.Printf("Simulated log write: %s\n", logEntry)
	return nil
}

func foundInLoki(t *testing.T, endpoint, message string, labels map[string]string) bool {
	// Build label selector
	var labelPairs []string
	for k, v := range labels {
		labelPairs = append(labelPairs, fmt.Sprintf(`%s="%s"`, k, v))
	}
	
	selector := "{" + strings.Join(labelPairs, ",") + "}"
	if len(labelPairs) == 0 {
		selector = "{}"
	}
	
	// Build query
	query := fmt.Sprintf(`%s |= "%s"`, selector, message)
	
	// Query Loki
	resp, err := queryLoki(endpoint, query)
	if err != nil {
		t.Logf("Error querying Loki: %v", err)
		return false
	}
	
	// Check if message is in response
	return bytes.Contains(resp, []byte(message))
}

func queryLoki(endpoint, query string) ([]byte, error) {
	// Build Loki query API URL
	url := fmt.Sprintf("%s/loki/api/v1/query_range?query=%s&start=%d&end=%d",
		endpoint,
		query,
		time.Now().Add(-1*time.Hour).Unix(),
		time.Now().Unix(),
	)
	
	// Make HTTP request
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	
	req.Header.Set("X-Scope-OrgID", "mcp-registry")
	
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	return io.ReadAll(resp.Body)
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}