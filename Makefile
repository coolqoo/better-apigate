.PHONY: build run test clean docker docker-publish docker-build-binaries webui webui-install all

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
BUILD_DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -ldflags "-s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.buildDate=$(BUILD_DATE)"

# Default: build everything
all: webui build

# Build webui and sync to embed location
webui:
	@echo "Building webui..."
	cd webui && npm run build
	@echo "Syncing webui to embed location..."
	rm -rf core/channel/http/webui/dist
	cp -r webui/build core/channel/http/webui/dist
	@echo "Webui build complete"

# Install webui dependencies
webui-install:
	cd webui && npm ci

# Build Go binary (embeds webui assets)
build:
	CGO_ENABLED=0 go build $(LDFLAGS) -o bin/better-apigate ./cmd/better-apigate

run: build
	./bin/better-apigate serve

dev:
	go run ./cmd/better-apigate serve

test:
	go test -v ./...

clean:
	rm -rf bin/
	rm -f better-apigate
	rm -rf webui/build
	rm -rf core/channel/http/webui/dist

docker:
	docker build -t better-apigate:$(VERSION) .

docker-run:
	docker compose up -d --wait

# Build and publish a multi-architecture image to Docker Hub.
# Log in with docker login first; credentials are never passed as build arguments.
DOCKER_REPO ?= $(if $(DOCKERHUB_USERNAME),$(DOCKERHUB_USERNAME)/better-apigate)
DOCKER_TAG ?= $(patsubst v%,%,$(VERSION))
DOCKER_PLATFORMS ?= linux/amd64,linux/arm64

.PHONY: docker-publish-config
docker-publish-config:
	@test -n "$(DOCKER_REPO)" || (echo "Set DOCKERHUB_USERNAME or DOCKER_REPO=your-namespace/better-apigate"; exit 1)
	@printf '%s\n' "$(DOCKER_TAG)" | grep -Eq '^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$$' || (echo "DOCKER_TAG must be a version such as 1.0.0"; exit 1)

docker-publish: docker-publish-config docker-build-binaries
	@echo "Building and publishing multi-arch image to $(DOCKER_REPO)..."
	docker buildx create --name apigate-builder --use 2>/dev/null || docker buildx use apigate-builder
	docker buildx build \
		--platform $(DOCKER_PLATFORMS) \
		--tag $(DOCKER_REPO):$(DOCKER_TAG) \
		--push \
		-f Dockerfile.release \
		.
	@echo "Published $(DOCKER_REPO):$(DOCKER_TAG)"

# Build binaries for Docker (pre-build to avoid memory issues in buildx)
docker-build-binaries: webui
	@echo "Building binaries for Docker..."
	@mkdir -p build/linux/amd64 build/linux/arm64
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o build/linux/amd64/better-apigate ./cmd/better-apigate
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o build/linux/arm64/better-apigate ./cmd/better-apigate
	@echo "Binaries built in build/linux/"

# Create a release
release: contracts webui
	@echo "Building for multiple platforms..."
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build $(LDFLAGS) -o dist/better-apigate-linux-amd64 ./cmd/better-apigate
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build $(LDFLAGS) -o dist/better-apigate-linux-arm64 ./cmd/better-apigate
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build $(LDFLAGS) -o dist/better-apigate-darwin-amd64 ./cmd/better-apigate
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build $(LDFLAGS) -o dist/better-apigate-darwin-arm64 ./cmd/better-apigate
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build $(LDFLAGS) -o dist/better-apigate-windows-amd64.exe ./cmd/better-apigate

# Generate API key for testing
genkey:
	@openssl rand -hex 32 | sed 's/^/ak_/'

help:
	@echo "Available targets:"
	@echo "  all        - Build webui + Go binary (default)"
	@echo "  webui      - Build webui and sync to embed location"
	@echo "  webui-install - Install webui npm dependencies"
	@echo "  build      - Build Go binary only"
	@echo "  run        - Build and run"
	@echo "  dev        - Run with go run"
	@echo "  test       - Run tests"
	@echo "  docker     - Build Docker image"
	@echo "  docker-run - Run the published Docker image"
	@echo "  docker-publish - Build and push multi-arch image to $(DOCKER_REPO)"
	@echo "  docker-build-binaries - Build linux binaries for Docker"
	@echo "  release    - Build for all platforms"
	@echo "  clean      - Remove build artifacts"

# Go models are the source of OpenAPI and frontend contracts.
.PHONY: contracts validate
contracts:
	go run ./cmd/contracts > docs/openapi-v2.json
	go run ./cmd/contracts -typescript > webui/src/v2/contracts.generated.ts
validate: contracts webui build
