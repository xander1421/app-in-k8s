.PHONY: all build build-local build-docker push test clean install-operators deploy undeploy lint fmt help

REGISTRY ?= ghcr.io/alexprut/twitter-clone
TAG ?= latest

SERVICES = user-service tweet-service timeline-service fanout-service media-service notification-service search-service

# Default target
all: build

# Build all services locally
build-local:
	@echo "Building all services locally..."
	@mkdir -p bin
	@for service in $(SERVICES); do \
		echo "Building $$service..."; \
		if [ "$$service" = "fanout-service" ]; then \
			(cd services/$$service && go build -o ../../bin/$$service ./cmd/worker); \
		else \
			(cd services/$$service && go build -o ../../bin/$$service ./cmd/server); \
		fi \
	done
	@echo "Done!"

# Build Docker images
build-docker:
	@echo "Building Docker images..."
	@for service in $(SERVICES); do \
		echo "Building $$service..."; \
		if [ "$$service" = "fanout-service" ]; then \
			docker build -t $(REGISTRY)/$$service:$(TAG) \
				--build-arg SERVICE=$$service \
				--build-arg CMD_PATH=cmd/worker \
				-f Dockerfile.service .; \
		else \
			docker build -t $(REGISTRY)/$$service:$(TAG) \
				--build-arg SERVICE=$$service \
				--build-arg CMD_PATH=cmd/server \
				-f Dockerfile.service .; \
		fi \
	done
	@echo "Done!"

# Build alias (defaults to Docker)
build: build-docker

# Push all images
push:
	@echo "Pushing images to $(REGISTRY)..."
	@for service in $(SERVICES); do \
		echo "Pushing $$service..."; \
		docker push $(REGISTRY)/$$service:$(TAG); \
	done
	@echo "Done!"

# Run tests
test:
	@echo "Running tests..."
	@go test ./...

# Lint code
lint:
	@echo "Linting code..."
	@golangci-lint run ./...

# Format code
fmt:
	@echo "Formatting code..."
	@gofmt -s -w .

# Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -rf bin/
	@go clean ./...

# Install operators
install-operators:
	@echo "Installing Kubernetes operators..."
	@chmod +x scripts/install-operators.sh
	@./scripts/install-operators.sh

# Deploy to Kubernetes
deploy:
	@echo "Deploying Twitter stack..."
	@helm upgrade --install twitter ./helm/twitter-stack -n twitter --create-namespace

# Undeploy from Kubernetes
undeploy:
	@echo "Undeploying Twitter stack..."
	@helm uninstall twitter -n twitter || true
	@kubectl delete namespace twitter || true

# Create k3d cluster for local development
k3d-create:
	@echo "Creating k3d cluster..."
	@k3d cluster create twitter-k8s --servers 1 --agents 3 \
		-p "80:80@loadbalancer" \
		-p "443:443@loadbalancer"

# Delete k3d cluster
k3d-delete:
	@echo "Deleting k3d cluster..."
	@k3d cluster delete twitter-k8s

# Local development setup
dev-setup: k3d-create install-operators
	@echo "Development environment ready!"
	@echo "Deploy with: make deploy"

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	@(cd pkg && go mod download)
	@for service in $(SERVICES); do \
		(cd services/$$service && go mod download); \
	done

# Generate mocks (if needed)
generate:
	@echo "Generating code..."
	@go generate ./...

# Port forward for local testing
port-forward:
	@echo "Port forwarding services..."
	@kubectl port-forward -n twitter svc/twitter-gateway 8080:80 &
	@echo "Gateway available at http://localhost:8080"

# Show logs
logs:
	@echo "Select a service: $(SERVICES)"
	@read -p "Service name: " service; \
	kubectl logs -n twitter -l app=$$service --tail=100 -f

# Help
help:
	@echo "Twitter Clone - Makefile Commands"
	@echo ""
	@echo "Build:"
	@echo "  build-local     Build all services locally"
	@echo "  build-docker    Build Docker images for all services"
	@echo "  build           Alias for build-docker"
	@echo "  push            Push all images to registry"
	@echo ""
	@echo "Development:"
	@echo "  test            Run tests"
	@echo "  lint            Lint code"
	@echo "  fmt             Format code"
	@echo "  clean           Clean build artifacts"
	@echo "  deps            Download dependencies"
	@echo ""
	@echo "Kubernetes:"
	@echo "  install-operators  Install required operators"
	@echo "  deploy             Deploy Twitter stack"
	@echo "  undeploy           Remove Twitter stack"
	@echo "  port-forward       Forward gateway port locally"
	@echo "  logs               View service logs"
	@echo ""
	@echo "Local Development:"
	@echo "  k3d-create      Create k3d cluster"
	@echo "  k3d-delete      Delete k3d cluster"
	@echo "  dev-setup       Full local dev setup"
	@echo ""
	@echo "Variables:"
	@echo "  REGISTRY=$(REGISTRY)"
	@echo "  TAG=$(TAG)"
