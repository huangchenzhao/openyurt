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
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGenerateKeyPair(t *testing.T) {
	key, err := generateKeyPair()
	require.NoError(t, err)
	assert.NotNil(t, key)
	assert.Equal(t, rsaKeySize, key.Size()*8)
}

func TestEncodePrivateKey(t *testing.T) {
	key, err := generateKeyPair()
	require.NoError(t, err)

	encoded := encodePrivateKey(key)
	assert.NotNil(t, encoded)
	assert.NotEmpty(t, encoded)

	// Verify it's a valid PEM block
	block, _ := pem.Decode(encoded)
	assert.NotNil(t, block)
	assert.Equal(t, "RSA PRIVATE KEY", block.Type)

	// Verify we can decode it back
	decodedKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	require.NoError(t, err)
	assert.NotNil(t, decodedKey)
}

func TestEncodePublicKey(t *testing.T) {
	key, err := generateKeyPair()
	require.NoError(t, err)

	encoded, err := encodePublicKey(&key.PublicKey)
	require.NoError(t, err)
	assert.NotNil(t, encoded)
	assert.NotEmpty(t, encoded)

	// Verify it's a valid PEM block
	block, _ := pem.Decode(encoded)
	assert.NotNil(t, block)
	assert.Equal(t, "PUBLIC KEY", block.Type)
}

func TestGenerateCertificate(t *testing.T) {
	// Generate CA
	caKey, err := generateKeyPair()
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName:   "Test CA",
			Organization: []string{"Test Org"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageKeyEncipherment,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	certBytes, err := generateCertificate(template, template, &caKey.PublicKey, caKey)
	require.NoError(t, err)
	assert.NotNil(t, certBytes)
	assert.NotEmpty(t, certBytes)

	// Verify it's a valid PEM block
	block, _ := pem.Decode(certBytes)
	assert.NotNil(t, block)
	assert.Equal(t, "CERTIFICATE", block.Type)

	// Parse the certificate
	cert, err := x509.ParseCertificate(block.Bytes)
	require.NoError(t, err)
	assert.Equal(t, "Test CA", cert.Subject.CommonName)
	assert.True(t, cert.IsCA)
}

func TestGenerateCertificateWithParent(t *testing.T) {
	// Generate CA
	caKey, err := generateKeyPair()
	require.NoError(t, err)

	caTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: "Test CA",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageKeyEncipherment,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	caCertBytes, err := generateCertificate(caTemplate, caTemplate, &caKey.PublicKey, caKey)
	require.NoError(t, err)

	// Parse CA cert
	block, _ := pem.Decode(caCertBytes)
	caCert, err := x509.ParseCertificate(block.Bytes)
	require.NoError(t, err)

	// Generate server cert signed by CA
	serverKey, err := generateKeyPair()
	require.NoError(t, err)

	serverTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject: pkix.Name{
			CommonName: "Test Server",
		},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().Add(24 * time.Hour),
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:    []string{"localhost"},
	}

	serverCertBytes, err := generateCertificate(serverTemplate, caCert, &serverKey.PublicKey, caKey)
	require.NoError(t, err)
	assert.NotNil(t, serverCertBytes)

	// Parse server cert
	block, _ = pem.Decode(serverCertBytes)
	serverCert, err := x509.ParseCertificate(block.Bytes)
	require.NoError(t, err)
	assert.Equal(t, "Test Server", serverCert.Subject.CommonName)
	assert.False(t, serverCert.IsCA)

	// Verify the chain
	roots := x509.NewCertPool()
	roots.AddCert(caCert)
	_, err = serverCert.Verify(x509.VerifyOptions{
		Roots: roots,
	})
	require.NoError(t, err)
}

func TestParseCAFromSecret(t *testing.T) {
	// Generate a test CA
	caKey, err := generateKeyPair()
	require.NoError(t, err)

	caTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: "Test CA",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageKeyEncipherment,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	caCertBytes, err := generateCertificate(caTemplate, caTemplate, &caKey.PublicKey, caKey)
	require.NoError(t, err)

	tests := []struct {
		name        string
		secret      *corev1.Secret
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid CA secret",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-ca",
					Namespace: "kube-system",
				},
				Data: map[string][]byte{
					"ca.crt": caCertBytes,
					"ca.key": encodePrivateKey(caKey),
				},
			},
			expectError: false,
		},
		{
			name: "missing ca.crt",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-ca",
					Namespace: "kube-system",
				},
				Data: map[string][]byte{
					"ca.key": encodePrivateKey(caKey),
				},
			},
			expectError: true,
			errorMsg:    "ca.crt",
		},
		{
			name: "missing ca.key",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-ca",
					Namespace: "kube-system",
				},
				Data: map[string][]byte{
					"ca.crt": caCertBytes,
				},
			},
			expectError: true,
			errorMsg:    "ca.key",
		},
		{
			name: "invalid ca.crt",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-ca",
					Namespace: "kube-system",
				},
				Data: map[string][]byte{
					"ca.crt": []byte("not a valid cert"),
					"ca.key": encodePrivateKey(caKey),
				},
			},
			expectError: true,
			errorMsg:    "decode",
		},
		{
			name: "invalid ca.key",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-ca",
					Namespace: "kube-system",
				},
				Data: map[string][]byte{
					"ca.crt": caCertBytes,
					"ca.key": []byte("not a valid key"),
				},
			},
			expectError: true,
			errorMsg:    "decode",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cert, key, err := parseCAFromSecret(tt.secret)
			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				require.NoError(t, err)
				assert.NotNil(t, cert)
				assert.NotNil(t, key)
				assert.Equal(t, "Test CA", cert.Subject.CommonName)
			}
		})
	}
}

func TestGenerateBootstrapToken(t *testing.T) {
	r := &ReconcileTenantCluster{}

	tokenID, tokenSecret, err := r.generateBootstrapToken()
	require.NoError(t, err)

	// Token ID should be 6 characters (3 bytes hex encoded)
	assert.Len(t, tokenID, 6)

	// Token secret should be 16 characters (8 bytes hex encoded)
	assert.Len(t, tokenSecret, 16)

	// Generate another and verify they're different
	tokenID2, tokenSecret2, err := r.generateBootstrapToken()
	require.NoError(t, err)
	assert.NotEqual(t, tokenID, tokenID2)
	assert.NotEqual(t, tokenSecret, tokenSecret2)
}

func TestSplitBootstrapToken(t *testing.T) {
	tests := []struct {
		name         string
		token        string
		expectParts  int
		expectedID   string
		expectedSec  string
	}{
		{
			name:        "valid token",
			token:       "abc123.xyz789def0",
			expectParts: 2,
			expectedID:  "abc123",
			expectedSec: "xyz789def0",
		},
		{
			name:        "no separator",
			token:       "abcxyz",
			expectParts: 0,
		},
		{
			name:        "empty string",
			token:       "",
			expectParts: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parts := splitBootstrapToken(tt.token)
			assert.Len(t, parts, tt.expectParts)
			if tt.expectParts == 2 {
				assert.Equal(t, tt.expectedID, parts[0])
				assert.Equal(t, tt.expectedSec, parts[1])
			}
		})
	}
}

// Helper to generate a test CA for other tests
func generateTestCA(t *testing.T) (*x509.Certificate, *rsa.PrivateKey) {
	caKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	caTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: "Test CA",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageKeyEncipherment,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	caCertDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	require.NoError(t, err)

	caCert, err := x509.ParseCertificate(caCertDER)
	require.NoError(t, err)

	return caCert, caKey
}
