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

package asg

import (
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/pkg/errors"
	expinfrav1 "sigs.k8s.io/cluster-api-provider-aws/exp/api/v1alpha3"
	"sigs.k8s.io/cluster-api-provider-aws/pkg/cloud/awserrors"
	"sigs.k8s.io/cluster-api-provider-aws/pkg/cloud/converters"
	"sigs.k8s.io/cluster-api-provider-aws/pkg/cloud/scope"
	"sigs.k8s.io/cluster-api-provider-aws/pkg/record"
)

// SDKToAutoScalingGroup converts an AWS EC2 SDK AugoScalingGroup to the CAPA AugoScalingGroup type.
// SDKToAugoScalingGroup populates all AugoScalingGroup fields
func (s *Service) SDKToAutoScalingGroup(v *autoscaling.Group) (*expinfrav1.AutoScalingGroup, error) {
	i := &expinfrav1.AutoScalingGroup{
		ID:   aws.StringValue(v.AutoScalingGroupARN),
		Name: aws.StringValue(v.AutoScalingGroupName),
		//TODO: determine what additional values go here and what else should be in the struct
	}

	if v.Status != nil {
		i.Status = expinfrav1.ASGStatus(*v.Status)
	}

	if len(v.Tags) > 0 {
		i.Tags = converters.ASGTagsToMap(v.Tags)
	}

	return i, nil
}

//TODO: SDKToAutoScalingGroupInstance probably needs to be done as well
// func (s *Service) SDKToAutoScalingGroupInstance(v *autoscaling.Instance) (*expinfrav1.AutoScalingGroup, error) {
// 	i := &expinfrav1.AutoScalingGroupInstance
// 		ID: aws.StringValue(v.AutoScalingGroupName),
// 	}
// 	// Will likely be similar to SDKToInstance

// 	return i, nil
// }

// AsgIfExists returns the existing autoscaling group or nothing if it doesn't exist.
func (s *Service) AsgIfExists(name *string) (*expinfrav1.AutoScalingGroup, error) {
	if name == nil {
		s.scope.Info("Autoscaling Group does not have a name")
		return nil, nil
	}

	s.scope.Info("Looking for asg by name", "name", *name)

	input := &autoscaling.DescribeAutoScalingGroupsInput{
		AutoScalingGroupNames: []*string{name},
	}

	out, err := s.ASGClient.DescribeAutoScalingGroups(input)
	switch {
	case awserrors.IsNotFound(err):
		return nil, nil
	case err != nil:
		record.Eventf(s.scope.InfraCluster(), "FailedDescribeAutoScalingGroups", "failed to describe ASG %q: %v", *name, err)
		return nil, errors.Wrapf(err, "failed to describe AutoScaling Group: %q", *name)
	}
	//TODO: double check if you're handling nil vals
	return s.SDKToAutoScalingGroup(out.AutoScalingGroups[0])

}

// GetAsgByName returns the existing ASG or nothing if it doesn't exist.
func (s *Service) GetAsgByName(scope *scope.MachinePoolScope) (*expinfrav1.AutoScalingGroup, error) {
	s.scope.Info("Looking for existing machine instance by tags")

	input := &autoscaling.DescribeAutoScalingGroupsInput{
		AutoScalingGroupNames: []*string{
			aws.String(scope.Name()),
		},
	}

	out, err := s.ASGClient.DescribeAutoScalingGroups(input)
	switch {
	case awserrors.IsNotFound(err):
		return nil, nil
	case err != nil:
		record.Eventf(s.scope.InfraCluster(), "FailedDescribeInstances", "Failed to describe instances by tags: %v", err)
		return nil, errors.Wrap(err, "failed to describe instances by tags")
	case len(out.AutoScalingGroups) == 0:
		record.Eventf(scope.AWSMachinePool, "FailedDescribeInstances", "No Auto Scaling Groups with %s found", scope.Name())
		return nil, nil
	}

	return s.SDKToAutoScalingGroup(out.AutoScalingGroups[0])
}

// CreateASG runs an autoscaling group.
func (s *Service) CreateASG(scope *scope.MachinePoolScope) (*expinfrav1.AutoScalingGroup, error) {
	s.scope.Info("Creating an autoscaling group for a machine pool")

	input := &expinfrav1.AutoScalingGroup{
		Name:              scope.Name(), //TODO: define dynamically - borrow logic from ec2
		DesiredCapacity:   1,            //TODO: define elsewhere
		MaxSize:           5,            //TODO: Define for realsies later
		MinSize:           1,
		VPCZoneIdentifier: scope.AWSMachinePool.Spec.Subnets,
	}

	// TODO: do additional tags
	s.scope.Info("Running instance")
	_, err := s.runPool(input) //TODO: log out for more debugging
	if err != nil {
		// Only record the failure event if the error is not related to failed dependencies.
		// This is to avoid spamming failure events since the machine will be requeued by the actuator.
		// if !awserrors.IsFailedDependency(errors.Cause(err)) {
		// 	record.Warnf(scope.AWSMachinePool, "FailedCreate", "Failed to create instance: %v", err)
		// }
		s.scope.Error(err, "nopeee")
		return nil, err
	}
	record.Eventf(scope.AWSMachinePool, "SuccessfulCreate", "Created new ASG: %s", scope.Name)

	return nil, nil
}

func (s *Service) runPool(i *expinfrav1.AutoScalingGroup) (*expinfrav1.AutoScalingGroup, error) {
	input := &autoscaling.CreateAutoScalingGroupInput{
		AutoScalingGroupName: aws.String(i.Name),
		DesiredCapacity:      aws.Int64(i.DesiredCapacity),
		LaunchTemplate: &autoscaling.LaunchTemplateSpecification{
			LaunchTemplateName: aws.String(i.Name),
		},
		MaxSize:           aws.Int64(i.MaxSize),
		MinSize:           aws.Int64(i.MinSize),
		VPCZoneIdentifier: aws.String(strings.Join(i.VPCZoneIdentifier, ", ")),
	}

	s.scope.Info("Creating AutoScalingGroup")

	out, err := s.ASGClient.CreateAutoScalingGroup(input)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create autoscaling group")
	}
	s.scope.Info("", "myscope", out)

	// verify ASG was created

	return s.SDKToAutoScalingGroup(&autoscaling.Group{}) //TODO: fill with real one
}

func (s *Service) DeleteASGAndWait(name string) error {
	if err := s.DeleteASG(name); err != nil {
		return err
	}

	s.scope.V(2).Info("Waiting for ASG to be deleted", "name", name)

	input := &autoscaling.DescribeAutoScalingGroupsInput{
		AutoScalingGroupNames: aws.StringSlice([]string{name}),
	}

	if err := s.ASGClient.WaitUntilGroupNotExists(input); err != nil {
		return errors.Wrapf(err, "failed to wait for ASG %q deletion", name)
	}

	return nil
}

func (s *Service) DeleteASG(name string) error {
	s.scope.V(2).Info("Attempting to delete ASG", "name", name)

	input := &autoscaling.DeleteAutoScalingGroupInput{
		AutoScalingGroupName: aws.String(name),
		ForceDelete:          aws.Bool(true),
	}

	if _, err := s.ASGClient.DeleteAutoScalingGroup(input); err != nil {
		return errors.Wrapf(err, "failed to delete ASG %q", name)
	}

	s.scope.V(2).Info("Deleted ASG", "name", name)
	return nil
}
