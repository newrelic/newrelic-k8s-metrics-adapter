// Copyright 2021 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package adapter_test

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/newrelic/newrelic-k8s-metrics-adapter/internal/adapter"
	"github.com/newrelic/newrelic-k8s-metrics-adapter/internal/provider/mock"
)

func Test_Creating_adapter(t *testing.T) { //nolint:funlen // Just a lot of test cases.
	t.Parallel()

	t.Run("accepts_generic_API_server_flags", func(t *testing.T) {
		t.Parallel()

		securePortFlag := "--secure-port"

		options := testOptions()
		options.Args = []string{securePortFlag + "=6443"}

		if _, err := adapter.NewAdapter(options); err != nil {
			t.Fatalf("Expected adapter to accept API server flags like %q, got: %v", securePortFlag, err)
		}
	})

	t.Run("accepts_klog_flags", func(t *testing.T) {
		t.Parallel()

		verbosityFlag := "-v"

		options := testOptions()
		options.Args = []string{verbosityFlag + "=4"}

		if _, err := adapter.NewAdapter(options); err != nil {
			t.Fatalf("Expected adapter to accept klog flags like %q, got: %v", verbosityFlag, err)
		}
	})

	//nolint:paralleltest // This test overrides stderr, which is global.
	t.Run("includes_adapter_name_in_usage_message", func(t *testing.T) {
		helpFlag := "-help"

		options := testOptions()
		options.Args = []string{helpFlag}

		output, err := captureStderr(t, func() error {
			_, err := adapter.NewAdapter(options)

			return err //nolint:wrapcheck // We don't care about wrapping in tests.
		})

		if err == nil {
			t.Fatalf("Expected error when passing help flag %q", helpFlag)
		}

		expectedUsage := fmt.Sprintf("Usage of %s", adapter.Name)

		if !strings.Contains(output, expectedUsage) {
			t.Fatalf("Expected usage message to contain %q, got \n\n%s", expectedUsage, output)
		}
	})

	t.Run("returns_error_when", func(t *testing.T) {
		t.Parallel()

		t.Run("undefined_flag_is_provided", func(t *testing.T) {
			t.Parallel()

			options := testOptions()
			options.Args = []string{"--non-existent-flag"}

			if _, err := adapter.NewAdapter(options); err == nil {
				t.Fatalf("Expected error")
			}
		})

		t.Run("no_external_metrics_provider_is_configured", func(t *testing.T) {
			t.Parallel()

			options := testOptions()
			options.ExternalMetricsProvider = nil

			if _, err := adapter.NewAdapter(options); err == nil {
				t.Fatalf("Expected error")
			}
		})
	})
}

func testOptions() adapter.Options {
	return adapter.Options{
		ExternalMetricsProvider: &mock.Provider{},
	}
}

// captureStderr is helper function for testing functions, which do not offer specifying where they should write
// output and write only to os.Stderr. Wrapping such functions using this allows to capture their output as string.
func captureStderr(t *testing.T, printFunction func() error) (string, error) {
	t.Helper()

	// Keep backup of the real stdout.
	old := os.Stderr

	// Get new pipe to temporarily override os.Stderr so we can capture the output.
	r, w, _ := os.Pipe()
	os.Stderr = w

	err := printFunction()

	outC := make(chan string)

	// Copy the output in a separate goroutine so printing can't block indefinitely.
	go func() {
		var buf bytes.Buffer
		if _, err := io.Copy(&buf, r); err != nil {
			t.Logf("Copying strerr: %v", err)
			t.Fail()
		}
		outC <- buf.String()
	}()

	// Back to normal state.
	if err := w.Close(); err != nil {
		t.Fatalf("Closing pipe writer: %v", err)
	}

	// Restoring the real stdout.
	os.Stderr = old

	return <-outC, err
}
