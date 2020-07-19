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
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// MachinePoolScope defines a scope defined around a machine and its cluster.
type MachinePoolScope struct {
	logr.Logger
	client     client.Client
	Name       string
	AWSMachine *infrav1.AWSMachine
	// Logger
	// client
	// patchHelper

	// Cluster:        params.Cluster,
	// MachinePool:    params.MachinePool,
	// AWSCluster:     params.AWSCluster,
	AWSMachinePool *infrav1.AWSMachinePool
}

// MachinePoolScopeParams defines a scope defined around a machine and its cluster.
type MachinePoolScopeParams struct {
	Name       string
	AWSMachine *infrav1.AWSMachine
	Logger     logr.Logger
	Client     client.Client
	// patchHelper

	// Cluster:        params.Cluster,
	MachinePool *infrav1.MachinePool //TODO: why is it in cluster-api for machines?
	clusterv1.Machine
	// AWSCluster:     params.AWSCluster,
	AWSMachinePool *infrav1.AWSMachinePool
}

// GetProviderID returns the AWSMachine providerID from the spec.
func (m *MachinePoolScope) GetProviderID() string {
	if m.AWSMachine.Spec.ProviderID != nil {
		return *m.AWSMachine.Spec.ProviderID
	}
	return ""
}

// NewMachinePoolScope creates a new MachinePoolScope from the supplied parameters.
// This is meant to be called for each reconcile iteration.
func NewMachinePoolScope(params MachinePoolScopeParams) (*MachinePoolScope, error) {
	if params.Client == nil {
		return nil, errors.New("client is required when creating a MachinePoolScope")
	}
	if params.MachinePool == nil {
		return nil, errors.New("machine pool is required when creating a MachinePoolScope")
	}
	// if params.Cluster == nil {
	// 	return nil, errors.New("cluster is required when creating a MachinePoolScope")
	// }
	// if params.AWSMachine == nil {
	// 	return nil, errors.New("aws machine is required when creating a MachinePoolScope")
	// }
	// if params.AWSCluster == nil {
	// 	return nil, errors.New("aws cluster is required when creating a MachinePoolScope")
	// }

	if params.Logger == nil {
		params.Logger = klogr.New()
	}

	// helper, err := patch.NewHelper(params.AWSMachine, params.Client)
	// if err != nil {
	// 	return nil, errors.Wrap(err, "failed to init patch helper")
	// }
	return &MachinePoolScope{
		Logger: params.Logger,
		// client:      params.Client,
		// patchHelper: helper,

		// Cluster:        params.Cluster,
		// MachinePool:    params.MachinePool,
		// AWSCluster:     params.AWSCluster,
		AWSMachinePool: params.AWSMachinePool,
	}, nil
}
