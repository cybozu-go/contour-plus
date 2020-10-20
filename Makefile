
# Image URL to use all building/pushing image targets
IMG ?= quay.io/cybozu/contour-plus:latest
# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:trivialVersions=true"

GO111MODULE = on
KUBEBUILDER_ASSETS := $(PWD)/bin
export GO111MODULE KUBEBUILDER_ASSETS 

GOOS = $(shell go env GOOS)
GOARCH = $(shell go env GOARCH)
SUDO = sudo
KUBEBUILDER_VERSION = 2.3.1
CTRLTOOLS_VERSION = 0.2.8
CERT_MANAGER_VERSION := 1.0.3
EXTERNAL_DNS_VERSION := 0.7.4
CONTOUR_VERSION := 1.9.0

.PHONY: all
all: bin/contour-plus

# Run tests
.PHONY: test
test:
	test -z "$$(gofmt -s -l . | tee /dev/stderr)"
	staticcheck ./...
	test -z "$$(nilerr ./... 2>&1 | tee /dev/stderr)"
	test -z "$$(custom-checker -restrictpkg.packages=html/template,log $$(go list -tags='$(GOTAGS)' ./... ) 2>&1 | tee /dev/stderr)"
	ineffassign .
	go test -race -v -count 1 ./controllers/... -coverprofile cover.out
	go install ./...
	go vet ./...

# Build contour-plus binary
bin/contour-plus: main.go cmd/root.go controllers/httpproxy_controller.go
	CGO_ENABLED=0 go build -o $@ .

# Generate manifests e.g. CRD, RBAC etc.
.PHONY: manifests
manifests: controller-gen
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=contour-plus paths="./..."

# Generate code
.PHONY: generate
generate: controller-gen
	$(CONTROLLER_GEN) object:headerFile=./hack/boilerplate.go.txt paths=./apis/contour/...

# Build the docker image
.PHONY: docker-build
docker-build: bin/contour-plus
	docker build . -t ${IMG}
	@echo "updating kustomize image patch file for manager resource"
	sed -i'' -e 's@image: .*@image: '"${IMG}"'@' ./config/default/manager_image_patch.yaml

# Push the docker image
.PHONY: docker-push
docker-push:
	docker push ${IMG}

# find or download controller-gen
# download controller-gen if necessary
.PHONY: controller-gen
controller-gen:
ifeq (, $(shell which controller-gen))
	cd $(shell mktemp -d) && curl -sSLfO https://github.com/kubernetes-sigs/controller-tools/archive/v$(CTRLTOOLS_VERSION).tar.gz && tar -x -z -f v$(CTRLTOOLS_VERSION).tar.gz && cd controller-tools-$(CTRLTOOLS_VERSION) && GOFLAGS= go install ./cmd/controller-gen
CONTROLLER_GEN=$(shell go env GOPATH)/bin/controller-gen
else
CONTROLLER_GEN=$(shell which controller-gen)
endif

.PHONY: clean
clean:
	rm -f bin/contour-plus $(CONTROLLER_GEN)

.PHONY: setup
setup: custom-checker staticcheck nilerr ineffassign
	mkdir -p bin
	curl -sfL https://go.kubebuilder.io/dl/$(KUBEBUILDER_VERSION)/$(GOOS)/$(GOARCH) | tar -xz -C /tmp/
	mv /tmp/kubebuilder_$(KUBEBUILDER_VERSION)_$(GOOS)_$(GOARCH)/bin/* bin/
	rm -rf /tmp/kubebuilder_*
	curl -o bin/kustomize -sfL https://go.kubebuilder.io/kustomize/$(GOOS)/$(GOARCH)
	chmod a+x bin/kustomize
	go install github.com/jstemmer/go-junit-report

.PHONY: mod
mod:
	go mod tidy
	git add go.mod

.PHONY: download-upstream-crd
download-upstream-crd:
	curl -o config/crd/third/certmanager.yml -sLf https://github.com/jetstack/cert-manager/releases/download/v$(CERT_MANAGER_VERSION)/cert-manager.crds.yaml
	curl -o config/crd/third/dnsendpoint.yml -sLf https://github.com/kubernetes-sigs/external-dns/raw/v$(EXTERNAL_DNS_VERSION)/docs/contributing/crd-source/crd-manifest.yaml
	curl -o config/crd/third/httpproxy.yml -sLf https://github.com/projectcontour/contour/raw/v$(CONTOUR_VERSION)/examples/contour/01-crds.yaml

.PHONY: custom-checker
custom-checker:
	if ! which custom-checker >/dev/null; then \
		cd /tmp; env GOFLAGS= GO111MODULE=on go get github.com/cybozu/neco-containers/golang/analyzer/cmd/custom-checker; \
	fi

.PHONY: staticcheck
staticcheck:
	if ! which staticcheck >/dev/null; then \
		cd /tmp; env GOFLAGS= GO111MODULE=on go get honnef.co/go/tools/cmd/staticcheck; \
	fi

.PHONY: nilerr
nilerr:
	if ! which nilerr >/dev/null; then \
		cd /tmp; env GOFLAGS= GO111MODULE=on go get github.com/gostaticanalysis/nilerr/cmd/nilerr; \
	fi

.PHONY: ineffassign
ineffassign:
	if ! which ineffassign >/dev/null; then \
		cd /tmp; env GOFLAGS= GO111MODULE=on go get github.com/gordonklaus/ineffassign; \
	fi
