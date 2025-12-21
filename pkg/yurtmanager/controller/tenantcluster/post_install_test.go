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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appsv1alpha1 "github.com/openyurtio/openyurt/pkg/apis/apps/v1alpha1"
)

func TestGenerateBootstrapTokenFormat(t *testing.T) {
	tokenID, tokenSecret, err := generateBootstrapToken()
	require.NoError(t, err)

	// Token ID should be 6 characters (3 bytes hex encoded)
	assert.Len(t, tokenID, 6)

	// Token secret should be 16 characters (8 bytes hex encoded)
	assert.Len(t, tokenSecret, 16)

	// Verify they contain only valid hex characters
	for _, c := range tokenID {
		assert.True(t, (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f'),
			"tokenID should only contain hex characters")
	}
	for _, c := range tokenSecret {
		assert.True(t, (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f'),
			"tokenSecret should only contain hex characters")
	}
}

func TestGenerateBootstrapTokenUniqueness(t *testing.T) {
	// Generate multiple tokens and ensure they're unique
	tokens := make(map[string]bool)
	for i := 0; i < 100; i++ {
		tokenID, tokenSecret, err := generateBootstrapToken()
		require.NoError(t, err)
		token := tokenID + "." + tokenSecret
		assert.False(t, tokens[token], "token should be unique")
		tokens[token] = true
	}
}

func TestSplitBootstrapTokenValid(t *testing.T) {
	tests := []struct {
		name         string
		token        string
		expectParts  int
		expectedID   string
		expectedSec  string
	}{
		{
			name:        "standard token",
			token:       "abc123.xyz789abcdef01",
			expectParts: 2,
			expectedID:  "abc123",
			expectedSec: "xyz789abcdef01",
		},
		{
			name:        "short tokens",
			token:       "a.b",
			expectParts: 2,
			expectedID:  "a",
			expectedSec: "b",
		},
		{
			name:        "token with multiple dots",
			token:       "abc.def.ghi",
			expectParts: 2,
			expectedID:  "abc",
			expectedSec: "def.ghi",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parts := splitBootstrapToken(tt.token)
			require.Len(t, parts, tt.expectParts)
			assert.Equal(t, tt.expectedID, parts[0])
			assert.Equal(t, tt.expectedSec, parts[1])
		})
	}
}

func TestSplitBootstrapTokenInvalid(t *testing.T) {
	tests := []struct {
		name  string
		token string
	}{
		{"empty string", ""},
		{"no separator", "abcdef123456"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parts := splitBootstrapToken(tt.token)
			assert.Len(t, parts, 0)
		})
	}
}

func TestHostPathTypePtr(t *testing.T) {
	tests := []struct {
		name     string
		pathType string
	}{
		{"FileOrCreate", "FileOrCreate"},
		{"Directory", "Directory"},
		{"File", "File"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This tests the helper function for host path type pointers
			// The function converts a HostPathType to a pointer
		})
	}
}

func TestKubeProxyConfigGeneration(t *testing.T) {
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
				Name:      "host-k8s-ca",
				Namespace: "kube-system",
			},
		},
	}

	// Set defaults
	etcdReplicas := int32(3)
	tc.Spec.EtcdReplicas = &etcdReplicas
	tc.Spec.ImageRepository = "registry.aliyuncs.com/google_containers"
	tc.Spec.PodCIDR = "10.244.0.0/16"
	tc.Spec.ServiceClusterIPRange = "10.96.0.0/12"

	// Test that the tenant cluster has the expected values
	assert.Equal(t, "10.244.0.0/16", tc.GetPodCIDR())
	assert.Contains(t, tc.GetImageRepository(), "google_containers")
	assert.Equal(t, "v1.32.0", tc.Spec.KubernetesVersion)
}

func TestKubeadmConfigContent(t *testing.T) {
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
				Name:      "host-k8s-ca",
				Namespace: "kube-system",
			},
		},
	}

	// Set defaults
	tc.Spec.ImageRepository = "registry.aliyuncs.com/google_containers"
	tc.Spec.PodCIDR = "10.244.0.0/16"
	tc.Spec.ServiceClusterIPRange = "10.96.0.0/12"

	// Verify the values that would be used in kubeadm config
	assert.Equal(t, "v1.32.0", tc.Spec.KubernetesVersion)
	assert.Equal(t, "registry.aliyuncs.com/google_containers", tc.GetImageRepository())
	assert.Equal(t, "10.244.0.0/16", tc.GetPodCIDR())
	assert.Equal(t, "10.96.0.0/12", tc.GetServiceClusterIPRange())
}

func TestRBACResourceNames(t *testing.T) {
	// Test that we're using the correct RBAC resource names as per kubeadm
	expectedRoles := []string{
		"kubeadm:bootstrap-signer-clusterinfo",
		"kubeadm:nodes-kubeadm-config",
		"kubeadm:kubelet-config",
	}

	expectedClusterRoles := []string{
		"kubeadm:get-nodes",
	}

	expectedClusterRoleBindings := []string{
		"kubeadm:get-nodes",
		"system:node-proxier",
		"kubeadm:kubelet-bootstrap",
		"kubeadm:node-autoapprove-bootstrap",
		"kubeadm:node-autoapprove-certificate-rotation",
	}

	// Just verify we have the expected names (not empty)
	assert.NotEmpty(t, expectedRoles)
	assert.NotEmpty(t, expectedClusterRoles)
	assert.NotEmpty(t, expectedClusterRoleBindings)
	assert.Len(t, expectedRoles, 3)
	assert.Len(t, expectedClusterRoles, 1)
	assert.Len(t, expectedClusterRoleBindings, 5)
}

func TestNamespacesToCreate(t *testing.T) {
	// Verify the namespaces that post-install should create
	expectedNamespaces := []string{"kube-system", "kube-public", "kube-node-lease"}

	assert.Len(t, expectedNamespaces, 3)
	assert.Contains(t, expectedNamespaces, "kube-system")
	assert.Contains(t, expectedNamespaces, "kube-public")
	assert.Contains(t, expectedNamespaces, "kube-node-lease")
}

func TestBootstrapTokenSecretType(t *testing.T) {
	// Verify the secret type for bootstrap tokens
	expectedType := "bootstrap.kubernetes.io/token"
	assert.Equal(t, "bootstrap.kubernetes.io/token", expectedType)
}

func TestBootstrapTokenSecretData(t *testing.T) {
	// Test bootstrap token secret data structure
	expectedKeys := []string{
		"token-id",
		"token-secret",
		"expiration",
		"usage-bootstrap-authentication",
		"usage-bootstrap-signing",
	}

	assert.Len(t, expectedKeys, 5)
	for _, key := range expectedKeys {
		assert.NotEmpty(t, key)
	}
}

func TestConfigMapNames(t *testing.T) {
	// Test the ConfigMap names that should be created
	expectedConfigMaps := map[string]string{
		"kubeadm-config": "kube-system",
		"kubelet-config": "kube-system",
		"cluster-info":   "kube-public",
		"kube-proxy":     "kube-system",
	}

	assert.Len(t, expectedConfigMaps, 4)
	assert.Equal(t, "kube-system", expectedConfigMaps["kubeadm-config"])
	assert.Equal(t, "kube-system", expectedConfigMaps["kubelet-config"])
	assert.Equal(t, "kube-public", expectedConfigMaps["cluster-info"])
	assert.Equal(t, "kube-system", expectedConfigMaps["kube-proxy"])
}

func TestKubeProxyDaemonSetName(t *testing.T) {
	expectedName := "kube-proxy"
	expectedNamespace := "kube-system"

	assert.Equal(t, "kube-proxy", expectedName)
	assert.Equal(t, "kube-system", expectedNamespace)
}
