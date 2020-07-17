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

package filter

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	infrav1 "sigs.k8s.io/cluster-api-provider-aws/api/v1alpha3"
)

var (
	// ASG exposes the AutoScalingGroup sdk related filters.
	ASG = new(asgFilters)
)

type asgFilters struct{}

//TODO: trim this.

// Cluster returns a filter based on the cluster name.
func (asgFilters) Cluster(clusterName string) *autoscaling.Filter {
	return &autoscaling.Filter{
		Name:   aws.String(filterNameTagKey),
		Values: aws.StringSlice([]string{infrav1.ClusterTagKey(clusterName)}),
	}
}

// Name returns a filter based on the resource name.
func (asgFilters) Name(name string) *autoscaling.Filter {
	return &autoscaling.Filter{
		Name:   aws.String("tag:Name"),
		Values: aws.StringSlice([]string{name}),
	}
}

// ClusterOwned returns a filter using the Cluster API per-cluster tag where
// the resource is owned
func (asgFilters) ClusterOwned(clusterName string) *autoscaling.Filter {
	return &autoscaling.Filter{
		Name:   aws.String(fmt.Sprintf("tag:%s", infrav1.ClusterTagKey(clusterName))),
		Values: aws.StringSlice([]string{string(infrav1.ResourceLifecycleOwned)}),
	}
}

// ClusterShared returns a filter using the Cluster API per-cluster tag where
// the resource is shared.
func (asgFilters) ClusterShared(clusterName string) *autoscaling.Filter {
	return &autoscaling.Filter{
		Name:   aws.String(fmt.Sprintf("tag:%s", infrav1.ClusterTagKey(clusterName))),
		Values: aws.StringSlice([]string{string(infrav1.ResourceLifecycleShared)}),
	}
}

// ProviderRole returns a filter using cluster-api-provider-aws role tag.
func (asgFilters) ProviderRole(role string) *autoscaling.Filter {
	return &autoscaling.Filter{
		Name:   aws.String(fmt.Sprintf("tag:%s", infrav1.NameAWSClusterAPIRole)),
		Values: aws.StringSlice([]string{role}),
	}
}

// ProviderOwned returns a filter using the cloud provider tag where the resource is owned.
func (asgFilters) ProviderOwned(clusterName string) *autoscaling.Filter {
	return &autoscaling.Filter{
		Name:   aws.String(fmt.Sprintf("tag:%s", infrav1.ClusterAWSCloudProviderTagKey(clusterName))),
		Values: aws.StringSlice([]string{string(infrav1.ResourceLifecycleOwned)}),
	}
}

// VPC returns a filter based on the id of the VPC.
func (asgFilters) VPC(vpcID string) *autoscaling.Filter {
	return &autoscaling.Filter{
		Name:   aws.String(filterNameVpcID),
		Values: aws.StringSlice([]string{vpcID}),
	}
}

// VPCAttachment returns a filter based on the vpc id attached to the resource.
func (asgFilters) VPCAttachment(vpcID string) *autoscaling.Filter {
	return &autoscaling.Filter{
		Name:   aws.String(filterNameVpcAttachment),
		Values: aws.StringSlice([]string{vpcID}),
	}
}

// Available returns a filter based on the state being available.
func (asgFilters) Available() *autoscaling.Filter {
	return &autoscaling.Filter{
		Name:   aws.String(filterNameState),
		Values: aws.StringSlice([]string{"available"}),
	}
}

// NATGatewayStates returns a filter based on the list of states passed in.
func (asgFilters) NATGatewayStates(states ...string) *autoscaling.Filter {
	return &autoscaling.Filter{
		Name:   aws.String("state"),
		Values: aws.StringSlice(states),
	}
}

// InstanceStates returns a filter based on the list of states passed in.
func (asgFilters) InstanceStates(states ...string) *autoscaling.Filter {
	return &autoscaling.Filter{
		Name:   aws.String("instance-state-name"),
		Values: aws.StringSlice(states),
	}
}

// VPCStates returns a filter based on the list of states passed in.
func (asgFilters) VPCStates(states ...string) *autoscaling.Filter {
	return &autoscaling.Filter{
		Name:   aws.String("state"),
		Values: aws.StringSlice(states),
	}
}

// SubnetStates returns a filter based on the list of states passed in.
func (asgFilters) SubnetStates(states ...string) *autoscaling.Filter {
	return &autoscaling.Filter{
		Name:   aws.String("state"),
		Values: aws.StringSlice(states),
	}
}
