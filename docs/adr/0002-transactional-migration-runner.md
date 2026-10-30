# ADR 0002 — Transactional migration runner, profile-first adoption

Accepted 2026-10-30. Branch platform-migrations-v1.

Choose a small shared Go runner using the existing pgx/PostgreSQL connection, rather than independent startup
DDL in every new module. No new service or migration library is necessary. Code lives in internal/migrate;
module-owned SQL is embedded from internal/coreprofile/migrations. Drive never imports platform.

Decisions implemented:

- A module has a stable ID, contiguous step IDs starting at 1, immutable names/SQL and SHA-256 checksums.
- Validate the registered DAG, missing dependencies, version bounds and cycles before touching the database.
  Dependencies use inclusive minimum/exclusive maximum integer schema versions; package semver is separate.
- One transaction holds advisory lock 681024 for all registered modules and writes platform_migrations.
  Lock timeout 30s, statement timeout 120s. Concurrent Desktop/Drive startups serialize safely.
- Validate all registered module history before pending SQL. Reject checksum/name drift, missing IDs,
  newer schema or newer runner history. Unregistered module histories are left untouched for independent services.
- DDL and applied ledger entries commit together. Error, cancellation or connection loss rolls them back.
  The ledger records applied only; failed attempts are logged, not persisted as misleading applied entries.
- This startup runner is for reviewed additive transactional SQL. It is not a SQL sandbox: only trusted embedded
  migration code is allowed. No plugin-supplied SQL, automatic down migrations, destructive contract steps,
  nontransactional index jobs or long backfills. Those require a separately reviewed operator workflow.

First adoption is profile schema v1. Existing user_profiles is verified for required columns, PK, identity FK,
validated locale/theme constraints and supported values; data is never rewritten/deleted. Users table must
already exist through the unchanged legacy identity bootstrap. Profile migration fails closed if incompatible.
CREATE TABLE IF NOT EXISTS alone is not treated as successful adoption verification.

The generic DAG runner exists, but the full module plan in contracts/migrations/modules.v1.yaml remains a
future target. Profile has an explicit external precondition on legacy users instead of pretending identity
has already adopted a module ledger. Legacy schema_version and platform/Drive identity DDL remain unchanged.
Do not add fictional identity ledger rows. Next adoption must verify users/sessions and preserve stable IDs.

Why transaction-per-plan first: this initial expand step is small, PostgreSQL DDL is transactional, and atomic
failure is easier to recover from. Long backfills will need checkpointed jobs and short separate commits;
this runner does not claim to solve those. A committed failed destructive operation cannot be undone by a down script.

Testing uses a dedicated PostgreSQL container/network with no host port and no production credentials.
Tests cover concurrency, drift, dependency order, old binaries, SQL failure, cancellation, backend termination,
fresh profile and existing-data adoption. Backup/restore rehearsal is separate from transaction rollback tests.


Compatibility limit: the schema-version guard applies to binaries using this runner. Pre-runner legacy binaries
cannot retroactively enforce the ledger; deployment/rollback checks must prevent incompatible legacy releases.
Transaction tests and DB restore rehearsal do not certify filesystem-volume recovery or destructive migrations.
