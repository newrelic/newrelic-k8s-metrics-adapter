// Copyright 2021 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package main_test

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	adapter "github.com/gsanchezgavier/metrics-adapter"
)

//nolint:paralleltest // We manipulate environment variables here which are global.
func Test_Run_reads_API_key_and_cluster_name_from_environment_variable(t *testing.T) {
	setenv(t, adapter.NewRelicAPIKeyEnv, "foo")
	setenv(t, adapter.ClusterNameEnv, "bar")

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := ioutil.WriteFile(configPath, []byte("accountID: 1"), 0o600); err != nil {
		t.Fatalf("Error writing test config file: %v", err)
	}

	err := adapter.Run(testContext(t), configPath, []string{"--cert-dir=" + t.TempDir()})
	if err == nil {
		t.Fatalf("Expected error running adapter")
	}

	if !strings.Contains(err.Error(), "failed to listen") {
		t.Fatalf("Expected error %v, got %v", context.DeadlineExceeded, err)
	}
}

//nolint:funlen // Just many test cases.
func Test_Run_fails_when(t *testing.T) {
	t.Parallel()

	t.Run("config_file_is_not_readable", func(t *testing.T) {
		t.Parallel()

		if err := adapter.Run(testContext(t), t.TempDir(), []string{"--cert-dir=" + t.TempDir()}); err == nil {
			t.Fatalf("Expected error running adapter")
		}
	})

	t.Run("config_file_is_not_valid_YAML", func(t *testing.T) {
		t.Parallel()

		configPath := filepath.Join(t.TempDir(), "config.yaml")
		if err := ioutil.WriteFile(configPath, []byte("badKey: 1"), 0o600); err != nil {
			t.Fatalf("Error writing test config file: %v", err)
		}

		if err := adapter.Run(testContext(t), configPath, []string{"--cert-dir=" + t.TempDir()}); err == nil {
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

		if err := adapter.Run(testContext(t), configPath, []string{"--cert-dir=" + t.TempDir()}); err == nil {
			t.Fatalf("Expected error running adapter")
		}
	})

	//nolint:paralleltest // We manipulate environment variables here which are global.
	t.Run("initializing_direct_metric_provider_fails", func(t *testing.T) {
		setenv(t, adapter.NewRelicAPIKeyEnv, "foo")
		unsetenv(t, adapter.ClusterNameEnv)

		configPath := filepath.Join(t.TempDir(), "config.yaml")
		if err := ioutil.WriteFile(configPath, []byte("accountID: 0"), 0o600); err != nil {
			t.Fatalf("Error writing test config file: %v", err)
		}

		if err := adapter.Run(testContext(t), configPath, []string{"--cert-dir=" + t.TempDir()}); err == nil {
			t.Fatalf("Expected error running adapter")
		}
	})

	//nolint:paralleltest // We manipulate environment variables here which are global.
	t.Run("intializing_adapter_fails", func(t *testing.T) {
		setenv(t, adapter.NewRelicAPIKeyEnv, "foo")
		setenv(t, adapter.ClusterNameEnv, "bar")

		configPath := filepath.Join(t.TempDir(), "config.yaml")
		if err := ioutil.WriteFile(configPath, []byte("accountID: 1"), 0o600); err != nil {
			t.Fatalf("Error writing test config file: %v", err)
		}

		if err := adapter.Run(testContext(t), configPath, []string{"-help"}); err == nil {
			t.Fatalf("Expected error running adapter")
		}
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
