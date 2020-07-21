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

package v1alpha3

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EBS from describe-launch-templates
type EBS struct {
	Encrypted  bool   `json:"encrypted,omitempty"`
	VolumeSize int64  `json:"volumeSize,omitempty"`
	VolumeType string `json:"volumeType,omitempty"`
}

// BlockDeviceMappings from describe-launch-templates
type BlockDeviceMapping struct {
	DeviceName string `json:"deviceName,omitempty"`
	Ebs        EBS    `json:"ebs,omitempty"`
}

// NetworkInterface from describe-launch-templates
type NetworkInterface struct {
	DeviceIndex int64    `json:"deviceIndex,omitempty"`
	Groups      []string `json:"groups,omitempty"`
}

// AwsLaunchTemplate defines the desired state of AWSLaunchTemplate
type AwsLaunchTemplate struct {
	// all the things needed for a launch template

	IamInstanceProfile  string               `json:"iamInstanceProfile,omitempty"`
	BlockDeviceMappings []BlockDeviceMapping `json:"blockDeviceMappings,omitempty"`
	NetworkInterfaces   []NetworkInterface   `json:"networkInterfaces,omitempty"`

	// todo: use a helper
	ImageId string `json:"imageId,omitempty"`

	// InstanceType is the type of instance to create. Example: m4.xlarge
	InstanceType string `json:"instanceType,omitempty"`

	// SSHKeyName is the name of the ssh key to attach to the instance. Valid values are empty string (do not use SSH keys), a valid SSH key name, or omitted (use the default SSH key name)
	// +optional
	SSHKeyName *string `json:"sshKeyName,omitempty"`

	VersionNumber *int64 `json:"versionNumber,omitempty"`
}

// LaunchTemplateSpecification from describe-auto-scaling-groups
type LaunchTemplateSpecification struct {
	LaunchTemplateID   string `json:"launchTemplateId,omitempty"`
	LaunchTemplateName string `json:"launchTemplateName,omitempty"`
	Version            string `json:"version,omitempty"`
}

// LaunchTemplate from describe-auto-scaling-groups
type LaunchTemplate struct {
	LaunchTemplateSpecification LaunchTemplateSpecification `json:"launchTemplateSpecification,omitempty"`
	Overrides                   []Overrides                 `json:"overrides,omitempty"`
}

// Overrides from describe-auto-scaling-groups
type Overrides struct {
	InstanceType string `json:"InstanceType"`
}

// InstancesDistribution from describe-auto-scaling-groups
type InstancesDistribution struct {
	OnDemandAllocationStrategy          string `json:"onDemandAllocationStrategy,omitempty"`
	OnDemandBaseCapacity                int    `json:"onDemandBaseCapacity,omitempty"`
	OnDemandPercentageAboveBaseCapacity int    `json:"onDemandPercentageAboveBaseCapacity,omitempty"`
	SpotAllocationStrategy              string `json:"spotAllocationStrategy,omitempty"`
}

// MixedInstancesPolicy from describe-auto-scaling-groups
type MixedInstancesPolicy struct {
	LaunchTemplate        LaunchTemplate        `json:"launchTemplate,omitempty"`
	InstancesDistribution InstancesDistribution `json:"instancesDistribution,omitempty"`
}

// Tags from describe-auto-scaling-groups
type Tags struct {
	ResourceID        string `json:"resourceId,omitempty"`
	ResourceType      string `json:"resourceType,omitempty"`
	Key               string `json:"key,omitempty"`
	Value             string `json:"value,omitempty"`
	PropagateAtLaunch bool   `json:"propagateAtLaunch,omitempty"`
}

// AWSMachinePoolSpec defines the desired state of AWSMachinePool
type AWSMachinePoolSpec struct {
	ProviderID                       *string              `json:"providerID,omitempty"` //TODO: is this needed?
	AutoScalingGroupName             string               `json:"autoScalingGroupName,omitempty"`
	MixedInstancesPolicy             MixedInstancesPolicy `json:"mixedInstancesPolicy,omitempty"`
	MinSize                          int                  `json:"minSize,omitempty"`
	MaxSize                          int                  `json:"maxSize,omitempty"`
	DesiredCapacity                  int                  `json:"desiredCapacity,omitempty"`
	DefaultCooldown                  int                  `json:"defaultCooldown,omitempty"`
	AvailabilityZones                []string             `json:"availabilityZones,omitempty"`
	HealthCheckType                  string               `json:"healthCheckType,omitempty"`
	HealthCheckGracePeriod           int                  `json:"healthCheckGracePeriod,omitempty"`
	VPCZoneIdentifier                string               `json:"vpcZoneIdentifier,omitempty"`
	Tags                             []Tags               `json:"tags,omitempty"`
	TerminationPolicies              []string             `json:"terminationPolicies,omitempty"`
	NewInstancesProtectedFromScaleIn bool                 `json:"newInstancesProtectedFromScaleIn,omitempty"`
	ServiceLinkedRoleARN             string               `json:"serviceLinkedRoleARN,omitempty"`
	AwsLaunchTemplate                AwsLaunchTemplate    `json:"awsLaunchTemplate,omitempty"`
}

// AWSMachinePoolStatus defines the observed state of AWSMachinePool
type AWSMachinePoolStatus struct {
	AutoScalingGroupARN string `json:"autoScalingGroupARN,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=awsmachinepools,scope=Namespaced,categories=cluster-api

// AWSMachinePool is the Schema for the awsmachinepools API
type AWSMachinePool struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AWSMachinePoolSpec   `json:"spec,omitempty"`
	Status AWSMachinePoolStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// AWSMachinePoolList contains a list of AWSMachinePool
type AWSMachinePoolList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AWSMachinePool `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AWSMachinePool{}, &AWSMachinePoolList{})
}

// MachinePool is the Schema for the machines API
type MachinePool struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MachinePoolSpec   `json:"spec,omitempty"`
	Status MachinePoolStatus `json:"status,omitempty"`
}

// func (m *MachinePool) GetConditions() Conditions {
// 	return m.Status.Conditions
// }

// func (m *MachinePool) SetConditions(conditions Conditions) {
// 	m.Status.Conditions = conditions
// }

// MachineSpec defines the desired state of Machine
type MachinePoolSpec struct {
	// ClusterName is the name of the Cluster this object belongs to.
	// +kubebuilder:validation:MinLength=1
	ClusterName string `json:"clusterName"`

	// Bootstrap is a reference to a local struct which encapsulates
	// fields to configure the Machine’s bootstrapping mechanism.
	// Bootstrap Bootstrap `json:"bootstrap"`

	// InfrastructureRef is a required reference to a custom resource
	// offered by an infrastructure provider.
	// InfrastructureRef corev1.ObjectReference `json:"infrastructureRef"`

	// Version defines the desired Kubernetes version.
	// This field is meant to be optionally used by bootstrap providers.
	// +optional
	Version *string `json:"version,omitempty"`

	// ProviderID is the identification ID of the machine provided by the provider.
	// This field must match the provider ID as seen on the node object corresponding to this machine.
	// This field is required by higher level consumers of cluster-api. Example use case is cluster autoscaler
	// with cluster-api as provider. Clean-up logic in the autoscaler compares machines to nodes to find out
	// machines at provider which could not get registered as Kubernetes nodes. With cluster-api as a
	// generic out-of-tree provider for autoscaler, this field is required by autoscaler to be
	// able to have a provider view of the list of machines. Another list of nodes is queried from the k8s apiserver
	// and then a comparison is done to find out unregistered machines and are marked for delete.
	// This field will be set by the actuators and consumed by higher level entities like autoscaler that will
	// be interfacing with cluster-api as generic provider.
	// +optional
	ProviderID *string `json:"providerID,omitempty"`

	// FailureDomain is the failure domain the machine will be created in.
	// Must match a key in the FailureDomains map stored on the cluster object.
	// +optional
	FailureDomain *string `json:"failureDomain,omitempty"`
}

// ANCHOR_END: MachineSpec

// ANCHOR: MachineStatus

// MachineStatus defines the observed state of Machine
type MachinePoolStatus struct {
	// NodeRef will point to the corresponding Node if it exists.
	// +optional
	// NodeRef *corev1.ObjectReference `json:"nodeRef,omitempty"`

	// LastUpdated identifies when the phase of the Machine last transitioned.
	// +optional
	LastUpdated *metav1.Time `json:"lastUpdated,omitempty"`

	// Version specifies the current version of Kubernetes running
	// on the corresponding Node. This is meant to be a means of bubbling
	// up status from the Node to the Machine.
	// It is entirely optional, but useful for end-user UX if it’s present.
	// +optional
	Version *string `json:"version,omitempty"`

	// // FailureReason will be set in the event that there is a terminal problem
	// // reconciling the Machine and will contain a succinct value suitable
	// // for machine interpretation.
	// //
	// // This field should not be set for transitive errors that a controller
	// // faces that are expected to be fixed automatically over
	// // time (like service outages), but instead indicate that something is
	// // fundamentally wrong with the Machine's spec or the configuration of
	// // the controller, and that manual intervention is required. Examples
	// // of terminal errors would be invalid combinations of settings in the
	// // spec, values that are unsupported by the controller, or the
	// // responsible controller itself being critically misconfigured.
	// //
	// // Any transient errors that occur during the reconciliation of Machines
	// // can be added as events to the Machine object and/or logged in the
	// // controller's output.
	// // +optional
	// // FailureReason *capierrors.MachineStatusError `json:"failureReason,omitempty"`

	// // FailureMessage will be set    in the event that there is a terminal problem
	// // reconciling the Machine and will contain a more verbose string suitable
	// // for logging and human consumption.
	// //
	// // This field should not be set for transitive errors that a controller
	// // faces that are expected to be fixed automatically over
	// // time (like service outages), but instead indicate that something is
	// // fundamentally wrong with the Machine's spec or the configuration of
	// // the controller, and that manual intervention is required. Examples
	// // of terminal errors would be invalid combinations of settings in the
	// // spec, values that are unsupported by the controller, or the
	// // responsible controller itself being critically misconfigured.
	// //
	// // Any transient errors that occur during the reconciliation of Machines
	// // can be added as events to the Machine object and/or logged in the
	// // controller's output.
	// // +optional
	// FailureMessage *string `json:"failureMessage,omitempty"`

	// // Addresses is a list of addresses assigned to the machine.
	// // This field is copied from the infrastructure provider reference.
	// // +optional
	// Addresses MachineAddresses `json:"addresses,omitempty"`

	// // Phase represents the current phase of machine actuation.
	// // E.g. Pending, Running, Terminating, Failed etc.
	// // +optional
	// Phase string `json:"phase,omitempty"`

	// // BootstrapReady is the state of the bootstrap provider.
	// // +optional
	// BootstrapReady bool `json:"bootstrapReady"`

	// // InfrastructureReady is the state of the infrastructure provider.
	// // +optional
	// InfrastructureReady bool `json:"infrastructureReady"`

	// // ObservedGeneration is the latest generation observed by the controller.
	// // +optional
	// ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Conditions defines current service state of the Machine.
	// +optional
	// Conditions Conditions `json:"conditions,omitempty"`
}
