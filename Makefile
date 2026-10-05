GO ?= go
.PHONY: run build test test-race test-integration vet lint fmt migrate-up migrate-down migrate-create docker-up docker-down docker-logs
run:
	$(GO) run ./cmd/library serve
build:
	$(GO) build -trimpath -o bin/library ./cmd/library
test:
	$(GO) test ./...
test-race:
	$(GO) test -race ./...
test-integration:
	@test -n "$(TEST_DATABASE_URL)" || (echo 'Set TEST_DATABASE_URL to a dedicated database ending in _test'; exit 1)
	$(GO) test -race -count=1 ./internal/integration
vet:
	$(GO) vet ./...
lint:
	golangci-lint run
fmt:
	gofmt -w cmd internal db docs
migrate-up:
	$(GO) run ./cmd/library migrate up
migrate-down:
	$(GO) run ./cmd/library migrate down
migrate-create:
	@test -n "$(name)" || (echo 'Usage: make migrate-create name=add_index'; exit 1)
	$(GO) run github.com/golang-migrate/migrate/v4/cmd/migrate create -ext sql -dir db/migrations -seq $(name)
docker-up:
	docker compose up --build -d --wait
docker-down:
	docker compose down
docker-logs:
	docker compose logs -f library-service postgres migrate
