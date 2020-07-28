/*
Copyright 2019 The Kubernetes Authors.

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

package controllers

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	expinfrav1 "sigs.k8s.io/cluster-api-provider-aws/api/v1alpha3"
	infrav1 "sigs.k8s.io/cluster-api-provider-aws/api/v1alpha3"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	capiv1exp "sigs.k8s.io/cluster-api/exp/api/v1alpha3"
)

func setupScheme() (*runtime.Scheme, error) {
	scheme := runtime.NewScheme()
	if err := infrav1.AddToScheme(scheme); err != nil {
		return nil, err
	}
	if err := clusterv1.AddToScheme(scheme); err != nil {
		return nil, err
	}
	if err := capiv1exp.AddToScheme(scheme); err != nil {
		return nil, err
	}
	if err := expinfrav1.AddToScheme(scheme); err != nil {
		return nil, err
	}
	return scheme, nil
}

func newMachinePool(clusterName, machineName string) *capiv1exp.MachinePool {
	return &capiv1exp.MachinePool{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				clusterv1.ClusterLabelName: clusterName,
			},
			Name:      machineName,
			Namespace: "default",
		},
	}
}

func newMachineWithInfrastructureRef(clusterName, machinePoolName string) *capiv1exp.MachinePool {
	m := newMachinePool(clusterName, machinePoolName)
	m.Spec.Template.Spec.InfrastructureRef = v1.ObjectReference{
		Kind:       "AWSMachinePool",
		Namespace:  "",
		Name:       "aws" + machinePoolName,
		APIVersion: infrav1.GroupVersion.String(),
	}
	return m
}

func newCluster(name string) *clusterv1.Cluster {
	return &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
	}
}
