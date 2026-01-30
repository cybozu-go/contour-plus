THIS_MK_DIR := $(dir $(abspath $(lastword $(MAKEFILE_LIST))))
PARENT_DIR := $(abspath $(THIS_MK_DIR)/..)

include $(PARENT_DIR)/Makefile
CRD_DIR := $(PARENT_DIR)/config/crd/third

STABLE ?= $(shell git tag --list | sort -rV | head -n1 | sed 's/^v//')

.PHONY: install-crds
install-crds:
	$(KUBECTL) apply -f $(CRD_DIR)

.PHONY: update-cert-manager-manifests
update-cert-manager-manifests:
	mkdir -p testdata/cert-manager/upstream
	$(call get-latest-gh-package-tag,cert-manager)
	curl -sSLf https://github.com/jetstack/cert-manager/releases/download/$(call upstream-tag,$(latest_tag))/cert-manager.yaml |\
		$(YQ) eval 'select(.$(KIND) != "CustomResourceDefinition")' > testdata/cert-manager/upstream/cert-manager.yaml
	$(YQ) -i "(.images[] | select(.name==\"ghcr.io/cybozu/cert-manager\")).newTag = \"$(latest_tag)\"" testdata/cert-manager/kustomization.yaml

.PHONY: install-cert-manager
install-cert-manager:
	$(KUSTOMIZE) build testdata/cert-manager | $(KUBECTL) apply -f -
	$(KUBECTL) -n cert-manager wait pod --for=condition=Ready -l app.kubernetes.io/instance=cert-manager --timeout=5m
	$(KUBECTL) apply -f testdata/cert-manager/clusterissuer.yaml

.PHONY: update-coredns-manifests
update-coredns-manifests:
	mkdir -p testdata/coredns/upstream
	curl -sSLf -o testdata/coredns/upstream/etcd.yaml https://raw.githubusercontent.com/kubernetes-sigs/external-dns/refs/tags/v$(EXTERNAL_DNS_VERSION)/docs/snippets/tutorials/coredns/etcd.yaml
	curl -sSLf -o testdata/coredns/upstream/values-coredns.yaml https://raw.githubusercontent.com/kubernetes-sigs/external-dns/refs/tags/v$(EXTERNAL_DNS_VERSION)/docs/snippets/tutorials/coredns/values-coredns.yaml
	# replace etcd resource namespace
	$(YQ) -i 'select(.metadata.namespace) | .metadata.namespace = "external-dns"' testdata/coredns/upstream/etcd.yaml	
	# replace binary path with that of cybozu build
	sed -i 's#/usr/local/bin/etcd#/usr/local/etcd/bin/etcd#' testdata/coredns/upstream/etcd.yaml
	# replace etcd servcie namespace in coredns config
	sed -i 's#http://etcd.default.svc.cluster.local:2379#http://etcd.external-dns.svc.cluster.local:2379#' testdata/coredns/upstream/values-coredns.yaml
	$(HELM) template -n external-dns -f testdata/coredns/upstream/values-coredns.yaml coredns coredns/coredns > testdata/coredns/upstream/coredns.yaml
	$(call get-latest-gh-package-tag,etcd)
	$(YQ) -i "(.images[] | select(.newName==\"ghcr.io/cybozu/etcd\")).newTag = \"$(latest_tag)\"" testdata/coredns/kustomization.yaml
	$(call get-latest-gh-package-tag,coredns)
	$(YQ) -i "(.images[] | select(.newName==\"ghcr.io/cybozu/coredns\")).newTag = \"$(latest_tag)\"" testdata/coredns/kustomization.yaml

.PHONY: install-coredns
install-coredns:
	$(KUSTOMIZE) build testdata/coredns | $(KUBECTL) apply -f -
	$(KUBECTL) rollout status statefulset/etcd -n external-dns --timeout=5m
	$(KUBECTL) rollout status deployment/coredns -n external-dns --timeout=5m

.PHONY: update-external-dns-manifests
update-external-dns-manifests:
	mkdir -p testdata/external-dns/upstream 
	$(call get-latest-gh-package-tag,external-dns)
	d=$$(mktemp -d); \
	cd $$d; git clone --depth 1 -b $(call upstream-tag,$(latest_tag)) https://github.com/kubernetes-sigs/external-dns.git; \
	cp -r $$d/external-dns/kustomize/. $(THIS_MK_DIR)/testdata/external-dns/upstream
	$(YQ) -i "(.images[] | select(.name==\"ghcr.io/cybozu/external-dns\")).newTag = \"$(latest_tag)\"" testdata/external-dns/kustomization.yaml

.PHONY: install-external-dns
install-external-dns:
	$(KUSTOMIZE) build testdata/external-dns | $(KUBECTL) apply -f -
	$(KUBECTL) rollout status deployment/external-dns -n external-dns --timeout=5m

.PHONY: update-contour-manifests
update-contour-manifests:
	mkdir -p testdata/contour/upstream 
	$(call get-latest-gh-package-tag,contour)
	d=$$(mktemp -d); \
	cd $$d; git clone --depth 1 -b $(call upstream-tag,$(latest_tag)) https://github.com/projectcontour/contour.git; \
	cp -r $$d/contour/examples/contour/. $(THIS_MK_DIR)/testdata/contour/upstream
	$(YQ) -i "(.images[] | select(.newName==\"ghcr.io/cybozu/contour\")).newTag = \"$(latest_tag)\"" testdata/contour/kustomization.yaml
	$(call get-latest-gh-package-tag,envoy)
	$(YQ) -i "(.images[] | select(.newName==\"ghcr.io/cybozu/envoy\")).newTag = \"$(latest_tag)\"" testdata/contour/kustomization.yaml

.PHONY: install-contour
install-contour:
	$(KUSTOMIZE) build testdata/contour | $(KUBECTL) apply -f -
	$(KUBECTL) rollout status deployment/contour -n projectcontour --timeout=5m
	$(KUBECTL) rollout status daemonset/envoy -n projectcontour --timeout=5m

.PHONY: install-contour-plus
install-contour-plus:
	cd $(PARENT_DIR); $(MAKE) docker-build
	$(KUSTOMIZE) build testdata/contour-plus | $(KUBECTL) apply -f -
	$(KUBECTL) rollout status deployment/contour-plus -n projectcontour --timeout=5m

.PHONY: start
start: update-dependencies
	$(KIND) create cluster --config testdata/cluster.yaml
	$(KUBECTL) apply -f testdata/namespace.yaml
	$(MAKE) install-crds
	$(MAKE) install-cert-manager
	$(MAKE) install-coredns
	$(MAKE) install-external-dns
	$(MAKE) install-contour
	cd $(PARENT_DIR); $(MAKE) docker-build IMG=ghcr.io/cybozu-go/contour-plus:dev
	$(KIND) load docker-image ghcr.io/cybozu-go/contour-plus:dev --name contour-plus-e2e
	# contour-plus is installed via the suite test

.PHONY: stop
stop:
	$(KIND) delete cluster --name contour-plus-e2e
	docker kill external-client || true

.PHONY: reload
reload:
	$(MAKE) stop
	$(MAKE) start

.PHONY: e2e
e2e: reload
	echo "Running e2e test with ${STABLE} as the base version."
	STABLE=${STABLE} go test -count=1 -v ./...

.PHONY: update-kind
update-kind:
	$(YQ) -i '.nodes[].image = "kindest/node:v${KINDEST_NODE_VERSION}"' testdata/cluster.yaml

.PHONY: update-dependencies
update-dependencies:
	$(MAKE) update-kind
	$(MAKE) update-cert-manager-manifests
	$(MAKE) update-coredns-manifests
	$(MAKE) update-external-dns-manifests
	$(MAKE) update-contour-manifests
