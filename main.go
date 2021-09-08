// Copyright 2021 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"

	"k8s.io/component-base/logs"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"

	"github.com/gsanchezgavier/metrics-adapter/internal/adapter"
)

func main() {
	logs.InitLogs()
	defer logs.FlushLogs()

	klog.Infof("Starting NewRelic metrics adapter")

	options := adapter.Options{
		Args: os.Args,
	}
	if err := adapter.Run(signals.SetupSignalHandler(), options); err != nil {
		klog.Fatalf("Running adapter: %v", err)
	}
}
