/*
Copyright The Kubernetes Authors.

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

	"sigs.k8s.io/cluster-api/controllers/noderefutil"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/pointer"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	expclusterv1 "sigs.k8s.io/cluster-api/exp/api/v1alpha3"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	infrav1 "sigs.k8s.io/cluster-api-provider-aws/api/v1alpha3"
	expinfrav1 "sigs.k8s.io/cluster-api-provider-aws/exp/api/v1alpha3"
	"sigs.k8s.io/cluster-api-provider-aws/pkg/cloud/scope"
	"sigs.k8s.io/cluster-api-provider-aws/pkg/cloud/services"
	"sigs.k8s.io/cluster-api-provider-aws/pkg/cloud/services/asg"
	"sigs.k8s.io/cluster-api-provider-aws/pkg/cloud/services/ec2"
)

// AWSMachinePoolReconciler reconciles a AWSMachinePool object
type AWSMachinePoolReconciler struct {
	client.Client
	Log               logr.Logger
	Recorder          record.EventRecorder
	asgServiceFactory func(*scope.ClusterScope) services.ASGMachineInterface
	ec2ServiceFactory func(*scope.ClusterScope) services.EC2MachineInterface
}

func (r *AWSMachinePoolReconciler) getASGService(scope *scope.ClusterScope) services.ASGMachineInterface {
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

// +kubebuilder:rbac:groups=exp.infrastructure.cluster.x-k8s.io,resources=awsmachinepools,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=exp.infrastructure.cluster.x-k8s.io,resources=awsmachinepools/status,verbs=get;update;patch

// Reconcile TODO: add comment bc exported
func (r *AWSMachinePoolReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
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

	// TODO: util.GetOwnerMachinePool?
	// TODO: util.GetClusterFromMetadata

	// Create the cluster scope
	clusterScope, err := scope.NewClusterScope(scope.ClusterScopeParams{
		Client:  r.Client,
		Logger:  logger,
		Cluster: &clusterv1.Cluster{},
		AWSCluster: &infrav1.AWSCluster{
			Spec: infrav1.AWSClusterSpec{
				Region: "us-east-1",
			},
		},
	})
	if err != nil {
		return ctrl.Result{}, err
	}

	// Create the machine poolscope
	machinePoolScope, err := scope.NewMachinePoolScope(scope.MachinePoolScopeParams{
		Logger:      logger,
		Client:      r.Client,
		Cluster:     &clusterv1.Cluster{},
		MachinePool: &expclusterv1.MachinePool{},
		AWSCluster: &infrav1.AWSCluster{
			Spec: infrav1.AWSClusterSpec{
				Region: "us-east-1",
			},
		},
		AWSMachinePool: awsMachinePool,
	})
	if err != nil {
		return ctrl.Result{}, errors.Errorf("failed to create scope: %+v", err)
	}

	// todo: defer conditions + machinePoolScope.Close()

	// todo: handle deletions
	if !awsMachinePool.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(machinePoolScope, clusterScope)
	}

	return r.reconcileNormal(ctx, machinePoolScope, clusterScope)
}

func (r *AWSMachinePoolReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&expinfrav1.AWSMachinePool{}).
		Complete(r)
}

func (r *AWSMachinePoolReconciler) reconcileNormal(_ context.Context, machinePoolScope *scope.MachinePoolScope, clusterScope *scope.ClusterScope) (ctrl.Result, error) {
	clusterScope.Info("Reconciling AWSMachine")

	// todo: check for failure state, return early

	// If the AWSMachinepool doesn't have our finalizer, add it
	controllerutil.AddFinalizer(machinePoolScope.AWSMachinePool, infrav1.MachineFinalizer)

	// todo: implement machinePoolScope.PatchObject for quickly registering the finalizer
	// todo: check cluster InfrastructureReady

	// Make sure bootstrap data is available and populated
	if machinePoolScope.MachinePool.Spec.Template.Spec.Bootstrap.DataSecretName == nil {
		machinePoolScope.Info("Bootstrap data secret reference is not yet available")
		// conditions.MarkFalse(machinePoolScope.AWSMachinePool, infrav1.InstanceReadyCondition, infrav1.WaitingForBootstrapDataReason, clusterv1.ConditionSeverityInfo, "") //TODO: GetCondition()
		return ctrl.Result{}, nil
	}

	userData, err := machinePoolScope.GetRawBootstrapData()
	if err != nil {
		r.Recorder.Eventf(machinePoolScope.AWSMachinePool, corev1.EventTypeWarning, "FailedGetBootstrapData", err.Error())
	}
	machinePoolScope.Info("checking for existing launch template")

	ec2svc := r.getEC2Service(clusterScope)
	launchTemplate, err := ec2svc.GetLaunchTemplate(machinePoolScope.Name())
	if err != nil {
		return ctrl.Result{}, err
	}
	if launchTemplate == nil {
		machinePoolScope.Info("no existing launch template found, creating")
		if _, err := ec2svc.CreateLaunchTemplate(machinePoolScope, userData); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Initialize ASG client
	asgsvc := r.getASGService(clusterScope)

	// Find existing ASG
	asg, err := r.findASG(machinePoolScope, asgsvc)
	if err != nil {
		return ctrl.Result{}, err
	}
	if asg == nil {

		// Create new ASG
		_, err = r.createPool(machinePoolScope, clusterScope)
		if err != nil {
			//TODO: ADd conditions
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// Make sure Spec.ProviderID is always set.
	// machinePoolScope.SetProviderID(instance.ID, instance.AvailabilityZone)

	// Get state of ASG
	// Set state
	// Reconcile AWSMachinePool State
	//Handle switch case instance.State{}
	return ctrl.Result{}, nil

}

func (r *AWSMachinePoolReconciler) reconcileDelete(machinePoolScope *scope.MachinePoolScope, clusterScope *scope.ClusterScope) (ctrl.Result, error) {
	// The ASG was never created or was deleted by some other entity
	// One way to reach this state:
	// 1. Scale deployment to 0
	// 2. Rename ASG and delete ProviderID from spec of both MachinePool
	// and AWSMachinePool
	// 3. Issue a delete
	// 4. Scale controller deployment to 1
	clusterScope.Info("Handling things")
	return ctrl.Result{}, nil
}

func (r *AWSMachinePoolReconciler) updatePool(machinePoolScope *scope.MachinePoolScope, clusterScope *scope.ClusterScope) (ctrl.Result, error) {
	clusterScope.Info("Handling things")
	return ctrl.Result{}, nil
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

func (r *AWSMachinePoolReconciler) findASG(machinePoolScope *scope.MachinePoolScope, asgsvc services.ASGMachineInterface) (*expinfrav1.AutoScalingGroup, error) {
	machinePoolScope.Info("Finding ASG")

	// Parse the ProviderID
	pid, err := noderefutil.NewProviderID(machinePoolScope.GetProviderID())
	if err != nil && err != noderefutil.ErrEmptyProviderID {
		return nil, errors.Wrapf(err, "failed to parse Spec.ProviderID")
	}

	// If the ProviderID is populated, describe the ASG using the ID.
	if err == nil {
		asg, err := asgsvc.AsgIfExists(pointer.StringPtr(pid.ID()))
		if err != nil {
			return nil, errors.Wrapf(err, "failed to query AWSMachinePool")
		}
		return asg, nil
	}

	// If the ProviderID is empty, try to query the instance using tags.
	asg, err := asgsvc.GetRunningAsgByName(machinePoolScope)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to query AWSMachinePool by tags")
	}

	return asg, nil
}
