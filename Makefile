.PHONY: all docs docs-serve

GIT_TAG         := $(shell git tag -l --contains HEAD)
GIT_SHA         := $(shell git rev-parse HEAD)
GIT_BRANCH      := $(subst heads/,,$(shell git rev-parse --abbrev-ref HEAD 2>/dev/null))
GIT_VERSION     := $(if $(GIT_TAG),$(GIT_TAG),$(GIT_SHA))
GIT_DESCRIBE    := $(shell git describe --tags --dirty)

VERSION     ?= $(GIT_DESCRIBE)
BIN_DIR     := dist
REPONAME    := $(shell echo $(REPO) | tr '[:upper:]' '[:lower:]')

BACKEND_BUILD_MARKER    := $(BIN_DIR)/traefik
BACKEND_SRC_FILES       := $(shell git ls-files '*.go' | grep -v '^vendor/')
WEBUI_BUILD_MARKER      := static/index.html
WEBUI_SRC_FILES         := $(shell git ls-files webui/)
GENERATE_BUILD_MARKER   := autogen/genstatic/gen.go
GENERATE_SRC_FILES      := $(shell test -e static && find static -type f)

CROSSBUILD_PLATFORMS				?= linux/386 linux/amd64 linux/arm64 windows/386 windows/amd64 darwin/amd64
CROSSBUILD_LINUX_PLATFORMS          := $(filter linux/%,$(CROSSBUILD_PLATFORMS))
CROSSBUILD_FREEBSD_PLATFORMS        := $(filter freebsd/%,$(CROSSBUILD_PLATFORMS))
CROSSBUILD_OPENBSD_PLATFORMS        := $(filter openbsd/%,$(CROSSBUILD_PLATFORMS))
CROSSBUILD_WINDOWS_PLATFORMS        := $(filter windows/%,$(CROSSBUILD_PLATFORMS))
CROSSBUILD_DARWIN_PLATFORMS         := $(filter darwin/%,$(CROSSBUILD_PLATFORMS))
CROSSBUILD_LINUX_TARGET_PATTERN     := dist/traefik_linux-%
CROSSBUILD_FREEBSD_TARGET_PATTERN   := dist/traefik_freebsd-%
CROSSBUILD_OPENBSD_TARGET_PATTERN   := dist/traefik_openbsd-%
CROSSBUILD_WINDOWS_TARGET_PATTERN   := dist/traefik_windows-%.exe
CROSSBUILD_DARWIN_TARGET_PATTERN    := dist/traefik_darwin-%
CROSSBUILD_TARGETS                  := $(patsubst linux/%,$(CROSSBUILD_LINUX_TARGET_PATTERN),$(CROSSBUILD_LINUX_PLATFORMS))
CROSSBUILD_TARGETS                  += $(patsubst freebsd/%,$(CROSSBUILD_FREEBSD_TARGET_PATTERN),$(CROSSBUILD_FREEBSD_PLATFORMS))
CROSSBUILD_TARGETS                  += $(patsubst openbsd/%,$(CROSSBUILD_OPENBSD_TARGET_PATTERN),$(CROSSBUILD_OPENBSD_PLATFORMS))
CROSSBUILD_TARGETS                  += $(patsubst windows/%,$(CROSSBUILD_WINDOWS_TARGET_PATTERN),$(CROSSBUILD_WINDOWS_PLATFORMS))
CROSSBUILD_TARGETS                  += $(patsubst darwin/%,$(CROSSBUILD_DARWIN_TARGET_PATTERN),$(CROSSBUILD_DARWIN_PLATFORMS))

ifeq ($(GIT_BRANCH),master)
DOCKER_IMAGE_VERSION	:= $(GIT_DESCRIBE)
else
DOCKER_IMAGE_VERSION 	:= $(subst /,-,$(GIT_BRANCH))
endif
DOCKER_IMAGE_VERSION 	:= $(subst +,_,$(DOCKER_IMAGE_VERSION))
DOCKER_BIN_VERSION		?= 18.09.7
DOCKER_REPO         	:= $(if $(REPONAME),$(REPONAME),"containous/traefik/")
DOCKER_BUILD_ARGS   	:= --build-arg="DOCKER_VERSION=$(DOCKER_BIN_VERSION)"
DOCKER_BUILD_ARGS   	+= --build-arg="TRAEFIK_IMAGE_VERSION=$(DOCKER_IMAGE_VERSION)"
DOCKER_ENV_VARS     	:= -e TESTFLAGS -e VERBOSE -e VERSION=$(VERSION) -e CODENAME
DOCKER_ENV_VARS     	+= -e CI -e CONTAINER=DOCKER # Indicator for integration tests that we are running inside a container.
DOCKER_ENV_VARS     	+= -e "CROSSBUILD_PLATFORMS=$(CROSSBUILD_PLATFORMS)"
DOCKER_DIST_MOUNT   	:= -v "$(CURDIR)/$(BIN_DIR):/go/src/github.com/containous/traefik/$(BIN_DIR)"
DOCKER_GO_PKG_MOUNT		:= -v "$(shell go env GOPATH)/pkg:/go/pkg"
DOCKER_NO_CACHE     	:= $(if $(DOCKER_NO_CACHE),--no-cache)

INTEGRATION_OPTS := $(if $(MAKE_DOCKER_HOST),-e "DOCKER_HOST=$(MAKE_DOCKER_HOST)", -e "TEST_CONTAINER=1" -v "/var/run/docker.sock:/var/run/docker.sock")

# -- all -----------------------------------------------------------------------

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
	@echo "---> go generate"
	@./script/generate

# build-backend depends on build-webui & buid-generate via $(WEBUI_BUILD_MARKER) & $(GENERATE_BUILD_MARKER)
build-backend: $(WEBUI_BUILD_MARKER) $(GENERATE_BUILD_MARKER) $(BACKEND_BUILD_MARKER)

$(BACKEND_BUILD_MARKER): $(BACKEND_SRC_FILES) $(GENERATE_BUILD_MARKER)
	@echo "== build-backend ==================================================="
	@echo "---> go build"
	@./script/binary

# main build target
build: build-backend

# -- crossbuild ----------------------------------------------------------------

.PHONY: crossbuild

crossbuild: $(CROSSBUILD_TARGETS)

$(CROSSBUILD_LINUX_TARGET_PATTERN): | build-generate
	@echo "---> Cross-building linux/$*"
	@OS=linux ARCH=$* VERSION=$(GIT_DESCRIBE) ./script/binary

$(CROSSBUILD_FREEBSD_TARGET_PATTERN): | build-generate
	@echo "---> Cross-building freebsd/$*"
	@OS=freebsd ARCH=$* VERSION=$(GIT_DESCRIBE) ./script/binary

$(CROSSBUILD_OPENBSD_TARGET_PATTERN): | build-generate
	@echo "---> Cross-building openbsd/$*"
	@OS=openbsd ARCH=$* VERSION=$(GIT_DESCRIBE) ./script/binary

$(CROSSBUILD_WINDOWS_TARGET_PATTERN): | build-generate
	@echo "---> Cross-building windows/$*"
	@OS=windows ARCH=$* VERSION=$(GIT_DESCRIBE) ./script/binary

$(CROSSBUILD_DARWIN_TARGET_PATTERN): | build-generate
	@echo "---> Cross-building darwin/$*"
	@OS=darwin ARCH=$* VERSION=$(GIT_DESCRIBE) ./script/binary

# -- docker --------------------------------------------------------------------

.PHONY: docker-build-frontend-image docker-build-backend-image docker-build-test-image docker-build docker-crossbuild

docker-build-frontend-image:
	@echo "== docker-build-frontend-image ====================================="
	@docker build $(DOCKER_NO_CACHE) $(DOCKER_BUILD_ARGS) -t "traefik-frontend:$(DOCKER_IMAGE_VERSION)" -f traefik-frontend.Dockerfile .

docker-build-backend-image: docker-build-frontend-image
	@echo "== docker-build-backend-image ======================================"
	@docker build $(DOCKER_NO_CACHE) $(DOCKER_BUILD_ARGS) -t "traefik-backend:$(DOCKER_IMAGE_VERSION)" -f traefik-backend.Dockerfile .

docker-build-test-image: docker-build-backend-image
	@echo "== docker-build-test-image ========================================="
	@docker build $(DOCKER_NO_CACHE) $(DOCKER_BUILD_ARGS) -t "traefik-test:$(DOCKER_IMAGE_VERSION)" -f traefik-test.Dockerfile .

docker-build-image: docker-build-backend-image
	@echo "== docker-build-image =============================================="
	@docker build $(DOCKER_NO_CACHE) $(DOCKER_BUILD_ARGS) -t "traefik:$(DOCKER_IMAGE_VERSION)" -f traefik.Dockerfile .
	@docker tag "traefik:$(DOCKER_IMAGE_VERSION)" "$(DOCKER_REPO)traefik:$(DOCKER_IMAGE_VERSION)"

docker-crossbuild: docker-build-frontend-image docker-build-backend-image
	@echo "== docker-crossbuild ==============================================="
	@docker run -it $(DOCKER_DIST_MOUNT) $(DOCKER_ENV_VARS) "traefik-backend:$(DOCKER_IMAGE_VERSION)" make crossbuild

# -- tests ---------------------------------------------------------------------

.PHONY: test test-unit test-integration

test-unit: build-generate
	@echo "== test-unit ======================================================="
	@./script/make.sh test-unit

test-integration: docker-build-test-image build
	@echo "== test-integration ================================================"
	@CI=1 TEST_CONTAINER=1 docker run -it $(DOCKER_ENV_VARS) $(INTEGRATION_OPTS) "traefik-test:$(DOCKER_IMAGE_VERSION)" ./script/make.sh test-integration
	@CI=1 TEST_HOST=1 ./script/make.sh test-integration

test: test-unit test-integration

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
	@docker run -it $(TRAEFIK_DIST_MOUNT) $(DOCKER_ENV_VARS) "traefik-backend:$(DOCKER_IMAGE_VERSION)" /bin/bash

# Pull all images for integration tests
pull-images:
	@echo "== pull-images ====================================================="
	@grep --no-filename -E '^\s+image:' ./integration/resources/compose/*.yml | awk '{print $$2}' | sort | uniq | xargs -P 6 -n 1 docker pull

# Generate CRD clientset
generate-crd:
	@./script/update-generated-crd-code.sh

# Create packages for the release
release-packages: docker-build-backend-image
	@rm -rf dist
	@docker run -i "traefik-backend:$(DOCKER_IMAGE_VERSION)" goreleaser release --skip-publish --timeout="60m"
	@docker run -i "traefik-backend:$(DOCKER_IMAGE_VERSION)" tar cfz dist/traefik-$(GIT_DESCRIBE).src.tar.gz \
		--exclude-vcs \
		--exclude .idea \
		--exclude .travis \
		--exclude .semaphoreci \
		--exclude .github \
		--exclude dist .
	@docker run -i "traefik-backend:$(DOCKER_IMAGE_VERSION)" chown -R $(shell id -u):$(shell id -g) dist/

# Format the Code
fmt:
	gofmt -s -l -w $(BACKEND_SRC_FILES)

run-dev: build-generate
	GO111MODULE=on go build ./cmd/traefik
	./traefik
