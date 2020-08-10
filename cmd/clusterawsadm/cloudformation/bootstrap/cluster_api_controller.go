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

package bootstrap

import (
	"fmt"

	"github.com/awslabs/goformation/v4/cloudformation"
	iamv1 "sigs.k8s.io/cluster-api-provider-aws/cmd/clusterawsadm/api/iam/v1alpha1"
)

func (t Template) controllersPolicyGroups() []string {
	groups := []string{}
	if t.Spec.BootstrapUser.Enable {
		groups = append(groups, cloudformation.Ref(AWSIAMGroupBootstrapper))
	}
	return groups
}

func (t Template) controllersPolicyRoleAttachments() []string {
	attachments := []string{
		cloudformation.Ref(AWSIAMRoleControllers),
	}
	if !t.Spec.ControlPlane.DisableClusterAPIControllerPolicyAttachment {
		attachments = append(
			attachments,
			cloudformation.Ref(AWSIAMRoleControlPlane),
		)
	}
	return attachments
}

func (t Template) controllersTrustPolicy() *iamv1.PolicyDocument {
	policyDocument := ec2AssumeRolePolicy()
	policyDocument.Statement = append(policyDocument.Statement, t.Spec.ClusterAPIControllers.TrustStatements...)
	return policyDocument
}

func (t Template) controllersPolicy() *iamv1.PolicyDocument {
	policyDocument := &iamv1.PolicyDocument{
		Version: iamv1.CurrentVersion,
		Statement: []iamv1.StatementEntry{
			{
				Effect:   iamv1.EffectAllow,
				Resource: iamv1.Resources{iamv1.Any},
				Action: iamv1.Actions{
					"ec2:AllocateAddress",
					"ec2:AssociateRouteTable",
					"ec2:AttachInternetGateway",
					"ec2:AuthorizeSecurityGroupIngress",
					"ec2:CreateInternetGateway",
					"ec2:CreateNatGateway",
					"ec2:CreateRoute",
					"ec2:CreateRouteTable",
					"ec2:CreateSecurityGroup",
					"ec2:CreateSubnet",
					"ec2:CreateTags",
					"ec2:CreateVpc",
					"ec2:ModifyVpcAttribute",
					"ec2:DeleteInternetGateway",
					"ec2:DeleteNatGateway",
					"ec2:DeleteRouteTable",
					"ec2:DeleteSecurityGroup",
					"ec2:DeleteSubnet",
					"ec2:DeleteTags",
					"ec2:DeleteVpc",
					"ec2:DescribeAccountAttributes",
					"ec2:DescribeAddresses",
					"ec2:DescribeAvailabilityZones",
					"ec2:DescribeInstances",
					"ec2:DescribeInternetGateways",
					"ec2:DescribeImages",
					"ec2:DescribeNatGateways",
					"ec2:DescribeNetworkInterfaces",
					"ec2:DescribeNetworkInterfaceAttribute",
					"ec2:DescribeRouteTables",
					"ec2:DescribeSecurityGroups",
					"ec2:DescribeSubnets",
					"ec2:DescribeVpcs",
					"ec2:DescribeVpcAttribute",
					"ec2:DescribeVolumes",
					"ec2:DetachInternetGateway",
					"ec2:DisassociateRouteTable",
					"ec2:DisassociateAddress",
					"ec2:ModifyInstanceAttribute",
					"ec2:ModifyNetworkInterfaceAttribute",
					"ec2:ModifySubnetAttribute",
					"ec2:ReleaseAddress",
					"ec2:RevokeSecurityGroupIngress",
					"ec2:RunInstances",
					"ec2:TerminateInstances",
					"tag:GetResources",
					"elasticloadbalancing:AddTags",
					"elasticloadbalancing:CreateLoadBalancer",
					"elasticloadbalancing:ConfigureHealthCheck",
					"elasticloadbalancing:DeleteLoadBalancer",
					"elasticloadbalancing:DescribeLoadBalancers",
					"elasticloadbalancing:DescribeLoadBalancerAttributes",
					"elasticloadbalancing:DescribeTags",
					"elasticloadbalancing:ModifyLoadBalancerAttributes",
					"elasticloadbalancing:RegisterInstancesWithLoadBalancer",
					"elasticloadbalancing:DeregisterInstancesFromLoadBalancer",
					"elasticloadbalancing:RemoveTags",
				},
			},
			{
				Effect: iamv1.EffectAllow,
				Resource: iamv1.Resources{
					"arn:*:iam::*:role/aws-service-role/elasticloadbalancing.amazonaws.com/AWSServiceRoleForElasticLoadBalancing",
				},
				Action: iamv1.Actions{
					"iam:CreateServiceLinkedRole",
				},
				Condition: iamv1.Conditions{
					iamv1.StringLike: map[string]string{"iam:AWSServiceName": "elasticloadbalancing.amazonaws.com"},
				},
			},
			{
				Effect:   iamv1.EffectAllow,
				Resource: t.allowedEC2InstanceProfiles(),
				Action: iamv1.Actions{
					"iam:PassRole",
				},
			},
			{
				Effect: iamv1.EffectAllow,
				Resource: iamv1.Resources{
					"arn:*:secretsmanager:*:*:secret:aws.cluster.x-k8s.io/*",
				},
				Action: iamv1.Actions{
					"secretsmanager:CreateSecret",
					"secretsmanager:DeleteSecret",
					"secretsmanager:TagResource",
				},
			},
			{
				Effect: iamv1.EffectAllow,
				Resource: iamv1.Resources{
					"arn:aws:ssm:*:*:parameter/aws/service/eks/optimized-ami/*",
				},
				Action: iamv1.Actions{
					"ssm:GetParameter",
				},
			},
		},
	}
	return policyDocument
}

func (t Template) allowedEC2InstanceProfiles() iamv1.Resources {
	if t.Spec.ClusterAPIControllers.AllowedEC2InstanceProfiles == nil {
		t.Spec.ClusterAPIControllers.AllowedEC2InstanceProfiles = []string{
			t.NewManagedName(iamv1.Any),
		}
	}
	instanceProfiles := make(iamv1.Resources, len(t.Spec.ClusterAPIControllers.AllowedEC2InstanceProfiles))

	for i, p := range t.Spec.ClusterAPIControllers.AllowedEC2InstanceProfiles {
		instanceProfiles[i] = fmt.Sprintf("arn:*:iam::*:role/%s", p)
	}

	return instanceProfiles
}
