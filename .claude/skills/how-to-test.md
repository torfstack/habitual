---
description: Test the habitual app: run unit/integration tests, build and start Docker, then verify in browser
---

When the user invokes this skill, guide them through the full test cycle:

## 1. Run tests

```bash
mise run test
```

This runs `go test ./...` including integration tests against ephemeral testcontainers Postgres instances.

For verbose integration output:
```bash
mise run test-integration
```

## 2. Build and start Docker

```bash
docker compose up -d --build
```

Builds the multi-stage Docker image and starts both the app and Postgres containers.
The app depends on the DB healthcheck, so it won't start until Postgres is ready.

## 3. Open in browser

Navigate to: **http://localhost:8080**

Use the date navigation arrows to go back and forward in time.
Toggle habit completion, add new habits, and check that streaks display correctly.

## 4. Teardown (optional)

```bash
docker compose down
```

Add `-v` to also remove the `pgdata` volume (wipes the database).

## Notes

- `mise run generate` must be run after changing `.templ` files or SQL query files
- The `Dockerfile` is a multi-stage build: builder (Go 1.26-alpine) → runtime (Alpine 3.21)
- Integration tests use testcontainers-go and require Docker to be running
