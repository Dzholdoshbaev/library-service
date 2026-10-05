# Design and invariants

The service uses Gin transport DTOs → application services → focused repository
contracts → GORM/PostgreSQL. Dependencies are constructed explicitly. Services
authorize actions; repository methods implement named, atomic database operations.

## Integrity

- UUID keys, UTC `timestamptz`, normalized unique emails, enum-like CHECK constraints.
- Registration always creates an ACTIVE READER. Only the explicit CLI creates ADMINs.
- Users are soft-deleted; email uniqueness and historical comment attribution remain.
  Books/comments/interactions are hard-deleted. Book deletion cascades its children.
- Favorites and likes have composite primary keys. Book counters are nonnegative and
  change in the same transaction as their source row, only when a row changes.
- Mutation lock order is account → book → comment/token. Account shared locks make
  reader mutations linearizable with blocking/deletion/password invalidation.
  Book row locks serialize interaction mutations, including duplicate like/unlike.
- Login/rotation checks account status under a lock; password hash comparison at
  issuance prevents a concurrently replaced password from creating a new session.
- Refresh credentials are 256-bit opaque random values; only SHA-256 digests persist.
  Rotation consumes one token under a row lock, inserts a successor in the same
  family, and uses an absolute family expiry. Reuse revokes the entire family, and
  the revocation commits even though the API reports authentication failure.
- Access JWTs use HS256, issuer/audience/expiry validation, and a credential version.
  Every authenticated request reads the current account. Blocking and password
  changes increment the version and revoke refresh sessions. Logout revokes its
  refresh family; already issued access tokens expire naturally (at most 15m).

## Schema and migrations

Versioned SQL migrations (golang-migrate) create users, books, favorites, likes,
comments, refresh tokens, and indexes. GORM never creates or modifies the schema.
Indexes cover foreign keys, deterministic listing order, and token family/reuse
lookups. The initial migration is reversible on an empty or populated test schema.

## API and validation

All resources use `/api/v1`. Strict JSON rejects unknown fields and extra values.
Dedicated PATCH DTOs allow only mutable fields. List sort fields map to constant
SQL identifiers, with UUID tie-breakers; pagination is bounded. Errors and response
envelopes are centralized. Details are published in `docs/openapi.json`.

## Operations and testing

Startup validates configuration and retries PostgreSQL with a bounded deadline.
Migrations and ADMIN bootstrap are explicit CLI commands. HTTP deadlines, graceful
shutdown, bounded authentication work, CORS allowlists, request IDs, safe slog logs,
Prometheus metrics and injected OpenTelemetry tracing support operations.
Unit/HTTP tests run without a database. Integration tests use isolated schemas in
a dedicated real PostgreSQL test database and exercise concurrent mutations.
