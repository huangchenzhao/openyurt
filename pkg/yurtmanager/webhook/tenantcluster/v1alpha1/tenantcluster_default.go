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
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/openyurtio/openyurt/pkg/apis/apps/v1alpha1"
)

// Default satisfies the defaulting webhook interface.
func (webhook *TenantClusterHandler) Default(ctx context.Context, obj runtime.Object) error {
	tc, ok := obj.(*v1alpha1.TenantCluster)
	if !ok {
		return apierrors.NewBadRequest(fmt.Sprintf("expected a TenantCluster but got a %T", obj))
	}

	// Set default etcd replicas
	if tc.Spec.EtcdReplicas == nil {
		defaultReplicas := int32(3)
		tc.Spec.EtcdReplicas = &defaultReplicas
	}

	// Set default controller-manager replicas
	if tc.Spec.ControllerManagerReplicas == nil {
		defaultReplicas := int32(2)
		tc.Spec.ControllerManagerReplicas = &defaultReplicas
	}

	// Set default scheduler replicas
	if tc.Spec.SchedulerReplicas == nil {
		defaultReplicas := int32(2)
		tc.Spec.SchedulerReplicas = &defaultReplicas
	}

	// Set default image repository
	if tc.Spec.ImageRepository == "" {
		tc.Spec.ImageRepository = "registry.aliyuncs.com/google_containers"
	}

	// Set default etcd image repository
	if tc.Spec.EtcdImageRepository == "" {
		tc.Spec.EtcdImageRepository = "quay.io/coreos"
	}

	// Set default etcd version
	if tc.Spec.EtcdVersion == "" {
		tc.Spec.EtcdVersion = "v3.6.0"
	}

	// Set default service cluster IP range
	if tc.Spec.ServiceClusterIPRange == "" {
		tc.Spec.ServiceClusterIPRange = "10.96.0.0/12"
	}

	// Set default pod CIDR
	if tc.Spec.PodCIDR == "" {
		tc.Spec.PodCIDR = "10.244.0.0/16"
	}

	// Set default etcd storage class
	if tc.Spec.EtcdStorageClass == "" {
		tc.Spec.EtcdStorageClass = "local-path"
	}

	// Set default etcd storage size
	if tc.Spec.EtcdStorageSize == "" {
		tc.Spec.EtcdStorageSize = "1Gi"
	}

	// Set default host CA secret namespace
	if tc.Spec.HostCASecretRef.Namespace == "" {
		tc.Spec.HostCASecretRef.Namespace = "kube-system"
	}

	// Initialize status if not set
	if tc.Status.Phase == "" {
		tc.Status.Phase = v1alpha1.TenantClusterPhasePending
	}

	return nil
}
