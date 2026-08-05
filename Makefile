.PHONY: run test lint build tidy

run:
	go run ./cmd/gateway

test:
	go test -race -cover ./...

lint:
	golangci-lint run

build:
	go build -o bin/gateway ./cmd/gateway

tidy:
	go mod tidy
