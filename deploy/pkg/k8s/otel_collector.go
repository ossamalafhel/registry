package k8s

import (
	appsv1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/apps/v1"
	corev1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/core/v1"
	metav1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/meta/v1"
	rbacv1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/rbac/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"

	"github.com/modelcontextprotocol/registry/deploy/infra/pkg/providers"
)

// DeployOTelCollector deploys the OpenTelemetry Collector as a DaemonSet
func DeployOTelCollector(ctx *pulumi.Context, cluster *providers.ProviderInfo, environment string) error {
	// Create namespace for OTel Collector
	ns, err := corev1.NewNamespace(ctx, "otel-system", &corev1.NamespaceArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name: pulumi.String("otel-system"),
			Labels: pulumi.StringMap{
				"environment": pulumi.String(environment),
			},
		},
	}, pulumi.Provider(cluster.Provider))
	if err != nil {
		return err
	}

	// Create ServiceAccount for OTel Collector
	sa, err := corev1.NewServiceAccount(ctx, "otel-collector", &corev1.ServiceAccountArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name:      pulumi.String("otel-collector"),
			Namespace: ns.Metadata.Name(),
			Labels: pulumi.StringMap{
				"app":         pulumi.String("otel-collector"),
				"environment": pulumi.String(environment),
			},
		},
	}, pulumi.Provider(cluster.Provider))
	if err != nil {
		return err
	}

	// Create ClusterRole for OTel Collector
	clusterRole, err := rbacv1.NewClusterRole(ctx, "otel-collector", &rbacv1.ClusterRoleArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name: pulumi.String("otel-collector"),
			Labels: pulumi.StringMap{
				"app":         pulumi.String("otel-collector"),
				"environment": pulumi.String(environment),
			},
		},
		Rules: rbacv1.PolicyRuleArray{
			&rbacv1.PolicyRuleArgs{
				ApiGroups: pulumi.StringArray{pulumi.String("")},
				Resources: pulumi.StringArray{
					pulumi.String("pods"),
					pulumi.String("namespaces"),
					pulumi.String("nodes"),
				},
				Verbs: pulumi.StringArray{
					pulumi.String("get"),
					pulumi.String("watch"),
					pulumi.String("list"),
				},
			},
			&rbacv1.PolicyRuleArgs{
				ApiGroups: pulumi.StringArray{pulumi.String("apps")},
				Resources: pulumi.StringArray{
					pulumi.String("deployments"),
					pulumi.String("daemonsets"),
					pulumi.String("statefulsets"),
					pulumi.String("replicasets"),
				},
				Verbs: pulumi.StringArray{
					pulumi.String("get"),
					pulumi.String("watch"),
					pulumi.String("list"),
				},
			},
		},
	}, pulumi.Provider(cluster.Provider))
	if err != nil {
		return err
	}

	// Create ClusterRoleBinding for OTel Collector
	_, err = rbacv1.NewClusterRoleBinding(ctx, "otel-collector", &rbacv1.ClusterRoleBindingArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name: pulumi.String("otel-collector"),
			Labels: pulumi.StringMap{
				"app":         pulumi.String("otel-collector"),
				"environment": pulumi.String(environment),
			},
		},
		RoleRef: &rbacv1.RoleRefArgs{
			ApiGroup: pulumi.String("rbac.authorization.k8s.io"),
			Kind:     pulumi.String("ClusterRole"),
			Name:     clusterRole.Metadata.Name(),
		},
		Subjects: rbacv1.SubjectArray{
			&rbacv1.SubjectArgs{
				Kind:      pulumi.String("ServiceAccount"),
				Name:      sa.Metadata.Name(),
				Namespace: ns.Metadata.Name(),
			},
		},
	}, pulumi.Provider(cluster.Provider))
	if err != nil {
		return err
	}

	// Create ConfigMap for OTel Collector configuration
	configMap, err := corev1.NewConfigMap(ctx, "otel-collector-config", &corev1.ConfigMapArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name:      pulumi.String("otel-collector-config"),
			Namespace: ns.Metadata.Name(),
			Labels: pulumi.StringMap{
				"app":         pulumi.String("otel-collector"),
				"environment": pulumi.String(environment),
			},
		},
		Data: pulumi.StringMap{
			"otel-collector-config.yaml": pulumi.String(`receivers:
  # Filelog receiver for container logs
  filelog:
    include:
      - /var/log/containers/*.log
    exclude:
      # Exclude OTel Collector's own logs to prevent infinite loop
      - /var/log/containers/*otel-collector*.log
    include_file_path: true
    include_file_name: false
    operators:
      # Parse container log format (Docker/CRI-O JSON logs)
      - type: json_parser
        id: parser-docker
        timestamp:
          parse_from: attributes.time
          layout: '%Y-%m-%dT%H:%M:%S.%LZ'
        output: extract_metadata_from_filepath
      
      # Extract metadata from file path
      - type: regex_parser
        id: extract_metadata_from_filepath
        regex: '^/var/log/containers/(?P<pod_name>[^_]+)_(?P<namespace>[^_]+)_(?P<container_name>.*)-(?P<container_id>[^.]+)\.log$'
        parse_from: attributes["log.file.path"]
        output: add_resource_attributes
      
      # Move parsed fields to resource attributes
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
      
      # Parse severity from stream (stdout/stderr)
      - type: add
        field: severity_text
        value: INFO
        if: 'attributes["log.iostream"] == "stdout"'
      - type: add
        field: severity_text
        value: ERROR
        if: 'attributes["log.iostream"] == "stderr"'

  # Kubernetes metadata receiver
  k8s_cluster:
    collection_interval: 10s
    node_conditions_to_report:
      - Ready
      - MemoryPressure
      - DiskPressure
      - PIDPressure
      - NetworkUnavailable

  # Host metrics receiver
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
  # Add Kubernetes metadata
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

  # Resource processor for adding environment info
  resource:
    attributes:
      - key: environment
        value: ` + environment + `
        action: upsert
      - key: cluster.name
        value: mcp-registry-` + environment + `
        action: upsert

  # Memory limiter to prevent OOM
  memory_limiter:
    check_interval: 1s
    limit_percentage: 75
    spike_limit_percentage: 15

  # Batch processor for better performance
  batch:
    timeout: 10s
    send_batch_size: 1024
    send_batch_max_size: 2048

exporters:
  # Debug exporter for troubleshooting (can be removed in production)
  debug:
    verbosity: basic
    sampling_initial: 5
    sampling_thereafter: 100

  # OTLP exporter for sending to backend (configure as needed)
  otlp:
    endpoint: "otel-backend:4317"  # Update with actual backend endpoint
    tls:
      insecure: true  # Set to false in production with proper TLS
    retry_on_failure:
      enabled: true
      initial_interval: 5s
      max_interval: 30s
      max_elapsed_time: 300s

  # Prometheus exporter for metrics
  prometheus:
    endpoint: "0.0.0.0:8888"
    namespace: otel_collector
    const_labels:
      environment: ` + environment + `
    resource_to_telemetry_conversion:
      enabled: true

extensions:
  # Health check extension
  health_check:
    endpoint: 0.0.0.0:13133
    path: "/health"
    check_collector_pipeline:
      enabled: true
      interval: 5s
      exporter_failure_threshold: 5

  # Performance profiler
  pprof:
    endpoint: 0.0.0.0:1777

  # Memory ballast for heap optimization
  memory_ballast:
    size_in_percentage: 20

service:
  extensions: [health_check, pprof, memory_ballast]
  
  pipelines:
    # Logs pipeline
    logs:
      receivers: [filelog]
      processors: [memory_limiter, k8sattributes, resource, batch]
      exporters: [debug, otlp]
    
    # Metrics pipeline
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
      address: 0.0.0.0:8889
`),
		},
	}, pulumi.Provider(cluster.Provider))
	if err != nil {
		return err
	}

	// Create DaemonSet for OTel Collector
	_, err = appsv1.NewDaemonSet(ctx, "otel-collector", &appsv1.DaemonSetArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name:      pulumi.String("otel-collector"),
			Namespace: ns.Metadata.Name(),
			Labels: pulumi.StringMap{
				"app":         pulumi.String("otel-collector"),
				"environment": pulumi.String(environment),
			},
		},
		Spec: &appsv1.DaemonSetSpecArgs{
			Selector: &metav1.LabelSelectorArgs{
				MatchLabels: pulumi.StringMap{
					"app": pulumi.String("otel-collector"),
				},
			},
			Template: &corev1.PodTemplateSpecArgs{
				Metadata: &metav1.ObjectMetaArgs{
					Labels: pulumi.StringMap{
						"app":         pulumi.String("otel-collector"),
						"environment": pulumi.String(environment),
					},
					Annotations: pulumi.StringMap{
						"prometheus.io/scrape": pulumi.String("true"),
						"prometheus.io/port":   pulumi.String("8889"),
						"prometheus.io/path":   pulumi.String("/metrics"),
					},
				},
				Spec: &corev1.PodSpecArgs{
					ServiceAccountName: sa.Metadata.Name(),
					HostNetwork:        pulumi.Bool(false),
					DnsPolicy:          pulumi.String("ClusterFirst"),
					Containers: corev1.ContainerArray{
						&corev1.ContainerArgs{
							Name:  pulumi.String("otel-collector"),
							Image: pulumi.String("otel/opentelemetry-collector-contrib:0.91.0"),
							Args: pulumi.StringArray{
								pulumi.String("--config=/conf/otel-collector-config.yaml"),
							},
							Env: corev1.EnvVarArray{
								&corev1.EnvVarArgs{
									Name: pulumi.String("K8S_NODE_NAME"),
									ValueFrom: &corev1.EnvVarSourceArgs{
										FieldRef: &corev1.ObjectFieldSelectorArgs{
											FieldPath: pulumi.String("spec.nodeName"),
										},
									},
								},
								&corev1.EnvVarArgs{
									Name: pulumi.String("K8S_POD_NAME"),
									ValueFrom: &corev1.EnvVarSourceArgs{
										FieldRef: &corev1.ObjectFieldSelectorArgs{
											FieldPath: pulumi.String("metadata.name"),
										},
									},
								},
								&corev1.EnvVarArgs{
									Name: pulumi.String("K8S_POD_NAMESPACE"),
									ValueFrom: &corev1.EnvVarSourceArgs{
										FieldRef: &corev1.ObjectFieldSelectorArgs{
											FieldPath: pulumi.String("metadata.namespace"),
										},
									},
								},
								&corev1.EnvVarArgs{
									Name: pulumi.String("K8S_POD_IP"),
									ValueFrom: &corev1.EnvVarSourceArgs{
										FieldRef: &corev1.ObjectFieldSelectorArgs{
											FieldPath: pulumi.String("status.podIP"),
										},
									},
								},
								&corev1.EnvVarArgs{
									Name: pulumi.String("K8S_POD_UID"),
									ValueFrom: &corev1.EnvVarSourceArgs{
										FieldRef: &corev1.ObjectFieldSelectorArgs{
											FieldPath: pulumi.String("metadata.uid"),
										},
									},
								},
								&corev1.EnvVarArgs{
									Name:  pulumi.String("GOMEMLIMIT"),
									Value: pulumi.String("400MiB"),
								},
							},
							Ports: corev1.ContainerPortArray{
								&corev1.ContainerPortArgs{
									Name:          pulumi.String("metrics"),
									ContainerPort: pulumi.Int(8889),
									Protocol:      pulumi.String("TCP"),
								},
								&corev1.ContainerPortArgs{
									Name:          pulumi.String("prometheus"),
									ContainerPort: pulumi.Int(8888),
									Protocol:      pulumi.String("TCP"),
								},
								&corev1.ContainerPortArgs{
									Name:          pulumi.String("healthcheck"),
									ContainerPort: pulumi.Int(13133),
									Protocol:      pulumi.String("TCP"),
								},
								&corev1.ContainerPortArgs{
									Name:          pulumi.String("pprof"),
									ContainerPort: pulumi.Int(1777),
									Protocol:      pulumi.String("TCP"),
								},
							},
							LivenessProbe: &corev1.ProbeArgs{
								HttpGet: &corev1.HTTPGetActionArgs{
									Path: pulumi.String("/health"),
									Port: pulumi.Int(13133),
								},
								InitialDelaySeconds: pulumi.Int(30),
								PeriodSeconds:       pulumi.Int(10),
								TimeoutSeconds:      pulumi.Int(5),
								FailureThreshold:    pulumi.Int(3),
							},
							ReadinessProbe: &corev1.ProbeArgs{
								HttpGet: &corev1.HTTPGetActionArgs{
									Path: pulumi.String("/health"),
									Port: pulumi.Int(13133),
								},
								InitialDelaySeconds: pulumi.Int(10),
								PeriodSeconds:       pulumi.Int(10),
								TimeoutSeconds:      pulumi.Int(5),
								FailureThreshold:    pulumi.Int(3),
							},
							Resources: &corev1.ResourceRequirementsArgs{
								Requests: pulumi.StringMap{
									"cpu":    pulumi.String("100m"),
									"memory": pulumi.String("256Mi"),
								},
								Limits: pulumi.StringMap{
									"cpu":    pulumi.String("500m"),
									"memory": pulumi.String("512Mi"),
								},
							},
							VolumeMounts: corev1.VolumeMountArray{
								&corev1.VolumeMountArgs{
									Name:      pulumi.String("otel-collector-config"),
									MountPath: pulumi.String("/conf"),
									ReadOnly:  pulumi.Bool(true),
								},
								&corev1.VolumeMountArgs{
									Name:      pulumi.String("varlog"),
									MountPath: pulumi.String("/var/log"),
									ReadOnly:  pulumi.Bool(true),
								},
								&corev1.VolumeMountArgs{
									Name:      pulumi.String("varlibdockercontainers"),
									MountPath: pulumi.String("/var/lib/docker/containers"),
									ReadOnly:  pulumi.Bool(true),
								},
							},
							SecurityContext: &corev1.SecurityContextArgs{
								Capabilities: &corev1.CapabilitiesArgs{
									Drop: pulumi.StringArray{pulumi.String("ALL")},
								},
								ReadOnlyRootFilesystem:   pulumi.Bool(true),
								AllowPrivilegeEscalation: pulumi.Bool(false),
								RunAsNonRoot:             pulumi.Bool(true),
								RunAsUser:                pulumi.Int(65534), // nobody user
							},
						},
					},
					Volumes: corev1.VolumeArray{
						&corev1.VolumeArgs{
							Name: pulumi.String("otel-collector-config"),
							ConfigMap: &corev1.ConfigMapVolumeSourceArgs{
								Name: configMap.Metadata.Name(),
								Items: corev1.KeyToPathArray{
									&corev1.KeyToPathArgs{
										Key:  pulumi.String("otel-collector-config.yaml"),
										Path: pulumi.String("otel-collector-config.yaml"),
									},
								},
							},
						},
						&corev1.VolumeArgs{
							Name: pulumi.String("varlog"),
							HostPath: &corev1.HostPathVolumeSourceArgs{
								Path: pulumi.String("/var/log"),
								Type: pulumi.String("Directory"),
							},
						},
						&corev1.VolumeArgs{
							Name: pulumi.String("varlibdockercontainers"),
							HostPath: &corev1.HostPathVolumeSourceArgs{
								Path: pulumi.String("/var/lib/docker/containers"),
								Type: pulumi.String("Directory"),
							},
						},
					},
					Tolerations: corev1.TolerationArray{
						&corev1.TolerationArgs{
							Key:      pulumi.String("node-role.kubernetes.io/master"),
							Operator: pulumi.String("Exists"),
							Effect:   pulumi.String("NoSchedule"),
						},
						&corev1.TolerationArgs{
							Key:      pulumi.String("node-role.kubernetes.io/control-plane"),
							Operator: pulumi.String("Exists"),
							Effect:   pulumi.String("NoSchedule"),
						},
					},
				},
			},
			UpdateStrategy: &appsv1.DaemonSetUpdateStrategyArgs{
				Type: pulumi.String("RollingUpdate"),
				RollingUpdate: &appsv1.RollingUpdateDaemonSetArgs{
					MaxUnavailable: pulumi.Int(1),
				},
			},
		},
	}, pulumi.Provider(cluster.Provider))
	if err != nil {
		return err
	}

	// Create Service for OTel Collector metrics endpoint
	_, err = corev1.NewService(ctx, "otel-collector-metrics", &corev1.ServiceArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name:      pulumi.String("otel-collector-metrics"),
			Namespace: ns.Metadata.Name(),
			Labels: pulumi.StringMap{
				"app":         pulumi.String("otel-collector"),
				"environment": pulumi.String(environment),
			},
		},
		Spec: &corev1.ServiceSpecArgs{
			Selector: pulumi.StringMap{
				"app": pulumi.String("otel-collector"),
			},
			Type: pulumi.String("ClusterIP"),
			Ports: corev1.ServicePortArray{
				&corev1.ServicePortArgs{
					Name:       pulumi.String("metrics"),
					Port:       pulumi.Int(8889),
					TargetPort: pulumi.Int(8889),
					Protocol:   pulumi.String("TCP"),
				},
				&corev1.ServicePortArgs{
					Name:       pulumi.String("prometheus"),
					Port:       pulumi.Int(8888),
					TargetPort: pulumi.Int(8888),
					Protocol:   pulumi.String("TCP"),
				},
			},
		},
	}, pulumi.Provider(cluster.Provider))
	if err != nil {
		return err
	}

	ctx.Export("otelCollectorNamespace", ns.Metadata.Name())
	ctx.Export("otelCollectorStatus", pulumi.String("deployed"))

	return nil
}