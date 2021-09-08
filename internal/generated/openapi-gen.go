// Copyright 2021 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//
// Copyright 2018 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// +build codegen

// Package is only a stub to ensure k8s.io/kube-openapi/cmd/openapi-gen is vendored
// so the same version of kube-openapi is used to generate and render the openapi spec
package main

//go:generate go run -mod=mod k8s.io/kube-openapi/cmd/openapi-gen --logtostderr -i k8s.io/metrics/pkg/apis/external_metrics,k8s.io/metrics/pkg/apis/external_metrics/v1beta1,k8s.io/apimachinery/pkg/apis/meta/v1,k8s.io/apimachinery/pkg/api/resource,k8s.io/apimachinery/pkg/version,k8s.io/api/core/v1 -p ./openapi -h ../../hack/boilerplate.go.txt -O zz_generated.openapi -o ./ -r /dev/null

import (
	_ "k8s.io/kube-openapi/cmd/openapi-gen"
)
