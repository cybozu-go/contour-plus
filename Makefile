CONTROLLER_RUNTIME_VERSION := $(shell awk '/sigs\.k8s\.io\/controller-runtime/ {print substr($$2, 2)}' go.mod)
CONTROLLER_TOOLS_VERSION = 0.5.0
KUSTOMIZE_VERSION = 3.8.10
CERT_MANAGER_VERSION := 1.3.1
EXTERNAL_DNS_VERSION := 0.7.6
CONTOUR_VERSION := 1.17.1

# Image URL to use all building/pushing image targets
IMG ?= quay.io/cybozu/contour-plus:latest

# Set the shell used to bash for better error handling.
SHELL = /bin/bash
.SHELLFLAGS = -e -o pipefail -c

.PHONY: all
all: build

.PHONY: crds
crds:
	mkdir -p config/crd/third
	curl -fsL -o config/crd/third/certmanager.yml -sLf https://github.com/jetstack/cert-manager/releases/download/v$(CERT_MANAGER_VERSION)/cert-manager.crds.yaml
	curl -fsL -o config/crd/third/dnsendpoint.yml -sLf https://github.com/kubernetes-sigs/external-dns/raw/v$(EXTERNAL_DNS_VERSION)/docs/contributing/crd-source/crd-manifest.yaml
	curl -fsL -o config/crd/third/httpproxy.yml -sLf https://github.com/projectcontour/contour/raw/v$(CONTOUR_VERSION)/examples/contour/01-crds.yaml

# Run tests, and set up envtest if not done already.
ENVTEST_ASSETS_DIR := testbin
ENVTEST_SCRIPT_URL := https://raw.githubusercontent.com/kubernetes-sigs/controller-runtime/v$(CONTROLLER_RUNTIME_VERSION)/hack/setup-envtest.sh
.PHONY: test
test: setup-envtest crds simple-test
	source <($(SETUP_ENVTEST) use -p env); \
		go test -race -v -count 1 ./...

# Generate manifests e.g. CRD, RBAC etc.
.PHONY: manifests
manifests: controller-gen
	$(CONTROLLER_GEN) rbac:roleName=contour-plus webhook paths="./..."

# Generate code
.PHONY: generate
generate: controller-gen
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

.PHONY: simple-test
simple-test: test-tools
	test -z "$$(gofmt -s -l . | tee /dev/stderr)"
	staticcheck ./...
	test -z "$$(custom-checker -restrictpkg.packages=html/template,log $$(go list -tags='$(GOTAGS)' ./... ) 2>&1 | tee /dev/stderr)"
	test -z "$$(nilerr $$(go list ./...) 2>&1 | tee /dev/stderr)"
	go vet ./...

.PHONY: check-generate
check-generate:
	$(MAKE) manifests
	$(MAKE) generate
	git diff --exit-code --name-only

# Build manager binary
.PHONY: build
build:
	CGO_ENABLED=0 go build -o bin/contour-plus -ldflags="-w -s" main.go

# Build the docker image
.PHONY: docker-build
docker-build: build
	docker build . -t ${IMG}

# Push the docker image
.PHONY: docker-push
docker-push:
	docker push ${IMG}

# Download controller-gen locally if necessary
CONTROLLER_GEN := $(PWD)/bin/controller-gen
.PHONY: controller-gen
controller-gen:
	$(call go-get-tool,$(CONTROLLER_GEN),sigs.k8s.io/controller-tools/cmd/controller-gen@v$(CONTROLLER_TOOLS_VERSION))

SETUP_ENVTEST := $(shell pwd)/bin/setup-envtest
.PHONY: setup-envtest
setup-envtest: $(SETUP_ENVTEST) ## Download setup-envtest locally if necessary
$(SETUP_ENVTEST):
	# see https://github.com/kubernetes-sigs/controller-runtime/tree/master/tools/setup-envtest
	GOBIN=$(shell pwd)/bin go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest

# Download kustomize locally if necessary
KUSTOMIZE := $(PWD)/bin/kustomize
.PHONY: kustomize
kustomize:
	$(call go-get-tool,$(KUSTOMIZE),sigs.k8s.io/kustomize/kustomize/v3@v$(KUSTOMIZE_VERSION))

.PHONY: mod
mod:
	go mod tidy
	git add go.mod go.sum

.PHONY: test-tools
test-tools: custom-checker staticcheck nilerr

.PHONY: custom-checker
custom-checker:
	if ! which custom-checker >/dev/null; then \
		env GOFLAGS= go install github.com/cybozu/neco-containers/golang/analyzer/cmd/custom-checker@latest; \
	fi

.PHONY: staticcheck
staticcheck:
	if ! which staticcheck >/dev/null; then \
		env GOFLAGS= go install honnef.co/go/tools/cmd/staticcheck@latest; \
	fi

.PHONY: nilerr
nilerr:
	if ! which nilerr >/dev/null; then \
		env GOFLAGS= go install github.com/gostaticanalysis/nilerr/cmd/nilerr@latest; \
	fi

clean:
	rm -rf bin testbin
	rm -f config/crd/third/*

# go-get-tool will 'go get' any package $2 and install it to $1.
PROJECT_DIR := $(shell dirname $(abspath $(lastword $(MAKEFILE_LIST))))
define go-get-tool
@[ -f $(1) ] || { \
set -e ;\
TMP_DIR=$$(mktemp -d) ;\
cd $$TMP_DIR ;\
go mod init tmp ;\
echo "Downloading $(2)" ;\
GOBIN=$(PROJECT_DIR)/bin go get $(2) ;\
rm -rf $$TMP_DIR ;\
}
endef
