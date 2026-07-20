.PHONY: build run-api run-worker test tidy compose-up compose-down migrate-up fmt vet

build:
	go build ./...

run-api:
	go run ./cmd/api

run-worker:
	go run ./cmd/worker

test:
	go test ./...

tidy:
	go mod tidy

fmt:
	go fmt ./...

vet:
	go vet ./...

compose-up:
	docker compose -f deployments/docker-compose.yml up -d --build

compose-down:
	docker compose -f deployments/docker-compose.yml down
