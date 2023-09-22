# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## Unreleased

### enhancement
- Fix CI workflow and remove old maintainers by @svetlanabrennan [#198](https://github.com/newrelic/newrelic-k8s-metrics-adapter/pull/198)

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
