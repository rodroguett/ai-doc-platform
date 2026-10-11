.PHONY: run run-db test lint build tidy bootstrap generate db-up db-down

# Go no debe cambiar de toolchain por su cuenta: una dependencia que exija
# una versión más nueva tiene que fallar, no descargarla (ADR-0002).
export GOTOOLCHAIN := local

# Postgres local para desarrollo y tests. La misma imagen se usa en CI.
PG_IMAGE     := pgvector/pgvector:0.8.7-pg17
PG_CONTAINER := aidoc-pg
PG_URL       := postgres://postgres:postgres@localhost:55432/aidoc?sslmode=disable

# Token de escritura para desarrollo local. No protege nada: es público.
DEV_TOKEN := dev-token

run:
	go run ./cmd/gateway

run-db: db-up
	DATABASE_URL='$(PG_URL)' API_WRITE_TOKEN='$(DEV_TOKEN)' go run ./cmd/gateway

test: db-up
	TEST_DATABASE_URL='$(PG_URL)' go test -race -cover ./...

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

db-up:
	@docker start $(PG_CONTAINER) >/dev/null 2>&1 || docker run -d --name $(PG_CONTAINER) \
		-e POSTGRES_PASSWORD=postgres -e POSTGRES_DB=aidoc -p 55432:5432 $(PG_IMAGE) >/dev/null
	@for i in $$(seq 1 30); do \
		docker exec $(PG_CONTAINER) psql -U postgres -d aidoc -qtAc 'select 1' >/dev/null 2>&1 && exit 0; \
		sleep 1; \
	done; echo "Postgres no respondió a tiempo" && exit 1

db-down:
	@docker rm -f $(PG_CONTAINER) >/dev/null 2>&1 || true
