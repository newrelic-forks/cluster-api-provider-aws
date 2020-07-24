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

package ec2

import (
	"encoding/base64"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/pkg/errors"
	"k8s.io/utils/pointer"
	infrav1 "sigs.k8s.io/cluster-api-provider-aws/api/v1alpha3"
	expinfrav1 "sigs.k8s.io/cluster-api-provider-aws/exp/api/v1alpha3"
	"sigs.k8s.io/cluster-api-provider-aws/pkg/cloud/scope"
)

// GetLaunchTemplate returns the existing LaunchTemplate or nothing if it doesn't exist.
// For now by name until we need the input to be something different
func (s *Service) GetLaunchTemplate(name string) (*expinfrav1.AWSLaunchTemplate, error) {
	s.scope.V(2).Info("Looking for existing LaunchTemplates")

	input := &ec2.DescribeLaunchTemplateVersionsInput{
		LaunchTemplateName: aws.String(name),
	}

	out, err := s.EC2Client.DescribeLaunchTemplateVersions(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			default:
				s.scope.Info("", "aerr", aerr.Error())
			}
		} else {
			// Print the error, cast err to awserr.Error to get the Code and
			// Message from an error.
			s.scope.Info("", "error", err.Error())
		}
	}

	for _, version := range out.LaunchTemplateVersions {
		return s.SDKToLaunchTemplate(version)
	}

	return nil, nil
}

// CreateLaunchTemplate generates a launch template to be used with the autoscaling group
func (s *Service) CreateLaunchTemplate(scope *scope.MachinePoolScope, userData []byte) (*expinfrav1.AWSLaunchTemplate, error) {
	s.scope.Info("Create a new launch template")
	s.scope.Info("UserData", "UserData", string(userData))
	s.scope.Info(scope.Name())

	input := &ec2.CreateLaunchTemplateInput{
		LaunchTemplateData: &ec2.RequestLaunchTemplateData{
			ImageId:      scope.AWSMachinePool.Spec.AWSLaunchTemplate.AMI.ID,
			InstanceType: aws.String(scope.AWSMachinePool.Spec.AWSLaunchTemplate.InstanceType),
			IamInstanceProfile: &ec2.LaunchTemplateIamInstanceProfileSpecificationRequest{
				Name: aws.String(scope.AWSMachinePool.Spec.AWSLaunchTemplate.IamInstanceProfile),
			},
			KeyName:  scope.AWSMachinePool.Spec.AWSLaunchTemplate.SSHKeyName,
			UserData: pointer.StringPtr(base64.StdEncoding.EncodeToString(userData)),
		},
		LaunchTemplateName: aws.String(scope.Name()),
	}

	additionalTags := scope.AdditionalTags()
	// Set the cloud provider tag
	additionalTags[infrav1.ClusterAWSCloudProviderTagKey(s.scope.Name())] = string(infrav1.ResourceLifecycleOwned)

	tags := infrav1.Build(infrav1.BuildParams{
		ClusterName: s.scope.Name(),
		Lifecycle:   infrav1.ResourceLifecycleOwned,
		Name:        aws.String(scope.Name()),
		Role:        aws.String("node"),
		Additional:  additionalTags,
	})

	if len(tags) > 0 {
		spec := &ec2.LaunchTemplateTagSpecificationRequest{ResourceType: aws.String(ec2.ResourceTypeInstance)}
		for key, value := range tags {
			spec.Tags = append(spec.Tags, &ec2.Tag{
				Key:   aws.String(key),
				Value: aws.String(value),
			})
		}
		input.LaunchTemplateData.TagSpecifications = append(input.LaunchTemplateData.TagSpecifications, spec)
	}

	ids, err := s.GetCoreNodeSecurityGroups()
	if err != nil {
		return nil, err
	}

	for _, id := range ids {
		s.scope.Info(id)
		input.LaunchTemplateData.SecurityGroupIds = append(input.LaunchTemplateData.SecurityGroupIds, aws.String(id))
	}

	// add additional security groups as well
	for _, additionalGroup := range scope.AWSMachinePool.Spec.AdditionalSecurityGroups {
		input.LaunchTemplateData.SecurityGroupIds = append(input.LaunchTemplateData.SecurityGroupIds, additionalGroup.ID)
	}
	s.scope.Info("Security Groups", "security groups", input.LaunchTemplateData.SecurityGroupIds)

	result, err := s.EC2Client.CreateLaunchTemplate(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			default:
				s.scope.Info("", "aerr", aerr.Error())
			}
		} else {
			// Print the error, cast err to awserr.Error to get the Code and
			// Message from an error.
			s.scope.Info("", "error", err.Error())
		}
	}

	s.scope.Info("Got it", "result", result.LaunchTemplate.LaunchTemplateName)

	return nil, nil
}

// DeleteLaunchTemplate delete a launch template
func (s *Service) DeleteLaunchTemplate(id string) error {
	s.scope.V(2).Info("Deleting launch template", "id", id)

	input := &ec2.DeleteLaunchTemplateInput{
		LaunchTemplateId: aws.String(id),
	}

	if _, err := s.EC2Client.DeleteLaunchTemplate(input); err != nil {
		return errors.Wrapf(err, "failed to delete launch template %q", id)
	}

	s.scope.V(2).Info("Deleted launch template", "id", id)
	return nil
}

// SDKToLaunchTemplate converts an AWS EC2 SDK instance to the CAPA instance type.
func (s *Service) SDKToLaunchTemplate(d *ec2.LaunchTemplateVersion) (*expinfrav1.AWSLaunchTemplate, error) {
	v := d.LaunchTemplateData
	i := &expinfrav1.AWSLaunchTemplate{
		ID:   aws.StringValue(d.LaunchTemplateId),
		Name: aws.StringValue(d.LaunchTemplateName),
		AMI: infrav1.AWSResourceReference{
			ID: v.ImageId,
		},
		InstanceType:  aws.StringValue(v.InstanceType),
		SSHKeyName:    v.KeyName,
		VersionNumber: d.VersionNumber,
	}

	// Extract IAM Instance Profile name from ARN
	if v.IamInstanceProfile != nil && v.IamInstanceProfile.Arn != nil {
		split := strings.Split(aws.StringValue(v.IamInstanceProfile.Arn), "instance-profile/")
		if len(split) > 1 && split[1] != "" {
			i.IamInstanceProfile = split[1]
		}
	}

	for _, bdm := range v.BlockDeviceMappings {
		tmp := &expinfrav1.BlockDeviceMapping{
			DeviceName: *bdm.DeviceName,
			Ebs: expinfrav1.EBS{
				Encrypted:  *bdm.Ebs.Encrypted,
				VolumeSize: *bdm.Ebs.VolumeSize,
				VolumeType: *bdm.Ebs.VolumeType,
			},
		}
		i.BlockDeviceMappings = append(i.BlockDeviceMappings, *tmp)
	}

	for _, ni := range v.NetworkInterfaces {
		var s []string
		for _, groups := range ni.Groups {
			s = append(s, *groups)
		}
		tmp := &expinfrav1.NetworkInterface{
			DeviceIndex: *ni.DeviceIndex,
			Groups:      s,
		}
		i.NetworkInterfaces = append(i.NetworkInterfaces, *tmp)
	}

	return i, nil
}
