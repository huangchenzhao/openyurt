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
	"net"
	"regexp"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/openyurtio/openyurt/pkg/apis/apps/v1alpha1"
)

var (
	// Regex pattern for K8s version format: v1.x.y
	k8sVersionPattern = regexp.MustCompile(`^v\d+\.\d+\.\d+$`)
	// Regex pattern for IP address
	ipPattern = regexp.MustCompile(`^(\d{1,3}\.){3}\d{1,3}$`)
)

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type.
func (webhook *TenantClusterHandler) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	tc, ok := obj.(*v1alpha1.TenantCluster)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected a TenantCluster but got a %T", obj))
	}

	if allErrs := validateTenantClusterSpec(ctx, webhook.Client, &tc.Spec, tc.Name, nil); len(allErrs) > 0 {
		return nil, apierrors.NewInvalid(v1alpha1.SchemeGroupVersion.WithKind("TenantCluster").GroupKind(), tc.Name, allErrs)
	}

	return nil, nil
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type.
func (webhook *TenantClusterHandler) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	newTC, ok := newObj.(*v1alpha1.TenantCluster)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected a TenantCluster but got a %T", newObj))
	}
	oldTC, ok := oldObj.(*v1alpha1.TenantCluster)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected a TenantCluster but got a %T", oldObj))
	}

	if allErrs := validateTenantClusterSpec(ctx, webhook.Client, &newTC.Spec, newTC.Name, &oldTC.Spec); len(allErrs) > 0 {
		return nil, apierrors.NewInvalid(v1alpha1.SchemeGroupVersion.WithKind("TenantCluster").GroupKind(), newTC.Name, allErrs)
	}

	// Validate immutable fields
	if allErrs := validateImmutableFields(&newTC.Spec, &oldTC.Spec); len(allErrs) > 0 {
		return nil, apierrors.NewForbidden(
			v1alpha1.SchemeGroupVersion.WithResource("tenantclusters").GroupResource(),
			newTC.Name,
			allErrs[0],
		)
	}

	return nil, nil
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type.
func (webhook *TenantClusterHandler) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	tc, ok := obj.(*v1alpha1.TenantCluster)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected a TenantCluster but got a %T", obj))
	}

	// Check if the tenant cluster is in a safe state to delete
	if tc.Status.Phase == v1alpha1.TenantClusterPhaseProvisioning {
		return admission.Warnings{"TenantCluster is still provisioning, deletion may leave orphaned resources"}, nil
	}

	return nil, nil
}

// validateTenantClusterSpec validates the TenantCluster spec
func validateTenantClusterSpec(ctx context.Context, cli client.Client, spec *v1alpha1.TenantClusterSpec, name string, oldSpec *v1alpha1.TenantClusterSpec) field.ErrorList {
	var allErrs field.ErrorList

	// Validate Kubernetes version format
	if !k8sVersionPattern.MatchString(spec.KubernetesVersion) {
		allErrs = append(allErrs, field.Invalid(
			field.NewPath("spec").Child("kubernetesVersion"),
			spec.KubernetesVersion,
			"must be in format v1.x.y (e.g., v1.32.0)",
		))
	}

	// Validate APIServer Service IP
	if !ipPattern.MatchString(spec.APIServerServiceIP) {
		allErrs = append(allErrs, field.Invalid(
			field.NewPath("spec").Child("apiServerServiceIP"),
			spec.APIServerServiceIP,
			"must be a valid IPv4 address",
		))
	} else {
		ip := net.ParseIP(spec.APIServerServiceIP)
		if ip == nil {
			allErrs = append(allErrs, field.Invalid(
				field.NewPath("spec").Child("apiServerServiceIP"),
				spec.APIServerServiceIP,
				"invalid IP address",
			))
		}
	}

	// Validate APIServer NodePort range
	if spec.APIServerNodePort < 30000 || spec.APIServerNodePort > 32767 {
		allErrs = append(allErrs, field.Invalid(
			field.NewPath("spec").Child("apiServerNodePort"),
			spec.APIServerNodePort,
			"must be in range 30000-32767",
		))
	}

	// Validate HostControlPlaneAddr format
	if spec.HostControlPlaneAddr == "" {
		allErrs = append(allErrs, field.Required(
			field.NewPath("spec").Child("hostControlPlaneAddr"),
			"hostControlPlaneAddr is required",
		))
	}

	// Validate HostCASecretRef
	if spec.HostCASecretRef.Name == "" {
		allErrs = append(allErrs, field.Required(
			field.NewPath("spec").Child("hostCASecretRef").Child("name"),
			"hostCASecretRef.name is required",
		))
	}

	// Validate the host CA secret exists
	if cli != nil && spec.HostCASecretRef.Name != "" {
		namespace := spec.HostCASecretRef.Namespace
		if namespace == "" {
			namespace = "kube-system"
		}
		secret := &corev1.Secret{}
		if err := cli.Get(ctx, client.ObjectKey{Name: spec.HostCASecretRef.Name, Namespace: namespace}, secret); err != nil {
			if apierrors.IsNotFound(err) {
				allErrs = append(allErrs, field.NotFound(
					field.NewPath("spec").Child("hostCASecretRef"),
					fmt.Sprintf("%s/%s", namespace, spec.HostCASecretRef.Name),
				))
			}
		} else {
			// Validate secret has required keys
			if _, ok := secret.Data["ca.crt"]; !ok {
				allErrs = append(allErrs, field.Invalid(
					field.NewPath("spec").Child("hostCASecretRef"),
					spec.HostCASecretRef.Name,
					"secret must contain 'ca.crt' key",
				))
			}
			if _, ok := secret.Data["ca.key"]; !ok {
				allErrs = append(allErrs, field.Invalid(
					field.NewPath("spec").Child("hostCASecretRef"),
					spec.HostCASecretRef.Name,
					"secret must contain 'ca.key' key",
				))
			}
		}
	}

	// Validate etcd replicas (should be odd for quorum)
	if spec.EtcdReplicas != nil {
		replicas := *spec.EtcdReplicas
		if replicas < 1 || replicas > 7 {
			allErrs = append(allErrs, field.Invalid(
				field.NewPath("spec").Child("etcdReplicas"),
				replicas,
				"must be between 1 and 7",
			))
		}
		if replicas%2 == 0 {
			allErrs = append(allErrs, field.Invalid(
				field.NewPath("spec").Child("etcdReplicas"),
				replicas,
				"should be an odd number for proper quorum (1, 3, 5, or 7)",
			))
		}
	}

	return allErrs
}

// validateImmutableFields validates that immutable fields are not changed
func validateImmutableFields(newSpec, oldSpec *v1alpha1.TenantClusterSpec) field.ErrorList {
	var allErrs field.ErrorList

	// APIServerServiceIP is immutable
	if newSpec.APIServerServiceIP != oldSpec.APIServerServiceIP {
		allErrs = append(allErrs, field.Forbidden(
			field.NewPath("spec").Child("apiServerServiceIP"),
			"apiServerServiceIP is immutable after creation",
		))
	}

	// APIServerNodePort is immutable
	if newSpec.APIServerNodePort != oldSpec.APIServerNodePort {
		allErrs = append(allErrs, field.Forbidden(
			field.NewPath("spec").Child("apiServerNodePort"),
			"apiServerNodePort is immutable after creation",
		))
	}

	// HostCASecretRef is immutable
	if newSpec.HostCASecretRef.Name != oldSpec.HostCASecretRef.Name ||
		newSpec.HostCASecretRef.Namespace != oldSpec.HostCASecretRef.Namespace {
		allErrs = append(allErrs, field.Forbidden(
			field.NewPath("spec").Child("hostCASecretRef"),
			"hostCASecretRef is immutable after creation",
		))
	}

	return allErrs
}
