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

	"github.com/spf13/pflag"
	basecmd "sigs.k8s.io/custom-metrics-apiserver/pkg/cmd"

	"github.com/newrelic/newrelic-k8s-metrics-adapter/internal/adapter"
	"github.com/newrelic/newrelic-k8s-metrics-adapter/internal/provider/mock"
)

func Test_Creating_adapter(t *testing.T) { //nolint:funlen // Just a lot of test cases.
	t.Parallel()

	t.Run("accepts_generic_API_server_flags", func(t *testing.T) {
		t.Parallel()

		securePortFlag := "--secure-port"

		options := testOptions()
		options.Args = []string{securePortFlag + "=12345"}

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

	t.Run("parses_configured_extra_flags", func(t *testing.T) {
		t.Parallel()

		flagSet := pflag.NewFlagSet("foo", pflag.ContinueOnError)
		customFlagValue := flagSet.String("custom-flag", "default-value", "description")

		expectedValue := "bar"

		options := testOptions()
		options.ExtraFlags = flagSet
		options.Args = []string{"--custom-flag=" + expectedValue}

		if _, err := adapter.NewAdapter(options); err != nil {
			t.Fatalf("Expected adapter to accept custom flags, got: %v", err)
		}

		if *customFlagValue != expectedValue {
			t.Fatalf("Expected custom flag to have value %q, got %q", expectedValue, *customFlagValue)
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

func Test_Parsing_flags(t *testing.T) {
	t.Parallel()

	t.Run("parses_flags_into_temporary_adapter_if_none_is_given", func(t *testing.T) {
		t.Parallel()

		extraFlags := pflag.NewFlagSet("foo", pflag.ContinueOnError)

		if err := adapter.ParseFlags([]string{"-v=1"}, extraFlags, nil); err != nil {
			t.Fatalf("Expected persing flags to succeed, got: %v", err)
		}
	})

	t.Run("does_not_require_extra_flags_to_be_specified", func(t *testing.T) {
		t.Parallel()

		a := &basecmd.AdapterBase{}

		if err := adapter.ParseFlags([]string{"-v=1"}, nil, a); err != nil {
			t.Fatalf("Expected persing flags to succeed, got: %v", err)
		}
	})

	t.Run("parses_flags_into_given_adapter", func(t *testing.T) {
		t.Parallel()

		extraFlags := pflag.NewFlagSet("foo", pflag.ContinueOnError)

		a := &basecmd.AdapterBase{}

		expectedSecurePort := "12345"

		if err := adapter.ParseFlags([]string{"--secure-port=" + expectedSecurePort}, extraFlags, a); err != nil {
			t.Fatalf("Expected persing flags to succeed, got: %v", err)
		}

		if securePort := fmt.Sprintf("%d", a.SecureServing.BindPort); securePort != expectedSecurePort {
			t.Fatalf("Expected adapter to have secure port configured to %q, got %q", expectedSecurePort, securePort)
		}
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
