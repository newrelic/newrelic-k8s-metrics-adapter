// Copyright 2021 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

//go:build integration
// +build integration

package adapter_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/newrelic/newrelic-k8s-metrics-adapter/internal/adapter"
	"github.com/newrelic/newrelic-k8s-metrics-adapter/internal/provider/mock"
	"github.com/newrelic/newrelic-k8s-metrics-adapter/internal/testutil"
)

const (
	// Each of integration tests should not take more than 10 seconds to complete
	// under normal circumstances. This should make detecting bugs faster.
	testMaxExecutionTime = 10 * time.Second
)

func Test_Adapter_responds_to(t *testing.T) {
	t.Parallel()

	testEnv := testEnv(t)

	testEnv.Flags = append(testEnv.Flags, "--v=2")

	runAdapter(t, testEnv)

	t.Run("metric_request", func(t *testing.T) {
		t.Parallel()

		url := fmt.Sprintf("%s/apis/external.metrics.k8s.io/v1beta1", testEnv.BaseURL)

		testutil.CheckStatusCodeOK(testEnv.Context, t, testEnv.HTTPClient, url)
	})

	t.Run("openAPI", func(t *testing.T) {
		t.Parallel()

		url := fmt.Sprintf("%s/openapi/v2", testEnv.BaseURL)

		body := testutil.CheckStatusCodeOK(testEnv.Context, t, testEnv.HTTPClient, url)

		t.Run("with_valid_title", func(t *testing.T) {
			t.Parallel()

			openAPISpec := &struct {
				Info struct {
					Title string
				}
			}{}

			if err := json.Unmarshal(body, openAPISpec); err != nil {
				t.Fatalf("OpenAPI spec is not a valid JSON: %v", err)
			}

			expectedTitle := adapter.Name

			if openAPISpec.Info.Title != expectedTitle {
				t.Fatalf("Expected OpenAPI spec title %q, got %q", expectedTitle, openAPISpec.Info.Title)
			}
		})
	})
}

func Test_Adapter_listens_on_non_privileged_port_by_default(t *testing.T) {
	t.Parallel()

	if expectedMinPort := 1024; adapter.DefaultSecurePort < expectedMinPort {
		t.Fatalf("Default adapter port use privileged port, got %d, expected >%d",
			adapter.DefaultSecurePort, expectedMinPort)
	}

	testEnv := testEnv(t)

	// Remove --secure-port flag to attempt to use default port.
	flags := []string{}

	for _, f := range testEnv.Flags {
		if !strings.Contains(f, "--secure-port") {
			flags = append(flags, f)
		}
	}

	testEnv.Flags = flags

	runAdapter(t, testEnv)

	url := fmt.Sprintf("https://%s:%d/healthz", testEnv.Host, adapter.DefaultSecurePort)

	testutil.CheckStatusCodeOK(testEnv.Context, t, testEnv.HTTPClient, url)
}

func testEnv(t *testing.T) *testutil.TestEnv {
	t.Helper()

	testEnv := &testutil.TestEnv{
		ContextTimeout:  testMaxExecutionTime,
		StartKubernetes: true,
	}

	testEnv.Generate(t)

	return testEnv
}

func runAdapter(t *testing.T, testEnv *testutil.TestEnv) {
	t.Helper()

	options := adapter.Options{
		ExternalMetricsProvider: &mock.Provider{},
		Args:                    testEnv.Flags,
	}

	adapter, err := adapter.NewAdapter(options)
	if err != nil {
		t.Fatalf("Creating adapter: %v", err)
	}

	go func() {
		if err := adapter.Run(testEnv.Context.Done()); err != nil {
			t.Logf("Running operator: %v\n", err)
			t.Fail()
		}
	}()
}
