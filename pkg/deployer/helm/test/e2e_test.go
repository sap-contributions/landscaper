// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors.
//
// SPDX-License-Identifier: Apache-2.0

package test_test

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	deployerlib "github.com/gardener/landscaper/pkg/deployer/lib"

	lsv1alpha1 "github.com/gardener/landscaper/apis/core/v1alpha1"
	"github.com/gardener/landscaper/apis/deployer/helm"
	helmv1alpha1 "github.com/gardener/landscaper/apis/deployer/helm/v1alpha1"
	"github.com/gardener/landscaper/apis/deployer/helm/v1alpha1/helper"
	"github.com/gardener/landscaper/pkg/api"
	helmctrl "github.com/gardener/landscaper/pkg/deployer/helm"
	kutil "github.com/gardener/landscaper/pkg/utils/kubernetes"
	"github.com/gardener/landscaper/test/utils"
	testutil "github.com/gardener/landscaper/test/utils"
	"github.com/gardener/landscaper/test/utils/envtest"
)

var _ = Describe("Helm Deployer", func() {

	var state *envtest.State

	BeforeEach(func() {
		var err error
		state, err = testenv.InitState(context.TODO())
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		Expect(testenv.CleanupState(context.TODO(), state)).To(Succeed())
	})

	It("should deploy an ingress-nginx chart from an oci artifact into the cluster", func() {
		ctx := context.Background()
		defer ctx.Done()

		deployer, err := helmctrl.NewDeployer(
			logr.Discard(),
			testenv.Client,
			testenv.Client,
			helmv1alpha1.Configuration{},
		)
		Expect(err).ToNot(HaveOccurred())

		ctrl := deployerlib.NewController(
			logr.Discard(),
			testenv.Client,
			api.LandscaperScheme,
			testenv.Client,
			api.LandscaperScheme,
			deployerlib.DeployerArgs{
				Type:     helmctrl.Type,
				Deployer: deployer,
			},
		)

		kubeconfigBytes, err := kutil.GenerateKubeconfigJSONBytes(testenv.Env.Config)
		Expect(err).ToNot(HaveOccurred())

		di := &lsv1alpha1.DeployItem{}
		di.Name = "ingress-test-di"
		di.Namespace = state.Namespace
		di.Spec.Target = &lsv1alpha1.ObjectReference{
			Name:      "test-target",
			Namespace: state.Namespace,
		}
		di.Spec.Type = helmctrl.Type

		// Create target
		target, err := testutil.CreateOrUpdateTarget(ctx,
			testenv.Client,
			di.Spec.Target.Namespace,
			di.Spec.Target.Name,
			string(lsv1alpha1.KubernetesClusterTargetType),
			lsv1alpha1.KubernetesClusterTargetConfig{
				Kubeconfig: lsv1alpha1.ValueRef{
					StrVal: pointer.StringPtr(string(kubeconfigBytes)),
				},
			},
		)
		Expect(err).ToNot(HaveOccurred())
		Expect(state.AddResources(target)).To(Succeed())

		// create helm provider config
		providerConfig := &helmv1alpha1.ProviderConfiguration{}
		providerConfig.Chart.Ref = "eu.gcr.io/gardener-project/landscaper/tutorials/charts/ingress-nginx:v3.29.0"
		providerConfig.Name = "ingress-test"
		providerConfig.Namespace = state.Namespace
		providerConfig.HealthChecks = helmv1alpha1.HealthChecksConfiguration{
			Timeout: &lsv1alpha1.Duration{Duration: 1 * time.Second},
		}

		di.Spec.Configuration, err = helper.ProviderConfigurationToRawExtension(providerConfig)
		Expect(err).ToNot(HaveOccurred())

		Expect(state.Create(ctx, testenv.Client, di, envtest.UpdateStatus(true))).To(Succeed())

		// At this stage, resources are not yet ready
		err = testutil.ShouldNotReconcile(ctx, ctrl, testutil.Request(di.GetName(), di.GetNamespace()))
		Expect(err).To(HaveOccurred())

		// Get the managed objects from Status and set them in Ready status
		status := &helm.ProviderStatus{}
		Expect(testenv.Client.Get(ctx, testutil.Request(di.GetName(), di.GetNamespace()).NamespacedName, di)).To(Succeed())

		helmDecoder := serializer.NewCodecFactory(helmctrl.HelmScheme).UniversalDecoder()
		_, _, err = helmDecoder.Decode(di.Status.ProviderStatus.Raw, nil, status)
		Expect(err).ToNot(HaveOccurred())

		for _, ref := range status.ManagedResources {
			tmpRef := &lsv1alpha1.TypedObjectReference{}
			utils.ExpectNoError(lsv1alpha1.Convert_core_TypedObjectReference_To_v1alpha1_TypedObjectReference(&ref, tmpRef, nil))
			obj := kutil.ObjectFromTypedObjectReference(tmpRef)
			Expect(testenv.Client.Get(ctx, testutil.Request(obj.GetName(), obj.GetNamespace()).NamespacedName, obj)).To(Succeed())
			Expect(testutil.SetReadyStatus(ctx, testenv.Client, obj)).To(Succeed())
		}
		di.Status.Phase = lsv1alpha1.ExecutionPhaseProgressing
		Expect(testenv.Client.Status().Update(ctx, di)).To(Succeed())

		testutil.ShouldReconcile(ctx, ctrl, testutil.Request(di.GetName(), di.GetNamespace()))
		Expect(testenv.Client.Get(ctx, testutil.Request(di.GetName(), di.GetNamespace()).NamespacedName, di)).To(Succeed())

		Expect(di.Status.Phase).To(Equal(lsv1alpha1.ExecutionPhaseSucceeded))

		deploymentList := &appsv1.DeploymentList{}
		Expect(testenv.Client.List(ctx, deploymentList, client.InNamespace(state.Namespace))).To(Succeed())
		Expect(deploymentList.Items).To(HaveLen(1))

		deployment := deploymentList.Items[0]
		Expect(deployment.Name).To(Equal("ingress-test-ingress-nginx-controller"))

		//testutil.ExpectNoError(testenv.Client.Delete(ctx, di))
		//// Expect that the deploy item gets deleted
		//Eventually(func() error{
		//	_, err := actuator.Reconcile(ctx, testutil.Request(di.GetName(), di.GetNamespace()))
		//	return err
		//}, time.Minute, 5 *time.Second).Should(Succeed())
		//
		//Expect(testenv.Client.Get(ctx, testutil.Request(di.GetName(), di.GetNamespace()).NamespacedName, di)).To(HaveOccurred())
	})

})
