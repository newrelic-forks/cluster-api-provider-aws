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

// Tags
type Tags map[string]string

// AutoScalingGroup describes an AWS autoscaling group.
type AutoScalingGroup struct {
	// The tags associated with the instance.
	ID                string            `json:"id,omitempty"`
	Tags              map[string]string `json:"tags,omitempty"`
	Name              string            `json:"name,omitempty"`
	DesiredCapacity   int32             `json:"desiredCapacity,omitempty"`
	MaxSize           int32             `json:"maxSize,omitempty"`
	MinSize           int32             `json:"minSize,omitempty"`
	PlacementGroup    string            `json:"placementGroup,omitempty"`
	VPCZoneIdentifier []string          `json:"vpcZoneIdentifier,omitempty"`

	Status    ASGStatus
	Instances []infrav1.Instance `json:"instances,omitempty"`

	// todo
	// MixedInstancesPolicy        *autoscaling.MixedInstancesPolicy
	// LaunchTemplateSpecification *autoscaling.LaunchTemplateSpecification
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
