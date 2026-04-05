# ANSWERS

## 1) Token-based auth: support API tokens alongside sessions

Current state: the API accepts a Zitadel bearer token at login and then creates a server-side session token.  
For programmatic clients, first-class API tokens can be added while keeping sessions unchanged.

### Auth methods
1. Session auth (existing): browser user flows.
2. Personal Access Tokens (PAT): user-owned tokens for CLI/scripts.
3. Service account tokens: non-human automation identity.

### Token storage and schema
Add `public.api_tokens` with fields such as:
- `id` (uuid)
- `token_hash` (unique, hash only; raw token never stored)
- `token_prefix` (optional for UI/debug)
- `token_type` (`pat` or `service`)
- `principal_id` (user/service account id)
- `active_group_id` (nullable or fixed by policy)
- `scopes` (jsonb or text[])
- `expires_at`, `revoked_at`, `last_used_at`
- `created_at`, `created_by`, `name`

Optional hardening tables:
- `api_token_audit_logs`
- `api_token_group_grants` (if token can access multiple groups)

### Validation changes
Use one Bearer header and resolve token kind in middleware:
1. Validate as existing session token (`api_sessions`).
2. If not a valid session token, validate as API token (`api_tokens`).
3. Build a unified auth context for handlers (`principal`, `auth_method`, `group`, `scopes`).

### Coexistence model
- No breaking change for frontend users on sessions.
- Programmatic clients use PAT/service tokens.
- Same endpoints can support both methods.
- Authorization checks become scope-based plus group-based.
- `switch-group` policy:
1. Allowed for sessions by default.
2. For API tokens, allowed only when scope and target-group grant are present.

### Security controls
- High-entropy random tokens, shown once at creation.
- Hash-at-rest, rotation, revocation, expiration.
- Last-used timestamp and audit logs.
- Rate limiting and anomaly detection on token usage.

---

## 2) RBAC design for managers/members with incremental rollout

The current model already includes group membership roles.  
This can be evolved into explicit RBAC with permissions and bindings.

### Data model
Add RBAC tables:
- `roles` (`id`, `name`, `scope_type`: group/project/global)
- `permissions` (`id`, `key`) such as `project.read`, `project.create`, `project.delete`
- `role_permissions` (`role_id`, `permission_id`)
- `principal_role_bindings` (`principal_type`, `principal_id`, `role_id`, `group_id`, `project_id`, timestamps)

Keep existing `group_memberships`, but map legacy roles into RBAC bootstrap roles:
- `owner` -> broad management permissions
- `member` -> limited read/write permissions

### Enforcement approach
- Add an authorization layer used by handlers: `Can(principal, action, resource)`.
- Resolve permissions from bindings scoped to resource context (group/project).
- Deny by default.
- Keep policy checks at endpoint/service boundaries.

### Incremental strategy
1. Introduce RBAC schema and seed default roles/permissions.
2. Add compatibility mapping from current owner/member roles.
3. Enforce RBAC first on project endpoints.
4. Expand progressively to folder/image/node resources.
5. Add caching only if performance requires it; DB remains source of truth.

### Scalability
- New permissions can be added without schema redesign.
- Resource-level checks fit naturally via scoped bindings.
- Model works for both human users and service tokens.

---

## 3) Frontend migration from GraphQL to new REST API

Current frontend depends on Hasura GraphQL plus legacy Go GraphQL endpoints.  
Migration should be phased to avoid a big-bang rewrite.

### Scope of frontend changes
- Replace GraphQL calls with REST client calls.
- Update auth store to use REST login/session flow.
- Update group switching and project management calls.
- Replace file explorer and node/image flows currently backed by Hasura GraphQL.

### Migration plan
1. Generate TypeScript REST client from OpenAPI and add an API adapter layer.
2. Migrate auth screens/flows first (`login`, `memberships`, `switch-group`).
3. Migrate project list/create screens.
4. Migrate file explorer features once corresponding REST endpoints exist.
5. Keep GraphQL fallback during transition using feature flags.
6. Remove GraphQL dependencies after parity and E2E pass.

### API gaps before full migration
- Nodes/folders/images CRUD endpoints equivalent to current Hasura usage.
- Tree traversal and move operations for explorer workflows.
- Upload integration endpoint(s) with metadata linkage.
- Filtering/pagination/sorting where frontend expects it.
- Any real-time/subscription behavior currently provided by GraphQL (if required).

### Risk control
- Contract tests between frontend and REST schema.
- Side-by-side rollout per feature area.
- Regression E2E tests for login, group switch, project flows, explorer actions.
