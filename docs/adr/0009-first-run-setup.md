# ADR 0009 — First-run setup: System Admin first, then the first organization

Status: accepted (2026-10-31)

## Context

Until now a fresh install created the account `admin` automatically, with the bootstrap secret as its password,
and adopted it into a default organization named *MinhAnOS*. The owner never chose the administrator's name or
password, and the first organization (ADR 0008) had a fixed name.

## Decision

- A fresh install (no row in `users`) creates **no account**. `GET /api/setup` answers `{"required":true}` and
  Desktop shows a two-step setup instead of sign-in: (1) create the System Admin (setup code, display name,
  username, password twice), (2) name the first organization. Both are sent in one `POST /api/setup`, then the
  browser signs in with the new account.
- The **setup code** is the operator-held secret `secrets/bootstrap_password` (compared in constant time, rate
  limited like sign-in, Origin-checked like every write). Nobody else on the LAN can claim a fresh install. The new
  password must differ from the code: the code is never a sign-in password.
- One transaction under the startup advisory lock (681021): re-check that no account exists, create the account
  with `role='admin'` (System Admin), rename the default organization (ADR 0008) to the chosen name, make the
  account its admin and a host operator (Files *System* root, like the former bootstrap admin), write the audit event `setup.completed` with that organization. Concurrent attempts: exactly
  one succeeds, the others get 409 `SETUP_DONE`. Once any account exists setup is closed for good.
- Errors: 403 `SETUP_CODE_INVALID`, 400 `SETUP_INVALID_USERNAME` / `SETUP_INVALID_NAME` / `SETUP_WEAK_PASSWORD` /
  `SETUP_INVALID_ORGANIZATION`, 409 `SETUP_DONE`, 503 `SETUP_UNAVAILABLE` (no setup code configured), 429.
- Unattended installs set `MINHANOS_BOOTSTRAP_ADMIN=true` to keep the old behaviour (account `admin`, password =
  bootstrap secret): Go integration tests, disposable stacks and standalone Files (`compose.drive.yaml`, which has
  no setup screen). Existing installs are unaffected (they already have accounts).

## Consequences

- `tests/setup-smoke.mjs` runs first on a fresh install; the other smoke tests then use the admin it created
  (`TEST_PASSWORD_FILE` must differ from the setup code). `tests/a11y-browser.mjs` audits the setup screen when the
  install is not set up yet.
- Standalone Files on port 8082 cannot set up a Desktop install; open Desktop (8081) first.
