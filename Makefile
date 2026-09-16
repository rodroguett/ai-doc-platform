.PHONY: run test lint build tidy bootstrap generate

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

bootstrap:
	./scripts/bootstrap.sh
generate:
	oapi-codegen -config api/codegen.yaml api/openapi.yaml
