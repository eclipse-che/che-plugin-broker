GOENV := CGO_ENABLED=0 GOOS=linux
GOFLAGS := -a -ldflags '-w -s' -a -installsuffix cgo
PLUGIN_REGISTRY_URL ?= "https://che-plugin-registry.openshift.io/v3"

all: ci build
.PHONY: all

.PHONY: ci
ci:
	docker build -f build/CI/Dockerfile .

.PHONY: build
build:
	$(GOENV) go build $(GOFLAGS) ./...

.PHONY: build-init
build-init:
	$(GOENV) go build $(GOFLAGS) -o init-plugin-broker brokers/init/cmd/main.go

.PHONY: build-unified
build-unified:
	$(GOENV) go build $(GOFLAGS) -o unified-plugin-broker brokers/unified/cmd/main.go

.PHONY: test
test:
	go test -v -race ./...

.PHONY: lint
lint:
	golangci-lint run -v

.PHONY: fmt
fmt:
	go fmt ./...

.PHONY: dep-update
dep-update:
	dep ensure

.PHONY: build-docker-init
build-docker-init:
	docker build -t eclipse/che-init-plugin-broker:latest -f build/init/Dockerfile .

.PHONY: build-docker-unified
build-docker-unified:
	docker build -t eclipse/che-unified-plugin-broker:latest -f build/unified/Dockerfile .

.PHONY: test-local
test-local:
	cd ./brokers/unified/cmd; \
		go build main.go; \
		./main \
			--disable-push \
			--runtime-id wsId:env:ownerId \
			--registry-address ${PLUGIN_REGISTRY_URL} \
			--metas ./config-plugin-ids.json

.PHONY: test-metadata
test-metadata:
	cd ./brokers/metadata/cmd; \
		go build main.go; \
		./main \
			--disable-push \
			--runtime-id wsId:env:ownerId \
			--registry-address ${PLUGIN_REGISTRY_URL} \
			--metas ../../unified/cmd/config-plugin-ids.json

.PHONY: test-artifacts
test-artifacts:
	cd ./brokers/artifacts/cmd; \
		go build main.go; \
		./main \
			--disable-push \
			--runtime-id wsId:env:ownerId \
			--registry-address ${PLUGIN_REGISTRY_URL} \
			--metas ../../unified/cmd/config-plugin-ids.json
