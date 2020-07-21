/*
Copyright 2018 The Kubernetes Authors.

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

package scope

import (
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"k8s.io/klog/klogr"
	infrav1 "sigs.k8s.io/cluster-api-provider-aws/api/v1alpha3"
	expinfrav1 "sigs.k8s.io/cluster-api-provider-aws/exp/api/v1alpha3"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	expclusterv1 "sigs.k8s.io/cluster-api/exp/api/v1alpha3"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// MachinePoolScopeParams defines the input parameters used to create a new MachinePoolScope.
type MachinePoolScopeParams struct {
	AWSClients
	Client         client.Client
	Logger         logr.Logger
	Cluster        *clusterv1.Cluster
	MachinePool    *expclusterv1.MachinePool
	AWSCluster     *infrav1.AWSCluster
	AWSMachinePool *expinfrav1.AWSMachinePool
}

// NewMachinePoolScope creates a new MachinePoolScope from the supplied parameters.
// This is meant to be called for each reconcile iteration.
func NewMachinePoolScope(params MachinePoolScopeParams) (*MachinePoolScope, error) {
	if params.Client == nil {
		return nil, errors.New("client is required when creating a MachinePoolScope")
	}
	if params.MachinePool == nil {
		return nil, errors.New("machinepool is required when creating a MachinePoolScope")
	}
	if params.Cluster == nil {
		return nil, errors.New("cluster is required when creating a MachinePoolScope")
	}
	if params.AWSMachinePool == nil {
		return nil, errors.New("aws machine pool is required when creating a MachinePoolScope")
	}
	if params.AWSCluster == nil {
		return nil, errors.New("aws cluster is required when creating a MachinePoolScope")
	}

	if params.Logger == nil {
		params.Logger = klogr.New()
	}

	helper, err := patch.NewHelper(params.AWSMachinePool, params.Client)
	if err != nil {
		return nil, errors.Wrap(err, "failed to init patch helper")
	}
	return &MachinePoolScope{
		Logger:      params.Logger,
		client:      params.Client,
		patchHelper: helper,

		Cluster:        params.Cluster,
		MachinePool:    params.MachinePool,
		AWSCluster:     params.AWSCluster,
		AWSMachinePool: params.AWSMachinePool,
	}, nil
}

// MachinePoolScope defines a scope defined around a machinepool and its cluster.
type MachinePoolScope struct {
	logr.Logger
	client      client.Client
	patchHelper *patch.Helper

	Cluster        *clusterv1.Cluster
	MachinePool    *expclusterv1.MachinePool
	AWSCluster     *infrav1.AWSCluster
	AWSMachinePool *expinfrav1.AWSMachinePool
}

// Name returns the AWSMachinePool name.
func (m *MachinePoolScope) Name() string {
	return m.AWSMachinePool.Name
}

// Namespace returns the namespace name.
func (m *MachinePoolScope) Namespace() string {
	return m.AWSMachinePool.Namespace
}


// MachinePoolScope defines a scope defined around a machine and its cluster.
type MachinePoolScope struct {
	Name       string
	AWSMachine *infrav1.AWSMachine
}

// GetProviderID returns the AWSMachine providerID from the spec.
func (m *MachinePoolScope) GetProviderID() string {
	if m.AWSMachine.Spec.ProviderID != nil {
		return *m.AWSMachine.Spec.ProviderID
	}
	return ""
