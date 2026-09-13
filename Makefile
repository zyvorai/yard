.PHONY: test build web run sim tidy

ESTATE_LISTEN ?= :8080
ESTATE_DATABASE_URL ?= file:data/estate.db?_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)

tidy:
	go mod tidy

test:
	go test ./...
	cd web && npm test

web:
	cd web && npm install && npm run build
	rm -rf cmd/estate/static
	cp -R web/dist cmd/estate/static

build: web
	mkdir -p bin
	go build -o bin/estate ./cmd/estate
	go build -o bin/estate-simulator ./cmd/simulator
	go build -o bin/estate-agent-gateway ./cmd/agent-gateway

run:
	mkdir -p data
	go run ./cmd/estate

sim:
	go run ./cmd/simulator
