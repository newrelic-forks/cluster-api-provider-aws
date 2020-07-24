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
	infrav1 "sigs.k8s.io/cluster-api-provider-aws/api/v1alpha3"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
)

const (
	MachinePoolFinalizer = "awsmachinepool.infrastructure.cluster.x-k8s.io"
)

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
	AdditionalTags                   infrav1.Tags         `json:"additionalTags,omitempty"`
	TerminationPolicies              []string             `json:"terminationPolicies,omitempty"`
	NewInstancesProtectedFromScaleIn bool                 `json:"newInstancesProtectedFromScaleIn,omitempty"`
	ServiceLinkedRoleARN             string               `json:"serviceLinkedRoleARN,omitempty"`
	AWSLaunchTemplate                AWSLaunchTemplate    `json:"awsLaunchTemplate,omitempty"`

	// AdditionalSecurityGroups is an array of references to security groups that should be applied to the
	// instance. These security groups would be set in addition to any security groups defined
	// at the cluster level or in the actuator.
	// +optional
	AdditionalSecurityGroups []infrav1.AWSResourceReference `json:"additionalSecurityGroups,omitempty"`
	Subnets                  []string                       `json:"subnets,omitempty"`
}

// AWSMachinePoolStatus defines the observed state of AWSMachinePool
type AWSMachinePoolStatus struct {
	AutoScalingGroupARN string               `json:"autoScalingGroupARN,omitempty"`
	Conditions          clusterv1.Conditions `json:"conditions,omitempty"`
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

func (r *AWSMachinePool) GetConditions() clusterv1.Conditions {
	return r.Status.Conditions
}

func (r *AWSMachinePool) SetConditions(conditions clusterv1.Conditions) {
	r.Status.Conditions = conditions
}
