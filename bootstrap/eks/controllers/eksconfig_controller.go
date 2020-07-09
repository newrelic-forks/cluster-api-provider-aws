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

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"

	bootstrapv1 "sigs.k8s.io/cluster-api-provider-aws/bootstrap/eks/api/v1alpha3"
)

// EKSConfigReconciler reconciles a EKSConfig object
type EKSConfigReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=bootstrap.cluster.x-k8s.io,resources=eksconfigs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=bootstrap.cluster.x-k8s.io,resources=eksconfigs/status,verbs=get;update;patch

func (r *EKSConfigReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	_ = r.Log.WithValues("eksconfig", req.NamespacedName)

	// get EKSConfig
	// check owner references and look up owning Machine object
	// look up Cluster object associated with owning Machine object
	// check for paused annotation
	// create "Scope" - struct TBD
	// set up patchHelper (import from CAPI?)
	// set up defer block for updating status
	// check Cluster's InfrastructureReady - requeue if false

	// enter joinWorker

	return r.joinWorker(ctx)
}

func (r *EKSConfigReconciler) joinWorker(ctx context.Context) (ctrl.Result, error) {
	// generate userdata
	// store userdata as secret (this can basically be totally copied from kubeadm provider - any way to reuse?)
	// set status.DataSecretName
	// set status.Ready to true
	// mark DataSecretAvailableCondition as true

	return ctrl.Result{}, nil
}

func (r *EKSConfigReconciler) SetupWithManager(mgr ctrl.Manager, option controller.Options) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&bootstrapv1.EKSConfig{}).
		WithOptions(option).
		Complete(r)
}
