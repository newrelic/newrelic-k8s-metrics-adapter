#!/usr/bin/env bash

# Test cluster
CLUSTER_NAME=""
KUBECONFIG=kubeconfig

# Values file to pass to Helm
VALUES_FILE=values.yaml

# New Relic account (production) details
ACCOUNT_ID=""
API_KEY=""
LICENSE_KEY=""

# Unset if you only want to setup a test cluster with E2E specifications
# Set to true if you additionally want to run tests
RUN_TESTS=""

function main() {
    parse_args "$@"
    create_cluster
    if [[ "$RUN_TESTS" == "true" ]]; then
        run_tests
        teardown
    fi
}

function parse_args() {
    totalArgs=$#

    # Arguments are passed by value, so other functions
    # are not affected by destructive processing
    while [[ $# -gt 0 ]]; do
        case $1 in
            --account_id)
            shift
            ACCOUNT_ID="$1"
            ;;
            --api_key)
            shift
            API_KEY="$1"
            ;;
            --help)
            help
            exit 0
            ;;
            --license_key)
            shift
            LICENSE_KEY="$1"
            ;;
            --run_tests)
            RUN_TESTS="true"
            ;;
            -*|--*|*)
            echo "Unknown field: $1"
            exit 1
            ;;
        esac
        shift
    done

    if [[ totalArgs -lt 6 ]]; then
        help
    fi
}

function help() {
    cat <<END
 Usage:
 ${0##*/}    --account_id <new_relic_prod> --api_key <api_key>
             --license_key <license_key> [--run_tests]

 --account_id:   New Relic account in production
 --api_key:      key type 'USER'
 --license_key:  key type 'INGEST - LICENSE'
 --run_tests:    if unset, create a cluster with specifications matching E2E tests
                 otherwise run tests in addition to setting up cluster
END
}

function create_cluster() {
    echo "🔄 Setup"
    cleanup
    now=$( date "+%Y-%m-%d-%H-%M-%S" )
    CLUSTER_NAME=${now}-e2e-tests
    touch ${KUBECONFIG} > /dev/null
    chmod go-r ${KUBECONFIG}
    export KUBECONFIG=${KUBECONFIG}

    echo "🔄 Creating cluster"
    make kind-up > /dev/null 2>&1

    echo "🔄 Adding Helm repositories"
    helm repo add newrelic-k8s-metrics-adapter https://newrelic.github.io/newrelic-k8s-metrics-adapter > /dev/null
    helm repo update > /dev/null

    echo "🔄 Installing newrelic-k8s-metrics-adapter"
    cat << EOF > ${VALUES_FILE}
global:
  licenseKey: ${LICENSE_KEY}
  cluster: ${CLUSTER_NAME}
  nrStaging: false
personalAPIKey: ${API_KEY}
config:
  accountID: ${ACCOUNT_ID}
  externalMetrics:
    e2e:
      query: "SELECT latest(attributeName) FROM (SELECT 0.123 AS 'attributeName' FROM NrUsage)"
      removeClusterFilter: true
EOF

    helm install --create-namespace \
        --namespace newrelic-k8s-metrics-adapter \
        newrelic-k8s-metrics-adapter/newrelic-k8s-metrics-adapter \
        --generate-name \
        --values ${VALUES_FILE} > /dev/null
}

function run_tests() {
    echo "🔄 Waiting for metrics adapter to settle"
    sleep 30

    echo "🔄 Starting E2E tests"
    export NEWRELIC_ACCOUNT_ID=${ACCOUNT_ID}
    export NEWRELIC_API_KEY=${API_KEY}
    export NEWRELIC_CLUSTER_NAME=${CLUSTER_NAME}
    export NEWRELIC_REGION=US
    make test-e2e
}

function teardown() {
    echo "🔄 Teardown"
    cleanup
}

function cleanup() {
    go clean -testcache > /dev/null
    make kind-down > /dev/null 2>&1
    docker stop kind-registry > /dev/null 2>&1
    docker rm kind-registry > /dev/null 2>&1
    rm -f ${KUBECONFIG} ${VALUES_FILE} > /dev/null
}

main "$@"
