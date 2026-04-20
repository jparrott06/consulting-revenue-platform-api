SHELL := /bin/bash

.PHONY: run test lint build migrate-up migrate-down seed retention-once openapi-validate demo-api

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

seed:
	go run ./cmd/seed

retention-once:
	go run ./cmd/retention

openapi-validate:
	python3 -m pip install -q openapi-spec-validator pyyaml
	python3 -c "import yaml; from openapi_spec_validator import validate_spec; validate_spec(yaml.safe_load(open('docs/openapi.yaml')))"

demo-api:
	./scripts/demo-api.sh
