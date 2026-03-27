// Copyright 2022 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

//go:build e2e
// +build e2e

// Package e2e_test implements e2e tests for metrics adapter, which are not related to any specific package.
//
// They also test Helm chart manifests to verify metrics reachability over Kubernetes API server.
package e2e_test

import (
	"context"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	appsv1client "k8s.io/client-go/kubernetes/typed/apps/v1"
	eclient "k8s.io/metrics/pkg/client/external_metrics"
	"k8s.io/utils/ptr"

	"github.com/newrelic/newrelic-k8s-metrics-adapter/internal/testutil"
)

const (
	// This metric must be configured when deploying the adapter.
	testMetric               = "e2e"
	testMetricAttributeKey   = "attributeName"
	testMetricAttributeValue = "0.123"

	testPrefix = "newrelic-metrics-adapter-e2e-tests-"
)

//nolint:funlen,cyclop,gocognit // Just many test cases.
func Test_Metrics_adapter_makes_sample_external_metric_available(t *testing.T) {
	t.Parallel()

	testEnv := &testutil.TestEnv{
		// Under normal circumstances it should not take more than 60 seconds for HPA to converge.
		ContextTimeout:  60 * time.Second,
		StartKubernetes: true,
	}

	testEnv.Generate(t)

	clientset, err := kubernetes.NewForConfig(testEnv.RestConfig)
	if err != nil {
		t.Fatalf("Unexpected error creating clientset: %v", err)
	}

	t.Run("to_local_client", func(t *testing.T) {
		t.Parallel()

		ns := withTestNamespace(testEnv.Context, t, clientset)

		externalMetricsClient, err := eclient.NewForConfig(testEnv.RestConfig)
		if err != nil {
			t.Fatalf("Creating metrics clientset: %v", err)
		}

		if _, err := externalMetricsClient.NamespacedMetrics(ns).List(testMetric, labels.Everything()); err != nil {
			t.Fatalf("Unexpected error getting metric %q: %v", testMetric, err)
		}
	})

	t.Run("to_HPA", func(t *testing.T) {
		t.Parallel()

		cases := map[string]metav1.LabelSelector{
			"when_metric_has_match_labels": {
				MatchLabels: map[string]string{testMetricAttributeKey: testMetricAttributeValue},
			},
			"when_metric_has_match_expression": {
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      testMetricAttributeKey,
						Operator: metav1.LabelSelectorOpExists,
					},
				},
			},
		}
		for testCaseName, testData := range cases {
			testData := testData

			t.Run(testCaseName, func(t *testing.T) {
				t.Parallel()

				ns := withTestNamespace(testEnv.Context, t, clientset)

				deploymentName := withTestDeployment(testEnv.Context, t, clientset.AppsV1().Deployments(ns))

				client := clientset.AutoscalingV2().HorizontalPodAutoscalers(ns)

				hpa := &autoscalingv2.HorizontalPodAutoscaler{
					ObjectMeta: metav1.ObjectMeta{
						GenerateName: "newrelic-metrics-adapter-e2e-test",
						Namespace:    ns,
					},
					Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
						MaxReplicas: 1,
						ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
							Kind:       "Deployment",
							APIVersion: "apps/v1",
							Name:       deploymentName,
						},
						Metrics: []autoscalingv2.MetricSpec{
							{
								Type: autoscalingv2.ExternalMetricSourceType,
								External: &autoscalingv2.ExternalMetricSource{
									Target: autoscalingv2.MetricTarget{
										Type:  "Value",
										Value: resource.NewQuantity(1, resource.DecimalSI),
									},
									Metric: autoscalingv2.MetricIdentifier{
										Name:     testMetric,
										Selector: &testData,
									},
								},
							},
						},
					},
				}

				hpa, err = client.Create(testEnv.Context, hpa, metav1.CreateOptions{})
				if err != nil {
					t.Fatalf("Unexpected error creating HPA object: %v", err)
				}

				t.Cleanup(func() {
					if err := client.Delete(context.Background(), hpa.Name, metav1.DeleteOptions{}); err != nil {
						t.Logf("Failed removing HPA %q: %v", hpa.Name, err)
					}
				})

				if err := wait.PollImmediateUntilWithContext(testEnv.Context, 1*time.Second, func(context.Context) (bool, error) {
					hpa, err = client.Get(testEnv.Context, hpa.Name, metav1.GetOptions{})
					if err != nil {
						t.Fatalf("Getting HPA %q: %v", hpa.Name, err)
					}

					scalingActive := false
					ableToScale := false

					for _, condition := range hpa.Status.Conditions {
						if condition.Status != "True" {
							t.Logf("Ignoring false condition %q: %v", condition.Type, condition.Message)

							continue
						}

						switch condition.Type {
						case autoscalingv2.ScalingActive:
							scalingActive = true
						case autoscalingv2.AbleToScale:
							ableToScale = true
						default:
							t.Logf("Ignoring condition %v", condition)
						}
					}

					return scalingActive && ableToScale, nil
				}); err != nil {
					t.Fatalf("Timed out waiting for HPA to converge: %v", err)
				}
			})
		}
	})
}

//nolint:funlen,cyclop,gocognit,paralleltest // scale_up and scale_down run sequentially for readable output.
func Test_Metrics_adapter_triggers_scaling(t *testing.T) {
	t.Parallel()

	testEnv := &testutil.TestEnv{
		ContextTimeout:  3 * time.Minute,
		StartKubernetes: true,
	}

	testEnv.Generate(t)

	clientset, err := kubernetes.NewForConfig(testEnv.RestConfig)
	if err != nil {
		t.Fatalf("creating clientset: %v", err)
	}

	t.Run("scale_up", func(t *testing.T) {

		ns := withTestNamespace(testEnv.Context, t, clientset)
		deploymentName := withTestDeployment(testEnv.Context, t, clientset.AppsV1().Deployments(ns))

		// Wait for deployment to reach its initial state before HPA fires.
		waitForAvailableReplicas(testEnv.Context, t, clientset.AppsV1().Deployments(ns), deploymentName, 1)

		hpaClient := clientset.AutoscalingV2().HorizontalPodAutoscalers(ns)

		// The e2e metric always returns 0.123. Target of 50m (0.05) is below the
		// metric value, so HPA scales up: ceil(1 * 0.123/0.05) = 3 replicas.
		hpa, err := hpaClient.Create(testEnv.Context, &autoscalingv2.HorizontalPodAutoscaler{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: testPrefix,
				Namespace:    ns,
			},
			Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
				MinReplicas: ptr.To[int32](1),
				MaxReplicas: 3,
				ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
					Kind:       "Deployment",
					APIVersion: "apps/v1",
					Name:       deploymentName,
				},
				Metrics: []autoscalingv2.MetricSpec{
					{
						Type: autoscalingv2.ExternalMetricSourceType,
						External: &autoscalingv2.ExternalMetricSource{
							Target: autoscalingv2.MetricTarget{
								Type:  autoscalingv2.ValueMetricType,
								Value: resource.NewMilliQuantity(50, resource.DecimalSI),
							},
							Metric: autoscalingv2.MetricIdentifier{
								Name: testMetric,
							},
						},
					},
				},
			},
		}, metav1.CreateOptions{})
		if err != nil {
			t.Fatalf("creating HPA: %v", err)
		}

		t.Cleanup(func() {
			if err := hpaClient.Delete(context.Background(), hpa.Name, metav1.DeleteOptions{}); err != nil {
				t.Logf("deleting HPA %q: %v", hpa.Name, err)
			}
		})

		waitForAvailableReplicas(testEnv.Context, t, clientset.AppsV1().Deployments(ns), deploymentName, 3)
	})

	t.Run("scale_down", func(t *testing.T) {

		ns := withTestNamespace(testEnv.Context, t, clientset)

		// Start with 3 replicas; HPA will scale it down to 1.
		deploymentName := withTestDeploymentReplicas(testEnv.Context, t, clientset.AppsV1().Deployments(ns), 3)

		// Wait for deployment to reach its initial state before HPA fires.
		waitForAvailableReplicas(testEnv.Context, t, clientset.AppsV1().Deployments(ns), deploymentName, 3)

		hpaClient := clientset.AutoscalingV2().HorizontalPodAutoscalers(ns)

		// The e2e metric always returns 0.123. Target of 1 is above the metric
		// value, so HPA scales down: ceil(3 * 0.123/1) = 1 replica.
		// stabilizationWindowSeconds: 0 removes the default 5-minute scale-down delay.
		hpa, err := hpaClient.Create(testEnv.Context, &autoscalingv2.HorizontalPodAutoscaler{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: testPrefix,
				Namespace:    ns,
			},
			Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
				MinReplicas: ptr.To[int32](1),
				MaxReplicas: 3,
				ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
					Kind:       "Deployment",
					APIVersion: "apps/v1",
					Name:       deploymentName,
				},
				Metrics: []autoscalingv2.MetricSpec{
					{
						Type: autoscalingv2.ExternalMetricSourceType,
						External: &autoscalingv2.ExternalMetricSource{
							Target: autoscalingv2.MetricTarget{
								Type:  autoscalingv2.ValueMetricType,
								Value: resource.NewQuantity(1, resource.DecimalSI),
							},
							Metric: autoscalingv2.MetricIdentifier{
								Name: testMetric,
							},
						},
					},
				},
				Behavior: &autoscalingv2.HorizontalPodAutoscalerBehavior{
					ScaleDown: &autoscalingv2.HPAScalingRules{
						StabilizationWindowSeconds: ptr.To[int32](0),
					},
				},
			},
		}, metav1.CreateOptions{})
		if err != nil {
			t.Fatalf("creating HPA: %v", err)
		}

		t.Cleanup(func() {
			if err := hpaClient.Delete(context.Background(), hpa.Name, metav1.DeleteOptions{}); err != nil {
				t.Logf("deleting HPA %q: %v", hpa.Name, err)
			}
		})

		waitForAvailableReplicas(testEnv.Context, t, clientset.AppsV1().Deployments(ns), deploymentName, 1)
	})
}

func withTestNamespace(ctx context.Context, t *testing.T, clientset *kubernetes.Clientset) string {
	t.Helper()

	namespaceTemplate := corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: testPrefix,
		},
	}

	namespaceClient := clientset.CoreV1().Namespaces()

	ns, err := namespaceClient.Create(ctx, &namespaceTemplate, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("creating namespace: %v", err)
	}

	namespaceName := ns.Name

	t.Cleanup(func() {
		if err := namespaceClient.Delete(context.Background(), namespaceName, metav1.DeleteOptions{}); err != nil {
			t.Logf("deleting test namespace %q: %v", ns.Name, err)
		}
	})

	return namespaceName
}

func withTestDeployment(ctx context.Context, t *testing.T, client appsv1client.DeploymentInterface) string {
	t.Helper()

	return withTestDeploymentReplicas(ctx, t, client, 1)
}

func withTestDeploymentReplicas(ctx context.Context, t *testing.T, client appsv1client.DeploymentInterface, initialReplicas int32) string {
	t.Helper()

	testLabels := map[string]string{
		"app": "test",
	}

	deploy, err := client.Create(ctx, &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: testPrefix,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: ptr.To(initialReplicas),
			Selector: &metav1.LabelSelector{
				MatchLabels: testLabels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: testLabels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "wait",
							Image: "registry.k8s.io/pause:3.5",
						},
					},
				},
			},
		},
	}, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Creating test Deployment: %v", err)
	}

	return deploy.Name
}

func waitForAvailableReplicas(ctx context.Context, t *testing.T, client appsv1client.DeploymentInterface, name string, expected int32) {
	t.Helper()

	if err := wait.PollImmediateUntilWithContext(ctx, 1*time.Second, func(ctx context.Context) (bool, error) {
		d, err := client.Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			t.Logf("getting deployment %q: %v", name, err)

			return false, nil
		}

		t.Logf("deployment %q: %d/%d available replicas", name, d.Status.AvailableReplicas, expected)

		return d.Status.AvailableReplicas == expected, nil
	}); err != nil {
		t.Fatalf("timed out waiting for %d available replicas on deployment %q: %v", expected, name, err)
	}
}
