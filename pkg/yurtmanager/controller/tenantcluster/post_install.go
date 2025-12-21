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
	"encoding/hex"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/openyurtio/openyurt/pkg/apis/apps/v1alpha1"
)

// postInstallReconciler applies configuration to the tenant cluster
type postInstallReconciler struct {
	hostClient   client.Client
	tenantClient kubernetes.Interface
	tc           *appsv1alpha1.TenantCluster
}

// newPostInstallReconciler creates a new post-install reconciler
func (r *ReconcileTenantCluster) newPostInstallReconciler(ctx context.Context, tc *appsv1alpha1.TenantCluster) (*postInstallReconciler, error) {
	// Get admin kubeconfig from secret
	kubeconfigSecret := &corev1.Secret{}
	if err := r.Get(ctx, client.ObjectKey{
		Name:      "tenant-admin-kubeconfig",
		Namespace: tc.GetNamespace(),
	}, kubeconfigSecret); err != nil {
		return nil, fmt.Errorf("failed to get admin kubeconfig: %w", err)
	}

	kubeconfig, ok := kubeconfigSecret.Data["kubeconfig"]
	if !ok {
		return nil, fmt.Errorf("kubeconfig key not found in secret")
	}

	// Create client config from kubeconfig
	clientConfig, err := clientcmd.RESTConfigFromKubeConfig(kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create REST config from kubeconfig: %w", err)
	}

	// Create kubernetes client for tenant cluster
	tenantClient, err := kubernetes.NewForConfig(clientConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create tenant kubernetes client: %w", err)
	}

	return &postInstallReconciler{
		hostClient:   r.Client,
		tenantClient: tenantClient,
		tc:           tc,
	}, nil
}

// reconcile applies all post-install configurations
func (p *postInstallReconciler) reconcile(ctx context.Context) error {
	// 1. Create namespaces
	if err := p.ensureNamespaces(ctx); err != nil {
		return fmt.Errorf("failed to ensure namespaces: %w", err)
	}

	// 2. Apply RBAC
	if err := p.applyRBAC(ctx); err != nil {
		return fmt.Errorf("failed to apply RBAC: %w", err)
	}

	// 3. Create bootstrap token
	if err := p.createBootstrapToken(ctx); err != nil {
		return fmt.Errorf("failed to create bootstrap token: %w", err)
	}

	// 4. Create kubeadm config
	if err := p.createKubeadmConfig(ctx); err != nil {
		return fmt.Errorf("failed to create kubeadm config: %w", err)
	}

	// 5. Create kubelet config
	if err := p.createKubeletConfig(ctx); err != nil {
		return fmt.Errorf("failed to create kubelet config: %w", err)
	}

	// 6. Create cluster-info ConfigMap
	if err := p.createClusterInfo(ctx); err != nil {
		return fmt.Errorf("failed to create cluster-info: %w", err)
	}

	// 7. Deploy kube-proxy
	if err := p.deployKubeProxy(ctx); err != nil {
		return fmt.Errorf("failed to deploy kube-proxy: %w", err)
	}

	return nil
}

// ensureNamespaces creates necessary namespaces in tenant cluster
func (p *postInstallReconciler) ensureNamespaces(ctx context.Context) error {
	namespaces := []string{"kube-system", "kube-public", "kube-node-lease"}

	for _, ns := range namespaces {
		namespace := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: ns,
			},
		}
		_, err := p.tenantClient.CoreV1().Namespaces().Create(ctx, namespace, metav1.CreateOptions{})
		if err != nil && !errors.IsAlreadyExists(err) {
			return fmt.Errorf("failed to create namespace %s: %w", ns, err)
		}
	}
	return nil
}

// applyRBAC applies RBAC resources to tenant cluster
func (p *postInstallReconciler) applyRBAC(ctx context.Context) error {
	// Roles
	roles := []rbacv1.Role{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kubeadm:bootstrap-signer-clusterinfo",
				Namespace: "kube-public",
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups:     []string{""},
					Resources:     []string{"configmaps"},
					ResourceNames: []string{"cluster-info"},
					Verbs:         []string{"get"},
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kubeadm:nodes-kubeadm-config",
				Namespace: "kube-system",
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups:     []string{""},
					Resources:     []string{"configmaps"},
					ResourceNames: []string{"kubeadm-config"},
					Verbs:         []string{"get"},
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kubeadm:kubelet-config",
				Namespace: "kube-system",
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups:     []string{""},
					Resources:     []string{"configmaps"},
					ResourceNames: []string{"kubelet-config"},
					Verbs:         []string{"get"},
				},
			},
		},
	}

	for _, role := range roles {
		_, err := p.tenantClient.RbacV1().Roles(role.Namespace).Create(ctx, &role, metav1.CreateOptions{})
		if err != nil && !errors.IsAlreadyExists(err) {
			return fmt.Errorf("failed to create role %s: %w", role.Name, err)
		}
	}

	// RoleBindings
	roleBindings := []rbacv1.RoleBinding{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kubeadm:bootstrap-signer-clusterinfo",
				Namespace: "kube-public",
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "Role",
				Name:     "kubeadm:bootstrap-signer-clusterinfo",
			},
			Subjects: []rbacv1.Subject{
				{
					APIGroup: "rbac.authorization.k8s.io",
					Kind:     "User",
					Name:     "system:anonymous",
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kubeadm:nodes-kubeadm-config",
				Namespace: "kube-system",
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "Role",
				Name:     "kubeadm:nodes-kubeadm-config",
			},
			Subjects: []rbacv1.Subject{
				{
					APIGroup: "rbac.authorization.k8s.io",
					Kind:     "Group",
					Name:     "system:bootstrappers",
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kubeadm:kubelet-config",
				Namespace: "kube-system",
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "Role",
				Name:     "kubeadm:kubelet-config",
			},
			Subjects: []rbacv1.Subject{
				{
					APIGroup: "rbac.authorization.k8s.io",
					Kind:     "Group",
					Name:     "system:bootstrappers",
				},
			},
		},
	}

	for _, rb := range roleBindings {
		_, err := p.tenantClient.RbacV1().RoleBindings(rb.Namespace).Create(ctx, &rb, metav1.CreateOptions{})
		if err != nil && !errors.IsAlreadyExists(err) {
			return fmt.Errorf("failed to create rolebinding %s: %w", rb.Name, err)
		}
	}

	// ClusterRoles
	clusterRoles := []rbacv1.ClusterRole{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "kubeadm:get-nodes",
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{""},
					Resources: []string{"nodes"},
					Verbs:     []string{"get"},
				},
			},
		},
	}

	for _, cr := range clusterRoles {
		_, err := p.tenantClient.RbacV1().ClusterRoles().Create(ctx, &cr, metav1.CreateOptions{})
		if err != nil && !errors.IsAlreadyExists(err) {
			return fmt.Errorf("failed to create clusterrole %s: %w", cr.Name, err)
		}
	}

	// ClusterRoleBindings
	clusterRoleBindings := []rbacv1.ClusterRoleBinding{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "kubeadm:get-nodes",
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     "kubeadm:get-nodes",
			},
			Subjects: []rbacv1.Subject{
				{
					APIGroup: "rbac.authorization.k8s.io",
					Kind:     "Group",
					Name:     "system:bootstrappers",
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "system:node-proxier",
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     "system:node-proxier",
			},
			Subjects: []rbacv1.Subject{
				{
					Kind:      "ServiceAccount",
					Name:      "kube-proxy",
					Namespace: "kube-system",
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "kubeadm:kubelet-bootstrap",
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     "system:node-bootstrapper",
			},
			Subjects: []rbacv1.Subject{
				{
					APIGroup: "rbac.authorization.k8s.io",
					Kind:     "Group",
					Name:     "system:bootstrappers",
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "kubeadm:node-autoapprove-bootstrap",
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     "system:certificates.k8s.io:certificatesigningrequests:nodeclient",
			},
			Subjects: []rbacv1.Subject{
				{
					APIGroup: "rbac.authorization.k8s.io",
					Kind:     "Group",
					Name:     "system:bootstrappers",
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "kubeadm:node-autoapprove-certificate-rotation",
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     "system:certificates.k8s.io:certificatesigningrequests:selfnodeclient",
			},
			Subjects: []rbacv1.Subject{
				{
					APIGroup: "rbac.authorization.k8s.io",
					Kind:     "Group",
					Name:     "system:nodes",
				},
			},
		},
	}

	for _, crb := range clusterRoleBindings {
		_, err := p.tenantClient.RbacV1().ClusterRoleBindings().Create(ctx, &crb, metav1.CreateOptions{})
		if err != nil && !errors.IsAlreadyExists(err) {
			return fmt.Errorf("failed to create clusterrolebinding %s: %w", crb.Name, err)
		}
	}

	klog.Info(Format("Applied RBAC to tenant cluster %s", p.tc.Name))
	return nil
}

// createBootstrapToken creates a bootstrap token for node joining
func (p *postInstallReconciler) createBootstrapToken(ctx context.Context) error {
	// Generate token if not already set
	if p.tc.Status.BootstrapToken == "" {
		tokenID, tokenSecret, err := generateBootstrapToken()
		if err != nil {
			return fmt.Errorf("failed to generate bootstrap token: %w", err)
		}
		p.tc.Status.BootstrapToken = fmt.Sprintf("%s.%s", tokenID, tokenSecret)
		p.tc.Status.BootstrapTokenExpiration = &metav1.Time{Time: time.Now().Add(24 * time.Hour)}

		// Update the status on the host cluster
		if err := p.hostClient.Status().Update(ctx, p.tc); err != nil {
			return fmt.Errorf("failed to update TenantCluster status with bootstrap token: %w", err)
		}
	}

	// Parse token
	tokenParts := splitBootstrapToken(p.tc.Status.BootstrapToken)
	if len(tokenParts) != 2 {
		return fmt.Errorf("invalid bootstrap token format")
	}
	tokenID, tokenSecret := tokenParts[0], tokenParts[1]

	// Create bootstrap token secret
	expiration := time.Now().Add(24 * time.Hour).Format(time.RFC3339)
	if p.tc.Status.BootstrapTokenExpiration != nil {
		expiration = p.tc.Status.BootstrapTokenExpiration.Format(time.RFC3339)
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("bootstrap-token-%s", tokenID),
			Namespace: "kube-system",
		},
		Type: corev1.SecretType("bootstrap.kubernetes.io/token"),
		StringData: map[string]string{
			"token-id":                       tokenID,
			"token-secret":                   tokenSecret,
			"expiration":                     expiration,
			"usage-bootstrap-authentication": "true",
			"usage-bootstrap-signing":        "true",
		},
	}

	_, err := p.tenantClient.CoreV1().Secrets("kube-system").Create(ctx, secret, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create bootstrap token secret: %w", err)
	}

	klog.Info(Format("Created bootstrap token for tenant cluster %s", p.tc.Name))
	return nil
}

// createKubeadmConfig creates the kubeadm-config ConfigMap
func (p *postInstallReconciler) createKubeadmConfig(ctx context.Context) error {
	clusterConfig := fmt.Sprintf(`apiServer: {}
apiVersion: kubeadm.k8s.io/v1beta4
caCertificateValidityPeriod: 87600h0m0s
certificateValidityPeriod: 8760h0m0s
certificatesDir: /etc/kubernetes/pki
clusterName: kubernetes
controllerManager: {}
dns: {}
encryptionAlgorithm: RSA-2048
etcd:
  local:
    dataDir: /var/lib/etcd
imageRepository: %s
kind: ClusterConfiguration
kubernetesVersion: %s
networking:
  dnsDomain: cluster.local
  podSubnet: %s
  serviceSubnet: %s
proxy: {}
scheduler: {}
`, p.tc.GetImageRepository(), p.tc.Spec.KubernetesVersion, p.tc.GetPodCIDR(), p.tc.GetServiceClusterIPRange())

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kubeadm-config",
			Namespace: "kube-system",
		},
		Data: map[string]string{
			"ClusterConfiguration": clusterConfig,
		},
	}

	_, err := p.tenantClient.CoreV1().ConfigMaps("kube-system").Create(ctx, cm, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create kubeadm-config: %w", err)
	}

	return nil
}

// createKubeletConfig creates the kubelet-config ConfigMap
func (p *postInstallReconciler) createKubeletConfig(ctx context.Context) error {
	kubeletConfig := `apiVersion: kubelet.config.k8s.io/v1beta1
authentication:
  anonymous:
    enabled: false
  webhook:
    cacheTTL: 0s
    enabled: true
  x509:
    clientCAFile: /etc/kubernetes/pki/ca.crt
authorization:
  mode: Webhook
  webhook:
    cacheAuthorizedTTL: 0s
    cacheUnauthorizedTTL: 0s
cgroupDriver: systemd
clusterDNS:
- 10.96.0.10
clusterDomain: cluster.local
containerRuntimeEndpoint: ""
cpuManagerReconcilePeriod: 0s
crashLoopBackOff: {}
evictionPressureTransitionPeriod: 0s
fileCheckFrequency: 0s
healthzBindAddress: 127.0.0.1
healthzPort: 10248
httpCheckFrequency: 0s
imageMaximumGCAge: 0s
imageMinimumGCAge: 0s
kind: KubeletConfiguration
logging:
  flushFrequency: 0
  options:
    json:
      infoBufferSize: "0"
    text:
      infoBufferSize: "0"
  verbosity: 0
memorySwap: {}
nodeStatusReportFrequency: 0s
nodeStatusUpdateFrequency: 0s
rotateCertificates: true
runtimeRequestTimeout: 0s
shutdownGracePeriod: 0s
shutdownGracePeriodCriticalPods: 0s
staticPodPath: /etc/kubernetes/manifests
streamingConnectionIdleTimeout: 0s
syncFrequency: 0s
volumeStatsAggPeriod: 0s
`

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kubelet-config",
			Namespace: "kube-system",
		},
		Data: map[string]string{
			"kubelet": kubeletConfig,
		},
	}

	_, err := p.tenantClient.CoreV1().ConfigMaps("kube-system").Create(ctx, cm, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create kubelet-config: %w", err)
	}

	return nil
}

// createClusterInfo creates the cluster-info ConfigMap for bootstrap
func (p *postInstallReconciler) createClusterInfo(ctx context.Context) error {
	// Get CA certificate from host CA secret
	hostCASecret := &corev1.Secret{}
	if err := p.hostClient.Get(ctx, client.ObjectKey{
		Name:      "host-k8s-ca-key-pair",
		Namespace: p.tc.GetNamespace(),
	}, hostCASecret); err != nil {
		return fmt.Errorf("failed to get host CA secret: %w", err)
	}

	caCert := string(hostCASecret.Data["ca.crt"])

	kubeconfig := fmt.Sprintf(`apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: %s
    server: https://%s:6443
  name: ""
contexts: null
current-context: ""
kind: Config
preferences: {}
users: null
`, caCert, p.tc.Spec.APIServerServiceIP)

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cluster-info",
			Namespace: "kube-public",
		},
		Data: map[string]string{
			"kubeconfig": kubeconfig,
		},
	}

	_, err := p.tenantClient.CoreV1().ConfigMaps("kube-public").Create(ctx, cm, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create cluster-info: %w", err)
	}

	return nil
}

// generateBootstrapToken generates a random bootstrap token
func generateBootstrapToken() (tokenID, tokenSecret string, err error) {
	idBytes := make([]byte, 3)
	if _, err := rand.Read(idBytes); err != nil {
		return "", "", err
	}
	tokenID = hex.EncodeToString(idBytes)

	secretBytes := make([]byte, 8)
	if _, err := rand.Read(secretBytes); err != nil {
		return "", "", err
	}
	tokenSecret = hex.EncodeToString(secretBytes)

	return tokenID, tokenSecret, nil
}

// splitBootstrapToken splits the token into ID and secret
func splitBootstrapToken(token string) []string {
	var result []string
	for i, r := range token {
		if r == '.' {
			result = append(result, token[:i], token[i+1:])
			return result
		}
	}
	return result
}

// deployKubeProxy deploys kube-proxy to the tenant cluster
func (p *postInstallReconciler) deployKubeProxy(ctx context.Context) error {
	// 1. Create ServiceAccount
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kube-proxy",
			Namespace: "kube-system",
		},
	}
	_, err := p.tenantClient.CoreV1().ServiceAccounts("kube-system").Create(ctx, sa, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create kube-proxy service account: %w", err)
	}

	// 2. Create ConfigMap
	kubeProxyConfig := fmt.Sprintf(`apiVersion: kubeproxy.config.k8s.io/v1alpha1
bindAddress: 0.0.0.0
bindAddressHardFail: false
clientConnection:
  acceptContentTypes: ""
  burst: 0
  contentType: ""
  kubeconfig: /var/lib/kube-proxy/kubeconfig.conf
  qps: 0
clusterCIDR: %s
configSyncPeriod: 0s
conntrack:
  maxPerCore: null
  min: null
  tcpBeLiberal: false
  tcpCloseWaitTimeout: null
  tcpEstablishedTimeout: null
  udpStreamTimeout: 0s
  udpTimeout: 0s
detectLocal:
  bridgeInterface: ""
  interfaceNamePrefix: ""
detectLocalMode: ""
enableProfiling: false
healthzBindAddress: ""
hostnameOverride: ""
iptables:
  localhostNodePorts: null
  masqueradeAll: false
  masqueradeBit: null
  minSyncPeriod: 0s
  syncPeriod: 0s
ipvs:
  excludeCIDRs: null
  minSyncPeriod: 0s
  scheduler: ""
  strictARP: false
  syncPeriod: 0s
  tcpFinTimeout: 0s
  tcpTimeout: 0s
  udpTimeout: 0s
kind: KubeProxyConfiguration
logging:
  flushFrequency: 0
  options:
    json:
      infoBufferSize: "0"
    text:
      infoBufferSize: "0"
  verbosity: 0
metricsBindAddress: ""
mode: ""
nftables:
  masqueradeAll: false
  masqueradeBit: null
  minSyncPeriod: 0s
  syncPeriod: 0s
nodePortAddresses: null
oomScoreAdj: null
portRange: ""
showHiddenMetricsForVersion: ""
winkernel:
  enableDSR: false
  forwardHealthCheckVip: false
  networkName: ""
  rootHnsEndpointName: ""
  sourceVip: ""
`, p.tc.GetPodCIDR())

	kubeProxyKubeconfig := fmt.Sprintf(`apiVersion: v1
kind: Config
clusters:
- cluster:
    certificate-authority: /var/run/secrets/kubernetes.io/serviceaccount/ca.crt
    server: https://%s:6443
  name: default
contexts:
- context:
    cluster: default
    namespace: default
    user: default
  name: default
current-context: default
users:
- name: default
  user:
    tokenFile: /var/run/secrets/kubernetes.io/serviceaccount/token
`, p.tc.Spec.APIServerServiceIP)

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kube-proxy",
			Namespace: "kube-system",
			Labels: map[string]string{
				"app": "kube-proxy",
			},
		},
		Data: map[string]string{
			"config.conf":     kubeProxyConfig,
			"kubeconfig.conf": kubeProxyKubeconfig,
		},
	}

	_, err = p.tenantClient.CoreV1().ConfigMaps("kube-system").Create(ctx, cm, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create kube-proxy configmap: %w", err)
	}

	// 3. Create DaemonSet
	privileged := true
	hostNetwork := true
	maxUnavailable := intstr.FromInt(1)
	maxSurge := intstr.FromInt(0)

	ds := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kube-proxy",
			Namespace: "kube-system",
			Labels: map[string]string{
				"k8s-app": "kube-proxy",
			},
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"k8s-app": "kube-proxy",
				},
			},
			UpdateStrategy: appsv1.DaemonSetUpdateStrategy{
				Type: appsv1.RollingUpdateDaemonSetStrategyType,
				RollingUpdate: &appsv1.RollingUpdateDaemonSet{
					MaxUnavailable: &maxUnavailable,
					MaxSurge:       &maxSurge,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"k8s-app": "kube-proxy",
					},
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: "kube-proxy",
					HostNetwork:        hostNetwork,
					PriorityClassName:  "system-node-critical",
					NodeSelector: map[string]string{
						"kubernetes.io/os": "linux",
					},
					Tolerations: []corev1.Toleration{
						{
							Operator: corev1.TolerationOpExists,
						},
					},
					Containers: []corev1.Container{
						{
							Name:  "kube-proxy",
							Image: fmt.Sprintf("%s/kube-proxy:%s", p.tc.GetImageRepository(), p.tc.Spec.KubernetesVersion),
							Command: []string{
								"/usr/local/bin/kube-proxy",
								"--config=/var/lib/kube-proxy/config.conf",
								"--hostname-override=$(NODE_NAME)",
							},
							Env: []corev1.EnvVar{
								{
									Name: "NODE_NAME",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											FieldPath: "spec.nodeName",
										},
									},
								},
							},
							SecurityContext: &corev1.SecurityContext{
								Privileged: &privileged,
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "kube-proxy",
									MountPath: "/var/lib/kube-proxy",
								},
								{
									Name:      "xtables-lock",
									MountPath: "/run/xtables.lock",
								},
								{
									Name:      "lib-modules",
									MountPath: "/lib/modules",
									ReadOnly:  true,
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "kube-proxy",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "kube-proxy",
									},
								},
							},
						},
						{
							Name: "xtables-lock",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: "/run/xtables.lock",
									Type: hostPathTypePtr(corev1.HostPathFileOrCreate),
								},
							},
						},
						{
							Name: "lib-modules",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: "/lib/modules",
								},
							},
						},
					},
				},
			},
		},
	}

	_, err = p.tenantClient.AppsV1().DaemonSets("kube-system").Create(ctx, ds, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create kube-proxy daemonset: %w", err)
	}

	klog.Info(Format("Deployed kube-proxy to tenant cluster %s", p.tc.Name))
	return nil
}

// hostPathTypePtr returns a pointer to HostPathType
func hostPathTypePtr(t corev1.HostPathType) *corev1.HostPathType {
	return &t
}
