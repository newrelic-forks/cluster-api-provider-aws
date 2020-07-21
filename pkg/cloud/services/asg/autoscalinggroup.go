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
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/pkg/errors"
	infrav1 "sigs.k8s.io/cluster-api-provider-aws/api/v1alpha3"
	"sigs.k8s.io/cluster-api-provider-aws/pkg/cloud/awserrors"
	"sigs.k8s.io/cluster-api-provider-aws/pkg/cloud/scope"
	"sigs.k8s.io/cluster-api-provider-aws/pkg/record"
)

// SDKToAutoScalingGroup converts an AWS EC2 SDK AugoScalingGroup to the CAPA AugoScalingGroup type.
// SDKToAugoScalingGroup populates all AugoScalingGroup fields
func (s *Service) SDKToAutoScalingGroup(v *autoscaling.Group) (*infrav1.AutoScalingGroup, error) {
	i := &infrav1.AutoScalingGroup{
		ID: aws.StringValue(v.AutoScalingGroupName),
		//TODO: determine what additional values go here and what else should be in the struct
	}

	return i, nil
}

//TODO: SDKToAutoScalingGroupInstance probably needs to be done as well
// func (s *Service) SDKToAutoScalingGroupInstance(v *autoscaling.Instance) (*infrav1.AutoScalingGroup, error) {
// 	i := &infrav1.AutoScalingGroupInstance
// 		ID: aws.StringValue(v.AutoScalingGroupName),
// 	}
// 	// Will likely be similar to SDKToInstance

// 	return i, nil
// }

// AsgIfExists returns the existing autoscaling group or nothing if it doesn't exist.
func (s *Service) AsgIfExists(name *string) (*infrav1.AutoScalingGroup, error) {
	if name == nil {
		s.scope.Info("Autoscaling Group does not have a name")
		return nil, nil
	}

	s.scope.V(2).Info("Looking for asg by name", "name", *name)

	input := &autoscaling.DescribeAutoScalingGroupsInput{
		AutoScalingGroupNames: []*string{name},
	}

	out, err := s.scope.ASG.DescribeAutoScalingGroups(input)
	switch {
	case awserrors.IsNotFound(err):
		return nil, nil
	case err != nil:
		record.Eventf(s.scope.AWSCluster, "FailedDescribeAutoScalingGroups", "failed to describe ASG %q: %v", *name, err)
		return nil, errors.Wrapf(err, "failed to describe AutoScaling Group: %q", *name)
	}
	//TODO: double check if you're handling nil vals
	return s.SDKToAutoScalingGroup(out.AutoScalingGroups[0])

}

// GetRunningAsgByName returns the existing ASG or nothing if it doesn't exist.
func (s *Service) GetRunningAsgByName(scope *scope.MachinePoolScope) (*infrav1.AutoScalingGroup, error) {
	s.scope.V(2).Info("Looking for existing machine instance by tags")

	input := &autoscaling.DescribeAutoScalingGroupsInput{
		AutoScalingGroupNames: []*string{
			aws.String(scope.Name()),
		},
	}

	out, err := s.scope.ASG.DescribeAutoScalingGroups(input)
	switch {
	case awserrors.IsNotFound(err):
		return nil, nil
	case err != nil:
		record.Eventf(s.scope.AWSCluster, "FailedDescribeInstances", "Failed to describe instances by tags: %v", err)
		return nil, errors.Wrap(err, "failed to describe instances by tags")
	}

	return s.SDKToAutoScalingGroup(out.AutoScalingGroups[0])
}

// CreateASG runs an autoscaling group.
func (s *Service) CreateASG(scope *scope.MachinePoolScope) (*infrav1.AutoScalingGroup, error) {
	s.scope.V(2).Info("Creating an autoscaling group for a machine pool")

	input := &infrav1.AutoScalingGroup{
		AutoScalingGroupName: "nicole-testy-westy", //TODO: define dynamically - borrow logic from ec2
		DesiredCapacity:      1,                    //TODO: define elsewhere
		LaunchTemplateSpecification: &autoscaling.LaunchTemplateSpecification{
			LaunchTemplateName: aws.String("mytu-test"),
		}, //TODO: get from mytu's code, remove hard code val, get machinepool.go
		MaxSize:              5, //TODO: Define for realsies later
		MinSize:              1,
		MixedInstancesPolicy: &autoscaling.MixedInstancesPolicy{},
	}

	// TODO: do additional tags
	s.scope.V(2).Info("Running instance")
	_, err := s.runPool(input) //TODO: log out for more debugging
	if err != nil {
		// Only record the failure event if the error is not related to failed dependencies.
		// This is to avoid spamming failure events since the machine will be requeued by the actuator.
		if !awserrors.IsFailedDependency(errors.Cause(err)) {
			record.Warnf(scope.AWSMachinePool, "FailedCreate", "Failed to create instance: %v", err)
		}
		return nil, err
	}
	record.Eventf(scope.AWSMachinePool, "SuccessfulCreate", "Created new ASG: %s", scope.Name)

	return nil, nil
}

func (s *Service) runPool(i *infrav1.AutoScalingGroup) (*infrav1.AutoScalingGroup, error) {
	input := &autoscaling.CreateAutoScalingGroupInput{
		AutoScalingGroupName: aws.String(i.AutoScalingGroupName),
		DesiredCapacity:      aws.Int64(i.DesiredCapacity),
		LaunchTemplate:       i.LaunchTemplateSpecification,
		MaxSize:              aws.Int64(i.MaxSize),
		MinSize:              aws.Int64(i.MinSize),
		MixedInstancesPolicy: i.MixedInstancesPolicy,
	}

	_, err := s.scope.ASG.CreateAutoScalingGroup(input)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create autoscaling group")
	}
	// verify ASG was created

	return s.SDKToAutoScalingGroup(&autoscaling.Group{}) //TODO: fill with real one
}
