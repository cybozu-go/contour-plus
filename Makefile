CONTROLLER_TOOLS_VERSION = 0.10.0
KUSTOMIZE_VERSION = 4.5.7
CERT_MANAGER_VERSION := 1.7.2
EXTERNAL_DNS_VERSION := 0.11.0
CONTOUR_VERSION := 1.22.1
ENVTEST_K8S_VERSION = 1.24.2

PROJECT_DIR := $(CURDIR)
BIN_DIR := $(PROJECT_DIR)/bin
CRD_DIR := $(PROJECT_DIR)/config/crd/third

KUSTOMIZE := $(BIN_DIR)/kustomize
CONTROLLER_GEN := $(BIN_DIR)/controller-gen
SETUP_ENVTEST := $(BIN_DIR)/setup-envtest
STATICCHECK := $(BIN_DIR)/staticcheck
CUSTOMCHECKER := $(BIN_DIR)/custom-checker

# Image URL to use all building/pushing image targets
IMG ?= quay.io/cybozu/contour-plus:latest

# Set the shell used to bash for better error handling.
SHELL = /bin/bash
.SHELLFLAGS = -e -o pipefail -c

.PHONY: all
all: help

##@ Basic
.PHONY: help
help: ## Display this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

.PHONY: setup
setup: download-tools download-crds ## Setup

.PHONY: download-tools
download-tools:
	GOBIN=$(BIN_DIR) go install sigs.k8s.io/controller-tools/cmd/controller-gen@v$(CONTROLLER_TOOLS_VERSION)
	GOBIN=$(BIN_DIR) go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest
	GOBIN=$(BIN_DIR) go install sigs.k8s.io/kustomize/kustomize/v4@v$(KUSTOMIZE_VERSION)
	GOBIN=$(BIN_DIR) go install github.com/cybozu/neco-containers/golang/analyzer/cmd/custom-checker@latest
	GOBIN=$(BIN_DIR) go install honnef.co/go/tools/cmd/staticcheck@latest

.PHONY: download-crds
download-crds:
	curl -fsL -o $(CRD_DIR)/certmanager.yml -sLf https://github.com/jetstack/cert-manager/releases/download/v$(CERT_MANAGER_VERSION)/cert-manager.crds.yaml
	curl -fsL -o $(CRD_DIR)/dnsendpoint.yml -sLf https://github.com/kubernetes-sigs/external-dns/raw/v$(EXTERNAL_DNS_VERSION)/docs/contributing/crd-source/crd-manifest.yaml
	curl -fsL -o $(CRD_DIR)/httpproxy.yml -sLf https://github.com/projectcontour/contour/raw/v$(CONTOUR_VERSION)/examples/contour/01-crds.yaml

.PHONY: clean
clean: ## Clean files
	rm -rf $(BIN_DIR)/* $(CRD_DIR)/*

##@ Build

.PHONY: manifests
manifests: ## Generate manifests e.g. CRD, RBAC etc.
	$(CONTROLLER_GEN) rbac:roleName=contour-plus webhook paths="./..."

.PHONY: generate
generate: ## Generate code
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

.PHONY: build
build: ## Build manager binary
	CGO_ENABLED=0 go build -o bin/contour-plus -ldflags="-w -s" main.go

.PHONY: docker-build
docker-build: build ## Build the docker image
	docker build . -t ${IMG}

##@ Test

.PHONY: check-generate
check-generate: ## Check for commit omissions of auto-generated files
	$(MAKE) manifests
	$(MAKE) generate
	go mod tidy
	git diff --exit-code --name-only

.PHONY: lint
lint: ## Run lint tools
	test -z "$$(gofmt -s -l . | tee /dev/stderr)"
	$(STATICCHECK) ./...
	test -z "$$($(CUSTOMCHECKER) -restrictpkg.packages=html/template,log $$(go list -tags='$(GOTAGS)' ./... ) 2>&1 | tee /dev/stderr)"
	go vet ./...

.PHONY: test
test: ## Run unit tests
	source <($(SETUP_ENVTEST) use -p env $(ENVTEST_K8S_VERSION)) && \
		go test -race -v -count 1 ./...
