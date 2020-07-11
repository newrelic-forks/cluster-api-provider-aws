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

package eks

import (
	"fmt"

	"github.com/pkg/errors"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/eks"

	infrav1 "sigs.k8s.io/cluster-api-provider-aws/api/v1alpha3"
	"sigs.k8s.io/cluster-api-provider-aws/pkg/cloud/converters"
)

var (
	// ErrNoSecurityGroup is an error when no security group is found for an EKS cluster
	ErrNoSecurityGroup = errors.New("no security group for EKS cluster")
)

func (s *Service) reconcileSecurityGroup(cluster *eks.Cluster) error {
	s.scope.Info("Reconciling control plane security group", "cluster-name", *cluster.Name)

	input := &ec2.DescribeSecurityGroupsInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("tag:aws:eks:cluster-name"),
				Values: []*string{cluster.Name},
			},
		},
	}

	output, err := s.EC2Client.DescribeSecurityGroups(input)
	if err != nil {
		return fmt.Errorf("describing security groups: %w", err)
	}

	if len(output.SecurityGroups) == 0 {
		return ErrNoSecurityGroup
	}

	sg := &infrav1.SecurityGroup{
		ID:   *output.SecurityGroups[0].GroupId,
		Name: *output.SecurityGroups[0].GroupName,
		Tags: converters.TagsToMap(output.SecurityGroups[0].Tags),
	}
	s.scope.ControlPlane.Status.SecurityGroup = sg

	return nil
}