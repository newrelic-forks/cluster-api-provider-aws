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
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/pkg/errors"
	infrav1 "sigs.k8s.io/cluster-api-provider-aws/api/v1alpha3"
	"sigs.k8s.io/cluster-api-provider-aws/pkg/cloud/awserrors"
	"sigs.k8s.io/cluster-api-provider-aws/pkg/record"
)

// AsgIfExists returns the existing autoscaling group or nothing if it doesn't exist.
func (s *Service) AsgIfExists(id *string) (*infrav1.AutoScalingGroup, error) {
	if id == nil {
		s.scope.Info("Instance does not have an instance id")
		return nil, nil
	}

	s.scope.V(2).Info("Looking for instance by id", "instance-id", *id)

	input := &ec2.DescribeInstancesInput{
		InstanceIds: []*string{id},
	}

	out, err := s.scope.EC2.DescribeInstances(input)
	switch {
	case awserrors.IsNotFound(err):
		return nil, nil
	case err != nil:
		record.Eventf(s.scope.AWSCluster, "FailedDescribeInstances", "failed to describe instance %q: %v", *id, err)
		return nil, errors.Wrapf(err, "failed to describe instance: %q", *id)
	}

	if len(out.Reservations) > 0 && len(out.Reservations[0].Instances) > 0 {
		return s.SDKToInstance(out.Reservations[0].Instances[0])
	}

	return nil, nil
}
