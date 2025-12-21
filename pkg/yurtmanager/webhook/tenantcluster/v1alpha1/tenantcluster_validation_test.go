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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/openyurtio/openyurt/pkg/apis/apps/v1alpha1"
)

func newTestTenantCluster(name string) *v1alpha1.TenantCluster {
	replicas := int32(3)
	return &v1alpha1.TenantCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: v1alpha1.TenantClusterSpec{
			KubernetesVersion:    "v1.32.0",
			APIServerServiceIP:   "10.96.100.1",
			APIServerNodePort:    31000,
			HostControlPlaneAddr: "192.168.24.180:6443",
			HostCASecretRef: v1alpha1.SecretReference{
				Name:      "host-k8s-ca",
				Namespace: "kube-system",
			},
			EtcdReplicas: &replicas,
		},
	}
}

func TestTenantClusterHandler_ValidateCreate(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = v1alpha1.AddToScheme(scheme)

	// Create the host CA secret
	hostCASecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "host-k8s-ca",
			Namespace: "kube-system",
		},
		Data: map[string][]byte{
			"ca.crt": []byte("fake-ca-cert"),
			"ca.key": []byte("fake-ca-key"),
		},
	}

	tests := []struct {
		name        string
		tc          *v1alpha1.TenantCluster
		objects     []runtime.Object
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid tenant cluster",
			tc:          newTestTenantCluster("test"),
			objects:     []runtime.Object{hostCASecret},
			expectError: false,
		},
		{
			name: "invalid K8s version format",
			tc: func() *v1alpha1.TenantCluster {
				tc := newTestTenantCluster("test")
				tc.Spec.KubernetesVersion = "1.32.0" // missing 'v' prefix
				return tc
			}(),
			objects:     []runtime.Object{hostCASecret},
			expectError: true,
			errorMsg:    "kubernetesVersion",
		},
		{
			name: "invalid API server service IP",
			tc: func() *v1alpha1.TenantCluster {
				tc := newTestTenantCluster("test")
				tc.Spec.APIServerServiceIP = "invalid-ip"
				return tc
			}(),
			objects:     []runtime.Object{hostCASecret},
			expectError: true,
			errorMsg:    "apiServerServiceIP",
		},
		{
			name: "invalid NodePort - too low",
			tc: func() *v1alpha1.TenantCluster {
				tc := newTestTenantCluster("test")
				tc.Spec.APIServerNodePort = 29999
				return tc
			}(),
			objects:     []runtime.Object{hostCASecret},
			expectError: true,
			errorMsg:    "apiServerNodePort",
		},
		{
			name: "invalid NodePort - too high",
			tc: func() *v1alpha1.TenantCluster {
				tc := newTestTenantCluster("test")
				tc.Spec.APIServerNodePort = 32768
				return tc
			}(),
			objects:     []runtime.Object{hostCASecret},
			expectError: true,
			errorMsg:    "apiServerNodePort",
		},
		{
			name: "missing host CA secret reference",
			tc: func() *v1alpha1.TenantCluster {
				tc := newTestTenantCluster("test")
				tc.Spec.HostCASecretRef.Name = ""
				return tc
			}(),
			objects:     []runtime.Object{hostCASecret},
			expectError: true,
			errorMsg:    "hostCASecretRef",
		},
		{
			name: "host CA secret not found",
			tc: func() *v1alpha1.TenantCluster {
				tc := newTestTenantCluster("test")
				tc.Spec.HostCASecretRef.Name = "non-existent"
				return tc
			}(),
			objects:     []runtime.Object{}, // No secret
			expectError: true,
			errorMsg:    "hostCASecretRef",
		},
		{
			name: "even number of etcd replicas",
			tc: func() *v1alpha1.TenantCluster {
				tc := newTestTenantCluster("test")
				replicas := int32(2)
				tc.Spec.EtcdReplicas = &replicas
				return tc
			}(),
			objects:     []runtime.Object{hostCASecret},
			expectError: true,
			errorMsg:    "etcdReplicas",
		},
		{
			name: "too many etcd replicas",
			tc: func() *v1alpha1.TenantCluster {
				tc := newTestTenantCluster("test")
				replicas := int32(9)
				tc.Spec.EtcdReplicas = &replicas
				return tc
			}(),
			objects:     []runtime.Object{hostCASecret},
			expectError: true,
			errorMsg:    "etcdReplicas",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(tt.objects...).Build()
			handler := &TenantClusterHandler{Client: client}

			_, err := handler.ValidateCreate(context.Background(), tt.tc)
			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestTenantClusterHandler_ValidateUpdate(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = v1alpha1.AddToScheme(scheme)

	hostCASecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "host-k8s-ca",
			Namespace: "kube-system",
		},
		Data: map[string][]byte{
			"ca.crt": []byte("fake-ca-cert"),
			"ca.key": []byte("fake-ca-key"),
		},
	}

	otherCASecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "other-ca",
			Namespace: "kube-system",
		},
		Data: map[string][]byte{
			"ca.crt": []byte("fake-ca-cert"),
			"ca.key": []byte("fake-ca-key"),
		},
	}

	tests := []struct {
		name        string
		oldTC       *v1alpha1.TenantCluster
		newTC       *v1alpha1.TenantCluster
		objects     []runtime.Object
		expectError bool
		errorMsg    string
	}{
		{
			name:  "valid update - no immutable field changes",
			oldTC: newTestTenantCluster("test"),
			newTC: func() *v1alpha1.TenantCluster {
				tc := newTestTenantCluster("test")
				replicas := int32(5)
				tc.Spec.EtcdReplicas = &replicas
				return tc
			}(),
			objects:     []runtime.Object{hostCASecret},
			expectError: false,
		},
		{
			name:  "invalid update - change apiServerServiceIP",
			oldTC: newTestTenantCluster("test"),
			newTC: func() *v1alpha1.TenantCluster {
				tc := newTestTenantCluster("test")
				tc.Spec.APIServerServiceIP = "10.96.100.2"
				return tc
			}(),
			objects:     []runtime.Object{hostCASecret},
			expectError: true,
			errorMsg:    "immutable",
		},
		{
			name:  "invalid update - change apiServerNodePort",
			oldTC: newTestTenantCluster("test"),
			newTC: func() *v1alpha1.TenantCluster {
				tc := newTestTenantCluster("test")
				tc.Spec.APIServerNodePort = 31001
				return tc
			}(),
			objects:     []runtime.Object{hostCASecret},
			expectError: true,
			errorMsg:    "immutable",
		},
		{
			name:  "invalid update - change hostCASecretRef",
			oldTC: newTestTenantCluster("test"),
			newTC: func() *v1alpha1.TenantCluster {
				tc := newTestTenantCluster("test")
				tc.Spec.HostCASecretRef.Name = "other-ca"
				return tc
			}(),
			objects:     []runtime.Object{hostCASecret, otherCASecret},
			expectError: true,
			errorMsg:    "immutable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(tt.objects...).Build()
			handler := &TenantClusterHandler{Client: client}

			_, err := handler.ValidateUpdate(context.Background(), tt.oldTC, tt.newTC)
			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
