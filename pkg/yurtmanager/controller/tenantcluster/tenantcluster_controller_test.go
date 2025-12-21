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
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appsv1alpha1 "github.com/openyurtio/openyurt/pkg/apis/apps/v1alpha1"
	tenantclusterconfig "github.com/openyurtio/openyurt/pkg/yurtmanager/controller/tenantcluster/config"
)

func newTestTenantCluster(name string) *appsv1alpha1.TenantCluster {
	replicas := int32(3)
	return &appsv1alpha1.TenantCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
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
			EtcdReplicas: &replicas,
		},
	}
}

func TestTenantCluster_GetNamespace(t *testing.T) {
	tests := []struct {
		name           string
		clusterName    string
		wantNamespace  string
	}{
		{
			name:          "simple name",
			clusterName:   "test",
			wantNamespace: "tenant-test",
		},
		{
			name:          "complex name",
			clusterName:   "my-tenant-cluster",
			wantNamespace: "tenant-my-tenant-cluster",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc := newTestTenantCluster(tt.clusterName)
			assert.Equal(t, tt.wantNamespace, tc.GetNamespace())
		})
	}
}

func TestTenantCluster_GetEtcdReplicas(t *testing.T) {
	tests := []struct {
		name     string
		replicas *int32
		want     int32
	}{
		{
			name:     "nil replicas returns default",
			replicas: nil,
			want:     3,
		},
		{
			name:     "specified replicas",
			replicas: int32Ptr(5),
			want:     5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc := newTestTenantCluster("test")
			tc.Spec.EtcdReplicas = tt.replicas
			assert.Equal(t, tt.want, tc.GetEtcdReplicas())
		})
	}
}

func TestTenantCluster_IsReady(t *testing.T) {
	tests := []struct {
		name       string
		conditions []appsv1alpha1.TenantClusterConditionType
		want       bool
	}{
		{
			name:       "no conditions",
			conditions: nil,
			want:       false,
		},
		{
			name: "all conditions true",
			conditions: []appsv1alpha1.TenantClusterConditionType{
				appsv1alpha1.TenantClusterConditionPKIReady,
				appsv1alpha1.TenantClusterConditionEtcdReady,
				appsv1alpha1.TenantClusterConditionAPIServerReady,
				appsv1alpha1.TenantClusterConditionSchedulerReady,
				appsv1alpha1.TenantClusterConditionControllerManagerReady,
				appsv1alpha1.TenantClusterConditionPostInstallReady,
			},
			want: true,
		},
		{
			name: "missing condition",
			conditions: []appsv1alpha1.TenantClusterConditionType{
				appsv1alpha1.TenantClusterConditionPKIReady,
				appsv1alpha1.TenantClusterConditionEtcdReady,
				appsv1alpha1.TenantClusterConditionAPIServerReady,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc := newTestTenantCluster("test")
			for _, cond := range tt.conditions {
				tc.SetCondition(cond, corev1.ConditionTrue, "Ready", "Component is ready")
			}
			assert.Equal(t, tt.want, tc.IsReady())
		})
	}
}

func TestTenantCluster_SetCondition(t *testing.T) {
	tc := newTestTenantCluster("test")

	// Set a new condition
	tc.SetCondition(appsv1alpha1.TenantClusterConditionPKIReady, corev1.ConditionTrue, "PKIReady", "PKI is ready")
	cond := tc.GetCondition(appsv1alpha1.TenantClusterConditionPKIReady)
	assert.NotNil(t, cond)
	assert.Equal(t, string(appsv1alpha1.TenantClusterConditionPKIReady), cond.Type)
	assert.Equal(t, metav1.ConditionTrue, cond.Status)
	assert.Equal(t, "PKIReady", cond.Reason)
	assert.Equal(t, "PKI is ready", cond.Message)

	// Update the condition
	tc.SetCondition(appsv1alpha1.TenantClusterConditionPKIReady, corev1.ConditionFalse, "PKIFailed", "PKI failed")
	cond = tc.GetCondition(appsv1alpha1.TenantClusterConditionPKIReady)
	assert.NotNil(t, cond)
	assert.Equal(t, metav1.ConditionFalse, cond.Status)
	assert.Equal(t, "PKIFailed", cond.Reason)
}

func TestReconcileTenantCluster_EnsureNamespace(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = appsv1alpha1.AddToScheme(scheme)

	tests := []struct {
		name          string
		existingNS    bool
		expectError   bool
	}{
		{
			name:        "namespace does not exist",
			existingNS:  false,
			expectError: false,
		},
		{
			name:        "namespace already exists",
			existingNS:  true,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc := newTestTenantCluster("test")
			
			objects := []runtime.Object{}
			if tt.existingNS {
				ns := &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: tc.GetNamespace(),
					},
				}
				objects = append(objects, ns)
			}

			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(objects...).Build()
			r := &ReconcileTenantCluster{
				Client:   fakeClient,
				recorder: record.NewFakeRecorder(10),
				cfg:      tenantclusterconfig.TenantClusterControllerConfiguration{},
			}

			err := r.ensureNamespace(context.Background(), tc)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			// Verify namespace exists
			ns := &corev1.Namespace{}
			err = fakeClient.Get(context.Background(), types.NamespacedName{Name: tc.GetNamespace()}, ns)
			assert.NoError(t, err)
			assert.Equal(t, tc.GetNamespace(), ns.Name)
		})
	}
}

func int32Ptr(i int32) *int32 {
	return &i
}

