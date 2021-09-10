// Copyright 2021 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

// Package adapter exports top-level adapter logic.
package adapter

import (
	"flag"
	"fmt"
	"strings"

	"github.com/spf13/pflag"
	openapinamer "k8s.io/apiserver/pkg/endpoints/openapi"
	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/klog/v2"
	"sigs.k8s.io/custom-metrics-apiserver/pkg/apiserver"
	basecmd "sigs.k8s.io/custom-metrics-apiserver/pkg/cmd"

	generatedopenapi "github.com/gsanchezgavier/metrics-adapter/internal/generated/openapi"
	"github.com/gsanchezgavier/metrics-adapter/internal/provider"
)

const adapterName = "newrelic-k8s-metrics-adapter"

var version = "dev" //nolint:gochecknoglobal // Version is set at building time.

// Options holds the configuration for the adapter.
type Options struct {
	Args []string
}

type adapter struct {
	basecmd.AdapterBase
}

// Adapter represents adapter functionality.
type Adapter interface {
	Run(<-chan struct{}) error
}

// NewAdapter validates given adapter options and creates new runnable adapter instance.
func NewAdapter(options Options) (Adapter, error) {
	adapter := &adapter{}
	adapter.Name = adapterName

	adapter.OpenAPIConfig = genericapiserver.DefaultOpenAPIConfig(
		generatedopenapi.GetOpenAPIDefinitions,
		openapinamer.NewDefinitionNamer(apiserver.Scheme),
	)
	adapter.OpenAPIConfig.Info.Title = adapter.Name
	adapter.OpenAPIConfig.Info.Version = version

	if err := adapter.initFlags(options.Args); err != nil {
		return nil, fmt.Errorf("initiating flags: %w", err)
	}

	adapter.WithExternalMetrics(&provider.Provider{})

	return adapter, nil
}

func (a *adapter) initFlags(args []string) error {
	a.FlagSet = pflag.NewFlagSet("newFlagSet", pflag.ExitOnError)

	// Add flags from klog to be able to control log level etc.
	klogFlagSet := &flag.FlagSet{}
	klog.InitFlags(klogFlagSet)
	a.FlagSet.AddGoFlagSet(klogFlagSet)

	if err := a.Flags().Parse(args); err != nil {
		return fmt.Errorf("parsing flags %q: %w", strings.Join(args, ","), err)
	}

	return nil
}
