/*
Copyright 2024 The OpenYurt Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package tenantcluster

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	appsv1alpha1 "github.com/openyurtio/openyurt/pkg/apis/apps/v1alpha1"
)

// createEtcdResources creates the etcd Service and StatefulSet
func (r *ReconcileTenantCluster) createEtcdResources(ctx context.Context, tc *appsv1alpha1.TenantCluster) error {
	// Create headless service for etcd
	etcdService := r.buildEtcdService(tc)
	if err := controllerutil.SetControllerReference(tc, etcdService, r.Scheme()); err != nil {
		return fmt.Errorf("failed to set controller reference on etcd service: %w", err)
	}
	if err := r.Create(ctx, etcdService); err != nil {
		return fmt.Errorf("failed to create etcd service: %w", err)
	}
	klog.Info(Format("Created etcd service for TenantCluster %s", tc.Name))

	// Create etcd StatefulSet
	etcdSts := r.buildEtcdStatefulSet(tc)
	if err := controllerutil.SetControllerReference(tc, etcdSts, r.Scheme()); err != nil {
		return fmt.Errorf("failed to set controller reference on etcd statefulset: %w", err)
	}
	if err := r.Create(ctx, etcdSts); err != nil {
		return fmt.Errorf("failed to create etcd statefulset: %w", err)
	}
	klog.Info(Format("Created etcd statefulset for TenantCluster %s", tc.Name))

	r.recorder.Event(tc, corev1.EventTypeNormal, "EtcdCreated", "Created etcd resources")
	return nil
}

func (r *ReconcileTenantCluster) buildEtcdService(tc *appsv1alpha1.TenantCluster) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "etcd",
			Namespace: tc.GetNamespace(),
			Labels: map[string]string{
				"app":                          "etcd",
				"tenantcluster.openyurt.io":    tc.Name,
				"app.kubernetes.io/managed-by": "tenantcluster-controller",
			},
		},
		Spec: corev1.ServiceSpec{
			Type:                     corev1.ServiceTypeClusterIP,
			ClusterIP:                "None",
			PublishNotReadyAddresses: true,
			Selector: map[string]string{
				"app": "etcd",
			},
			Ports: []corev1.ServicePort{
				{Name: "etcd-client", Port: 2379},
				{Name: "etcd-server", Port: 2380},
				{Name: "etcd-metrics", Port: 8080},
			},
		},
	}
}

func (r *ReconcileTenantCluster) buildEtcdStatefulSet(tc *appsv1alpha1.TenantCluster) *appsv1.StatefulSet {
	replicas := tc.GetEtcdReplicas()
	etcdImage := fmt.Sprintf("%s/etcd:%s", tc.GetEtcdImageRepository(), tc.GetEtcdVersion())

	// Build initial cluster string
	initialCluster := ""
	for i := int32(0); i < replicas; i++ {
		if i > 0 {
			initialCluster += ","
		}
		initialCluster += fmt.Sprintf("etcd-%d=$(URI_SCHEME)://etcd-%d.etcd:2380", i, i)
	}

	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "etcd",
			Namespace: tc.GetNamespace(),
			Labels: map[string]string{
				"app":                          "etcd",
				"tenantcluster.openyurt.io":    tc.Name,
				"app.kubernetes.io/managed-by": "tenantcluster-controller",
			},
		},
		Spec: appsv1.StatefulSetSpec{
			ServiceName:         "etcd",
			Replicas:            &replicas,
			PodManagementPolicy: appsv1.ParallelPodManagement,
			UpdateStrategy: appsv1.StatefulSetUpdateStrategy{
				Type: appsv1.RollingUpdateStatefulSetStrategyType,
			},
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "etcd",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":                       "etcd",
						"tenantcluster.openyurt.io": tc.Name,
					},
					Annotations: map[string]string{
						"serviceName": "etcd",
					},
				},
				Spec: corev1.PodSpec{
					HostNetwork: true,
					DNSPolicy:   corev1.DNSClusterFirstWithHostNet,
					Affinity: &corev1.Affinity{
						PodAntiAffinity: &corev1.PodAntiAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
								{
									LabelSelector: &metav1.LabelSelector{
										MatchExpressions: []metav1.LabelSelectorRequirement{
											{
												Key:      "app",
												Operator: metav1.LabelSelectorOpIn,
												Values:   []string{"etcd"},
											},
										},
									},
									TopologyKey: "kubernetes.io/hostname",
								},
							},
						},
					},
					Containers: []corev1.Container{
						{
							Name:            "etcd",
							Image:           etcdImage,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Ports: []corev1.ContainerPort{
								{Name: "etcd-client", ContainerPort: 2379},
								{Name: "etcd-server", ContainerPort: 2380},
								{Name: "etcd-metrics", ContainerPort: 8080},
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/readyz",
										Port: intstr.FromInt(8080),
									},
								},
								InitialDelaySeconds: 10,
								PeriodSeconds:       5,
								TimeoutSeconds:      5,
								SuccessThreshold:    1,
								FailureThreshold:    30,
							},
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/livez",
										Port: intstr.FromInt(8080),
									},
								},
								InitialDelaySeconds: 15,
								PeriodSeconds:       10,
								TimeoutSeconds:      5,
								FailureThreshold:    3,
							},
							Env: []corev1.EnvVar{
								{Name: "K8S_NAMESPACE", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.namespace"}}},
								{Name: "HOSTNAME", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.name"}}},
								{Name: "SERVICE_NAME", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.annotations['serviceName']"}}},
								{Name: "ETCDCTL_ENDPOINTS", Value: "$(HOSTNAME).$(SERVICE_NAME):2379"},
								{Name: "ETCDCTL_KEY", Value: "/etc/etcd/certs/client/tls.key"},
								{Name: "ETCDCTL_CERT", Value: "/etc/etcd/certs/client/tls.crt"},
								{Name: "ETCDCTL_CACERT", Value: "/etc/etcd/certs/client/ca.crt"},
								{Name: "URI_SCHEME", Value: "https"},
							},
							Command: []string{"/usr/local/bin/etcd"},
							Args: []string{
								"--name=$(HOSTNAME)",
								"--data-dir=/data",
								"--wal-dir=/data/wal",
								"--listen-peer-urls=$(URI_SCHEME)://0.0.0.0:2380",
								"--listen-client-urls=$(URI_SCHEME)://0.0.0.0:2379",
								"--advertise-client-urls=$(URI_SCHEME)://$(HOSTNAME).$(SERVICE_NAME):2379",
								"--initial-cluster-state=new",
								"--initial-cluster-token=etcd-$(K8S_NAMESPACE)",
								"--initial-cluster=" + initialCluster,
								"--initial-advertise-peer-urls=$(URI_SCHEME)://$(HOSTNAME).$(SERVICE_NAME):2380",
								"--listen-metrics-urls=http://0.0.0.0:8080",
								"--client-cert-auth",
								"--trusted-ca-file=$(ETCDCTL_CACERT)",
								"--cert-file=$(ETCDCTL_CERT)",
								"--key-file=$(ETCDCTL_KEY)",
								"--peer-client-cert-auth",
								"--peer-trusted-ca-file=/etc/etcd/certs/server/ca.crt",
								"--peer-cert-file=/etc/etcd/certs/server/tls.crt",
								"--peer-key-file=/etc/etcd/certs/server/tls.key",
							},
							VolumeMounts: []corev1.VolumeMount{
								{Name: "etcd-data", MountPath: "/data"},
								{Name: "etcd-client-tls", MountPath: "/etc/etcd/certs/client", ReadOnly: true},
								{Name: "etcd-server-tls", MountPath: "/etc/etcd/certs/server", ReadOnly: true},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "etcd-client-tls",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: "tenant-apiserver-etcd-client-certs",
								},
							},
						},
						{
							Name: "etcd-server-tls",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: "tenant-etcd-server-tls",
								},
							},
						},
					},
				},
			},
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "etcd-data",
					},
					Spec: corev1.PersistentVolumeClaimSpec{
						AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
						StorageClassName: stringPtr(tc.GetEtcdStorageClass()),
						Resources: corev1.VolumeResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: resource.MustParse(tc.GetEtcdStorageSize()),
							},
						},
					},
				},
			},
		},
	}
}

// createAPIServerResources creates the APIServer Service and DaemonSet
func (r *ReconcileTenantCluster) createAPIServerResources(ctx context.Context, tc *appsv1alpha1.TenantCluster) error {
	// Create APIServer service
	apiServerService := r.buildAPIServerService(tc)
	if err := controllerutil.SetControllerReference(tc, apiServerService, r.Scheme()); err != nil {
		return fmt.Errorf("failed to set controller reference on apiserver service: %w", err)
	}
	if err := r.Create(ctx, apiServerService); err != nil {
		return fmt.Errorf("failed to create apiserver service: %w", err)
	}
	klog.Info(Format("Created apiserver service for TenantCluster %s", tc.Name))

	// Create APIServer DaemonSet
	apiServerDs := r.buildAPIServerDaemonSet(tc)
	if err := controllerutil.SetControllerReference(tc, apiServerDs, r.Scheme()); err != nil {
		return fmt.Errorf("failed to set controller reference on apiserver daemonset: %w", err)
	}
	if err := r.Create(ctx, apiServerDs); err != nil {
		return fmt.Errorf("failed to create apiserver daemonset: %w", err)
	}
	klog.Info(Format("Created apiserver daemonset for TenantCluster %s", tc.Name))

	r.recorder.Event(tc, corev1.EventTypeNormal, "APIServerCreated", "Created APIServer resources")
	return nil
}

func (r *ReconcileTenantCluster) buildAPIServerService(tc *appsv1alpha1.TenantCluster) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tenant-apiserver",
			Namespace: tc.GetNamespace(),
			Labels: map[string]string{
				"component":                    "tenant-apiserver",
				"tenantcluster.openyurt.io":    tc.Name,
				"app.kubernetes.io/managed-by": "tenantcluster-controller",
			},
		},
		Spec: corev1.ServiceSpec{
			Type:      corev1.ServiceTypeNodePort,
			ClusterIP: tc.Spec.APIServerServiceIP,
			Selector: map[string]string{
				"component": "tenant-apiserver",
			},
			Ports: []corev1.ServicePort{
				{
					Name:       "https",
					Port:       6443,
					Protocol:   corev1.ProtocolTCP,
					TargetPort: intstr.FromInt(6443),
					NodePort:   tc.Spec.APIServerNodePort,
				},
			},
		},
	}
}

func (r *ReconcileTenantCluster) buildAPIServerDaemonSet(tc *appsv1alpha1.TenantCluster) *appsv1.DaemonSet {
	apiServerImage := fmt.Sprintf("%s/kube-apiserver:%s", tc.GetImageRepository(), tc.Spec.KubernetesVersion)

	// Build etcd servers string
	etcdServers := ""
	for i := int32(0); i < tc.GetEtcdReplicas(); i++ {
		if i > 0 {
			etcdServers += ","
		}
		etcdServers += fmt.Sprintf("https://etcd-%d.etcd:2379", i)
	}

	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tenant-apiserver",
			Namespace: tc.GetNamespace(),
			Labels: map[string]string{
				"tier":                         "control-plane",
				"component":                    "tenant-apiserver",
				"tenantcluster.openyurt.io":    tc.Name,
				"app.kubernetes.io/managed-by": "tenantcluster-controller",
			},
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"tier":      "control-plane",
					"component": "tenant-apiserver",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"tier":                      "control-plane",
						"component":                 "tenant-apiserver",
						"tenantcluster.openyurt.io": tc.Name,
					},
				},
				Spec: corev1.PodSpec{
					HostNetwork:                   true,
					DNSPolicy:                     corev1.DNSClusterFirstWithHostNet,
					PriorityClassName:             "system-cluster-critical",
					TerminationGracePeriodSeconds: int64Ptr(30),
					SecurityContext: &corev1.PodSecurityContext{
						SeccompProfile: &corev1.SeccompProfile{
							Type: corev1.SeccompProfileTypeRuntimeDefault,
						},
					},
					Containers: []corev1.Container{
						{
							Name:            "tenant-apiserver",
							Image:           apiServerImage,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Command:         []string{"kube-apiserver"},
							Args: []string{
								"--advertise-address=$(NODE_IP)",
								"--allow-privileged=true",
								"--authorization-mode=Node,RBAC",
								"--client-ca-file=/etc/kubernetes/pki/ca.crt",
								"--enable-admission-plugins=NodeRestriction",
								"--enable-bootstrap-token-auth=true",
								"--etcd-cafile=/etc/kubernetes/pki/etcd/ca.crt",
								"--etcd-certfile=/etc/kubernetes/pki/etcd/apiserver-etcd-client.crt",
								"--etcd-keyfile=/etc/kubernetes/pki/etcd/apiserver-etcd-client.key",
								"--etcd-servers=" + etcdServers,
								"--kubelet-client-certificate=/etc/kubernetes/pki/apiserver-kubelet-client.crt",
								"--kubelet-client-key=/etc/kubernetes/pki/apiserver-kubelet-client.key",
								"--kubelet-preferred-address-types=InternalIP,ExternalIP,Hostname",
								"--proxy-client-cert-file=/etc/kubernetes/pki/front-proxy-client.crt",
								"--proxy-client-key-file=/etc/kubernetes/pki/front-proxy-client.key",
								"--requestheader-allowed-names=front-proxy-client",
								"--requestheader-client-ca-file=/etc/kubernetes/pki/front-proxy-ca.crt",
								"--requestheader-extra-headers-prefix=X-Remote-Extra-",
								"--requestheader-group-headers=X-Remote-Group",
								"--requestheader-username-headers=X-Remote-User",
								"--secure-port=6443",
								"--service-account-issuer=https://kubernetes.default.svc.cluster.local",
								"--service-account-key-file=/etc/kubernetes/pki/sa.pub",
								"--service-account-signing-key-file=/etc/kubernetes/pki/sa.key",
								"--service-cluster-ip-range=" + tc.GetServiceClusterIPRange(),
								"--tls-cert-file=/etc/kubernetes/pki/apiserver.crt",
								"--tls-private-key-file=/etc/kubernetes/pki/apiserver.key",
							},
							Env: []corev1.EnvVar{
								{Name: "NODE_IP", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "status.hostIP"}}},
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU: resource.MustParse("250m"),
								},
							},
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path:   "/livez",
										Port:   intstr.FromInt(6443),
										Scheme: corev1.URISchemeHTTPS,
										Host:   "127.0.0.1",
									},
								},
								InitialDelaySeconds: 10,
								PeriodSeconds:       10,
								TimeoutSeconds:      15,
								FailureThreshold:    8,
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path:   "/readyz",
										Port:   intstr.FromInt(6443),
										Scheme: corev1.URISchemeHTTPS,
										Host:   "127.0.0.1",
									},
								},
								PeriodSeconds:    1,
								TimeoutSeconds:   15,
								FailureThreshold: 3,
							},
							StartupProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path:   "/livez",
										Port:   intstr.FromInt(6443),
										Scheme: corev1.URISchemeHTTPS,
										Host:   "127.0.0.1",
									},
								},
								InitialDelaySeconds: 10,
								PeriodSeconds:       10,
								TimeoutSeconds:      15,
								FailureThreshold:    24,
							},
							VolumeMounts: []corev1.VolumeMount{
								{Name: "ca-vol", MountPath: "/etc/kubernetes/pki/ca.crt", SubPath: "ca.crt"},
								{Name: "apiserver-certs-vol", MountPath: "/etc/kubernetes/pki/apiserver.crt", SubPath: "tls.crt"},
								{Name: "apiserver-certs-vol", MountPath: "/etc/kubernetes/pki/apiserver.key", SubPath: "tls.key"},
								{Name: "apiserver-kubelet-client-certs-vol", MountPath: "/etc/kubernetes/pki/apiserver-kubelet-client.crt", SubPath: "tls.crt"},
								{Name: "apiserver-kubelet-client-certs-vol", MountPath: "/etc/kubernetes/pki/apiserver-kubelet-client.key", SubPath: "tls.key"},
								{Name: "sa-keys-vol", MountPath: "/etc/kubernetes/pki/sa.pub", SubPath: "sa.pub"},
								{Name: "sa-keys-vol", MountPath: "/etc/kubernetes/pki/sa.key", SubPath: "sa.key"},
								{Name: "front-proxy-ca-vol", MountPath: "/etc/kubernetes/pki/front-proxy-ca.crt", SubPath: "front-proxy-ca.crt"},
								{Name: "front-proxy-client-certs-vol", MountPath: "/etc/kubernetes/pki/front-proxy-client.crt", SubPath: "tls.crt"},
								{Name: "front-proxy-client-certs-vol", MountPath: "/etc/kubernetes/pki/front-proxy-client.key", SubPath: "tls.key"},
								{Name: "apiserver-etcd-client-certs-vol", MountPath: "/etc/kubernetes/pki/etcd/ca.crt", SubPath: "ca.crt"},
								{Name: "apiserver-etcd-client-certs-vol", MountPath: "/etc/kubernetes/pki/etcd/apiserver-etcd-client.crt", SubPath: "tls.crt"},
								{Name: "apiserver-etcd-client-certs-vol", MountPath: "/etc/kubernetes/pki/etcd/apiserver-etcd-client.key", SubPath: "tls.key"},
							},
						},
					},
					Volumes: []corev1.Volume{
						{Name: "ca-vol", VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: "host-k8s-ca-key-pair"}}},
						{Name: "apiserver-certs-vol", VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: "tenant-apiserver-certs"}}},
						{Name: "apiserver-kubelet-client-certs-vol", VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: "tenant-apiserver-kubelet-client-certs"}}},
						{Name: "sa-keys-vol", VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: "tenant-sa-keys"}}},
						{Name: "front-proxy-ca-vol", VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: "tenant-front-proxy-ca"}}},
						{Name: "front-proxy-client-certs-vol", VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: "tenant-front-proxy-client-certs"}}},
						{Name: "apiserver-etcd-client-certs-vol", VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: "tenant-apiserver-etcd-client-certs"}}},
					},
				},
			},
		},
	}
}

// createSchedulerResources creates the Scheduler Deployment
func (r *ReconcileTenantCluster) createSchedulerResources(ctx context.Context, tc *appsv1alpha1.TenantCluster) error {
	schedulerDeploy := r.buildSchedulerDeployment(tc)
	if err := controllerutil.SetControllerReference(tc, schedulerDeploy, r.Scheme()); err != nil {
		return fmt.Errorf("failed to set controller reference on scheduler deployment: %w", err)
	}
	if err := r.Create(ctx, schedulerDeploy); err != nil {
		return fmt.Errorf("failed to create scheduler deployment: %w", err)
	}
	klog.Info(Format("Created scheduler deployment for TenantCluster %s", tc.Name))

	r.recorder.Event(tc, corev1.EventTypeNormal, "SchedulerCreated", "Created Scheduler resources")
	return nil
}

func (r *ReconcileTenantCluster) buildSchedulerDeployment(tc *appsv1alpha1.TenantCluster) *appsv1.Deployment {
	replicas := tc.GetSchedulerReplicas()
	schedulerImage := fmt.Sprintf("%s/kube-scheduler:%s", tc.GetImageRepository(), tc.Spec.KubernetesVersion)

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tenant-scheduler",
			Namespace: tc.GetNamespace(),
			Labels: map[string]string{
				"component":                    "tenant-scheduler",
				"tier":                         "control-plane",
				"tenantcluster.openyurt.io":    tc.Name,
				"app.kubernetes.io/managed-by": "tenantcluster-controller",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RecreateDeploymentStrategyType,
			},
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"component": "tenant-scheduler",
					"tier":      "control-plane",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"component":                 "tenant-scheduler",
						"tier":                      "control-plane",
						"tenantcluster.openyurt.io": tc.Name,
					},
				},
				Spec: corev1.PodSpec{
					HostNetwork:       true,
					DNSPolicy:         corev1.DNSClusterFirstWithHostNet,
					PriorityClassName: "system-cluster-critical",
					SecurityContext: &corev1.PodSecurityContext{
						SeccompProfile: &corev1.SeccompProfile{
							Type: corev1.SeccompProfileTypeRuntimeDefault,
						},
					},
					Containers: []corev1.Container{
						{
							Name:            "tenant-scheduler",
							Image:           schedulerImage,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Command:         []string{"kube-scheduler"},
							Args: []string{
								"--authentication-kubeconfig=/etc/kubernetes/scheduler.conf",
								"--authorization-kubeconfig=/etc/kubernetes/scheduler.conf",
								"--bind-address=127.0.0.1",
								"--kubeconfig=/etc/kubernetes/scheduler.conf",
								"--leader-elect=true",
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU: resource.MustParse("100m"),
								},
							},
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path:   "/livez",
										Port:   intstr.FromInt(10259),
										Scheme: corev1.URISchemeHTTPS,
										Host:   "127.0.0.1",
									},
								},
								InitialDelaySeconds: 10,
								PeriodSeconds:       10,
								TimeoutSeconds:      15,
								FailureThreshold:    8,
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path:   "/readyz",
										Port:   intstr.FromInt(10259),
										Scheme: corev1.URISchemeHTTPS,
										Host:   "127.0.0.1",
									},
								},
								PeriodSeconds:    1,
								TimeoutSeconds:   15,
								FailureThreshold: 3,
							},
							StartupProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path:   "/livez",
										Port:   intstr.FromInt(10259),
										Scheme: corev1.URISchemeHTTPS,
										Host:   "127.0.0.1",
									},
								},
								InitialDelaySeconds: 10,
								PeriodSeconds:       10,
								TimeoutSeconds:      15,
								FailureThreshold:    24,
							},
							VolumeMounts: []corev1.VolumeMount{
								{Name: "kubeconfig-volume", MountPath: "/etc/kubernetes/scheduler.conf", SubPath: "kubeconfig", ReadOnly: true},
							},
						},
					},
					Volumes: []corev1.Volume{
						{Name: "kubeconfig-volume", VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: "tenant-scheduler-kubeconfig"}}},
					},
				},
			},
		},
	}
}

// createControllerManagerResources creates the Controller Manager Deployment
func (r *ReconcileTenantCluster) createControllerManagerResources(ctx context.Context, tc *appsv1alpha1.TenantCluster) error {
	kcmDeploy := r.buildControllerManagerDeployment(tc)
	if err := controllerutil.SetControllerReference(tc, kcmDeploy, r.Scheme()); err != nil {
		return fmt.Errorf("failed to set controller reference on controller-manager deployment: %w", err)
	}
	if err := r.Create(ctx, kcmDeploy); err != nil {
		return fmt.Errorf("failed to create controller-manager deployment: %w", err)
	}
	klog.Info(Format("Created controller-manager deployment for TenantCluster %s", tc.Name))

	r.recorder.Event(tc, corev1.EventTypeNormal, "ControllerManagerCreated", "Created ControllerManager resources")
	return nil
}

func (r *ReconcileTenantCluster) buildControllerManagerDeployment(tc *appsv1alpha1.TenantCluster) *appsv1.Deployment {
	replicas := tc.GetControllerManagerReplicas()
	kcmImage := fmt.Sprintf("%s/kube-controller-manager:%s", tc.GetImageRepository(), tc.Spec.KubernetesVersion)

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tenant-controller-manager",
			Namespace: tc.GetNamespace(),
			Labels: map[string]string{
				"component":                    "tenant-controller-manager",
				"tier":                         "control-plane",
				"tenantcluster.openyurt.io":    tc.Name,
				"app.kubernetes.io/managed-by": "tenantcluster-controller",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"component": "tenant-controller-manager",
					"tier":      "control-plane",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"component":                 "tenant-controller-manager",
						"tier":                      "control-plane",
						"tenantcluster.openyurt.io": tc.Name,
					},
				},
				Spec: corev1.PodSpec{
					HostNetwork:       true,
					DNSPolicy:         corev1.DNSClusterFirstWithHostNet,
					PriorityClassName: "system-cluster-critical",
					SecurityContext: &corev1.PodSecurityContext{
						SeccompProfile: &corev1.SeccompProfile{
							Type: corev1.SeccompProfileTypeRuntimeDefault,
						},
					},
					Containers: []corev1.Container{
						{
							Name:            "tenant-controller-manager",
							Image:           kcmImage,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Command:         []string{"kube-controller-manager"},
							Args: []string{
								"--allocate-node-cidrs=true",
								"--authentication-kubeconfig=/etc/kubernetes/config/kubeconfig",
								"--authorization-kubeconfig=/etc/kubernetes/config/kubeconfig",
								"--bind-address=127.0.0.1",
								"--client-ca-file=/etc/kubernetes/pki/ca.crt",
								"--cluster-cidr=" + tc.GetPodCIDR(),
								"--cluster-name=kubernetes",
								"--cluster-signing-cert-file=/etc/kubernetes/pki/ca.crt",
								"--cluster-signing-key-file=/etc/kubernetes/pki/ca.key",
								"--controllers=*,bootstrapsigner,tokencleaner",
								"--kubeconfig=/etc/kubernetes/config/kubeconfig",
								"--leader-elect=true",
								"--requestheader-client-ca-file=/etc/kubernetes/pki/front-proxy-ca.crt",
								"--root-ca-file=/etc/kubernetes/pki/ca.crt",
								"--service-account-private-key-file=/etc/kubernetes/pki/sa.key",
								"--service-cluster-ip-range=" + tc.GetServiceClusterIPRange(),
								"--use-service-account-credentials=true",
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU: resource.MustParse("200m"),
								},
							},
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path:   "/healthz",
										Port:   intstr.FromInt(10257),
										Scheme: corev1.URISchemeHTTPS,
										Host:   "127.0.0.1",
									},
								},
								InitialDelaySeconds: 10,
								PeriodSeconds:       10,
								TimeoutSeconds:      15,
								FailureThreshold:    8,
							},
							StartupProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path:   "/healthz",
										Port:   intstr.FromInt(10257),
										Scheme: corev1.URISchemeHTTPS,
										Host:   "127.0.0.1",
									},
								},
								InitialDelaySeconds: 10,
								PeriodSeconds:       10,
								TimeoutSeconds:      15,
								FailureThreshold:    24,
							},
							VolumeMounts: []corev1.VolumeMount{
								{Name: "kubeconfig-vol", MountPath: "/etc/kubernetes/config", ReadOnly: true},
								{Name: "ca-vol", MountPath: "/etc/kubernetes/pki/ca.crt", SubPath: "ca.crt", ReadOnly: true},
								{Name: "ca-vol", MountPath: "/etc/kubernetes/pki/ca.key", SubPath: "ca.key", ReadOnly: true},
								{Name: "sa-keys-vol", MountPath: "/etc/kubernetes/pki/sa.key", SubPath: "sa.key", ReadOnly: true},
								{Name: "front-proxy-ca-vol", MountPath: "/etc/kubernetes/pki/front-proxy-ca.crt", SubPath: "front-proxy-ca.crt", ReadOnly: true},
							},
						},
					},
					Volumes: []corev1.Volume{
						{Name: "kubeconfig-vol", VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: "tenant-kcm-kubeconfig"}}},
						{Name: "ca-vol", VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: "host-k8s-ca-key-pair"}}},
						{Name: "sa-keys-vol", VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: "tenant-sa-keys"}}},
						{Name: "front-proxy-ca-vol", VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: "tenant-front-proxy-ca"}}},
					},
				},
			},
		},
	}
}

// Helper functions
func stringPtr(s string) *string {
	return &s
}

func int64Ptr(i int64) *int64 {
	return &i
}
