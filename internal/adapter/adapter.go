// Copyright 2021 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

// Package adapter exports top-level adapter logic.
package adapter

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/spf13/pflag"
	openapinamer "k8s.io/apiserver/pkg/endpoints/openapi"
	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/klog/v2"
	"sigs.k8s.io/custom-metrics-apiserver/pkg/apiserver"
	basecmd "sigs.k8s.io/custom-metrics-apiserver/pkg/cmd"
	o "sigs.k8s.io/custom-metrics-apiserver/pkg/cmd/options"
	"sigs.k8s.io/custom-metrics-apiserver/pkg/provider"

	generatedopenapi "github.com/newrelic/newrelic-k8s-metrics-adapter/internal/generated/openapi"
)

const (
	// Name of the adapter.
	Name = "newrelic-k8s-metrics-adapter"
	// DefaultSecurePort is a default port adapter will be listening on using HTTPS.
	DefaultSecurePort = 6443
)

var version = "dev" //nolint:gochecknoglobal // Version is set at building time.

// Options holds the configuration for the adapter.
type Options struct {
	Args                    []string
	ExtraFlags              *pflag.FlagSet
	ExternalMetricsProvider provider.ExternalMetricsProvider
}

type adapter struct {
	basecmd.AdapterBase
}

// Adapter represents adapter functionality.
type Adapter interface {
	Run(context.Context) error
}

// NewAdapter validates given adapter options and creates new runnable adapter instance.
func NewAdapter(options Options) (Adapter, error) {
	a := &adapter{}
	// Used as identifier in logs with -v=6, defaults to "custom-metrics-adapter", so we want to override that.
	a.Name = Name

	a.OpenAPIConfig = genericapiserver.DefaultOpenAPIConfig(
		generatedopenapi.GetOpenAPIDefinitions,
		openapinamer.NewDefinitionNamer(apiserver.Scheme),
	)
	a.OpenAPIConfig.Info.Title = a.Name
	a.OpenAPIConfig.Info.Version = version

	// Initialize part of the struct by hand to be able to specify default secure port.
	a.CustomMetricsAdapterServerOptions = o.NewCustomMetricsAdapterServerOptions()
	a.CustomMetricsAdapterServerOptions.OpenAPIConfig = a.OpenAPIConfig
	a.SecureServing.BindPort = DefaultSecurePort

	if err := ParseFlags(options.Args, options.ExtraFlags, &a.AdapterBase); err != nil {
		return nil, fmt.Errorf("initiating flags: %w", err)
	}

	if options.ExternalMetricsProvider == nil {
		return nil, fmt.Errorf("external metrics provider must be configured")
	}

	a.WithExternalMetrics(options.ExternalMetricsProvider)

	return a, nil
}

// ParseFlags parses given arguments as custom custom-metrics-apiserver into given adapter base.
//
// It also allows specifying extra flags if one wants to add extra flags to API server built on top
// of adapter base.
//
// If adapter is nil, temporary adapter will be used.
//
// Extra flags are also optional.
func ParseFlags(args []string, extraFlags *pflag.FlagSet, adapter *basecmd.AdapterBase) error {
	if adapter == nil {
		adapter = &basecmd.AdapterBase{}
	}

	if adapter.FlagSet == nil {
		adapter.FlagSet = pflag.NewFlagSet(Name, pflag.ContinueOnError)
	}

	adapter.FlagSet.AddFlagSet(extraFlags)

	// Add flags from klog to be able to control log level etc.
	klogFlagSet := &flag.FlagSet{}
	klog.InitFlags(klogFlagSet)
	adapter.FlagSet.AddGoFlagSet(klogFlagSet)

	if err := adapter.Flags().Parse(args); err != nil {
		return fmt.Errorf("parsing flags %q: %w", strings.Join(args, ","), err)
	}

	return nil
}
