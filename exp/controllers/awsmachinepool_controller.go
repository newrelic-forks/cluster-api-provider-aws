/*
Copyright 2020 The Kubernetes Authors.

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

package controllers

import (
	"context"
	"fmt"
	"reflect"

	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/conditions"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/record"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	capiv1exp "sigs.k8s.io/cluster-api/exp/api/v1alpha3"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	infrav1 "sigs.k8s.io/cluster-api-provider-aws/api/v1alpha3"
	expinfrav1 "sigs.k8s.io/cluster-api-provider-aws/exp/api/v1alpha3"
	"sigs.k8s.io/cluster-api-provider-aws/pkg/cloud/scope"
	"sigs.k8s.io/cluster-api-provider-aws/pkg/cloud/services"
	asg "sigs.k8s.io/cluster-api-provider-aws/pkg/cloud/services/autoscaling"
	"sigs.k8s.io/cluster-api-provider-aws/pkg/cloud/services/ec2"
)

// AWSMachinePoolReconciler reconciles a AWSMachinePool object
type AWSMachinePoolReconciler struct {
	client.Client
	Log               logr.Logger
	Recorder          record.EventRecorder
	asgServiceFactory func(*scope.ClusterScope) services.ASGInterface
	ec2ServiceFactory func(*scope.ClusterScope) services.EC2MachineInterface
}

func (r *AWSMachinePoolReconciler) getASGService(scope *scope.ClusterScope) services.ASGInterface {
	if r.asgServiceFactory != nil {
		return r.asgServiceFactory(scope)
	}
	return asg.NewService(scope)
}

func (r *AWSMachinePoolReconciler) getEC2Service(scope *scope.ClusterScope) services.EC2MachineInterface {
	if r.ec2ServiceFactory != nil {
		return r.ec2ServiceFactory(scope)
	}

	return ec2.NewService(scope)
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=awsmachinepools,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=awsmachinepools/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=exp.cluster.x-k8s.io,resources=machinepools;machinepools/status,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups="",resources=secrets;,verbs=get;list;watch

// Reconcile is the reconciliation loop for AWSMachinePool
func (r *AWSMachinePoolReconciler) Reconcile(req ctrl.Request) (_ ctrl.Result, reterr error) {
	ctx := context.TODO()
	logger := r.Log.WithValues("namespace", req.Namespace, "awsMachinePool", req.Name)

	// Fetch the AWSMachinePool .
	awsMachinePool := &expinfrav1.AWSMachinePool{}
	err := r.Get(ctx, req.NamespacedName, awsMachinePool)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Fetch the CAPI MachinePool
	machinePool, err := getOwnerMachinePool(ctx, r.Client, awsMachinePool.ObjectMeta)
	if err != nil {
		return reconcile.Result{}, err
	}
	if machinePool == nil {
		logger.Info("MachinePool Controller has not yet set OwnerRef")
		return reconcile.Result{}, nil
	}
	logger = logger.WithValues("machinePool", machinePool.Name)

	// Fetch the Cluster.
	cluster, err := util.GetClusterFromMetadata(ctx, r.Client, machinePool.ObjectMeta)
	if err != nil {
		logger.Info("MachinePool is missing cluster label or cluster does not exist")
		return reconcile.Result{}, nil
	}

	logger = logger.WithValues("cluster", cluster.Name)

	awsClusterName := client.ObjectKey{
		Namespace: awsMachinePool.Namespace,
		Name:      cluster.Spec.InfrastructureRef.Name,
	}
	awsCluster := &infrav1.AWSCluster{}
	if err := r.Client.Get(ctx, awsClusterName, awsCluster); err != nil {
		logger.Info("InfraCluster is not available yet")
		return reconcile.Result{}, nil
	}

	logger = logger.WithValues("InfraCluster", awsCluster.Name)

	// Create the cluster scope
	clusterScope, err := scope.NewClusterScope(scope.ClusterScopeParams{
		Client:     r.Client,
		Logger:     logger,
		Cluster:    cluster,
		AWSCluster: awsCluster,
	})
	if err != nil {
		return ctrl.Result{}, err
	}

	// Create the machine poolscope
	machinePoolScope, err := scope.NewMachinePoolScope(scope.MachinePoolScopeParams{
		Logger:         logger,
		Client:         r.Client,
		Cluster:        cluster,
		MachinePool:    machinePool,
		AWSCluster:     awsCluster,
		AWSMachinePool: awsMachinePool,
	})
	if err != nil {
		return ctrl.Result{}, errors.Errorf("failed to create scope: %+v", err)
	}

	// Always close the scope when exiting this function so we can persist any AWSMachine changes.
	defer func() {
		// set Ready condition before AWSMachinePool is patched
		conditions.SetSummary(machinePoolScope.AWSMachinePool,
			conditions.WithConditions(
				expinfrav1.ASGReadyCondition,
				expinfrav1.LaunchTemplateReadyCondition,
			),
			conditions.WithStepCounterIfOnly(
				expinfrav1.ASGReadyCondition,
				expinfrav1.LaunchTemplateReadyCondition,
			),
		)

		if err := machinePoolScope.Close(); err != nil && reterr == nil {
			reterr = err
		}
	}()

	if !awsMachinePool.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(machinePoolScope, clusterScope)
	}

	return r.reconcileNormal(ctx, machinePoolScope, clusterScope)
}

func (r *AWSMachinePoolReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// todo: add watch for MachinePool object (see AWSMachine's SetupWithManager)
	return ctrl.NewControllerManagedBy(mgr).
		For(&expinfrav1.AWSMachinePool{}).
		Watches(
			&source.Kind{Type: &capiv1exp.MachinePool{}},
			&handler.EnqueueRequestsFromMapFunc{
				ToRequests: machinePoolToInfrastructureMapFunc(expinfrav1.GroupVersion.WithKind("AWSMachinePool")),
			},
		).
		Complete(r)
}

func (r *AWSMachinePoolReconciler) reconcileNormal(_ context.Context, machinePoolScope *scope.MachinePoolScope, clusterScope *scope.ClusterScope) (ctrl.Result, error) {
	clusterScope.Info("Reconciling AWSMachinePool")

	// If the AWSMachine is in an error state, return early.
	if machinePoolScope.HasFailed() {
		machinePoolScope.Info("Error state detected, skipping reconciliation")

		// TODO: If we are in a failed state, delete the secret regardless of instance state

		return ctrl.Result{}, nil
	}

	// If the AWSMachinepool doesn't have our finalizer, add it
	controllerutil.AddFinalizer(machinePoolScope.AWSMachinePool, expinfrav1.MachinePoolFinalizer)

	// Register finalizer immediately to avoid orphaning AWS resources
	if err := machinePoolScope.PatchObject(); err != nil {
		return ctrl.Result{}, err
	}

	if !machinePoolScope.Cluster.Status.InfrastructureReady {
		machinePoolScope.Info("Cluster infrastructure is not ready yet")
		conditions.MarkFalse(machinePoolScope.AWSMachinePool, expinfrav1.ASGReadyCondition, infrav1.WaitingForClusterInfrastructureReason, clusterv1.ConditionSeverityInfo, "")
		return ctrl.Result{}, nil
	}

	// Make sure bootstrap data is available and populated
	if machinePoolScope.MachinePool.Spec.Template.Spec.Bootstrap.DataSecretName == nil {
		machinePoolScope.Info("Bootstrap data secret reference is not yet available")
		conditions.MarkFalse(machinePoolScope.AWSMachinePool, expinfrav1.ASGReadyCondition, infrav1.WaitingForBootstrapDataReason, clusterv1.ConditionSeverityInfo, "")
		return ctrl.Result{}, nil
	}

	if err := r.reconcileLaunchTemplate(machinePoolScope, clusterScope); err != nil {
		r.Recorder.Eventf(machinePoolScope.AWSMachinePool, corev1.EventTypeWarning, "FailedLaunchTemplateReconcile", "Failed to reconcile launch template: %v", err)
		machinePoolScope.Error(err, "failed to reconcile launch template")
		return ctrl.Result{}, err
	}

	// set the LaunchTemplateReady condition
	conditions.MarkTrue(machinePoolScope.AWSMachinePool, expinfrav1.LaunchTemplateReadyCondition)

	// Initialize ASG client
	asgsvc := r.getASGService(clusterScope)

	// Find existing ASG
	asg, err := r.findASG(machinePoolScope, asgsvc)
	if err != nil {
		conditions.MarkUnknown(machinePoolScope.AWSMachinePool, expinfrav1.ASGReadyCondition, expinfrav1.ASGNotFoundReason, err.Error())
		return ctrl.Result{}, err
	}

	if asg == nil {
		// Create new ASG
		_, err = r.createPool(machinePoolScope, clusterScope)
		if err != nil {
			conditions.MarkFalse(machinePoolScope.AWSMachinePool, expinfrav1.ASGReadyCondition, expinfrav1.ASGProvisionFailedReason, clusterv1.ConditionSeverityError, err.Error())
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	if err := r.updatePool(machinePoolScope, clusterScope, asg); err != nil {
		return ctrl.Result{}, errors.Wrap(err, "error updating AWSMachinePool")
	}

	// Make sure Spec.ProviderID is always set.
	machinePoolScope.AWSMachinePool.Spec.ProviderID = asg.ID
	providerIDList := make([]string, len(asg.Instances))

	for i, ec2 := range asg.Instances {
		providerIDList[i] = fmt.Sprintf("aws:///%s/%s", ec2.AvailabilityZone, ec2.ID)
	}

	machinePoolScope.SetAnnotation("cluster-api-provider-aws", "true")

	machinePoolScope.AWSMachinePool.Spec.ProviderIDList = providerIDList
	machinePoolScope.AWSMachinePool.Status.Replicas = int32(len(providerIDList))
	machinePoolScope.AWSMachinePool.Status.Ready = true
	conditions.MarkTrue(machinePoolScope.AWSMachinePool, expinfrav1.ASGReadyCondition)

	return ctrl.Result{}, nil
}

func (r *AWSMachinePoolReconciler) reconcileDelete(machinePoolScope *scope.MachinePoolScope, clusterScope *scope.ClusterScope) (ctrl.Result, error) {
	clusterScope.Info("Handling deleted AWSMachinePool")

	ec2Svc := r.getEC2Service(clusterScope)
	asgSvc := r.getASGService(clusterScope)

	asg, err := r.findASG(machinePoolScope, asgSvc)
	if err != nil {
		return ctrl.Result{}, err
	}

	machinePoolScope.SetASGStatus(asg.Status)
	if asg == nil {
		machinePoolScope.V(2).Info("Unable to locate ASG")
		r.Recorder.Eventf(machinePoolScope.AWSMachinePool, corev1.EventTypeNormal, "NoASGFound", "Unable to find matching ASG")
	} else {
		switch asg.Status {
		case expinfrav1.ASGStatusDeleteInProgress:
			// ASG is already deleting
			// machinePoolScope.AWSMachinePool.Status.Ready = false
			machinePoolScope.SetNotReady()
			conditions.MarkFalse(machinePoolScope.AWSMachinePool, expinfrav1.ASGReadyCondition, expinfrav1.ASGDeletionInProgress, clusterv1.ConditionSeverityWarning, "")
			r.Recorder.Eventf(machinePoolScope.AWSMachinePool, corev1.EventTypeWarning, "DeletionInProgress", "ASG deletion in progress: %q", asg.Name)
			machinePoolScope.Info("ASG is already deleting", "name", asg.Name)
		default:
			machinePoolScope.Info("Deleting ASG", "id", asg.Name, "status", asg.Status)
			if err := asgSvc.DeleteASGAndWait(asg.Name); err != nil {
				r.Recorder.Eventf(machinePoolScope.AWSMachinePool, corev1.EventTypeWarning, "FailedDelete", "Failed to delete ASG %q: %v", asg.Name, err)
				return ctrl.Result{}, errors.Wrap(err, "failed to delete ASG")
			}
		}
	}

	launchTemplate, err := ec2Svc.GetLaunchTemplate(machinePoolScope.AWSMachinePool.Status.LaunchTemplateID)
	if err != nil {
		return ctrl.Result{}, err
	}

	if launchTemplate == nil {
		machinePoolScope.V(2).Info("Unable to locate launch template")
		r.Recorder.Eventf(machinePoolScope.AWSMachinePool, corev1.EventTypeNormal, "NoASGFound", "Unable to find matching ASG")
		controllerutil.RemoveFinalizer(machinePoolScope.AWSMachinePool, expinfrav1.MachinePoolFinalizer)
		return ctrl.Result{}, nil
	}

	machinePoolScope.Info("deleting launch template", "name", launchTemplate.Name)
	if err := ec2Svc.DeleteLaunchTemplate(launchTemplate.ID); err != nil {
		r.Recorder.Eventf(machinePoolScope.AWSMachinePool, corev1.EventTypeWarning, "FailedDelete", "Failed to delete launch template %q: %v", launchTemplate.Name, err)
		return ctrl.Result{}, errors.Wrap(err, "failed to delete ASG")
	}

	machinePoolScope.Info("successfully deleted AutoScalingGroup and Launch Template")

	// remove finalizer
	controllerutil.RemoveFinalizer(machinePoolScope.AWSMachinePool, expinfrav1.MachinePoolFinalizer)

	return ctrl.Result{}, nil
}

func (r *AWSMachinePoolReconciler) updatePool(machinePoolScope *scope.MachinePoolScope, clusterScope *scope.ClusterScope, existingASG *expinfrav1.AutoScalingGroup) error {
	if asgNeedsUpdates(machinePoolScope, existingASG) {
		machinePoolScope.Info("updating AutoScalingGroup")
		asgSvc := r.getASGService(clusterScope)

		if err := asgSvc.UpdateASG(machinePoolScope); err != nil {
			r.Recorder.Eventf(machinePoolScope.AWSMachinePool, corev1.EventTypeWarning, "FailedUpdate", "Failed to update ASG: %v", err)
			return errors.Wrap(err, "unable to update ASG")
		}
	}

	return nil
}

func (r *AWSMachinePoolReconciler) createPool(machinePoolScope *scope.MachinePoolScope, clusterScope *scope.ClusterScope) (*expinfrav1.AutoScalingGroup, error) {
	clusterScope.Info("Initializing ASG client")

	asgsvc := r.getASGService(clusterScope)

	machinePoolScope.Info("Creating Autoscaling Group")
	asg, err := asgsvc.CreateASG(machinePoolScope)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create AWSMachinePool")
	}

	return asg, nil
}

func (r *AWSMachinePoolReconciler) findASG(machinePoolScope *scope.MachinePoolScope, asgsvc services.ASGInterface) (*expinfrav1.AutoScalingGroup, error) {
	// Query the instance using tags.
	asg, err := asgsvc.GetASGByName(machinePoolScope)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to query AWSMachinePool by name")
	}

	return asg, nil
}

func (r *AWSMachinePoolReconciler) reconcileLaunchTemplate(machinePoolScope *scope.MachinePoolScope, clusterScope *scope.ClusterScope) error {
	userData, err := machinePoolScope.GetRawBootstrapData()
	if err != nil {
		r.Recorder.Eventf(machinePoolScope.AWSMachinePool, corev1.EventTypeWarning, "FailedGetBootstrapData", err.Error())
	}
	machinePoolScope.Info("checking for existing launch template")

	ec2svc := r.getEC2Service(clusterScope)
	launchTemplate, err := ec2svc.GetLaunchTemplate(machinePoolScope.AWSMachinePool.Status.LaunchTemplateID)
	if err != nil {
		conditions.MarkUnknown(machinePoolScope.AWSMachinePool, expinfrav1.LaunchTemplateReadyCondition, expinfrav1.LaunchTemplateNotFoundReason, err.Error())
		return err
	}
	if launchTemplate == nil {
		machinePoolScope.Info("no existing launch template found, creating")
		launchTemplateID, err := ec2svc.CreateLaunchTemplate(machinePoolScope, userData)
		if err != nil {
			conditions.MarkFalse(machinePoolScope.AWSMachinePool, expinfrav1.LaunchTemplateReadyCondition, expinfrav1.LaunchTemplateCreateFailedReason, clusterv1.ConditionSeverityError, err.Error())
			return err
		}

		machinePoolScope.AWSMachinePool.Status.LaunchTemplateID = launchTemplateID

		return machinePoolScope.PatchObject()
	}

	needsUpdate, err := ec2svc.LaunchTemplateNeedsUpdate(&machinePoolScope.AWSMachinePool.Spec.AWSLaunchTemplate, launchTemplate)
	if err != nil {
		return err
	}
	if needsUpdate {
		machinePoolScope.Info("creating new version for launch template", "existing", launchTemplate, "incoming", machinePoolScope.AWSMachinePool.Spec.AWSLaunchTemplate)
		if err := ec2svc.CreateLaunchTemplateVersion(machinePoolScope, userData); err != nil {
			return err
		}
	}

	return nil
}

// asgNeedsUpdates compares incoming AWSMachinePool and compares against existing ASG
func asgNeedsUpdates(machinePoolScope *scope.MachinePoolScope, existingASG *expinfrav1.AutoScalingGroup) bool {
	if machinePoolScope.MachinePool.Spec.Replicas != nil && machinePoolScope.MachinePool.Spec.Replicas != existingASG.DesiredCapacity {
		return true
	}

	if machinePoolScope.AWSMachinePool.Spec.MaxSize != existingASG.MaxSize {
		return true
	}

	if machinePoolScope.AWSMachinePool.Spec.MinSize != existingASG.MinSize {
		return true
	}

	if !reflect.DeepEqual(machinePoolScope.AWSMachinePool.Spec.MixedInstancesPolicy, existingASG.MixedInstancesPolicy) {
		machinePoolScope.Info("got a mixed diff here", "incoming", machinePoolScope.AWSMachinePool.Spec.MixedInstancesPolicy, "existing", existingASG.MixedInstancesPolicy)
		return true
	}

	// todo subnet diff

	return false
}

// getOwnerMachinePool returns the MachinePool object owning the current resource.
func getOwnerMachinePool(ctx context.Context, c client.Client, obj metav1.ObjectMeta) (*capiv1exp.MachinePool, error) {
	for _, ref := range obj.OwnerReferences {
		if ref.Kind != "MachinePool" {
			continue
		}
		gv, err := schema.ParseGroupVersion(ref.APIVersion)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		if gv.Group == capiv1exp.GroupVersion.Group {
			return getMachinePoolByName(ctx, c, obj.Namespace, ref.Name)
		}
	}
	return nil, nil
}

// getMachinePoolByName finds and return a Machine object using the specified params.
func getMachinePoolByName(ctx context.Context, c client.Client, namespace, name string) (*capiv1exp.MachinePool, error) {
	m := &capiv1exp.MachinePool{}
	key := client.ObjectKey{Name: name, Namespace: namespace}
	if err := c.Get(ctx, key, m); err != nil {
		return nil, err
	}
	return m, nil
}

func machinePoolToInfrastructureMapFunc(gvk schema.GroupVersionKind) handler.ToRequestsFunc {
	return func(o handler.MapObject) []reconcile.Request {
		m, ok := o.Object.(*capiv1exp.MachinePool)
		if !ok {
			return nil
		}

		gk := gvk.GroupKind()
		// Return early if the GroupKind doesn't match what we expect
		infraGK := m.Spec.Template.Spec.InfrastructureRef.GroupVersionKind().GroupKind()
		if gk != infraGK {
			return nil
		}

		return []reconcile.Request{
			{
				NamespacedName: client.ObjectKey{
					Namespace: m.Namespace,
					Name:      m.Spec.Template.Spec.InfrastructureRef.Name,
				},
			},
		}
	}
}
