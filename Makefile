-include .env
export

.PHONY: help db db-down run web-dev test test-short lint web-check check build up down

help: ## List the targets
	@grep -hE '^[a-z-]+:.*## ' $(MAKEFILE_LIST) | awk -F ':.*## ' '{printf "%-12s %s\n", $$1, $$2}'

db: ## Start the development database
	docker compose -f compose.dev.yaml up -d --wait

db-down: ## Stop the development database, keeping its data
	docker compose -f compose.dev.yaml down

run: ## Run the server against the development database
	go run ./cmd/server

web-dev: ## Run the front's dev server on http://localhost:5173
	cd web && pnpm install && pnpm run dev

test: ## Run the Go tests, the Postgres ones included (needs Docker)
	go test ./...

test-short: ## Run the Go tests without Docker
	go test -short ./...

lint: ## Lint the Go code, with and without the embedded front
	golangci-lint run ./...
	golangci-lint run --build-tags embed ./...

web-check: ## Check the front: types, lint, format, tests
	cd web && pnpm install --frozen-lockfile && pnpm run typecheck && pnpm run lint && pnpm run format:check && pnpm test

check: test lint web-check ## Run every check

build: ## Build the hushtongue binary, with the front embedded
	cd web && pnpm install --frozen-lockfile && pnpm run build
	go build -tags embed -o hushtongue ./cmd/server

up: ## Build and start the whole service on http://localhost:8080
	docker compose up -d --build --wait

down: ## Stop the whole service, keeping its data
	docker compose down

# Creates .env from the template on the first run
.env:
	cp .env.example .env
