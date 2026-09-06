.PHONY: run run-worker build test lint migrate migrate-down tidy sqlc

run: ## sobe a API (aplica migrações no boot)
	go run ./cmd/api

run-worker: ## sobe o territory-worker
	go run ./cmd/worker

build:
	go build -o bin/api ./cmd/api
	go build -o bin/worker ./cmd/worker

test:
	go test ./... -race -count=1

lint:
	go vet ./...
	@command -v golangci-lint >/dev/null 2>&1 && golangci-lint run || echo "golangci-lint não instalado — pulando"

migrate: ## aplica migrações pendentes
	go run ./cmd/migrate up

migrate-down: ## desfaz a última migração
	go run ./cmd/migrate down

migrate-status:
	go run ./cmd/migrate status

tidy:
	go mod tidy

sqlc: ## gera código a partir de db/query/*.sql
	@command -v sqlc >/dev/null 2>&1 && sqlc generate || echo "sqlc não instalado (go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest)"

help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-16s\033[0m %s\n", $$1, $$2}'
