# Hiring Challenge Backend

Standalone stripped-down backend challenge based on the production stack:

- `postgres` + Hasura metadata/migrations
- Go API with `gqlgen` resolvers (`login`, `renewToken`, `ping`)
- custom non-GraphQL upload endpoint (`/upload/image`)
- Redis container available in the stack (not used for auth/session in scaffold)
- React + Vite frontend with login, group/project management, and file explorer actions

The tasks can be found in [Challenge.md](./CHALLENGE.md)

## What Is Pre-Provisioned

The scaffold already includes:

- local runtime services (`postgres`, `redis`, `hasura`, `api`, `frontend`)
- baseline DB schema + metadata for challenge flows
- legacy login baseline for GraphQL (`login`, `renewToken`)
- a hosted ZITADEL tenant — credentials and service account key are already configured in `.env.example` and `api-backend.json`

## What Is Intentionally Not Included

The scaffold deliberately does **not** include:

- credential-based login against Zitadel for the new public API
- session management for the new public API
- code-generated REST API for auth, groups, and projects
- Go SDK / client for the public API
- OTEL/Jaeger/Prometheus stack and instrumentation

Those are part of the candidate implementation task in `CHALLENGE.md`.

## Hosted ZITADEL Configuration

A hosted ZITADEL tenant is pre-provisioned. The following values are already set in `.env.example`:

- `ZITADEL_DOMAIN`
- `ZITADEL_BROWSER_CLIENT_ID`
- `ZITADEL_REDIRECT_URI`
- `ZITADEL_SERVICE_ACCOUNT_KEY_PATH`

The service account key is in `api-backend.json` at the repo root and is mounted into the API container at runtime. No local ZITADEL containers or seed scripts are needed.

## Quick Start

1. Copy env file:

```bash
cp .env.example .env
```

2. Start everything:

```bash
make up
```

3. Open:

- Hasura console: `http://localhost:8080/console`
- Go API GraphQL: `http://localhost:8081/graphql`
- Frontend (Vite app): `http://localhost:5173`

## Seed Users

- `owner@example.com` / `password` — member of all three groups (role: owner)
- `member@example.com` / `password` — member of Core Team and Design Team (role: member)

## Regenerate Hasura Client (gqlgenc)

Because `gqlgenc` works from a local schema file in this project:

```bash
make hasura-up
make gen-hasura-client
```

This does:

1. fetch live Hasura schema into `api/internal/hasuragql/schema.graphql`
2. generate typed client into `api/internal/hasuragql/generated.go`

If you later add Hasura remote schemas that depend on the API, fetch the schema before enabling that remote schema (or ensure the API is reachable first), otherwise introspection can be incomplete.

## Useful Commands

- `make up` - start stack with build
- `make down` - stop stack and remove volumes
- `make logs` - stream logs
- `make gqlgen` - regenerate Go GraphQL server code
- `make gqlgenc` - regenerate Hasura typed client
- `make gen-hasura-client` - fetch schema + regenerate Hasura typed client
- `make api-test` - run Go tests
