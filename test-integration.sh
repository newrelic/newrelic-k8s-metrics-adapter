#!/usr/bin/env bash

# Test cluster
KUBECONFIG=kubeconfig

# New Relic account (production) details
ACCOUNT_ID=""
API_KEY=""

# Unset if you only want to setup a test cluster with integration specifications
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

    if [[ totalArgs -lt 4 ]]; then
        help
        exit 1
    fi
}

function help() {
    cat <<END
 Usage:
 ${0##*/}    --account_id <new_relic_prod> --api_key <api_key> [--run_tests]

 --account_id:   New Relic account in production
 --api_key:      key type 'USER'
 --run_tests:    if unset, create a cluster with specifications matching integration tests
                 otherwise run tests in addition to setting up cluster
END
}

function create_cluster() {
    echo "🔄 Setup"
    cleanup
    touch ${KUBECONFIG} > /dev/null

    echo "🔄 Creating cluster"
    make kind-up > /dev/null 2>&1
}

function run_tests() {
    echo "🔄 Starting integration tests"
    export NEWRELIC_ACCOUNT_ID=${ACCOUNT_ID}
    export NEWRELIC_API_KEY=${API_KEY}
    make test-integration
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
    rm -f ${KUBECONFIG} > /dev/null
}

main "$@"
