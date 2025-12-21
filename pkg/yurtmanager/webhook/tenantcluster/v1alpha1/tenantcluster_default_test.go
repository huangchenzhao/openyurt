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

package v1alpha1

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appsv1alpha1 "github.com/openyurtio/openyurt/pkg/apis/apps/v1alpha1"
)

func TestDefault_SetsEtcdReplicas(t *testing.T) {
	tc := newDefaultTestTenantCluster()
	handler := newDefaultTestHandler()

	err := handler.Default(context.Background(), tc)
	require.NoError(t, err)

	assert.NotNil(t, tc.Spec.EtcdReplicas)
	assert.Equal(t, int32(3), *tc.Spec.EtcdReplicas)
}

func TestDefault_SetsControllerManagerReplicas(t *testing.T) {
	tc := newDefaultTestTenantCluster()
	handler := newDefaultTestHandler()

	err := handler.Default(context.Background(), tc)
	require.NoError(t, err)

	assert.NotNil(t, tc.Spec.ControllerManagerReplicas)
	assert.Equal(t, int32(2), *tc.Spec.ControllerManagerReplicas)
}

func TestDefault_SetsSchedulerReplicas(t *testing.T) {
	tc := newDefaultTestTenantCluster()
	handler := newDefaultTestHandler()

	err := handler.Default(context.Background(), tc)
	require.NoError(t, err)

	assert.NotNil(t, tc.Spec.SchedulerReplicas)
	assert.Equal(t, int32(2), *tc.Spec.SchedulerReplicas)
}

func TestDefault_SetsImageRepository(t *testing.T) {
	tc := newDefaultTestTenantCluster()
	handler := newDefaultTestHandler()

	err := handler.Default(context.Background(), tc)
	require.NoError(t, err)

	assert.Equal(t, "registry.aliyuncs.com/google_containers", tc.Spec.ImageRepository)
}

func TestDefault_SetsEtcdImageRepository(t *testing.T) {
	tc := newDefaultTestTenantCluster()
	handler := newDefaultTestHandler()

	err := handler.Default(context.Background(), tc)
	require.NoError(t, err)

	assert.Equal(t, "quay.io/coreos", tc.Spec.EtcdImageRepository)
}

func TestDefault_SetsEtcdVersion(t *testing.T) {
	tc := newDefaultTestTenantCluster()
	handler := newDefaultTestHandler()

	err := handler.Default(context.Background(), tc)
	require.NoError(t, err)

	assert.Equal(t, "v3.6.0", tc.Spec.EtcdVersion)
}

func TestDefault_SetsServiceClusterIPRange(t *testing.T) {
	tc := newDefaultTestTenantCluster()
	handler := newDefaultTestHandler()

	err := handler.Default(context.Background(), tc)
	require.NoError(t, err)

	assert.Equal(t, "10.96.0.0/12", tc.Spec.ServiceClusterIPRange)
}

func TestDefault_SetsPodCIDR(t *testing.T) {
	tc := newDefaultTestTenantCluster()
	handler := newDefaultTestHandler()

	err := handler.Default(context.Background(), tc)
	require.NoError(t, err)

	assert.Equal(t, "10.244.0.0/16", tc.Spec.PodCIDR)
}

func TestDefault_SetsEtcdStorageClass(t *testing.T) {
	tc := newDefaultTestTenantCluster()
	handler := newDefaultTestHandler()

	err := handler.Default(context.Background(), tc)
	require.NoError(t, err)

	assert.Equal(t, "local-path", tc.Spec.EtcdStorageClass)
}

func TestDefault_SetsEtcdStorageSize(t *testing.T) {
	tc := newDefaultTestTenantCluster()
	handler := newDefaultTestHandler()

	err := handler.Default(context.Background(), tc)
	require.NoError(t, err)

	assert.Equal(t, "1Gi", tc.Spec.EtcdStorageSize)
}

func TestDefault_SetsHostCASecretRefNamespace(t *testing.T) {
	tc := newDefaultTestTenantCluster()
	tc.Spec.HostCASecretRef.Namespace = "" // Clear the namespace
	handler := newDefaultTestHandler()

	err := handler.Default(context.Background(), tc)
	require.NoError(t, err)

	assert.Equal(t, "kube-system", tc.Spec.HostCASecretRef.Namespace)
}

func TestDefault_SetsStatusPhase(t *testing.T) {
	tc := newDefaultTestTenantCluster()
	handler := newDefaultTestHandler()

	err := handler.Default(context.Background(), tc)
	require.NoError(t, err)

	assert.Equal(t, appsv1alpha1.TenantClusterPhasePending, tc.Status.Phase)
}

func TestDefault_PreservesExistingEtcdReplicas(t *testing.T) {
	tc := newDefaultTestTenantCluster()
	customReplicas := int32(5)
	tc.Spec.EtcdReplicas = &customReplicas
	handler := newDefaultTestHandler()

	err := handler.Default(context.Background(), tc)
	require.NoError(t, err)

	assert.Equal(t, int32(5), *tc.Spec.EtcdReplicas)
}

func TestDefault_PreservesExistingImageRepository(t *testing.T) {
	tc := newDefaultTestTenantCluster()
	tc.Spec.ImageRepository = "my-custom-registry/images"
	handler := newDefaultTestHandler()

	err := handler.Default(context.Background(), tc)
	require.NoError(t, err)

	assert.Equal(t, "my-custom-registry/images", tc.Spec.ImageRepository)
}

func TestDefault_PreservesExistingPodCIDR(t *testing.T) {
	tc := newDefaultTestTenantCluster()
	tc.Spec.PodCIDR = "192.168.0.0/16"
	handler := newDefaultTestHandler()

	err := handler.Default(context.Background(), tc)
	require.NoError(t, err)

	assert.Equal(t, "192.168.0.0/16", tc.Spec.PodCIDR)
}

func TestDefault_PreservesExistingServiceClusterIPRange(t *testing.T) {
	tc := newDefaultTestTenantCluster()
	tc.Spec.ServiceClusterIPRange = "172.16.0.0/16"
	handler := newDefaultTestHandler()

	err := handler.Default(context.Background(), tc)
	require.NoError(t, err)

	assert.Equal(t, "172.16.0.0/16", tc.Spec.ServiceClusterIPRange)
}

func TestDefault_PreservesExistingEtcdStorageClass(t *testing.T) {
	tc := newDefaultTestTenantCluster()
	tc.Spec.EtcdStorageClass = "my-storage-class"
	handler := newDefaultTestHandler()

	err := handler.Default(context.Background(), tc)
	require.NoError(t, err)

	assert.Equal(t, "my-storage-class", tc.Spec.EtcdStorageClass)
}

func TestDefault_PreservesExistingHostCASecretRefNamespace(t *testing.T) {
	tc := newDefaultTestTenantCluster()
	tc.Spec.HostCASecretRef.Namespace = "custom-namespace"
	handler := newDefaultTestHandler()

	err := handler.Default(context.Background(), tc)
	require.NoError(t, err)

	assert.Equal(t, "custom-namespace", tc.Spec.HostCASecretRef.Namespace)
}

func TestDefault_ReturnsErrorForWrongType(t *testing.T) {
	handler := newDefaultTestHandler()
	pod := &corev1.Pod{}

	err := handler.Default(context.Background(), pod)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expected a TenantCluster")
}

func TestDefault_AllDefaultsApplied(t *testing.T) {
	tc := &appsv1alpha1.TenantCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
		Spec: appsv1alpha1.TenantClusterSpec{
			KubernetesVersion:    "v1.32.0",
			APIServerServiceIP:   "10.96.100.1",
			APIServerNodePort:    31000,
			HostControlPlaneAddr: "192.168.24.180:6443",
			HostCASecretRef: appsv1alpha1.SecretReference{
				Name: "host-k8s-ca",
				// Namespace intentionally left empty to test default
			},
		},
	}
	handler := newDefaultTestHandler()

	err := handler.Default(context.Background(), tc)
	require.NoError(t, err)

	// Verify ALL defaults are applied
	assert.Equal(t, int32(3), *tc.Spec.EtcdReplicas)
	assert.Equal(t, int32(2), *tc.Spec.ControllerManagerReplicas)
	assert.Equal(t, int32(2), *tc.Spec.SchedulerReplicas)
	assert.Equal(t, "registry.aliyuncs.com/google_containers", tc.Spec.ImageRepository)
	assert.Equal(t, "quay.io/coreos", tc.Spec.EtcdImageRepository)
	assert.Equal(t, "v3.6.0", tc.Spec.EtcdVersion)
	assert.Equal(t, "10.96.0.0/12", tc.Spec.ServiceClusterIPRange)
	assert.Equal(t, "10.244.0.0/16", tc.Spec.PodCIDR)
	assert.Equal(t, "local-path", tc.Spec.EtcdStorageClass)
	assert.Equal(t, "1Gi", tc.Spec.EtcdStorageSize)
	assert.Equal(t, "kube-system", tc.Spec.HostCASecretRef.Namespace)
	assert.Equal(t, appsv1alpha1.TenantClusterPhasePending, tc.Status.Phase)
}

// Helper functions

func newDefaultTestHandler() *TenantClusterHandler {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = appsv1alpha1.AddToScheme(scheme)

	client := fake.NewClientBuilder().WithScheme(scheme).Build()
	return &TenantClusterHandler{Client: client}
}

func newDefaultTestTenantCluster() *appsv1alpha1.TenantCluster {
	return &appsv1alpha1.TenantCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
		Spec: appsv1alpha1.TenantClusterSpec{
			KubernetesVersion:    "v1.32.0",
			APIServerServiceIP:   "10.96.100.1",
			APIServerNodePort:    31000,
			HostControlPlaneAddr: "192.168.24.180:6443",
			HostCASecretRef: appsv1alpha1.SecretReference{
				Name:      "host-k8s-ca",
				Namespace: "kube-system",
			},
		},
	}
}
