# -- WEBUI ---------------------------------------------------------------------

FROM --platform=$BUILDPLATFORM node:14.19 as webui

WORKDIR /src/webui/

COPY ./webui/package.json ./
COPY ./webui/yarn.lock ./

RUN yarn install

COPY ./webui/ ./

RUN yarn build

# -- GO BUILD ------------------------------------------------------------------

FROM --platform=$BUILDPLATFORM golang:1.20-alpine as gobuild

WORKDIR /go/src/github.com/traefik/traefik

COPY go.mod .
COPY go.sum .

RUN go mod download

RUN apk --update upgrade \
    && apk --no-cache --no-progress add git mercurial bash gcc musl-dev curl tar ca-certificates tzdata libcap \
    && update-ca-certificates

RUN mkdir -p /usr/local/bin \
    && curl -fsSL -o /tmp/go-bindata.tgz "https://github.com/containous/go-bindata/archive/refs/tags/v1.0.0.tar.gz" \
    && cd /tmp && tar xvzf go-bindata.tgz && cd go-bindata-1.0.0 && go mod init github.com/containous/go-bindata && go get ./... && go mod download && go install ./...

COPY . .

RUN rm -rf static/

COPY --from=webui /src/webui/static/ ./webui/static/

RUN ./script/make.sh generate

ARG TARGETPLATFORM
ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT

SHELL ["bash", "-c"]

RUN VERSION="$(git describe --tags --always)" OUTPUT="dist/$TARGETPLATFORM/traefik" GOOS=${TARGETOS} GOARCH=${TARGETARCH} GOARM=${TARGETVARIANT/v/} ./script/make.sh binary
RUN setcap cap_net_bind_service=+ep "dist/$TARGETPLATFORM/traefik"

# -- scratch -------------------------------------------------------------------

FROM scratch

ARG TARGETPLATFORM

COPY --from=gobuild /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=gobuild /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=gobuild /etc/passwd /etc/passwd
COPY --from=gobuild /etc/group /etc/group
COPY --from=gobuild /etc/services /etc/services

COPY --from=gobuild /go/src/github.com/traefik/traefik/dist/$TARGETPLATFORM/traefik /

EXPOSE 80
VOLUME ["/tmp"]

ENTRYPOINT ["/traefik"]
