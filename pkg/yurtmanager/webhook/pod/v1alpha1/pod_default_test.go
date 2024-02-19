/*
Copyright 2024 The OpenYurt Authors.

Licensed under the Apache License, Version 2.0 (the License);
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an AS IS BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	"context"
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/openyurtio/openyurt/pkg/apis/apps"
)

func TestDefault(t *testing.T) {
	testcases := map[string]struct {
		obj         runtime.Object
		errHappened bool
		wantedPod   *corev1.Pod
	}{
		"it is not a pod": {
			obj:         &corev1.Node{},
			errHappened: true,
		},
		"pod with annotation['apps.openyurt.io/exclude-host-network-pool'] = true": {
			obj: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod0",
					Namespace: metav1.NamespaceDefault,
					Annotations: map[string]string{
						apps.AnnotationExcludeHostNetworkPool: "true",
					},
				},
			},
			wantedPod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod0",
					Namespace: metav1.NamespaceDefault,
					Annotations: map[string]string{
						apps.AnnotationExcludeHostNetworkPool: "true",
					},
				},
				Spec: corev1.PodSpec{
					Affinity: &corev1.Affinity{
						NodeAffinity: &corev1.NodeAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
								NodeSelectorTerms: []corev1.NodeSelectorTerm{
									{
										MatchExpressions: []corev1.NodeSelectorRequirement{
											{
												Key:      "nodepool.openyurt.io/hostnetwork",
												Operator: corev1.NodeSelectorOpNotIn,
												Values:   []string{"true"},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		"pod without annotation['apps.openyurt.io/exclude-host-network-pool'] = true": {
			obj: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod1",
					Namespace: metav1.NamespaceDefault,
				},
			},
			wantedPod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod1",
					Namespace: metav1.NamespaceDefault,
				},
			},
		},
	}

	for k, tc := range testcases {
		t.Run(k, func(t *testing.T) {
			h := PodHandler{}
			err := h.Default(context.TODO(), tc.obj, admission.Request{})
			if tc.errHappened {
				if err == nil {
					t.Errorf("expect error, got nil")
				}
			} else if err != nil {
				t.Errorf("expect no error, but got %v", err)
			} else {
				currentPod := tc.obj.(*corev1.Pod)
				if !reflect.DeepEqual(currentPod, tc.wantedPod) {
					t.Errorf("expect %#+v, got %#+v", tc.wantedPod, currentPod)
				}
			}
		})
	}
}
