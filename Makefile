SHELL := /bin/bash

.PHONY: run test lint build migrate-up migrate-down

run:
	go run ./cmd/api

test:
	go test ./...

lint:
	@echo "Checking gofmt..."
	@test -z "$$(gofmt -l .)" || (echo "Run gofmt on listed files"; gofmt -l .; exit 1)

build:
	go build ./...

migrate-up:
	go run ./cmd/migrate -direction up

migrate-down:
	go run ./cmd/migrate -direction down
