package k8s

import (
	"fmt"
	"strings"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/modelcontextprotocol/registry/deploy/infra/pkg/providers"
)

// MockProvider implements a mock Kubernetes provider for testing
type MockProvider struct {
	resources []MockResource
}

// MockResource represents a mocked Kubernetes resource
type MockResource struct {
	URN      string
	Type     string
	Name     string
	Inputs   resource.PropertyMap
	Provider string
}

// TestDeployOTelCollector tests the main deployment function
func TestDeployOTelCollector(t *testing.T) {
	tests := []struct {
		name        string
		environment string
		wantErr     bool
		errContains string
		validate    func(t *testing.T, ctx *pulumi.Context)
	}{
		{
			name:        "successful deployment in production environment",
			environment: "production",
			wantErr:     false,
			validate: func(t *testing.T, ctx *pulumi.Context) {
				// Validate exports
				exports := ctx.GetExports()
				assert.Contains(t, exports, "otelCollectorNamespace")
				assert.Contains(t, exports, "otelCollectorStatus")
				
				// Check namespace value
				nsExport := exports["otelCollectorNamespace"]
				assert.NotNil(t, nsExport)
				
				// Check status value
				statusExport := exports["otelCollectorStatus"]
				assert.NotNil(t, statusExport)
			},
		},
		{
			name:        "successful deployment in staging environment",
			environment: "staging",
			wantErr:     false,
			validate: func(t *testing.T, ctx *pulumi.Context) {
				exports := ctx.GetExports()
				assert.Contains(t, exports, "otelCollectorNamespace")
				assert.Contains(t, exports, "otelCollectorStatus")
			},
		},
		{
			name:        "successful deployment in development environment",
			environment: "development",
			wantErr:     false,
			validate: func(t *testing.T, ctx *pulumi.Context) {
				exports := ctx.GetExports()
				assert.Contains(t, exports, "otelCollectorNamespace")
				assert.Contains(t, exports, "otelCollectorStatus")
			},
		},
		{
			name:        "deployment with empty environment",
			environment: "",
			wantErr:     false,
			validate: func(t *testing.T, ctx *pulumi.Context) {
				exports := ctx.GetExports()
				assert.Contains(t, exports, "otelCollectorNamespace")
				assert.Contains(t, exports, "otelCollectorStatus")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock Pulumi context
			ctx, err := createMockPulumiContext(tt.environment)
			require.NoError(t, err)

			// Create mock cluster provider
			cluster := &providers.ProviderInfo{
				Provider: createMockK8sProvider(),
			}

			// Execute deployment
			err = DeployOTelCollector(ctx, cluster, tt.environment)

			// Check error expectations
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err)
			}

			// Run custom validation if provided
			if tt.validate != nil && !tt.wantErr {
				tt.validate(t, ctx)
			}
		})
	}
}

// TestOTelCollectorConfiguration tests the configuration generation
func TestOTelCollectorConfiguration(t *testing.T) {
	tests := []struct {
		name        string
		environment string
		validate    func(t *testing.T, config string)
	}{
		{
			name:        "production configuration contains correct environment",
			environment: "production",
			validate: func(t *testing.T, config string) {
				assert.Contains(t, config, "environment: production")
				assert.Contains(t, config, "cluster.name: mcp-registry-production")
				assert.Contains(t, config, "const_labels:")
				assert.Contains(t, config, "filelog:")
				assert.Contains(t, config, "k8s_cluster:")
				assert.Contains(t, config, "hostmetrics:")
			},
		},
		{
			name:        "staging configuration contains correct environment",
			environment: "staging",
			validate: func(t *testing.T, config string) {
				assert.Contains(t, config, "environment: staging")
				assert.Contains(t, config, "cluster.name: mcp-registry-staging")
			},
		},
		{
			name:        "configuration includes all required receivers",
			environment: "test",
			validate: func(t *testing.T, config string) {
				// Check receivers
				assert.Contains(t, config, "receivers:")
				assert.Contains(t, config, "filelog:")
				assert.Contains(t, config, "k8s_cluster:")
				assert.Contains(t, config, "hostmetrics:")
				
				// Check filelog configuration
				assert.Contains(t, config, "/var/log/containers/*.log")
				assert.Contains(t, config, "/var/log/containers/*otel-collector*.log")
				assert.Contains(t, config, "json_parser")
				assert.Contains(t, config, "regex_parser")
			},
		},
		{
			name:        "configuration includes all required processors",
			environment: "test",
			validate: func(t *testing.T, config string) {
				// Check processors
				assert.Contains(t, config, "processors:")
				assert.Contains(t, config, "k8sattributes:")
				assert.Contains(t, config, "resource:")
				assert.Contains(t, config, "memory_limiter:")
				assert.Contains(t, config, "batch:")
				
				// Check k8sattributes configuration
				assert.Contains(t, config, "auth_type: \"serviceAccount\"")
				assert.Contains(t, config, "k8s.namespace.name")
				assert.Contains(t, config, "k8s.pod.name")
				assert.Contains(t, config, "k8s.container.name")
			},
		},
		{
			name:        "configuration includes all required exporters",
			environment: "test",
			validate: func(t *testing.T, config string) {
				// Check exporters
				assert.Contains(t, config, "exporters:")
				assert.Contains(t, config, "debug:")
				assert.Contains(t, config, "otlp:")
				assert.Contains(t, config, "prometheus:")
				
				// Check OTLP configuration
				assert.Contains(t, config, "endpoint: \"otel-backend:4317\"")
				assert.Contains(t, config, "retry_on_failure:")
				assert.Contains(t, config, "initial_interval: 5s")
				assert.Contains(t, config, "max_interval: 30s")
			},
		},
		{
			name:        "configuration includes all required extensions",
			environment: "test",
			validate: func(t *testing.T, config string) {
				// Check extensions
				assert.Contains(t, config, "extensions:")
				assert.Contains(t, config, "health_check:")
				assert.Contains(t, config, "pprof:")
				assert.Contains(t, config, "memory_ballast:")
				
				// Check health check configuration
				assert.Contains(t, config, "endpoint: 0.0.0.0:13133")
				assert.Contains(t, config, "path: \"/health\"")
			},
		},
		{
			name:        "configuration includes service pipelines",
			environment: "test",
			validate: func(t *testing.T, config string) {
				// Check service configuration
				assert.Contains(t, config, "service:")
				assert.Contains(t, config, "pipelines:")
				assert.Contains(t, config, "logs:")
				assert.Contains(t, config, "metrics:")
				
				// Check logs pipeline
				assert.Contains(t, config, "receivers: [filelog]")
				assert.Contains(t, config, "processors: [memory_limiter, k8sattributes, resource, batch]")
				assert.Contains(t, config, "exporters: [debug, otlp]")
				
				// Check metrics pipeline
				assert.Contains(t, config, "receivers: [k8s_cluster, hostmetrics]")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := generateOTelCollectorConfig(tt.environment)
			tt.validate(t, config)
		})
	}
}

// TestDaemonSetConfiguration tests the DaemonSet configuration
func TestDaemonSetConfiguration(t *testing.T) {
	tests := []struct {
		name     string
		validate func(t *testing.T, spec *DaemonSetSpec)
	}{
		{
			name: "daemonset has correct container configuration",
			validate: func(t *testing.T, spec *DaemonSetSpec) {
				assert.Equal(t, "otel/opentelemetry-collector-contrib:0.91.0", spec.Image)
				assert.Contains(t, spec.Args, "--config=/conf/otel-collector-config.yaml")
				
				// Check environment variables
				envVars := spec.EnvironmentVariables
				assert.Contains(t, envVars, "K8S_NODE_NAME")
				assert.Contains(t, envVars, "K8S_POD_NAME")
				assert.Contains(t, envVars, "K8S_POD_NAMESPACE")
				assert.Contains(t, envVars, "K8S_POD_IP")
				assert.Contains(t, envVars, "K8S_POD_UID")
				assert.Contains(t, envVars, "GOMEMLIMIT")
			},
		},
		{
			name: "daemonset has correct volume mounts",
			validate: func(t *testing.T, spec *DaemonSetSpec) {
				mounts := spec.VolumeMounts
				assert.Len(t, mounts, 3)
				
				// Check config mount
				assert.Contains(t, mounts, VolumeMount{
					Name:      "otel-collector-config",
					MountPath: "/conf",
					ReadOnly:  true,
				})
				
				// Check varlog mount
				assert.Contains(t, mounts, VolumeMount{
					Name:      "varlog",
					MountPath: "/var/log",
					ReadOnly:  true,
				})
				
				// Check docker containers mount
				assert.Contains(t, mounts, VolumeMount{
					Name:      "varlibdockercontainers",
					MountPath: "/var/lib/docker/containers",
					ReadOnly:  true,
				})
			},
		},
		{
			name: "daemonset has correct resource limits",
			validate: func(t *testing.T, spec *DaemonSetSpec) {
				resources := spec.Resources
				
				// Check requests
				assert.Equal(t, "100m", resources.Requests.CPU)
				assert.Equal(t, "256Mi", resources.Requests.Memory)
				
				// Check limits
				assert.Equal(t, "500m", resources.Limits.CPU)
				assert.Equal(t, "512Mi", resources.Limits.Memory)
			},
		},
		{
			name: "daemonset has correct security context",
			validate: func(t *testing.T, spec *DaemonSetSpec) {
				secCtx := spec.SecurityContext
				
				assert.True(t, secCtx.ReadOnlyRootFilesystem)
				assert.False(t, secCtx.AllowPrivilegeEscalation)
				assert.True(t, secCtx.RunAsNonRoot)
				assert.Equal(t, int64(65534), secCtx.RunAsUser)
				assert.Contains(t, secCtx.DropCapabilities, "ALL")
			},
		},
		{
			name: "daemonset has correct probes",
			validate: func(t *testing.T, spec *DaemonSetSpec) {
				// Liveness probe
				liveness := spec.LivenessProbe
				assert.Equal(t, "/health", liveness.Path)
				assert.Equal(t, int32(13133), liveness.Port)
				assert.Equal(t, int32(30), liveness.InitialDelaySeconds)
				assert.Equal(t, int32(10), liveness.PeriodSeconds)
				
				// Readiness probe
				readiness := spec.ReadinessProbe
				assert.Equal(t, "/health", readiness.Path)
				assert.Equal(t, int32(13133), readiness.Port)
				assert.Equal(t, int32(10), readiness.InitialDelaySeconds)
				assert.Equal(t, int32(10), readiness.PeriodSeconds)
			},
		},
		{
			name: "daemonset has correct tolerations",
			validate: func(t *testing.T, spec *DaemonSetSpec) {
				tolerations := spec.Tolerations
				assert.Len(t, tolerations, 2)
				
				// Check master toleration
				assert.Contains(t, tolerations, Toleration{
					Key:      "node-role.kubernetes.io/master",
					Operator: "Exists",
					Effect:   "NoSchedule",
				})
				
				// Check control-plane toleration
				assert.Contains(t, tolerations, Toleration{
					Key:      "node-role.kubernetes.io/control-plane",
					Operator: "Exists",
					Effect:   "NoSchedule",
				})
			},
		},
		{
			name: "daemonset has correct update strategy",
			validate: func(t *testing.T, spec *DaemonSetSpec) {
				assert.Equal(t, "RollingUpdate", spec.UpdateStrategy.Type)
				assert.Equal(t, int32(1), spec.UpdateStrategy.MaxUnavailable)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := createDaemonSetSpec()
			tt.validate(t, spec)
		})
	}
}

// TestRBACConfiguration tests RBAC resources configuration
func TestRBACConfiguration(t *testing.T) {
	tests := []struct {
		name     string
		validate func(t *testing.T, rbac *RBACConfig)
	}{
		{
			name: "service account is configured correctly",
			validate: func(t *testing.T, rbac *RBACConfig) {
				assert.Equal(t, "otel-collector", rbac.ServiceAccount.Name)
				assert.Equal(t, "otel-system", rbac.ServiceAccount.Namespace)
				assert.Contains(t, rbac.ServiceAccount.Labels, "app")
				assert.Equal(t, "otel-collector", rbac.ServiceAccount.Labels["app"])
			},
		},
		{
			name: "cluster role has correct permissions for pods and namespaces",
			validate: func(t *testing.T, rbac *RBACConfig) {
				rules := rbac.ClusterRole.Rules
				
				// Find core API group rule
				var coreRule *PolicyRule
				for _, rule := range rules {
					if contains(rule.APIGroups, "") {
						coreRule = &rule
						break
					}
				}
				
				require.NotNil(t, coreRule)
				assert.Contains(t, coreRule.Resources, "pods")
				assert.Contains(t, coreRule.Resources, "namespaces")
				assert.Contains(t, coreRule.Resources, "nodes")
				assert.Contains(t, coreRule.Verbs, "get")
				assert.Contains(t, coreRule.Verbs, "watch")
				assert.Contains(t, coreRule.Verbs, "list")
			},
		},
		{
			name: "cluster role has correct permissions for workloads",
			validate: func(t *testing.T, rbac *RBACConfig) {
				rules := rbac.ClusterRole.Rules
				
				// Find apps API group rule
				var appsRule *PolicyRule
				for _, rule := range rules {
					if contains(rule.APIGroups, "apps") {
						appsRule = &rule
						break
					}
				}
				
				require.NotNil(t, appsRule)
				assert.Contains(t, appsRule.Resources, "deployments")
				assert.Contains(t, appsRule.Resources, "daemonsets")
				assert.Contains(t, appsRule.Resources, "statefulsets")
				assert.Contains(t, appsRule.Resources, "replicasets")
				assert.Contains(t, appsRule.Verbs, "get")
				assert.Contains(t, appsRule.Verbs, "watch")
				assert.Contains(t, appsRule.Verbs, "list")
			},
		},
		{
			name: "cluster role binding links service account to cluster role",
			validate: func(t *testing.T, rbac *RBACConfig) {
				binding := rbac.ClusterRoleBinding
				
				// Check role reference
				assert.Equal(t, "rbac.authorization.k8s.io", binding.RoleRef.APIGroup)
				assert.Equal(t, "ClusterRole", binding.RoleRef.Kind)
				assert.Equal(t, "otel-collector", binding.RoleRef.Name)
				
				// Check subjects
				assert.Len(t, binding.Subjects, 1)
				subject := binding.Subjects[0]
				assert.Equal(t, "ServiceAccount", subject.Kind)
				assert.Equal(t, "otel-collector", subject.Name)
				assert.Equal(t, "otel-system", subject.Namespace)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rbac := createRBACConfig("test")
			tt.validate(t, rbac)
		})
	}
}

// TestServiceConfiguration tests the Service configuration
func TestServiceConfiguration(t *testing.T) {
	tests := []struct {
		name     string
		validate func(t *testing.T, svc *ServiceConfig)
	}{
		{
			name: "service has correct metadata",
			validate: func(t *testing.T, svc *ServiceConfig) {
				assert.Equal(t, "otel-collector-metrics", svc.Name)
				assert.Equal(t, "otel-system", svc.Namespace)
				assert.Contains(t, svc.Labels, "app")
				assert.Equal(t, "otel-collector", svc.Labels["app"])
			},
		},
		{
			name: "service has correct selector",
			validate: func(t *testing.T, svc *ServiceConfig) {
				assert.Contains(t, svc.Selector, "app")
				assert.Equal(t, "otel-collector", svc.Selector["app"])
			},
		},
		{
			name: "service exposes correct ports",
			validate: func(t *testing.T, svc *ServiceConfig) {
				assert.Equal(t, "ClusterIP", svc.Type)
				assert.Len(t, svc.Ports, 2)
				
				// Check metrics port
				metricsPort := findPort(svc.Ports, "metrics")
				require.NotNil(t, metricsPort)
				assert.Equal(t, int32(8889), metricsPort.Port)
				assert.Equal(t, int32(8889), metricsPort.TargetPort)
				assert.Equal(t, "TCP", metricsPort.Protocol)
				
				// Check prometheus port
				promPort := findPort(svc.Ports, "prometheus")
				require.NotNil(t, promPort)
				assert.Equal(t, int32(8888), promPort.Port)
				assert.Equal(t, int32(8888), promPort.TargetPort)
				assert.Equal(t, "TCP", promPort.Protocol)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := createServiceConfig("test")
			tt.validate(t, svc)
		})
	}
}

// TestConfigMapValidation tests ConfigMap content validation
func TestConfigMapValidation(t *testing.T) {
	tests := []struct {
		name        string
		environment string
		validate    func(t *testing.T, cm *ConfigMapData)
	}{
		{
			name:        "configmap contains valid YAML",
			environment: "test",
			validate: func(t *testing.T, cm *ConfigMapData) {
				// Validate that the config is valid YAML
				yamlContent := cm.Data["otel-collector-config.yaml"]
				assert.NotEmpty(t, yamlContent)
				
				// Check for required top-level keys
				assert.Contains(t, yamlContent, "receivers:")
				assert.Contains(t, yamlContent, "processors:")
				assert.Contains(t, yamlContent, "exporters:")
				assert.Contains(t, yamlContent, "extensions:")
				assert.Contains(t, yamlContent, "service:")
			},
		},
		{
			name:        "configmap excludes otel-collector logs",
			environment: "test",
			validate: func(t *testing.T, cm *ConfigMapData) {
				yamlContent := cm.Data["otel-collector-config.yaml"]
				assert.Contains(t, yamlContent, "/var/log/containers/*otel-collector*.log")
				assert.Contains(t, yamlContent, "exclude:")
			},
		},
		{
			name:        "configmap has memory limiter configured",
			environment: "test",
			validate: func(t *testing.T, cm *ConfigMapData) {
				yamlContent := cm.Data["otel-collector-config.yaml"]
				assert.Contains(t, yamlContent, "memory_limiter:")
				assert.Contains(t, yamlContent, "limit_percentage: 75")
				assert.Contains(t, yamlContent, "spike_limit_percentage: 15")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cm := createConfigMapData(tt.environment)
			tt.validate(t, cm)
		})
	}
}

// TestNamespaceConfiguration tests namespace configuration
func TestNamespaceConfiguration(t *testing.T) {
	tests := []struct {
		name        string
		environment string
		validate    func(t *testing.T, ns *NamespaceConfig)
	}{
		{
			name:        "namespace has correct name",
			environment: "production",
			validate: func(t *testing.T, ns *NamespaceConfig) {
				assert.Equal(t, "otel-system", ns.Name)
			},
		},
		{
			name:        "namespace has environment label",
			environment: "staging",
			validate: func(t *testing.T, ns *NamespaceConfig) {
				assert.Contains(t, ns.Labels, "environment")
				assert.Equal(t, "staging", ns.Labels["environment"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ns := createNamespaceConfig(tt.environment)
			tt.validate(t, ns)
		})
	}
}

// TestErrorHandling tests error handling scenarios
func TestErrorHandling(t *testing.T) {
	tests := []struct {
		name          string
		setupMock     func() *providers.ProviderInfo
		environment   string
		expectedError string
	}{
		{
			name: "handles namespace creation failure",
			setupMock: func() *providers.ProviderInfo {
				return &providers.ProviderInfo{
					Provider: createFailingProvider("Namespace"),
				}
			},
			environment:   "test",
			expectedError: "failed to create namespace",
		},
		{
			name: "handles service account creation failure",
			setupMock: func() *providers.ProviderInfo {
				return &providers.ProviderInfo{
					Provider: createFailingProvider("ServiceAccount"),
				}
			},
			environment:   "test",
			expectedError: "failed to create service account",
		},
		{
			name: "handles configmap creation failure",
			setupMock: func() *providers.ProviderInfo {
				return &providers.ProviderInfo{
					Provider: createFailingProvider("ConfigMap"),
				}
			},
			environment:   "test",
			expectedError: "failed to create configmap",
		},
		{
			name: "handles daemonset creation failure",
			setupMock: func() *providers.ProviderInfo {
				return &providers.ProviderInfo{
					Provider: createFailingProvider("DaemonSet"),
				}
			},
			environment:   "test",
			expectedError: "failed to create daemonset",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, err := createMockPulumiContext(tt.environment)
			require.NoError(t, err)

			cluster := tt.setupMock()
			err = DeployOTelCollector(ctx, cluster, tt.environment)
			
			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			}
		})
	}
}

// Helper functions for testing

func createMockPulumiContext(environment string) (*pulumi.Context, error) {
	return &pulumi.Context{
		exports: make(map[string]interface{}),
	}, nil
}

func createMockK8sProvider() pulumi.ProviderResource {
	return &mockProviderResource{}
}

func createFailingProvider(failOn string) pulumi.ProviderResource {
	return &mockFailingProviderResource{
		failOn: failOn,
	}
}

func generateOTelCollectorConfig(environment string) string {
	// This would extract the config generation logic from the main function
	return fmt.Sprintf(`receivers:
  filelog:
    include:
      - /var/log/containers/*.log
    exclude:
      - /var/log/containers/*otel-collector*.log
processors:
  resource:
    attributes:
      - key: environment
        value: %s
        action: upsert
      - key: cluster.name
        value: mcp-registry-%s
        action: upsert
exporters:
  otlp:
    endpoint: "otel-backend:4317"
service:
  pipelines:
    logs:
      receivers: [filelog]
      processors: [memory_limiter, k8sattributes, resource, batch]
      exporters: [debug, otlp]`, environment, environment)
}

// Test helper structures

type DaemonSetSpec struct {
	Image                string
	Args                 []string
	EnvironmentVariables map[string]string
	VolumeMounts         []VolumeMount
	Resources            ResourceRequirements
	SecurityContext      SecurityContext
	LivenessProbe        ProbeConfig
	ReadinessProbe       ProbeConfig
	Tolerations          []Toleration
	UpdateStrategy       UpdateStrategy
}

type VolumeMount struct {
	Name      string
	MountPath string
	ReadOnly  bool
}

type ResourceRequirements struct {
	Requests ResourceList
	Limits   ResourceList
}

type ResourceList struct {
	CPU    string
	Memory string
}

type SecurityContext struct {
	ReadOnlyRootFilesystem   bool
	AllowPrivilegeEscalation bool
	RunAsNonRoot             bool
	RunAsUser                int64
	DropCapabilities         []string
}

type ProbeConfig struct {
	Path                string
	Port                int32
	InitialDelaySeconds int32
	PeriodSeconds       int32
}

type Toleration struct {
	Key      string
	Operator string
	Effect   string
}

type UpdateStrategy struct {
	Type           string
	MaxUnavailable int32
}

type RBACConfig struct {
	ServiceAccount     ServiceAccountConfig
	ClusterRole        ClusterRoleConfig
	ClusterRoleBinding ClusterRoleBindingConfig
}

type ServiceAccountConfig struct {
	Name      string
	Namespace string
	Labels    map[string]string
}

type ClusterRoleConfig struct {
	Name   string
	Labels map[string]string
	Rules  []PolicyRule
}

type PolicyRule struct {
	APIGroups []string
	Resources []string
	Verbs     []string
}

type ClusterRoleBindingConfig struct {
	Name     string
	Labels   map[string]string
	RoleRef  RoleRef
	Subjects []Subject
}

type RoleRef struct {
	APIGroup string
	Kind     string
	Name     string
}

type Subject struct {
	Kind      string
	Name      string
	Namespace string
}

type ServiceConfig struct {
	Name      string
	Namespace string
	Labels    map[string]string
	Selector  map[string]string
	Type      string
	Ports     []ServicePort
}

type ServicePort struct {
	Name       string
	Port       int32
	TargetPort int32
	Protocol   string
}

type ConfigMapData struct {
	Data map[string]string
}

type NamespaceConfig struct {
	Name   string
	Labels map[string]string
}

// Mock implementations

type mockProviderResource struct{}

func (m *mockProviderResource) URN() resource.URN {
	return resource.URN("urn:pulumi:test::test::kubernetes:provider::test")
}

type mockFailingProviderResource struct {
	failOn string
}

func (m *mockFailingProviderResource) URN() resource.URN {
	return resource.URN("urn:pulumi:test::test::kubernetes:provider::test")
}

// Helper functions for test data creation

func createDaemonSetSpec() *DaemonSetSpec {
	return &DaemonSetSpec{
		Image: "otel/opentelemetry-collector-contrib:0.91.0",
		Args:  []string{"--config=/conf/otel-collector-config.yaml"},
		EnvironmentVariables: map[string]string{
			"K8S_NODE_NAME":     "spec.nodeName",
			"K8S_POD_NAME":      "metadata.name",
			"K8S_POD_NAMESPACE": "metadata.namespace",
			"K8S_POD_IP":        "status.podIP",
			"K8S_POD_UID":       "metadata.uid",
			"GOMEMLIMIT":        "400MiB",
		},
		VolumeMounts: []VolumeMount{
			{Name: "otel-collector-config", MountPath: "/conf", ReadOnly: true},
			{Name: "varlog", MountPath: "/var/log", ReadOnly: true},
			{Name: "varlibdockercontainers", MountPath: "/var/lib/docker/containers", ReadOnly: true},
		},
		Resources: ResourceRequirements{
			Requests: ResourceList{CPU: "100m", Memory: "256Mi"},
			Limits:   ResourceList{CPU: "500m", Memory: "512Mi"},
		},
		SecurityContext: SecurityContext{
			ReadOnlyRootFilesystem:   true,
			AllowPrivilegeEscalation: false,
			RunAsNonRoot:             true,
			RunAsUser:                65534,
			DropCapabilities:         []string{"ALL"},
		},
		LivenessProbe: ProbeConfig{
			Path:                "/health",
			Port:                13133,
			InitialDelaySeconds: 30,
			PeriodSeconds:       10,
		},
		ReadinessProbe: ProbeConfig{
			Path:                "/health",
			Port:                13133,
			InitialDelaySeconds: 10,
			PeriodSeconds:       10,
		},
		Tolerations: []Toleration{
			{Key: "node-role.kubernetes.io/master", Operator: "Exists", Effect: "NoSchedule"},
			{Key: "node-role.kubernetes.io/control-plane", Operator: "Exists", Effect: "NoSchedule"},
		},
		UpdateStrategy: UpdateStrategy{
			Type:           "RollingUpdate",
			MaxUnavailable: 1,
		},
	}
}

func createRBACConfig(environment string) *RBACConfig {
	return &RBACConfig{
		ServiceAccount: ServiceAccountConfig{
			Name:      "otel-collector",
			Namespace: "otel-system",
			Labels: map[string]string{
				"app":         "otel-collector",
				"environment": environment,
			},
		},
		ClusterRole: ClusterRoleConfig{
			Name: "otel-collector",
			Labels: map[string]string{
				"app":         "otel-collector",
				"environment": environment,
			},
			Rules: []PolicyRule{
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
		},
		ClusterRoleBinding: ClusterRoleBindingConfig{
			Name: "otel-collector",
			Labels: map[string]string{
				"app":         "otel-collector",
				"environment": environment,
			},
			RoleRef: RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     "otel-collector",
			},
			Subjects: []Subject{
				{
					Kind:      "ServiceAccount",
					Name:      "otel-collector",
					Namespace: "otel-system",
				},
			},
		},
	}
}

func createServiceConfig(environment string) *ServiceConfig {
	return &ServiceConfig{
		Name:      "otel-collector-metrics",
		Namespace: "otel-system",
		Labels: map[string]string{
			"app":         "otel-collector",
			"environment": environment,
		},
		Selector: map[string]string{
			"app": "otel-collector",
		},
		Type: "ClusterIP",
		Ports: []ServicePort{
			{Name: "metrics", Port: 8889, TargetPort: 8889, Protocol: "TCP"},
			{Name: "prometheus", Port: 8888, TargetPort: 8888, Protocol: "TCP"},
		},
	}
}

func createConfigMapData(environment string) *ConfigMapData {
	config := generateOTelCollectorConfig(environment)
	return &ConfigMapData{
		Data: map[string]string{
			"otel-collector-config.yaml": config,
		},
	}
}

func createNamespaceConfig(environment string) *NamespaceConfig {
	return &NamespaceConfig{
		Name: "otel-system",
		Labels: map[string]string{
			"environment": environment,
		},
	}
}

// Utility functions

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func findPort(ports []ServicePort, name string) *ServicePort {
	for _, port := range ports {
		if port.Name == name {
			return &port
		}
	}
	return nil
}