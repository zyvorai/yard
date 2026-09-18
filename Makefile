.PHONY: fmt vet test race check build web run sim tidy ship backup restore help ci status deploy-remote

YARD_LISTEN ?= :8080
YARD_DATABASE_URL ?= file:data/yard.db?_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)

tidy: ## go mod tidy
	go mod tidy

fmt: ## Fail if any Go file needs gofmt
	@test -z "$$(gofmt -l .)" || (echo "Run gofmt on:"; gofmt -l .; exit 1)

vet: ## go vet
	go vet ./...

test: ## Go tests and web tests
	go test ./...
	cd web && npm test

race: ## Tests with the race detector
	go test -race ./...

check: fmt vet race ## gofmt, vet, and race tests
	cd web && npm test

web: ## Build the console and copy it into the binary tree
	cd web && npm install && npm run build
	rm -rf cmd/yard/static
	cp -R web/dist cmd/yard/static

build: web ## Build yard, simulator, and agent gateway
	mkdir -p bin
	go build -o bin/yard ./cmd/yard
	go build -o bin/yard-simulator ./cmd/simulator
	go build -o bin/yard-agent-gateway ./cmd/agent-gateway

run: ## Run the server (YARD_LISTEN, default :8080)
	mkdir -p data
	go run ./cmd/yard

sim: ## Run the simulator
	go run ./cmd/simulator

# make ship HOST=sus@1.2.3.4 ARGS='--with-sim'
ship: ## Install on a host: make ship HOST=user@host
	./scripts/ship $(HOST) $(ARGS)

backup: ## Write a database backup
	./scripts/backup.sh $(ARGS)

# make restore FILE=backups/yard-20260101T000000Z.db
restore: ## Restore a database backup
	./scripts/restore.sh $(FILE) $(ARGS)

ci: fmt vet test ## Local gate: gofmt, vet, Go tests, web tests

status: ## GET /healthz on a running server (YARD_URL, default http://127.0.0.1:8080)
	curl -fsS "$(or $(YARD_URL),http://127.0.0.1:8080)/healthz"
	@echo

deploy-remote: ## Deploy: make deploy-remote H=<host> [U=sus] [ARGS=--with-sim]
	@test -n "$(H)" || (echo "Usage: make deploy-remote H=<host> [U=user] [ARGS=--with-sim]"; exit 1)
	./scripts/deploy-remote.sh $(or $(U),sus)@$(H) $(ARGS)

help: ## Show targets
	@grep -E '^[a-zA-Z0-9_-]+:.*## ' $(MAKEFILE_LIST) | sort | awk -F':.*## ' '{printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'
