package integration

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

// TestOTelCollectorManifestValidation validates the YAML manifest structure
func TestOTelCollectorManifestValidation(t *testing.T) {
	manifestPath := filepath.Join("..", "..", "deploy", "otel-collector-daemonset.yaml")
	
	// Check if manifest exists
	_, err := os.Stat(manifestPath)
	require.NoError(t, err, "OTel Collector manifest should exist at %s", manifestPath)
	
	// Read the manifest
	manifestContent, err := os.ReadFile(manifestPath)
	require.NoError(t, err, "Should be able to read manifest file")
	
	// Split the manifest into individual resources
	resources := splitYAMLDocuments(string(manifestContent))
	
	// Validate we have the expected number of resources
	assert.GreaterOrEqual(t, len(resources), 6, "Should have at least 6 resources (Namespace, ServiceAccount, ClusterRole, ClusterRoleBinding, ConfigMap, DaemonSet, Service)")
	
	// Track what resources we've found
	foundResources := make(map[string]bool)
	
	for _, resource := range resources {
		var obj map[string]interface{}
		err := yaml.Unmarshal([]byte(resource), &obj)
		require.NoError(t, err, "Each resource should be valid YAML")
		
		kind, ok := obj["kind"].(string)
		require.True(t, ok, "Resource should have a kind")
		
		foundResources[kind] = true
		
		// Validate each resource type
		switch kind {
		case "Namespace":
			validateNamespace(t, obj)
		case "ServiceAccount":
			validateServiceAccount(t, obj)
		case "ClusterRole":
			validateClusterRole(t, obj)
		case "ClusterRoleBinding":
			validateClusterRoleBinding(t, obj)
		case "ConfigMap":
			validateConfigMap(t, obj)
		case "DaemonSet":
			validateDaemonSet(t, obj)
		case "Service":
			validateService(t, obj)
		}
	}
	
	// Ensure all required resources are present
	requiredResources := []string{
		"Namespace",
		"ServiceAccount",
		"ClusterRole",
		"ClusterRoleBinding",
		"ConfigMap",
		"DaemonSet",
		"Service",
	}
	
	for _, required := range requiredResources {
		assert.True(t, foundResources[required], "Should have %s resource", required)
	}
}

// TestOTelCollectorDeployment tests the deployment with a fake Kubernetes client
func TestOTelCollectorDeployment(t *testing.T) {
	// Create a fake Kubernetes client
	clientset := fake.NewSimpleClientset()
	
	// Deploy namespace
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "otel-system",
			Labels: map[string]string{
				"environment": "test",
			},
		},
	}
	
	createdNs, err := clientset.CoreV1().Namespaces().Create(context.TODO(), namespace, metav1.CreateOptions{})
	require.NoError(t, err)
	assert.Equal(t, "otel-system", createdNs.Name)
	
	// Deploy ServiceAccount
	serviceAccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "otel-collector",
			Namespace: "otel-system",
			Labels: map[string]string{
				"app": "otel-collector",
			},
		},
	}
	
	createdSA, err := clientset.CoreV1().ServiceAccounts("otel-system").Create(context.TODO(), serviceAccount, metav1.CreateOptions{})
	require.NoError(t, err)
	assert.Equal(t, "otel-collector", createdSA.Name)
	
	// Deploy ClusterRole
	clusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "otel-collector",
			Labels: map[string]string{
				"app": "otel-collector",
			},
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"pods", "namespaces", "nodes"},
				Verbs:     []string{"get", "watch", "list"},
			},
			{
				APIGroups: []string{"apps"},
				Resources: []string{"deployments", "daemonsets", "statefulsets", "replicasets"},
				Verbs:     []string{"get", "watch", "list"},
			},
		},
	}
	
	createdCR, err := clientset.RbacV1().ClusterRoles().Create(context.TODO(), clusterRole, metav1.CreateOptions{})
	require.NoError(t, err)
	assert.Equal(t, "otel-collector", createdCR.Name)
	
	// Deploy ClusterRoleBinding
	clusterRoleBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "otel-collector",
			Labels: map[string]string{
				"app": "otel-collector",
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "otel-collector",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      "otel-collector",
				Namespace: "otel-system",
			},
		},
	}
	
	createdCRB, err := clientset.RbacV1().ClusterRoleBindings().Create(context.TODO(), clusterRoleBinding, metav1.CreateOptions{})
	require.NoError(t, err)
	assert.Equal(t, "otel-collector", createdCRB.Name)
	
	// Deploy ConfigMap
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "otel-collector-config",
			Namespace: "otel-system",
			Labels: map[string]string{
				"app": "otel-collector",
			},
		},
		Data: map[string]string{
			"otel-collector-config.yaml": getTestOTelConfig(),
		},
	}
	
	createdCM, err := clientset.CoreV1().ConfigMaps("otel-system").Create(context.TODO(), configMap, metav1.CreateOptions{})
	require.NoError(t, err)
	assert.Equal(t, "otel-collector-config", createdCM.Name)
	
	// Deploy DaemonSet
	daemonSet := createTestDaemonSet()
	
	createdDS, err := clientset.AppsV1().DaemonSets("otel-system").Create(context.TODO(), daemonSet, metav1.CreateOptions{})
	require.NoError(t, err)
	assert.Equal(t, "otel-collector", createdDS.Name)
	
	// Deploy Service
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "otel-collector-metrics",
			Namespace: "otel-system",
			Labels: map[string]string{
				"app": "otel-collector",
			},
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"app": "otel-collector",
			},
			Type: corev1.ServiceTypeClusterIP,
			Ports: []corev1.ServicePort{
				{
					Name:       "metrics",
					Port:       8889,
					TargetPort: intstr.FromInt(8889),
					Protocol:   corev1.ProtocolTCP,
				},
				{
					Name:       "prometheus",
					Port:       8888,
					TargetPort: intstr.FromInt(8888),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}
	
	createdSvc, err := clientset.CoreV1().Services("otel-system").Create(context.TODO(), service, metav1.CreateOptions{})
	require.NoError(t, err)
	assert.Equal(t, "otel-collector-metrics", createdSvc.Name)
	
	// Verify all resources are created
	verifyDeployment(t, clientset)
}

// TestOTelCollectorConfigurationParsing tests that the OTel configuration is valid
func TestOTelCollectorConfigurationParsing(t *testing.T) {
	config := getTestOTelConfig()
	
	var configMap map[string]interface{}
	err := yaml.Unmarshal([]byte(config), &configMap)
	require.NoError(t, err, "OTel configuration should be valid YAML")
	
	// Validate receivers section
	receivers, ok := configMap["receivers"].(map[string]interface{})
	require.True(t, ok, "Configuration should have receivers section")
	
	// Check filelog receiver
	filelog, ok := receivers["filelog"].(map[string]interface{})
	require.True(t, ok, "Should have filelog receiver")
	
	include, ok := filelog["include"].([]interface{})
	require.True(t, ok, "Filelog should have include paths")
	assert.Contains(t, include, "/var/log/containers/*.log")
	
	exclude, ok := filelog["exclude"].([]interface{})
	require.True(t, ok, "Filelog should have exclude paths")
	assert.Contains(t, exclude, "/var/log/containers/*otel-collector*.log")
	
	// Check k8s_cluster receiver
	_, ok = receivers["k8s_cluster"].(map[string]interface{})
	assert.True(t, ok, "Should have k8s_cluster receiver")
	
	// Check hostmetrics receiver
	_, ok = receivers["hostmetrics"].(map[string]interface{})
	assert.True(t, ok, "Should have hostmetrics receiver")
	
	// Validate processors section
	processors, ok := configMap["processors"].(map[string]interface{})
	require.True(t, ok, "Configuration should have processors section")
	
	// Check required processors
	requiredProcessors := []string{"k8sattributes", "resource", "memory_limiter", "batch"}
	for _, proc := range requiredProcessors {
		_, ok := processors[proc].(map[string]interface{})
		assert.True(t, ok, "Should have %s processor", proc)
	}
	
	// Validate exporters section
	exporters, ok := configMap["exporters"].(map[string]interface{})
	require.True(t, ok, "Configuration should have exporters section")
	
	// Check required exporters
	requiredExporters := []string{"debug", "otlp", "prometheus"}
	for _, exp := range requiredExporters {
		_, ok := exporters[exp].(map[string]interface{})
		assert.True(t, ok, "Should have %s exporter", exp)
	}
	
	// Validate extensions section
	extensions, ok := configMap["extensions"].(map[string]interface{})
	require.True(t, ok, "Configuration should have extensions section")
	
	// Check required extensions
	requiredExtensions := []string{"health_check", "pprof", "memory_ballast"}
	for _, ext := range requiredExtensions {
		_, ok := extensions[ext].(map[string]interface{})
		assert.True(t, ok, "Should have %s extension", ext)
	}
	
	// Validate service section
	service, ok := configMap["service"].(map[string]interface{})
	require.True(t, ok, "Configuration should have service section")
	
	// Check pipelines
	pipelines, ok := service["pipelines"].(map[string]interface{})
	require.True(t, ok, "Service should have pipelines")
	
	// Validate logs pipeline
	logs, ok := pipelines["logs"].(map[string]interface{})
	require.True(t, ok, "Should have logs pipeline")
	
	logsReceivers, ok := logs["receivers"].([]interface{})
	require.True(t, ok, "Logs pipeline should have receivers")
	assert.Contains(t, logsReceivers, "filelog")
	
	// Validate metrics pipeline
	metrics, ok := pipelines["metrics"].(map[string]interface{})
	require.True(t, ok, "Should have metrics pipeline")
	
	metricsReceivers, ok := metrics["receivers"].([]interface{})
	require.True(t, ok, "Metrics pipeline should have receivers")
	assert.Contains(t, metricsReceivers, "k8s_cluster")
	assert.Contains(t, metricsReceivers, "hostmetrics")
}

// TestDaemonSetResourceConfiguration tests DaemonSet resource limits and requests
func TestDaemonSetResourceConfiguration(t *testing.T) {
	ds := createTestDaemonSet()
	
	// Check container count
	assert.Len(t, ds.Spec.Template.Spec.Containers, 1, "Should have exactly one container")
	
	container := ds.Spec.Template.Spec.Containers[0]
	
	// Validate resource requests
	cpuRequest := container.Resources.Requests[corev1.ResourceCPU]
	assert.Equal(t, "100m", cpuRequest.String(), "CPU request should be 100m")
	
	memoryRequest := container.Resources.Requests[corev1.ResourceMemory]
	assert.Equal(t, "256Mi", memoryRequest.String(), "Memory request should be 256Mi")
	
	// Validate resource limits
	cpuLimit := container.Resources.Limits[corev1.ResourceCPU]
	assert.Equal(t, "500m", cpuLimit.String(), "CPU limit should be 500m")
	
	memoryLimit := container.Resources.Limits[corev1.ResourceMemory]
	assert.Equal(t, "512Mi", memoryLimit.String(), "Memory limit should be 512Mi")
}

// TestDaemonSetVolumeMounts tests that all required volumes are mounted
func TestDaemonSetVolumeMounts(t *testing.T) {
	ds := createTestDaemonSet()
	
	container := ds.Spec.Template.Spec.Containers[0]
	volumes := ds.Spec.Template.Spec.Volumes
	
	// Check volume mounts
	requiredMounts := map[string]string{
		"otel-collector-config":  "/conf",
		"varlog":                  "/var/log",
		"varlibdockercontainers": "/var/lib/docker/containers",
	}
	
	for name, path := range requiredMounts {
		found := false
		for _, mount := range container.VolumeMounts {
			if mount.Name == name {
				found = true
				assert.Equal(t, path, mount.MountPath, "Mount path for %s should be %s", name, path)
				assert.True(t, mount.ReadOnly, "Mount %s should be read-only", name)
				break
			}
		}
		assert.True(t, found, "Should have volume mount for %s", name)
	}
	
	// Check volumes
	requiredVolumes := map[string]string{
		"varlog":                  "/var/log",
		"varlibdockercontainers": "/var/lib/docker/containers",
	}
	
	for name, hostPath := range requiredVolumes {
		found := false
		for _, volume := range volumes {
			if volume.Name == name {
				found = true
				assert.NotNil(t, volume.HostPath, "Volume %s should have hostPath", name)
				assert.Equal(t, hostPath, volume.HostPath.Path, "HostPath for %s should be %s", name, hostPath)
				assert.Equal(t, corev1.HostPathDirectory, *volume.HostPath.Type, "HostPath type should be Directory")
				break
			}
		}
		assert.True(t, found, "Should have volume %s", name)
	}
	
	// Check ConfigMap volume
	found := false
	for _, volume := range volumes {
		if volume.Name == "otel-collector-config" {
			found = true
			assert.NotNil(t, volume.ConfigMap, "Config volume should use ConfigMap")
			assert.Equal(t, "otel-collector-config", volume.ConfigMap.Name, "ConfigMap name should match")
			break
		}
	}
	assert.True(t, found, "Should have ConfigMap volume")
}

// TestDaemonSetSecurityContext tests security settings
func TestDaemonSetSecurityContext(t *testing.T) {
	ds := createTestDaemonSet()
	
	container := ds.Spec.Template.Spec.Containers[0]
	secCtx := container.SecurityContext
	
	require.NotNil(t, secCtx, "Container should have security context")
	
	// Check security settings
	assert.True(t, *secCtx.ReadOnlyRootFilesystem, "Should have read-only root filesystem")
	assert.False(t, *secCtx.AllowPrivilegeEscalation, "Should not allow privilege escalation")
	assert.True(t, *secCtx.RunAsNonRoot, "Should run as non-root")
	assert.Equal(t, int64(65534), *secCtx.RunAsUser, "Should run as nobody user (65534)")
	
	// Check capabilities
	assert.NotNil(t, secCtx.Capabilities, "Should have capabilities defined")
	assert.Contains(t, secCtx.Capabilities.Drop, corev1.Capability("ALL"), "Should drop all capabilities")
}

// TestDaemonSetProbes tests liveness and readiness probes
func TestDaemonSetProbes(t *testing.T) {
	ds := createTestDaemonSet()
	
	container := ds.Spec.Template.Spec.Containers[0]
	
	// Test liveness probe
	liveness := container.LivenessProbe
	require.NotNil(t, liveness, "Should have liveness probe")
	assert.NotNil(t, liveness.HTTPGet, "Liveness probe should use HTTP GET")
	assert.Equal(t, "/health", liveness.HTTPGet.Path, "Liveness probe path should be /health")
	assert.Equal(t, intstr.FromInt(13133), liveness.HTTPGet.Port, "Liveness probe port should be 13133")
	assert.Equal(t, int32(30), liveness.InitialDelaySeconds, "Liveness initial delay should be 30s")
	assert.Equal(t, int32(10), liveness.PeriodSeconds, "Liveness period should be 10s")
	assert.Equal(t, int32(5), liveness.TimeoutSeconds, "Liveness timeout should be 5s")
	assert.Equal(t, int32(3), liveness.FailureThreshold, "Liveness failure threshold should be 3")
	
	// Test readiness probe
	readiness := container.ReadinessProbe
	require.NotNil(t, readiness, "Should have readiness probe")
	assert.NotNil(t, readiness.HTTPGet, "Readiness probe should use HTTP GET")
	assert.Equal(t, "/health", readiness.HTTPGet.Path, "Readiness probe path should be /health")
	assert.Equal(t, intstr.FromInt(13133), readiness.HTTPGet.Port, "Readiness probe port should be 13133")
	assert.Equal(t, int32(10), readiness.InitialDelaySeconds, "Readiness initial delay should be 10s")
	assert.Equal(t, int32(10), readiness.PeriodSeconds, "Readiness period should be 10s")
	assert.Equal(t, int32(5), readiness.TimeoutSeconds, "Readiness timeout should be 5s")
	assert.Equal(t, int32(3), readiness.FailureThreshold, "Readiness failure threshold should be 3")
}

// TestDaemonSetTolerations tests node tolerations
func TestDaemonSetTolerations(t *testing.T) {
	ds := createTestDaemonSet()
	
	tolerations := ds.Spec.Template.Spec.Tolerations
	
	// Check for master node toleration
	hasMasterToleration := false
	hasControlPlaneToleration := false
	
	for _, toleration := range tolerations {
		if toleration.Key == "node-role.kubernetes.io/master" {
			hasMasterToleration = true
			assert.Equal(t, corev1.TolerationOpExists, toleration.Operator, "Master toleration should use Exists operator")
			assert.Equal(t, corev1.TaintEffectNoSchedule, toleration.Effect, "Master toleration should have NoSchedule effect")
		}
		if toleration.Key == "node-role.kubernetes.io/control-plane" {
			hasControlPlaneToleration = true
			assert.Equal(t, corev1.TolerationOpExists, toleration.Operator, "Control-plane toleration should use Exists operator")
			assert.Equal(t, corev1.TaintEffectNoSchedule, toleration.Effect, "Control-plane toleration should have NoSchedule effect")
		}
	}
	
	assert.True(t, hasMasterToleration, "Should tolerate master nodes")
	assert.True(t, hasControlPlaneToleration, "Should tolerate control-plane nodes")
}

// TestServiceEndpoints tests the Service configuration
func TestServiceEndpoints(t *testing.T) {
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "otel-collector-metrics",
			Namespace: "otel-system",
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"app": "otel-collector",
			},
			Type: corev1.ServiceTypeClusterIP,
			Ports: []corev1.ServicePort{
				{
					Name:       "metrics",
					Port:       8889,
					TargetPort: intstr.FromInt(8889),
					Protocol:   corev1.ProtocolTCP,
				},
				{
					Name:       "prometheus",
					Port:       8888,
					TargetPort: intstr.FromInt(8888),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}
	
	// Check service type
	assert.Equal(t, corev1.ServiceTypeClusterIP, service.Spec.Type, "Service should be ClusterIP type")
	
	// Check selector
	assert.Equal(t, "otel-collector", service.Spec.Selector["app"], "Service should select otel-collector pods")
	
	// Check ports
	assert.Len(t, service.Spec.Ports, 2, "Service should expose 2 ports")
	
	// Validate metrics port
	metricsPort := findServicePort(service.Spec.Ports, "metrics")
	require.NotNil(t, metricsPort, "Should have metrics port")
	assert.Equal(t, int32(8889), metricsPort.Port, "Metrics port should be 8889")
	assert.Equal(t, intstr.FromInt(8889), metricsPort.TargetPort, "Metrics target port should be 8889")
	
	// Validate prometheus port
	promPort := findServicePort(service.Spec.Ports, "prometheus")
	require.NotNil(t, promPort, "Should have prometheus port")
	assert.Equal(t, int32(8888), promPort.Port, "Prometheus port should be 8888")
	assert.Equal(t, intstr.FromInt(8888), promPort.TargetPort, "Prometheus target port should be 8888")
}

// TestRBACPermissions tests RBAC configuration
func TestRBACPermissions(t *testing.T) {
	clusterRole := &rbacv1.ClusterRole{
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"pods", "namespaces", "nodes"},
				Verbs:     []string{"get", "watch", "list"},
			},
			{
				APIGroups: []string{"apps"},
				Resources: []string{"deployments", "daemonsets", "statefulsets", "replicasets"},
				Verbs:     []string{"get", "watch", "list"},
			},
		},
	}
	
	// Check core API permissions
	coreRule := findPolicyRule(clusterRole.Rules, "")
	require.NotNil(t, coreRule, "Should have core API permissions")
	
	assert.Contains(t, coreRule.Resources, "pods", "Should have pods permission")
	assert.Contains(t, coreRule.Resources, "namespaces", "Should have namespaces permission")
	assert.Contains(t, coreRule.Resources, "nodes", "Should have nodes permission")
	
	// Check that permissions are read-only
	for _, verb := range coreRule.Verbs {
		assert.Contains(t, []string{"get", "watch", "list"}, verb, "Should only have read permissions")
	}
	
	// Check apps API permissions
	appsRule := findPolicyRule(clusterRole.Rules, "apps")
	require.NotNil(t, appsRule, "Should have apps API permissions")
	
	assert.Contains(t, appsRule.Resources, "deployments", "Should have deployments permission")
	assert.Contains(t, appsRule.Resources, "daemonsets", "Should have daemonsets permission")
	assert.Contains(t, appsRule.Resources, "statefulsets", "Should have statefulsets permission")
	assert.Contains(t, appsRule.Resources, "replicasets", "Should have replicasets permission")
	
	// Check that permissions are read-only
	for _, verb := range appsRule.Verbs {
		assert.Contains(t, []string{"get", "watch", "list"}, verb, "Should only have read permissions")
	}
}

// TestEnvironmentVariables tests that required environment variables are set
func TestEnvironmentVariables(t *testing.T) {
	ds := createTestDaemonSet()
	
	container := ds.Spec.Template.Spec.Containers[0]
	envVars := container.Env
	
	requiredEnvVars := map[string]string{
		"K8S_NODE_NAME":     "spec.nodeName",
		"K8S_POD_NAME":      "metadata.name",
		"K8S_POD_NAMESPACE": "metadata.namespace",
		"K8S_POD_IP":        "status.podIP",
		"K8S_POD_UID":       "metadata.uid",
	}
	
	for envName, fieldPath := range requiredEnvVars {
		found := false
		for _, env := range envVars {
			if env.Name == envName {
				found = true
				assert.NotNil(t, env.ValueFrom, "Env var %s should use ValueFrom", envName)
				assert.NotNil(t, env.ValueFrom.FieldRef, "Env var %s should use FieldRef", envName)
				assert.Equal(t, fieldPath, env.ValueFrom.FieldRef.FieldPath, "Env var %s should reference %s", envName, fieldPath)
				break
			}
		}
		assert.True(t, found, "Should have environment variable %s", envName)
	}
	
	// Check GOMEMLIMIT
	found := false
	for _, env := range envVars {
		if env.Name == "GOMEMLIMIT" {
			found = true
			assert.Equal(t, "400MiB", env.Value, "GOMEMLIMIT should be set to 400MiB")
			break
		}
	}
	assert.True(t, found, "Should have GOMEMLIMIT environment variable")
}

// Helper functions

func splitYAMLDocuments(content string) []string {
	var documents []string
	parts := strings.Split(content, "---")
	
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			documents = append(documents, trimmed)
		}
	}
	
	return documents
}

func validateNamespace(t *testing.T, obj map[string]interface{}) {
	metadata := obj["metadata"].(map[string]interface{})
	assert.Equal(t, "otel-system", metadata["name"], "Namespace should be named otel-system")
	
	labels, ok := metadata["labels"].(map[string]interface{})
	if ok {
		assert.Contains(t, labels, "environment", "Namespace should have environment label")
	}
}

func validateServiceAccount(t *testing.T, obj map[string]interface{}) {
	metadata := obj["metadata"].(map[string]interface{})
	assert.Equal(t, "otel-collector", metadata["name"], "ServiceAccount should be named otel-collector")
	assert.Equal(t, "otel-system", metadata["namespace"], "ServiceAccount should be in otel-system namespace")
}

func validateClusterRole(t *testing.T, obj map[string]interface{}) {
	metadata := obj["metadata"].(map[string]interface{})
	assert.Equal(t, "otel-collector", metadata["name"], "ClusterRole should be named otel-collector")
	
	rules, ok := obj["rules"].([]interface{})
	assert.True(t, ok, "ClusterRole should have rules")
	assert.GreaterOrEqual(t, len(rules), 2, "ClusterRole should have at least 2 rules")
}

func validateClusterRoleBinding(t *testing.T, obj map[string]interface{}) {
	metadata := obj["metadata"].(map[string]interface{})
	assert.Equal(t, "otel-collector", metadata["name"], "ClusterRoleBinding should be named otel-collector")
	
	roleRef := obj["roleRef"].(map[string]interface{})
	assert.Equal(t, "ClusterRole", roleRef["kind"], "Should reference ClusterRole")
	assert.Equal(t, "otel-collector", roleRef["name"], "Should reference otel-collector ClusterRole")
	
	subjects := obj["subjects"].([]interface{})
	assert.Len(t, subjects, 1, "Should have one subject")
	
	subject := subjects[0].(map[string]interface{})
	assert.Equal(t, "ServiceAccount", subject["kind"], "Subject should be ServiceAccount")
	assert.Equal(t, "otel-collector", subject["name"], "Subject should be otel-collector")
	assert.Equal(t, "otel-system", subject["namespace"], "Subject should be in otel-system namespace")
}

func validateConfigMap(t *testing.T, obj map[string]interface{}) {
	metadata := obj["metadata"].(map[string]interface{})
	assert.Equal(t, "otel-collector-config", metadata["name"], "ConfigMap should be named otel-collector-config")
	assert.Equal(t, "otel-system", metadata["namespace"], "ConfigMap should be in otel-system namespace")
	
	data, ok := obj["data"].(map[string]interface{})
	assert.True(t, ok, "ConfigMap should have data")
	assert.Contains(t, data, "otel-collector-config.yaml", "ConfigMap should contain otel-collector-config.yaml")
}

func validateDaemonSet(t *testing.T, obj map[string]interface{}) {
	metadata := obj["metadata"].(map[string]interface{})
	assert.Equal(t, "otel-collector", metadata["name"], "DaemonSet should be named otel-collector")
	assert.Equal(t, "otel-system", metadata["namespace"], "DaemonSet should be in otel-system namespace")
	
	spec := obj["spec"].(map[string]interface{})
	
	// Check selector
	selector := spec["selector"].(map[string]interface{})
	matchLabels := selector["matchLabels"].(map[string]interface{})
	assert.Equal(t, "otel-collector", matchLabels["app"], "DaemonSet should select app=otel-collector")
	
	// Check update strategy
	updateStrategy, ok := spec["updateStrategy"].(map[string]interface{})
	if ok {
		assert.Equal(t, "RollingUpdate", updateStrategy["type"], "Should use RollingUpdate strategy")
	}
	
	// Check template
	template := spec["template"].(map[string]interface{})
	templateSpec := template["spec"].(map[string]interface{})
	
	// Check service account
	assert.Equal(t, "otel-collector", templateSpec["serviceAccountName"], "Should use otel-collector service account")
	
	// Check containers
	containers := templateSpec["containers"].([]interface{})
	assert.Len(t, containers, 1, "Should have one container")
	
	container := containers[0].(map[string]interface{})
	assert.Equal(t, "otel-collector", container["name"], "Container should be named otel-collector")
	assert.Contains(t, container["image"].(string), "otel/opentelemetry-collector", "Should use OpenTelemetry collector image")
	
	// Check volumes
	volumes := templateSpec["volumes"].([]interface{})
	assert.GreaterOrEqual(t, len(volumes), 3, "Should have at least 3 volumes")
}

func validateService(t *testing.T, obj map[string]interface{}) {
	metadata := obj["metadata"].(map[string]interface{})
	assert.Equal(t, "otel-collector-metrics", metadata["name"], "Service should be named otel-collector-metrics")
	assert.Equal(t, "otel-system", metadata["namespace"], "Service should be in otel-system namespace")
	
	spec := obj["spec"].(map[string]interface{})
	
	// Check selector
	selector := spec["selector"].(map[string]interface{})
	assert.Equal(t, "otel-collector", selector["app"], "Service should select app=otel-collector")
	
	// Check type
	assert.Equal(t, "ClusterIP", spec["type"], "Service should be ClusterIP type")
	
	// Check ports
	ports := spec["ports"].([]interface{})
	assert.GreaterOrEqual(t, len(ports), 2, "Service should expose at least 2 ports")
}

func verifyDeployment(t *testing.T, clientset kubernetes.Interface) {
	// Verify namespace exists
	ns, err := clientset.CoreV1().Namespaces().Get(context.TODO(), "otel-system", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, "otel-system", ns.Name)
	
	// Verify ServiceAccount exists
	sa, err := clientset.CoreV1().ServiceAccounts("otel-system").Get(context.TODO(), "otel-collector", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, "otel-collector", sa.Name)
	
	// Verify ClusterRole exists
	cr, err := clientset.RbacV1().ClusterRoles().Get(context.TODO(), "otel-collector", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, "otel-collector", cr.Name)
	
	// Verify ClusterRoleBinding exists
	crb, err := clientset.RbacV1().ClusterRoleBindings().Get(context.TODO(), "otel-collector", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, "otel-collector", crb.Name)
	
	// Verify ConfigMap exists
	cm, err := clientset.CoreV1().ConfigMaps("otel-system").Get(context.TODO(), "otel-collector-config", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, "otel-collector-config", cm.Name)
	
	// Verify DaemonSet exists
	ds, err := clientset.AppsV1().DaemonSets("otel-system").Get(context.TODO(), "otel-collector", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, "otel-collector", ds.Name)
	
	// Verify Service exists
	svc, err := clientset.CoreV1().Services("otel-system").Get(context.TODO(), "otel-collector-metrics", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, "otel-collector-metrics", svc.Name)
}

func createTestDaemonSet() *appsv1.DaemonSet {
	maxUnavailable := intstr.FromInt(1)
	readOnly := true
	allowPrivilegeEscalation := false
	runAsNonRoot := true
	runAsUser := int64(65534)
	hostPathType := corev1.HostPathDirectory
	
	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "otel-collector",
			Namespace: "otel-system",
			Labels: map[string]string{
				"app": "otel-collector",
			},
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "otel-collector",
				},
			},
			UpdateStrategy: appsv1.DaemonSetUpdateStrategy{
				Type: appsv1.RollingUpdateDaemonSetStrategyType,
				RollingUpdate: &appsv1.RollingUpdateDaemonSet{
					MaxUnavailable: &maxUnavailable,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "otel-collector",
					},
					Annotations: map[string]string{
						"prometheus.io/scrape": "true",
						"prometheus.io/port":   "8889",
						"prometheus.io/path":   "/metrics",
					},
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: "otel-collector",
					HostNetwork:        false,
					DNSPolicy:          corev1.DNSClusterFirst,
					Containers: []corev1.Container{
						{
							Name:  "otel-collector",
							Image: "otel/opentelemetry-collector-contrib:0.91.0",
							Args: []string{
								"--config=/conf/otel-collector-config.yaml",
							},
							Env: []corev1.EnvVar{
								{
									Name: "K8S_NODE_NAME",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											FieldPath: "spec.nodeName",
										},
									},
								},
								{
									Name: "K8S_POD_NAME",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											FieldPath: "metadata.name",
										},
									},
								},
								{
									Name: "K8S_POD_NAMESPACE",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											FieldPath: "metadata.namespace",
										},
									},
								},
								{
									Name: "K8S_POD_IP",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											FieldPath: "status.podIP",
										},
									},
								},
								{
									Name: "K8S_POD_UID",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											FieldPath: "metadata.uid",
										},
									},
								},
								{
									Name:  "GOMEMLIMIT",
									Value: "400MiB",
								},
							},
							Ports: []corev1.ContainerPort{
								{
									Name:          "metrics",
									ContainerPort: 8889,
									Protocol:      corev1.ProtocolTCP,
								},
								{
									Name:          "prometheus",
									ContainerPort: 8888,
									Protocol:      corev1.ProtocolTCP,
								},
								{
									Name:          "healthcheck",
									ContainerPort: 13133,
									Protocol:      corev1.ProtocolTCP,
								},
								{
									Name:          "pprof",
									ContainerPort: 1777,
									Protocol:      corev1.ProtocolTCP,
								},
							},
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/health",
										Port: intstr.FromInt(13133),
									},
								},
								InitialDelaySeconds: 30,
								PeriodSeconds:       10,
								TimeoutSeconds:      5,
								FailureThreshold:    3,
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/health",
										Port: intstr.FromInt(13133),
									},
								},
								InitialDelaySeconds: 10,
								PeriodSeconds:       10,
								TimeoutSeconds:      5,
								FailureThreshold:    3,
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("100m"),
									corev1.ResourceMemory: resource.MustParse("256Mi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("500m"),
									corev1.ResourceMemory: resource.MustParse("512Mi"),
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "otel-collector-config",
									MountPath: "/conf",
									ReadOnly:  readOnly,
								},
								{
									Name:      "varlog",
									MountPath: "/var/log",
									ReadOnly:  readOnly,
								},
								{
									Name:      "varlibdockercontainers",
									MountPath: "/var/lib/docker/containers",
									ReadOnly:  readOnly,
								},
							},
							SecurityContext: &corev1.SecurityContext{
								Capabilities: &corev1.Capabilities{
									Drop: []corev1.Capability{
										"ALL",
									},
								},
								ReadOnlyRootFilesystem:   &readOnly,
								AllowPrivilegeEscalation: &allowPrivilegeEscalation,
								RunAsNonRoot:             &runAsNonRoot,
								RunAsUser:                &runAsUser,
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "otel-collector-config",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "otel-collector-config",
									},
									Items: []corev1.KeyToPath{
										{
											Key:  "otel-collector-config.yaml",
											Path: "otel-collector-config.yaml",
										},
									},
								},
							},
						},
						{
							Name: "varlog",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: "/var/log",
									Type: &hostPathType,
								},
							},
						},
						{
							Name: "varlibdockercontainers",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: "/var/lib/docker/containers",
									Type: &hostPathType,
								},
							},
						},
					},
					Tolerations: []corev1.Toleration{
						{
							Key:      "node-role.kubernetes.io/master",
							Operator: corev1.TolerationOpExists,
							Effect:   corev1.TaintEffectNoSchedule,
						},
						{
							Key:      "node-role.kubernetes.io/control-plane",
							Operator: corev1.TolerationOpExists,
							Effect:   corev1.TaintEffectNoSchedule,
						},
					},
				},
			},
		},
	}
}

func getTestOTelConfig() string {
	return `receivers:
  filelog:
    include:
      - /var/log/containers/*.log
    exclude:
      - /var/log/containers/*otel-collector*.log
    include_file_path: true
    include_file_name: false
    operators:
      - type: json_parser
        id: parser-docker
        timestamp:
          parse_from: attributes.time
          layout: '%Y-%m-%dT%H:%M:%S.%LZ'
        output: extract_metadata_from_filepath
      
      - type: regex_parser
        id: extract_metadata_from_filepath
        regex: '^/var/log/containers/(?P<pod_name>[^_]+)_(?P<namespace>[^_]+)_(?P<container_name>.*)-(?P<container_id>[^.]+)\.log$'
        parse_from: attributes["log.file.path"]
        output: add_resource_attributes
      
      - type: move
        id: add_resource_attributes
        from: attributes.pod_name
        to: resource["k8s.pod.name"]
      - type: move
        from: attributes.namespace
        to: resource["k8s.namespace.name"]
      - type: move
        from: attributes.container_name
        to: resource["k8s.container.name"]
      - type: move
        from: attributes.container_id
        to: resource["container.id"]
      - type: move
        from: attributes.log
        to: body
      - type: move
        from: attributes.stream
        to: attributes["log.iostream"]
      
      - type: add
        field: severity_text
        value: INFO
        if: 'attributes["log.iostream"] == "stdout"'
      - type: add
        field: severity_text
        value: ERROR
        if: 'attributes["log.iostream"] == "stderr"'

  k8s_cluster:
    collection_interval: 10s
    node_conditions_to_report:
      - Ready
      - MemoryPressure
      - DiskPressure
      - PIDPressure
      - NetworkUnavailable

  hostmetrics:
    collection_interval: 30s
    scrapers:
      cpu:
      disk:
      filesystem:
      load:
      memory:
      network:
      paging:
      processes:

processors:
  k8sattributes:
    auth_type: "serviceAccount"
    passthrough: false
    extract:
      metadata:
        - k8s.namespace.name
        - k8s.pod.name
        - k8s.pod.uid
        - k8s.pod.start_time
        - k8s.deployment.name
        - k8s.node.name
        - k8s.container.name
      labels:
        - tag_name: app
          key: app
          from: pod
        - tag_name: team
          key: team
          from: pod
      annotations:
        - tag_name: version
          key: version
          from: pod
    pod_association:
      - sources:
        - from: resource_attribute
          name: k8s.pod.name
        - from: resource_attribute
          name: k8s.namespace.name

  resource:
    attributes:
      - key: environment
        value: test
        action: upsert
      - key: cluster.name
        value: mcp-registry-test
        action: upsert

  memory_limiter:
    check_interval: 1s
    limit_percentage: 75
    spike_limit_percentage: 15

  batch:
    timeout: 10s
    send_batch_size: 1024
    send_batch_max_size: 2048

exporters:
  debug:
    verbosity: basic
    sampling_initial: 5
    sampling_thereafter: 100

  otlp:
    endpoint: "otel-backend:4317"
    tls:
      insecure: true
    retry_on_failure:
      enabled: true
      initial_interval: 5s
      max_interval: 30s
      max_elapsed_time: 300s

  prometheus:
    endpoint: "0.0.0.0:8888"
    namespace: otel_collector
    const_labels:
      environment: test
    resource_to_telemetry_conversion:
      enabled: true

extensions:
  health_check:
    endpoint: 0.0.0.0:13133
    path: "/health"
    check_collector_pipeline:
      enabled: true
      interval: 5s
      exporter_failure_threshold: 5

  pprof:
    endpoint: 0.0.0.0:1777

  memory_ballast:
    size_in_percentage: 20

service:
  extensions: [health_check, pprof, memory_ballast]
  
  pipelines:
    logs:
      receivers: [filelog]
      processors: [memory_limiter, k8sattributes, resource, batch]
      exporters: [debug, otlp]
    
    metrics:
      receivers: [k8s_cluster, hostmetrics]
      processors: [memory_limiter, resource, batch]
      exporters: [prometheus, otlp]

  telemetry:
    logs:
      level: info
      initial_fields:
        service.name: otel-collector
        service.version: 0.1.0
    metrics:
      level: detailed
      address: 0.0.0.0:8889`
}

func findServicePort(ports []corev1.ServicePort, name string) *corev1.ServicePort {
	for _, port := range ports {
		if port.Name == name {
			return &port
		}
	}
	return nil
}

func findPolicyRule(rules []rbacv1.PolicyRule, apiGroup string) *rbacv1.PolicyRule {
	for _, rule := range rules {
		for _, group := range rule.APIGroups {
			if group == apiGroup {
				return &rule
			}
		}
	}
	return nil
}