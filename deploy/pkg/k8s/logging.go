package k8s

import (
	appsv1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/apps/v1"
	corev1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/core/v1"
	metav1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/meta/v1"
	rbacv1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/rbac/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"

	"github.com/modelcontextprotocol/registry/deploy/infra/pkg/providers"
)

// DeployLoggingStack deploys the OTel Collector DaemonSet for centralized logging
func DeployLoggingStack(ctx *pulumi.Context, cluster *providers.ProviderInfo, environment string) error {
	// Create namespace for logging
	ns, err := corev1.NewNamespace(ctx, "logging", &corev1.NamespaceArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name: pulumi.String("logging"),
		},
	}, pulumi.Provider(cluster.Provider))
	if err != nil {
		return err
	}

	// Create ServiceAccount for OTel Collector
	serviceAccount, err := corev1.NewServiceAccount(ctx, "otel-collector", &corev1.ServiceAccountArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name:      pulumi.String("otel-collector"),
			Namespace: ns.Metadata.Name().Elem(),
		},
	}, pulumi.Provider(cluster.Provider))
	if err != nil {
		return err
	}

	// Create ClusterRole for OTel Collector
	clusterRole, err := rbacv1.NewClusterRole(ctx, "otel-collector", &rbacv1.ClusterRoleArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name: pulumi.String("otel-collector"),
		},
		Rules: rbacv1.PolicyRuleArray{
			&rbacv1.PolicyRuleArgs{
				ApiGroups: pulumi.StringArray{
					pulumi.String(""),
				},
				Resources: pulumi.StringArray{
					pulumi.String("pods"),
					pulumi.String("namespaces"),
				},
				Verbs: pulumi.StringArray{
					pulumi.String("get"),
					pulumi.String("watch"),
					pulumi.String("list"),
				},
			},
			&rbacv1.PolicyRuleArgs{
				ApiGroups: pulumi.StringArray{
					pulumi.String("apps"),
				},
				Resources: pulumi.StringArray{
					pulumi.String("replicasets"),
				},
				Verbs: pulumi.StringArray{
					pulumi.String("get"),
					pulumi.String("watch"),
					pulumi.String("list"),
				},
			},
			&rbacv1.PolicyRuleArgs{
				ApiGroups: pulumi.StringArray{
					pulumi.String("extensions"),
				},
				Resources: pulumi.StringArray{
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

	// Create ClusterRoleBinding
	_, err = rbacv1.NewClusterRoleBinding(ctx, "otel-collector", &rbacv1.ClusterRoleBindingArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name: pulumi.String("otel-collector"),
		},
		RoleRef: &rbacv1.RoleRefArgs{
			ApiGroup: pulumi.String("rbac.authorization.k8s.io"),
			Kind:     pulumi.String("ClusterRole"),
			Name:     clusterRole.Metadata.Name().Elem(),
		},
		Subjects: rbacv1.SubjectArray{
			&rbacv1.SubjectArgs{
				Kind:      pulumi.String("ServiceAccount"),
				Name:      serviceAccount.Metadata.Name().Elem(),
				Namespace: ns.Metadata.Name().Elem(),
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
			Namespace: ns.Metadata.Name().Elem(),
		},
		Data: pulumi.StringMap{
			"otel-collector-config.yaml": pulumi.String(`
receivers:
  filelog:
    include:
      - /var/log/pods/*/*/*.log
    exclude:
      - /var/log/pods/*/otel-collector*/*.log
    start_at: beginning
    include_file_path: true
    include_file_name: false
    operators:
      - type: container
        id: container-parser
      - type: regex_parser
        regex: '^(?P<time>[^ ^Z]+Z?) (?P<stream>stdout|stderr) (?P<log_flags>[^\s]*) (?P<log>.*)$'
        timestamp:
          parse_from: attributes.time
          layout: '%Y-%m-%dT%H:%M:%S.%LZ'
      - type: move
        from: attributes.log
        to: body
      - type: remove
        field: attributes.time
      - type: remove
        field: attributes.log_flags
      - type: resource
        id: k8s.pod
        from_attributes:
          - key: k8s.namespace.name
            from: pod_namespace
          - key: k8s.pod.name
            from: pod_name
          - key: k8s.container.name
            from: container_name
          - key: k8s.pod.uid
            from: pod_uid
  
  # Kubernetes events receiver for cluster events
  k8s_events:
    auth_type: serviceAccount
    namespaces: [all]

processors:
  batch:
    timeout: 1s
    send_batch_size: 1024
    send_batch_max_size: 2048
  
  memory_limiter:
    check_interval: 1s
    limit_mib: 128
    spike_limit_mib: 32
  
  k8sattributes:
    auth_type: serviceAccount
    passthrough: false
    filter:
      node_from_env_var: KUBE_NODE_NAME
    extract:
      metadata:
        - k8s.pod.name
        - k8s.pod.uid
        - k8s.deployment.name
        - k8s.namespace.name
        - k8s.node.name
        - k8s.pod.start_time
      labels:
        - tag_name: app
          key: app
        - tag_name: component
          key: component
    pod_association:
      - sources:
        - from: resource_attribute
          name: k8s.pod.ip
      - sources:
        - from: resource_attribute
          name: k8s.pod.uid
      - sources:
        - from: connection

  resource:
    attributes:
      - key: cluster.name
        value: mcp-registry-` + environment + `
        action: insert
      - key: environment
        value: ` + environment + `
        action: insert

exporters:
  logging:
    verbosity: normal
    sampling_initial: 2
    sampling_thereafter: 500
  
  # Configure your preferred backend here (e.g., Elasticsearch, Loki, CloudWatch, etc.)
  # For now, we'll use debug exporter for development
  debug:
    verbosity: detailed
    sampling_initial: 5
    sampling_thereafter: 200

extensions:
  health_check:
    endpoint: :13133
  pprof:
    endpoint: :1888
  zpages:
    endpoint: :55679
  memory_ballast:
    size_mib: 64

service:
  extensions: [health_check, pprof, zpages, memory_ballast]
  pipelines:
    logs:
      receivers: [filelog, k8s_events]
      processors: [memory_limiter, k8sattributes, resource, batch]
      exporters: [logging, debug]
  telemetry:
    logs:
      level: info
    metrics:
      level: detailed
      address: :8888
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
			Namespace: ns.Metadata.Name().Elem(),
			Labels: pulumi.StringMap{
				"app":       pulumi.String("otel-collector"),
				"component": pulumi.String("logging"),
			},
		},
		Spec: &appsv1.DaemonSetSpecArgs{
			Selector: &metav1.LabelSelectorArgs{
				MatchLabels: pulumi.StringMap{
					"app":       pulumi.String("otel-collector"),
					"component": pulumi.String("logging"),
				},
			},
			Template: &corev1.PodTemplateSpecArgs{
				Metadata: &metav1.ObjectMetaArgs{
					Labels: pulumi.StringMap{
						"app":       pulumi.String("otel-collector"),
						"component": pulumi.String("logging"),
					},
					Annotations: pulumi.StringMap{
						"prometheus.io/scrape": pulumi.String("true"),
						"prometheus.io/port":   pulumi.String("8888"),
						"prometheus.io/path":   pulumi.String("/metrics"),
					},
				},
				Spec: &corev1.PodSpecArgs{
					ServiceAccountName: serviceAccount.Metadata.Name().Elem(),
					Containers: corev1.ContainerArray{
						&corev1.ContainerArgs{
							Name:  pulumi.String("otel-collector"),
							Image: pulumi.String("otel/opentelemetry-collector-contrib:0.91.0"),
							Args: pulumi.StringArray{
								pulumi.String("--config=/conf/otel-collector-config.yaml"),
							},
							Resources: &corev1.ResourceRequirementsArgs{
								Requests: pulumi.StringMap{
									"cpu":    pulumi.String("100m"),
									"memory": pulumi.String("128Mi"),
								},
								Limits: pulumi.StringMap{
									"cpu":    pulumi.String("500m"),
									"memory": pulumi.String("256Mi"),
								},
							},
							VolumeMounts: corev1.VolumeMountArray{
								&corev1.VolumeMountArgs{
									Name:      pulumi.String("config"),
									MountPath: pulumi.String("/conf"),
									ReadOnly:  pulumi.Bool(true),
								},
								&corev1.VolumeMountArgs{
									Name:      pulumi.String("varlogpods"),
									MountPath: pulumi.String("/var/log/pods"),
									ReadOnly:  pulumi.Bool(true),
								},
								&corev1.VolumeMountArgs{
									Name:      pulumi.String("varlibdockercontainers"),
									MountPath: pulumi.String("/var/lib/docker/containers"),
									ReadOnly:  pulumi.Bool(true),
								},
							},
							Env: corev1.EnvVarArray{
								&corev1.EnvVarArgs{
									Name: pulumi.String("KUBE_NODE_NAME"),
									ValueFrom: &corev1.EnvVarSourceArgs{
										FieldRef: &corev1.ObjectFieldSelectorArgs{
											FieldPath: pulumi.String("spec.nodeName"),
										},
									},
								},
								&corev1.EnvVarArgs{
									Name:  pulumi.String("GOGC"),
									Value: pulumi.String("80"),
								},
							},
							Ports: corev1.ContainerPortArray{
								&corev1.ContainerPortArgs{
									Name:          pulumi.String("health"),
									ContainerPort: pulumi.Int(13133),
									Protocol:      pulumi.String("TCP"),
								},
								&corev1.ContainerPortArgs{
									Name:          pulumi.String("metrics"),
									ContainerPort: pulumi.Int(8888),
									Protocol:      pulumi.String("TCP"),
								},
								&corev1.ContainerPortArgs{
									Name:          pulumi.String("pprof"),
									ContainerPort: pulumi.Int(1888),
									Protocol:      pulumi.String("TCP"),
								},
								&corev1.ContainerPortArgs{
									Name:          pulumi.String("zpages"),
									ContainerPort: pulumi.Int(55679),
									Protocol:      pulumi.String("TCP"),
								},
							},
							LivenessProbe: &corev1.ProbeArgs{
								HttpGet: &corev1.HTTPGetActionArgs{
									Path: pulumi.String("/"),
									Port: pulumi.Int(13133),
								},
								InitialDelaySeconds: pulumi.Int(30),
								PeriodSeconds:       pulumi.Int(30),
							},
							ReadinessProbe: &corev1.ProbeArgs{
								HttpGet: &corev1.HTTPGetActionArgs{
									Path: pulumi.String("/"),
									Port: pulumi.Int(13133),
								},
								InitialDelaySeconds: pulumi.Int(5),
								PeriodSeconds:       pulumi.Int(10),
							},
						},
					},
					Volumes: corev1.VolumeArray{
						&corev1.VolumeArgs{
							Name: pulumi.String("config"),
							ConfigMap: &corev1.ConfigMapVolumeSourceArgs{
								Name: configMap.Metadata.Name().Elem(),
								Items: corev1.KeyToPathArray{
									&corev1.KeyToPathArgs{
										Key:  pulumi.String("otel-collector-config.yaml"),
										Path: pulumi.String("otel-collector-config.yaml"),
									},
								},
							},
						},
						&corev1.VolumeArgs{
							Name: pulumi.String("varlogpods"),
							HostPath: &corev1.HostPathVolumeSourceArgs{
								Path: pulumi.String("/var/log/pods"),
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
					HostNetwork: pulumi.Bool(false),
					DnsPolicy:   pulumi.String("ClusterFirst"),
				},
			},
			UpdateStrategy: &appsv1.DaemonSetUpdateStrategyArgs{
				Type: pulumi.String("RollingUpdate"),
				RollingUpdate: &appsv1.RollingUpdateDaemonSetArgs{
					MaxUnavailable: pulumi.IntPtr(1),
				},
			},
		},
	}, pulumi.Provider(cluster.Provider))
	if err != nil {
		return err
	}

	// Create Service for OTel Collector metrics endpoint
	_, err = corev1.NewService(ctx, "otel-collector", &corev1.ServiceArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name:      pulumi.String("otel-collector"),
			Namespace: ns.Metadata.Name().Elem(),
			Labels: pulumi.StringMap{
				"app":       pulumi.String("otel-collector"),
				"component": pulumi.String("logging"),
			},
		},
		Spec: &corev1.ServiceSpecArgs{
			Selector: pulumi.StringMap{
				"app":       pulumi.String("otel-collector"),
				"component": pulumi.String("logging"),
			},
			Ports: corev1.ServicePortArray{
				&corev1.ServicePortArgs{
					Name:       pulumi.String("metrics"),
					Port:       pulumi.Int(8888),
					TargetPort: pulumi.Int(8888),
					Protocol:   pulumi.String("TCP"),
				},
				&corev1.ServicePortArgs{
					Name:       pulumi.String("health"),
					Port:       pulumi.Int(13133),
					TargetPort: pulumi.Int(13133),
					Protocol:   pulumi.String("TCP"),
				},
			},
			Type: pulumi.String("ClusterIP"),
		},
	}, pulumi.Provider(cluster.Provider))
	if err != nil {
		return err
	}

	return nil
}