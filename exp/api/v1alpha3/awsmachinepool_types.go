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

package v1alpha3

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	infrav1 "sigs.k8s.io/cluster-api-provider-aws/api/v1alpha3"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/cluster-api/errors"
)

const (
	MachinePoolFinalizer        = "awsmachinepool.infrastructure.cluster.x-k8s.io"
	LaunchTemplateLatestVersion = "$Latest"
)

// AWSMachinePoolSpec defines the desired state of AWSMachinePool
type AWSMachinePoolSpec struct {
	ProviderID           string            `json:"providerID,omitempty"` //TODO: is this needed?
	AutoScalingGroupName string            `json:"autoScalingGroupName,omitempty"`
	MinSize              int32             `json:"minSize,omitempty"`
	MaxSize              int32             `json:"maxSize,omitempty"`
	DesiredCapacity      int32             `json:"desiredCapacity,omitempty"`
	AvailabilityZones    []string          `json:"availabilityZones,omitempty"`
	Subnets              []string          `json:"subnets,omitempty"`
	AdditionalTags       infrav1.Tags      `json:"additionalTags,omitempty"`
	AWSLaunchTemplate    AWSLaunchTemplate `json:"awsLaunchTemplate,omitempty"`

	// MixedInstancesPolicy describes how multiple instance types will be used by the ASG.
	MixedInstancesPolicy *MixedInstancesPolicy `json:"mixedInstancesPolicy,omitempty"`

	// ProviderIDList are the identification IDs of machine instances provided by the provider.
	// This field must match the provider IDs as seen on the node objects corresponding to a machine pool's machine instances.
	// +optional
	ProviderIDList []string `json:"providerIDList,omitempty"`
}

// AWSMachinePoolStatus defines the observed state of AWSMachinePool
type AWSMachinePoolStatus struct {
	// Ready is true when the provider resource is ready.
	// +optional
	Ready bool `json:"ready"`

	// Replicas is the most recently observed number of replicas
	// +optional
	Replicas int32 `json:"replicas"`

	AutoScalingGroupARN string               `json:"autoScalingGroupARN,omitempty"`
	Conditions          clusterv1.Conditions `json:"conditions,omitempty"`
	LaunchTemplateID    string               `json:"launchTemplateID,omitempty"`

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

	ASGStatus *ASGStatus `json:"asgStatus,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=awsmachinepools,scope=Namespaced,categories=cluster-api
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.ready",description="Machine ready status"
// +kubebuilder:printcolumn:name="Replicas",type="integer",JSONPath=".status.replicas",description="Machine ready status"
// +kubebuilder:printcolumn:name="LaunchTemplate ID",type="string",JSONPath=".status.launchTemplateID",description="Launch Template ID"

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

func (r *AWSMachinePool) GetConditions() clusterv1.Conditions {
	return r.Status.Conditions
}

func (r *AWSMachinePool) SetConditions(conditions clusterv1.Conditions) {
	r.Status.Conditions = conditions
}

func (obj *AWSMachinePool) GetObjectKind() schema.ObjectKind {
	return &obj.TypeMeta
}

func (obj *AWSMachinePoolList) GetObjectKind() schema.ObjectKind {
	return &obj.TypeMeta
}
