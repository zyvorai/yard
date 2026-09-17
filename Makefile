.PHONY: fmt vet test race check build web run sim tidy ship backup restore

YARD_LISTEN ?= :8080
YARD_DATABASE_URL ?= file:data/yard.db?_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)

tidy:
	go mod tidy

fmt:
	@test -z "$$(gofmt -l .)" || (echo "Run gofmt on:"; gofmt -l .; exit 1)

vet:
	go vet ./...

test:
	go test ./...
	cd web && npm test

race:
	go test -race ./...

check: fmt vet race
	cd web && npm test

web:
	cd web && npm install && npm run build
	rm -rf cmd/yard/static
	cp -R web/dist cmd/yard/static

build: web
	mkdir -p bin
	go build -o bin/yard ./cmd/yard
	go build -o bin/yard-simulator ./cmd/simulator
	go build -o bin/yard-agent-gateway ./cmd/agent-gateway

run:
	mkdir -p data
	go run ./cmd/yard

sim:
	go run ./cmd/simulator

# make ship HOST=sus@1.2.3.4 ARGS='--with-sim'
ship:
	./scripts/ship $(HOST) $(ARGS)

backup:
	./scripts/backup.sh $(ARGS)

# make restore FILE=backups/yard-20260101T000000Z.db
restore:
	./scripts/restore.sh $(FILE) $(ARGS)
