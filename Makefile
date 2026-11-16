.PHONY: build run test migrate-up migrate-down docker-up docker-down lint

build:
	go build -o bin/server ./cmd/server

run:
	go run ./cmd/server

test:
	go test -v -race -cover ./...

migrate-up:
	migrate -path migrations -database "postgres://postgres:postgres@localhost:5432/order_management?sslmode=disable" up

migrate-down:
	migrate -path migrations -database "postgres://postgres:postgres@localhost:5432/order_management?sslmode=disable" down

docker-up:
	docker compose up -d --build

docker-down:
	docker compose down

lint:
	golangci-lint run ./...
