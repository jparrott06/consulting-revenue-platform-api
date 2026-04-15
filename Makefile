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
	@echo "No migrations configured yet."

migrate-down:
	@echo "No migrations configured yet."
