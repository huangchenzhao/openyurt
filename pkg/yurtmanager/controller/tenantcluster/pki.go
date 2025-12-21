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
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/openyurtio/openyurt/pkg/apis/apps/v1alpha1"
)

const (
	// Certificate validity period
	certValidityDays = 3650 // 10 years

	// RSA key size
	rsaKeySize = 2048
)

// generatePKI generates all PKI materials for the tenant cluster
func (r *ReconcileTenantCluster) generatePKI(ctx context.Context, tc *appsv1alpha1.TenantCluster) error {
	klog.Info(Format("Generating PKI materials for TenantCluster %s", tc.Name))

	// Get host CA from secret
	hostCASecret := &corev1.Secret{}
	hostCASecretKey := client.ObjectKey{
		Name:      tc.Spec.HostCASecretRef.Name,
		Namespace: tc.Spec.HostCASecretRef.Namespace,
	}
	if hostCASecretKey.Namespace == "" {
		hostCASecretKey.Namespace = "kube-system"
	}
	if err := r.Get(ctx, hostCASecretKey, hostCASecret); err != nil {
		return fmt.Errorf("failed to get host CA secret: %w", err)
	}

	hostCACert, hostCAKey, err := parseCAFromSecret(hostCASecret)
	if err != nil {
		return fmt.Errorf("failed to parse host CA: %w", err)
	}

	namespace := tc.GetNamespace()

	// Copy host CA to tenant namespace
	if err := r.createHostCASecret(ctx, tc, hostCASecret); err != nil {
		return err
	}

	// 1. Generate etcd server certificate
	if err := r.generateEtcdServerCert(ctx, tc, namespace, hostCACert, hostCAKey); err != nil {
		return err
	}

	// 2. Generate etcd client certificate for apiserver
	if err := r.generateEtcdClientCert(ctx, tc, namespace, hostCACert, hostCAKey); err != nil {
		return err
	}

	// 3. Generate front-proxy CA (self-signed)
	frontProxyCACert, frontProxyCAKey, err := r.generateFrontProxyCA(ctx, tc, namespace)
	if err != nil {
		return err
	}

	// 4. Generate service account keys
	if err := r.generateSAKeys(ctx, tc, namespace); err != nil {
		return err
	}

	// 5. Generate apiserver certificate
	if err := r.generateAPIServerCert(ctx, tc, namespace, hostCACert, hostCAKey); err != nil {
		return err
	}

	// 6. Generate apiserver-kubelet-client certificate
	if err := r.generateAPIServerKubeletClientCert(ctx, tc, namespace, hostCACert, hostCAKey); err != nil {
		return err
	}

	// 7. Generate front-proxy-client certificate
	if err := r.generateFrontProxyClientCert(ctx, tc, namespace, frontProxyCACert, frontProxyCAKey); err != nil {
		return err
	}

	// 8-10. Generate kubeconfigs
	if err := r.generateKCMKubeconfig(ctx, tc, namespace, hostCACert, hostCAKey); err != nil {
		return err
	}
	if err := r.generateSchedulerKubeconfig(ctx, tc, namespace, hostCACert, hostCAKey); err != nil {
		return err
	}
	if err := r.generateAdminKubeconfig(ctx, tc, namespace, hostCACert, hostCAKey); err != nil {
		return err
	}

	klog.Info(Format("PKI materials generated successfully for TenantCluster %s", tc.Name))
	r.recorder.Event(tc, corev1.EventTypeNormal, "PKIGenerated", "All PKI materials generated successfully")
	return nil
}

func parseCAFromSecret(secret *corev1.Secret) (*x509.Certificate, *rsa.PrivateKey, error) {
	caCertPEM, ok := secret.Data["ca.crt"]
	if !ok {
		return nil, nil, fmt.Errorf("ca.crt not found in secret")
	}
	caKeyPEM, ok := secret.Data["ca.key"]
	if !ok {
		return nil, nil, fmt.Errorf("ca.key not found in secret")
	}

	caCertBlock, _ := pem.Decode(caCertPEM)
	if caCertBlock == nil {
		return nil, nil, fmt.Errorf("failed to decode CA certificate PEM")
	}
	caCert, err := x509.ParseCertificate(caCertBlock.Bytes)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse CA certificate: %w", err)
	}

	caKeyBlock, _ := pem.Decode(caKeyPEM)
	if caKeyBlock == nil {
		return nil, nil, fmt.Errorf("failed to decode CA key PEM")
	}
	caKey, err := x509.ParsePKCS1PrivateKey(caKeyBlock.Bytes)
	if err != nil {
		// Try PKCS8
		key, err := x509.ParsePKCS8PrivateKey(caKeyBlock.Bytes)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to parse CA key: %w", err)
		}
		var ok bool
		caKey, ok = key.(*rsa.PrivateKey)
		if !ok {
			return nil, nil, fmt.Errorf("CA key is not RSA")
		}
	}

	return caCert, caKey, nil
}

func (r *ReconcileTenantCluster) createHostCASecret(ctx context.Context, tc *appsv1alpha1.TenantCluster, hostCASecret *corev1.Secret) error {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "host-k8s-ca-key-pair",
			Namespace: tc.GetNamespace(),
		},
		Data: map[string][]byte{
			"ca.crt": hostCASecret.Data["ca.crt"],
			"ca.key": hostCASecret.Data["ca.key"],
		},
	}

	existing := &corev1.Secret{}
	if err := r.Get(ctx, client.ObjectKeyFromObject(secret), existing); err != nil {
		if errors.IsNotFound(err) {
			if err := r.Create(ctx, secret); err != nil {
				return fmt.Errorf("failed to create host-k8s-ca-key-pair secret: %w", err)
			}
			klog.Info(Format("Created host-k8s-ca-key-pair secret for TenantCluster %s", tc.Name))
		} else {
			return err
		}
	}
	return nil
}

func generateKeyPair() (*rsa.PrivateKey, error) {
	return rsa.GenerateKey(rand.Reader, rsaKeySize)
}

func generateCertificate(template, parent *x509.Certificate, publicKey *rsa.PublicKey, privateKey *rsa.PrivateKey) ([]byte, error) {
	certDER, err := x509.CreateCertificate(rand.Reader, template, parent, publicKey, privateKey)
	if err != nil {
		return nil, err
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER}), nil
}

func encodePrivateKey(key *rsa.PrivateKey) []byte {
	return pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
}

func encodePublicKey(key *rsa.PublicKey) ([]byte, error) {
	pubBytes, err := x509.MarshalPKIXPublicKey(key)
	if err != nil {
		return nil, err
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubBytes}), nil
}

func (r *ReconcileTenantCluster) createSecretIfNotExists(ctx context.Context, secret *corev1.Secret) error {
	existing := &corev1.Secret{}
	if err := r.Get(ctx, client.ObjectKeyFromObject(secret), existing); err != nil {
		if errors.IsNotFound(err) {
			return r.Create(ctx, secret)
		}
		return err
	}
	return nil
}

func (r *ReconcileTenantCluster) generateEtcdServerCert(ctx context.Context, tc *appsv1alpha1.TenantCluster, namespace string, caCert *x509.Certificate, caKey *rsa.PrivateKey) error {
	key, err := generateKeyPair()
	if err != nil {
		return fmt.Errorf("failed to generate etcd server key: %w", err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: "etcd-server"},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().AddDate(0, 0, certValidityDays),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		DNSNames: []string{
			"etcd",
			fmt.Sprintf("etcd.%s.svc", namespace),
			fmt.Sprintf("*.etcd.%s.svc", namespace),
			"etcd-0.etcd",
			"etcd-1.etcd",
			"etcd-2.etcd",
		},
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1")},
	}

	certPEM, err := generateCertificate(template, caCert, &key.PublicKey, caKey)
	if err != nil {
		return fmt.Errorf("failed to generate etcd server certificate: %w", err)
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tenant-etcd-server-tls",
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"tls.crt": certPEM,
			"tls.key": encodePrivateKey(key),
			"ca.crt":  pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caCert.Raw}),
		},
	}

	return r.createSecretIfNotExists(ctx, secret)
}

func (r *ReconcileTenantCluster) generateEtcdClientCert(ctx context.Context, tc *appsv1alpha1.TenantCluster, namespace string, caCert *x509.Certificate, caKey *rsa.PrivateKey) error {
	key, err := generateKeyPair()
	if err != nil {
		return fmt.Errorf("failed to generate etcd client key: %w", err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: "etcd-client"},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().AddDate(0, 0, certValidityDays),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		DNSNames: []string{
			"etcd",
			fmt.Sprintf("etcd.%s.svc", namespace),
			"etcd-0.etcd",
			"etcd-1.etcd",
			"etcd-2.etcd",
		},
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1")},
	}

	certPEM, err := generateCertificate(template, caCert, &key.PublicKey, caKey)
	if err != nil {
		return fmt.Errorf("failed to generate etcd client certificate: %w", err)
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tenant-apiserver-etcd-client-certs",
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"tls.crt": certPEM,
			"tls.key": encodePrivateKey(key),
			"ca.crt":  pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caCert.Raw}),
		},
	}

	return r.createSecretIfNotExists(ctx, secret)
}

func (r *ReconcileTenantCluster) generateFrontProxyCA(ctx context.Context, tc *appsv1alpha1.TenantCluster, namespace string) (*x509.Certificate, *rsa.PrivateKey, error) {
	key, err := generateKeyPair()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate front-proxy CA key: %w", err)
	}

	template := &x509.Certificate{
		SerialNumber:          big.NewInt(time.Now().UnixNano()),
		Subject:               pkix.Name{CommonName: "front-proxy-ca"},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(0, 0, certValidityDays),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	certPEM, err := generateCertificate(template, template, &key.PublicKey, key)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate front-proxy CA certificate: %w", err)
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tenant-front-proxy-ca",
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"front-proxy-ca.crt": certPEM,
			"front-proxy-ca.key": encodePrivateKey(key),
		},
	}

	if err := r.createSecretIfNotExists(ctx, secret); err != nil {
		return nil, nil, err
	}

	// Parse the cert back
	block, _ := pem.Decode(certPEM)
	cert, _ := x509.ParseCertificate(block.Bytes)
	return cert, key, nil
}

func (r *ReconcileTenantCluster) generateSAKeys(ctx context.Context, tc *appsv1alpha1.TenantCluster, namespace string) error {
	key, err := generateKeyPair()
	if err != nil {
		return fmt.Errorf("failed to generate SA key: %w", err)
	}

	pubPEM, err := encodePublicKey(&key.PublicKey)
	if err != nil {
		return fmt.Errorf("failed to encode SA public key: %w", err)
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tenant-sa-keys",
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"sa.key": encodePrivateKey(key),
			"sa.pub": pubPEM,
		},
	}

	return r.createSecretIfNotExists(ctx, secret)
}

func (r *ReconcileTenantCluster) generateAPIServerCert(ctx context.Context, tc *appsv1alpha1.TenantCluster, namespace string, caCert *x509.Certificate, caKey *rsa.PrivateKey) error {
	key, err := generateKeyPair()
	if err != nil {
		return fmt.Errorf("failed to generate apiserver key: %w", err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: "kube-apiserver"},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().AddDate(0, 0, certValidityDays),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames: []string{
			"kubernetes",
			"kubernetes.default",
			"kubernetes.default.svc",
			"kubernetes.default.svc.cluster.local",
			fmt.Sprintf("tenant-apiserver.%s.svc", namespace),
		},
		IPAddresses: []net.IP{
			net.ParseIP("127.0.0.1"),
			net.ParseIP("10.96.0.1"), // Default kubernetes service IP
			net.ParseIP(tc.Spec.APIServerServiceIP),
		},
	}

	certPEM, err := generateCertificate(template, caCert, &key.PublicKey, caKey)
	if err != nil {
		return fmt.Errorf("failed to generate apiserver certificate: %w", err)
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tenant-apiserver-certs",
			Namespace: namespace,
		},
		Type: corev1.SecretTypeTLS,
		Data: map[string][]byte{
			"tls.crt": certPEM,
			"tls.key": encodePrivateKey(key),
		},
	}

	return r.createSecretIfNotExists(ctx, secret)
}

func (r *ReconcileTenantCluster) generateAPIServerKubeletClientCert(ctx context.Context, tc *appsv1alpha1.TenantCluster, namespace string, caCert *x509.Certificate, caKey *rsa.PrivateKey) error {
	key, err := generateKeyPair()
	if err != nil {
		return fmt.Errorf("failed to generate apiserver-kubelet-client key: %w", err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject: pkix.Name{
			CommonName:   "kube-apiserver-kubelet-client",
			Organization: []string{"system:masters"},
		},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().AddDate(0, 0, certValidityDays),
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}

	certPEM, err := generateCertificate(template, caCert, &key.PublicKey, caKey)
	if err != nil {
		return fmt.Errorf("failed to generate apiserver-kubelet-client certificate: %w", err)
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tenant-apiserver-kubelet-client-certs",
			Namespace: namespace,
		},
		Type: corev1.SecretTypeTLS,
		Data: map[string][]byte{
			"tls.crt": certPEM,
			"tls.key": encodePrivateKey(key),
		},
	}

	return r.createSecretIfNotExists(ctx, secret)
}

func (r *ReconcileTenantCluster) generateFrontProxyClientCert(ctx context.Context, tc *appsv1alpha1.TenantCluster, namespace string, caCert *x509.Certificate, caKey *rsa.PrivateKey) error {
	key, err := generateKeyPair()
	if err != nil {
		return fmt.Errorf("failed to generate front-proxy-client key: %w", err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: "front-proxy-client"},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().AddDate(0, 0, certValidityDays),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}

	certPEM, err := generateCertificate(template, caCert, &key.PublicKey, caKey)
	if err != nil {
		return fmt.Errorf("failed to generate front-proxy-client certificate: %w", err)
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tenant-front-proxy-client-certs",
			Namespace: namespace,
		},
		Type: corev1.SecretTypeTLS,
		Data: map[string][]byte{
			"tls.crt": certPEM,
			"tls.key": encodePrivateKey(key),
		},
	}

	return r.createSecretIfNotExists(ctx, secret)
}

func (r *ReconcileTenantCluster) generateKubeconfig(caCert *x509.Certificate, caKey *rsa.PrivateKey, serverURL, user, org string) ([]byte, error) {
	// Generate client certificate
	key, err := generateKeyPair()
	if err != nil {
		return nil, err
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject: pkix.Name{
			CommonName:   user,
			Organization: []string{org},
		},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().AddDate(0, 0, certValidityDays),
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}

	certPEM, err := generateCertificate(template, caCert, &key.PublicKey, caKey)
	if err != nil {
		return nil, err
	}

	caCertPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caCert.Raw})

	config := clientcmdapi.Config{
		Clusters: map[string]*clientcmdapi.Cluster{
			"tenant": {
				Server:                   serverURL,
				CertificateAuthorityData: caCertPEM,
			},
		},
		AuthInfos: map[string]*clientcmdapi.AuthInfo{
			user: {
				ClientCertificateData: certPEM,
				ClientKeyData:         encodePrivateKey(key),
			},
		},
		Contexts: map[string]*clientcmdapi.Context{
			"default": {
				Cluster:  "tenant",
				AuthInfo: user,
			},
		},
		CurrentContext: "default",
	}

	return clientcmd.Write(config)
}

func (r *ReconcileTenantCluster) generateKCMKubeconfig(ctx context.Context, tc *appsv1alpha1.TenantCluster, namespace string, caCert *x509.Certificate, caKey *rsa.PrivateKey) error {
	serverURL := fmt.Sprintf("https://tenant-apiserver.%s.svc:6443", namespace)
	kubeconfig, err := r.generateKubeconfig(caCert, caKey, serverURL, "system:kube-controller-manager", "system:kube-controller-manager")
	if err != nil {
		return fmt.Errorf("failed to generate KCM kubeconfig: %w", err)
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tenant-kcm-kubeconfig",
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"kubeconfig": kubeconfig,
		},
	}

	return r.createSecretIfNotExists(ctx, secret)
}

func (r *ReconcileTenantCluster) generateSchedulerKubeconfig(ctx context.Context, tc *appsv1alpha1.TenantCluster, namespace string, caCert *x509.Certificate, caKey *rsa.PrivateKey) error {
	serverURL := fmt.Sprintf("https://tenant-apiserver.%s.svc:6443", namespace)
	kubeconfig, err := r.generateKubeconfig(caCert, caKey, serverURL, "system:kube-scheduler", "system:kube-scheduler")
	if err != nil {
		return fmt.Errorf("failed to generate scheduler kubeconfig: %w", err)
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tenant-scheduler-kubeconfig",
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"kubeconfig": kubeconfig,
		},
	}

	return r.createSecretIfNotExists(ctx, secret)
}

func (r *ReconcileTenantCluster) generateAdminKubeconfig(ctx context.Context, tc *appsv1alpha1.TenantCluster, namespace string, caCert *x509.Certificate, caKey *rsa.PrivateKey) error {
	// Admin kubeconfig uses NodePort for external access
	serverURL := fmt.Sprintf("https://127.0.0.1:%d", tc.Spec.APIServerNodePort)
	kubeconfig, err := r.generateKubeconfig(caCert, caKey, serverURL, "admin", "system:masters")
	if err != nil {
		return fmt.Errorf("failed to generate admin kubeconfig: %w", err)
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tenant-admin-kubeconfig",
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"kubeconfig": kubeconfig,
		},
	}

	return r.createSecretIfNotExists(ctx, secret)
}

// generateBootstrapToken generates a bootstrap token for node joining
func (r *ReconcileTenantCluster) generateBootstrapToken() (tokenID, tokenSecret string, err error) {
	// Generate random token ID (6 chars)
	tokenIDBytes := make([]byte, 3)
	if _, err := rand.Read(tokenIDBytes); err != nil {
		return "", "", err
	}
	tokenID = fmt.Sprintf("%x", tokenIDBytes)

	// Generate random token secret (16 chars)
	tokenSecretBytes := make([]byte, 8)
	if _, err := rand.Read(tokenSecretBytes); err != nil {
		return "", "", err
	}
	tokenSecret = fmt.Sprintf("%x", tokenSecretBytes)

	return tokenID, tokenSecret, nil
}
