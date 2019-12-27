# Building and Testing

Compile and Test Your Own Traefik!
{: .subtitle }

So you want to build your own Traefik binary from the sources?
Let's see how.

## Building

You need either [Docker](https://github.com/docker/docker) and `make` (Method 1), or `go` and `make` (Method 2) in order to build Traefik.

### Method 1: Building in docker

This method allows you to build traefik inside docker. It does not require anything else than docker.

```bash
$ make docker-build CROSSBUILD_PLATFORMS="linux/386 linux/amd64"
...
$ ls dist/
traefik*
```

### Method 2: Build on your machine

Requirements:

- `make`
- `go` v1.13+`
- [go-bindata](https://github.com/containous/go-bindata) `GO111MODULE=off go get -u github.com/containous/go-bindata/...`

!!! tip "Source Directory"

    It is recommended that you clone Traefik into the `~/go/src/github.com/containous/traefik` directory.
    This is the official golang workspace hierarchy that will allow dependencies to be properly resolved.

!!! note "Environment"

    Set your `GOPATH` and `PATH` variable to be set to `~/go` via:

    ```bash
    export GOPATH=~/go
    export PATH=$PATH:$GOPATH/bin
    ```

    For convenience, add `GOPATH` and `PATH` to your `.bashrc` or `.bash_profile`

    Verify your environment is setup properly by running `$ go env`.
    Depending on your OS and environment, you should see an output similar to:

    ```bash
    GOARCH="amd64"
    GOBIN=""
    GOEXE=""
    GOHOSTARCH="amd64"
    GOHOSTOS="linux"
    GOOS="linux"
    GOPATH="/home/<yourusername>/go"
    GORACE=""
    ## ... and the list goes on
    ```

#### Build Traefik

Once you've set up your go environment and cloned the source repository, you can build Traefik.

Beforehand, you need to get [go-bindata](https://github.com/containous/go-bindata) (the first time) in order to be able to use the `go generate` command (which is part of the build process).

```bash
cd ~/go/src/github.com/containous/traefik

# Get go-bindata. (Important: the ellipses are required.)
GO111MODULE=off go get github.com/containous/go-bindata/...
```

```bash
# Generate UI static files
make build-frontend

# Transform static files into go sources
make build-generate

# Build traefik
make build-backend
```

```bash
# Standard go build
go build ./cmd/traefik
```

You will find the Traefik executable (`traefik`) in the `~/go/src/github.com/containous/traefik/dist` directory.

## Testing

### Method 1: `Docker` and `make`

Run unit tests using the `test-unit` target.
Run integration tests using the `test-integration` target.
Run all tests (unit and integration) using the `test` target.

```bash
$ make test-unit
docker build -t "traefik-dev:your-feature-branch" -f build.Dockerfile .
# […]
docker run --rm -it -e OS_ARCH_ARG -e OS_PLATFORM_ARG -e TESTFLAGS -v "/home/user/go/src/github/containous/traefik/dist:/go/src/github.com/containous/traefik/dist" "traefik-dev:your-feature-branch" ./script/make.sh generate test-unit
---> Making bundle: generate (in .)
removed 'gen.go'

---> Making bundle: test-unit (in .)
+ go test -cover -coverprofile=cover.out .
ok      github.com/containous/traefik   0.005s  coverage: 4.1% of statements

Test success
```

For development purposes, you can specify which tests to run by using (only works the `test-integration` target):

```bash
# Run every tests in the MyTest suite
TESTFLAGS="-check.f MyTestSuite" make test-integration

# Run the test "MyTest" in the MyTest suite
TESTFLAGS="-check.f MyTestSuite.MyTest" make test-integration

# Run every tests starting with "My", in the MyTest suite
TESTFLAGS="-check.f MyTestSuite.My" make test-integration

# Run every tests ending with "Test", in the MyTest suite
TESTFLAGS="-check.f MyTestSuite.*Test" make test-integration
```

More: https://labix.org/gocheck

### Method 2: `go`

Unit tests can be run from the cloned directory using `$ go test ./...` which should return `ok`, similar to:

```test
ok      _/home/user/go/src/github/containous/traefik    0.004s
```

Integration tests must be run from the `integration/` directory and require the `-integration` switch: `$ cd integration && go test -integration ./...`.
