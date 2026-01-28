# Twitter Clone Makefile
REGISTRY := localhost:5000
VERSION := latest
CLUSTER := twitter-clone

# Service list
SERVICES := user-service tweet-service timeline-service search-service media-service notification-service fanout-service

# Colors for output
RED := \033[0;31m
GREEN := \033[0;32m
YELLOW := \033[0;33m
NC := \033[0m # No Color

.PHONY: help
help: ## Show this help message
	@echo "Twitter Clone - Build and Deployment Automation"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  $(GREEN)%-20s$(NC) %s\n", $$1, $$2}'

.PHONY: build
build: ## Build a specific service (usage: make build SERVICE=user-service)
ifndef SERVICE
	@echo "$(RED)Error: SERVICE is not set$(NC)"
	@echo "Usage: make build SERVICE=user-service"
	@exit 1
endif
	@echo "$(YELLOW)Building $(SERVICE)...$(NC)"
	docker build -t $(REGISTRY)/$(SERVICE):$(VERSION) -f services/$(SERVICE)/Dockerfile services/$(SERVICE)
	@echo "$(GREEN)✓ $(SERVICE) built successfully$(NC)"

.PHONY: build-all
build-all: ## Build all services
	@echo "$(YELLOW)Building all services...$(NC)"
	@for service in $(SERVICES); do \
		echo "$(YELLOW)Building $$service...$(NC)"; \
		docker build -t $(REGISTRY)/$$service:$(VERSION) -f services/$$service/Dockerfile services/$$service || exit 1; \
		echo "$(GREEN)✓ $$service built$(NC)"; \
	done
	@echo "$(GREEN)✓ All services built successfully$(NC)"

.PHONY: push
push: ## Push a specific service to registry (usage: make push SERVICE=user-service)
ifndef SERVICE
	@echo "$(RED)Error: SERVICE is not set$(NC)"
	@echo "Usage: make push SERVICE=user-service"
	@exit 1
endif
	@echo "$(YELLOW)Pushing $(SERVICE) to registry...$(NC)"
	docker push $(REGISTRY)/$(SERVICE):$(VERSION)
	@echo "$(GREEN)✓ $(SERVICE) pushed successfully$(NC)"

.PHONY: push-all
push-all: ## Push all services to registry
	@echo "$(YELLOW)Pushing all services to registry...$(NC)"
	@for service in $(SERVICES); do \
		echo "$(YELLOW)Pushing $$service...$(NC)"; \
		docker push $(REGISTRY)/$$service:$(VERSION) || exit 1; \
		echo "$(GREEN)✓ $$service pushed$(NC)"; \
	done
	@echo "$(GREEN)✓ All services pushed successfully$(NC)"

.PHONY: import-k3d
import-k3d: ## Import images to k3d cluster (usage: make import-k3d [CLUSTER=name])
	@echo "$(YELLOW)Importing images to k3d cluster $(CLUSTER)...$(NC)"
	@for service in $(SERVICES); do \
		echo "$(YELLOW)Importing $$service...$(NC)"; \
		k3d image import $(REGISTRY)/$$service:$(VERSION) -c $(CLUSTER) || exit 1; \
		echo "$(GREEN)✓ $$service imported$(NC)"; \
	done
	@echo "$(GREEN)✓ All images imported successfully$(NC)"

.PHONY: test
test: ## Run unit tests for all services
	@echo "$(YELLOW)Running unit tests...$(NC)"
	@for service in $(SERVICES); do \
		echo "$(YELLOW)Testing $$service...$(NC)"; \
		cd services/$$service && go test ./... || exit 1; \
		cd ../..; \
		echo "$(GREEN)✓ $$service tests passed$(NC)"; \
	done
	@echo "$(GREEN)✓ All tests passed$(NC)"

.PHONY: test-service
test-service: ## Run tests for a specific service (usage: make test-service SERVICE=user-service)
ifndef SERVICE
	@echo "$(RED)Error: SERVICE is not set$(NC)"
	@echo "Usage: make test-service SERVICE=user-service"
	@exit 1
endif
	@echo "$(YELLOW)Testing $(SERVICE)...$(NC)"
	cd services/$(SERVICE) && go test -v ./...
	@echo "$(GREEN)✓ $(SERVICE) tests passed$(NC)"

.PHONY: test-integration
test-integration: ## Run integration tests
	@echo "$(YELLOW)Running integration tests...$(NC)"
	@for service in $(SERVICES); do \
		if [ -f "services/$$service/internal/handlers/integration_test.go" ]; then \
			echo "$(YELLOW)Integration testing $$service...$(NC)"; \
			cd services/$$service && go test -tags=integration ./... || exit 1; \
			cd ../..; \
			echo "$(GREEN)✓ $$service integration tests passed$(NC)"; \
		fi; \
	done
	@echo "$(GREEN)✓ All integration tests passed$(NC)"

.PHONY: lint
lint: ## Run linter on all services
	@echo "$(YELLOW)Running linter...$(NC)"
	@for service in $(SERVICES); do \
		echo "$(YELLOW)Linting $$service...$(NC)"; \
		cd services/$$service && golangci-lint run || exit 1; \
		cd ../..; \
		echo "$(GREEN)✓ $$service linted$(NC)"; \
	done
	@echo "$(GREEN)✓ All services linted successfully$(NC)"

.PHONY: fmt
fmt: ## Format all Go code
	@echo "$(YELLOW)Formatting code...$(NC)"
	@for service in $(SERVICES); do \
		echo "$(YELLOW)Formatting $$service...$(NC)"; \
		cd services/$$service && go fmt ./... || exit 1; \
		cd ../..; \
	done
	cd pkg && go fmt ./...
	@echo "$(GREEN)✓ All code formatted$(NC)"

.PHONY: mod-tidy
mod-tidy: ## Run go mod tidy for all services
	@echo "$(YELLOW)Tidying Go modules...$(NC)"
	@for service in $(SERVICES); do \
		echo "$(YELLOW)Tidying $$service...$(NC)"; \
		cd services/$$service && go mod tidy || exit 1; \
		cd ../..; \
	done
	cd pkg && go mod tidy
	@echo "$(GREEN)✓ All modules tidied$(NC)"

.PHONY: deploy
deploy: ## Deploy to Kubernetes using Helm
	@echo "$(YELLOW)Deploying Twitter Clone to Kubernetes...$(NC)"
	helm upgrade --install twitter ./helm/twitter-stack \
		-n twitter \
		--create-namespace \
		--set image.registry=$(REGISTRY) \
		--set image.tag=$(VERSION)
	@echo "$(GREEN)✓ Deployment successful$(NC)"

.PHONY: deploy-dev
deploy-dev: ## Deploy with development values
	@echo "$(YELLOW)Deploying Twitter Clone (dev)...$(NC)"
	helm upgrade --install twitter ./helm/twitter-stack \
		-n twitter \
		--create-namespace \
		-f ./helm/twitter-stack/values-dev.yaml \
		--set image.registry=$(REGISTRY) \
		--set image.tag=$(VERSION)
	@echo "$(GREEN)✓ Development deployment successful$(NC)"

.PHONY: deploy-prod
deploy-prod: ## Deploy with production values
	@echo "$(YELLOW)Deploying Twitter Clone (prod)...$(NC)"
	helm upgrade --install twitter ./helm/twitter-stack \
		-n twitter \
		--create-namespace \
		-f ./helm/twitter-stack/values-prod.yaml \
		--set image.registry=$(REGISTRY) \
		--set image.tag=$(VERSION)
	@echo "$(GREEN)✓ Production deployment successful$(NC)"

.PHONY: undeploy
undeploy: ## Remove deployment from Kubernetes
	@echo "$(YELLOW)Removing Twitter Clone deployment...$(NC)"
	helm uninstall twitter -n twitter
	@echo "$(GREEN)✓ Deployment removed$(NC)"

.PHONY: install-operators
install-operators: ## Install required Kubernetes operators
	@echo "$(YELLOW)Installing Kubernetes operators...$(NC)"
	./helm/twitter-stack/scripts/install-operators.sh
	@echo "$(GREEN)✓ Operators installed$(NC)"

.PHONY: security-audit
security-audit: ## Run security audit on deployed pods
	@echo "$(YELLOW)Running security audit...$(NC)"
	./helm/twitter-stack/scripts/run-security-audit.sh
	@echo "$(GREEN)✓ Security audit complete$(NC)"

.PHONY: helm-test
helm-test: ## Run Helm chart tests
	@echo "$(YELLOW)Running Helm tests...$(NC)"
	./helm/twitter-stack/scripts/run-tests.sh
	@echo "$(GREEN)✓ Helm tests passed$(NC)"

.PHONY: logs
logs: ## Show logs for a service (usage: make logs SERVICE=user-service)
ifndef SERVICE
	@echo "$(RED)Error: SERVICE is not set$(NC)"
	@echo "Usage: make logs SERVICE=user-service"
	@exit 1
endif
	kubectl logs -n twitter -l app=$(SERVICE) --tail=100 -f

.PHONY: status
status: ## Show status of all services
	@echo "$(YELLOW)Checking service status...$(NC)"
	@echo ""
	@echo "Pods:"
	@kubectl get pods -n twitter
	@echo ""
	@echo "Services:"
	@kubectl get svc -n twitter
	@echo ""
	@echo "PostgreSQL Clusters:"
	@kubectl get clusters.postgresql.cnpg.io -n twitter
	@echo ""
	@echo "Redis:"
	@kubectl get redisfailovers -n twitter
	@echo ""
	@echo "Elasticsearch:"
	@kubectl get elasticsearch -n twitter
	@echo ""
	@echo "RabbitMQ:"
	@kubectl get rabbitmqclusters -n twitter

.PHONY: port-forward
port-forward: ## Port forward a service (usage: make port-forward SERVICE=user-service PORT=8080)
ifndef SERVICE
	@echo "$(RED)Error: SERVICE is not set$(NC)"
	@echo "Usage: make port-forward SERVICE=user-service PORT=8080"
	@exit 1
endif
PORT ?= 8080
	@echo "$(YELLOW)Port forwarding $(SERVICE) on port $(PORT)...$(NC)"
	kubectl port-forward -n twitter svc/$(SERVICE) $(PORT):8080

.PHONY: clean
clean: ## Clean up Docker images
	@echo "$(YELLOW)Cleaning up Docker images...$(NC)"
	@for service in $(SERVICES); do \
		docker rmi $(REGISTRY)/$$service:$(VERSION) 2>/dev/null || true; \
	done
	@echo "$(GREEN)✓ Cleanup complete$(NC)"

.PHONY: dev
dev: build-all import-k3d deploy-dev ## Complete dev deployment (build, import, deploy)
	@echo "$(GREEN)✓ Development environment ready!$(NC)"

.PHONY: prod
prod: build-all push-all deploy-prod ## Complete prod deployment (build, push, deploy)
	@echo "$(GREEN)✓ Production deployment complete!$(NC)"

# Docker Compose targets for local development
.PHONY: docker-up
docker-up: ## Start services with docker-compose
	@echo "$(YELLOW)Starting services with docker-compose...$(NC)"
	docker-compose up -d
	@echo "$(GREEN)✓ Services started$(NC)"

.PHONY: docker-down
docker-down: ## Stop services with docker-compose
	@echo "$(YELLOW)Stopping services...$(NC)"
	docker-compose down
	@echo "$(GREEN)✓ Services stopped$(NC)"

.PHONY: docker-logs
docker-logs: ## Show docker-compose logs
	docker-compose logs -f

# Default target
.DEFAULT_GOAL := help