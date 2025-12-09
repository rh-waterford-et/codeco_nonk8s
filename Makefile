# Variables
IMAGE_NAME ?= codeconk8
IMAGE_TAG ?= latest
IMAGE_REGISTRY ?= quay.io/rh_et_wd/codeco
FULL_IMAGE ?= $(IMAGE_REGISTRY)/$(IMAGE_NAME):$(IMAGE_TAG)
NAMESPACE ?= codeco

.PHONY: lint fmt test build clean vendor vendor-update vendor-verify

lint:
	golangci-lint run ./...

fmt:
	gofmt -s -w .

vendor: ## Download and vendor all dependencies
	go mod tidy
	go mod vendor
	go mod verify

vendor-update: ## Update dependencies and re-vendor
	go get -u ./...
	go mod tidy
	go mod vendor

vendor-verify: ## Verify vendored dependencies are up to date
	go mod verify
	@echo "Verifying vendor directory is in sync..."
	@git diff --exit-code vendor/ || (echo "ERROR: vendor/ is out of sync. Run 'make vendor'" && exit 1)

test:
	go test -v -race -coverprofile=coverage.out ./...

test-contract:
	go test -v -race ./tests/unit/*_contract_test.go

test-integration:
	go test -v -race ./tests/integration/...

build: ## Build the binary using vendored dependencies
	go build -mod=vendor -o bin/vk-flightctl-provider ./cmd/vk-flightctl-provider

clean: ## Clean build artifacts
	rm -rf bin/ coverage.out

##@ Docker

.PHONY: docker-build docker-push docker-run

docker-build: ## Build Docker image
	docker build -t $(FULL_IMAGE) .

docker-push: ## Push Docker image to registry
	docker push $(FULL_IMAGE)

docker-run: ## Run container locally (requires env vars)
	docker run --rm \
		-e FLIGHTCTL_API_URL \
		-e FLIGHTCTL_CLIENT_ID \
		-e FLIGHTCTL_CLIENT_SECRET \
		-e FLIGHTCTL_TOKEN_URL \
		-e FLIGHTCTL_INSECURE_TLS \
		$(FULL_IMAGE)

##@ Kubernetes

.PHONY: deploy deploy-kustomize create-secret delete status logs restart

deploy: ## Deploy to Kubernetes
	kubectl apply -f deploy/rbac.yaml
	kubectl apply -f deploy/configmap.yaml
	@echo "Note: Create secret if not already created with: make create-secret CLIENT_ID=xxx CLIENT_SECRET=yyy"
	kubectl apply -f deploy/deployment.yaml

deploy-kustomize: ## Deploy using kustomize
	kubectl apply -k deploy/

create-secret: ## Create Kubernetes secret (requires CLIENT_ID and CLIENT_SECRET)
	@if [ -z "$(CLIENT_ID)" ] || [ -z "$(CLIENT_SECRET)" ]; then \
		echo "Error: CLIENT_ID and CLIENT_SECRET must be set"; \
		echo "Usage: make create-secret CLIENT_ID=xxx CLIENT_SECRET=yyy"; \
		exit 1; \
	fi
	kubectl create secret generic vk-flightctl-oauth \
		--from-literal=client-id=$(CLIENT_ID) \
		--from-literal=client-secret=$(CLIENT_SECRET) \
		--namespace=$(NAMESPACE) \
		--dry-run=client -o yaml | kubectl apply -f -

delete: ## Delete Kubernetes resources
	kubectl delete -f deploy/ --ignore-not-found=true

status: ## Check deployment status
	@echo "=== Deployment Status ==="
	kubectl get deployment -l app=vk-flightctl-provider -n $(NAMESPACE)
	@echo ""
	@echo "=== Pod Status ==="
	kubectl get pods -l app=vk-flightctl-provider -n $(NAMESPACE)
	@echo ""
	@echo "=== Virtual Nodes ==="
	kubectl get nodes | grep vk- || echo "No virtual nodes found"

logs: ## Tail logs from the provider pod
	kubectl logs -l app=vk-flightctl-provider -n $(NAMESPACE) -f

restart: ## Restart the deployment
	kubectl rollout restart deployment/vk-flightctl-provider -n $(NAMESPACE)

##@ All-in-one

.PHONY: all full-deploy

all: fmt test build docker-build ## Run all local build steps

full-deploy: docker-build docker-push deploy ## Build, push, and deploy
	@echo "Deployment complete! Check status with: make status"

.DEFAULT_GOAL := build
