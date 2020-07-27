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
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/eks"
	. "github.com/onsi/gomega"
	infrav1 "sigs.k8s.io/cluster-api-provider-aws/api/v1alpha3"
	infrav1exp "sigs.k8s.io/cluster-api-provider-aws/exp/api/v1alpha3"
)

func TestMakeEksEncryptionConfigs(t *testing.T) {
	providerOne := "provider"
	resourceOne := "resourceOne"
	resourceTwo := "resourceTwo"
	testCases := []struct {
		name   string
		input  *infrav1exp.EncryptionConfig
		expect []*eks.EncryptionConfig
	}{
		{
			name:   "nil input",
			input:  nil,
			expect: []*eks.EncryptionConfig{},
		},
		{
			name: "nil input",
			input: &infrav1exp.EncryptionConfig{
				Provider:  &providerOne,
				Resources: []*string{&resourceOne, &resourceTwo},
			},
			expect: []*eks.EncryptionConfig{{
				Provider:  &eks.Provider{KeyArn: &providerOne},
				Resources: []*string{&resourceOne, &resourceTwo},
			}},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			g.Expect(makeEksEncryptionConfigs(tc.input)).To(Equal(tc.expect))
		})
	}
}

func TestMakeVPCConfig(t *testing.T) {
	type input struct {
		subnets        infrav1.Subnets
		endpointAccess infrav1exp.EndpointAccess
	}

	subnetIDOne := "one"
	subnetIDTwo := "two"
	testCases := []struct {
		name   string
		input  input
		err    bool
		expect *eks.VpcConfigRequest
	}{
		{
			name: "no subnets",
			input: input{
				subnets:        nil,
				endpointAccess: infrav1exp.EndpointAccess{},
			},
			err:    true,
			expect: nil,
		},
		{
			name: "enough subnets",
			input: input{
				subnets: []*infrav1.SubnetSpec{
					{
						ID:               subnetIDOne,
						CidrBlock:        "10.0.10.0/24",
						AvailabilityZone: "us-west-2a",
						IsPublic:         true,
					},
					{
						ID:               subnetIDTwo,
						CidrBlock:        "10.0.10.0/24",
						AvailabilityZone: "us-west-2b",
						IsPublic:         false,
					},
				},
				endpointAccess: infrav1exp.EndpointAccess{},
			},
			expect: &eks.VpcConfigRequest{
				SubnetIds: []*string{&subnetIDOne, &subnetIDTwo},
			},
		},
		{
			name: "non canonical public access CIDR",
			input: input{
				subnets: []*infrav1.SubnetSpec{
					{
						ID:               subnetIDOne,
						CidrBlock:        "10.0.10.0/24",
						AvailabilityZone: "us-west-2a",
						IsPublic:         true,
					},
					{
						ID:               subnetIDTwo,
						CidrBlock:        "10.0.10.1/24",
						AvailabilityZone: "us-west-2b",
						IsPublic:         false,
					},
				},
				endpointAccess: infrav1exp.EndpointAccess{
					PublicCIDRs: []*string{aws.String("10.0.0.1/24")},
				},
			},
			expect: &eks.VpcConfigRequest{
				SubnetIds:         []*string{&subnetIDOne, &subnetIDTwo},
				PublicAccessCidrs: []*string{aws.String("10.0.0.0/24")},
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			config, err := makeVpcConfig(tc.input.subnets, tc.input.endpointAccess)
			if tc.err {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(config).To(Equal(tc.expect))
			}
		})
	}

}

func TestPublicAccessCIDRsEqual(t *testing.T) {
	testCases := []struct {
		name   string
		a      []*string
		b      []*string
		expect bool
	}{
		{
			name:   "no CIDRs",
			a:      nil,
			b:      nil,
			expect: true,
		},
		{
			name:   "every address",
			a:      []*string{aws.String("0.0.0.0/0")},
			b:      nil,
			expect: true,
		},
		{
			name:   "every address",
			a:      []*string{aws.String("1.1.1.0/24")},
			b:      []*string{aws.String("1.1.1.0/24")},
			expect: true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			g.Expect(publicAccessCIDRsEqual(tc.a, tc.b)).To(Equal(tc.expect))
		})
	}
}
