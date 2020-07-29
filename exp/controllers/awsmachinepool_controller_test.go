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

package controllers

import (
	"bytes"
	"context"
	"flag"
	"fmt"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog"
	"k8s.io/utils/pointer"
	infrav1 "sigs.k8s.io/cluster-api-provider-aws/api/v1alpha3"
	expinfrav1 "sigs.k8s.io/cluster-api-provider-aws/exp/api/v1alpha3"
	"sigs.k8s.io/cluster-api-provider-aws/pkg/cloud/scope"
	"sigs.k8s.io/cluster-api-provider-aws/pkg/cloud/services"
	"sigs.k8s.io/cluster-api-provider-aws/pkg/cloud/services/mock_services"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	capierrors "sigs.k8s.io/cluster-api/errors"
	expclusterv1 "sigs.k8s.io/cluster-api/exp/api/v1alpha3"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("AWSMachinePoolReconciler", func() {
	var (
		reconciler AWSMachinePoolReconciler
		cs         *scope.ClusterScope
		ms         *scope.MachinePoolScope
		mockCtrl   *gomock.Controller
		ec2Svc     *mock_services.MockEC2MachineInterface
		asgSvc     *mock_services.MockASGInterface
		recorder   *record.FakeRecorder
	)

	BeforeEach(func() {
		var err error //TODO: check out LogToOutput

		if err := flag.Set("logtostderr", "false"); err != nil {
			_ = fmt.Errorf("Error setting logtostderr flag")
		}
		if err := flag.Set("v", "2"); err != nil {
			_ = fmt.Errorf("Error setting v flag")
		}

		klog.SetOutput(GinkgoWriter)

		awsMachinePool := &expinfrav1.AWSMachinePool{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
			},
			Spec: expinfrav1.AWSMachinePoolSpec{},
		}

		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name: "bootstrap-data",
			},
			Data: map[string][]byte{
				"value": []byte("shell-script"),
			},
		}

		ms, err = scope.NewMachinePoolScope(
			scope.MachinePoolScopeParams{
				Client: fake.NewFakeClient([]runtime.Object{awsMachinePool, secret}...),
				Cluster: &clusterv1.Cluster{
					Status: clusterv1.ClusterStatus{
						InfrastructureReady: true,
					},
				},
				MachinePool: &expclusterv1.MachinePool{
					Spec: expclusterv1.MachinePoolSpec{
						Template: clusterv1.MachineTemplateSpec{
							Spec: clusterv1.MachineSpec{
								Bootstrap: clusterv1.Bootstrap{
									DataSecretName: pointer.StringPtr("bootstrap-data"),
								},
							},
						},
					},
				},
				AWSCluster:     &infrav1.AWSCluster{},
				AWSMachinePool: awsMachinePool,
			},
		)
		Expect(err).To(BeNil())

		cs, err = scope.NewClusterScope(
			scope.ClusterScopeParams{
				Cluster:    &clusterv1.Cluster{},
				AWSCluster: &infrav1.AWSCluster{},
			},
		)
		Expect(err).To(BeNil())

		mockCtrl = gomock.NewController(GinkgoT())
		ec2Svc = mock_services.NewMockEC2MachineInterface(mockCtrl)
		asgSvc = mock_services.NewMockASGInterface(mockCtrl)

		// If the test hangs for 9 minutes, increase the value here to the number of events during a reconciliation loop
		recorder = record.NewFakeRecorder(2)

		reconciler = AWSMachinePoolReconciler{
			ec2ServiceFactory: func(*scope.ClusterScope) services.EC2MachineInterface {
				return ec2Svc
			},
			asgServiceFactory: func(*scope.ClusterScope) services.ASGInterface {
				return asgSvc
			},
			Recorder: recorder,
		}
	})
	AfterEach(func() {
		mockCtrl.Finish()
	})

	Context("Reconciling an AWSMachinePool", func() {
		When("we can't reach amazon", func() {
			expectedErr := errors.New("no connection available ")

			BeforeEach(func() {
				ec2Svc.EXPECT().GetLaunchTemplate(gomock.Any()).Return(nil, expectedErr).AnyTimes()
				asgSvc.EXPECT().GetASGByName(gomock.Any()).Return(nil, expectedErr).AnyTimes()
			})

			It("should exit immediately on an error state", func() {
				er := capierrors.CreateMachineError
				ms.AWSMachinePool.Status.FailureReason = &er
				ms.AWSMachinePool.Status.FailureMessage = pointer.StringPtr("Couldn't create machine pool")

				buf := new(bytes.Buffer)
				klog.SetOutput(buf)

				_, _ = reconciler.reconcileNormal(context.Background(), ms, cs)
				Expect(buf).To(ContainSubstring("Error state detected, skipping reconciliation"))
			})

			It("should add our finalizer to the machinepool", func() {
				_, _ = reconciler.reconcileNormal(context.Background(), ms, cs)

				Expect(ms.AWSMachinePool.Finalizers).To(ContainElement(expinfrav1.MachinePoolFinalizer))
			})

			It("should exit immediately if cluster infra isn't ready", func() {
				ms.Cluster.Status.InfrastructureReady = false

				buf := new(bytes.Buffer)
				klog.SetOutput(buf)

				_, err := reconciler.reconcileNormal(context.Background(), ms, cs)
				Expect(err).To(BeNil())
				Expect(buf.String()).To(ContainSubstring("Cluster infrastructure is not ready yet"))
				expectConditions(ms.AWSMachinePool, []conditionAssertion{{expinfrav1.ASGReadyCondition, corev1.ConditionFalse, clusterv1.ConditionSeverityInfo, infrav1.WaitingForClusterInfrastructureReason}})
			})

			It("should exit immediately if bootstrap data secret reference isn't available", func() {
				ms.MachinePool.Spec.Template.Spec.Bootstrap.DataSecretName = nil
				buf := new(bytes.Buffer)
				klog.SetOutput(buf)

				_, err := reconciler.reconcileNormal(context.Background(), ms, cs)

				Expect(err).To(BeNil())
				Expect(buf.String()).To(ContainSubstring("Bootstrap data secret reference is not yet available"))
				expectConditions(ms.AWSMachinePool, []conditionAssertion{{expinfrav1.ASGReadyCondition, corev1.ConditionFalse, clusterv1.ConditionSeverityInfo, infrav1.WaitingForBootstrapDataReason}})
			})

			It("should return an error when we can't list instances by tags", func() {
				_, err := reconciler.reconcileNormal(context.Background(), ms, cs)
				Expect(errors.Cause(err)).To(MatchError(expectedErr))
			})
		})

		// When("there's a provider ID", func() {
		// 	id := "aws:////myMachine"
		// 	BeforeEach(func() {
		// 		_, err := noderefutil.NewProviderID(id)
		// 		Expect(err).To(BeNil())

		// 		ms.AWSMachine.Spec.ProviderID = &id
		// 	})

		// 		It("it should look up by provider ID when one exists", func() {
		// 			expectedErr := errors.New("no connection available ")
		// 			ec2Svc.EXPECT().InstanceIfExists(PointsTo("myMachine")).Return(nil, expectedErr)

		// 			_, err := reconciler.reconcileNormal(context.Background(), ms, cs)
		// 			Expect(errors.Cause(err)).To(MatchError(expectedErr))
		// 		})

		// 		It("should try to create a new machine if none exists", func() {
		// 			expectedErr := errors.New("Invalid instance")
		// 			ec2Svc.EXPECT().InstanceIfExists(gomock.Any()).Return(nil, nil)
		// 			secretSvc.EXPECT().Create(gomock.Any(), gomock.Any()).Return("test", int32(1), nil).Times(1)
		// 			ec2Svc.EXPECT().CreateInstance(gomock.Any(), gomock.Any()).Return(nil, expectedErr)

		// 			_, err := reconciler.reconcileNormal(context.Background(), ms, cs)
		// 			Expect(errors.Cause(err)).To(MatchError(expectedErr))
		// 		})
		// 	})

		// 	When("instance creation succeeds", func() {
		// 		var instance *infrav1.Instance
		// 		BeforeEach(func() {
		// 			instance = &infrav1.Instance{
		// 				ID: "myMachine",
		// 			}
		// 			instance.State = infrav1.InstanceStatePending

		// 			ec2Svc.EXPECT().GetRunningInstanceByTags(gomock.Any()).Return(nil, nil)
		// 			secretSvc.EXPECT().Create(gomock.Any(), gomock.Any()).Return("test", int32(1), nil).Times(1)
		// 			ec2Svc.EXPECT().CreateInstance(gomock.Any(), gomock.Any()).Return(instance, nil)
		// 		})

		// 		Context("instance security group errors", func() {
		// 			BeforeEach(func() {
		// 				ec2Svc.EXPECT().GetInstanceSecurityGroups(gomock.Any()).Return(nil, errors.New("stop here"))
		// 			})

		// 			It("should set attributes after creating an instance", func() {
		// 				_, _ = reconciler.reconcileNormal(context.Background(), ms, cs)
		// 				Expect(ms.AWSMachine.Spec.ProviderID).To(PointTo(Equal("aws:////myMachine")))
		// 				Expect(ms.AWSMachine.Annotations).To(Equal(map[string]string{"cluster-api-provider-aws": "true"}))
		// 			})

		// 			Context("with captured logging", func() {
		// 				var buf *bytes.Buffer

		// 				BeforeEach(func() {
		// 					buf = new(bytes.Buffer)
		// 					klog.SetOutput(buf)
		// 				})

		// 				It("should set instance to pending", func() {
		// 					instance.State = infrav1.InstanceStatePending
		// 					_, _ = reconciler.reconcileNormal(context.Background(), ms, cs)
		// 					Expect(ms.AWSMachine.Status.InstanceState).To(PointTo(Equal(infrav1.InstanceStatePending)))
		// 					Expect(ms.AWSMachine.Status.Ready).To(Equal(false))
		// 					Expect(buf.String()).To(ContainSubstring(("EC2 instance state changed")))
		// 					expectConditions(ms.AWSMachine, []conditionAssertion{{infrav1.InstanceReadyCondition, corev1.ConditionFalse, clusterv1.ConditionSeverityWarning, infrav1.InstanceNotReadyReason}})
		// 				})

		// 				It("should set instance to running", func() {
		// 					instance.State = infrav1.InstanceStateRunning
		// 					_, _ = reconciler.reconcileNormal(context.Background(), ms, cs)
		// 					Expect(ms.AWSMachine.Status.InstanceState).To(PointTo(Equal(infrav1.InstanceStateRunning)))
		// 					Expect(ms.AWSMachine.Status.Ready).To(Equal(true))
		// 					Expect(buf.String()).To(ContainSubstring(("EC2 instance state changed")))
		// 					expectConditions(ms.AWSMachine, []conditionAssertion{
		// 						{conditionType: infrav1.InstanceReadyCondition, status: corev1.ConditionTrue},
		// 					})
		// 				})
		// 			})
		// 		})

		// 		Context("New EC2 instance state", func() {
		// 			It("should error when the instance state is a new unseen one", func() {
		// 				buf := new(bytes.Buffer)
		// 				klog.SetOutput(buf)
		// 				instance.State = "NewAWSMachineState"
		// 				secretSvc.EXPECT().Delete(gomock.Any()).Return(nil).Times(1)
		// 				_, _ = reconciler.reconcileNormal(context.Background(), ms, cs)
		// 				Expect(ms.AWSMachine.Status.Ready).To(Equal(false))
		// 				Expect(buf.String()).To(ContainSubstring(("EC2 instance state is undefined")))
		// 				Eventually(recorder.Events).Should(Receive(ContainSubstring("InstanceUnhandledState")))
		// 				Expect(ms.AWSMachine.Status.FailureMessage).To(PointTo(Equal("EC2 instance state \"NewAWSMachineState\" is undefined")))
		// 				expectConditions(ms.AWSMachine, []conditionAssertion{{conditionType: infrav1.InstanceReadyCondition, status: corev1.ConditionUnknown}})
		// 			})
		// 		})

		// 		Context("Security Groups succeed", func() {
		// 			BeforeEach(func() {
		// 				ec2Svc.EXPECT().GetInstanceSecurityGroups(gomock.Any()).
		// 					Return(map[string][]string{"eid": {}}, nil)
		// 				ec2Svc.EXPECT().GetCoreSecurityGroups(gomock.Any()).Return([]string{}, nil)
		// 			})

		// 			It("should reconcile security groups", func() {
		// 				ms.AWSMachine.Spec.AdditionalSecurityGroups = []infrav1.AWSResourceReference{
		// 					{
		// 						ID: pointer.StringPtr("sg-2345"),
		// 					},
		// 				}
		// 				// ms.AWSMachine.Spec.AdditionalSecurityGroups = []infrav1
		// 				ec2Svc.EXPECT().UpdateInstanceSecurityGroups(instance.ID, []string{"sg-2345"})

		// 				_, _ = reconciler.reconcileNormal(context.Background(), ms, cs)
		// 				expectConditions(ms.AWSMachine, []conditionAssertion{{conditionType: infrav1.SecurityGroupsReadyCondition, status: corev1.ConditionTrue}})
		// 			})

		// 			It("should not tag anything if there's not tags", func() {
		// 				ec2Svc.EXPECT().UpdateInstanceSecurityGroups(gomock.Any(), gomock.Any()).Times(0)
		// 				if _, err := reconciler.reconcileNormal(context.Background(), ms, cs); err != nil {
		// 					_ = fmt.Errorf("reconcileNormal reutrned an error during test")
		// 				}
		// 			})

		// 			It("should tag instances from machine and cluster tags", func() {
		// 				ms.AWSMachine.Spec.AdditionalTags = infrav1.Tags{"kind": "alicorn"}
		// 				ms.AWSCluster.Spec.AdditionalTags = infrav1.Tags{"colour": "lavender"}

		// 				ec2Svc.EXPECT().UpdateResourceTags(
		// 					PointsTo("myMachine"),
		// 					map[string]string{
		// 						"kind":   "alicorn",
		// 						"colour": "lavender",
		// 					},
		// 					map[string]string{},
		// 				).Return(nil)

		// 				_, err := reconciler.reconcileNormal(context.Background(), ms, cs)
		// 				Expect(err).To(BeNil())
		// 			})
		// 		})

		// 		When("temporarily stopping then starting the AWSMachine", func() {
		// 			var buf *bytes.Buffer
		// 			BeforeEach(func() {
		// 				buf = new(bytes.Buffer)
		// 				klog.SetOutput(buf)
		// 				ec2Svc.EXPECT().GetInstanceSecurityGroups(gomock.Any()).
		// 					Return(map[string][]string{"eid": {}}, nil).Times(1)
		// 				ec2Svc.EXPECT().GetCoreSecurityGroups(gomock.Any()).Return([]string{}, nil).Times(1)
		// 			})

		// 			It("should set instance to stopping and unready", func() {
		// 				instance.State = infrav1.InstanceStateStopping
		// 				_, _ = reconciler.reconcileNormal(context.Background(), ms, cs)
		// 				Expect(ms.AWSMachine.Status.InstanceState).To(PointTo(Equal(infrav1.InstanceStateStopping)))
		// 				Expect(ms.AWSMachine.Status.Ready).To(Equal(false))
		// 				Expect(buf.String()).To(ContainSubstring(("EC2 instance state changed")))
		// 				expectConditions(ms.AWSMachine, []conditionAssertion{{infrav1.InstanceReadyCondition, corev1.ConditionFalse, clusterv1.ConditionSeverityError, infrav1.InstanceStoppedReason}})
		// 			})

		// 			It("should then set instance to stopped and unready", func() {
		// 				instance.State = infrav1.InstanceStateStopped
		// 				_, _ = reconciler.reconcileNormal(context.Background(), ms, cs)
		// 				Expect(ms.AWSMachine.Status.InstanceState).To(PointTo(Equal(infrav1.InstanceStateStopped)))
		// 				Expect(ms.AWSMachine.Status.Ready).To(Equal(false))
		// 				Expect(buf.String()).To(ContainSubstring(("EC2 instance state changed")))
		// 				expectConditions(ms.AWSMachine, []conditionAssertion{{infrav1.InstanceReadyCondition, corev1.ConditionFalse, clusterv1.ConditionSeverityError, infrav1.InstanceStoppedReason}})
		// 			})

		// 			It("should then set instance to running and ready once it is restarted", func() {
		// 				instance.State = infrav1.InstanceStateRunning
		// 				_, _ = reconciler.reconcileNormal(context.Background(), ms, cs)
		// 				Expect(ms.AWSMachine.Status.InstanceState).To(PointTo(Equal(infrav1.InstanceStateRunning)))
		// 				Expect(ms.AWSMachine.Status.Ready).To(Equal(true))
		// 				Expect(buf.String()).To(ContainSubstring(("EC2 instance state changed")))
		// 			})
		// 		})

		// 		When("deleting the AWSMachine outside of Kubernetes", func() {
		// 			var buf *bytes.Buffer
		// 			BeforeEach(func() {
		// 				buf = new(bytes.Buffer)
		// 				klog.SetOutput(buf)
		// 				secretSvc.EXPECT().Delete(gomock.Any()).Return(nil).Times(1)
		// 			})

		// 			It("should warn if an instance is shutting-down", func() {
		// 				instance.State = infrav1.InstanceStateShuttingDown
		// 				_, _ = reconciler.reconcileNormal(context.Background(), ms, cs)
		// 				Expect(ms.AWSMachine.Status.Ready).To(Equal(false))
		// 				Expect(buf.String()).To(ContainSubstring(("Unexpected EC2 instance termination")))
		// 				Eventually(recorder.Events).Should(Receive(ContainSubstring("UnexpectedTermination")))
		// 			})

		// 			It("should error when the instance is seen as terminated", func() {
		// 				instance.State = infrav1.InstanceStateTerminated
		// 				_, _ = reconciler.reconcileNormal(context.Background(), ms, cs)
		// 				Expect(ms.AWSMachine.Status.Ready).To(Equal(false))
		// 				Expect(buf.String()).To(ContainSubstring(("Unexpected EC2 instance termination")))
		// 				Eventually(recorder.Events).Should(Receive(ContainSubstring("UnexpectedTermination")))
		// 				Expect(ms.AWSMachine.Status.FailureMessage).To(PointTo(Equal("EC2 instance state \"terminated\" is unexpected")))
		// 				expectConditions(ms.AWSMachine, []conditionAssertion{{infrav1.InstanceReadyCondition, corev1.ConditionFalse, clusterv1.ConditionSeverityError, infrav1.InstanceTerminatedReason}})
		// 			})
		// 		})
		// 	})
	})
})

func expectConditions(m *expinfrav1.AWSMachinePool, expected []conditionAssertion) {
	Expect(len(m.Status.Conditions)).To(BeNumerically(">=", len(expected)), "number of conditions")
	for _, c := range expected {
		actual := conditions.Get(m, c.conditionType)
		Expect(actual).To(Not(BeNil()))
		Expect(actual.Type).To(Equal(c.conditionType))
		Expect(actual.Status).To(Equal(c.status))
		Expect(actual.Severity).To(Equal(c.severity))
		Expect(actual.Reason).To(Equal(c.reason))
	}
}

type conditionAssertion struct {
	conditionType clusterv1.ConditionType
	status        corev1.ConditionStatus
	severity      clusterv1.ConditionSeverity
	reason        string
}
