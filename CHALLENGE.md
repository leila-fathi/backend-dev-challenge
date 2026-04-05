# Candidate Assignment

## Goal

Extend this scaffold with a public REST API backed by Zitadel authentication and a companion Go SDK, while preserving the existing stack and frontend.

## Existing Baseline (Do Not Break)

- Go GraphQL API (`login`, `renewToken`, `ping`)
- Upload endpoint (`POST /upload/image`)
- Hasura GraphQL engine with metadata, permissions, and full CRUD capabilities
- React + Vite frontend (login, group/project management, file explorer)

## Pre-Provisioned

- A hosted ZITADEL tenant — credentials and service account key are already configured in `.env.example` and `api-backend.json`

## Required Work

### 1) Public REST API via Code Generation

Implement a new REST API (separate from the existing GraphQL API) using **any** server stub code generation tool. Acceptable options include but are not limited to:

- [oapi-codegen](https://github.com/oapi-codegen/oapi-codegen) (OpenAPI)
- [goa](https://github.com/goadesign/goa) (DSL-based)
- any other code generation approach that produces typed Go server interfaces

Required endpoints:

- `POST /api/v1/auth/login` — authenticate user credentials against Zitadel, establish a session
- `POST /api/v1/auth/switch-group` — switch the active group for the authenticated caller; enforce group membership; return refreshed auth context
- `GET /api/v1/auth/memberships` — list the caller's group memberships, indicating which group is currently active
- `GET /api/v1/projects` — list projects for the active session group
- `POST /api/v1/projects` — create a project in the active session group

### 2) Authentication with Zitadel

Implement credential-based authentication for the public API:

- Verify user credentials against Zitadel (service account key and client configuration are pre-provisioned)
- Handle the full login flow including any edge cases you identify in the existing data model
- Define any required schema changes yourself

### 3) Session Management

- The backend must maintain sessions for authenticated users
- Sessions must support group context (active group) to enable group switching and future features
- The API must be stateless and HA-ready — it must behave correctly with multiple API replicas behind a load balancer

### 4) Database Access

- Use Postgres directly for all public API data access
- Any Go Postgres adapter is acceptable (`database/sql`, `pgx`, `sqlc`, etc.)
- **No ORM** (no GORM, no ent, no similar)
- Preserve existing DB model semantics (`nodes`, `folders`/`images` views, `image_data`, closure table)

### 5) Go SDK

Provide a Go SDK module for the public API:

- Generate a typed HTTP client from the same API definition used for the server
- Provide a higher-level SDK that is ergonomic to use
- Include a runnable CLI or example that exercises the full flow: login → list memberships → switch group → list projects → create project → list projects

### 6) Observability and Testing

- Wrap the HTTP server with OpenTelemetry instrumentation
- Add trace spans in request handlers
- Use structured logging (e.g., `slog` with JSON output)
- Set up a base testing framework and add some tests (dont need to cover everything!)

## Constraints

- Do not touch frontend code — it must keep working as-is
- Do not remove the existing legacy login flow (GraphQL `login`/`renewToken`)
- Hasura and its metadata/permissions must remain functional

---

## Bonus Tasks (Optional)

### B1) Monitoring Stack

Add and configure a local observability backend in Docker Compose:

- Metrics (e.g., Prometheus)
- Logs aggregation
- Distributed traces (e.g., Jaeger, Tempo)
- Wire the OTEL SDK exporter to the collector

### B2) Image Uploads

Add an image upload endpoint to the public API:

- `POST /api/v1/images` — upload an image file, create a node (`node_type = 'image'`) and associated `image_data` record
- Respect the existing DB constraints (project/group consistency, closure table)

---

## Bonus Questions (Written Answers Only — Do Not Implement)

Answer these in a `ANSWERS.md` file:

1. **Token-based auth**: The current API uses server-side sessions. How would you expand it to also support token-based authentication (e.g., personal access tokens or service account tokens) for programmatic API clients? Describe the auth flow, token storage, validation changes, and how both auth methods would coexist.

2. **Role-based access control**: A new requirement says project access must be role-based. There are managers and members with different permission levels. How would you introduce a scalable RBAC system to the API such that resource-level and scope-based access control can be added incrementally? Describe the data model, enforcement approach, and how it would interact with the existing group membership model.

3. **Frontend migration**: The frontend currently uses the Hasura GraphQL API and the legacy Go GraphQL API. What would be required to migrate the frontend to use the new public REST API instead? Describe the scope of changes, migration strategy, and any API gaps that would need to be addressed.
