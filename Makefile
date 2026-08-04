.PHONY: help up down scan logs tidy build api-dev web-dev test

help:
	@echo "FareWatch targets:"
	@echo "  make up        Start full stack (loads .env)"
	@echo "  make down      Stop stack"
	@echo "  make scan      Run one scanner job"
	@echo "  make logs      Tail api + web logs"
	@echo "  make build     Build Go binaries + frontend"
	@echo "  make api-dev   Run API on host"
	@echo "  make web-dev   Run Vite dev server"

up:
	set -a && . ./.env && set +a && docker-compose up --build -d

down:
	docker-compose down

scan:
	set -a && . ./.env && set +a && docker-compose --profile tools run --rm scanner

logs:
	docker-compose logs -f api web

tidy:
	cd backend && go mod tidy

build:
	cd backend && go build -o bin/api ./cmd/api && go build -o bin/scanner ./cmd/scanner
	cd frontend && npm run build

api-dev:
	cd backend && go run ./cmd/api

web-dev:
	cd frontend && npm run dev

test:
	cd backend && go test ./...
