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
	. "github.com/onsi/gomega/gstruct"

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
	"sigs.k8s.io/cluster-api/controllers/noderefutil"
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
		// var launchtemplate *expinfrav1.AWSLaunchTemplate

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

		When("there's a provider ID", func() {
			id := "<cloudProvider>://<optional>/<segments>/<providerid>"
			BeforeEach(func() {
				_, err := noderefutil.NewProviderID(id)
				Expect(err).To(BeNil())

				ms.AWSMachinePool.Spec.ProviderID = id
			})

			It("it should look up by provider ID when one exists", func() {
				expectedErr := errors.New("no connection available ")
				ec2Svc.EXPECT().GetLaunchTemplate(gomock.Any()).Return(nil, expectedErr)
				_, err := reconciler.reconcileNormal(context.Background(), ms, cs)
				Expect(errors.Cause(err)).To(MatchError(expectedErr))
			})

			It("should try to create a new machinepool if none exists", func() {
				expectedErr := errors.New("Invalid instance")
				asgSvc.EXPECT().ASGIfExists(gomock.Any()).Return(nil, nil).AnyTimes()
				ec2Svc.EXPECT().GetLaunchTemplate(gomock.Any()).Return(nil, nil)
				ec2Svc.EXPECT().CreateLaunchTemplate(gomock.Any(), gomock.Any()).Return("", expectedErr).AnyTimes()

				_, err := reconciler.reconcileNormal(context.Background(), ms, cs)
				Expect(errors.Cause(err)).To(MatchError(expectedErr))
			})
		})

		//TODO:
		// The current state of the group when the DeleteAutoScalingGroup operation
		// is in progress. in type Group struct {}
		//  Status *string `min:"1" type:"string"`
		// Check out output of describe autoscaling groups

		When("ASG creation succeeds", func() {
			var asg *expinfrav1.AutoScalingGroup
			BeforeEach(func() {
				asg = &expinfrav1.AutoScalingGroup{
					ID: "myMachinePool",
				}
				asg.State = expinfrav1.ASGStatePending

				ec2Svc.EXPECT().GetLaunchTemplate(gomock.Any()).Return(nil, nil).AnyTimes()
				ec2Svc.EXPECT().CreateLaunchTemplate(gomock.Any(), gomock.Any()).Return("", nil).AnyTimes()
				asgSvc.EXPECT().GetASGByName(gomock.Any()).Return(nil, nil).AnyTimes()

				// ec2Svc.EXPECT().GetRunningInstanceByTags(gomock.Any()).Return(nil, nil)
				// ec2Svc.EXPECT().CreateInstance(gomock.Any(), gomock.Any()).Return(asg, nil)
			})

			//TODO: make this pass in machinepool_controller.go
			Context("New ASG state", func() {
				It("should error when the instance state is a new unseen one", func() {
					buf := new(bytes.Buffer)
					klog.SetOutput(buf)
					asg.State = "NewAWSMachinePoolState"
					asgSvc.EXPECT().CreateASG(gomock.Any()).Return(asg, nil)
					_, _ = reconciler.reconcileNormal(context.Background(), ms, cs)
					Expect(ms.AWSMachinePool.Status.Ready).To(Equal(false))
					Expect(buf.String()).To(ContainSubstring(("ASG state is undefined")))
					Eventually(recorder.Events).Should(Receive(ContainSubstring("ASGUnhandledState")))
					Expect(ms.AWSMachinePool.Status.FailureMessage).To(PointTo(Equal("ASG state \"NewAWSMachinePoolState\" is undefined")))
					expectConditions(ms.AWSMachinePool, []conditionAssertion{{conditionType: expinfrav1.ASGReadyCondition, status: corev1.ConditionUnknown}})
				})
			})
		})
	})

	Context("deleting an AWSMachinePool", func() {
		BeforeEach(func() {
			ms.AWSMachinePool.Finalizers = []string{
				expinfrav1.MachinePoolFinalizer,
				metav1.FinalizerDeleteDependents,
			}
		})

		It("should exit immediately on an error state", func() {
			expectedErr := errors.New("no connection available ")
			asgSvc.EXPECT().GetASGByName(gomock.Any()).Return(nil, expectedErr).AnyTimes()

			_, err := reconciler.reconcileDelete(ms, cs)
			Expect(errors.Cause(err)).To(MatchError(expectedErr))
		})

		It("should log and remove finalizer when no machinepool exists", func() {
			asgSvc.EXPECT().GetASGByName(gomock.Any()).Return(nil, nil)
			ec2Svc.EXPECT().GetLaunchTemplate(gomock.Any()).Return(nil, nil).AnyTimes()

			buf := new(bytes.Buffer)
			klog.SetOutput(buf)

			_, err := reconciler.reconcileDelete(ms, cs)
			Expect(err).To(BeNil())
			Expect(buf.String()).To(ContainSubstring("Unable to locate ASG"))
			Expect(ms.AWSMachinePool.Finalizers).To(ConsistOf(metav1.FinalizerDeleteDependents))
			Eventually(recorder.Events).Should(Receive(ContainSubstring("NoASGFound")))
		})
	})
})

//TODO: This was taken from awsmachine_controller_test, i think it should be moved to elsewhere in both locations like test/helpers
func PointsTo(s string) gomock.Matcher {
	return &pointsTo{
		val: s,
	}
}

type pointsTo struct {
	val string
}

func (p *pointsTo) Matches(x interface{}) bool {
	ptr, ok := x.(*string)
	if !ok {
		return false
	}

	if ptr == nil {
		return false
	}

	return *ptr == p.val
}

func (p *pointsTo) String() string {
	return fmt.Sprintf("Pointer to string %q", p.val)
}

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
