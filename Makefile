PROJECT_DIR := $(dir $(abspath $(lastword $(MAKEFILE_LIST))))
include $(PROJECT_DIR)/Makefile.versions

BIN_DIR := $(PROJECT_DIR)/bin
CRD_DIR := $(PROJECT_DIR)/config/crd/third
WORKFLOWS_DIR := $(PROJECT_DIR)/.github/workflows

KUSTOMIZE := $(shell aqua which kustomize)
CONTROLLER_GEN := $(shell aqua which controller-gen)
SETUP_ENVTEST := $(shell aqua which setup-envtest)
STATICCHECK := $(shell aqua which staticcheck)
CUSTOMCHECKER := $(BIN_DIR)/custom-checker
GOIMPORTS := $(shell aqua which goimports)
KIND := $(shell aqua which kind)
GH := $(shell aqua which gh)
YQ := $(shell aqua which yq)
KUBECTL := $(shell aqua which kubectl)
HELM := $(shell aqua which helm)

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
download-tools:
	aqua install
	GOBIN=$(BIN_DIR) go install github.com/cybozu-go/golang-custom-analyzer/cmd/custom-checker@v$(CUSTOM_CHECKER_VERSION)

.PHONY: download-crds
download-crds:
	curl -fsL -o $(CRD_DIR)/certmanager.yml -sLf https://github.com/jetstack/cert-manager/releases/download/$(call upstream-tag,$(CERT_MANAGER_VERSION))/cert-manager.crds.yaml
	echo "$(CERTMANAGER_CRD_SHA256)  $(CRD_DIR)/certmanager.yml" | sha256sum --check
	curl -fsL -o $(CRD_DIR)/dnsendpoint.yml -sLf https://github.com/kubernetes-sigs/external-dns/raw/$(EXTERNAL_DNS_COMMIT)/config/crd/standard/dnsendpoints.externaldns.k8s.io.yaml
	curl -fsL -o $(CRD_DIR)/httpproxy.yml -sLf https://github.com/projectcontour/contour/raw/$(CONTOUR_COMMIT)/examples/contour/01-crds.yaml

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
update-contour: ## Update Contour and Kubernetes in go.mod to the pinned CONTOUR_VERSION
	go get github.com/projectcontour/contour@$(call upstream-tag,$(CONTOUR_VERSION))
	K8S_MINOR_VERSION="0."$$(go list -m -f '{{.Version}}' k8s.io/api | cut -d'.' -f2); \
	K8S_PACKAGE_VERSION="$$(go list -m -versions k8s.io/api | tr ' ' '\n' | grep $${K8S_MINOR_VERSION} | sort -V | tail -n 1)"; \
	go get k8s.io/api@$${K8S_PACKAGE_VERSION}; \
	go get k8s.io/apimachinery@$${K8S_PACKAGE_VERSION}; \
	go get k8s.io/client-go@$${K8S_PACKAGE_VERSION}; \
	go mod tidy

.PHONY: update-cert-manager
update-cert-manager: ## Update cert-manager in go.mod to the pinned CERT_MANAGER_VERSION
	go get github.com/cert-manager/cert-manager@$(call upstream-tag,$(CERT_MANAGER_VERSION))
	go mod tidy

.PHONY: update-hashes
update-hashes: ## Update commit hashes and file checksums for the pinned dependency versions
	$(call update-commit,kubernetes-sigs/external-dns,$(call upstream-tag,$(EXTERNAL_DNS_VERSION)),EXTERNAL_DNS_COMMIT)
	$(call update-commit,projectcontour/contour,$(call upstream-tag,$(CONTOUR_VERSION)),CONTOUR_COMMIT)
	$(call update-sha256,https://github.com/jetstack/cert-manager/releases/download/$(call upstream-tag,$(CERT_MANAGER_VERSION))/cert-manager.crds.yaml,CERTMANAGER_CRD_SHA256)
	$(call update-sha256,https://github.com/jetstack/cert-manager/releases/download/$(call upstream-tag,$(CERT_MANAGER_VERSION))/cert-manager.yaml,CERTMANAGER_MANIFEST_SHA256)

.PHONY: version
version: login-gh ## Update dependent versions
	$(call update-version,actions/checkout,ACTIONS_CHECKOUT_VERSION,1)
	$(call update-version,actions/create-release,ACTIONS_CREATE_RELEASE_VERSION,1)
	$(call update-version,actions/setup-go,ACTIONS_SETUP_GO_VERSION,1)
	$(call update-version,cybozu-go/golang-custom-analyzer,CUSTOM_CHECKER_VERSION)
	$(call update-version-ghcr,cert-manager,CERT_MANAGER_VERSION)
	$(call update-version-ghcr,contour,CONTOUR_VERSION)
	$(call update-version-ghcr,external-dns,EXTERNAL_DNS_VERSION)
	$(call update-version-ghcr,envoy,ENVOY_VERSION)
	$(call update-version-ghcr,etcd,ETCD_VERSION)
	$(call update-version-ghcr,coredns,COREDNS_VERSION)

	$(call get-latest-gh-release,aquaproj/aqua-registry)
	$(YQ) -i '.registries[0].ref = "$(latest_gh)"' aqua.yaml
	$(call update-aqua-package,cli/cli)
	$(call update-aqua-package,helm/helm)
	$(call update-aqua-package,kubernetes-sigs/kind)
	$(call update-aqua-package,mikefarah/yq)
	$(call update-aqua-package,kubernetes-sigs/controller-tools/controller-gen)
	$(call update-aqua-package,kubernetes-sigs/controller-runtime/setup-envtest)
	$(call update-aqua-package,dominikh/go-tools/staticcheck)
	$(call update-aqua-package,golang/tools/goimports)

	# kustomize follows the version bundled in the Argo CD image, since that's
	# what renders our manifests for deployment
	# https://github.com/cybozu/neco-containers/blob/main/argocd/Dockerfile
	$(call get-latest-gh-package-tag,argocd)
	KUSTOMIZE_VERSION=$$(docker run ghcr.io/cybozu/argocd:$(latest_tag) kustomize version | cut -c2-); \
	$(YQ) -i '(.packages[] | select(.name | test("^kubernetes-sigs/kustomize@"))).name = "kubernetes-sigs/kustomize@kustomize/v'"$$KUSTOMIZE_VERSION"'"' aqua.yaml

	K8S_MINOR_VERSION="1."$$(go list -m -f '{{.Version}}' k8s.io/api | cut -d'.' -f2); \
	NEW_VERSION=$$($(SETUP_ENVTEST) list | tr -s ' ' | cut -d' ' -f2 | fgrep $${K8S_MINOR_VERSION} | sort -V | tail -n 1 | cut -c2-); \
	sed -i -e "s/ENVTEST_K8S_VERSION := .*/ENVTEST_K8S_VERSION := $${NEW_VERSION}/g" Makefile.versions; \
	$(YQ) -i '(.packages[] | select(.name | test("^kubernetes/kubectl@"))).name = "kubernetes/kubectl@v'"$$NEW_VERSION"'"' aqua.yaml

	# update kindest node version
	K8S_MINOR_VERSION="1."$$(go list -m -f '{{.Version}}' k8s.io/api | cut -d'.' -f2); \
	NEW_VERSION=$$( \
	  curl -fsSL "https://hub.docker.com/v2/repositories/kindest/node/tags?page_size=50&name=v$${K8S_MINOR_VERSION}." \
	  | jq -r '.results[].name' \
	  | grep -E "^v$${K8S_MINOR_VERSION}\.[0-9]+$$" \
	  | sort -V \
	  | tail -n 1 \
	); \
	echo "Updating kindest/node to $$NEW_KINDEST_TAG"; \
	sed -i -e "s/KINDEST_NODE_VERSION := .*/KINDEST_NODE_VERSION := $${NEW_VERSION#v}/g" Makefile.versions

	aqua update-checksum --prune
	aqua install


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
	$(GOIMPORTS) -w -local github.com/cybozu-go/contour-plus .
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
		go test -race -v -count 1 ./controllers/...

# usage get-latest-gh OWNER/REPO
define get-latest-gh
	$(eval latest_gh := $(shell $(GH) release list --repo $1 | grep Latest | cut -f3))
endef

# usage: get-latest-gh-release OWNER/REPO VARNAMESUFFIXopt
# get the latest release from github.com/OWNER/REPO
define get-latest-gh-release
$(eval latest_gh$2 := $(shell curl -sSf https://api.github.com/repos/$1/releases/latest | jq -r '.tag_name'))
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
	sed -i -e "s/$2 := .*/$2 := $(latest_tag)/g" Makefile.versions
endef

# usage: update-commit OWNER/REPO TAG VAR
# resolve TAG to its commit SHA (the GitHub commits API dereferences annotated tags too) and record it in Makefile.versions
define update-commit
	COMMIT_SHA=$$(curl -sSf https://api.github.com/repos/$1/commits/$2 | jq -r '.sha'); \
	sed -i -e "s/$3 := .*/$3 := $${COMMIT_SHA}/g" Makefile.versions
endef

# usage: update-sha256 URL VAR
# compute the sha256 checksum of a downloaded file and record it in Makefile.versions
define update-sha256
	NEW_SHA256=$$(curl -sSLf $1 | sha256sum | cut -d' ' -f1); \
	sed -i -e "s/$2 := .*/$2 := $${NEW_SHA256}/g" Makefile.versions
endef

# usage: update-aqua-package NAME (e.g. cli/cli, kubernetes-sigs/controller-tools/controller-gen)
# bump an aqua.yaml package to its latest release
# "aqua g" prints nothing when the pinned version is already the latest, so skip the update in that case
define update-aqua-package
	NEW_VERSION=$$(aqua g $1 2>/dev/null | tail -n 1 | sed -E 's/.*@//'); \
	if [ -n "$$NEW_VERSION" ]; then \
		$(YQ) -i '(.packages[] | select(.name | test("^$1@"))).name = "$1@'"$$NEW_VERSION"'"' aqua.yaml; \
	fi
endef

# usage update-trusted-action OWNER/REPO VERSION
define update-trusted-action
	for i in $(shell ls $(WORKFLOWS_DIR)); do \
		$(YQ) -i '(.. | select(has("uses")) | select(.uses | contains("$1"))).uses = "$1@v$2"' $(WORKFLOWS_DIR)/$$i; \
	done
endef
