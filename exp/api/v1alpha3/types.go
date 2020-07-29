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
	"k8s.io/apimachinery/pkg/util/sets"
	infrav1 "sigs.k8s.io/cluster-api-provider-aws/api/v1alpha3"
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
type AWSLaunchTemplate struct {
	// all the things needed for a launch template
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`

	IamInstanceProfile string             `json:"iamInstanceProfile,omitempty"`
	NetworkInterfaces  []NetworkInterface `json:"networkInterfaces,omitempty"`

	// todo: use a helper
	AMI infrav1.AWSResourceReference `json:"ami,omitempty"`

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

	// RootVolume encapsulates the configuration options for the root volume
	// +optional
	RootVolume *infrav1.RootVolume `json:"rootVolume,omitempty"`

	// SSHKeyName is the name of the ssh key to attach to the instance. Valid values are empty string (do not use SSH keys), a valid SSH key name, or omitted (use the default SSH key name)
	// +optional
	SSHKeyName *string `json:"sshKeyName,omitempty"`

	VersionNumber *int64 `json:"versionNumber,omitempty"`
}

// Overrides from describe-auto-scaling-groups
type Overrides struct {
	InstanceType string `json:"instanceType"`
}

// InstancesDistribution from describe-auto-scaling-groups
type InstancesDistribution struct {
	// +kubebuilder:validation:Enum=prioritized
	OnDemandAllocationStrategy *string `json:"onDemandAllocationStrategy,omitempty"`

	// +kubebuilder:validation:Enum=lowest-price;capacity-optimized
	SpotAllocationStrategy *string `json:"spotAllocationStrategy,omitempty"`

	OnDemandBaseCapacity                *int64 `json:"onDemandBaseCapacity,omitempty"`
	OnDemandPercentageAboveBaseCapacity *int64 `json:"onDemandPercentageAboveBaseCapacity,omitempty"`
}

// MixedInstancesPolicy from describe-auto-scaling-groups
type MixedInstancesPolicy struct {
	InstancesDistribution *InstancesDistribution `json:"instancesDistribution,omitempty"`
	Overrides             []Overrides            `json:"overrides,omitempty"`
}

// Tags
type Tags map[string]string

// AutoScalingGroup describes an AWS autoscaling group.
type AutoScalingGroup struct {
	// The tags associated with the instance.
	ID              string            `json:"id,omitempty"`
	Tags            map[string]string `json:"tags,omitempty"`
	Name            string            `json:"name,omitempty"`
	DesiredCapacity *int32            `json:"desiredCapacity,omitempty"`
	MaxSize         int32             `json:"maxSize,omitempty"`
	MinSize         int32             `json:"minSize,omitempty"`
	PlacementGroup  string            `json:"placementGroup,omitempty"`
	Subnets         []string          `json:"subnets,omitempty"`

	MixedInstancesPolicy *MixedInstancesPolicy `json:"mixedInstancesPolicy,omitempty"`
	State                ASGState              `json:"asgState,omitempty"` //TODO: Is this the same as status?
	Status               ASGStatus
	Instances            []infrav1.Instance `json:"instances,omitempty"`
}

// ASGStatus is a status string returned by the autoscaling API
type ASGStatus string

var (
	// ASGStatusDeleteInProgress is the string representing an ASG that is currently deleting
	ASGStatusDeleteInProgress = ASGStatus("Delete in progress")
)

// launchTemplateNeedsUpdate checks if a new launch template version is needed
func LaunchTemplateNeedsUpdate(incoming *AWSLaunchTemplate, existing *AWSLaunchTemplate) bool {
	if incoming.IamInstanceProfile != existing.IamInstanceProfile {
		return true
	}

	if incoming.InstanceType != existing.InstanceType {
		return true
	}

	// todo: security groups
	// todo: block devices
	// todo: more fields

	return false
}

// ASGState contains the state of the ASG. e.g. pending, running, etc
type ASGState string

//TODO: maybe make it match this more? https://docs.aws.amazon.com/autoscaling/ec2/userguide/AutoScalingGroupLifecycle.html
var (
	// ASGStatePending is the string representing an ASG in a pending state
	ASGStatePending = ASGState("pending")

	// ASGStateRunning is the string representing an ASG in a pending state
	ASGStateRunning = ASGState("running")

	// ASGStateShuttingDown is the string representing an ASG shutting down
	ASGStateShuttingDown = ASGState("shutting-down")

	// ASGStateTerminated is the string representing an ASG that has been terminated
	ASGStateTerminated = ASGState("terminated")

	// ASGStateStopping is the string representing an ASG
	// that is in the process of being stopped and can be restarted
	ASGStateStopping = ASGState("stopping")

	// ASGStateStopped is the string representing an ASG
	// that has been stopped and can be restarted
	ASGStateStopped = ASGState("stopped")

	// ASGRunningStates defines the set of states in which an EC2 ASG is
	// running or going to be running soon
	ASGRunningStates = sets.NewString(
		string(ASGStatePending),
		string(ASGStateRunning),
	)

	// ASGOperationalStates defines the set of states in which an EC2 ASG is
	// or can return to running, and supports all EC2 operations
	ASGOperationalStates = ASGRunningStates.Union(
		sets.NewString(
			string(ASGStateStopping),
			string(ASGStateStopped),
		),
	)

	// ASGKnownStates represents all known ASG states
	ASGKnownStates = ASGOperationalStates.Union(
		sets.NewString(
			string(ASGStateShuttingDown),
			string(ASGStateTerminated),
		),
	)
)
