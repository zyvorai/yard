.PHONY: test build web run sim tidy ship

YARD_LISTEN ?= :8080
YARD_DATABASE_URL ?= file:data/yard.db?_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)

tidy:
	go mod tidy

test:
	go test ./...
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
