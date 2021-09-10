// Copyright 2021 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package adapter_test

import (
	"testing"

	"github.com/gsanchezgavier/metrics-adapter/internal/adapter"
	"github.com/gsanchezgavier/metrics-adapter/internal/provider/mock"
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
}

func testOptions() adapter.Options {
	return adapter.Options{
		ExternalMetricsProvider: &mock.Provider{},
	}
}
