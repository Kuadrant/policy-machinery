# Setting SHELL to bash allows bash commands to be executed by recipes.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

MKFILE_PATH := $(abspath $(lastword $(MAKEFILE_LIST)))
PROJECT_PATH := $(patsubst %/,%,$(dir $(MKFILE_PATH)))

##@ General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk commands is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-30s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

.PHONY: project-path
project-path: ## Print the project path.
	@echo $(PROJECT_PATH)

##@ Tools

# go-install-tool will 'go install' any package $2 and install it to $1.
PROJECT_DIR := $(shell dirname $(abspath $(lastword $(MAKEFILE_LIST))))
define go-install-tool
@[ -f $(1) ] || { \
set -e ;\
TMP_DIR=$$(mktemp -d) ;\
cd $$TMP_DIR ;\
go mod init tmp ;\
echo "Downloading $(2)" ;\
GOBIN=$(PROJECT_DIR)/bin go install $(2) ;\
rm -rf $$TMP_DIR ;\
}
endef

CONTROLLER_GEN = $(PROJECT_PATH)/bin/controller-gen
$(CONTROLLER_GEN):
	$(call go-install-tool,$(CONTROLLER_GEN),sigs.k8s.io/controller-tools/cmd/controller-gen@v0.16.0)

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN)  ## Download controller-gen locally if necessary.

HELM_VERSION = v3.15.0
HELM = $(PROJECT_PATH)/bin/helm
$(HELM):
	@{ \
	set -e ;\
	mkdir -p $(dir $(HELM)) ;\
	OS=$(shell go env GOOS) && ARCH=$(shell go env GOARCH) && \
	curl -sL -o helm.tar.gz https://get.helm.sh/helm-$(HELM_VERSION)-$${OS}-$${ARCH}.tar.gz ;\
	tar -zxvf helm.tar.gz ;\
	mv $${OS}-$${ARCH}/helm $(HELM) ;\
	chmod +x $(HELM) ;\
	rm -rf $${OS}-$${ARCH} helm.tar.gz ;\
	}

.PHONY: helm
helm: $(HELM) ## Download helm locally if necessary.

##@ Development

.PHONY: generate
generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

.PHONY: manifests
manifests: controller-gen ## Generate CustomResourceDefinition objects.
	$(CONTROLLER_GEN) crd paths="./apis/v1alpha2;./apis/v1beta3" output:crd:artifacts:config=config/crds

.PHONY: install
install: manifests ## Install CRDs into a cluster.
	kubectl apply -f https://github.com/kubernetes-sigs/gateway-api/releases/download/v1.1.0/standard-install.yaml
	kubectl apply -f config/crds

.PHONY: run
run: generate ## Run the controller.
ifneq ($(PROVIDERS),)
	go run *.go --gateway-providers $(PROVIDERS)
else
	go run *.go
endif

##@ Testing

.PHONY: install-envoy-gateway
install-envoy-gateway: helm ## Install Envoy Gateway.
	$(HELM) install eg oci://docker.io/envoyproxy/gateway-helm --version v1.0.2 -n envoy-gateway-system --create-namespace
	kubectl wait --timeout=5m -n envoy-gateway-system deployment/envoy-gateway --for=condition=Available

.PHONY: install-istio
install-istio: helm ## Install Istio.
	$(HELM) repo add istio https://istio-release.storage.googleapis.com/charts
	$(HELM) repo update
	kubectl create namespace istio-system
	$(HELM) install istio-base istio/base -n istio-system --set defaultRevision=default
	$(HELM) install istiod istio/istiod -n istio-system --wait

.PHONY: install-kuadrant
install-kuadrant: ## Install Kuadrant CRDs.
	kubectl apply -f config/crds
