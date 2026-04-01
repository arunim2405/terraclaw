APP_NAME    := terraclaw
DOCKER_IMG  := $(APP_NAME)
DOCKER_TAG  := latest
GO_FLAGS    := -trimpath
ENV_FILE    := .docker.env

# ──────────────────────────────────────────────
# Go
# ──────────────────────────────────────────────

.PHONY: build
build: ## Build the binary for the current platform
	go build $(GO_FLAGS) -o bin/$(APP_NAME) .

.PHONY: build-linux
build-linux: ## Cross-compile for linux/amd64
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build $(GO_FLAGS) -o bin/$(APP_NAME)-linux-amd64 .

.PHONY: run
run: build ## Build and run with sample args (override with ARGS=)
	./bin/$(APP_NAME) $(ARGS)

.PHONY: test
test: ## Run all tests
	go test ./...

.PHONY: lint
lint: ## Run go vet
	go vet ./...

.PHONY: clean
clean: ## Remove build artifacts
	rm -rf bin/

# ──────────────────────────────────────────────
# Docker
# ──────────────────────────────────────────────

.PHONY: docker-build
docker-build: ## Build the Docker image
	docker build -t $(DOCKER_IMG):$(DOCKER_TAG) .

.PHONY: docker-run
docker-run: ## Run the Docker container with .docker.env
	docker run --rm --env-file $(ENV_FILE) $(DOCKER_IMG):$(DOCKER_TAG)

.PHONY: docker-run-artifacts
docker-run-artifacts: ## Run and extract output to ./artifacts
	mkdir -p artifacts
	docker run --rm \
		--env-file $(ENV_FILE) \
		-e LOCAL_ARTIFACTS_DIR=/artifacts \
		-v $(CURDIR)/artifacts:/artifacts \
		$(DOCKER_IMG):$(DOCKER_TAG)

.PHONY: docker-shell
docker-shell: ## Open a shell in the Docker container
	docker run --rm -it --env-file $(ENV_FILE) --entrypoint /bin/bash $(DOCKER_IMG):$(DOCKER_TAG)

# ──────────────────────────────────────────────
# Helpers
# ──────────────────────────────────────────────

.PHONY: env-sample
env-sample: ## Copy sample env to .docker.env
	cp -n .sample.docker.env .docker.env || true
	@echo "Edit .docker.env with your credentials"

.PHONY: help
help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-22s\033[0m %s\n", $$1, $$2}'

.DEFAULT_GOAL := help
