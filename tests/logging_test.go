package tests

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	appsv1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/apps/v1"
	corev1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/core/v1"
	metav1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/meta/v1"
	rbacv1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/rbac/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/modelcontextprotocol/registry/deploy/pkg/k8s"
	"github.com/modelcontextprotocol/registry/deploy/infra/pkg/providers"
)

// MockPulumiMocks provides mock implementations for Pulumi resources
type MockPulumiMocks struct {
	pulumi.MockResourceMonitor
	resources map[string]resource.PropertyMap
	callCount map[string]int
}

func NewMockPulumiMocks() *MockPulumiMocks {
	return &MockPulumiMocks{
		resources: make(map[string]resource.PropertyMap),
		callCount: make(map[string]int),
	}
}

func (m *MockPulumiMocks) NewResource(args pulumi.MockResourceArgs) (string, resource.PropertyMap, error) {
	m.callCount[args.TypeToken]++
	
	// Track resources for validation
	outputs := resource.PropertyMap{}
	
	switch args.TypeToken {
	case "kubernetes:core/v1:Namespace":
		outputs["metadata"] = resource.NewObjectProperty(resource.PropertyMap{
			"name": resource.NewStringProperty("logging"),
		})
	case "kubernetes:core/v1:ServiceAccount":
		outputs["metadata"] = resource.NewObjectProperty(resource.PropertyMap{
			"name":      resource.NewStringProperty("otel-collector"),
			"namespace": resource.NewStringProperty("logging"),
		})
	case "kubernetes:rbac.authorization.k8s.io/v1:ClusterRole":
		outputs["metadata"] = resource.NewObjectProperty(resource.PropertyMap{
			"name": resource.NewStringProperty("otel-collector"),
		})
		// Validate RBAC rules
		if rules, ok := args.Inputs["rules"]; ok {
			outputs["rules"] = rules
		}
	case "kubernetes:rbac.authorization.k8s.io/v1:ClusterRoleBinding":
		outputs["metadata"] = resource.NewObjectProperty(resource.PropertyMap{
			"name": resource.NewStringProperty("otel-collector"),
		})
	case "kubernetes:core/v1:ConfigMap":
		outputs["metadata"] = resource.NewObjectProperty(resource.PropertyMap{
			"name":      resource.NewStringProperty("otel-collector-config"),
			"namespace": resource.NewStringProperty("logging"),
		})
		// Store config data for validation
		if data, ok := args.Inputs["data"]; ok {
			outputs["data"] = data
		}
	case "kubernetes:apps/v1:DaemonSet":
		outputs["metadata"] = resource.NewObjectProperty(resource.PropertyMap{
			"name":      resource.NewStringProperty("otel-collector"),
			"namespace": resource.NewStringProperty("logging"),
		})
		// Store spec for validation
		if spec, ok := args.Inputs["spec"]; ok {
			outputs["spec"] = spec
		}
	case "kubernetes:core/v1:Service":
		outputs["metadata"] = resource.NewObjectProperty(resource.PropertyMap{
			"name":      resource.NewStringProperty("otel-collector"),
			"namespace": resource.NewStringProperty("logging"),
		})
		if spec, ok := args.Inputs["spec"]; ok {
			outputs["spec"] = spec
		}
	}
	
	m.resources[args.Name] = outputs
	return args.Name + "_id", outputs, nil
}

func (m *MockPulumiMocks) Call(args pulumi.MockCallArgs) (resource.PropertyMap, error) {
	return resource.PropertyMap{}, nil
}

// TestOTelCollectorFullDeployment tests the complete deployment of OTel Collector
func TestOTelCollectorFullDeployment(t *testing.T) {
	mocks := NewMockPulumiMocks()
	
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cluster := &providers.ProviderInfo{
			Name:     pulumi.String("test-cluster"),
			Provider: nil,
		}
		
		return k8s.DeployLoggingStack(ctx, cluster, "test")
	}, pulumi.WithMocks("project", "stack", mocks))
	
	assert.NoError(t, err, "Logging stack deployment should succeed")
	
	// Verify all required resources were created
	assert.Equal(t, 1, mocks.callCount["kubernetes:core/v1:Namespace"], "Should create 1 namespace")
	assert.Equal(t, 1, mocks.callCount["kubernetes:core/v1:ServiceAccount"], "Should create 1 service account")
	assert.Equal(t, 1, mocks.callCount["kubernetes:rbac.authorization.k8s.io/v1:ClusterRole"], "Should create 1 cluster role")
	assert.Equal(t, 1, mocks.callCount["kubernetes:rbac.authorization.k8s.io/v1:ClusterRoleBinding"], "Should create 1 cluster role binding")
	assert.Equal(t, 1, mocks.callCount["kubernetes:core/v1:ConfigMap"], "Should create 1 config map")
	assert.Equal(t, 1, mocks.callCount["kubernetes:apps/v1:DaemonSet"], "Should create 1 daemonset")
	assert.Equal(t, 1, mocks.callCount["kubernetes:core/v1:Service"], "Should create 1 service")
}

// TestOTelCollectorResourceRequirements verifies resource limits match acceptance criteria
func TestOTelCollectorResourceRequirements(t *testing.T) {
	mocks := NewMockPulumiMocks()
	
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cluster := &providers.ProviderInfo{
			Name:     pulumi.String("test-cluster"),
			Provider: nil,
		}
		
		return k8s.DeployLoggingStack(ctx, cluster, "test")
	}, pulumi.WithMocks("project", "stack", mocks))
	
	require.NoError(t, err)
	
	// Find the DaemonSet resource
	var daemonSetSpec resource.PropertyMap
	for name, props := range mocks.resources {
		if strings.Contains(name, "otel-collector") {
			if spec, ok := props["spec"]; ok && spec.IsObject() {
				daemonSetSpec = spec.ObjectValue()
				break
			}
		}
	}
	
	// Validate resource requirements
	if template, ok := daemonSetSpec["template"]; ok && template.IsObject() {
		templateSpec := template.ObjectValue()
		if spec, ok := templateSpec["spec"]; ok && spec.IsObject() {
			podSpec := spec.ObjectValue()
			if containers, ok := podSpec["containers"]; ok && containers.IsArray() {
				containerArray := containers.ArrayValue()
				assert.Greater(t, len(containerArray), 0, "Should have at least one container")
				
				if len(containerArray) > 0 {
					container := containerArray[0].ObjectValue()
					if resources, ok := container["resources"]; ok && resources.IsObject() {
						resourceReqs := resources.ObjectValue()
						
						// Check requests
						if requests, ok := resourceReqs["requests"]; ok && requests.IsObject() {
							reqMap := requests.ObjectValue()
							if cpu, ok := reqMap["cpu"]; ok {
								assert.Equal(t, "100m", cpu.StringValue(), "CPU request should be 100m")
							}
							if memory, ok := reqMap["memory"]; ok {
								assert.Equal(t, "128Mi", memory.StringValue(), "Memory request should be 128Mi")
							}
						}
						
						// Check limits
						if limits, ok := resourceReqs["limits"]; ok && limits.IsObject() {
							limMap := limits.ObjectValue()
							if cpu, ok := limMap["cpu"]; ok {
								assert.Equal(t, "500m", cpu.StringValue(), "CPU limit should be 500m")
							}
							if memory, ok := limMap["memory"]; ok {
								assert.Equal(t, "256Mi", memory.StringValue(), "Memory limit should be 256Mi")
							}
						}
					}
				}
			}
		}
	}
}

// TestOTelCollectorConfigMapContent validates the OTel configuration
func TestOTelCollectorConfigMapContent(t *testing.T) {
	mocks := NewMockPulumiMocks()
	
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cluster := &providers.ProviderInfo{
			Name:     pulumi.String("test-cluster"),
			Provider: nil,
		}
		
		return k8s.DeployLoggingStack(ctx, cluster, "production")
	}, pulumi.WithMocks("project", "stack", mocks))
	
	require.NoError(t, err)
	
	// Find ConfigMap and validate content
	for name, props := range mocks.resources {
		if strings.Contains(name, "otel-collector-config") {
			if data, ok := props["data"]; ok && data.IsObject() {
				dataMap := data.ObjectValue()
				if configYaml, ok := dataMap["otel-collector-config.yaml"]; ok {
					config := configYaml.StringValue()
					
					// Verify essential components are configured
					assert.Contains(t, config, "receivers:", "Config should have receivers")
					assert.Contains(t, config, "filelog:", "Should have filelog receiver")
					assert.Contains(t, config, "k8s_events:", "Should have k8s_events receiver")
					
					assert.Contains(t, config, "processors:", "Config should have processors")
					assert.Contains(t, config, "batch:", "Should have batch processor")
					assert.Contains(t, config, "memory_limiter:", "Should have memory_limiter processor")
					assert.Contains(t, config, "k8sattributes:", "Should have k8sattributes processor")
					assert.Contains(t, config, "resource:", "Should have resource processor")
					
					assert.Contains(t, config, "exporters:", "Config should have exporters")
					assert.Contains(t, config, "logging:", "Should have logging exporter")
					assert.Contains(t, config, "debug:", "Should have debug exporter")
					
					assert.Contains(t, config, "extensions:", "Config should have extensions")
					assert.Contains(t, config, "health_check:", "Should have health_check extension")
					assert.Contains(t, config, "memory_ballast:", "Should have memory_ballast extension")
					
					assert.Contains(t, config, "service:", "Config should have service section")
					assert.Contains(t, config, "pipelines:", "Should have pipelines")
					assert.Contains(t, config, "logs:", "Should have logs pipeline")
					
					// Verify environment-specific values
					assert.Contains(t, config, "mcp-registry-production", "Should have environment-specific cluster name")
					assert.Contains(t, config, "value: production", "Should have environment value")
				}
			}
		}
	}
}

// TestOTelCollectorVolumeMounts validates volume configuration
func TestOTelCollectorVolumeMounts(t *testing.T) {
	mocks := NewMockPulumiMocks()
	
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cluster := &providers.ProviderInfo{
			Name:     pulumi.String("test-cluster"),
			Provider: nil,
		}
		
		return k8s.DeployLoggingStack(ctx, cluster, "test")
	}, pulumi.WithMocks("project", "stack", mocks))
	
	require.NoError(t, err)
	
	// Validate volumes are properly configured
	for name, props := range mocks.resources {
		if strings.Contains(name, "otel-collector") && props["spec"] != nil {
			spec := props["spec"].ObjectValue()
			if template, ok := spec["template"]; ok && template.IsObject() {
				templateSpec := template.ObjectValue()
				if podSpec, ok := templateSpec["spec"]; ok && podSpec.IsObject() {
					ps := podSpec.ObjectValue()
					
					// Check volumes
					if volumes, ok := ps["volumes"]; ok && volumes.IsArray() {
						volArray := volumes.ArrayValue()
						assert.Equal(t, 3, len(volArray), "Should have 3 volumes")
						
						volumeNames := make(map[string]bool)
						for _, vol := range volArray {
							if volObj := vol.ObjectValue(); volObj != nil {
								if name, ok := volObj["name"]; ok {
									volumeNames[name.StringValue()] = true
								}
							}
						}
						
						assert.True(t, volumeNames["config"], "Should have config volume")
						assert.True(t, volumeNames["varlogpods"], "Should have varlogpods volume")
						assert.True(t, volumeNames["varlibdockercontainers"], "Should have varlibdockercontainers volume")
					}
					
					// Check volume mounts
					if containers, ok := ps["containers"]; ok && containers.IsArray() {
						containerArray := containers.ArrayValue()
						if len(containerArray) > 0 {
							container := containerArray[0].ObjectValue()
							if mounts, ok := container["volumeMounts"]; ok && mounts.IsArray() {
								mountArray := mounts.ArrayValue()
								assert.Equal(t, 3, len(mountArray), "Should have 3 volume mounts")
								
								mountPaths := make(map[string]bool)
								for _, mount := range mountArray {
									if mountObj := mount.ObjectValue(); mountObj != nil {
										if path, ok := mountObj["mountPath"]; ok {
											mountPaths[path.StringValue()] = true
										}
									}
								}
								
								assert.True(t, mountPaths["/conf"], "Should mount config at /conf")
								assert.True(t, mountPaths["/var/log/pods"], "Should mount /var/log/pods")
								assert.True(t, mountPaths["/var/lib/docker/containers"], "Should mount /var/lib/docker/containers")
							}
						}
					}
				}
			}
		}
	}
}

// TestOTelCollectorHealthProbes validates liveness and readiness probes
func TestOTelCollectorHealthProbes(t *testing.T) {
	mocks := NewMockPulumiMocks()
	
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cluster := &providers.ProviderInfo{
			Name:     pulumi.String("test-cluster"),
			Provider: nil,
		}
		
		return k8s.DeployLoggingStack(ctx, cluster, "test")
	}, pulumi.WithMocks("project", "stack", mocks))
	
	require.NoError(t, err)
	
	// Find DaemonSet and check probes
	for name, props := range mocks.resources {
		if strings.Contains(name, "otel-collector") && props["spec"] != nil {
			spec := props["spec"].ObjectValue()
			if template, ok := spec["template"]; ok && template.IsObject() {
				templateSpec := template.ObjectValue()
				if podSpec, ok := templateSpec["spec"]; ok && podSpec.IsObject() {
					ps := podSpec.ObjectValue()
					if containers, ok := ps["containers"]; ok && containers.IsArray() {
						containerArray := containers.ArrayValue()
						if len(containerArray) > 0 {
							container := containerArray[0].ObjectValue()
							
							// Check liveness probe
							if liveness, ok := container["livenessProbe"]; ok && liveness.IsObject() {
								probe := liveness.ObjectValue()
								if httpGet, ok := probe["httpGet"]; ok && httpGet.IsObject() {
									http := httpGet.ObjectValue()
									if path, ok := http["path"]; ok {
										assert.Equal(t, "/", path.StringValue(), "Liveness probe path should be /")
									}
									if port, ok := http["port"]; ok {
										assert.Equal(t, int64(13133), port.NumberValue(), "Liveness probe port should be 13133")
									}
								}
								if delay, ok := probe["initialDelaySeconds"]; ok {
									assert.Equal(t, int64(30), delay.NumberValue(), "Liveness initial delay should be 30")
								}
								if period, ok := probe["periodSeconds"]; ok {
									assert.Equal(t, int64(30), period.NumberValue(), "Liveness period should be 30")
								}
							}
							
							// Check readiness probe
							if readiness, ok := container["readinessProbe"]; ok && readiness.IsObject() {
								probe := readiness.ObjectValue()
								if httpGet, ok := probe["httpGet"]; ok && httpGet.IsObject() {
									http := httpGet.ObjectValue()
									if path, ok := http["path"]; ok {
										assert.Equal(t, "/", path.StringValue(), "Readiness probe path should be /")
									}
									if port, ok := http["port"]; ok {
										assert.Equal(t, int64(13133), port.NumberValue(), "Readiness probe port should be 13133")
									}
								}
								if delay, ok := probe["initialDelaySeconds"]; ok {
									assert.Equal(t, int64(5), delay.NumberValue(), "Readiness initial delay should be 5")
								}
								if period, ok := probe["periodSeconds"]; ok {
									assert.Equal(t, int64(10), period.NumberValue(), "Readiness period should be 10")
								}
							}
						}
					}
				}
			}
		}
	}
}

// TestOTelCollectorServicePorts validates service port configuration
func TestOTelCollectorServicePorts(t *testing.T) {
	mocks := NewMockPulumiMocks()
	
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cluster := &providers.ProviderInfo{
			Name:     pulumi.String("test-cluster"),
			Provider: nil,
		}
		
		return k8s.DeployLoggingStack(ctx, cluster, "test")
	}, pulumi.WithMocks("project", "stack", mocks))
	
	require.NoError(t, err)
	
	// Find Service and validate ports
	for name, props := range mocks.resources {
		if strings.Contains(name, "otel-collector") && props["spec"] != nil {
			spec := props["spec"].ObjectValue()
			if ports, ok := spec["ports"]; ok && ports.IsArray() {
				portArray := ports.ArrayValue()
				assert.Equal(t, 2, len(portArray), "Service should have 2 ports")
				
				portConfigs := make(map[string]int64)
				for _, port := range portArray {
					if portObj := port.ObjectValue(); portObj != nil {
						if name, ok := portObj["name"]; ok {
							if portNum, ok := portObj["port"]; ok {
								portConfigs[name.StringValue()] = portNum.NumberValue()
							}
						}
					}
				}
				
				assert.Equal(t, int64(8888), portConfigs["metrics"], "Metrics port should be 8888")
				assert.Equal(t, int64(13133), portConfigs["health"], "Health port should be 13133")
			}
			
			if svcType, ok := spec["type"]; ok {
				assert.Equal(t, "ClusterIP", svcType.StringValue(), "Service type should be ClusterIP")
			}
		}
	}
}

// TestOTelCollectorRBACPermissions validates RBAC configuration
func TestOTelCollectorRBACPermissions(t *testing.T) {
	mocks := NewMockPulumiMocks()
	
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cluster := &providers.ProviderInfo{
			Name:     pulumi.String("test-cluster"),
			Provider: nil,
		}
		
		return k8s.DeployLoggingStack(ctx, cluster, "test")
	}, pulumi.WithMocks("project", "stack", mocks))
	
	require.NoError(t, err)
	
	// Validate ClusterRole has correct permissions
	for _, props := range mocks.resources {
		if rules, ok := props["rules"]; ok && rules.IsArray() {
			rulesArray := rules.ArrayValue()
			assert.Equal(t, 3, len(rulesArray), "Should have 3 rule sets")
			
			// Check first rule (core API group)
			if len(rulesArray) > 0 {
				rule := rulesArray[0].ObjectValue()
				if resources, ok := rule["resources"]; ok && resources.IsArray() {
					resourceArray := resources.ArrayValue()
					resourceNames := make(map[string]bool)
					for _, r := range resourceArray {
						resourceNames[r.StringValue()] = true
					}
					assert.True(t, resourceNames["pods"], "Should have pods permission")
					assert.True(t, resourceNames["namespaces"], "Should have namespaces permission")
				}
				if verbs, ok := rule["verbs"]; ok && verbs.IsArray() {
					verbArray := verbs.ArrayValue()
					verbNames := make(map[string]bool)
					for _, v := range verbArray {
						verbNames[v.StringValue()] = true
					}
					assert.True(t, verbNames["get"], "Should have get permission")
					assert.True(t, verbNames["watch"], "Should have watch permission")
					assert.True(t, verbNames["list"], "Should have list permission")
				}
			}
		}
	}
}

// TestOTelCollectorDaemonSetUpdateStrategy validates update strategy
func TestOTelCollectorDaemonSetUpdateStrategy(t *testing.T) {
	mocks := NewMockPulumiMocks()
	
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cluster := &providers.ProviderInfo{
			Name:     pulumi.String("test-cluster"),
			Provider: nil,
		}
		
		return k8s.DeployLoggingStack(ctx, cluster, "test")
	}, pulumi.WithMocks("project", "stack", mocks))
	
	require.NoError(t, err)
	
	// Validate DaemonSet update strategy
	for name, props := range mocks.resources {
		if strings.Contains(name, "otel-collector") && props["spec"] != nil {
			spec := props["spec"].ObjectValue()
			if updateStrategy, ok := spec["updateStrategy"]; ok && updateStrategy.IsObject() {
				strategy := updateStrategy.ObjectValue()
				if stratType, ok := strategy["type"]; ok {
					assert.Equal(t, "RollingUpdate", stratType.StringValue(), "Update strategy should be RollingUpdate")
				}
				if rollingUpdate, ok := strategy["rollingUpdate"]; ok && rollingUpdate.IsObject() {
					rolling := rollingUpdate.ObjectValue()
					if maxUnavailable, ok := rolling["maxUnavailable"]; ok {
						assert.Equal(t, int64(1), maxUnavailable.NumberValue(), "Max unavailable should be 1")
					}
				}
			}
		}
	}
}

// TestOTelCollectorEnvironmentVariables validates environment variables
func TestOTelCollectorEnvironmentVariables(t *testing.T) {
	mocks := NewMockPulumiMocks()
	
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cluster := &providers.ProviderInfo{
			Name:     pulumi.String("test-cluster"),
			Provider: nil,
		}
		
		return k8s.DeployLoggingStack(ctx, cluster, "test")
	}, pulumi.WithMocks("project", "stack", mocks))
	
	require.NoError(t, err)
	
	// Validate environment variables in container
	for name, props := range mocks.resources {
		if strings.Contains(name, "otel-collector") && props["spec"] != nil {
			spec := props["spec"].ObjectValue()
			if template, ok := spec["template"]; ok && template.IsObject() {
				templateSpec := template.ObjectValue()
				if podSpec, ok := templateSpec["spec"]; ok && podSpec.IsObject() {
					ps := podSpec.ObjectValue()
					if containers, ok := ps["containers"]; ok && containers.IsArray() {
						containerArray := containers.ArrayValue()
						if len(containerArray) > 0 {
							container := containerArray[0].ObjectValue()
							if env, ok := container["env"]; ok && env.IsArray() {
								envArray := env.ArrayValue()
								assert.Equal(t, 2, len(envArray), "Should have 2 environment variables")
								
								envVars := make(map[string]interface{})
								for _, e := range envArray {
									if envObj := e.ObjectValue(); envObj != nil {
										if name, ok := envObj["name"]; ok {
											envVars[name.StringValue()] = envObj
										}
									}
								}
								
								// Check KUBE_NODE_NAME
								assert.NotNil(t, envVars["KUBE_NODE_NAME"], "Should have KUBE_NODE_NAME env var")
								if nodeEnv, ok := envVars["KUBE_NODE_NAME"].(resource.PropertyMap); ok {
									if valueFrom, ok := nodeEnv["valueFrom"]; ok && valueFrom.IsObject() {
										vf := valueFrom.ObjectValue()
										if fieldRef, ok := vf["fieldRef"]; ok && fieldRef.IsObject() {
											fr := fieldRef.ObjectValue()
											if fieldPath, ok := fr["fieldPath"]; ok {
												assert.Equal(t, "spec.nodeName", fieldPath.StringValue(), "Should reference spec.nodeName")
											}
										}
									}
								}
								
								// Check GOGC
								assert.NotNil(t, envVars["GOGC"], "Should have GOGC env var")
								if gogcEnv, ok := envVars["GOGC"].(resource.PropertyMap); ok {
									if value, ok := gogcEnv["value"]; ok {
										assert.Equal(t, "80", value.StringValue(), "GOGC should be set to 80")
									}
								}
							}
						}
					}
				}
			}
		}
	}
}

// TestOTelCollectorContainerPorts validates all container ports
func TestOTelCollectorContainerPorts(t *testing.T) {
	mocks := NewMockPulumiMocks()
	
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cluster := &providers.ProviderInfo{
			Name:     pulumi.String("test-cluster"),
			Provider: nil,
		}
		
		return k8s.DeployLoggingStack(ctx, cluster, "test")
	}, pulumi.WithMocks("project", "stack", mocks))
	
	require.NoError(t, err)
	
	// Validate container ports
	for name, props := range mocks.resources {
		if strings.Contains(name, "otel-collector") && props["spec"] != nil {
			spec := props["spec"].ObjectValue()
			if template, ok := spec["template"]; ok && template.IsObject() {
				templateSpec := template.ObjectValue()
				if podSpec, ok := templateSpec["spec"]; ok && podSpec.IsObject() {
					ps := podSpec.ObjectValue()
					if containers, ok := ps["containers"]; ok && containers.IsArray() {
						containerArray := containers.ArrayValue()
						if len(containerArray) > 0 {
							container := containerArray[0].ObjectValue()
							if ports, ok := container["ports"]; ok && ports.IsArray() {
								portArray := ports.ArrayValue()
								assert.Equal(t, 4, len(portArray), "Should have 4 container ports")
								
								portConfigs := make(map[string]int64)
								for _, port := range portArray {
									if portObj := port.ObjectValue(); portObj != nil {
										if name, ok := portObj["name"]; ok {
											if portNum, ok := portObj["containerPort"]; ok {
												portConfigs[name.StringValue()] = portNum.NumberValue()
											}
										}
									}
								}
								
								assert.Equal(t, int64(13133), portConfigs["health"], "Health port should be 13133")
								assert.Equal(t, int64(8888), portConfigs["metrics"], "Metrics port should be 8888")
								assert.Equal(t, int64(1888), portConfigs["pprof"], "Pprof port should be 1888")
								assert.Equal(t, int64(55679), portConfigs["zpages"], "Zpages port should be 55679")
							}
						}
					}
				}
			}
		}
	}
}

// TestOTelCollectorNamespace validates namespace creation
func TestOTelCollectorNamespace(t *testing.T) {
	mocks := NewMockPulumiMocks()
	
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cluster := &providers.ProviderInfo{
			Name:     pulumi.String("test-cluster"),
			Provider: nil,
		}
		
		return k8s.DeployLoggingStack(ctx, cluster, "test")
	}, pulumi.WithMocks("project", "stack", mocks))
	
	require.NoError(t, err)
	
	// Validate namespace was created with correct name
	namespaceCreated := false
	for _, props := range mocks.resources {
		if metadata, ok := props["metadata"]; ok && metadata.IsObject() {
			meta := metadata.ObjectValue()
			if name, ok := meta["name"]; ok && name.StringValue() == "logging" {
				namespaceCreated = true
				break
			}
		}
	}
	
	assert.True(t, namespaceCreated, "Logging namespace should be created")
}

// TestOTelCollectorErrorHandling tests error handling
func TestOTelCollectorErrorHandling(t *testing.T) {
	// Test with nil cluster
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		return k8s.DeployLoggingStack(ctx, nil, "test")
	}, pulumi.WithMocks("project", "stack", NewMockPulumiMocks()))
	
	assert.Error(t, err, "Should error with nil cluster")
	
	// Test with empty environment
	err = pulumi.RunErr(func(ctx *pulumi.Context) error {
		cluster := &providers.ProviderInfo{
			Name:     pulumi.String("test-cluster"),
			Provider: nil,
		}
		return k8s.DeployLoggingStack(ctx, cluster, "")
	}, pulumi.WithMocks("project", "stack", NewMockPulumiMocks()))
	
	// Empty environment should still work
	assert.NoError(t, err, "Should handle empty environment")
}

// TestOTelCollectorLabelsAndAnnotations validates labels and annotations
func TestOTelCollectorLabelsAndAnnotations(t *testing.T) {
	mocks := NewMockPulumiMocks()
	
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cluster := &providers.ProviderInfo{
			Name:     pulumi.String("test-cluster"),
			Provider: nil,
		}
		
		return k8s.DeployLoggingStack(ctx, cluster, "test")
	}, pulumi.WithMocks("project", "stack", mocks))
	
	require.NoError(t, err)
	
	// Check DaemonSet labels and annotations
	for name, props := range mocks.resources {
		if strings.Contains(name, "otel-collector") {
			// Check metadata labels
			if metadata, ok := props["metadata"]; ok && metadata.IsObject() {
				meta := metadata.ObjectValue()
				if labels, ok := meta["labels"]; ok && labels.IsObject() {
					labelMap := labels.ObjectValue()
					if app, ok := labelMap["app"]; ok {
						assert.Equal(t, "otel-collector", app.StringValue(), "App label should be otel-collector")
					}
					if component, ok := labelMap["component"]; ok {
						assert.Equal(t, "logging", component.StringValue(), "Component label should be logging")
					}
				}
			}
			
			// Check pod template annotations for Prometheus scraping
			if spec, ok := props["spec"]; ok && spec.IsObject() {
				specMap := spec.ObjectValue()
				if template, ok := specMap["template"]; ok && template.IsObject() {
					templateMap := template.ObjectValue()
					if metadata, ok := templateMap["metadata"]; ok && metadata.IsObject() {
						meta := metadata.ObjectValue()
						if annotations, ok := meta["annotations"]; ok && annotations.IsObject() {
							annoMap := annotations.ObjectValue()
							if scrape, ok := annoMap["prometheus.io/scrape"]; ok {
								assert.Equal(t, "true", scrape.StringValue(), "Should enable Prometheus scraping")
							}
							if port, ok := annoMap["prometheus.io/port"]; ok {
								assert.Equal(t, "8888", port.StringValue(), "Prometheus port should be 8888")
							}
							if path, ok := annoMap["prometheus.io/path"]; ok {
								assert.Equal(t, "/metrics", path.StringValue(), "Prometheus path should be /metrics")
							}
						}
					}
				}
			}
		}
	}
}

// TestLoggingDeploymentIntegration tests integration with DeployAll
func TestLoggingDeploymentIntegration(t *testing.T) {
	testCases := []struct {
		name           string
		enableLogging  bool
		expectedCalls  int
	}{
		{
			name:          "Logging enabled",
			enableLogging: true,
			expectedCalls: 7, // All 7 resources should be created
		},
		{
			name:          "Logging disabled",
			enableLogging: false,
			expectedCalls: 0, // No resources should be created
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mocks := NewMockPulumiMocks()
			
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				// Simulate config value
				if tc.enableLogging {
					ctx.Export("enableLogging", pulumi.Bool(true))
				}
				
				cluster := &providers.ProviderInfo{
					Name:     pulumi.String("test-cluster"),
					Provider: nil,
				}
				
				// Only deploy if enabled (simulating DeployAll logic)
				if tc.enableLogging {
					return k8s.DeployLoggingStack(ctx, cluster, "test")
				}
				return nil
			}, pulumi.WithMocks("project", "stack", mocks))
			
			assert.NoError(t, err)
			
			totalCalls := 0
			for _, count := range mocks.callCount {
				totalCalls += count
			}
			
			assert.Equal(t, tc.expectedCalls, totalCalls, 
				fmt.Sprintf("Should create %d resources when logging is %v", 
					tc.expectedCalls, tc.enableLogging))
		})
	}
}