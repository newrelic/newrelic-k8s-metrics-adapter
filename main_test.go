// Copyright 2021 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package main_test

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"k8s.io/component-base/metrics"
	"k8s.io/component-base/metrics/legacyregistry"

	adapter "github.com/newrelic/newrelic-k8s-metrics-adapter"
	"github.com/newrelic/newrelic-k8s-metrics-adapter/internal/provider/cache"
)

//nolint:paralleltest // We manipulate environment variables here which are global.
func Test_Run_reads_API_key_and_cluster_name_from_environment_variable(t *testing.T) {
	setenv(t, adapter.NewRelicAPIKeyEnv, "foo")
	setenv(t, adapter.ClusterNameEnv, "bar")
	withoutGlobalMetricsRegistry(t)

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := ioutil.WriteFile(configPath, []byte("accountID: 1"), 0o600); err != nil {
		t.Fatalf("Error writing test config file: %v", err)
	}

	flags := []string{"--cert-dir=" + t.TempDir(), "--secure-port=0", "--config-file=" + configPath}

	err := adapter.Run(testContext(t), flags)
	if err == nil {
		t.Fatalf("Expected error running adapter")
	}

	expectedError := "failed to get delegated authentication kubeconfig"

	if !strings.Contains(err.Error(), expectedError) {
		t.Fatalf("Expected error %q, got: %v", expectedError, err)
	}
}

func Test_Run_does_not_return_error_when_help_flag_is_specified(t *testing.T) {
	t.Parallel()

	if err := adapter.Run(testContext(t), []string{"--help"}); err != nil {
		t.Fatalf("Unexpected error running adapter: %v", err)
	}
}

//nolint:funlen,cyclop // Just many test cases.
func Test_Run_fails_when(t *testing.T) {
	t.Parallel()

	t.Run("config_file_is_not_readable", func(t *testing.T) {
		t.Parallel()

		flags := []string{"--cert-dir=" + t.TempDir(), "--config-file=" + t.TempDir()}

		if err := adapter.Run(testContext(t), flags); err == nil {
			t.Fatalf("Expected error running adapter")
		}
	})

	t.Run("config_file_is_not_valid_YAML", func(t *testing.T) {
		t.Parallel()

		configPath := filepath.Join(t.TempDir(), "config.yaml")
		if err := ioutil.WriteFile(configPath, []byte("badKey: 1"), 0o600); err != nil {
			t.Fatalf("Error writing test config file: %v", err)
		}

		flags := []string{"--cert-dir=" + t.TempDir(), "--config-file=" + configPath}

		if err := adapter.Run(testContext(t), flags); err == nil {
			t.Fatalf("Expected error running adapter")
		}
	})

	//nolint:paralleltest // We manipulate environment variables here which are global.
	t.Run("initializing_NewRelic_client_fails", func(t *testing.T) {
		unsetenv(t, adapter.NewRelicAPIKeyEnv)

		configPath := filepath.Join(t.TempDir(), "config.yaml")
		if err := ioutil.WriteFile(configPath, []byte("accountID: 1"), 0o600); err != nil {
			t.Fatalf("Error writing test config file: %v", err)
		}

		flags := []string{"--cert-dir=" + t.TempDir(), "--config-file=" + configPath}

		if err := adapter.Run(testContext(t), flags); err == nil {
			t.Fatalf("Expected error running adapter")
		}
	})

	//nolint:paralleltest // We manipulate environment variables here which are global.
	t.Run("initializing_direct_metric_provider_fails", func(t *testing.T) {
		setenv(t, adapter.NewRelicAPIKeyEnv, "foo")
		unsetenv(t, adapter.ClusterNameEnv)
		withoutGlobalMetricsRegistry(t)

		configPath := filepath.Join(t.TempDir(), "config.yaml")
		if err := ioutil.WriteFile(configPath, []byte("accountID: 0"), 0o600); err != nil {
			t.Fatalf("Error writing test config file: %v", err)
		}

		flags := []string{"--cert-dir=" + t.TempDir(), "--config-file=" + configPath}

		if err := adapter.Run(testContext(t), flags); err == nil {
			t.Fatalf("Expected error running adapter")
		}
	})

	t.Run("parsing_given_flags_fails", func(t *testing.T) {
		t.Parallel()

		if err := adapter.Run(testContext(t), []string{"--secure-port=foo"}); err == nil {
			t.Fatalf("Expected error running adapter")
		}
	})

	//nolint:paralleltest // We manipulate environment variables here which are global.
	t.Run("invalid_region_is_configured", func(t *testing.T) {
		setenv(t, adapter.NewRelicAPIKeyEnv, "foo")
		setenv(t, adapter.ClusterNameEnv, "bar")

		configPath := filepath.Join(t.TempDir(), "config.yaml")
		if err := ioutil.WriteFile(configPath, []byte("accountID: 1\nregion: BAR"), 0o600); err != nil {
			t.Fatalf("Error writing test config file: %v", err)
		}

		flags := []string{"--cert-dir=" + t.TempDir(), "--config-file=" + configPath}

		if err := adapter.Run(testContext(t), flags); err == nil {
			t.Fatalf("Expected error running adapter")
		}
	})

	//nolint:paralleltest // We manipulate environment variables here which are global.
	t.Run("initializing_cache_provider_fails", func(t *testing.T) {
		setenv(t, adapter.NewRelicAPIKeyEnv, "foo")
		setenv(t, adapter.ClusterNameEnv, "bar")

		configPath := filepath.Join(t.TempDir(), "config.yaml")
		if err := ioutil.WriteFile(configPath, []byte("accountID: 1\ncacheTTLSeconds: 10\n"), 0o600); err != nil {
			t.Fatalf("Error writing test config file: %v", err)
		}

		expectedError := "sample error"
		reg := legacyregistry.Register

		legacyregistry.Register = func(m metrics.Registerable) error {
			t.Log(m.FQName())

			if strings.Contains(m.FQName(), cache.MetricsSubsystem) {
				return fmt.Errorf(expectedError)
			}

			return nil
		}

		t.Cleanup(func() {
			legacyregistry.Register = reg
		})

		err := adapter.Run(testContext(t), []string{"--cert-dir=" + t.TempDir(), "--config-file=" + configPath})
		if err == nil {
			t.Fatalf("Expected error running adapter")
		}

		if !strings.Contains(err.Error(), expectedError) {
			t.Fatalf("Expected error to contain %q, got %q", expectedError, err.Error())
		}
	})
}

func withoutGlobalMetricsRegistry(t *testing.T) {
	t.Helper()

	reg := legacyregistry.Register

	legacyregistry.Register = nil

	t.Cleanup(func() {
		legacyregistry.Register = reg
	})
}

func testContext(t *testing.T) context.Context {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)

	t.Cleanup(func() {
		cancel()
	})

	return ctx
}

func setenv(t *testing.T, key, value string) {
	t.Helper()

	if err := os.Setenv(key, value); err != nil {
		t.Fatalf("Setting environment variable %q to %q: %v", key, value, err)
	}
}

func unsetenv(t *testing.T, key string) {
	t.Helper()

	if err := os.Unsetenv(key); err != nil {
		t.Fatalf("Unsetting environment variable %q: %v", key, err)
	}
}
