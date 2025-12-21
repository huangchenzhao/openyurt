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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	yurtClient "github.com/openyurtio/openyurt/cmd/yurt-manager/app/client"
	"github.com/openyurtio/openyurt/cmd/yurt-manager/app/config"
	"github.com/openyurtio/openyurt/cmd/yurt-manager/names"
	appsv1alpha1 "github.com/openyurtio/openyurt/pkg/apis/apps/v1alpha1"
	tenantclusterconfig "github.com/openyurtio/openyurt/pkg/yurtmanager/controller/tenantcluster/config"
)

var (
	controllerResource = appsv1alpha1.SchemeGroupVersion.WithResource("tenantclusters")
)

const (
	// RequeueAfterDefault is the default requeue interval
	RequeueAfterDefault = 30 * time.Second
	// RequeueAfterShort is a short requeue interval for faster reconciliation
	RequeueAfterShort = 10 * time.Second
)

// Format formats the log message with controller name prefix
func Format(format string, args ...interface{}) string {
	s := fmt.Sprintf(format, args...)
	return fmt.Sprintf("%s: %s", names.TenantClusterController, s)
}

// ReconcileTenantCluster reconciles a TenantCluster object
type ReconcileTenantCluster struct {
	client.Client
	recorder record.EventRecorder
	cfg      tenantclusterconfig.TenantClusterControllerConfiguration
}

var _ reconcile.Reconciler = &ReconcileTenantCluster{}

// Add creates a new TenantCluster Controller and adds it to the Manager with default RBAC.
// The Manager will set fields on the Controller and Start it when the Manager is Started.
func Add(ctx context.Context, c *config.CompletedConfig, mgr manager.Manager) error {
	klog.Infof("tenantcluster-controller add controller %s", controllerResource.String())
	r := &ReconcileTenantCluster{
		cfg:      c.ComponentConfig.TenantClusterController,
		recorder: mgr.GetEventRecorderFor(names.TenantClusterController),
		Client:   yurtClient.GetClientByControllerNameOrDie(mgr, names.TenantClusterController),
	}

	// Create a new controller
	ctrl, err := controller.New(names.TenantClusterController, mgr, controller.Options{
		Reconciler:              r,
		MaxConcurrentReconciles: int(c.ComponentConfig.TenantClusterController.ConcurrentTenantClusterWorkers),
	})
	if err != nil {
		return err
	}

	if _, err := mgr.GetRESTMapper().KindFor(controllerResource); err != nil {
		klog.Infof("resource %s doesn't exist", controllerResource.String())
		return err
	}

	// Watch for changes to TenantCluster
	err = ctrl.Watch(
		source.Kind[client.Object](mgr.GetCache(), &appsv1alpha1.TenantCluster{}, &handler.EnqueueRequestForObject{}),
	)
	if err != nil {
		return err
	}

	// Watch for StatefulSet changes (etcd)
	err = ctrl.Watch(source.Kind[client.Object](mgr.GetCache(), &appsv1.StatefulSet{},
		handler.EnqueueRequestForOwner(mgr.GetScheme(), mgr.GetRESTMapper(), &appsv1alpha1.TenantCluster{}, handler.OnlyControllerOwner())))
	if err != nil {
		return err
	}

	// Watch for DaemonSet changes (apiserver)
	err = ctrl.Watch(source.Kind[client.Object](mgr.GetCache(), &appsv1.DaemonSet{},
		handler.EnqueueRequestForOwner(mgr.GetScheme(), mgr.GetRESTMapper(), &appsv1alpha1.TenantCluster{}, handler.OnlyControllerOwner())))
	if err != nil {
		return err
	}

	// Watch for Deployment changes (scheduler, controller-manager)
	err = ctrl.Watch(source.Kind[client.Object](mgr.GetCache(), &appsv1.Deployment{},
		handler.EnqueueRequestForOwner(mgr.GetScheme(), mgr.GetRESTMapper(), &appsv1alpha1.TenantCluster{}, handler.OnlyControllerOwner())))
	if err != nil {
		return err
	}

	return nil
}

// +kubebuilder:rbac:groups=apps.openyurt.io,resources=tenantclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps.openyurt.io,resources=tenantclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps.openyurt.io,resources=tenantclusters/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=namespaces,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=daemonsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete

// Reconcile reads that state of the cluster for a TenantCluster object and makes changes based on the state read
// and what is in the TenantCluster.Spec
func (r *ReconcileTenantCluster) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	klog.Info(Format("Reconcile TenantCluster %s", req.Name))

	// Fetch the TenantCluster instance
	tc := &appsv1alpha1.TenantCluster{}
	if err := r.Get(ctx, req.NamespacedName, tc); err != nil {
		if errors.IsNotFound(err) {
			klog.Info(Format("TenantCluster %s not found, may have been deleted", req.Name))
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Handle deletion
	if !tc.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, tc)
	}

	// Skip if paused
	if tc.Spec.Paused {
		klog.Info(Format("TenantCluster %s is paused, skipping reconciliation", tc.Name))
		return ctrl.Result{}, nil
	}

	// Update observed generation
	if tc.Status.ObservedGeneration != tc.Generation {
		tc.Status.ObservedGeneration = tc.Generation
	}

	// Set initial phase if not set
	if tc.Status.Phase == "" {
		tc.Status.Phase = appsv1alpha1.TenantClusterPhasePending
	}

	// Main reconciliation logic
	result, err := r.reconcile(ctx, tc)
	if err != nil {
		r.recorder.Eventf(tc, corev1.EventTypeWarning, "ReconcileFailed", "Failed to reconcile: %v", err)
		tc.Status.Phase = appsv1alpha1.TenantClusterPhaseFailed
	}

	// Update status
	if statusErr := r.Status().Update(ctx, tc); statusErr != nil {
		klog.Error(Format("Failed to update TenantCluster %s status: %v", tc.Name, statusErr))
		return ctrl.Result{}, statusErr
	}

	return result, err
}

func (r *ReconcileTenantCluster) reconcile(ctx context.Context, tc *appsv1alpha1.TenantCluster) (reconcile.Result, error) {
	tc.Status.Phase = appsv1alpha1.TenantClusterPhaseProvisioning
	tc.Status.Namespace = tc.GetNamespace()

	// Step 1: Ensure namespace exists
	if err := r.ensureNamespace(ctx, tc); err != nil {
		klog.Error(Format("Failed to ensure namespace for TenantCluster %s: %v", tc.Name, err))
		return ctrl.Result{}, err
	}

	// Step 2: Reconcile PKI
	if err := r.reconcilePKI(ctx, tc); err != nil {
		klog.Error(Format("Failed to reconcile PKI for TenantCluster %s: %v", tc.Name, err))
		return ctrl.Result{RequeueAfter: RequeueAfterShort}, err
	}
	if !tc.Status.ComponentStatuses.PKI.Ready {
		klog.Info(Format("PKI not ready for TenantCluster %s, requeuing", tc.Name))
		return ctrl.Result{RequeueAfter: RequeueAfterShort}, nil
	}

	// Step 3: Reconcile etcd
	if err := r.reconcileEtcd(ctx, tc); err != nil {
		klog.Error(Format("Failed to reconcile etcd for TenantCluster %s: %v", tc.Name, err))
		return ctrl.Result{RequeueAfter: RequeueAfterShort}, err
	}
	if !tc.Status.ComponentStatuses.Etcd.Ready {
		klog.Info(Format("Etcd not ready for TenantCluster %s, requeuing", tc.Name))
		return ctrl.Result{RequeueAfter: RequeueAfterShort}, nil
	}

	// Step 4: Reconcile APIServer
	if err := r.reconcileAPIServer(ctx, tc); err != nil {
		klog.Error(Format("Failed to reconcile APIServer for TenantCluster %s: %v", tc.Name, err))
		return ctrl.Result{RequeueAfter: RequeueAfterShort}, err
	}
	if !tc.Status.ComponentStatuses.APIServer.Ready {
		klog.Info(Format("APIServer not ready for TenantCluster %s, requeuing", tc.Name))
		return ctrl.Result{RequeueAfter: RequeueAfterShort}, nil
	}

	// Step 5: Reconcile Scheduler
	if err := r.reconcileScheduler(ctx, tc); err != nil {
		klog.Error(Format("Failed to reconcile Scheduler for TenantCluster %s: %v", tc.Name, err))
		return ctrl.Result{RequeueAfter: RequeueAfterShort}, err
	}

	// Step 6: Reconcile Controller Manager
	if err := r.reconcileControllerManager(ctx, tc); err != nil {
		klog.Error(Format("Failed to reconcile ControllerManager for TenantCluster %s: %v", tc.Name, err))
		return ctrl.Result{RequeueAfter: RequeueAfterShort}, err
	}

	// Step 7: Check if all control plane components are ready
	if !tc.Status.ComponentStatuses.Scheduler.Ready || !tc.Status.ComponentStatuses.ControllerManager.Ready {
		klog.Info(Format("Control plane not fully ready for TenantCluster %s, requeuing", tc.Name))
		return ctrl.Result{RequeueAfter: RequeueAfterShort}, nil
	}

	// Step 8: Reconcile Post-Install configurations
	if err := r.reconcilePostInstall(ctx, tc); err != nil {
		klog.Error(Format("Failed to reconcile PostInstall for TenantCluster %s: %v", tc.Name, err))
		return ctrl.Result{RequeueAfter: RequeueAfterShort}, err
	}

	// All components are ready
	if tc.IsReady() {
		tc.Status.Phase = appsv1alpha1.TenantClusterPhaseRunning
		r.recorder.Event(tc, corev1.EventTypeNormal, "Ready", "TenantCluster is ready")
	}

	return ctrl.Result{RequeueAfter: RequeueAfterDefault}, nil
}

func (r *ReconcileTenantCluster) reconcileDelete(ctx context.Context, tc *appsv1alpha1.TenantCluster) (reconcile.Result, error) {
	klog.Info(Format("Reconciling deletion for TenantCluster %s", tc.Name))
	tc.Status.Phase = appsv1alpha1.TenantClusterPhaseDeleting

	// Update status first
	if err := r.Status().Update(ctx, tc); err != nil {
		return ctrl.Result{}, err
	}

	// Clean up namespace (this will cascade delete all resources)
	namespace := &corev1.Namespace{}
	if err := r.Get(ctx, client.ObjectKey{Name: tc.GetNamespace()}, namespace); err != nil {
		if errors.IsNotFound(err) {
			klog.Info(Format("Namespace %s already deleted for TenantCluster %s", tc.GetNamespace(), tc.Name))
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if namespace.DeletionTimestamp.IsZero() {
		if err := r.Delete(ctx, namespace); err != nil {
			klog.Error(Format("Failed to delete namespace %s for TenantCluster %s: %v", tc.GetNamespace(), tc.Name, err))
			return ctrl.Result{}, err
		}
		klog.Info(Format("Deleted namespace %s for TenantCluster %s", tc.GetNamespace(), tc.Name))
	}

	return ctrl.Result{RequeueAfter: RequeueAfterShort}, nil
}

func (r *ReconcileTenantCluster) ensureNamespace(ctx context.Context, tc *appsv1alpha1.TenantCluster) error {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: tc.GetNamespace(),
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "tenantcluster-controller",
				"tenantcluster.openyurt.io":    tc.Name,
			},
		},
	}

	existing := &corev1.Namespace{}
	if err := r.Get(ctx, client.ObjectKey{Name: ns.Name}, existing); err != nil {
		if errors.IsNotFound(err) {
			klog.Info(Format("Creating namespace %s for TenantCluster %s", ns.Name, tc.Name))
			if err := r.Create(ctx, ns); err != nil {
				return fmt.Errorf("failed to create namespace: %w", err)
			}
			r.recorder.Eventf(tc, corev1.EventTypeNormal, "NamespaceCreated", "Created namespace %s", ns.Name)
			return nil
		}
		return err
	}

	klog.V(4).Info(Format("Namespace %s already exists for TenantCluster %s", ns.Name, tc.Name))
	return nil
}

// reconcilePKI generates and manages PKI materials for the tenant cluster
func (r *ReconcileTenantCluster) reconcilePKI(ctx context.Context, tc *appsv1alpha1.TenantCluster) error {
	// This will be implemented to generate certificates
	// For now, mark as ready if all required secrets exist
	requiredSecrets := []string{
		"tenant-etcd-server-tls",
		"tenant-apiserver-etcd-client-certs",
		"tenant-front-proxy-ca",
		"tenant-sa-keys",
		"tenant-apiserver-certs",
		"tenant-apiserver-kubelet-client-certs",
		"tenant-front-proxy-client-certs",
		"tenant-kcm-kubeconfig",
		"tenant-scheduler-kubeconfig",
		"tenant-admin-kubeconfig",
	}

	allExist := true
	for _, secretName := range requiredSecrets {
		secret := &corev1.Secret{}
		if err := r.Get(ctx, client.ObjectKey{Name: secretName, Namespace: tc.GetNamespace()}, secret); err != nil {
			if errors.IsNotFound(err) {
				allExist = false
				break
			}
			return err
		}
	}

	if !allExist {
		// Generate PKI materials
		if err := r.generatePKI(ctx, tc); err != nil {
			tc.Status.ComponentStatuses.PKI = appsv1alpha1.ComponentStatus{
				Ready:              false,
				Message:            fmt.Sprintf("Failed to generate PKI: %v", err),
				LastTransitionTime: metav1.Now(),
			}
			tc.SetCondition(appsv1alpha1.TenantClusterConditionPKIReady, corev1.ConditionFalse, "PKIGenerationFailed", err.Error())
			return err
		}
	}

	tc.Status.ComponentStatuses.PKI = appsv1alpha1.ComponentStatus{
		Ready:              true,
		Message:            "PKI materials are ready",
		LastTransitionTime: metav1.Now(),
	}
	tc.SetCondition(appsv1alpha1.TenantClusterConditionPKIReady, corev1.ConditionTrue, "PKIReady", "All PKI materials are generated")
	tc.Status.AdminKubeconfigSecret = "tenant-admin-kubeconfig"

	return nil
}

// reconcileEtcd manages the etcd StatefulSet
func (r *ReconcileTenantCluster) reconcileEtcd(ctx context.Context, tc *appsv1alpha1.TenantCluster) error {
	// Check if etcd StatefulSet exists and is ready
	sts := &appsv1.StatefulSet{}
	if err := r.Get(ctx, client.ObjectKey{Name: "etcd", Namespace: tc.GetNamespace()}, sts); err != nil {
		if errors.IsNotFound(err) {
			// Create etcd resources
			if err := r.createEtcdResources(ctx, tc); err != nil {
				tc.Status.ComponentStatuses.Etcd = appsv1alpha1.ComponentStatus{
					Ready:              false,
					Message:            fmt.Sprintf("Failed to create etcd: %v", err),
					LastTransitionTime: metav1.Now(),
				}
				tc.SetCondition(appsv1alpha1.TenantClusterConditionEtcdReady, corev1.ConditionFalse, "EtcdCreationFailed", err.Error())
				return err
			}
			tc.Status.ComponentStatuses.Etcd = appsv1alpha1.ComponentStatus{
				Ready:              false,
				Message:            "etcd is being provisioned",
				LastTransitionTime: metav1.Now(),
			}
			tc.SetCondition(appsv1alpha1.TenantClusterConditionEtcdReady, corev1.ConditionFalse, "EtcdProvisioning", "etcd StatefulSet is being created")
			return nil
		}
		return err
	}

	// Check if etcd is ready
	if sts.Status.ReadyReplicas == *sts.Spec.Replicas {
		tc.Status.ComponentStatuses.Etcd = appsv1alpha1.ComponentStatus{
			Ready:              true,
			Message:            fmt.Sprintf("etcd is ready with %d replicas", sts.Status.ReadyReplicas),
			LastTransitionTime: metav1.Now(),
		}
		tc.SetCondition(appsv1alpha1.TenantClusterConditionEtcdReady, corev1.ConditionTrue, "EtcdReady", "etcd cluster is healthy")
	} else {
		tc.Status.ComponentStatuses.Etcd = appsv1alpha1.ComponentStatus{
			Ready:              false,
			Message:            fmt.Sprintf("etcd has %d/%d ready replicas", sts.Status.ReadyReplicas, *sts.Spec.Replicas),
			LastTransitionTime: metav1.Now(),
		}
		tc.SetCondition(appsv1alpha1.TenantClusterConditionEtcdReady, corev1.ConditionFalse, "EtcdNotReady", "etcd cluster is not fully ready")
	}

	return nil
}

// reconcileAPIServer manages the API Server DaemonSet
func (r *ReconcileTenantCluster) reconcileAPIServer(ctx context.Context, tc *appsv1alpha1.TenantCluster) error {
	// Check if APIServer DaemonSet exists and is ready
	ds := &appsv1.DaemonSet{}
	if err := r.Get(ctx, client.ObjectKey{Name: "tenant-apiserver", Namespace: tc.GetNamespace()}, ds); err != nil {
		if errors.IsNotFound(err) {
			// Create APIServer resources
			if err := r.createAPIServerResources(ctx, tc); err != nil {
				tc.Status.ComponentStatuses.APIServer = appsv1alpha1.ComponentStatus{
					Ready:              false,
					Message:            fmt.Sprintf("Failed to create APIServer: %v", err),
					LastTransitionTime: metav1.Now(),
				}
				tc.SetCondition(appsv1alpha1.TenantClusterConditionAPIServerReady, corev1.ConditionFalse, "APIServerCreationFailed", err.Error())
				return err
			}
			tc.Status.ComponentStatuses.APIServer = appsv1alpha1.ComponentStatus{
				Ready:              false,
				Message:            "APIServer is being provisioned",
				LastTransitionTime: metav1.Now(),
			}
			tc.SetCondition(appsv1alpha1.TenantClusterConditionAPIServerReady, corev1.ConditionFalse, "APIServerProvisioning", "APIServer DaemonSet is being created")
			return nil
		}
		return err
	}

	// Check if APIServer is ready
	if ds.Status.NumberReady > 0 && ds.Status.NumberReady == ds.Status.DesiredNumberScheduled {
		tc.Status.ComponentStatuses.APIServer = appsv1alpha1.ComponentStatus{
			Ready:              true,
			Message:            fmt.Sprintf("APIServer is ready with %d pods", ds.Status.NumberReady),
			LastTransitionTime: metav1.Now(),
		}
		tc.SetCondition(appsv1alpha1.TenantClusterConditionAPIServerReady, corev1.ConditionTrue, "APIServerReady", "APIServer is healthy")
		tc.Status.APIServerEndpoint = fmt.Sprintf("%s:6443", tc.Spec.APIServerServiceIP)
		tc.Status.ExternalAPIServerEndpoint = fmt.Sprintf("127.0.0.1:%d", tc.Spec.APIServerNodePort)
	} else {
		tc.Status.ComponentStatuses.APIServer = appsv1alpha1.ComponentStatus{
			Ready:              false,
			Message:            fmt.Sprintf("APIServer has %d/%d ready pods", ds.Status.NumberReady, ds.Status.DesiredNumberScheduled),
			LastTransitionTime: metav1.Now(),
		}
		tc.SetCondition(appsv1alpha1.TenantClusterConditionAPIServerReady, corev1.ConditionFalse, "APIServerNotReady", "APIServer is not fully ready")
	}

	return nil
}

// reconcileScheduler manages the Scheduler Deployment
func (r *ReconcileTenantCluster) reconcileScheduler(ctx context.Context, tc *appsv1alpha1.TenantCluster) error {
	deploy := &appsv1.Deployment{}
	if err := r.Get(ctx, client.ObjectKey{Name: "tenant-scheduler", Namespace: tc.GetNamespace()}, deploy); err != nil {
		if errors.IsNotFound(err) {
			if err := r.createSchedulerResources(ctx, tc); err != nil {
				tc.Status.ComponentStatuses.Scheduler = appsv1alpha1.ComponentStatus{
					Ready:              false,
					Message:            fmt.Sprintf("Failed to create Scheduler: %v", err),
					LastTransitionTime: metav1.Now(),
				}
				tc.SetCondition(appsv1alpha1.TenantClusterConditionSchedulerReady, corev1.ConditionFalse, "SchedulerCreationFailed", err.Error())
				return err
			}
			tc.Status.ComponentStatuses.Scheduler = appsv1alpha1.ComponentStatus{
				Ready:              false,
				Message:            "Scheduler is being provisioned",
				LastTransitionTime: metav1.Now(),
			}
			tc.SetCondition(appsv1alpha1.TenantClusterConditionSchedulerReady, corev1.ConditionFalse, "SchedulerProvisioning", "Scheduler Deployment is being created")
			return nil
		}
		return err
	}

	if deploy.Status.ReadyReplicas == *deploy.Spec.Replicas {
		tc.Status.ComponentStatuses.Scheduler = appsv1alpha1.ComponentStatus{
			Ready:              true,
			Message:            fmt.Sprintf("Scheduler is ready with %d replicas", deploy.Status.ReadyReplicas),
			LastTransitionTime: metav1.Now(),
		}
		tc.SetCondition(appsv1alpha1.TenantClusterConditionSchedulerReady, corev1.ConditionTrue, "SchedulerReady", "Scheduler is healthy")
	} else {
		tc.Status.ComponentStatuses.Scheduler = appsv1alpha1.ComponentStatus{
			Ready:              false,
			Message:            fmt.Sprintf("Scheduler has %d/%d ready replicas", deploy.Status.ReadyReplicas, *deploy.Spec.Replicas),
			LastTransitionTime: metav1.Now(),
		}
		tc.SetCondition(appsv1alpha1.TenantClusterConditionSchedulerReady, corev1.ConditionFalse, "SchedulerNotReady", "Scheduler is not fully ready")
	}

	return nil
}

// reconcileControllerManager manages the Controller Manager Deployment
func (r *ReconcileTenantCluster) reconcileControllerManager(ctx context.Context, tc *appsv1alpha1.TenantCluster) error {
	deploy := &appsv1.Deployment{}
	if err := r.Get(ctx, client.ObjectKey{Name: "tenant-controller-manager", Namespace: tc.GetNamespace()}, deploy); err != nil {
		if errors.IsNotFound(err) {
			if err := r.createControllerManagerResources(ctx, tc); err != nil {
				tc.Status.ComponentStatuses.ControllerManager = appsv1alpha1.ComponentStatus{
					Ready:              false,
					Message:            fmt.Sprintf("Failed to create ControllerManager: %v", err),
					LastTransitionTime: metav1.Now(),
				}
				tc.SetCondition(appsv1alpha1.TenantClusterConditionControllerManagerReady, corev1.ConditionFalse, "ControllerManagerCreationFailed", err.Error())
				return err
			}
			tc.Status.ComponentStatuses.ControllerManager = appsv1alpha1.ComponentStatus{
				Ready:              false,
				Message:            "ControllerManager is being provisioned",
				LastTransitionTime: metav1.Now(),
			}
			tc.SetCondition(appsv1alpha1.TenantClusterConditionControllerManagerReady, corev1.ConditionFalse, "ControllerManagerProvisioning", "ControllerManager Deployment is being created")
			return nil
		}
		return err
	}

	if deploy.Status.ReadyReplicas == *deploy.Spec.Replicas {
		tc.Status.ComponentStatuses.ControllerManager = appsv1alpha1.ComponentStatus{
			Ready:              true,
			Message:            fmt.Sprintf("ControllerManager is ready with %d replicas", deploy.Status.ReadyReplicas),
			LastTransitionTime: metav1.Now(),
		}
		tc.SetCondition(appsv1alpha1.TenantClusterConditionControllerManagerReady, corev1.ConditionTrue, "ControllerManagerReady", "ControllerManager is healthy")
	} else {
		tc.Status.ComponentStatuses.ControllerManager = appsv1alpha1.ComponentStatus{
			Ready:              false,
			Message:            fmt.Sprintf("ControllerManager has %d/%d ready replicas", deploy.Status.ReadyReplicas, *deploy.Spec.Replicas),
			LastTransitionTime: metav1.Now(),
		}
		tc.SetCondition(appsv1alpha1.TenantClusterConditionControllerManagerReady, corev1.ConditionFalse, "ControllerManagerNotReady", "ControllerManager is not fully ready")
	}

	return nil
}

// reconcilePostInstall applies post-install configurations to the tenant cluster
func (r *ReconcileTenantCluster) reconcilePostInstall(ctx context.Context, tc *appsv1alpha1.TenantCluster) error {
	// Create post-install reconciler with tenant cluster client
	postInstall, err := r.newPostInstallReconciler(ctx, tc)
	if err != nil {
		// If we can't create the post-install reconciler (e.g., tenant API server not ready),
		// mark as not ready and requeue
		tc.Status.ComponentStatuses.PostInstall = appsv1alpha1.ComponentStatus{
			Ready:              false,
			Message:            fmt.Sprintf("Waiting for tenant API server: %v", err),
			LastTransitionTime: metav1.Now(),
		}
		tc.SetCondition(appsv1alpha1.TenantClusterConditionPostInstallReady, corev1.ConditionFalse, "TenantAPIServerNotReady", err.Error())
		return nil // Don't return error, just wait for API server
	}

	// Apply post-install configurations
	if err := postInstall.reconcile(ctx); err != nil {
		tc.Status.ComponentStatuses.PostInstall = appsv1alpha1.ComponentStatus{
			Ready:              false,
			Message:            fmt.Sprintf("Failed to apply post-install configurations: %v", err),
			LastTransitionTime: metav1.Now(),
		}
		tc.SetCondition(appsv1alpha1.TenantClusterConditionPostInstallReady, corev1.ConditionFalse, "PostInstallFailed", err.Error())
		return err
	}

	tc.Status.ComponentStatuses.PostInstall = appsv1alpha1.ComponentStatus{
		Ready:              true,
		Message:            "Post-install configurations applied",
		LastTransitionTime: metav1.Now(),
	}
	tc.SetCondition(appsv1alpha1.TenantClusterConditionPostInstallReady, corev1.ConditionTrue, "PostInstallReady", "All post-install configurations are applied")

	// Set join command
	if tc.Status.BootstrapToken != "" {
		tc.Status.JoinCommand = fmt.Sprintf(
			"yurtadm join %s:6443 --node-type=local --host-control-plane-addr=%s --token=%s --discovery-token-unsafe-skip-ca-verification --cri-socket=/run/containerd/containerd.sock",
			tc.Spec.APIServerServiceIP,
			tc.Spec.HostControlPlaneAddr,
			tc.Status.BootstrapToken,
		)
	}

	return nil
}
