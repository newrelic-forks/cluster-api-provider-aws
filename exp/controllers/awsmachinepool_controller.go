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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
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
	Scheme            *runtime.Scheme
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

	r.createPool(machinePoolScope, clusterScope)
	return ctrl.Result{}, nil

	// From our first mob. this should be moved to createPool
	// ec2svc := ec2.NewService(clusterScope)
	// _, err = ec2svc.GetLaunchTemplate()
	// if err != nil {
	// 	return ctrl.Result{}, err
	// }

	// return r.reconcileNormal(ctx, machinePoolScope, clusterScope)
}

func (r *AWSMachinePoolReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&expinfrav1.AWSMachinePool{}).
		Complete(r)
}

func (r *AWSMachinePoolReconciler) reconcileNormal(_ context.Context, machinePoolScope *scope.MachinePoolScope, clusterScope *scope.ClusterScope) (ctrl.Result, error) {
	clusterScope.Info("Reconciling AWSMachine")
	// If the AWSMachinepool doesn't have our finalizer, add it
	controllerutil.AddFinalizer(machinePoolScope.AWSMachinePool, infrav1.MachineFinalizer)
	// Make sure bootstrap data is available and populated

	// Find existing Instance
	r.getASGService(clusterScope)

	// Create new ASG

	// Make sure Spec.ProviderID is always set.
	// machinePoolScope.SetProviderID(instance.ID, instance.AvailabilityZone)

	// Get state of ASG
	// Set state
	// Reconcile AWSMachinePool State
	//Handle switch case instance.State{}
	return ctrl.Result{}, nil

}

func (r *AWSMachinePoolReconciler) reconcileDelete(machinePoolScope *scope.MachinePoolScope, clusterScope *scope.ClusterScope) (ctrl.Result, error) {
	clusterScope.Info("Handling things")
	return ctrl.Result{}, nil
}

func (r *AWSMachinePoolReconciler) updatePool(machinePoolScope *scope.MachinePoolScope, clusterScope *scope.ClusterScope) (ctrl.Result, error) {
	clusterScope.Info("Handling things")
	return ctrl.Result{}, nil
}

func (r *AWSMachinePoolReconciler) createPool(machinePoolScope *scope.MachinePoolScope, clusterScope *scope.ClusterScope) (*infrav1.AutoScalingGroup, error) {
	clusterScope.Info("Initializing ASG client")

	asgsvc := r.getASGService(clusterScope)
	clusterScope.Info("Creating Autoscaling Group")
	asg, err := asgsvc.CreateASG(machinePoolScope)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create AWSMachinePool")
	}
	return asg, nil
}

func (r *AWSMachinePoolReconciler) findASG(machinePoolScope *scope.MachinePoolScope, asgsvc services.ASGMachineInterface, clusterScope *scope.ClusterScope) (*infrav1.AutoScalingGroup, error) {
	clusterScope.Info("Finding ASG")
	// if instance is nil
	//   createPool() (both launch template and ASG)
	// else
	//   updatePool()

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
