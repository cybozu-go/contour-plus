include Makefile.versions

CONTROLLER_TOOLS_VERSION = 0.13.0

PROJECT_DIR := $(CURDIR)
BIN_DIR := $(PROJECT_DIR)/bin
CRD_DIR := $(PROJECT_DIR)/config/crd/third
WORKFLOWS_DIR := $(PROJECT_DIR)/.github/workflows

KUSTOMIZE := $(BIN_DIR)/kustomize
CONTROLLER_GEN := $(BIN_DIR)/controller-gen
SETUP_ENVTEST := $(BIN_DIR)/setup-envtest
STATICCHECK := $(BIN_DIR)/staticcheck
CUSTOMCHECKER := $(BIN_DIR)/custom-checker
GH := $(BIN_DIR)/gh
YQ := $(BIN_DIR)/yq

# Image URL to use all building/pushing image targets
IMG ?= ghcr.io/cybozu-go/contour-plus:latest

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
download-tools: $(GH) $(YQ)
	GOBIN=$(BIN_DIR) go install sigs.k8s.io/controller-tools/cmd/controller-gen@v$(CONTROLLER_TOOLS_VERSION)
	GOBIN=$(BIN_DIR) go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest
	GOBIN=$(BIN_DIR) go install sigs.k8s.io/kustomize/kustomize/v5@v$(KUSTOMIZE_VERSION)
	GOBIN=$(BIN_DIR) go install github.com/cybozu-go/golang-custom-analyzer/cmd/custom-checker@latest
	GOBIN=$(BIN_DIR) go install honnef.co/go/tools/cmd/staticcheck@latest

.PHONY: download-crds
download-crds:
	curl -fsL -o $(CRD_DIR)/certmanager.yml -sLf https://github.com/jetstack/cert-manager/releases/download/v$(CERT_MANAGER_VERSION)/cert-manager.crds.yaml
	curl -fsL -o $(CRD_DIR)/dnsendpoint.yml -sLf https://github.com/kubernetes-sigs/external-dns/raw/v$(EXTERNAL_DNS_VERSION)/docs/contributing/crd-source/crd-manifest.yaml
	curl -fsL -o $(CRD_DIR)/httpproxy.yml -sLf https://github.com/projectcontour/contour/raw/v$(CONTOUR_VERSION)/examples/contour/01-crds.yaml

$(GH):
	mkdir -p $(BIN_DIR)
	wget -qO - https://github.com/cli/cli/releases/download/v$(GH_VERSION)/gh_$(GH_VERSION)_linux_amd64.tar.gz | tar -zx -O gh_$(GH_VERSION)_linux_amd64/bin/gh > $@
	chmod +x $@

$(YQ):
	mkdir -p $(BIN_DIR)
	wget -qO $@ https://github.com/mikefarah/yq/releases/download/v$(YQ_VERSION)/yq_linux_amd64
	chmod +x $@

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

##@ Maintenance
.PHONY: login-gh
login-gh: ## Login to GitHub
	if ! $(GH) auth status 2>/dev/null; then \
		echo; \
		echo '!! You need login to GitHub to proceed. Please follow the next command with "Authenticate Git with your GitHub credentials? (Y)".'; \
		echo; \
		$(GH) auth login -h github.com -p HTTPS -w; \
	fi

.PHONY: logout-gh
logout-gh: ## Logout from GitHub
	$(GH) auth logout

.PHONY: update-contour
update-contour: ## Update Contour and Kubernetes in go.mod
	$(call get-latest-gh-package-tag,contour)
	go get github.com/projectcontour/contour@$(call upstream-tag,$(latest_tag))
	K8S_MINOR_VERSION="0."$$(go list -m -f '{{.Version}}' k8s.io/api | cut -d'.' -f2); \
	K8S_PACKAGE_VERSION="$$(go list -m -versions k8s.io/api | tr ' ' '\n' | grep $${K8S_MINOR_VERSION} | sort -V | tail -n 1)"; \
	go get k8s.io/api@$${K8S_PACKAGE_VERSION}; \
	go get k8s.io/apimachinery@$${K8S_PACKAGE_VERSION}; \
	go get k8s.io/client-go@$${K8S_PACKAGE_VERSION}; \
	go mod tidy

.PHONY: version
version: login-gh ## Update dependent versions
	$(call update-version,actions/checkout,ACTIONS_CHECKOUT_VERSION,1)
	$(call update-version,actions/create-release,ACTIONS_CREATE_RELEASE_VERSION,1)
	$(call update-version,actions/setup-go,ACTIONS_SETUP_GO_VERSION,1)
	$(call update-version-ghcr,cert-manager,CERT_MANAGER_VERSION)
	$(call update-version-ghcr,contour,CONTOUR_VERSION)
	$(call update-version-ghcr,external-dns,EXTERNAL_DNS_VERSION)

	$(call get-latest-gh-package-tag,argocd)
	NEW_VERSION=$$(docker run ghcr.io/cybozu/argocd:$(latest_tag) kustomize version | cut -c2-); \
	sed -i -e "s/KUSTOMIZE_VERSION := .*/KUSTOMIZE_VERSION := $${NEW_VERSION}/g" Makefile.versions

	K8S_MINOR_VERSION="1."$$(go list -m -f '{{.Version}}' k8s.io/api | cut -d'.' -f2); \
	NEW_VERSION=$$($(SETUP_ENVTEST) list | tr -s ' ' | cut -d' ' -f2 | fgrep $${K8S_MINOR_VERSION} | sort -V | tail -n 1 | cut -c2-); \
	sed -i -e "s/ENVTEST_K8S_VERSION := .*/ENVTEST_K8S_VERSION := $${NEW_VERSION}/g" Makefile.versions

.PHONY: update-actions
update-actions:
	$(call update-trusted-action,actions/checkout,$(ACTIONS_CHECKOUT_VERSION))
	$(call update-trusted-action,actions/create-release,$(ACTIONS_CREATE_RELEASE_VERSION))
	$(call update-trusted-action,actions/setup-go,$(ACTIONS_SETUP_GO_VERSION))

.PHONY: maintenance
maintenance: ## Update dependent manifests
	$(MAKE) update-actions
	$(MAKE) download-crds

.PHONY: list-actions
list-actions: ## List used GitHub Actions
	@{ for i in $(shell ls $(WORKFLOWS_DIR)); do \
		$(YQ) '.. | select(has("uses")).uses' $(WORKFLOWS_DIR)/$$i; \
	done } | sort | uniq

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

# usage get-latest-gh OWNER/REPO
define get-latest-gh
	$(eval latest_gh := $(shell $(GH) release list --repo $1 | grep Latest | cut -f3))
endef

# usage: get-latest-gh-package-tag NAME
define get-latest-gh-package-tag
$(eval latest_tag := $(shell curl -sSf -H "Authorization: Bearer $(shell curl -sSf "https://ghcr.io/token?scope=repository%3Acybozu%2F$1%3Apull&service=ghcr.io" | jq -r .token)" https://ghcr.io/v2/cybozu/$1/tags/list | jq -r '.tags[]' | sort -Vr | head -n 1))
endef

# usage: upstream-tag 1.2.3.4
# do not indent because it appears on output
define upstream-tag
$(shell echo $1 | sed -E 's/^(.*)\.[[:digit:]]+$$/v\1/')
endef

# usage update-version OWNER/REPO VAR MAJOR
define update-version
	$(call get-latest-gh,$1)
	NEW_VERSION=$$(echo $(latest_gh) | if [ -z "$3" ]; then cut -b 2-; else cut -b 2; fi); \
	sed -i -e "s/$2 := .*/$2 := $${NEW_VERSION}/g" Makefile.versions
endef

# usage update-version-ghcr NAME VAR
define update-version-ghcr
	$(call get-latest-gh-package-tag,$1)
	NEW_VERSION=$$(echo $(call upstream-tag,$(latest_tag)) | cut -b 2-); \
	sed -i -e "s/$2 := .*/$2 := $${NEW_VERSION}/g" Makefile.versions
endef

# usage update-trusted-action OWNER/REPO VERSION
define update-trusted-action
	for i in $(shell ls $(WORKFLOWS_DIR)); do \
		$(YQ) -i '(.. | select(has("uses")) | select(.uses | contains("$1"))).uses = "$1@v$2"' $(WORKFLOWS_DIR)/$$i; \
	done
endef
