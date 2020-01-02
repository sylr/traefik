# See https://github.com/golang/go/issues/14481
FROM golang:1.13-alpine AS race

WORKDIR /tmp/race

RUN apk --update -q --progress --no-cache add git g++
RUN git clone --single-branch https://llvm.org/git/compiler-rt.git . &> /dev/null
RUN git reset --hard fe2c72c59aa7f4afa45e3f65a5d16a374b6cce26 && \
    wget -q https://github.com/golang/go/files/3615484/0001-hack-to-make-Go-s-race-flag-work-on-Alpine.patch.gz -O patch.gz && \
    gunzip patch.gz && \
    patch -p1 -i patch
RUN cd lib/tsan/go && \
    ./buildgo.sh &> /dev/null


FROM golang:1.13-alpine

# Patch for go test -race on Alpine
COPY --from=race /tmp/race/lib/tsan/go/race_linux_amd64.syso /usr/local/go/src/runtime/race/race_linux_amd64.syso

RUN apk --update upgrade \
    && apk --no-cache --no-progress add git mercurial bash gcc musl-dev curl tar ca-certificates tzdata \
    && update-ca-certificates \
    && rm -rf /var/cache/apk/*

# Which docker version to test on
ARG DOCKER_VERSION=18.09.7

# Download docker
RUN mkdir -p /usr/local/bin \
    && curl -fL https://download.docker.com/linux/static/stable/x86_64/docker-${DOCKER_VERSION}.tgz \
    | tar -xzC /usr/local/bin --transform 's#^.+/##x'

# Download go-bindata binary to bin folder in $GOPATH
RUN mkdir -p /usr/local/bin \
    && curl -fsSL -o /usr/local/bin/go-bindata https://github.com/containous/go-bindata/releases/download/v1.0.0/go-bindata \
    && chmod +x /usr/local/bin/go-bindata

# Download golangci-lint binary to bin folder in $GOPATH
RUN curl -sfL https://install.goreleaser.com/github.com/golangci/golangci-lint.sh | bash -s -- -b $GOPATH/bin v1.20.0

# Download golangci-lint and misspell binary to bin folder in $GOPATH
RUN GO111MODULE=off go get github.com/client9/misspell/cmd/misspell

# Download goreleaser binary to bin folder in $GOPATH
RUN curl -sfL https://install.goreleaser.com/github.com/goreleaser/goreleaser.sh | sh

WORKDIR /go/src/github.com/containous/traefik

# Download go modules
COPY go.mod .
COPY go.sum .
RUN GO111MODULE=on GOPROXY=https://proxy.golang.org go mod download

COPY . /go/src/github.com/containous/traefik
