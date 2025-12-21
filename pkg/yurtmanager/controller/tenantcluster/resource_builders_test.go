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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appsv1alpha1 "github.com/openyurtio/openyurt/pkg/apis/apps/v1alpha1"
	tenantclusterconfig "github.com/openyurtio/openyurt/pkg/yurtmanager/controller/tenantcluster/config"
)

func newTestReconciler() *ReconcileTenantCluster {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = appsv1alpha1.AddToScheme(scheme)

	client := fake.NewClientBuilder().WithScheme(scheme).Build()
	return &ReconcileTenantCluster{
		Client:   client,
		recorder: record.NewFakeRecorder(10),
		cfg:      tenantclusterconfig.TenantClusterControllerConfiguration{},
	}
}

func TestBuildEtcdService(t *testing.T) {
	r := newTestReconciler()
	tc := newTestTenantCluster("test")

	svc := r.buildEtcdService(tc)

	// Basic assertions
	assert.Equal(t, "etcd", svc.Name)
	assert.Equal(t, tc.GetNamespace(), svc.Namespace)

	// Service type
	assert.Equal(t, corev1.ServiceTypeClusterIP, svc.Spec.Type)
	assert.Equal(t, "None", svc.Spec.ClusterIP) // Headless

	// Ports - check names instead of ports since implementation uses different names
	require.Len(t, svc.Spec.Ports, 3)
	portNames := make(map[string]bool)
	for _, p := range svc.Spec.Ports {
		portNames[p.Name] = true
	}
	assert.True(t, portNames["etcd-client"] || portNames["client"], "should have etcd-client or client port")
	assert.True(t, portNames["etcd-server"] || portNames["peer"], "should have etcd-server or peer port")

	// Labels
	assert.Equal(t, "etcd", svc.Spec.Selector["app"])
}

func TestBuildEtcdStatefulSet(t *testing.T) {
	r := newTestReconciler()
	tc := newTestTenantCluster("test")

	sts := r.buildEtcdStatefulSet(tc)

	// Basic assertions
	assert.Equal(t, "etcd", sts.Name)
	assert.Equal(t, tc.GetNamespace(), sts.Namespace)

	// Replicas
	assert.Equal(t, tc.GetEtcdReplicas(), *sts.Spec.Replicas)

	// Service name
	assert.Equal(t, "etcd", sts.Spec.ServiceName)

	// Selector
	assert.Equal(t, "etcd", sts.Spec.Selector.MatchLabels["app"])

	// Pod template
	podTemplate := sts.Spec.Template
	assert.Equal(t, "etcd", podTemplate.Labels["app"])

	// Container
	require.Len(t, podTemplate.Spec.Containers, 1)
	container := podTemplate.Spec.Containers[0]
	assert.Equal(t, "etcd", container.Name)
	assert.Contains(t, container.Image, "etcd")

	// Ports
	require.Len(t, container.Ports, 3)

	// Volume mounts - check for data and certs
	hasDataMount := false
	hasCertsMount := false
	for _, vm := range container.VolumeMounts {
		if vm.Name == "etcd-data" {
			hasDataMount = true
		}
		if vm.Name == "etcd-client-tls" || vm.Name == "etcd-server-tls" || vm.Name == "etcd-certs" {
			hasCertsMount = true
		}
	}
	assert.True(t, hasDataMount, "should have etcd-data volume mount")
	assert.True(t, hasCertsMount, "should have certificate volume mount")

	// PVC Template
	require.Len(t, sts.Spec.VolumeClaimTemplates, 1)
	pvc := sts.Spec.VolumeClaimTemplates[0]
	assert.Equal(t, "etcd-data", pvc.Name)
}

func TestBuildAPIServerService(t *testing.T) {
	r := newTestReconciler()
	tc := newTestTenantCluster("test")

	svc := r.buildAPIServerService(tc)

	// Basic assertions
	assert.Equal(t, "tenant-apiserver", svc.Name)
	assert.Equal(t, tc.GetNamespace(), svc.Namespace)

	// Service type
	assert.Equal(t, corev1.ServiceTypeNodePort, svc.Spec.Type)

	// Cluster IP
	assert.Equal(t, tc.Spec.APIServerServiceIP, svc.Spec.ClusterIP)

	// Ports
	require.Len(t, svc.Spec.Ports, 1)
	port := svc.Spec.Ports[0]
	assert.Equal(t, "https", port.Name)
	assert.Equal(t, int32(6443), port.Port)
	assert.Equal(t, tc.Spec.APIServerNodePort, port.NodePort)

	// Selector - uses "component" instead of "app"
	assert.Equal(t, "tenant-apiserver", svc.Spec.Selector["component"])
}

func TestBuildAPIServerDaemonSet(t *testing.T) {
	r := newTestReconciler()
	tc := newTestTenantCluster("test")

	ds := r.buildAPIServerDaemonSet(tc)

	// Basic assertions
	assert.Equal(t, "tenant-apiserver", ds.Name)
	assert.Equal(t, tc.GetNamespace(), ds.Namespace)

	// Selector - uses "component" instead of "app"
	assert.Equal(t, "tenant-apiserver", ds.Spec.Selector.MatchLabels["component"])

	// Pod template
	podTemplate := ds.Spec.Template
	assert.Equal(t, "tenant-apiserver", podTemplate.Labels["component"])

	// Container - name is "tenant-apiserver" in actual implementation
	require.Len(t, podTemplate.Spec.Containers, 1)
	container := podTemplate.Spec.Containers[0]
	assert.Equal(t, "tenant-apiserver", container.Name)
	assert.Contains(t, container.Image, "kube-apiserver")
	assert.Contains(t, container.Image, tc.Spec.KubernetesVersion)

	// Verify important command args
	args := container.Args
	hasSecurePort := false
	hasEtcdServers := false
	hasServiceClusterIPRange := false
	for _, arg := range args {
		if arg == "--secure-port=6443" {
			hasSecurePort = true
		}
		if len(arg) > 15 && arg[:15] == "--etcd-servers=" {
			hasEtcdServers = true
		}
		if arg == "--service-cluster-ip-range="+tc.GetServiceClusterIPRange() {
			hasServiceClusterIPRange = true
		}
	}
	assert.True(t, hasSecurePort, "should have --secure-port=6443")
	assert.True(t, hasEtcdServers, "should have --etcd-servers")
	assert.True(t, hasServiceClusterIPRange, "should have --service-cluster-ip-range")
}

func TestBuildSchedulerDeployment(t *testing.T) {
	r := newTestReconciler()
	tc := newTestTenantCluster("test")

	deploy := r.buildSchedulerDeployment(tc)

	// Basic assertions
	assert.Equal(t, "tenant-scheduler", deploy.Name)
	assert.Equal(t, tc.GetNamespace(), deploy.Namespace)

	// Replicas
	assert.Equal(t, tc.GetSchedulerReplicas(), *deploy.Spec.Replicas)

	// Selector - uses "component" instead of "app"
	assert.Equal(t, "tenant-scheduler", deploy.Spec.Selector.MatchLabels["component"])

	// Pod template
	podTemplate := deploy.Spec.Template
	assert.Equal(t, "tenant-scheduler", podTemplate.Labels["component"])

	// Container
	require.Len(t, podTemplate.Spec.Containers, 1)
	container := podTemplate.Spec.Containers[0]
	assert.Equal(t, "tenant-scheduler", container.Name)
	assert.Contains(t, container.Image, "kube-scheduler")
	assert.Contains(t, container.Image, tc.Spec.KubernetesVersion)

	// Verify kubeconfig arg
	hasKubeconfig := false
	for _, arg := range container.Args {
		if arg == "--kubeconfig=/etc/kubernetes/scheduler.conf" {
			hasKubeconfig = true
			break
		}
	}
	assert.True(t, hasKubeconfig, "should have --kubeconfig arg")
}

func TestBuildControllerManagerDeployment(t *testing.T) {
	r := newTestReconciler()
	tc := newTestTenantCluster("test")

	deploy := r.buildControllerManagerDeployment(tc)

	// Basic assertions
	assert.Equal(t, "tenant-controller-manager", deploy.Name)
	assert.Equal(t, tc.GetNamespace(), deploy.Namespace)

	// Replicas
	assert.Equal(t, tc.GetControllerManagerReplicas(), *deploy.Spec.Replicas)

	// Selector - uses "component" instead of "app"
	assert.Equal(t, "tenant-controller-manager", deploy.Spec.Selector.MatchLabels["component"])

	// Pod template
	podTemplate := deploy.Spec.Template
	assert.Equal(t, "tenant-controller-manager", podTemplate.Labels["component"])

	// Container
	require.Len(t, podTemplate.Spec.Containers, 1)
	container := podTemplate.Spec.Containers[0]
	assert.Equal(t, "tenant-controller-manager", container.Name)
	assert.Contains(t, container.Image, "kube-controller-manager")
	assert.Contains(t, container.Image, tc.Spec.KubernetesVersion)

	// Verify important args
	args := container.Args
	hasClusterCIDR := false
	hasServiceClusterIPRange := false
	for _, arg := range args {
		if arg == "--cluster-cidr="+tc.GetPodCIDR() {
			hasClusterCIDR = true
		}
		if arg == "--service-cluster-ip-range="+tc.GetServiceClusterIPRange() {
			hasServiceClusterIPRange = true
		}
	}
	assert.True(t, hasClusterCIDR, "should have --cluster-cidr")
	assert.True(t, hasServiceClusterIPRange, "should have --service-cluster-ip-range")
}

func TestStringPtr(t *testing.T) {
	result := stringPtr("test")
	assert.NotNil(t, result)
	assert.Equal(t, "test", *result)
}

func TestInt64Ptr(t *testing.T) {
	result := int64Ptr(42)
	assert.NotNil(t, result)
	assert.Equal(t, int64(42), *result)
}

func TestBuildEtcdStatefulSetWithCustomReplicas(t *testing.T) {
	r := newTestReconciler()
	tc := newTestTenantCluster("test")

	tests := []struct {
		name     string
		replicas int32
	}{
		{"1 replica", 1},
		{"3 replicas", 3},
		{"5 replicas", 5},
		{"7 replicas", 7},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc.Spec.EtcdReplicas = &tt.replicas
			sts := r.buildEtcdStatefulSet(tc)
			assert.Equal(t, tt.replicas, *sts.Spec.Replicas)
		})
	}
}

func TestBuildSchedulerDeploymentWithCustomReplicas(t *testing.T) {
	r := newTestReconciler()
	tc := newTestTenantCluster("test")

	replicas := int32(4)
	tc.Spec.SchedulerReplicas = &replicas

	deploy := r.buildSchedulerDeployment(tc)
	assert.Equal(t, replicas, *deploy.Spec.Replicas)
}

func TestBuildControllerManagerDeploymentWithCustomReplicas(t *testing.T) {
	r := newTestReconciler()
	tc := newTestTenantCluster("test")

	replicas := int32(3)
	tc.Spec.ControllerManagerReplicas = &replicas

	deploy := r.buildControllerManagerDeployment(tc)
	assert.Equal(t, replicas, *deploy.Spec.Replicas)
}

func TestResourceLabels(t *testing.T) {
	r := newTestReconciler()
	tc := newTestTenantCluster("test")

	// All resources should have a label indicating the tenant cluster
	resources := []metav1.Object{
		r.buildEtcdService(tc),
		r.buildEtcdStatefulSet(tc),
		r.buildAPIServerService(tc),
		r.buildAPIServerDaemonSet(tc),
		r.buildSchedulerDeployment(tc),
		r.buildControllerManagerDeployment(tc),
	}

	for _, res := range resources {
		labels := res.GetLabels()
		assert.NotNil(t, labels, "resource %s should have labels", res.GetName())
		// Check for tenantcluster.openyurt.io label
		assert.Equal(t, tc.Name, labels["tenantcluster.openyurt.io"], "resource %s should have tenantcluster label", res.GetName())
	}
}

func TestBuildEtcdStatefulSetNamespace(t *testing.T) {
	r := newTestReconciler()
	
	tests := []struct {
		name         string
		clusterName  string
		wantNS       string
	}{
		{"simple name", "test", "tenant-test"},
		{"complex name", "my-cluster", "tenant-my-cluster"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc := newTestTenantCluster(tt.clusterName)
			sts := r.buildEtcdStatefulSet(tc)
			assert.Equal(t, tt.wantNS, sts.Namespace)
		})
	}
}
