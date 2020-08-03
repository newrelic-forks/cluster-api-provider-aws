# Copyright 2018 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# If you update this file, please follow
# https://suva.sh/posts/well-documented-makefiles

# Ensure Make is run with bash shell as some syntax below is bash-specific
SHELL:=/usr/bin/env bash

.DEFAULT_GOAL:=help

# Use GOPROXY environment variable if set
GOPROXY := $(shell go env GOPROXY)
ifeq ($(GOPROXY),)
GOPROXY := https://proxy.golang.org
endif
export GOPROXY

# Active module mode, as we use go modules to manage dependencies
export GO111MODULE=on

# Directories.
ARTIFACTS ?= $(REPO_ROOT)/_artifacts
TOOLS_DIR := hack/tools
TOOLS_BIN_DIR := $(TOOLS_DIR)/bin
TOOLS_SHARE_DIR := $(TOOLS_DIR)/share
BIN_DIR := bin
REPO_ROOT := $(shell git rev-parse --show-toplevel)
TEST_E2E_DIR := test/e2e
TEST_E2E_NEW_DIR := test/e2e_new

# Files
E2E_DATA_DIR ?= $(REPO_ROOT)/test/e2e_new/data
E2E_CONF_PATH  ?= $(E2E_DATA_DIR)/e2e_conf.yaml
KUBETEST_CONF_PATH ?= $(abspath $(E2E_DATA_DIR)/kubetest/conformance.yaml)
KUBETEST_FAST_CONF_PATH ?= $(abspath $(REPO_ROOT)/test/e2e_new/data/kubetest/conformance-fast.yaml)

# Binaries.
CLUSTERCTL := $(BIN_DIR)/clusterctl
KUSTOMIZE := $(TOOLS_BIN_DIR)/kustomize
KIND := $(TOOLS_BIN_DIR)/kind
CONTROLLER_GEN := $(TOOLS_BIN_DIR)/controller-gen
DEFAULTER_GEN := $(TOOLS_BIN_DIR)/defaulter-gen
ENVSUBST := $(TOOLS_BIN_DIR)/envsubst
GOLANGCI_LINT := $(TOOLS_BIN_DIR)/golangci-lint
MOCKGEN := $(TOOLS_BIN_DIR)/mockgen
CONVERSION_GEN := $(TOOLS_BIN_DIR)/conversion-gen
RELEASE_NOTES_BIN := bin/release-notes
RELEASE_NOTES := $(TOOLS_DIR)/$(RELEASE_NOTES_BIN)
GINKGO := $(TOOLS_BIN_DIR)/ginkgo
SSM_PLUGIN := $(TOOLS_BIN_DIR)/session-manager-plugin

UNAME := $(shell uname -s)
PATH := $(abspath $(TOOLS_BIN_DIR)):$(PATH)

export PATH

# Define Docker related variables. Releases should modify and double check these vars.

# TODO this means anyone without a default gcloud project + gcloud binary will break on default `make docker-build` target, remove this gcloud dep if possible.
REGISTRY ?= gcr.io/$(shell gcloud config get-value project)
STAGING_REGISTRY := gcr.io/k8s-staging-cluster-api-aws
PROD_REGISTRY := us.gcr.io/k8s-artifacts-prod/cluster-api-aws
IMAGE_NAME ?= cluster-api-aws-controller
CONTROLLER_IMG ?= $(REGISTRY)/$(IMAGE_NAME)
TAG ?= dev
ARCH ?= amd64
ALL_ARCH = amd64 arm arm64 ppc64le s390x

# bootstrap
EKS_BOOTSTRAP_IMAGE_NAME ?= eks-bootstrap-controller
EKS_BOOTSTRAP_CONTROLLER_IMG ?= $(REGISTRY)/$(EKS_BOOTSTRAP_IMAGE_NAME)

# Allow overriding manifest generation destination directory
MANIFEST_ROOT ?= config
CRD_ROOT ?= $(MANIFEST_ROOT)/crd/bases
WEBHOOK_ROOT ?= $(MANIFEST_ROOT)/webhook
RBAC_ROOT ?= $(MANIFEST_ROOT)/rbac

# Allow overriding the imagePullPolicy
PULL_POLICY ?= Always

# Hosts running SELinux need :z added to volume mounts
SELINUX_ENABLED := $(shell cat /sys/fs/selinux/enforce 2> /dev/null || echo 0)

ifeq ($(SELINUX_ENABLED),1)
  DOCKER_VOL_OPTS?=:z
endif

# Set build time variables including version details
LDFLAGS := $(shell source ./hack/version.sh; version::ldflags)

GOLANG_VERSION := 1.13.8

# 'functional tests' as the ginkgo filter will run ALL tests ~ 2 hours @ 3 node concurrency.
E2E_FOCUS ?= "functional tests"
# Instead, you can run a quick smoke test, it should run fast (9 minutes)...
# E2E_FOCUS := "Create cluster with name having"

GINKGO_NODES ?= 2

## --------------------------------------
## Help
## --------------------------------------

help:  ## Display this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

## --------------------------------------
## Testing
## --------------------------------------

$(ARTIFACTS):
	mkdir -p $@

.PHONY: test
test: ## Run tests
	source ./scripts/fetch_ext_bins.sh; fetch_tools; setup_envs; go test -v ./...

.PHONY: test-integration
test-integration: ## Run integration tests
	source ./scripts/fetch_ext_bins.sh; fetch_tools; setup_envs; go test -v -tags=integration ./test/integration/...

.PHONY: test-e2e
test-e2e: $(GINKGO) $(KIND) ## Run e2e tests
	PULL_POLICY=IfNotPresent $(MAKE) docker-build
	cd $(TEST_E2E_DIR); time $(GINKGO) -nodes=$(GINKGO_NODES) -v -tags=e2e -focus=$(E2E_FOCUS) $(GINKGO_ARGS) ./... -- -managerImage=$(CONTROLLER_IMG)-$(ARCH):$(TAG) $(E2E_ARGS)

.PHONY: test-e2e-new ## Run new e2e tests using clusterctl
test-e2e-new: $(GINKGO) $(KIND) $(SSM_PLUGIN) $(KUSTOMIZE) e2e-image ## Run e2e tests
	time $(GINKGO) -trace -progress -v -tags=e2e -focus=$(E2E_FOCUS) $(GINKGO_ARGS) ./test/e2e_new/... -- -config-path="$(E2E_CONF_PATH)" -artifacts-folder="$(ARTIFACTS)" $(E2E_ARGS)

.PHONY: e2e-image
e2e-image:
ifndef FASTBUILD
	docker build -f Dockerfile --tag="capa-manager:e2e" .
else
	$(MAKE) manager
	docker build -f Dockerfile.fastbuild --tag="capa-manager:e2e" .
endif

.PHONY: test-conformance
test-conformance: ## Run conformance test on workload cluster
	PULL_POLICY=IfNotPresent $(MAKE) docker-build
	cd $(TEST_E2E_DIR); go test -v -tags=e2e -timeout=4h . -args -ginkgo.v -ginkgo.focus "conformance tests" --managerImage $(CONTROLLER_IMG)-$(ARCH):$(TAG)

CONFORMANCE_E2E_ARGS ?= -kubetest.config-file=$(KUBETEST_CONF_PATH)
CONFORMANCE_E2E_ARGS += $(E2E_ARGS)
CONFORMANCE_GINKGO_ARGS ?= -stream
CONFORMANCE_GINKGO_ARGS += $(GINKGO_ARGS)
.PHONY: test-conformance-new
test-conformance-new: ## Run clusterctl based conformance test on workload cluster (requires Docker).
	$(MAKE) test-e2e-new E2E_FOCUS="conformance" E2E_ARGS='$(CONFORMANCE_E2E_ARGS)' GINKGO_ARGS='$(CONFORMANCE_GINKGO_ARGS)'

test-conformance-fast: ## Run clusterctl based conformance test on workload cluster (requires Docker) using a subset of the conformance suite in parallel. Run with FASTBUILD=true to skip full CAPA rebuild.
	$(MAKE) test-conformance-new CONFORMANCE_E2E_ARGS="-kubetest.config-file=$(KUBETEST_FAST_CONF_PATH) -kubetest.ginkgo-nodes=5 $(E2E_ARGS)"
## --------------------------------------
## Binaries
## --------------------------------------
.PHONY: binaries
binaries: managers clusterawsadm ## Builds and installs all binaries

.PHONY: managers
managers:
	$(MAKE) manager-aws-infrastructure
	$(MAKE) manager-eks-bootstrap

.PHONY: manager-aws-infrastructure
manager-aws-infrastructure: ## Build manager binary.
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "${LDFLAGS} -extldflags '-static'" -o $(BIN_DIR)/manager .

.PHONY: manager-eks-bootstrap
manager-eks-bootstrap:
	go build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/eks-bootstrap-manager sigs.k8s.io/cluster-api-provider-aws/bootstrap/eks

.PHONY: clusterawsadm
clusterawsadm: ## Build clusterawsadm binary.
	go build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/clusterawsadm ./cmd/clusterawsadm

## --------------------------------------
## Tooling Binaries
## --------------------------------------

$(TOOLS_BIN_DIR):
	mkdir -p $@

$(TOOLS_SHARE_DIR):
	mkdir -p $@

$(CLUSTERCTL): go.mod ## Build clusterctl binary.
	go build -o $(BIN_DIR)/clusterctl sigs.k8s.io/cluster-api/cmd/clusterctl

$(CONTROLLER_GEN): $(TOOLS_DIR)/go.mod # Build controller-gen from tools folder.
	cd $(TOOLS_DIR); go build -tags=tools -o $(subst hack/tools/,,$@) sigs.k8s.io/controller-tools/cmd/controller-gen

$(ENVSUBST): $(TOOLS_DIR)/go.mod # Build envsubst from tools folder.
	cd $(TOOLS_DIR); go build -tags=tools -o $(subst hack/tools/,,$@) github.com/a8m/envsubst/cmd/envsubst

$(GOLANGCI_LINT): $(TOOLS_DIR)/go.mod # Build golangci-lint from tools folder.
	cd $(TOOLS_DIR); go build -tags=tools -o $(subst hack/tools/,,$@) github.com/golangci/golangci-lint/cmd/golangci-lint

$(MOCKGEN): $(TOOLS_DIR)/go.mod # Build mockgen from tools folder.
	cd $(TOOLS_DIR); go build -tags=tools -o $(subst hack/tools/,,$@) github.com/golang/mock/mockgen

$(CONVERSION_GEN): $(TOOLS_DIR)/go.mod
	cd $(TOOLS_DIR); go build -tags=tools -o $(subst hack/tools/,,$@) k8s.io/code-generator/cmd/conversion-gen

$(DEFAULTER_GEN): $(TOOLS_DIR)/go.mod
	cd $(TOOLS_DIR); go build -tags=tools -o $(subst hack/tools/,,$@) k8s.io/code-generator/cmd/defaulter-gen

$(KUSTOMIZE): $(TOOLS_DIR)/go.mod # Build kustomize from tools folder.
	cd $(TOOLS_DIR); go build -tags=tools -o $(subst hack/tools/,,$@) sigs.k8s.io/kustomize/kustomize/v3

$(RELEASE_NOTES) : $(TOOLS_DIR)/go.mod
	cd $(TOOLS_DIR) && go build -tags tools -o $(subst hack/tools/,,$@) sigs.k8s.io/cluster-api/hack/tools/release

$(KIND): $(TOOLS_DIR)/go.mod
	cd $(TOOLS_DIR) && go build -tags tools -o $(subst hack/tools/,,$@) sigs.k8s.io/kind

$(GINKGO): $(TOOLS_DIR)/go.mod
	cd $(TOOLS_DIR) && go build -tags=tools -o $(subst hack/tools/,,$@) github.com/onsi/ginkgo/ginkgo

## ------------------------------------------------------------------------------------------------
## AWS Session Manager Plugin Installation. Currently support Linux and MacOS AMD64 architectures.
## ------------------------------------------------------------------------------------------------

SSM_SHARE := $(TOOLS_SHARE_DIR)/ssm

$(SSM_SHARE): $(TOOLS_SHARE_DIR)
	mkdir -p $@

$(SSM_SHARE)/session-manager-plugin.deb: $(SSM_SHARE)
	curl "https://s3.amazonaws.com/session-manager-downloads/plugin/latest/ubuntu_64bit/session-manager-plugin.deb" -o $@

$(SSM_SHARE)/sessionmanager-bundle.zip: $(SSM_SHARE)
	curl "https://s3.amazonaws.com/session-manager-downloads/plugin/latest/mac/sessionmanager-bundle.zip" -o $@

$(SSM_SHARE)/data.tar.gz: $(SSM_SHARE)/session-manager-plugin.deb
	cd $(SSM_SHARE) && ar x session-manager-plugin.deb data.tar.gz

$(SSM_PLUGIN): $(TOOLS_BIN_DIR)
ifeq ($(UNAME), Linux)
	$(MAKE) $(SSM_SHARE)/data.tar.gz
	cd $(TOOLS_BIN_DIR) && tar -xvf ../share/ssm/data.tar.gz usr/local/sessionmanagerplugin/bin/session-manager-plugin --strip-components 4 --directory $(TOOLS_BIN_DIR)
endif
ifeq ($(UNAME), Darwin)
	$(MAKE) $(SSM_SHARE)/sessionmanager-bundle.zip
	cd $(TOOLS_BIN_DIR) && unzip -j ../share/ssm/sessionmanager-bundle.zip sessionmanager-bundle/bin/session-manager-plugin
endif

## --------------------------------------
## Linting
## --------------------------------------

.PHONY: lint
lint: $(GOLANGCI_LINT) ## Lint codebase
	$(GOLANGCI_LINT) run -v --fast=false

## --------------------------------------
## Generate
## --------------------------------------

.PHONY: modules
modules: ## Runs go mod to ensure proper vendoring.
	go mod tidy
	cd $(TOOLS_DIR); go mod tidy
	cd $(TEST_E2E_DIR); go mod tidy

.PHONY: generate
generate: ## Generate code
	$(MAKE) generate-go
	$(MAKE) generate-manifests

.PHONY: generate-go
generate-go:
	$(MAKE) generate-go-core
	$(MAKE) generate-go-eks-bootstrap

.PHONY: generate-go-core
generate-go-core: $(CONTROLLER_GEN) $(CONVERSION_GEN) $(MOCKGEN) $(DEFAULTER_GEN) ## Runs Go related generate targets
	$(CONTROLLER_GEN) \
		paths=./api/... \
		object:headerFile=./hack/boilerplate/boilerplate.generatego.txt

	$(CONTROLLER_GEN) \
		paths=./cmd/clusterawsadm/api/... \
		object:headerFile=./hack/boilerplate/boilerplate.generatego.txt

	$(DEFAULTER_GEN) \
		--input-dirs=./cmd/clusterawsadm/api/bootstrap/v1alpha1,./cmd/clusterawsadm/api/iam/v1alpha1 \
		--v=0 \
		--go-header-file=./hack/boilerplate/boilerplate.generatego.txt

	$(CONVERSION_GEN) \
		--input-dirs=./api/v1alpha2 \
		--output-file-base=zz_generated.conversion \
		--go-header-file=./hack/boilerplate/boilerplate.generatego.txt

.PHONY: generate-go-eks-bootstrap
generate-go-eks-bootstrap: $(CONTROLLER_GEN) $(CONVERSION_GEN)
	$(CONTROLLER_GEN) \
		paths=./bootstrap/eks/api/... \
		paths=./bootstrap/eks/types/... \
		object:headerFile=./hack/boilerplate/boilerplate.generatego.txt

.PHONY: generate-manifests
generate-manifests:
	$(MAKE) generate-core-manifests
	$(MAKE) generate-eks-bootstrap-manifests

.PHONY: generate-core-manifests
generate-core-manifests: $(CONTROLLER_GEN) ## Generate manifests e.g. CRD, RBAC etc.
	$(CONTROLLER_GEN) \
		paths=./api/... \
		crd:crdVersions=v1 \
		output:crd:dir=$(CRD_ROOT) \
		output:webhook:dir=$(WEBHOOK_ROOT) \
		webhook
	$(CONTROLLER_GEN) \
		paths=./controllers/... \
		output:rbac:dir=$(RBAC_ROOT) \
		rbac:roleName=manager-role

.PHONY: generate-eks-bootstrap-manifests
generate-eks-bootstrap-manifests: $(CONTROLLER_GEN)
	$(CONTROLLER_GEN) \
		paths=./bootstrap/eks/api/... \
		paths=./bootstrap/eks/controllers/... \
		crd:crdVersions=v1 \
		rbac:roleName=manager-role \
		output:crd:dir=./bootstrap/eks/config/crd/bases \
		output:rbac:dir=./bootstrap/eks/config/rbac \
		output:webhook:dir=./bootstrap/eks/config/webhook \
		webhook

## --------------------------------------
## Docker
## --------------------------------------

.PHONY: docker-build
docker-build:
	$(MAKE) ARCH=$(ARCH) docker-build-core
	$(MAKE) ARCH=$(ARCH) docker-build-eks-bootstrap

.PHONY: docker-build-core
docker-build-core: ## Build the docker image for controller-manager
	docker build --pull --build-arg ARCH=$(ARCH) --build-arg LDFLAGS="$(LDFLAGS)" . -t $(CONTROLLER_IMG)-$(ARCH):$(TAG)
	$(MAKE) set-manifest-image MANIFEST_IMG=$(CONTROLLER_IMG)-$(ARCH) MANIFEST_TAG=$(TAG) TARGET_RESOURCE="./config/manager/manager_image_patch.yaml"
	$(MAKE) set-manifest-pull-policy TARGET_RESOURCE="./config/manager/manager_pull_policy.yaml"

.PHONY: docker-build-eks-bootstrap
docker-build-eks-bootstrap:
	docker build --pull --build-arg ARCH=$(ARCH) --build-arg LDFLAGS="$(LDFLAGS)" --build-arg package=./bootstrap/eks . -t $(EKS_BOOTSTRAP_CONTROLLER_IMG)-$(ARCH):$(TAG)
	$(MAKE) set-manifest-image MANIFEST_IMG=$(EKS_BOOTSTRAP_CONTROLLER_IMG)-$(ARCH) MANIFEST_TAG=$(TAG) TARGET_RESOURCE="./bootstrap/eks/config/manager/manager_image_patch.yaml"
	$(MAKE) set-manifest-pull-policy TARGET_RESOURCE="./bootstrap/eks/config/manager/manager_pull_policy.yaml"


.PHONY: docker-push
docker-push: ## Push the docker image
	docker push $(CONTROLLER_IMG)-$(ARCH):$(TAG)
	docker push $(EKS_BOOTSTRAP_CONTROLLER_IMG)-$(ARCH):$(TAG)

## --------------------------------------
## Docker — All ARCH
## --------------------------------------

.PHONY: docker-build-all ## Build all the architecture docker images
docker-build-all: $(addprefix docker-build-,$(ALL_ARCH))

docker-build-%:
	$(MAKE) ARCH=$* docker-build

.PHONY: docker-push-all ## Push all the architecture docker images
docker-push-all: $(addprefix docker-push-,$(ALL_ARCH))
	$(MAKE) docker-push-core-manifest
	$(MAKE) docker-push-eks-bootstrap-manifest

docker-push-%:
	$(MAKE) ARCH=$* docker-push

.PHONY: docker-push-core-manifest
docker-push-core-manifest: ## Push the fat manifest docker image.
	## Minimum docker version 18.06.0 is required for creating and pushing manifest images.
	docker manifest create --amend $(CONTROLLER_IMG):$(TAG) $(shell echo $(ALL_ARCH) | sed -e "s~[^ ]*~$(CONTROLLER_IMG)\-&:$(TAG)~g")
	@for arch in $(ALL_ARCH); do docker manifest annotate --arch $${arch} ${CONTROLLER_IMG}:${TAG} ${CONTROLLER_IMG}-$${arch}:${TAG}; done
	docker manifest push --purge ${CONTROLLER_IMG}:${TAG}
	$(MAKE) set-manifest-image MANIFEST_IMG=$(CONTROLLER_IMG)-$(ARCH) MANIFEST_TAG=$(TAG) TARGET_RESOURCE="./config/manager/manager_image_patch.yaml"
	$(MAKE) set-manifest-pull-policy TARGET_RESOURCE="./config/manager/manager_pull_policy.yaml"

.PHONY: docker-push-eks-bootstrap-manifest
docker-push-eks-bootstrap-manifest: ## Push the fat manifest docker image.
	## Minimum docker version 18.06.0 is required for creating and pushing manifest images.
	docker manifest create --amend $(EKS_BOOTSTRAP_CONTROLLER_IMG):$(TAG) $(shell echo $(ALL_ARCH) | sed -e "s~[^ ]*~$(EKS_BOOTSTRAP_CONTROLLER_IMG)\-&:$(TAG)~g")
	@for arch in $(ALL_ARCH); do docker manifest annotate --arch $${arch} ${EKS_BOOTSTRAP_CONTROLLER_IMG}:${TAG} ${EKS_BOOTSTRAP_CONTROLLER_IMG}-$${arch}:${TAG}; done
	docker manifest push --purge ${EKS_BOOTSTRAP_CONTROLLER_IMG}:${TAG}
	$(MAKE) set-manifest-image MANIFEST_IMG=$(EKS_BOOTSTRAP_CONTROLLER_IMG)-$(ARCH) MANIFEST_TAG=$(TAG) TARGET_RESOURCE="./bootstrap/eks/config/manager/manager_image_patch.yaml"
	$(MAKE) set-manifest-pull-policy TARGET_RESOURCE="./bootstrap/eks/config/manager/manager_pull_policy.yaml"

.PHONY: set-manifest-image
set-manifest-image:
	$(info Updating kustomize image patch file for manager resource)
	sed -i'' -e 's@image: .*@image: '"${MANIFEST_IMG}:$(MANIFEST_TAG)"'@' $(TARGET_RESOURCE)

.PHONY: set-manifest-pull-policy
set-manifest-pull-policy:
	$(info Updating kustomize pull policy file for manager resources)
	sed -i'' -e 's@imagePullPolicy: .*@imagePullPolicy: '"$(PULL_POLICY)"'@' $(TARGET_RESOURCE)

## --------------------------------------
## Release
## --------------------------------------

RELEASE_TAG := $(shell git describe --abbrev=0 2>/dev/null)
RELEASE_DIR := out

$(RELEASE_DIR):
	mkdir -p $@

.PHONY: release
release: clean-release  ## Builds and push container images using the latest git tag for the commit.
	@if [ -z "${RELEASE_TAG}" ]; then echo "RELEASE_TAG is not set"; exit 1; fi
	@if ! [ -z "$$(git status --porcelain)" ]; then echo "Your local git repository contains uncommitted changes, use git clean before proceeding."; exit 1; fi
	git checkout "${RELEASE_TAG}"
	# Build binaries prior to marking the git tree as dirty
	$(MAKE) release-binaries
	# Set the manifest image to the production bucket.
	$(MAKE) set-manifest-image \
		MANIFEST_IMG=$(PROD_REGISTRY)/$(IMAGE_NAME) MANIFEST_TAG=$(RELEASE_TAG) \
		TARGET_RESOURCE="./config/manager/manager_image_patch.yaml"
	# Set manifest image for EKS bootstrap provider to the production bucket.
	$(MAKE) set-manifest-image \
		MANIFEST_IMG=$(PROD_REGISTRY)/$(EKS_BOOTSTRAP_IMAGE_NAME) MANIFEST_TAG=$(RELEASE_TAG) \
		TARGET_RESOURCE="./bootstrap/eks/config/manager/manager_image_patch.yaml"
	$(MAKE) set-manifest-pull-policy PULL_POLICY=IfNotPresent TARGET_RESOURCE="./config/manager/manager_pull_policy.yaml"
	$(MAKE) set-manifest-pull-policy PULL_POLICY=IfNotPresent TARGET_RESOURCE="./bootstrap/eks/config/manager/manager_pull_policy.yaml"
	$(MAKE) release-manifests

.PHONY: release-manifests
release-manifests: $(RELEASE_DIR) ## Builds the manifests to publish with a release
	$(KUSTOMIZE) build config > $(RELEASE_DIR)/infrastructure-components.yaml
	$(KUSTOMIZE) build bootstrap/eks/config > $(RELEASE_DIR)/eks-bootstrap-components.yaml

.PHONY: release-binaries
release-binaries: ## Builds the binaries to publish with a release
	RELEASE_BINARY=./cmd/clusterawsadm GOOS=linux GOARCH=amd64 $(MAKE) release-binary
	RELEASE_BINARY=./cmd/clusterawsadm GOOS=darwin GOARCH=amd64 $(MAKE) release-binary

.PHONY: release-binary
release-binary: $(RELEASE_DIR)
	docker run \
		--rm \
		-e CGO_ENABLED=0 \
		-e GOOS=$(GOOS) \
		-e GOARCH=$(GOARCH) \
		-v "$$(pwd):/workspace$(DOCKER_VOL_OPTS)" \
		-w /workspace \
		golang:$(GOLANG_VERSION) \
		go build -a -ldflags '$(LDFLAGS) -extldflags "-static"' \
		-o $(RELEASE_DIR)/$(notdir $(RELEASE_BINARY))-$(GOOS)-$(GOARCH) $(RELEASE_BINARY)

.PHONY: release-staging
release-staging: ## Builds and push container images to the staging bucket.
	REGISTRY=$(STAGING_REGISTRY) $(MAKE) docker-build-all docker-push-all release-alias-tag

RELEASE_ALIAS_TAG=$(PULL_BASE_REF)

.PHONY: release-alias-tag
release-alias-tag: # Adds the tag to the last build tag.
	gcloud container images add-tag $(CONTROLLER_IMG):$(TAG) $(CONTROLLER_IMG):$(RELEASE_ALIAS_TAG)
	gcloud container images add-tag $(EKS_BOOTSTRAP_CONTROLLER_IMG):$(TAG) $(EKS_BOOTSTRAP_CONTROLLER_IMG):$(RELEASE_ALIAS_TAG)

.PHONY: release-notes
release-notes: $(RELEASE_NOTES)
	$(RELEASE_NOTES) $(ARGS)

## --------------------------------------
## Cleanup / Verification
## --------------------------------------

.PHONY: clean
clean: ## Remove all generated files
	$(MAKE) clean-bin
	$(MAKE) clean-temporary

.PHONY: clean-bin
clean-bin: ## Remove all generated binaries
	rm -rf bin
	rm -rf hack/tools/bin

.PHONY: clean-temporary
clean-temporary: ## Remove all temporary files and folders
	rm -f minikube.kubeconfig
	rm -f kubeconfig
	rm -rf _artifacts
	rm -rf test/e2e/.artifacts/*
	rm -rf test/e2e/*.xml
	rm -rf test/e2e/capa-controller-manager
	rm -rf test/e2e/capi-controller-manager
	rm -rf test/e2e/capi-kubeadm-bootstrap-controller-manager
	rm -rf test/e2e/capi-kubeadm-control-plane-controller-manager
	rm -rf test/e2e/logs
	rm -rf test/e2e/resources

.PHONY: clean-release
clean-release: ## Remove the release folder
	rm -rf $(RELEASE_DIR)

.PHONY: verify
verify: verify-boilerplate verify-modules verify-gen

.PHONY: verify-boilerplate
verify-boilerplate:
	./hack/verify-boilerplate.sh

.PHONY: verify-modules
verify-modules: modules
	@if !(git diff --quiet HEAD -- go.sum go.mod hack/tools/go.mod hack/tools/go.sum); then \
		git diff; \
		echo "go module files are out of date"; exit 1; \
	fi

verify-gen: generate
	@if !(git diff --quiet HEAD); then \
		git diff; \
		echo "generated files are out of date, run make generate"; exit 1; \
	fi

.PHONY: docs
docs: ## Build all documents and diagrams
	$(MAKE) -C docs docs
