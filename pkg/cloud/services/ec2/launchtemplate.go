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
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ec2"
	infrav1 "sigs.k8s.io/cluster-api-provider-aws/exp/api/v1alpha3"
)

// GetLaunchTemplate returns the existing LaunchTemplate or nothing if it doesn't exist.
func (s *Service) GetLaunchTemplate() (*infrav1.AwsLaunchTemplate, error) {
	s.scope.V(2).Info("Looking for existing LaunchTemplates")

	input := &ec2.DescribeLaunchTemplateVersionsInput{
		LaunchTemplateId: aws.String("lt-048eb7aa5b12cfd9d"),
	}

	result, err := s.scope.EC2.DescribeLaunchTemplateVersions(input)
	"sigs.k8s.io/cluster-api-provider-aws/pkg/cloud/scope"
)

// GetLaunchTemplate returns the existing LaunchTemplate or nothing if it doesn't exist.
// For now by name until we need the input to be something different
func (s *Service) GetLaunchTemplate(name string) (*infrav1.AwsLaunchTemplate, error) {
	s.scope.V(2).Info("Looking for existing LaunchTemplates")

	input := &ec2.DescribeLaunchTemplateVersionsInput{
		LaunchTemplateName: aws.String(name),
	}

	out, err := s.scope.EC2.DescribeLaunchTemplateVersions(input)
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

func (s *Service) CreateLaunchTemplate(scope *scope.MachinePoolScope) (*infrav1.AwsLaunchTemplate, error) {
	s.scope.Info("Create a new launch template")

	s.scope.Info(scope.Name())

	input := &ec2.CreateLaunchTemplateInput{
		LaunchTemplateData: &ec2.RequestLaunchTemplateData{
			ImageId:      aws.String(scope.AWSMachinePool.Spec.AwsLaunchTemplate.ImageId),
			InstanceType: aws.String(scope.AWSMachinePool.Spec.AwsLaunchTemplate.InstanceType),
			KeyName:      scope.AWSMachinePool.Spec.AwsLaunchTemplate.SSHKeyName,
		},
		LaunchTemplateName: aws.String(scope.Name()),
	}

	result, err := s.scope.EC2.CreateLaunchTemplate(input)

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
		s.scope.Info("oh man you have a problem", "error", err)
	}

	s.scope.Info("got it", "result", result)
	}

	s.scope.Info("Got it", "result", result.LaunchTemplate.LaunchTemplateName)

	return nil, nil
}

// // LaunchTemplateIfExists returns the existing LaunchTemplate or nothing if it doesn't exist.
// func (s *Service) LaunchTemplateIfExists(id *string) (*infrav1.LaunchTemplate, error) {
// 	if id == nil {
// 		s.scope.Info("LaunchTemplate does not have an LaunchTemplate id")
// 		return nil, nil
// 	}

// 	s.scope.V(2).Info("Looking for LaunchTemplate by id", "LaunchTemplate-id", *id)

// 	input := &ec2.DescribeLaunchTemplatesInput{
// 		LaunchTemplateIds: []*string{id},
// 	}

// 	out, err := s.scope.EC2.DescribeLaunchTemplates(input)
// 	switch {
// 	case awserrors.IsNotFound(err):
// 		return nil, nil
// 	case err != nil:
// 		record.Eventf(s.scope.AWSCluster, "FailedDescribeLaunchTemplates", "failed to describe LaunchTemplate %q: %v", *id, err)
// 		return nil, errors.Wrapf(err, "failed to describe LaunchTemplate: %q", *id)
// 	}

// 	if len(out.Reservations) > 0 && len(out.Reservations[0].LaunchTemplates) > 0 {
// 		return s.SDKToLaunchTemplate(out.Reservations[0].LaunchTemplates[0])
// 	}

// 	return nil, nil
// }

// // CreateLaunchTemplate runs an ec2 LaunchTemplate.
// func (s *Service) CreateLaunchTemplate(scope *scope.MachineScope, userData []byte) (*infrav1.LaunchTemplate, error) {
// 	s.scope.V(2).Info("Creating an LaunchTemplate for a machine")

// 	input := &infrav1.LaunchTemplate{
// 		Type:              scope.AWSMachine.Spec.LaunchTemplateType,
// 		IAMProfile:        scope.AWSMachine.Spec.IAMLaunchTemplateProfile,
// 		RootVolume:        scope.AWSMachine.Spec.RootVolume,
// 		NetworkInterfaces: scope.AWSMachine.Spec.NetworkInterfaces,
// 	}

// 	// Make sure to use the MachineScope here to get the merger of AWSCluster and AWSMachine tags
// 	additionalTags := scope.AdditionalTags()
// 	// Set the cloud provider tag
// 	additionalTags[infrav1.ClusterAWSCloudProviderTagKey(s.scope.Name())] = string(infrav1.ResourceLifecycleOwned)

// 	input.Tags = infrav1.Build(infrav1.BuildParams{
// 		ClusterName: s.scope.Name(),
// 		Lifecycle:   infrav1.ResourceLifecycleOwned,
// 		Name:        aws.String(scope.Name()),
// 		Role:        aws.String(scope.Role()),
// 		Additional:  additionalTags,
// 	})

// 	var err error
// 	// Pick image from the machine configuration, or use a default one.
// 	if scope.AWSMachine.Spec.AMI.ID != nil {
// 		input.ImageID = *scope.AWSMachine.Spec.AMI.ID
// 	} else {
// 		if scope.Machine.Spec.Version == nil {
// 			err := errors.New("Either AWSMachine's spec.ami.id or Machine's spec.version must be defined")
// 			scope.SetFailureReason(capierrors.CreateMachineError)
// 			scope.SetFailureMessage(err)
// 			return nil, err
// 		}

// 		imageLookupFormat := scope.AWSMachine.Spec.ImageLookupFormat
// 		if imageLookupFormat == "" {
// 			imageLookupFormat = scope.AWSCluster.Spec.ImageLookupFormat
// 		}

// 		imageLookupOrg := scope.AWSMachine.Spec.ImageLookupOrg
// 		if imageLookupOrg == "" {
// 			imageLookupOrg = scope.AWSCluster.Spec.ImageLookupOrg
// 		}

// 		imageLookupBaseOS := scope.AWSMachine.Spec.ImageLookupBaseOS
// 		if imageLookupBaseOS == "" {
// 			imageLookupBaseOS = scope.AWSCluster.Spec.ImageLookupBaseOS
// 		}

// 		input.ImageID, err = s.defaultAMILookup(imageLookupFormat, imageLookupOrg, imageLookupBaseOS, *scope.Machine.Spec.Version)
// 		if err != nil {
// 			return nil, err
// 		}
// 	}

// 	// Prefer AWSMachine.Spec.FailureDomain for now while migrating to the use of
// 	// Machine.Spec.FailureDomain. The MachineController will handle migrating the value for us.
// 	failureDomain := scope.AWSMachine.Spec.FailureDomain
// 	if failureDomain == nil {
// 		failureDomain = scope.Machine.Spec.FailureDomain
// 	}

// 	// Pick subnet from the machine configuration, or based on the availability zone specified,
// 	// or default to the first private subnet available.
// 	// TODO(vincepri): Move subnet picking logic to its own function/method.
// 	switch {
// 	case scope.AWSMachine.Spec.Subnet != nil && scope.AWSMachine.Spec.Subnet.ID != nil:
// 		input.SubnetID = *scope.AWSMachine.Spec.Subnet.ID

// 	case failureDomain != nil:
// 		subnets := s.scope.Subnets().FilterPrivate().FilterByZone(*failureDomain)
// 		if len(subnets) == 0 {
// 			record.Warnf(scope.AWSMachine, "FailedCreate",
// 				"Failed to create LaunchTemplate: no subnets available in availability zone %q", *failureDomain)

// 			return nil, awserrors.NewFailedDependency(
// 				errors.Errorf("failed to run machine %q, no subnets available in availability zone %q",
// 					scope.Name(),
// 					*failureDomain,
// 				),
// 			)
// 		}
// 		input.SubnetID = subnets[0].ID

// 		// TODO(vincepri): Define a tag that would allow to pick a preferred subnet in an AZ when working
// 		// with control plane machines.

// 	case input.SubnetID == "":
// 		sns := s.scope.Subnets().FilterPrivate()
// 		if len(sns) == 0 {
// 			record.Eventf(s.scope.AWSCluster, "FailedCreateLaunchTemplate", "Failed to run machine %q, no subnets available", scope.Name())
// 			return nil, awserrors.NewFailedDependency(
// 				errors.Errorf("failed to run machine %q, no subnets available", scope.Name()),
// 			)
// 		}
// 		input.SubnetID = sns[0].ID
// 	}

// 	if s.scope.Network().APIServerELB.DNSName == "" {
// 		record.Eventf(s.scope.AWSCluster, "FailedCreateLaunchTemplate", "Failed to run controlplane, APIServer ELB not available")
// 		return nil, awserrors.NewFailedDependency(
// 			errors.New("failed to run controlplane, APIServer ELB not available"),
// 		)
// 	}
// 	if !scope.UserDataIsUncompressed() {
// 		userData, err = userdata.GzipBytes(userData)
// 		if err != nil {
// 			return nil, errors.New("failed to gzip userdata")
// 		}
// 	}

// 	input.UserData = pointer.StringPtr(base64.StdEncoding.EncodeToString(userData))

// 	// Set security groups.
// 	ids, err := s.GetCoreSecurityGroups(scope)
// 	if err != nil {
// 		return nil, err
// 	}
// 	input.SecurityGroupIDs = append(input.SecurityGroupIDs, ids...)

// 	// If SSHKeyName WAS NOT provided in the AWSMachine Spec, fallback to the value provided in the AWSCluster Spec.
// 	// If a value was not provided in the AWSCluster Spec, then use the defaultSSHKeyName
// 	input.SSHKeyName = scope.AWSMachine.Spec.SSHKeyName
// 	if input.SSHKeyName == nil {
// 		if scope.AWSCluster.Spec.SSHKeyName != nil {
// 			input.SSHKeyName = scope.AWSCluster.Spec.SSHKeyName
// 		} else {
// 			input.SSHKeyName = aws.String(defaultSSHKeyName)
// 		}
// 	}

// 	s.scope.V(2).Info("Running LaunchTemplate", "machine-role", scope.Role())
// 	out, err := s.runLaunchTemplate(scope.Role(), input)
// 	if err != nil {
// 		// Only record the failure event if the error is not related to failed dependencies.
// 		// This is to avoid spamming failure events since the machine will be requeued by the actuator.
// 		if !awserrors.IsFailedDependency(errors.Cause(err)) {
// 			record.Warnf(scope.AWSMachine, "FailedCreate", "Failed to create LaunchTemplate: %v", err)
// 		}
// 		return nil, err
// 	}

// 	if len(input.NetworkInterfaces) > 0 {
// 		for _, id := range input.NetworkInterfaces {
// 			s.scope.V(2).Info("Attaching security groups to provided network interface", "groups", input.SecurityGroupIDs, "interface", id)
// 			if err := s.attachSecurityGroupsToNetworkInterface(input.SecurityGroupIDs, id); err != nil {
// 				return nil, err
// 			}
// 		}
// 	}

// 	record.Eventf(scope.AWSMachine, "SuccessfulCreate", "Created new %s LaunchTemplate with id %q", scope.Role(), out.ID)
// 	return out, nil
// }
// SDKToLaunchTemplate converts an AWS EC2 SDK instance to the CAPA instance type.
func (s *Service) SDKToLaunchTemplate(d *ec2.LaunchTemplateVersion) (*infrav1.AwsLaunchTemplate, error) {
	v := d.LaunchTemplateData
	i := &infrav1.AwsLaunchTemplate{
		ImageId:       aws.StringValue(v.ImageId),
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
		tmp := &infrav1.BlockDeviceMapping{
			DeviceName: *bdm.DeviceName,
			Ebs: infrav1.EBS{
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
		tmp := &infrav1.NetworkInterface{
			DeviceIndex: *ni.DeviceIndex,
			Groups:      s,
		}
		i.NetworkInterfaces = append(i.NetworkInterfaces, *tmp)
	}

	return i, nil
}
