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

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/pkg/errors"

	infrav1 "sigs.k8s.io/cluster-api-provider-aws/api/v1alpha3"
	"sigs.k8s.io/cluster-api-provider-aws/pkg/cloud/converters"
	"sigs.k8s.io/cluster-api-provider-aws/pkg/cloud/tags"
)

func (s *Service) reconcileTags(cluster *eks.Cluster) error {
	clusterTags := converters.MapPtrToMap(cluster.Tags)
	if err := tags.Ensure(clusterTags, s.getEKSTagParams(*cluster.Arn), s.applyTags); err != nil {
		return fmt.Errorf("failed ensuring tags on cluster: %w", err)
	}

	return nil
}

func (s *Service) getEKSTagParams(id string) *infrav1.BuildParams {
	name := fmt.Sprintf("%s-eks-cp", s.scope.Name())

	return &infrav1.BuildParams{
		ClusterName: s.scope.Name(),
		ResourceID:  id,
		Lifecycle:   infrav1.ResourceLifecycleOwned,
		Name:        aws.String(name),
		Role:        aws.String(infrav1.CommonRoleTagValue),
		Additional:  s.scope.AdditionalTags(),
	}
}

func (s *Service) applyTags(params *infrav1.BuildParams) error {
	tags := infrav1.Build(*params)

	eksTags := make(map[string]*string, len(tags))
	for k, v := range tags {
		eksTags[k] = aws.String(v)
	}

	tagResourcesInput := &eks.TagResourceInput{
		ResourceArn: aws.String(params.ResourceID),
		Tags:        eksTags,
	}

	_, err := s.EKSClient.TagResource(tagResourcesInput)
	return errors.Wrapf(err, "failed to tag eks cluster %q in cluster %q", params.ResourceID, params.ClusterName)
}
