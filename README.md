[![CircleCI](https://circleci.com/gh/eclipse/che-plugin-broker.svg?style=svg)](https://circleci.com/gh/eclipse/che-plugin-broker)

[![codecov](https://codecov.io/gh/eclipse/che-plugin-broker/branch/master/graph/badge.svg)](https://codecov.io/gh/eclipse/che-plugin-broker)

# This repo contains implementations of several Che plugin brokers

## init-plugin-broker

Cleanups content of /plugins/ folder.
Should be started before other brokers not to remove files they are adding to plugins folder.

## unified-plugin-broker

Which can process plugins of types:
- Che Plugin
- VS Code extension
- Che Editor
- Theia plugin

But it ignores case of plugin type, so any other variants of the same type but with different case of letters is considered the same.

What it actually does:

### All plugin/editor types

- Downloads meta.yaml of a plugin or editor from Che plugin registry 
- Evaluates Che workspace sidecars config from the above mentioned meta.yaml

### Theia plugin/VS Code extension

- Downloads .theia and/or vsix archives and
- If meta.spec contains `containers` field with a container definition extension/plugin is considered remote. Otherwise it is considered local
- Unzip it to a temp folder
- Check content of package.json file in it
- Copies plugin or extension to /plugins/ in packed or unpacked state depending on its type and whether Che plugin is local or remote
- For remote plugin case, evaluates Che workspace sidecar config for running VS Code or Theia extensions/plugins as Che Theia remote plugins in a sidecar:
  - adds an endpoint with random port between 4000 and 10000 and name `port<port>`
  - adds env var to workspace-wide env vars with name
 `THEIA_PLUGIN_REMOTE_ENDPOINT_<plugin_publisher_and_name from package.json>` and value
 `ws://port<port>:<port>`
  - adds env var to sidecar env vars with name
 `THEIA_PLUGIN_ENDPOINT_PORT` and value `port`
  - with projects volume
  - with plugin volume
- Sends sidecar config to Che workspace master

## Development

Mocks are generated from interfaces using library [mockery](https://github.com/vektra/mockery)
To add new mock implementation for an interface or regenerate to an existing one use following
command when current dir is location of the folder containing the interface:

```shell
mockery -name=NameOfAnInterfaceToMock
```

### Build

- build all the code:

```shell
CGO_ENABLED=0 GOOS=linux go build -a -ldflags '-w -s' -a -installsuffix cgo ./...
```

- build Init plugin broker binary:

```shell
CGO_ENABLED=0 GOOS=linux go build -a -ldflags '-w -s' -a -installsuffix cgo -o init-plugin-broker brokers/init/cmd/main.go
```

- build Unified plugin broker binary:

```shell
CGO_ENABLED=0 GOOS=linux go build -a -ldflags '-w -s' -a -installsuffix cgo -o unified-plugin-broker brokers/unified/cmd/main.go
```

### Run checks

- run tests:

```shell
go test -v -race ./...
```

- run linters:

```shell
golangci-lint run -v
```

- run CI checks locally in Docker (includes build/test/linters):

```shell
docker build -f build/CI/Dockerfile .
```

### Check brokers locally

Prerequisites:

    - Folder /plugins exists on the host and writable for the user

- Go to a broker cmd directory, e.g. `brokers/unified/cmd`
- Compile binaries `go build main.go`
- Run binary `./main -disable-push -runtime-id wsId:env:ownerId`
- Check JSON with sidecar configuration in the very bottom of the output
- Check that needed files are in `/plugins`
- To cleanup `/plugins` folder **init** broker can be used

### Dependencies

Dependencies in the project are managed by Go Dep.
After you added a dependency you need to run the following command to download dependencies to vendor repo and lock file and then commit changes:

```shell
dep ensure
```

`dep ensure` doesn't automatically change Gopkg.toml which contains dependencies constrants.
So, when a dependency is introduced or changed it should be reflected in Gopkg.toml.

### Build of Docker images

- build Init plugin broker

```shell
docker build -t eclipse/che-init-plugin-broker:latest -f build/init/Dockerfile .
```

- build Unified plugin broker

```shell
docker build -t eclipse/che-unified-plugin-broker:latest -f build/unified/Dockerfile .
```
