# Use ?= for variable assignment so they can be overwritten with environment variables.
GO_PACKAGES ?= ./...
GO_TESTS ?= ^.*$
GO_CMD ?= go
GO_TEST ?= $(GO_CMD) test -covermode=atomic -run $(GO_TESTS)

GOOS ?=
GOARCH ?=
CGO_ENABLED ?= 0

BINARY_NAME ?= newrelic-k8s-metrics-adapter

TAG ?= "dev"
X_LD_FLAGS ?= -X 'github.com/gsanchezgavier/metrics-adapter/internal/adapter.version=$(TAG)'
EXT_LD_FLAGS ?= -extldflags '-static'
LD_FLAGS ?= "$(EXT_LD_FLAGS) $(X_LD_FLAGS)"

ifeq (, $(shell which golangci-lint))
GOLANGCI_LINT ?= go run -modfile=tools/go.mod github.com/golangci/golangci-lint/cmd/golangci-lint
else
GOLANGCI_LINT ?= golangci-lint
endif
GOLANGCI_LINT_CONFIG_FILE ?= .golangci.yml

DOCKER_CMD ?= docker
IMAGE_REPO ?= newrelic/newrelic-k8s-metrics-adapter

TILT_CMD ?= tilt
TEST_KUBECONFIG ?= $(shell realpath kubeconfig)

KIND_CMD ?= kind
KIND_SCRIPT ?= hack/kind-with-registry.sh
KIND_IMAGE ?= kindest/node:v1.22.1

.PHONY: build
build: BINARY_NAME := $(if $(GOOS),$(BINARY_NAME)-$(GOOS),$(BINARY_NAME))
build: BINARY_NAME := $(if $(GOARCH),$(BINARY_NAME)-$(GOARCH),$(BINARY_NAME))
build: ## Compiles metrics-adapter binary.
	CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) GOARCH=$(GOARCH) $(GO_CMD) build -o $(BINARY_NAME) -v -buildmode=exe -ldflags $(LD_FLAGS) .

.PHONY: build-test
build-test: ## Compiles unit tests.
	$(GO_TEST) -run=nonexistent -tags integration,e2e $(GO_PACKAGES)

.PHONY: test
test: ## Runs all unit tests.
	$(GO_TEST) $(GO_PACKAGES)

.PHONY: test-integration
test-integration: ## Runs all integration tests.
	KUBECONFIG=$(TEST_KUBECONFIG) USE_EXISTING_CLUSTER=true $(GO_TEST) -tags integration $(GO_PACKAGES)

.PHONY: test-e2e
test-e2e: ## Runs all e2e tests. Expects metrics-adapter to be installed on the cluster using Helm chart.
	KUBECONFIG=$(TEST_KUBECONFIG) $(GO_TEST) -tags e2e $(GO_PACKAGES)

.PHONY: ci
ci: check-tidy check-generate build test ## Runs checks performed by CI without external dependencies required (e.g. golangci-lint).

.PHONY: check-working-tree-clean
check-working-tree-clean: ## Checks if working directory is clean.
	@test -z "$$(git status --porcelain)" || (echo "Commit all changes before running this target"; exit 1)

.PHONY: check-tidy
check-tidy: check-working-tree-clean ## Checks if Go module files are clean.
	go mod tidy
	@test -z "$$(git status --porcelain)" || (echo "Please run 'go mod tidy' and commit generated changes."; git status; exit 1)

.PHONY: check-update-linters
check-update-linters: check-working-tree-clean update-linters ## Checks if list of enabled golangci-lint linters is up to date.
	@test -z "$$(git status --porcelain)" || (echo "Linter configuration outdated. Run 'make update-linters' and commit generated changes to fix."; git diff; exit 1)

.PHONY: check-generate
check-generate: check-working-tree-clean generate ## Checks if all generated files are up to date.
	@test -z "$$(git status --porcelain)" || (echo "Generated files are outdated. Run 'make generate' and commit generated changes to fix."; git diff; exit 1)

.PHONY: update-linters
update-linters: ## Updates list of enabled golangci-lint linters.
	# Remove all enabled linters.
	sed -i '/^  enable:/q0' $(GOLANGCI_LINT_CONFIG_FILE)
	# Then add all possible linters to config.
	$(GOLANGCI_LINT) linters | grep -E '^\S+:' | cut -d: -f1 | sort | sed 's/^/    - /g' | grep -v -E "($$(sed -e '1,/^  disable:$$/d' .golangci.yml  | grep -E '    - \S+$$' | awk '{print $$2}' | tr \\n '|' | sed 's/|$$//g'))" >> $(GOLANGCI_LINT_CONFIG_FILE)

.PHONY: lint
lint: build build-test ## Runs golangci-lint.
	$(GOLANGCI_LINT) run $(GO_PACKAGES)

.PHONY: codespell
codespell: CODESPELL_BIN := codespell
codespell: ## Runs spell checking.
	which $(CODESPELL_BIN) >/dev/null 2>&1 || (echo "$(CODESPELL_BIN) binary not found, skipping spell checking"; exit 0)
	$(CODESPELL_BIN)

.PHONY: image
## GOOS and GOARCH are manually set so the output BINARY_NAME includes them as suffixes.
## Additionally, DOCKER_BUILDKIT is set since it's needed for Docker to populate TARGETOS and TARGETARCH ARGs.
## Here we call $(MAKE) build instead of using a dependency because the latter would, for some reason, prevent
## the BINARY_NAME conditional from working.
image: GOOS := $(if $(GOOS),$(GOOS),linux)
image: GOARCH := $(if $(GOARCH),$(GOARCH),$(shell go env GOARCH))
image: ## Builds metrics-adapter Docker image.
	@if [ "$(GOOS)" != "linux" ]; then echo "'make image' must be called with GOOS=linux (or empty), found '$(GOOS)'"; exit 1; fi
	$(MAKE) build GOOS=$(GOOS) GOARCH=$(GOARCH)
	DOCKER_BUILDKIT=1 $(DOCKER_CMD) build --rm=true -t $(IMAGE_REPO) .

.PHONY: image-push
image-push: image ## Builds and pushes metrics-adapter Docker image.
	$(DOCKER_CMD) push $(IMAGE_REPO)

.PHONY: kind-up
kind-up: ## Creates local Kind cluster for development.
	env KUBECONFIG=$(TEST_KUBECONFIG) $(KIND_SCRIPT)

.PHONY: update-kind
update-kind: ## Updates hack/kind-with-registry.sh file.
	wget https://kind.sigs.k8s.io/examples/kind-with-registry.sh -O $(KIND_SCRIPT)
	sed -i 's|kind create cluster|kind create cluster --image=$(KIND_IMAGE)|g' $(KIND_SCRIPT)
	chmod +x $(KIND_SCRIPT)

.PHONY: kind-down
kind-down: ## Cleans up local Kind cluster.
	$(KIND_CMD) delete cluster

.PHONY: tilt-up
tilt-up: ## Builds project and deploys it to local Kind cluster.
	env KUBECONFIG=$(TEST_KUBECONFIG) $(TILT_CMD) up

.PHONY: tilt-down
tilt-down: ## Cleans up resources created by Tilt.
	env KUBECONFIG=$(TEST_KUBECONFIG) $(TILT_CMD) down

.PHONY: help
help: ## Prints help message.
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

.PHONY: buildLicenseNotice
buildLicenseNotice:
	@go list -mod=mod -m -json all | go-licence-detector -noticeOut=NOTICE.txt -rules ./assets/licence/rules.json  -noticeTemplate ./assets/licence/THIRD_PARTY_NOTICES.md.tmpl -noticeOut THIRD_PARTY_NOTICES.md -overrides ./assets/licence/overrides -includeIndirect

.PHONY: generate
generate: ## Runs code generation from //go:generate statements
	$(GO_CMD) generate -tags codegen ./...
