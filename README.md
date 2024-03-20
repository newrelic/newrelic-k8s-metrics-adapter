<a href="https://opensource.newrelic.com/oss-category/#community-plus"><picture><source media="(prefers-color-scheme: dark)" srcset="https://github.com/newrelic/opensource-website/raw/main/src/images/categories/dark/Community_Plus.png"><source media="(prefers-color-scheme: light)" srcset="https://github.com/newrelic/opensource-website/raw/main/src/images/categories/Community_Plus.png"><img alt="New Relic Open Source community plus project banner." src="https://github.com/newrelic/opensource-website/raw/main/src/images/categories/Community_Plus.png"></picture></a>

# newrelic-k8s-metrics-adapter [![codecov](https://codecov.io/gh/newrelic/newrelic-k8s-metrics-adapter/graph/badge.svg?token=P5WVUPZ6US)](https://codecov.io/gh/newrelic/newrelic-k8s-metrics-adapter)

The newrelic-k8s-metrics-adapter implements the `external.metrics.k8s.io` API to support the use of external metrics based New Relic NRQL queries. 

During installation, a set of metrics can be configured to be available for `Horizontal Pod Autoscalers` to be used. Once deployed, the metrics values are fetched from the configured New Relic account using the NerdGraph API and the configured NRQL query.

The adapter uses the [Custom Metrics Adapter Server Boilerplate](https://github.com/kubernetes-sigs/custom-metrics-apiserver) as a base code to implement the external metric api server.


## Installation

This project has the `newrelic-k8s-metrics-adapter` Helm Chart in [charts/newrelic-k8s-metrics-adapter](https://github.com/newrelic/newrelic-k8s-metrics-adapter/tree/main/charts/newrelic-k8s-metrics-adapter) and can also be installed through the `nri-bundle` chart in [newrelic helm charts repo](https://github.com/newrelic/helm-charts).

For further information regarding the installation refer to the official docs and to the README.md and the values.yaml of the [chart](https://github.com/newrelic/newrelic-k8s-metrics-adapter/tree/main/charts/newrelic-k8s-metrics-adapter).

## Getting Started

In order to start using the adapter, please start by [installing](#Installation) and configuring the adapter using the provided Helm Chart. After this, metrics will be available for consumption by `Horizontal Pod Autoscaler` using the configured metric names. For further information regarding the usage of the adapter refer to the official [docs](https://docs.newrelic.com/docs/integrations/kubernetes-integration/installation/).

### Develop, Test and Run Locally

For the development process [kind](https://kind.sigs.k8s.io) and [tilt](https://tilt.dev/) tools are used.

* [Install kind](https://kind.sigs.k8s.io/docs/user/quick-start/#installation)
* [Install Tilt](https://docs.tilt.dev/install.html)

#### Building

To build the image:
```sh
GOOS=linux make image
```

To build the binary:
```sh
GOOS=linux make build
```

#### Configure Tilt

If you want to use a `kind` cluster for testing, configure Tilt using the command below:

```sh
cat <<EOF > tilt_option.json
{
  "default_registry": "localhost:5000"
}
EOF
```

If you want to use existing Kubernetes cluster, create `tilt_option.json` file with content similar to below:

```json
{
  "default_registry": "quay.io/<your username>",
  "allowed_contexts": "<kubeconfig context to use>"
}
```

#### Creating kind cluster

If you want to use a local `kind` cluster for testing, create it with command below:

```sh
make kind-up
```

#### Run

If you use a `kind` cluster, simply run:

```sh
make tilt-up
```

If you deploy on external cluster, run the command below, pointing `TILT_KUBECONFIG` to your `kubeconfig` file:

```sh
TILT_KUBECONFIG=~/.kube/config make tilt-down
```

Now, when you make changes to the code, the metrics adapter binary will be built locally, copied to the Pod, and then executed.

#### Testing

In order to run unit tests run:
```sh
make test
```
In order to run integration tests locally, you can use `test-integration.sh`. To get help on usage call the script with the `--help` flag:
```sh
./test-integration.sh --help
```
In order to run E2E tests locally, you can use `test-e2e.sh`. To get help on usage call the script with the `--help` flag:
```sh
./test-e2e.sh --help
```

#### Personalized tests
Sometimes you may need extra flexibility on how to run tests. Here are the instructions to allow you to personalize the test experience.

In order to run integration and e2e tests run:

```sh
make test-integration
make test-e2e
```

Notice that in order to run both integration tests and e2e is required to configure access to a New Relic account. This can be done either via environment variable directly or by putting required environment variables into `.env` file, which will be read by `Makefile` and they will be used for other commands.

`.env` example content looks like following:
```sh
NEWRELIC_API_KEY=NRAK-XXX
NEWRELIC_ACCOUNT_ID=1
#NEWRELIC_REGION=EU
NEWRELIC_CLUSTER_NAME=my-cluster
```

Also in order to run e2e tests, you will need a working environment available with the `newrelic-k8s-metrics-adapter` running. Both installing the `newrelic-k8s-metrics-adapter` chart or spinning up the environment with `make tilt-up` are possible options.

It is also possible to run such tests against any cluster you have access to by setting the environment variable `TEST_KUBECONFIG=/your/kube/config/path`. 
## Support

* [New Relic Documentation](https://docs.newrelic.com): Comprehensive guidance for using our platform
* [New Relic Community](https://forum.newrelic.com/): The best place to engage in troubleshooting questions
* [New Relic Developer](https://developer.newrelic.com/): Resources for building a custom observability applications
* [New Relic University](https://learn.newrelic.com/): A range of online training for New Relic users of every level
* [New Relic Technical Support](https://support.newrelic.com/) 24/7/365 ticketed support. Read more about our [Technical Support Offerings](https://docs.newrelic.com/docs/licenses/license-information/general-usage-licenses/support-plan).

## Contribute

We encourage your contributions to improve newrelic-k8s-metrics-adapter! Keep in mind that when you submit your pull request, you'll need to sign the CLA via the click-through using CLA-Assistant. You only have to sign the CLA one time per project.

If you have any questions, or to execute our corporate CLA (which is required if your contribution is on behalf of a company), drop us an email at opensource@newrelic.com.

**A note about vulnerabilities**

As noted in our [security policy](../../security/policy), New Relic is committed to the privacy and security of our
customers and their data. We believe that providing coordinated disclosure by security researchers and engaging with
the security community are important means to achieve our security goals.

If you believe you have found a security vulnerability in this project or any of New Relic's products or websites,
we welcome and greatly appreciate you reporting it to New Relic through [our bug bounty program](https://docs.newrelic.com/docs/security/security-privacy/information-security/report-security-vulnerabilities/).

If you would like to contribute to this project, review [these guidelines](./CONTRIBUTING.md).

To all contributors, we thank you!  Without your contribution, this project would not be what it is today.
## License
The newrelic-k8s-metrics-adapter is licensed under the [Apache 2.0](http://apache.org/licenses/LICENSE-2.0.txt) License.

> The newrelic-k8s-metrics-adapter also uses source code from third-party libraries. 
> You can find full details on which libraries are used, and the terms under which they are licensed in the third-party 
> notices document.
