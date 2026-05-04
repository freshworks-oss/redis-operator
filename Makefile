VERSION ?= v1.3.0-rc0

# Name of this service/application
SERVICE_NAME := redis-operator

# Docker image name for this project
IMAGE_NAME := freshworks/$(SERVICE_NAME)

# Repository url for this project
REPOSITORY := ghcr.io/$(IMAGE_NAME)

# Shell to use for running scripts
SHELL := $(shell which bash)

# Local builds use Podman when available, otherwise Docker. Override explicitly,
# e.g. `make CONTAINER_ENGINE=docker image` when both are installed.
CONTAINER_ENGINE ?= $(shell command -v podman 2>/dev/null || command -v docker 2>/dev/null)
# Non-empty when CONTAINER_ENGINE points at Podman (path may be .../bin/podman).
IS_PODMAN := $(findstring podman,$(CONTAINER_ENGINE))

# Get the main unix group for the user running make (to be used by docker-compose later)
GID := $(shell id -g)

# Get the unix user id for the user running make (to be used by docker-compose later)
UID := $(shell id -u)

# Commit hash from git
COMMIT=$(shell git rev-parse HEAD)
GITTAG_COMMIT := $(shell git rev-list --tags --max-count=1)
GITTAG := $(shell git describe --abbrev=0 --tags ${GITTAG_COMMIT} 2>/dev/null || true)

# Branch from git
BRANCH=$(shell git rev-parse --abbrev-ref HEAD)

TAG := $(GITTAG)
ifneq ($(COMMIT), $(GITTAG_COMMIT))
    TAG := $(COMMIT)
endif

ifneq ($(shell git status --porcelain),)
    TAG := $(TAG)-dirty
endif


PROJECT_PACKAGE := github.com/freshworks/redis-operator
CODEGEN_IMAGE := ghcr.io/slok/kube-code-generator:v0.7.0
PORT := 9710

# CMDs
UNIT_TEST_CMD := go test `go list ./... | grep -v /vendor/` -v
GO_GENERATE_CMD := go generate `go list ./... | grep -v /vendor/`
GO_INTEGRATION_TEST_CMD := go test `go list ./... | grep test/integration` -v -tags='integration' -timeout=20m
GET_DEPS_CMD := dep ensure
LINT_CMD := golangci-lint run --timeout=15m
LINT_NEW_CMD := golangci-lint run --timeout=15m --new-from-rev=HEAD~1
UPDATE_DEPS_CMD := dep ensure
MOCKS_CMD := go generate ./mocks

# environment dirs
DEV_DIR := docker/development
APP_DIR := docker/app

# workdir
WORKDIR := /go/src/github.com/freshworks/redis-operator

# The default action of this Makefile is to build the development docker image
.PHONY: default
default: build

# Run the development environment in non-daemonized mode (foreground)
.PHONY: docker-build
docker-build: deps-development
	$(CONTAINER_ENGINE) build \
		--build-arg uid=$(UID) \
		-t $(REPOSITORY)-dev:latest \
		-t $(REPOSITORY)-dev:$(COMMIT) \
		-f $(DEV_DIR)/Dockerfile \
		.

# Run a shell into the development docker image
.PHONY: shell
shell: docker-build
	$(CONTAINER_ENGINE) run -ti --rm -v ~/.kube:/.kube:ro -v $(PWD):$(WORKDIR) -u $(UID):$(UID) --name $(SERVICE_NAME) -p $(PORT):$(PORT) $(REPOSITORY)-dev /bin/bash

# Build redis-failover executable file
.PHONY: build
build: docker-build
	$(CONTAINER_ENGINE) run -ti --rm -v $(PWD):$(WORKDIR) -u $(UID):$(UID) --name $(SERVICE_NAME) $(REPOSITORY)-dev ./scripts/build.sh

# Run the development environment in the background
.PHONY: run
run: docker-build
	$(CONTAINER_ENGINE) run -ti --rm -v ~/.kube:/.kube:ro -v $(PWD):$(WORKDIR) -u $(UID):$(UID) --name $(SERVICE_NAME) -p $(PORT):$(PORT) $(REPOSITORY)-dev ./scripts/run.sh

# Build the production image based on the public one
.PHONY: image
image: deps-development
	$(CONTAINER_ENGINE) build \
	-t $(SERVICE_NAME) \
	-t $(REPOSITORY):latest \
	-t $(REPOSITORY):$(COMMIT) \
	-t $(REPOSITORY):$(BRANCH) \
	-f $(APP_DIR)/Dockerfile \
	.

# Multi-arch registry push uses Docker Buildx. Podman does not support the same
# `buildx build --push` flow; with Podman we build for the current architecture and
# push tags with `podman push` (same as `docker push`).
.PHONY: image-release
ifneq ($(IS_PODMAN),)
image-release:
	@echo ">> image-release: using Podman (single-arch build + podman push). For multi-arch use Docker: make CONTAINER_ENGINE=docker image-release"
	$(CONTAINER_ENGINE) build \
	--build-arg VERSION=$(TAG) \
	-t $(REPOSITORY):latest \
	-t $(REPOSITORY):$(COMMIT) \
	-t $(REPOSITORY):$(TAG) \
	-f $(APP_DIR)/Dockerfile \
	.
	$(CONTAINER_ENGINE) push $(REPOSITORY):latest
	$(CONTAINER_ENGINE) push $(REPOSITORY):$(COMMIT)
	$(CONTAINER_ENGINE) push $(REPOSITORY):$(TAG)
else
image-release:
	$(CONTAINER_ENGINE) buildx build \
	--platform linux/amd64,linux/arm64,linux/arm/v7 \
	--push \
	--build-arg VERSION=$(TAG) \
	-t $(REPOSITORY):latest \
	-t $(REPOSITORY):$(COMMIT) \
	-t $(REPOSITORY):$(TAG) \
	-f $(APP_DIR)/Dockerfile \
	.
endif

.PHONY: testing
testing: image
	$(CONTAINER_ENGINE) push $(REPOSITORY):$(BRANCH)

.PHONY: tag
tag:
	git tag $(VERSION)

.PHONY: publish
publish:
	@COMMIT_VERSION="$$(git rev-list -n 1 $(VERSION))"; \
	$(CONTAINER_ENGINE) tag $(REPOSITORY):"$$COMMIT_VERSION" $(REPOSITORY):$(VERSION)
	$(CONTAINER_ENGINE) push $(REPOSITORY):$(VERSION)
	$(CONTAINER_ENGINE) push $(REPOSITORY):latest

.PHONY: release
release: tag image-release

# Test stuff in dev
.PHONY: unit-test
unit-test: docker-build
	$(CONTAINER_ENGINE) run -ti --rm -v $(PWD):$(WORKDIR) -u $(UID):$(UID) --name $(SERVICE_NAME) $(REPOSITORY)-dev /bin/sh -c '$(UNIT_TEST_CMD)'

.PHONY: ci-unit-test
ci-unit-test:
	$(UNIT_TEST_CMD)

.PHONY: ci-govulncheck
ci-govulncheck:
	go run golang.org/x/vuln/cmd/govulncheck@latest ./...

.PHONY: ci-integration-test
ci-integration-test:
	$(GO_INTEGRATION_TEST_CMD)

.PHONY: integration-test
integration-test:
	./scripts/integration-tests.sh

.PHONY: helm-test
helm-test:
	./scripts/helm-tests.sh

# Run all tests
.PHONY: test
test: ci-lint ci-unit-test ci-integration-test helm-test

.PHONY: lint
lint: docker-build
	$(CONTAINER_ENGINE) run -ti --rm -v $(PWD):$(WORKDIR) -u $(UID):$(UID) --name $(SERVICE_NAME) $(REPOSITORY)-dev /bin/sh -c '$(LINT_CMD)'

.PHONY: new-lint
new-lint: docker-build
	$(CONTAINER_ENGINE) run -ti --rm -v $(PWD):$(WORKDIR) -u $(UID):$(UID) --name $(SERVICE_NAME) $(REPOSITORY)-dev /bin/sh -c '$(LINT_NEW_CMD)'

.PHONY: ci-lint
ci-lint:
	$(LINT_CMD)

.PHONY: ci-new-lint
ci-new-lint:
	$(LINT_NEW_CMD)

.PHONY: go-generate
go-generate: docker-build
	$(CONTAINER_ENGINE) run -ti --rm -v $(PWD):$(WORKDIR) -u $(UID):$(UID) --name $(SERVICE_NAME) $(REPOSITORY)-dev /bin/sh -c '$(GO_GENERATE_CMD)'

.PHONY: generate
generate: go-generate

.PHONY: get-deps
get-deps: docker-build
	$(CONTAINER_ENGINE) run -ti --rm -v $(PWD):$(WORKDIR) -u $(UID):$(UID) --name $(SERVICE_NAME) $(REPOSITORY)-dev /bin/sh -c '$(GET_DEPS_CMD)'

.PHONY: update-deps
update-deps: docker-build
	$(CONTAINER_ENGINE) run -ti --rm -v $(PWD):$(WORKDIR) -u $(UID):$(UID) --name $(SERVICE_NAME) $(REPOSITORY)-dev /bin/sh -c '$(UPDATE_DEPS_CMD)'

.PHONY: mocks
mocks: docker-build
	$(CONTAINER_ENGINE) run -ti --rm -v $(PWD):$(WORKDIR) -u $(UID):$(UID) --name $(SERVICE_NAME) $(REPOSITORY)-dev /bin/sh -c '$(MOCKS_CMD)'

.PHONY: deps-development
# Test if the dependencies we need to run this Makefile are installed
deps-development:
ifeq ($(CONTAINER_ENGINE),)
	@echo "Neither podman nor docker found in PATH. Install one or set CONTAINER_ENGINE."
	@exit 1
endif

# Generate kubernetes code for types..
.PHONY: update-codegen
update-codegen:
	@echo ">> Generating code for Kubernetes CRD types..."
	$(CONTAINER_ENGINE) run --rm -it \
	-v $(PWD):/app \
	-e KUBE_CODE_GENERATOR_GO_GEN_OUT=./client/k8s \
	-e KUBE_CODE_GENERATOR_APIS_IN=./api \
	-e GROUPS_VERSION="redisfailover:v1" \
	-e GENERATION_TARGETS="deepcopy,client" \
	$(CODEGEN_IMAGE)

generate-crd:
	@echo ">> Generating CRD..."
	$(CONTAINER_ENGINE) run --rm -it \
	-v $(PWD):/app \
	-e KUBE_CODE_GENERATOR_APIS_IN=./api \
	-e KUBE_CODE_GENERATOR_CRD_GEN_OUT=./manifests \
	-e GROUPS_VERSION="redisfailover:v1" \
	$(CODEGEN_IMAGE)
	cp -f manifests/databases.spotahome.com_redisfailovers.yaml manifests/kustomize/base
