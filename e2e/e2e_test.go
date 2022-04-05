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
	autoscalingv2beta2 "k8s.io/api/autoscaling/v2beta2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	appsv1client "k8s.io/client-go/kubernetes/typed/apps/v1"
	eclient "k8s.io/metrics/pkg/client/external_metrics"

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

				client := clientset.AutoscalingV2beta2().HorizontalPodAutoscalers(ns)

				hpa := &autoscalingv2beta2.HorizontalPodAutoscaler{
					ObjectMeta: metav1.ObjectMeta{
						GenerateName: "newrelic-metrics-adapter-e2e-test",
						Namespace:    ns,
					},
					Spec: autoscalingv2beta2.HorizontalPodAutoscalerSpec{
						MaxReplicas: 1,
						ScaleTargetRef: autoscalingv2beta2.CrossVersionObjectReference{
							Kind:       "Deployment",
							APIVersion: "apps/v1",
							Name:       deploymentName,
						},
						Metrics: []autoscalingv2beta2.MetricSpec{
							{
								Type: autoscalingv2beta2.ExternalMetricSourceType,
								External: &autoscalingv2beta2.ExternalMetricSource{
									Target: autoscalingv2beta2.MetricTarget{
										Type:  "Value",
										Value: resource.NewQuantity(1, resource.DecimalSI),
									},
									Metric: autoscalingv2beta2.MetricIdentifier{
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
					if err := client.Delete(testEnv.Context, hpa.Name, metav1.DeleteOptions{}); err != nil {
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
						case autoscalingv2beta2.ScalingActive:
							scalingActive = true
						case autoscalingv2beta2.AbleToScale:
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
		if err := namespaceClient.Delete(ctx, namespaceName, metav1.DeleteOptions{}); err != nil {
			t.Logf("deleting test namespace %q: %v", ns.Name, err)
		}
	})

	return namespaceName
}

func withTestDeployment(ctx context.Context, t *testing.T, client appsv1client.DeploymentInterface) string {
	t.Helper()

	testLabels := map[string]string{
		"app": "test",
	}

	deployTemplate := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: testPrefix,
		},
		Spec: appsv1.DeploymentSpec{
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
							Image: "k8s.gcr.io/pause:3.5",
						},
					},
				},
			},
		},
	}

	deploy, err := client.Create(ctx, &deployTemplate, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Creating test Deployment: %v", err)
	}

	return deploy.Name
}
