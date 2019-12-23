.PHONY: all docs docs-serve

TAG_NAME    := $(shell git tag -l --contains HEAD)
SHA         := $(shell git rev-parse HEAD)
VERSION_GIT := $(if $(TAG_NAME),$(TAG_NAME),$(SHA))
VERSION     := $(if $(VERSION),$(VERSION),$(VERSION_GIT))
BIN_DIR     := dist

GIT_BRANCH          := $(subst heads/,,$(shell git rev-parse --abbrev-ref HEAD 2>/dev/null))
TRAEFIK_DEV_VERSION := $(if $(GIT_BRANCH),$(subst /,-,$(GIT_BRANCH)))
TRAEFIK_DEV_IMAGE   := traefik-dev$(if $(GIT_BRANCH),:$(TRAEFIK_DEV_VERSION))

REPONAME      := $(shell echo $(REPO) | tr '[:upper:]' '[:lower:]')
TRAEFIK_IMAGE := $(if $(REPONAME),$(REPONAME),"containous/traefik")

INTEGRATION_OPTS  := $(if $(MAKE_DOCKER_HOST),-e "DOCKER_HOST=$(MAKE_DOCKER_HOST)", -e "TEST_CONTAINER=1" -v "/var/run/docker.sock:/var/run/docker.sock")

DOCKER_BUILD_ARGS := $(if $(DOCKER_VERSION), "--build-arg=DOCKER_VERSION=$(DOCKER_VERSION)",)
DOCKER_BUILD_ARGS += --build-arg="TRAEFIK_IMAGE_VERSION=$(TRAEFIK_DEV_VERSION)"

BACKEND_BUILD_MARKER  := $(BIN_DIR)/traefik
BACKEND_SRC_FILES     := $(shell git ls-files '*.go' | grep -v '^vendor/')
WEBUI_BUILD_MARKER    := static/index.html
WEBUI_SRC_FILES       := $(shell git ls-files webui/)
GENERATE_BUILD_MARKER := autogen/genstatic/gen.go
GENERATE_SRC_FILES    := $(shell test -e static && find static -type f)

DOCKER_ENV_VARS := -e TESTFLAGS -e VERBOSE -e VERSION -e CODENAME -e TESTDIRS
DOCKER_ENV_VARS += -e CI -e CONTAINER=DOCKER # Indicator for integration tests that we are running inside a container.

TRAEFIK_DIST_MOUNT        := -v "$(CURDIR)/$(BIN_DIR):/go/src/github.com/containous/traefik/$(BIN_DIR)"
DOCKER_NO_CACHE           := $(if $(DOCKER_NO_CACHE),--no-cache)

CROSSBUILD_LINUX_PLATFORMS        ?= linux/386 linux/amd64 linux/arm64
CROSSBUILD_WINDOWS_PLATFORMS      ?= windows/386 windows/amd64
CROSSBUILD_DARWIN_PLATFORMS       ?= darwin/amd64
CROSSBUILD_LINUX_TARGET_PATTERN   := dist/traefik_linux-%
CROSSBUILD_WINDOWS_TARGET_PATTERN := dist/traefik_windows-%.exe
CROSSBUILD_DARWIN_TARGET_PATTERN  := dist/traefik_darwin-%
CROSSBUILD_TARGETS                := $(patsubst linux/%,   $(CROSSBUILD_LINUX_TARGET_PATTERN),   $(CROSSBUILD_LINUX_PLATFORMS))
CROSSBUILD_TARGETS                += $(patsubst windows/%, $(CROSSBUILD_WINDOWS_TARGET_PATTERN), $(CROSSBUILD_WINDOWS_PLATFORMS))
CROSSBUILD_TARGETS                += $(patsubst darwin/%,  $(CROSSBUILD_DARWIN_TARGET_PATTERN),  $(CROSSBUILD_DARWIN_PLATFORMS))

all: build

# -- build ---------------------------------------------------------------------

.PHONY: build-frontend build-generate build-backend build

build-frontend: $(WEBUI_BUILD_MARKER)

$(WEBUI_BUILD_MARKER): $(WEBUI_SRC_FILES)
	@echo "== build-frontend =================================================="
	@echo "---> npm install"
	@npm install --prefix=webui
	@echo "---> npm build"
	@npm run  build:nc --prefix=webui
	@cp -R webui/dist/pwa/* static/
	@echo 'For more informations show `webui/readme.md`' > $$PWD/static/DONT-EDIT-FILES-IN-THIS-DIRECTORY.md

# build-generate depends on build-frontend via $(WEBUI_BUILD_MARKER)
build-generate: $(WEBUI_BUILD_MARKER) $(GENERATE_BUILD_MARKER)

$(GENERATE_BUILD_MARKER): $(GENERATE_SRC_FILES)
	@echo "== build-generate =================================================="
	@./script/generate

# build-backend depends on build-webui & buid-generate via $(WEBUI_BUILD_MARKER) & $(GENERATE_BUILD_MARKER)
build-backend: $(WEBUI_BUILD_MARKER) $(GENERATE_BUILD_MARKER) $(BACKEND_BUILD_MARKER)

$(BACKEND_BUILD_MARKER): $(BACKEND_SRC_FILES) $(GENERATE_BUILD_MARKER)
	@echo "== build-backend ==================================================="
	@./script/binary

# main build target
build: build-backend

# -- crossbuild ---------------------------------------------------------------------

.PHONY: crossbuild

crossbuild: $(CROSSBUILD_TARGETS)

$(CROSSBUILD_LINUX_TARGET_PATTERN): | build-generate
	@echo "---> Cross-building linux/$*"
	@OS=linux ARCH=$* ./script/binary

$(CROSSBUILD_WINDOWS_TARGET_PATTERN): | build-generate
	@echo "---> Cross-building windows/$*"
	@OS=windows ARCH=$* ./script/binary

$(CROSSBUILD_DARWIN_TARGET_PATTERN): | build-generate
	@echo "---> Cross-building darwin/$*"
	@OS=darwin ARCH=$* ./script/binary

# -- docker --------------------------------------------------------------------

.PHONY: docker-build-frontend docker-build-backend docker-build-test docker-build docker-crossbuild

docker-build-frontend:
	@echo "== docker-build-frontend ==========================================="
	@docker build $(DOCKER_NO_CACHE) $(DOCKER_BUILD_ARGS) -t "traefik-frontend:$(TRAEFIK_DEV_VERSION)" -f traefik-frontend.Dockerfile .

docker-build-backend: docker-build-frontend
	@echo "== docker-build-backend ============================================"
	@docker build $(DOCKER_NO_CACHE) $(DOCKER_BUILD_ARGS) -t "traefik-backend:$(TRAEFIK_DEV_VERSION)" -f traefik-backend.Dockerfile .

docker-build-test: docker-build-backend
	@echo "== docker-build-test ==============================================="
	@docker build $(DOCKER_NO_CACHE) $(DOCKER_BUILD_ARGS) -t "traefik-test:$(TRAEFIK_DEV_VERSION)" -f traefik-test.Dockerfile .

docker-build: docker-build-backend
	@echo "== docker-build ===================================================="
	@docker build $(DOCKER_NO_CACHE) $(DOCKER_BUILD_ARGS) -t "traefik:$(TRAEFIK_DEV_VERSION)" -f traefik.Dockerfile .

docker-crossbuild:
	@echo "== docker-crossbuild ==============================================="
	@docker run -it $(TRAEFIK_DIST_MOUNT) $(DOCKER_ENV_VARS) "traefik-backend:$(TRAEFIK_DEV_VERSION)" make crossbuild

# -- tests ---------------------------------------------------------------------

.PHONY: test test-unit test-integration

test: build
	@echo "== test ============================================================"
	./script/make.sh test-unit test-integration

test-unit: 
	@echo "== test-unit ======================================================="
	./script/make.sh test-unit

test-integration: docker-build-test
	@echo "== test-integration ================================================"
	CI=1 TEST_CONTAINER=1 docker run -it $(DOCKER_ENV_VARS) $(INTEGRATION_OPTS) traefik-test:$(TRAEFIK_DEV_VERSION) ./script/make.sh test-integration
	CI=1 TEST_HOST=1 ./script/make.sh test-integration

# -- validation ----------------------------------------------------------------

.PHONY: validate-dependencies validate-files validate-vendor validate-shell-script validate

validate-dependencies:
	@which misspell >/dev/null 2>&1      || (echo >&2 "Command misspell not found." && exit 1)
	@which golangci-lint >/dev/null 2>&1 || (echo >&2 "Command golangci-lint not found." && exit 1)
	@which shellcheck >/dev/null 2>&1    || (echo >&2 "Command shellcheck not found." && exit 1)

validate-files: build-generate | validate-dependencies
	@echo "== validate-files =================================================="
	@./script/make.sh validate-lint validate-misspell

validate-vendor: build-generate | validate-dependencies
	@echo "== validate-vendor ================================================="
	@./script/validate-vendor

validate-shell-script: build-generate | validate-dependencies
	@echo "== validate-shell-script ==========================================="
	@./script/validate-shell-script.sh

# Validate code, docs, and vendor
validate: build-generate validate-files validate-vendor validate-shell-script | validate-dependencies

# -- docs ----------------------------------------------------------------------

.PHONY: docs docs-serve

docs:
	make -C ./docs docs

docs-serve:
	make -C ./docs docs-serve

# -- misc ----------------------------------------------------------------------

.PHONY: shell pull-images generate-crd release-packages fmt run-dev

dist:
	mkdir dist

shell:
	@echo "== shell ==========================================================="
	@docker run -it $(TRAEFIK_DIST_MOUNT) $(DOCKER_ENV_VARS) "traefik-backend:$(TRAEFIK_DEV_VERSION)" /bin/bash

# Pull all images for integration tests
pull-images:
	@echo "== pull-images ====================================================="
	@grep --no-filename -E '^\s+image:' ./integration/resources/compose/*.yml | awk '{print $$2}' | sort | uniq | xargs -P 6 -n 1 docker pull

# Generate CRD clientset
generate-crd:
	@./script/update-generated-crd-code.sh

# Create packages for the release
release-packages: docker-build-backend
	@rm -rf dist
	@docker run -i "traefik-backend:$(TRAEFIK_DEV_VERSION)" goreleaser release --skip-publish --timeout="60m"
	@docker run -i "traefik-backend:$(TRAEFIK_DEV_VERSION)" tar cfz dist/traefik-${VERSION}.src.tar.gz \
		--exclude-vcs \
		--exclude .idea \
		--exclude .travis \
		--exclude .semaphoreci \
		--exclude .github \
		--exclude dist .
	@docker run -i "traefik-backend:$(TRAEFIK_DEV_VERSION)" chown -R $(shell id -u):$(shell id -g) dist/

# Format the Code
fmt:
	gofmt -s -l -w $(BACKEND_SRC_FILES)

run-dev: build-generate
	GO111MODULE=on go build ./cmd/traefik
	./traefik
