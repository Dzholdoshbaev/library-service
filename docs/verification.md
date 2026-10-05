# Verification record

Verified on 2026-10-05 using Go 1.27.1, golangci-lint 2.14.0, and real PostgreSQL
18 in Docker. No SQLite stand-in was used.

| Check | Result |
| --- | --- |
| `go build ./...` | Passed |
| `go test ./...` | Passed (database tests explicitly skip when URL is unset) |
| `go vet ./...` | Passed |
| `go test -race -count=1 ./...` with `TEST_DATABASE_URL` | Passed, including all PostgreSQL integration tests |
| `golangci-lint run` | Passed, 0 issues |
| `gofmt -l cmd internal db docs` | No unformatted files |
| Migration from empty schema; down/up; repeated up | Passed on PostgreSQL |
| Unique/FK/CHECK constraints, UUIDs, cancellation and rollback | Passed |
| Concurrent duplicate/distinct likes and comment counters | Passed |
| Concurrent refresh reuse, expiry and password invalidation | Passed |
| Complete httptest reader/ADMIN lifecycle with real PostgreSQL | Passed |
| OpenAPI/router agreement and schema references | Passed for all 30 operations |
| `docker compose build` | Passed with final source |
| `docker compose up -d --wait` | API and PostgreSQL healthy, migration job succeeded |
| CLI ADMIN bootstrap in dedicated test database | Passed with generated credentials |
| `scripts/smoke.sh` against final image | Passed reader/ADMIN curl flows, blocking and counters |
| `/health/ready` and `/docs` on development deployment | HTTP 200 |
| SIGTERM of verification container | Graceful shutdown logged; container stopped cleanly |

The temporary verification container, generated test accounts and credential file
were removed. The development deployment remains running on localhost:18080 with
PostgreSQL on localhost:55432, configured by the ignored `.env`. No default ADMIN
was provisioned in the development database. Follow README bootstrap instructions
to create your own administrator.
