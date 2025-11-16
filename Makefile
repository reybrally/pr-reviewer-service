BINARY := pr-reviewer-service
CMD_PATH := ./cmd
DOCKER_COMPOSE := docker compose

.PHONY: build run test lint docker-build docker-up docker-down

build:
	go build -o bin/$(BINARY) $(CMD_PATH)

run:
	go run ./cmd

test:
	go test ./...

lint:
	golangci-lint run ./...

docker-build:
	$(DOCKER_COMPOSE) build

docker-up:
	$(DOCKER_COMPOSE) up

docker-down:
	$(DOCKER_COMPOSE) down