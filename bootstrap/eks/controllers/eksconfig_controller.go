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
	"github.com/prometheus/common/log"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
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
	ctx = context.Background()
	_ = r.Log.WithValues("eksconfig", req.NamespacedName)

	config := &bootstrapv1.EKSConfig{}
	if err := r.Client.Get(ctx, req.NamespacedName, config); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get config")
		return ctrl.Result{}, err
	}

	// always update the readyCondition; the summary is represented using the "1 of x completed" notation.
	conditions.SetSummary(config,
		conditions.WithConditions(
			bootstrapv1.DataSecretAvailableCondition,
			bootstrapv1.CertificatesAvailableCondition,
		),
		conditions.WithStepCounter(),
	)
	// Patch ObservedGeneration only if the reconciliation completed successfully
	patchOpts := []patch.Option{}
	if rerr == nil {
		patchOpts = append(patchOpts, patch.WithStatusObservedGeneration{})
	}
	if err := patchHelper.Patch(ctx, config, patchOpts...); err != nil {
		log.Error(rerr, "Failed to patch config")
		if rerr == nil {
			rerr = err
		}
	}

	return ctrl.Result{}, nil
}

func (r *EKSConfigReconciler) SetupWithManager(mgr ctrl.Manager, option controller.Options) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&bootstrapv1.EKSConfig{}).
		WithOptions(option).
		Complete(r)
}
