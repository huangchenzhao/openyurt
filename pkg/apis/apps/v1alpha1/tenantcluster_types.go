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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TenantClusterPhase represents the current phase of the tenant cluster
type TenantClusterPhase string

const (
	// TenantClusterPhasePending means the TenantCluster has been created but not yet processed
	TenantClusterPhasePending TenantClusterPhase = "Pending"
	// TenantClusterPhaseProvisioning means the control plane components are being deployed
	TenantClusterPhaseProvisioning TenantClusterPhase = "Provisioning"
	// TenantClusterPhaseRunning means all control plane components are ready
	TenantClusterPhaseRunning TenantClusterPhase = "Running"
	// TenantClusterPhaseFailed means the cluster failed to provision
	TenantClusterPhaseFailed TenantClusterPhase = "Failed"
	// TenantClusterPhaseDeleting means the cluster is being deleted
	TenantClusterPhaseDeleting TenantClusterPhase = "Deleting"
)

// TenantClusterConditionType represents condition types for TenantCluster
type TenantClusterConditionType string

const (
	// TenantClusterConditionPKIReady indicates PKI materials are generated
	TenantClusterConditionPKIReady TenantClusterConditionType = "PKIReady"
	// TenantClusterConditionEtcdReady indicates etcd cluster is ready
	TenantClusterConditionEtcdReady TenantClusterConditionType = "EtcdReady"
	// TenantClusterConditionAPIServerReady indicates API server is ready
	TenantClusterConditionAPIServerReady TenantClusterConditionType = "APIServerReady"
	// TenantClusterConditionSchedulerReady indicates scheduler is ready
	TenantClusterConditionSchedulerReady TenantClusterConditionType = "SchedulerReady"
	// TenantClusterConditionControllerManagerReady indicates controller-manager is ready
	TenantClusterConditionControllerManagerReady TenantClusterConditionType = "ControllerManagerReady"
	// TenantClusterConditionPostInstallReady indicates post-install configurations are applied
	TenantClusterConditionPostInstallReady TenantClusterConditionType = "PostInstallReady"
)

// SecretReference contains information to reference a Secret
type SecretReference struct {
	// Name is the name of the secret
	Name string `json:"name"`
	// Namespace is the namespace of the secret
	// +optional
	Namespace string `json:"namespace,omitempty"`
}

// TenantClusterSpec defines the desired state of TenantCluster
type TenantClusterSpec struct {
	// KubernetesVersion specifies the version of Kubernetes for the tenant cluster
	// Example: "v1.32.0"
	// +kubebuilder:validation:Pattern=`^v\d+\.\d+\.\d+$`
	KubernetesVersion string `json:"kubernetesVersion"`

	// APIServerServiceIP is the ClusterIP for the tenant APIServer service
	// Must be within the host cluster's service-cluster-ip-range
	// +kubebuilder:validation:Pattern=`^(\d{1,3}\.){3}\d{1,3}$`
	APIServerServiceIP string `json:"apiServerServiceIP"`

	// APIServerNodePort is the NodePort for external access to APIServer
	// +kubebuilder:validation:Minimum=30000
	// +kubebuilder:validation:Maximum=32767
	APIServerNodePort int32 `json:"apiServerNodePort"`

	// HostControlPlaneAddr is the address of the host cluster's API server
	// Accessible from tenant nodes for yurtadm join
	// Example: "192.168.24.180:6443"
	HostControlPlaneAddr string `json:"hostControlPlaneAddr"`

	// HostCASecretRef references a secret containing the host cluster's CA certificate and key
	// The secret should contain 'ca.crt' and 'ca.key' keys
	HostCASecretRef SecretReference `json:"hostCASecretRef"`

	// EtcdReplicas specifies the number of etcd replicas
	// +kubebuilder:default=3
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=7
	// +optional
	EtcdReplicas *int32 `json:"etcdReplicas,omitempty"`

	// ControllerManagerReplicas specifies replicas for controller-manager
	// +kubebuilder:default=2
	// +kubebuilder:validation:Minimum=1
	// +optional
	ControllerManagerReplicas *int32 `json:"controllerManagerReplicas,omitempty"`

	// SchedulerReplicas specifies replicas for scheduler
	// +kubebuilder:default=2
	// +kubebuilder:validation:Minimum=1
	// +optional
	SchedulerReplicas *int32 `json:"schedulerReplicas,omitempty"`

	// ImageRepository for kube components
	// +kubebuilder:default="registry.aliyuncs.com/google_containers"
	// +optional
	ImageRepository string `json:"imageRepository,omitempty"`

	// EtcdImageRepository for etcd image
	// +kubebuilder:default="quay.io/coreos"
	// +optional
	EtcdImageRepository string `json:"etcdImageRepository,omitempty"`

	// EtcdVersion specifies the version of etcd
	// +kubebuilder:default="v3.6.0"
	// +optional
	EtcdVersion string `json:"etcdVersion,omitempty"`

	// ServiceClusterIPRange for tenant cluster services
	// +kubebuilder:default="10.96.0.0/12"
	// +optional
	ServiceClusterIPRange string `json:"serviceClusterIPRange,omitempty"`

	// PodCIDR for tenant cluster pods
	// +kubebuilder:default="10.244.0.0/16"
	// +optional
	PodCIDR string `json:"podCIDR,omitempty"`

	// EtcdStorageClass specifies the storage class for etcd PVCs
	// +kubebuilder:default="local-path"
	// +optional
	EtcdStorageClass string `json:"etcdStorageClass,omitempty"`

	// EtcdStorageSize specifies the storage size for etcd PVCs
	// +kubebuilder:default="1Gi"
	// +optional
	EtcdStorageSize string `json:"etcdStorageSize,omitempty"`

	// Paused prevents any changes to the tenant cluster when set to true
	// +optional
	Paused bool `json:"paused,omitempty"`
}

// ComponentStatus represents the status of a control plane component
type ComponentStatus struct {
	// Ready indicates whether the component is ready
	Ready bool `json:"ready"`
	// Message provides additional information about the component status
	// +optional
	Message string `json:"message,omitempty"`
	// LastTransitionTime is the last time the status changed
	// +optional
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`
}

// ComponentStatuses tracks the status of each control plane component
type ComponentStatuses struct {
	// PKI status
	// +optional
	PKI ComponentStatus `json:"pki,omitempty"`
	// Etcd status
	// +optional
	Etcd ComponentStatus `json:"etcd,omitempty"`
	// APIServer status
	// +optional
	APIServer ComponentStatus `json:"apiServer,omitempty"`
	// Scheduler status
	// +optional
	Scheduler ComponentStatus `json:"scheduler,omitempty"`
	// ControllerManager status
	// +optional
	ControllerManager ComponentStatus `json:"controllerManager,omitempty"`
	// PostInstall status
	// +optional
	PostInstall ComponentStatus `json:"postInstall,omitempty"`
}

// TenantClusterStatus defines the observed state of TenantCluster
type TenantClusterStatus struct {
	// Phase represents the current phase of the tenant cluster
	// +optional
	Phase TenantClusterPhase `json:"phase,omitempty"`

	// Conditions represent the latest available observations
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// AdminKubeconfigSecret is the name of the secret containing admin kubeconfig
	// +optional
	AdminKubeconfigSecret string `json:"adminKubeconfigSecret,omitempty"`

	// BootstrapToken for node joining (format: token-id.token-secret)
	// +optional
	BootstrapToken string `json:"bootstrapToken,omitempty"`

	// BootstrapTokenExpiration is the expiration time of the bootstrap token
	// +optional
	BootstrapTokenExpiration *metav1.Time `json:"bootstrapTokenExpiration,omitempty"`

	// APIServerEndpoint is the internal ClusterIP endpoint for the tenant API server
	// +optional
	APIServerEndpoint string `json:"apiServerEndpoint,omitempty"`

	// ExternalAPIServerEndpoint is the external NodePort endpoint
	// +optional
	ExternalAPIServerEndpoint string `json:"externalAPIServerEndpoint,omitempty"`

	// Namespace is the namespace where tenant control plane components are deployed
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// ComponentStatuses tracks the status of each control plane component
	// +optional
	ComponentStatuses ComponentStatuses `json:"componentStatuses,omitempty"`

	// ObservedGeneration is the generation observed by the controller
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// JoinCommand provides the yurtadm join command for adding nodes
	// +optional
	JoinCommand string `json:"joinCommand,omitempty"`
}

// +genclient
// +genclient:nonNamespaced
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster,path=tenantclusters,shortName=tc,categories=yurt
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Version",type="string",JSONPath=".spec.kubernetesVersion",description="Kubernetes version"
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase",description="Cluster phase"
// +kubebuilder:printcolumn:name="APIServer",type="string",JSONPath=".status.apiServerEndpoint",description="API server endpoint"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// TenantCluster is the Schema for the tenantclusters API.
// It represents a tenant Kubernetes cluster whose control plane runs on the host cluster.
type TenantCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TenantClusterSpec   `json:"spec,omitempty"`
	Status TenantClusterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// TenantClusterList contains a list of TenantCluster
type TenantClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TenantCluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&TenantCluster{}, &TenantClusterList{})
}

// GetCondition returns the condition with the given type
func (tc *TenantCluster) GetCondition(conditionType TenantClusterConditionType) *metav1.Condition {
	for i := range tc.Status.Conditions {
		if tc.Status.Conditions[i].Type == string(conditionType) {
			return &tc.Status.Conditions[i]
		}
	}
	return nil
}

// SetCondition sets or updates the condition with the given type
func (tc *TenantCluster) SetCondition(conditionType TenantClusterConditionType, status corev1.ConditionStatus, reason, message string) {
	now := metav1.Now()
	condition := metav1.Condition{
		Type:               string(conditionType),
		Status:             metav1.ConditionStatus(status),
		LastTransitionTime: now,
		Reason:             reason,
		Message:            message,
	}

	for i := range tc.Status.Conditions {
		if tc.Status.Conditions[i].Type == string(conditionType) {
			if tc.Status.Conditions[i].Status != condition.Status {
				tc.Status.Conditions[i] = condition
			} else {
				// Only update reason and message if status hasn't changed
				tc.Status.Conditions[i].Reason = reason
				tc.Status.Conditions[i].Message = message
			}
			return
		}
	}
	tc.Status.Conditions = append(tc.Status.Conditions, condition)
}

// IsReady returns true if all conditions are True
func (tc *TenantCluster) IsReady() bool {
	requiredConditions := []TenantClusterConditionType{
		TenantClusterConditionPKIReady,
		TenantClusterConditionEtcdReady,
		TenantClusterConditionAPIServerReady,
		TenantClusterConditionSchedulerReady,
		TenantClusterConditionControllerManagerReady,
		TenantClusterConditionPostInstallReady,
	}

	for _, condType := range requiredConditions {
		cond := tc.GetCondition(condType)
		if cond == nil || cond.Status != metav1.ConditionTrue {
			return false
		}
	}
	return true
}

// GetNamespace returns the namespace for tenant control plane components
func (tc *TenantCluster) GetNamespace() string {
	return "tenant-" + tc.Name
}

// GetEtcdReplicas returns the etcd replicas with default value
func (tc *TenantCluster) GetEtcdReplicas() int32 {
	if tc.Spec.EtcdReplicas != nil {
		return *tc.Spec.EtcdReplicas
	}
	return 3
}

// GetControllerManagerReplicas returns the controller-manager replicas with default value
func (tc *TenantCluster) GetControllerManagerReplicas() int32 {
	if tc.Spec.ControllerManagerReplicas != nil {
		return *tc.Spec.ControllerManagerReplicas
	}
	return 2
}

// GetSchedulerReplicas returns the scheduler replicas with default value
func (tc *TenantCluster) GetSchedulerReplicas() int32 {
	if tc.Spec.SchedulerReplicas != nil {
		return *tc.Spec.SchedulerReplicas
	}
	return 2
}

// GetImageRepository returns the image repository with default value
func (tc *TenantCluster) GetImageRepository() string {
	if tc.Spec.ImageRepository != "" {
		return tc.Spec.ImageRepository
	}
	return "registry.aliyuncs.com/google_containers"
}

// GetEtcdImageRepository returns the etcd image repository with default value
func (tc *TenantCluster) GetEtcdImageRepository() string {
	if tc.Spec.EtcdImageRepository != "" {
		return tc.Spec.EtcdImageRepository
	}
	return "quay.io/coreos"
}

// GetEtcdVersion returns the etcd version with default value
func (tc *TenantCluster) GetEtcdVersion() string {
	if tc.Spec.EtcdVersion != "" {
		return tc.Spec.EtcdVersion
	}
	return "v3.6.0"
}

// GetServiceClusterIPRange returns the service cluster IP range with default value
func (tc *TenantCluster) GetServiceClusterIPRange() string {
	if tc.Spec.ServiceClusterIPRange != "" {
		return tc.Spec.ServiceClusterIPRange
	}
	return "10.96.0.0/12"
}

// GetPodCIDR returns the pod CIDR with default value
func (tc *TenantCluster) GetPodCIDR() string {
	if tc.Spec.PodCIDR != "" {
		return tc.Spec.PodCIDR
	}
	return "10.244.0.0/16"
}

// GetEtcdStorageClass returns the etcd storage class with default value
func (tc *TenantCluster) GetEtcdStorageClass() string {
	if tc.Spec.EtcdStorageClass != "" {
		return tc.Spec.EtcdStorageClass
	}
	return "local-path"
}

// GetEtcdStorageSize returns the etcd storage size with default value
func (tc *TenantCluster) GetEtcdStorageSize() string {
	if tc.Spec.EtcdStorageSize != "" {
		return tc.Spec.EtcdStorageSize
	}
	return "1Gi"
}
