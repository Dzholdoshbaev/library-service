# Library Service

A Go REST backend with reader and administrator roles, PostgreSQL integrity,
explicit dependency injection, rotating refresh credentials, and concurrent book
interactions. Gin handles HTTP, application services enforce policy, and GORM
implements focused PostgreSQL operations. The module targets Go 1.27 and pins the
Go 1.27.1 toolchain; see [Go releases](https://go.dev/doc/devel/release).

## Start locally with Docker

Prerequisites: Docker with Compose. Local Go development also needs Go 1.27+.
The API has no default administrator or shipped password.

```sh
cp .env.example .env                 # only if .env does not already exist
chmod 600 .env
# Put independent random values into DB_PASSWORD and JWT_ACCESS_SECRET.
# Generate each value with: openssl rand -hex 32
make docker-up
```

Compose starts PostgreSQL 18, runs the embedded golang-migrate migrations in a
one-shot `migrate` container, then starts the non-root API. PostgreSQL has persistent
storage. The API root filesystem is read-only and capabilities are dropped. Compose
binds database/API ports to localhost. `make docker-down` retains the volume.

The checked-out development environment may already have a generated, ignored
`.env`. This implementation's verified local environment uses API port **18080**
and PostgreSQL port **55432** to avoid conflicts. `.env.example` defaults to
8080/5432. Never copy local development secrets into production.

- Swagger UI: `http://localhost:18080/docs` for the prepared local environment;
  default installation: `http://localhost:8080/docs`.
- OpenAPI JSON: `/docs/openapi.json` (development/test only); checked-in contract:
  [docs/openapi.json](docs/openapi.json).
- Liveness: `/health/live`; readiness with PostgreSQL ping: `/health/ready`.
- Prometheus metrics: `/metrics`, requiring an ADMIN Bearer token.

Swagger UI uses pinned public CDN assets and needs Internet access; the OpenAPI
JSON is embedded and works offline. Both docs routes are disabled in production.

## Initial administrator

An explicit `bootstrap-admin` command is the only administrator provisioning path.
It creates a new account; it never upgrades an existing reader or changes an
existing password. A duplicate normalized email fails. Supply credentials through
your secret manager or temporary environment. For an interactive Bash session:

```bash
export ADMIN_EMAIL='admin@example.com'
export ADMIN_FIRST_NAME='Library'
export ADMIN_LAST_NAME='Administrator'
export ADMIN_BIRTH_YEAR=1990
read -rsp 'New admin password (12+ characters): ' ADMIN_PASSWORD; echo
export ADMIN_PASSWORD
docker compose run --rm --no-deps   -e ADMIN_EMAIL -e ADMIN_FIRST_NAME -e ADMIN_LAST_NAME   -e ADMIN_BIRTH_YEAR -e ADMIN_PASSWORD library-service bootstrap-admin
unset ADMIN_PASSWORD
```

There is no public role field. Unknown JSON fields such as `role`, `status`,
`reader_id`, `likes_count` and `password_hash` are rejected, rather than bound to
persistence entities. Use a separate administrative identity for operations.

## Development without an API container

Start just PostgreSQL, then export your local configuration:

```sh
docker compose up -d postgres
set -a
. ./.env
set +a
make migrate-up
make run
# Or: make build && ./bin/library serve
```

Set `DB_HOST=localhost` and `DB_PORT` to the published database port. Compose
internally overrides the database host/port to `postgres:5432`. Environment files
are not implicitly read by the Go binary. Production configuration errors fail
startup with variable names, without printing secret values.

## Architecture and project structure

```text
cmd/library/main.go        signal handling and CLI entrypoint
internal/app/              dependency wiring, server lifecycle, migration runner
internal/config/           validated environment configuration
internal/domain/           internal entities, principals, filters, typed errors
internal/security/         Argon2id passwords, JWTs, random refresh credentials
internal/service/          auth, user, book, interaction and comment use cases
internal/store/            GORM PostgreSQL queries and named atomic operations
internal/httpapi/          explicit DTOs, handlers, validation and middleware
internal/telemetry/        private metrics registry, SQL timing, OTel provider
internal/integration/      real PostgreSQL migrations, HTTP and concurrency tests
db/migrations/             embedded versioned SQL up/down files
docs/                      design notes, OpenAPI contract, Swagger page
scripts/                   contract generator and executable curl flow
.github/workflows/ci.yml   build, vet, race/integration, lint and Docker build
Dockerfile                 multi-stage, non-root runtime
docker-compose.yml         PostgreSQL, migration job and API
Makefile                   development and verification commands
.env.example               configuration template, no credentials
```

Request flow is **handler → service → repository → GORM → PostgreSQL**. Handlers
bind/validate and map explicit responses. Services enforce roles, ownership,
normalization and domain validation. Repositories implement database-specific
locking, queries and transactional mutations. Constructors receive explicit
repositories, password hasher and token manager. Interfaces exist at the service
persistence boundary and authentication middleware for testing; there is no
service locator, DI framework, generic base repository or global DB/logger.

Internal entities carry intentional GORM column/key metadata. They are never
serialized directly. `Comment.Reader` is the only loaded association: a bounded
explicit preload selecting safe attribution fields, giving two queries per comment
list instead of N+1. Book lists read counters without loading likes/comments.

## Database model and deletion

| Table | Integrity and purpose |
| --- | --- |
| `users` | UUID key, unique normalized email, role/status CHECKs, profile bounds, password hash, credential version, timestamps, nullable deletion timestamp |
| `books` | UUID key, bounded title/author/year/volume, nonnegative likes/comment counters |
| `reader_favorites` | Composite primary key `(reader_id, book_id)`; idempotent membership |
| `book_likes` | Composite primary key `(reader_id, book_id)`; one like per reader |
| `comments` | UUID key, foreign keys, nonblank bounded content, timestamps |
| `refresh_tokens` | UUID key, unique SHA-256 token fingerprint, user/family keys, absolute expiry, consumed/revoked timestamps |

Application-generated cryptographic UUIDs need no database extension. All times
are UTC `timestamptz`. Foreign keys and CHECK constraints are part of integrity;
indexes cover referencing columns, chronological pagination, publication year,
reader administration, token families and token expiry. Substring search uses
bound `ILIKE` arguments with escaped wildcard characters; for a large catalogue,
add PostgreSQL trigram indexes through a measured migration.

Reader deletion is an explicit tombstone (`deleted_at`); GORM automatic soft-delete
scopes are intentionally not used. Every account lookup excludes tombstones. Email
remains reserved and authentication is revoked. Existing likes/comments remain in
counters and preserve history; comment DTOs mask deleted names as `Deleted reader`.
This is not a legal erasure workflow; implement retention/anonymization policy for
your deployment. Books/comments are hard-deleted. Deleting a book cascades favorites,
likes and comments. User foreign keys restrict hard deletion of historical content.

## Migrations and GORM

`AutoMigrate` is not used. SQL migrations are reviewed, versioned, reversible and
embedded in the binary. `golang-migrate` manages versions and PostgreSQL advisory
locking. Run migrations as a separate deployment step before scaling the API;
starting `serve` never silently changes the schema.

```sh
make migrate-up
make migrate-down                  # one version down; initial down deletes tables
make migrate-create name=add_index # creates the next numbered up/down pair
# With Compose:
docker compose run --rm --no-deps migrate migrate up
```

Do not edit already-applied migrations. If a migration becomes dirty, inspect and
repair the database before deliberately fixing its version with golang-migrate;
the application does not automatically force versions. Runtime and migration
credentials can be distinct in production; grant runtime DML only.

Every database operation propagates the request context with `WithContext`.
PATCHes use allowlisted explicit maps, so zero values and omitted fields are
intentional. No `Save`, unrestricted `Order`, generic association preload, or
SQLite substitution is used. Known constraint errors become typed API errors;
SQL errors and parameter values never reach clients or application logs.

## Authentication, roles and account blocking

1. Register an ACTIVE READER with a normalized email and Argon2id password hash.
2. Login verifies the password and current status. Unknown accounts, wrong
   passwords and blocked accounts all receive `INVALID_CREDENTIALS`.
3. Receive a short access JWT and a 256-bit random opaque refresh credential.
4. Send `Authorization: Bearer <access>` for protected routes.
5. Refresh rotates the credential transactionally. Clients must serialize refresh
   attempts and replace the old credential. Reusing a consumed credential revokes
   every token in its family, including a concurrently issued successor.
6. Logout revokes that refresh family. It is idempotent. Existing access JWTs remain
   usable for their remaining short lifetime. Password changes and blocking revoke
   access immediately through the account credential version.

JWT validation pins HS256 and checks issuer, audience, issued-at, expiration,
subject UUID, role and credential version. Claims contain no profile/secret data.
Refresh tokens are not JWTs: only a SHA-256 fingerprint is stored, so
`JWT_REFRESH_SECRET` is intentionally unnecessary. Tokens have 256 bits of entropy,
so password-style slow hashing is unnecessary for their storage. A family has an
absolute expiry (default 7 days); rotation does not extend it. Keep consumed rows
until family expiry to detect reuse, then prune expired rows with a scheduled
maintenance job according to retention policy.

Argon2id uses a fresh 16-byte salt, 64 MiB, 3 iterations, 2 lanes and a 32-byte key.
Password length is 12+ characters, at most 128 UTF-8 bytes. Hash verification rejects
unrecognized or unbounded parameters and compares in constant time. An auth work
semaphore bounds Argon2 CPU/memory pressure; a busy instance returns 429. Configure
per-IP/account rate limits at the gateway across instances. No proxy is trusted
implicitly. Passwords, hashes, token strings, headers and request bodies are never
logged. Use TLS and a secret manager in production.

Each authenticated request reads the current account, which enforces blocking and
credential revocation immediately instead of trusting a previously issued JWT.
This costs one indexed PostgreSQL lookup per request; no cache delays revocation.
Reader mutations also hold an account SHARE lock and recheck status/version until
commit. Blocking/deletion/password changes lock the account exclusively, so a
mutation either completes before the block or is rejected after it. Unblocking
requires a fresh login; old tokens never become valid again.

Middleware handles coarse RBAC; services check comment ownership. ADMIN manages
books/readers and can delete any comment. READER manages their own favorites,
likes and comments. ADMIN does not inherit reader interaction permissions.

## Transactions and concurrency

| Operation | Transaction boundary |
| --- | --- |
| Like/unlike | Lock account then book; insert with `ON CONFLICT DO NOTHING` or delete; update counter atomically only if a row changed |
| Favorite/unfavorite | Lock account then book; insert/delete composite-key membership idempotently |
| Create/delete comment | Lock account then book; insert/delete comment plus atomic counter change |
| Edit comment | Account/book/comment locks, ownership recheck, explicit content update |
| Refresh | Discover account, lock account then token, consume token and create successor; commit family revocation on reuse |
| Block/delete/password change | Exclusive account lock, credential version increment and refresh revocation |

PostgreSQL protects invariants across instances; in-process mutexes do not protect
business state. Nonnegative CHECKs detect corruption and roll back a failed
mutation instead of hiding it. Per-book locks favor simple correctness at the
cost of serializing writes to a very popular book. Lock order is account → book →
comment/token. Read-only lists do not use transactions. COUNT and page reads can
observe intervening committed writes; pagination is a bounded offset view with
UUID tie-breaking, not a snapshot cursor.

## API endpoints

All application routes below are under `/api/v1`.

| Method | Path | Access |
| --- | --- | --- |
| POST | `/auth/register` | Public; READER only |
| POST | `/auth/login` | Public |
| POST | `/auth/refresh` | Refresh credential |
| POST | `/auth/logout` | Refresh credential |
| GET, PATCH | `/me` | ADMIN, READER |
| PATCH | `/me/password` | ADMIN, READER; current password required |
| GET | `/admin/readers` | ADMIN |
| GET, PATCH, DELETE | `/admin/readers/:id` | ADMIN |
| PATCH | `/admin/readers/:id/block` | ADMIN |
| PATCH | `/admin/readers/:id/unblock` | ADMIN |
| GET | `/books` | ADMIN, READER |
| POST | `/books` | ADMIN |
| GET | `/books/:id` | ADMIN, READER |
| PATCH, DELETE | `/books/:id` | ADMIN |
| POST, DELETE | `/books/:id/favorite` | READER |
| GET | `/me/favorites` | READER |
| POST, DELETE | `/books/:id/like` | READER |
| GET | `/books/:id/comments` | ADMIN, READER |
| POST | `/books/:id/comments` | READER |
| PATCH | `/books/:id/comments/:commentId` | READER owner |
| DELETE | `/books/:id/comments/:commentId` | READER owner or ADMIN |

Lists accept `page` (default 1, maximum 1,000,000), `page_size` (default 20,
maximum 100), `sort`, and `order=asc|desc` (default `created_at desc`). Readers
support `status`, `first_name`, `last_name`, `email`; books support `title`,
`author`, `publication_year`. See OpenAPI for each route's sort allowlist.
Malformed, repeated and invalid pagination query values are rejected. Unknown
query names are ignored. Searches are literal case-insensitive substrings.

Single responses use `{"data":{...}}`; lists use
`{"data":[],"pagination":{"page":1,"page_size":20,"total":0,"total_pages":0}}`.
Errors use `{"error":{"code":"BOOK_NOT_FOUND","message":"book not found"}}`;
validation adds field-specific `fields`. Invalid/missing authentication is 401,
permission denial is 403, missing resources are 404, duplicate email is 409,
invalid fields are 422, and unexpected failures are redacted 500s.

Creates return 201. Idempotent favorite/like writes and deletions return 204.
Malformed or unknown JSON fields return 400. JSON PATCH null/omission leaves a
field unchanged; an empty object is a no-op. Empty middle-name text clears the
visible name. Optional volume can be set on create/update but null in PATCH does
not clear it. Password changes require their separate DTO and invalidate every
existing login.

## Configuration

| Variable | Default / rule |
| --- | --- |
| `APP_ENV` | `development`; `production` or `test` also supported |
| `HTTP_HOST`, `HTTP_PORT` | `0.0.0.0`, `8080` |
| `DB_HOST`, `DB_PORT`, `DB_NAME`, `DB_USER` | `localhost`, `5432`, `library`, `library` |
| `DB_PASSWORD` | Required, no default |
| `DB_SSLMODE` | `disable` locally; production requires `verify-full` |
| `DB_SSLROOTCERT` | Optional CA file path; mount into production container |
| `DB_MAX_OPEN_CONNS` | 25; per-instance budget, size for database capacity |
| `DB_MAX_IDLE_CONNS` | 10; cannot exceed open limit |
| `DB_CONN_MAX_LIFETIME` | 30m; recycle long-lived connections |
| `DB_CONN_MAX_IDLE_TIME` | 5m; release idle capacity |
| `JWT_ACCESS_SECRET` | Required, at least 32 bytes; generate cryptographically |
| `JWT_ISSUER`, `JWT_AUDIENCE` | `library-service`, `library-api` |
| `JWT_ACCESS_TTL` | 15m; supported 1m–30m |
| `JWT_REFRESH_TTL` | 168h; supported 1h–90d; absolute family lifetime |
| `AUTH_MAX_CONCURRENCY` | 4; up to ~256 MiB Argon2 working memory at default |
| `LOG_LEVEL` | `info`; debug/info/warn/error |
| `CORS_ALLOWED_ORIGINS` | Empty denies cross-origin requests; explicit comma-separated origins only |
| `SHUTDOWN_TIMEOUT` | 15s; supported 1s–1m |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | Optional OTLP/HTTP collector URL |
| `GIN_MODE` | Set `release` for production; Compose sets it |
| `POSTGRES_HOST_PORT` | Compose-only host database port, default 5432 |
| `TEST_DATABASE_URL` | Test-only PostgreSQL URL; database name must end in `_test` |
| `ADMIN_*` | Bootstrap-only identity/password variables shown above |

Pool defaults suit a small service; budget total connections across all replicas
and leave room for migrations/operations. Configuration builds a properly escaped
PostgreSQL URL from individual fields, so punctuation in passwords is supported.
`APP_ENV=production` requires server-verified database TLS. Local Compose is a
local development deployment; use managed TLS PostgreSQL and secret injection in
production, plus HTTPS termination, network policy, gateway rate limits and backups.

## Observability and lifecycle

`log/slog` emits text in development and JSON elsewhere. Request logs contain a
validated/generated UUID request ID, method, route template, status, duration,
and authenticated user ID when available. Route templates avoid query strings and
raw path identifiers. Panics are recovered without logging their arbitrary values
or stack traces. GORM's SQL formatter is never evaluated by the production logger,
preventing SQL parameters from disclosing hashes/tokens. Error responses contain
no internal errors. Correlate failures with request IDs and infrastructure metrics.

Each application owns a Prometheus registry and OpenTelemetry tracer provider;
there is no global telemetry configuration. Metrics include request counts,
durations, status codes, DB operation durations/errors, Go/process metrics and pool
usage. Labels use route templates and bounded methods, never user IDs. Tracing
propagates W3C trace context and exports OTLP/HTTP when configured (10% root
sampling); without an endpoint, spans are unsampled. Traces exclude credentials.

The explicit HTTP server uses 5s header, 10s read, 15s write and 60s idle timeouts,
a 16 KiB header ceiling and 1 MiB body limit. Request DB work has a 10s context
deadline. Startup retries PostgreSQL for up to 30s with 3s ping attempts. Readiness
pings PostgreSQL within 2s. SIGINT/SIGTERM stops accepts and drains requests within
15s, then closes the pool and shuts down telemetry. Configure the orchestrator's
termination grace period above these budgets (Compose uses 25s).

## Tests and checks

```sh
make build
make test
make test-race
make vet
# Install once (or use your organization's pinned tool installation):
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.14.0
make lint
```

Unit/httptest tests require no database. Repository and end-to-end HTTP tests
**skip explicitly** unless `TEST_DATABASE_URL` is set. No SQLite behavior is used.
To run the complete suite against the Compose PostgreSQL instance:

```sh
docker compose exec -T postgres sh -c 'createdb -U "$POSTGRES_USER" library_test'
# Export a URL for this dedicated DB using your local credentials/port.
# Example shape: postgres://USER:PASSWORD@localhost:5432/library_test?sslmode=disable
export TEST_DATABASE_URL='YOUR_DEDICATED_TEST_DATABASE_URL'
make test-integration
make test-race
```

Each integration test creates a random private schema, applies migrations from
empty state, then drops only that schema. Tests assert UUID mappings, uniqueness,
foreign keys, counter rollback, account tombstones, cascade deletion, context
cancellation, migration down/up, concurrent duplicate/distinct likes, comments,
refresh races and the full HTTP lifecycle. CI always supplies real PostgreSQL and
runs the race suite. OpenAPI tests compare registered routes and schema references;
regenerate the contract with `python3 scripts/generate_openapi.py` after intentional
API changes.

See [docs/curl-examples.md](docs/curl-examples.md) for the complete executable reader
and administrator flows, and [docs/design.md](docs/design.md) for the invariants.

The completed build/deployment verification is recorded in [docs/verification.md](docs/verification.md).
