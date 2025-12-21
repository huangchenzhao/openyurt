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
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/openyurtio/openyurt/pkg/apis/apps/v1alpha1"
)

const (
	testTimeout  = 5 * time.Minute
	testInterval = 5 * time.Second
)

var _ = Describe("TenantCluster Controller E2E", func() {
	var (
		ctx        context.Context
		k8sClient  client.Client
		testName   string
		hostCAName string
	)

	BeforeEach(func() {
		ctx = context.Background()
		testName = fmt.Sprintf("test-tenant-%d", time.Now().Unix())
		hostCAName = "host-k8s-ca"

		// Skip if client is not available
		if k8sClient == nil {
			Skip("k8sClient is not set up - run with a real cluster")
		}
	})

	AfterEach(func() {
		// Cleanup
		if k8sClient != nil {
			tc := &appsv1alpha1.TenantCluster{}
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: testName}, tc); err == nil {
				_ = k8sClient.Delete(ctx, tc)
			}
		}
	})

	Context("TenantCluster Lifecycle", func() {
		It("should create a TenantCluster successfully", func() {
			By("Creating a TenantCluster")
			tc := &appsv1alpha1.TenantCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: testName,
				},
				Spec: appsv1alpha1.TenantClusterSpec{
					KubernetesVersion:    "v1.32.0",
					APIServerServiceIP:   "10.96.200.1",
					APIServerNodePort:    31100,
					HostControlPlaneAddr: "192.168.24.180:6443",
					HostCASecretRef: appsv1alpha1.SecretReference{
						Name:      hostCAName,
						Namespace: "kube-system",
					},
				},
			}

			Expect(k8sClient.Create(ctx, tc)).Should(Succeed())

			By("Waiting for the namespace to be created")
			ns := &corev1.Namespace{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: tc.GetNamespace()}, ns)
				return err == nil
			}, testTimeout, testInterval).Should(BeTrue())

			By("Waiting for PKI to be ready")
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: testName}, tc)
				if err != nil {
					return false
				}
				return tc.Status.ComponentStatuses.PKI.Ready
			}, testTimeout, testInterval).Should(BeTrue())

			By("Waiting for etcd to be ready")
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: testName}, tc)
				if err != nil {
					return false
				}
				return tc.Status.ComponentStatuses.Etcd.Ready
			}, testTimeout, testInterval).Should(BeTrue())

			By("Waiting for APIServer to be ready")
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: testName}, tc)
				if err != nil {
					return false
				}
				return tc.Status.ComponentStatuses.APIServer.Ready
			}, testTimeout, testInterval).Should(BeTrue())

			By("Waiting for phase to become Running")
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: testName}, tc)
				if err != nil {
					return false
				}
				return tc.Status.Phase == appsv1alpha1.TenantClusterPhaseRunning
			}, testTimeout, testInterval).Should(BeTrue())

			By("Verifying admin kubeconfig secret exists")
			secret := &corev1.Secret{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      "tenant-admin-kubeconfig",
				Namespace: tc.GetNamespace(),
			}, secret)).Should(Succeed())

			By("Verifying bootstrap token is set")
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: testName}, tc)).Should(Succeed())
			Expect(tc.Status.BootstrapToken).ShouldNot(BeEmpty())
			Expect(tc.Status.JoinCommand).ShouldNot(BeEmpty())
		})

		It("should delete a TenantCluster and cleanup resources", func() {
			By("Creating a TenantCluster")
			tc := &appsv1alpha1.TenantCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: testName,
				},
				Spec: appsv1alpha1.TenantClusterSpec{
					KubernetesVersion:    "v1.32.0",
					APIServerServiceIP:   "10.96.200.2",
					APIServerNodePort:    31101,
					HostControlPlaneAddr: "192.168.24.180:6443",
					HostCASecretRef: appsv1alpha1.SecretReference{
						Name:      hostCAName,
						Namespace: "kube-system",
					},
				},
			}

			Expect(k8sClient.Create(ctx, tc)).Should(Succeed())

			By("Waiting for the TenantCluster to be provisioning")
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: testName}, tc)
				if err != nil {
					return false
				}
				return tc.Status.Phase == appsv1alpha1.TenantClusterPhaseProvisioning ||
					tc.Status.Phase == appsv1alpha1.TenantClusterPhaseRunning
			}, testTimeout, testInterval).Should(BeTrue())

			By("Deleting the TenantCluster")
			Expect(k8sClient.Delete(ctx, tc)).Should(Succeed())

			By("Waiting for namespace to be deleted")
			ns := &corev1.Namespace{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: tc.GetNamespace()}, ns)
				return apierrors.IsNotFound(err)
			}, testTimeout, testInterval).Should(BeTrue())
		})

		It("should respect paused spec", func() {
			By("Creating a paused TenantCluster")
			tc := &appsv1alpha1.TenantCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: testName,
				},
				Spec: appsv1alpha1.TenantClusterSpec{
					KubernetesVersion:    "v1.32.0",
					APIServerServiceIP:   "10.96.200.3",
					APIServerNodePort:    31102,
					HostControlPlaneAddr: "192.168.24.180:6443",
					HostCASecretRef: appsv1alpha1.SecretReference{
						Name:      hostCAName,
						Namespace: "kube-system",
					},
					Paused: true,
				},
			}

			Expect(k8sClient.Create(ctx, tc)).Should(Succeed())

			By("Checking that no namespace is created")
			time.Sleep(10 * time.Second)
			ns := &corev1.Namespace{}
			err := k8sClient.Get(ctx, types.NamespacedName{Name: tc.GetNamespace()}, ns)
			Expect(apierrors.IsNotFound(err)).Should(BeTrue())

			By("Unpausing the TenantCluster")
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: testName}, tc)).Should(Succeed())
			tc.Spec.Paused = false
			Expect(k8sClient.Update(ctx, tc)).Should(Succeed())

			By("Waiting for namespace to be created after unpausing")
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: tc.GetNamespace()}, ns)
				return err == nil
			}, testTimeout, testInterval).Should(BeTrue())
		})
	})

	Context("TenantCluster Validation", func() {
		It("should reject invalid API server service IP", func() {
			tc := &appsv1alpha1.TenantCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: testName,
				},
				Spec: appsv1alpha1.TenantClusterSpec{
					KubernetesVersion:    "v1.32.0",
					APIServerServiceIP:   "invalid-ip",
					APIServerNodePort:    31103,
					HostControlPlaneAddr: "192.168.24.180:6443",
					HostCASecretRef: appsv1alpha1.SecretReference{
						Name:      hostCAName,
						Namespace: "kube-system",
					},
				},
			}

			err := k8sClient.Create(ctx, tc)
			Expect(err).Should(HaveOccurred())
			Expect(err.Error()).Should(ContainSubstring("apiServerServiceIP"))
		})

		It("should reject invalid NodePort", func() {
			tc := &appsv1alpha1.TenantCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: testName,
				},
				Spec: appsv1alpha1.TenantClusterSpec{
					KubernetesVersion:    "v1.32.0",
					APIServerServiceIP:   "10.96.200.4",
					APIServerNodePort:    29999, // Invalid - too low
					HostControlPlaneAddr: "192.168.24.180:6443",
					HostCASecretRef: appsv1alpha1.SecretReference{
						Name:      hostCAName,
						Namespace: "kube-system",
					},
				},
			}

			err := k8sClient.Create(ctx, tc)
			Expect(err).Should(HaveOccurred())
			Expect(err.Error()).Should(ContainSubstring("apiServerNodePort"))
		})

		It("should reject changes to immutable fields", func() {
			By("Creating a TenantCluster")
			tc := &appsv1alpha1.TenantCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: testName,
				},
				Spec: appsv1alpha1.TenantClusterSpec{
					KubernetesVersion:    "v1.32.0",
					APIServerServiceIP:   "10.96.200.5",
					APIServerNodePort:    31104,
					HostControlPlaneAddr: "192.168.24.180:6443",
					HostCASecretRef: appsv1alpha1.SecretReference{
						Name:      hostCAName,
						Namespace: "kube-system",
					},
				},
			}

			Expect(k8sClient.Create(ctx, tc)).Should(Succeed())

			By("Trying to change an immutable field")
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: testName}, tc)).Should(Succeed())
			tc.Spec.APIServerServiceIP = "10.96.200.100"

			err := k8sClient.Update(ctx, tc)
			Expect(err).Should(HaveOccurred())
			Expect(err.Error()).Should(ContainSubstring("immutable"))
		})
	})
})
