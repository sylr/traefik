# -- WEBUI ---------------------------------------------------------------------

FROM --platform=$BUILDPLATFORM node:12.22 as webui

ARG ARG_PLATFORM_URL=https://pilot.traefik.io
ENV PLATFORM_URL=${ARG_PLATFORM_URL}

WORKDIR /src/webui/

COPY ./webui/ /src/webui/

RUN npm install
RUN npm run build

# -- GO BUILD ------------------------------------------------------------------

FROM --platform=$BUILDPLATFORM golang:1.16-alpine as gobuild

WORKDIR /go/src/github.com/traefik/traefik

COPY go.mod .
COPY go.sum .

RUN go mod download

RUN apk --update upgrade \
    && apk --no-cache --no-progress add git mercurial bash gcc musl-dev curl tar ca-certificates tzdata libcap \
    && update-ca-certificates

RUN mkdir -p /usr/local/bin \
    && curl -fsSL -o /usr/local/bin/go-bindata https://github.com/containous/go-bindata/releases/download/v1.0.0/go-bindata \
    && chmod +x /usr/local/bin/go-bindata

COPY . .

RUN rm -rf static/

COPY --from=webui /src/static/ static/

RUN ./script/make.sh generate

ARG TARGETPLATFORM
ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT

SHELL ["bash", "-c"]

RUN OUTPUT="dist/$TARGETPLATFORM/traefik" GOOS=${TARGETOS} GOARCH=${TARGETARCH} GOARM=${TARGETVARIANT/v/} ./script/make.sh binary
RUN setcap cap_net_bind_service=+ep "dist/$TARGETPLATFORM/traefik"

# -- scratch -------------------------------------------------------------------

FROM scratch

ARG TARGETPLATFORM

COPY script/ca-certificates.crt /etc/ssl/certs/
COPY --from=gobuild /go/src/github.com/traefik/traefik/dist/$TARGETPLATFORM/traefik /

EXPOSE 80
VOLUME ["/tmp"]

ENTRYPOINT ["/traefik"]
