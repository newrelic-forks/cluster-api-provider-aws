/*
Copyright 2019 The Kubernetes Authors.

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
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/cluster-api/errors"
)

//TODO: Put in experimental
type AWSMachinePool struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AWSMachinePoolSpec   `json:"spec,omitempty"`
	Status AWSMachinePoolStatus `json:"status,omitempty"`
}

// AWSMachinePoolSpec defines the desired state of AWSMachine
type AWSMachinePoolSpec struct {
	// ProviderID is the unique identifier as specified by the cloud provider.
	ProviderID *string `json:"providerID,omitempty"`

	// AMI is the reference to the AMI from which to create the machine instance.
	AMI AWSResourceReference `json:"ami,omitempty"`

	// ImageLookupFormat is the AMI naming format to look up the image for this
	// machine It will be ignored if an explicit AMI is set. Supports
	// substitutions for {{.BaseOS}} and {{.K8sVersion}} with the base OS and
	// kubernetes version, respectively. The BaseOS will be the value in
	// ImageLookupBaseOS or ubuntu (the default), and the kubernetes version as
	// defined by the packages produced by kubernetes/release without v as a
	// prefix: 1.13.0, 1.12.5-mybuild.1, or 1.17.3. For example, the default
	// image format of capa-ami-{{.BaseOS}}-?{{.K8sVersion}}-* will end up
	// searching for AMIs that match the pattern capa-ami-ubuntu-?1.18.0-* for a
	// Machine that is targeting kubernetes v1.18.0 and the ubuntu base OS. See
	// also: https://golang.org/pkg/text/template/
	// +optional
	ImageLookupFormat string `json:"imageLookupFormat,omitempty"`

	// ImageLookupOrg is the AWS Organization ID to use for image lookup if AMI is not set.
	ImageLookupOrg string `json:"imageLookupOrg,omitempty"`

	// ImageLookupBaseOS is the name of the base operating system to use for
	// image lookup the AMI is not set.
	ImageLookupBaseOS string `json:"imageLookupBaseOS,omitempty"`

	// InstanceType is the type of instance to create. Example: m4.xlarge
	InstanceType string `json:"instanceType,omitempty"`

	// AdditionalTags is an optional set of tags to add to an instance, in addition to the ones added by default by the
	// AWS provider. If both the AWSCluster and the AWSMachine specify the same tag name with different values, the
	// AWSMachine's value takes precedence.
	// +optional
	AdditionalTags Tags `json:"additionalTags,omitempty"`

	// IAMInstanceProfile is a name of an IAM instance profile to assign to the instance
	// +optional
	IAMInstanceProfile string `json:"iamInstanceProfile,omitempty"`

	// PublicIP specifies whether the instance should get a public IP.
	// Precedence for this setting is as follows:
	// 1. This field if set
	// 2. Cluster/flavor setting
	// 3. Subnet default
	// +optional
	PublicIP *bool `json:"publicIP,omitempty"`

	// AdditionalSecurityGroups is an array of references to security groups that should be applied to the
	// instance. These security groups would be set in addition to any security groups defined
	// at the cluster level or in the actuator.
	// +optional
	AdditionalSecurityGroups []AWSResourceReference `json:"additionalSecurityGroups,omitempty"`

	// FailureDomain is the failure domain unique identifier this Machine should be attached to, as defined in Cluster API.
	// For this infrastructure provider, the ID is equivalent to an AWS Availability Zone.
	// If multiple subnets are matched for the availability zone, the first one returned is picked.
	FailureDomain *string `json:"failureDomain,omitempty"`

	// Subnet is a reference to the subnet to use for this instance. If not specified,
	// the cluster subnet will be used.
	// +optional
	Subnet *AWSResourceReference `json:"subnet,omitempty"`

	// SSHKeyName is the name of the ssh key to attach to the instance. Valid values are empty string (do not use SSH keys), a valid SSH key name, or omitted (use the default SSH key name)
	// +optional
	SSHKeyName *string `json:"sshKeyName,omitempty"`

	// RootVolume encapsulates the configuration options for the root volume
	// +optional
	RootVolume *RootVolume `json:"rootVolume,omitempty"`

	// NetworkInterfaces is a list of ENIs to associate with the instance.
	// A maximum of 2 may be specified.
	// +optional
	// +kubebuilder:validation:MaxItems=2
	NetworkInterfaces []string `json:"networkInterfaces,omitempty"`

	// UncompressedUserData specify whether the user data is gzip-compressed before it is sent to ec2 instance.
	// cloud-init has built-in support for gzip-compressed user data
	// user data stored in aws secret manager is always gzip-compressed.
	//
	// +optional
	UncompressedUserData *bool `json:"uncompressedUserData,omitempty"`

	// CloudInit defines options related to the bootstrapping systems where
	// CloudInit is used.
	// +optional
	CloudInit CloudInit `json:"cloudInit,omitempty"`
}

// AWSMachinePoolStatus defines the observed state of AWSMachine
type AWSMachinePoolStatus struct {
	// Ready is true when the provider resource is ready.
	// +optional
	Ready bool `json:"ready"`

	// Addresses contains the AWS instance associated addresses.
	Addresses []clusterv1.MachineAddress `json:"addresses,omitempty"`

	// InstanceState is the state of the AWS instance for this machine.
	// +optional
	InstanceState *InstanceState `json:"instanceState,omitempty"`

	// FailureReason will be set in the event that there is a terminal problem
	// reconciling the Machine and will contain a succinct value suitable
	// for machine interpretation.
	//
	// This field should not be set for transitive errors that a controller
	// faces that are expected to be fixed automatically over
	// time (like service outages), but instead indicate that something is
	// fundamentally wrong with the Machine's spec or the configuration of
	// the controller, and that manual intervention is required. Examples
	// of terminal errors would be invalid combinations of settings in the
	// spec, values that are unsupported by the controller, or the
	// responsible controller itself being critically misconfigured.
	//
	// Any transient errors that occur during the reconciliation of Machines
	// can be added as events to the Machine object and/or logged in the
	// controller's output.
	// +optional
	FailureReason *errors.MachineStatusError `json:"failureReason,omitempty"`

	// FailureMessage will be set in the event that there is a terminal problem
	// reconciling the Machine and will contain a more verbose string suitable
	// for logging and human consumption.
	//
	// This field should not be set for transitive errors that a controller
	// faces that are expected to be fixed automatically over
	// time (like service outages), but instead indicate that something is
	// fundamentally wrong with the Machine's spec or the configuration of
	// the controller, and that manual intervention is required. Examples
	// of terminal errors would be invalid combinations of settings in the
	// spec, values that are unsupported by the controller, or the
	// responsible controller itself being critically misconfigured.
	//
	// Any transient errors that occur during the reconciliation of Machines
	// can be added as events to the Machine object and/or logged in the
	// controller's output.
	// +optional
	FailureMessage *string `json:"failureMessage,omitempty"`

	// Conditions defines current service state of the AWSMachine.
	// +optional
	Conditions clusterv1.Conditions `json:"conditions,omitempty"`
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

	// // FailureMessage will be set in the event that there is a terminal problem
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
