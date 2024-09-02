# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## Unreleased

## v0.13.2 - 2024-09-02

### ⛓️ Dependencies
- Updated github.com/newrelic/newrelic-client-go/v2 to v2.43.2 - [Changelog 🔗](https://github.com/newrelic/newrelic-client-go/releases/tag/v2.43.2)
- Updated k8s.io/klog/v2 to v2.130.1
- Updated go to v1.23.0
- Updated k8s.io/utils digest to f90d014

## v0.13.1 - 2024-07-29

### ⛓️ Dependencies
- Updated alpine to v3.20.2

## v0.13.0 - 2024-06-24

### 🚀 Enhancements
- Add 1.29 and 1.30 support and drop 1.25 and 1.24 @dbudziwojskiNR [#333](https://github.com/newrelic/newrelic-k8s-metrics-adapter/pull/333)

### ⛓️ Dependencies
- Updated alpine to v3.20.1

## v0.12.5 - 2024-06-17

### ⛓️ Dependencies
- Updated go to v1.22.4

## v0.12.4 - 2024-06-10

### ⛓️ Dependencies
- Updated go to v1.22.3

## v0.12.3 - 2024-05-27

### ⛓️ Dependencies
- Updated github.com/newrelic/newrelic-client-go/v2 to v2.34.1 - [Changelog 🔗](https://github.com/newrelic/newrelic-client-go/releases/tag/v2.34.1)
- Updated k8s.io/utils digest to fe8a2dd
- Updated alpine

## v0.12.2 - 2024-03-25

### ⛓️ Dependencies
- Updated k8s.io/utils digest
- Updated github.com/newrelic/newrelic-client-go/v2 to v2.26.1 - [Changelog 🔗](https://github.com/newrelic/newrelic-client-go/releases/tag/v2.26.1)

## v0.12.1 - 2024-03-04

### ⛓️ Dependencies
- Updated github.com/newrelic/newrelic-client-go/v2 to v2.25.0 - [Changelog 🔗](https://github.com/newrelic/newrelic-client-go/releases/tag/v2.25.0)

## v0.12.0 - 2024-02-26

### 🚀 Enhancements
- Add linux node selector @dbudziwojskiNR [#297](https://github.com/newrelic/newrelic-k8s-metrics-adapter/pull/297)

## v0.11.0 - 2024-02-05

### 🚀 Enhancements
- Add Codecov @dbudziwojskiNR [#285](https://github.com/newrelic/newrelic-k8s-metrics-adapter/pull/285)

## v0.10.3 - 2024-01-29

### ⛓️ Dependencies
- Updated k8s.io/klog/v2 to v2.120.1
- Updated alpine to v3.19.1

## v0.10.2 - 2024-01-22

### ⛓️ Dependencies
- Updated go to v1.21.6

## v0.10.1 - 2024-01-09

### ⛓️ Dependencies
- Updated github.com/newrelic/newrelic-client-go/v2 to v2.23.0 - [Changelog 🔗](https://github.com/newrelic/newrelic-client-go/releases/tag/v2.23.0)
- Updated k8s.io/utils digest to e7106e6

## v0.10.0 - 2023-12-09

### 🚀 Enhancements
- Trigger release creation by @juanjjaramillo [#263](https://github.com/newrelic/newrelic-k8s-metrics-adapter/pull/263)

### ⛓️ Dependencies
- Updated go to v1.21.5
- Updated github.com/newrelic/newrelic-client-go to v2
- Updated kubernetes packages to v0.28.4
- Updated k8s.io/utils digest to b307cd5
- Updated github.com/elazarl/goproxy digest
- Updated alpine

## v0.9.0 - 2023-12-06

### 🚀 Enhancements
- Update reusable workflow dependency by @juanjjaramillo [#254](https://github.com/newrelic/newrelic-k8s-metrics-adapter/pull/254)
- Make E2E testing run for all supported K8s versions by @juanjjaramillo in [#253](https://github.com/newrelic/newrelic-k8s-metrics-adapter/pull/253)
- E2E testing: use `autoscaling/v2` instead of `autoscaling/v2beta2` by @juanjjaramillo in [#252](https://github.com/newrelic/newrelic-k8s-metrics-adapter/pull/252)
- Automate local integration and E2E testing by @juanjjaramillo in [#251](https://github.com/newrelic/newrelic-k8s-metrics-adapter/pull/251)

### ⛓️ Dependencies
- Updated alpine to v3.18.5

## v0.8.0 - 2023-11-13

### 🚀 Enhancements
- Replace k8s v1.28.0-rc.1 with k8s 1.28.3 support by @svetlanabrennan in [#245](https://github.com/newrelic/newrelic-k8s-metrics-adapter/pull/245)

## v0.7.0 - 2023-10-30

### 🚀 Enhancements
- Remove 1.23 support by @svetlanabrennan in [#233](https://github.com/newrelic/newrelic-k8s-metrics-adapter/pull/233)
- Bump google.golang.org/grpc from 1.58.2 to 1.58.3 by @svetlanabrennan in [#237](https://github.com/newrelic/newrelic-k8s-metrics-adapter/pull/237)
- Add k8s 1.28.0-rc.1 support by @svetlanabrennan in [#235](https://github.com/newrelic/newrelic-k8s-metrics-adapter/pull/235)

### ⛓️ Dependencies
- Updated sigs.k8s.io/yaml to v1.4.0

## v0.6.4 - 2023-10-23

### 🐞 Bug fixes
- Address CVE-2023-45142 by juanjjaramillo in [#226](https://github.com/newrelic/newrelic-k8s-metrics-adapter/pull/226)

### ⛓️ Dependencies
- Updated sigs.k8s.io/controller-runtime to v0.16.3
- Updated github.com/google/go-cmp to v0.6.0 - [Changelog 🔗](https://github.com/google/go-cmp/releases/tag/v0.6.0)
- Updated k8s.io/metrics to v0.28.3

## v0.6.3 - 2023-10-17

### 🐞 Bug fixes
- Address CVE-2023-3978, CVE-2023-44487 and CVE-2023-39325 by juanjjaramillo in [#224](https://github.com/newrelic/newrelic-k8s-metrics-adapter/pull/224)

## v0.6.2 - 2023-09-30

### ⛓️ Dependencies
- Updated alpine to v3.18.4

## v0.6.1 - 2023-09-28

### 🐞 Bug fixes
- Fix renovate config file by @svetlanabrennan [#211](https://github.com/newrelic/newrelic-k8s-metrics-adapter/pull/211)

## v0.6.0 - 2023-09-26

### 🚀 Enhancements
- Remove old maintainers by @svetlanabrennan [#206](https://github.com/newrelic/newrelic-k8s-metrics-adapter/pull/206)

## v0.5.0 - 2023-09-25

### 🚀 Enhancements
- Fix CI workflow and remove old maintainers by @svetlanabrennan [#198](https://github.com/newrelic/newrelic-k8s-metrics-adapter/pull/198)

### ⛓️ Dependencies
- Updated k8s.io/utils digest
- Updated github.com/elazarl/goproxy digest to f99041a [security]
- Updated alpine to v3.18.3

## 0.4.3

### What's Changed
- Remove manual go cache since setup-go/v4 automatically caches by @htroisi in https://github.com/newrelic/newrelic-k8s-metrics-adapter/pull/149
- Cleanup test-values.yaml by @htroisi in https://github.com/newrelic/newrelic-k8s-metrics-adapter/pull/152
- Bump chart by @htroisi in https://github.com/newrelic/newrelic-k8s-metrics-adapter/pull/150
- Update golangci-lint config to avoid cache contention by @htroisi in https://github.com/newrelic/newrelic-k8s-metrics-adapter/pull/153
- Make the Nrdb Timeout Configurable by @xqi-nr in https://github.com/newrelic/newrelic-k8s-metrics-adapter/pull/142
- Update helm/kind-action action to v1.7.0 by @renovate in https://github.com/newrelic/newrelic-k8s-metrics-adapter/pull/155
- Update aquasecurity/trivy-action action to v0.11.0 by @renovate in https://github.com/newrelic/newrelic-k8s-metrics-adapter/pull/159
- Update aquasecurity/trivy-action action to v0.11.2 by @renovate in https://github.com/newrelic/newrelic-k8s-metrics-adapter/pull/163
- Update Alpine image to address vulnerabilities by @juanjjaramillo in https://github.com/newrelic/newrelic-k8s-metrics-adapter/pull/167

[0.4.3]: https://github.com/newrelic/newrelic-k8s-metrics-adapter/releases/tag/v0.4.3

## 0.4.0

### Added

- Fix driver and CRI used for the various k8s versions in tests
- Update Kubernetes image registry

[0.4.0]: https://github.com/newrelic/newrelic-k8s-metrics-adapter/releases/tag/v0.4.0

## 0.3.0

### Added

- Updated Go version from 1.18 to 1.19
- Updated dependencies

[0.3.0]: https://github.com/newrelic/newrelic-k8s-metrics-adapter/releases/tag/v0.3.0

## 0.2.0

### Added

- Updated dependencies

[0.2.0]: https://github.com/newrelic/newrelic-k8s-metrics-adapter/releases/tag/v0.2.0

## 0.1.0

### Added

- Initial release

[0.1.0]: https://github.com/newrelic/newrelic-k8s-metrics-adapter/releases/tag/v0.1.0
