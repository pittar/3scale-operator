package component

import (
	appsv1 "github.com/openshift/api/apps/v1"
	routev1 "github.com/openshift/api/route/v1"
	templatev1 "github.com/openshift/api/template/v1"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
)

type Backend struct {
	options []string
}

func NewBackend(options []string) *Backend {
	backend := &Backend{
		options: options,
	}
	return backend
}

func (backend *Backend) AssembleIntoTemplate(template *templatev1.Template, otherComponents []Component) {
	backend.buildParameters(template)
	backend.buildObjects(template)
}

func (backend *Backend) PostProcess(template *templatev1.Template, otherComponents []Component) {

}

func (backend *Backend) buildParameters(template *templatev1.Template) {
	parameters := []templatev1.Parameter{}
	template.Parameters = append(template.Parameters, parameters...)
}

func (backend *Backend) buildObjects(template *templatev1.Template) {
	backendCronDeploymentConfig := backend.buildBackendCronDeploymentConfig()
	backendListenerDeploymentConfig := backend.buildBackendListenerDeploymentConfig()
	backendListenerService := backend.buildBackendListenerService()
	backendListenerRoute := backend.buildBackendListenerRoute()
	backendWorkerDeploymentConfig := backend.buildBackendWorkerDeploymentConfig()
	backendEnvConfigMap := backend.buildBackendEnvConfigMap()

	backendInternalApiCredsForSystem := backend.buildBackendInternalApiCredsForSystem()
	backendRedisSecrets := backend.buildBackendRedisSecrets()
	backendListenerSecrets := backend.buildBackendListenerSecrets()

	objects := []runtime.RawExtension{
		runtime.RawExtension{Object: backendCronDeploymentConfig},
		runtime.RawExtension{Object: backendListenerDeploymentConfig},
		runtime.RawExtension{Object: backendListenerService},
		runtime.RawExtension{Object: backendListenerRoute},
		runtime.RawExtension{Object: backendWorkerDeploymentConfig},
		runtime.RawExtension{Object: backendEnvConfigMap},
		runtime.RawExtension{Object: backendInternalApiCredsForSystem},
		runtime.RawExtension{Object: backendRedisSecrets},
		runtime.RawExtension{Object: backendListenerSecrets},
	}
	template.Objects = append(template.Objects, objects...)
}

func (backend *Backend) buildBackendWorkerDeploymentConfig() *appsv1.DeploymentConfig {
	return &appsv1.DeploymentConfig{
		TypeMeta: metav1.TypeMeta{
			Kind:       "DeploymentConfig",
			APIVersion: "apps.openshift.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   "backend-worker",
			Labels: map[string]string{"3scale.component": "backend", "3scale.component-element": "worker", "app": "${APP_LABEL}"},
		},
		Spec: appsv1.DeploymentConfigSpec{
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.DeploymentStrategyType("Rolling"),
				RollingParams: &appsv1.RollingDeploymentStrategyParams{
					UpdatePeriodSeconds: &[]int64{1}[0],
					IntervalSeconds:     &[]int64{1}[0],
					TimeoutSeconds:      &[]int64{1200}[0],
					MaxUnavailable: &intstr.IntOrString{
						Type:   intstr.Type(1),
						IntVal: 0,
						StrVal: "25%",
					},
					MaxSurge: &intstr.IntOrString{
						Type:   intstr.Type(1),
						IntVal: 0,
						StrVal: "25%"}},
			},
			MinReadySeconds: 0,
			Triggers: appsv1.DeploymentTriggerPolicies{
				appsv1.DeploymentTriggerPolicy{
					Type: appsv1.DeploymentTriggerType("ConfigChange"),
				}, appsv1.DeploymentTriggerPolicy{
					Type: appsv1.DeploymentTriggerType("ImageChange"),
					ImageChangeParams: &appsv1.DeploymentTriggerImageChangeParams{
						Automatic:      true,
						ContainerNames: []string{"backend-redis-svc", "backend-worker"},
						From: v1.ObjectReference{
							Kind: "ImageStreamTag",
							Name: "amp-backend:latest"}}},
			},
			Replicas: 1,
			Selector: map[string]string{"deploymentConfig": "backend-worker"},
			Template: &v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"3scale.component": "backend", "3scale.component-element": "worker", "app": "${APP_LABEL}", "deploymentConfig": "backend-worker"},
				},
				Spec: v1.PodSpec{InitContainers: []v1.Container{
					v1.Container{
						Name:  "backend-redis-svc",
						Image: "amp-backend:latest",
						Command: []string{
							"/opt/app/entrypoint.sh",
							"sh",
							"-c",
							"until rake connectivity:redis_storage_queue_check; do sleep $SLEEP_SECONDS; done",
						}, Env: []v1.EnvVar{
							createEnvVarFromValue("SLEEP_SECONDS", "1"),
							createEnvvarFromSecret("CONFIG_QUEUES_MASTER_NAME", "backend-redis", "REDIS_QUEUES_URL"),
						},
					},
				},
					Containers: []v1.Container{
						v1.Container{
							Name:  "backend-worker",
							Image: "amp-backend:latest",
							Args:  []string{"bin/3scale_backend_worker", "run"},
							Env:   backend.buildBackendWorkerEnv(),
							Resources: v1.ResourceRequirements{
								Limits: v1.ResourceList{
									v1.ResourceCPU:    resource.MustParse("1000m"),
									v1.ResourceMemory: resource.MustParse("300Mi"),
								},
								Requests: v1.ResourceList{
									v1.ResourceCPU:    resource.MustParse("150m"),
									v1.ResourceMemory: resource.MustParse("50Mi"),
								},
							},
							ImagePullPolicy: v1.PullPolicy("IfNotPresent"),
						},
					},
					ServiceAccountName: "amp"}},
		},
	}
}

func (backend *Backend) buildBackendCronDeploymentConfig() *appsv1.DeploymentConfig {
	return &appsv1.DeploymentConfig{
		TypeMeta: metav1.TypeMeta{
			Kind:       "DeploymentConfig",
			APIVersion: "apps.openshift.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   "backend-cron",
			Labels: map[string]string{"3scale.component": "backend", "3scale.component-element": "cron", "app": "${APP_LABEL}"},
		},
		Spec: appsv1.DeploymentConfigSpec{
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.DeploymentStrategyType("Rolling"),
				RollingParams: &appsv1.RollingDeploymentStrategyParams{
					UpdatePeriodSeconds: &[]int64{1}[0],
					IntervalSeconds:     &[]int64{1}[0],
					TimeoutSeconds:      &[]int64{1200}[0],
					MaxUnavailable: &intstr.IntOrString{
						Type:   intstr.Type(1),
						IntVal: 0,
						StrVal: "25%",
					},
					MaxSurge: &intstr.IntOrString{
						Type:   intstr.Type(1),
						IntVal: 0,
						StrVal: "25%"}},
			},
			MinReadySeconds: 0,
			Triggers: appsv1.DeploymentTriggerPolicies{
				appsv1.DeploymentTriggerPolicy{
					Type: appsv1.DeploymentTriggerType("ConfigChange"),
				}, appsv1.DeploymentTriggerPolicy{
					Type: appsv1.DeploymentTriggerType("ImageChange"),
					ImageChangeParams: &appsv1.DeploymentTriggerImageChangeParams{
						Automatic:      true,
						ContainerNames: []string{"backend-redis-svc", "backend-cron"},
						From: v1.ObjectReference{
							Kind: "ImageStreamTag",
							Name: "amp-backend:latest"}}},
			},
			Replicas: 1,
			Selector: map[string]string{"deploymentConfig": "backend-cron"},
			Template: &v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"3scale.component": "backend", "3scale.component-element": "cron", "app": "${APP_LABEL}", "deploymentConfig": "backend-cron"},
				},
				Spec: v1.PodSpec{InitContainers: []v1.Container{
					v1.Container{
						Name:  "backend-redis-svc",
						Image: "amp-backend:latest",
						Command: []string{
							"/opt/app/entrypoint.sh",
							"sh",
							"-c",
							"until rake connectivity:redis_storage_queue_check; do sleep $SLEEP_SECONDS; done",
						}, Env: []v1.EnvVar{
							createEnvVarFromValue("SLEEP_SECONDS", "1"),
							createEnvvarFromSecret("CONFIG_QUEUES_MASTER_NAME", "backend-redis", "REDIS_QUEUES_URL"),
						},
					},
				},
					Containers: []v1.Container{
						v1.Container{
							Name:  "backend-cron",
							Image: "amp-backend:latest",
							Args:  []string{"backend-cron"},
							Env:   backend.buildBackendCronEnv(),
							Resources: v1.ResourceRequirements{
								Limits: v1.ResourceList{
									v1.ResourceCPU:    resource.MustParse("150m"),
									v1.ResourceMemory: resource.MustParse("80Mi"),
								},
								Requests: v1.ResourceList{
									v1.ResourceCPU:    resource.MustParse("50m"),
									v1.ResourceMemory: resource.MustParse("40Mi"),
								},
							},

							ImagePullPolicy: v1.PullPolicy("IfNotPresent"),
						},
					},
					ServiceAccountName: "amp",
				}},
		},
	}
}

func (backend *Backend) buildBackendListenerDeploymentConfig() *appsv1.DeploymentConfig {
	return &appsv1.DeploymentConfig{
		TypeMeta: metav1.TypeMeta{
			Kind:       "DeploymentConfig",
			APIVersion: "apps.openshift.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   "backend-listener",
			Labels: map[string]string{"3scale.component": "backend", "3scale.component-element": "listener", "app": "${APP_LABEL}"},
		},
		Spec: appsv1.DeploymentConfigSpec{
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.DeploymentStrategyType("Rolling"),
				RollingParams: &appsv1.RollingDeploymentStrategyParams{
					UpdatePeriodSeconds: &[]int64{1}[0],
					IntervalSeconds:     &[]int64{1}[0],
					TimeoutSeconds:      &[]int64{600}[0],
					MaxUnavailable: &intstr.IntOrString{
						Type:   intstr.Type(1),
						IntVal: 0,
						StrVal: "25%",
					},
					MaxSurge: &intstr.IntOrString{
						Type:   intstr.Type(1),
						IntVal: 0,
						StrVal: "25%"}},
			},
			MinReadySeconds: 0,
			Triggers: appsv1.DeploymentTriggerPolicies{
				appsv1.DeploymentTriggerPolicy{
					Type: appsv1.DeploymentTriggerType("ConfigChange"),
				}, appsv1.DeploymentTriggerPolicy{
					Type: appsv1.DeploymentTriggerType("ImageChange"),
					ImageChangeParams: &appsv1.DeploymentTriggerImageChangeParams{
						Automatic:      true,
						ContainerNames: []string{"backend-listener"},
						From: v1.ObjectReference{
							Kind: "ImageStreamTag",
							Name: "amp-backend:latest"}}},
			},
			Replicas: 1,
			Selector: map[string]string{"deploymentConfig": "backend-listener"},
			Template: &v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"3scale.component": "backend", "3scale.component-element": "listener", "app": "${APP_LABEL}", "deploymentConfig": "backend-listener"},
				},
				Spec: v1.PodSpec{Containers: []v1.Container{
					v1.Container{
						Name:  "backend-listener",
						Image: "amp-backend:latest",
						Args:  []string{"bin/3scale_backend", "start", "-e", "production", "-p", "3000", "-x", "/dev/stdout"},
						Ports: []v1.ContainerPort{
							v1.ContainerPort{HostPort: 0,
								ContainerPort: 3000,
								Protocol:      v1.Protocol("TCP")},
						},
						Env: backend.buildBackendListenerEnv(),
						Resources: v1.ResourceRequirements{
							Limits: v1.ResourceList{
								v1.ResourceCPU:    resource.MustParse("1000m"),
								v1.ResourceMemory: resource.MustParse("700Mi"),
							},
							Requests: v1.ResourceList{
								v1.ResourceCPU:    resource.MustParse("500m"),
								v1.ResourceMemory: resource.MustParse("550Mi"),
							},
						},

						LivenessProbe: &v1.Probe{
							Handler: v1.Handler{TCPSocket: &v1.TCPSocketAction{
								Port: intstr.IntOrString{
									Type:   intstr.Type(0),
									IntVal: 3000}},
							},
							InitialDelaySeconds: 30,
							TimeoutSeconds:      0,
							PeriodSeconds:       10,
							SuccessThreshold:    0,
							FailureThreshold:    0,
						},
						ReadinessProbe: &v1.Probe{
							Handler: v1.Handler{HTTPGet: &v1.HTTPGetAction{
								Path: "/status",
								Port: intstr.IntOrString{
									Type:   intstr.Type(0),
									IntVal: 3000}},
							},
							InitialDelaySeconds: 30,
							TimeoutSeconds:      5,
							PeriodSeconds:       0,
							SuccessThreshold:    0,
							FailureThreshold:    0,
						},
						ImagePullPolicy: v1.PullPolicy("IfNotPresent"),
					},
				},
					ServiceAccountName: "amp",
				}},
		},
	}
}

func (backend *Backend) buildBackendListenerService() *v1.Service {
	return &v1.Service{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Service",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "backend-listener",
			Labels: map[string]string{
				"3scale.component":         "backend",
				"3scale.component-element": "listener",
				"app":                      "${APP_LABEL}",
			},
		},
		Spec: v1.ServiceSpec{
			Ports: []v1.ServicePort{
				v1.ServicePort{
					Name:     "http",
					Protocol: v1.Protocol("TCP"),
					Port:     3000,
					TargetPort: intstr.IntOrString{
						Type:   intstr.Type(0),
						IntVal: 3000,
					},
				},
			},
			Selector: map[string]string{"deploymentConfig": "backend-listener"},
		},
	}
}

func (backend *Backend) buildBackendListenerRoute() *routev1.Route {
	return &routev1.Route{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Route",
			APIVersion: "route.openshift.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   "backend",
			Labels: map[string]string{"app": "${APP_LABEL}", "3scale.component": "backend"},
		},
		Spec: routev1.RouteSpec{
			Host: "backend-${TENANT_NAME}.${WILDCARD_DOMAIN}",
			To: routev1.RouteTargetReference{
				Kind: "Service",
				Name: "backend-listener",
			},
			Port: &routev1.RoutePort{
				TargetPort: intstr.FromString("http"),
			},
			TLS: &routev1.TLSConfig{
				Termination:                   routev1.TLSTerminationType("edge"),
				InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyType("Allow")},
		},
	}
}

func (backend *Backend) buildBackendEnvConfigMap() *v1.ConfigMap {
	return &v1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   "backend-environment",
			Labels: map[string]string{"3scale.component": "backend", "app": "${APP_LABEL}"},
		},
		Data: map[string]string{
			"RACK_ENV": "production",
		},
	}
}

func (backend *Backend) buildBackendRedisSecrets() *v1.Secret {
	return &v1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "backend-redis",
			Labels: map[string]string{
				"app":              "${APP_LABEL}",
				"3scale.component": "backend",
			},
		},
		StringData: map[string]string{
			"REDIS_STORAGE_URL":            "redis://backend-redis:6379/0",
			"REDIS_QUEUES_URL":             "redis://backend-redis:6379/1",
			"REDIS_STORAGE_SENTINEL_HOSTS": "",
			"REDIS_STORAGE_SENTINEL_ROLE":  "",
			"REDIS_QUEUES_SENTINEL_HOSTS":  "",
			"REDIS_QUEUES_SENTINEL_ROLE":   "",
		},
		Type: v1.SecretTypeOpaque,
	}
}

func (backend *Backend) buildBackendCommonEnv() []v1.EnvVar {
	return []v1.EnvVar{
		createEnvvarFromSecret("CONFIG_REDIS_PROXY", "backend-redis", "REDIS_STORAGE_URL"),
		createEnvvarFromSecret("CONFIG_REDIS_SENTINEL_HOSTS", "backend-redis", "REDIS_STORAGE_SENTINEL_HOSTS"),
		createEnvvarFromSecret("CONFIG_REDIS_SENTINEL_ROLE", "backend-redis", "REDIS_STORAGE_SENTINEL_ROLE"),
		createEnvvarFromSecret("CONFIG_QUEUES_MASTER_NAME", "backend-redis", "REDIS_QUEUES_URL"),
		createEnvvarFromSecret("CONFIG_QUEUES_SENTINEL_HOSTS", "backend-redis", "REDIS_QUEUES_SENTINEL_HOSTS"),
		createEnvvarFromSecret("CONFIG_QUEUES_SENTINEL_ROLE", "backend-redis", "REDIS_QUEUES_SENTINEL_ROLE"),
		createEnvVarFromConfigMap("RACK_ENV", "backend-environment", "RACK_ENV"),
	}
}

func (backend *Backend) buildBackendWorkerEnv() []v1.EnvVar {
	result := []v1.EnvVar{}
	result = append(result, backend.buildBackendCommonEnv()...)
	result = append(result,
		createEnvvarFromSecret("CONFIG_EVENTS_HOOK", "system-events-hook", "URL"),
		createEnvvarFromSecret("CONFIG_EVENTS_HOOK_SHARED_SECRET", "system-events-hook", "PASSWORD"),
	)
	return result
}

func (backend *Backend) buildBackendCronEnv() []v1.EnvVar {
	result := []v1.EnvVar{}
	result = append(result, backend.buildBackendCommonEnv()...)
	return result
}

func (backend *Backend) buildBackendListenerEnv() []v1.EnvVar {
	result := []v1.EnvVar{}
	result = append(result, backend.buildBackendCommonEnv()...)
	result = append(result,
		createEnvVarFromValue("PUMA_WORKERS", "16"),
		createEnvvarFromSecret("CONFIG_INTERNAL_API_USER", "backend-internal-api", "username"),
		createEnvvarFromSecret("CONFIG_INTERNAL_API_PASSWORD", "backend-internal-api", "password"),
	)
	return result
}

func (backend *Backend) buildBackendInternalApiCredsForSystem() *v1.Secret {
	return &v1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "backend-internal-api",
			Labels: map[string]string{
				"app":              "${APP_LABEL}",
				"3scale.component": "backend",
			},
		},
		StringData: map[string]string{
			"username": "${SYSTEM_BACKEND_USERNAME}",
			"password": "${SYSTEM_BACKEND_PASSWORD}",
		},
		Type: v1.SecretTypeOpaque,
	}
}

func (backend *Backend) buildBackendListenerSecrets() *v1.Secret {
	return &v1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "backend-listener",
			Labels: map[string]string{
				"app":              "${APP_LABEL}",
				"3scale.component": "backend",
			},
		},
		StringData: map[string]string{
			"service_endpoint": "http://backend-listener:3000",
			"route_endpoint":   "https://backend-${TENANT_NAME}.${WILDCARD_DOMAIN}",
		},
		Type: v1.SecretTypeOpaque,
	}
}