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

package actuators

import (
	"fmt"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/cluster-api-provider-aws/pkg/apis/awsprovider/v1alpha1"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
	client "sigs.k8s.io/cluster-api/pkg/client/clientset_generated/clientset/typed/cluster/v1alpha1"
	"sigs.k8s.io/yaml"
)

// MachineScopeParams defines the input parameters used to create a new MachineScope.
type MachineScopeParams struct {
	AWSClients
	Cluster *clusterv1.Cluster
	Machine *clusterv1.Machine
	Client  client.ClusterV1alpha1Interface
	Logger  logr.Logger
}

// NewMachineScope creates a new MachineScope from the supplied parameters.
// This is meant to be called for each machine actuator operation.
func NewMachineScope(params MachineScopeParams) (*MachineScope, error) {
	scope, err := NewScope(ScopeParams{
		AWSClients: params.AWSClients,
		Client:     params.Client, Cluster: params.Cluster,
		Logger: params.Logger,
	})
	if err != nil {
		return nil, err
	}

	machineConfig, err := MachineConfigFromProviderSpec(params.Client, params.Machine.Spec.ProviderSpec, scope.Logger)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get machine config")
	}

	machineStatus, err := v1alpha1.MachineStatusFromProviderStatus(params.Machine.Status.ProviderStatus)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get machine provider status")
	}

	var machineClient client.MachineInterface
	if params.Client != nil {
		machineClient = params.Client.Machines(params.Machine.Namespace)
	}
	scope.Logger = scope.Logger.WithName(params.Machine.Name)
	return &MachineScope{
		Scope:         scope,
		Machine:       params.Machine,
		MachineClient: machineClient,
		MachineConfig: machineConfig,
		MachineStatus: machineStatus,
	}, nil
}

// MachineScope defines a scope defined around a machine and its cluster.
type MachineScope struct {
	*Scope

	Machine       *clusterv1.Machine
	MachineClient client.MachineInterface
	MachineConfig *v1alpha1.AWSMachineProviderSpec
	MachineStatus *v1alpha1.AWSMachineProviderStatus
}

// Name returns the machine name.
func (m *MachineScope) Name() string {
	return m.Machine.Name
}

// Namespace returns the machine namespace.
func (m *MachineScope) Namespace() string {
	return m.Machine.Namespace
}

// Role returns the machine role from the labels.
func (m *MachineScope) Role() string {
	return m.Machine.Labels["set"]
}

// Region returns the machine region.
func (m *MachineScope) Region() string {
	return m.Scope.Region()
}

// GetMachine returns the machine wrapped in the scope.
func (m *MachineScope) GetMachine() *clusterv1.Machine {
	return m.Machine
}

// GetScope() returns the scope that is wrapping the machine.
func (m *MachineScope) GetScope() *Scope {
	return m.Scope
}

func (m *MachineScope) storeMachineSpec(machine *clusterv1.Machine) (*clusterv1.Machine, error) {
	ext, err := v1alpha1.EncodeMachineSpec(m.MachineConfig)
	if err != nil {
		return nil, err
	}

	machine.Spec.ProviderSpec.Value = ext
	return m.MachineClient.Update(machine)
}

func (m *MachineScope) storeMachineStatus(machine *clusterv1.Machine) (*clusterv1.Machine, error) {
	ext, err := v1alpha1.EncodeMachineStatus(m.MachineStatus)
	if err != nil {
		return nil, err
	}

	m.Machine.Status.DeepCopyInto(&machine.Status)
	machine.Status.ProviderStatus = ext
	return m.MachineClient.UpdateStatus(machine)
}

// Close the MachineScope by updating the machine spec, machine status.
func (m *MachineScope) Close() {
	if m.MachineClient == nil {
		return
	}

	latestMachine, err := m.storeMachineSpec(m.Machine)
	if err != nil {
		m.Error(err, "failed to update machine")
		return
	}

	_, err = m.storeMachineStatus(latestMachine)
	if err != nil {
		m.Error(err, "failed to store machine provider status")
	}
}

// MachineConfigFromProviderSpec tries to decode the JSON-encoded spec, falling back on getting a MachineClass if the value is absent.
func MachineConfigFromProviderSpec(clusterClient client.MachineClassesGetter, providerConfig clusterv1.ProviderSpec, log logr.Logger) (*v1alpha1.AWSMachineProviderSpec, error) {
	var config v1alpha1.AWSMachineProviderSpec
	if providerConfig.Value != nil {
		log.V(4).Info("Decoding ProviderConfig from Value")
		return unmarshalProviderSpec(providerConfig.Value, log)
	}

	if providerConfig.ValueFrom != nil && providerConfig.ValueFrom.MachineClass != nil {
		ref := providerConfig.ValueFrom.MachineClass
		log.V(4).Info("Decoding ProviderConfig from MachineClass")
		log.V(6).Info("Machine class reference", "ref", fmt.Sprintf("%+v", ref))
		if ref.Provider != "" && ref.Provider != "aws" {
			return nil, errors.Errorf("Unsupported provider: %q", ref.Provider)
		}

		if len(ref.Namespace) > 0 && len(ref.Name) > 0 {
			log.V(4).Info("Getting MachineClass", "reference-namespace", ref.Namespace, "reference-name", ref.Name)
			mc, err := clusterClient.MachineClasses(ref.Namespace).Get(ref.Name, metav1.GetOptions{})
			log.V(6).Info("Retrieved MachineClass", "machine-class", fmt.Sprintf("%+v", mc))
			if err != nil {
				return nil, err
			}
			providerConfig.Value = &mc.ProviderSpec
			return unmarshalProviderSpec(&mc.ProviderSpec, log)
		}
	}

	return &config, nil
}

func unmarshalProviderSpec(spec *runtime.RawExtension, log logr.Logger) (*v1alpha1.AWSMachineProviderSpec, error) {
	var config v1alpha1.AWSMachineProviderSpec
	if spec != nil {
		if err := yaml.Unmarshal(spec.Raw, &config); err != nil {
			return nil, err
		}
	}
	log.V(6).Info("Found ProviderSpec", "provider-spec", fmt.Sprintf("%+v", config))
	return &config, nil
}
