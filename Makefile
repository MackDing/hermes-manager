.DEFAULT_GOAL := help

.PHONY: help build test lint dev dev-stop docker-image helm-lint clean

help: ## Show available targets
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Build hermesmanager binary (requires npm + go)
	cd web && npm ci && npm run build
	go build -ldflags "-s -w -X main.version=$$(git describe --tags --always --dirty)" \
		-o hermesmanager ./cmd/hermesmanager

test: ## Run all tests (Go race + coverage)
	go test ./... -race -cover

lint: ## Run go vet + gofmt check
	go vet ./...
	@test -z "$$(gofmt -l .)" || (echo "gofmt needed on:"; gofmt -l .; exit 1)

dev: ## Start local Postgres + run backend (foreground)
	docker compose up -d postgres
	@echo "Waiting for Postgres..."
	@until docker compose exec -T postgres pg_isready -U hermesmanager >/dev/null 2>&1; do sleep 1; done
	@echo "Postgres ready. Starting hermesmanager..."
	DATABASE_URL=postgres://hermesmanager:dev@localhost:5432/hermesmanager?sslmode=disable \
		go run ./cmd/hermesmanager

dev-stop: ## Stop local Postgres
	docker compose down

docker-image: ## Build local Docker image
	mkdir -p build/linux/amd64
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" \
		-o build/linux/amd64/hermesmanager ./cmd/hermesmanager
	docker build -t hermesmanager:dev .

helm-lint: ## Lint Helm chart
	helm lint deploy/helm/hermesmanager

clean: ## Remove build artifacts
	rm -rf hermesmanager build/ web/dist/
