CREATE TABLE public.api_sessions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  token_hash text NOT NULL UNIQUE, -- sha256 hex of raw session token
  user_id uuid NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
  active_group_id uuid NOT NULL,
  expires_at timestamptz NOT NULL,
  revoked_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT NOW(),
  last_seen_at timestamptz NOT NULL DEFAULT NOW(),

  CONSTRAINT api_sessions_token_hash_hex CHECK (token_hash ~ '^[0-9a-f]{64}$'),
  CONSTRAINT api_sessions_expires_after_create CHECK (expires_at > created_at),
  CONSTRAINT fk_api_sessions_active_membership
    FOREIGN KEY (user_id, active_group_id)
    REFERENCES public.group_memberships(user_id, group_id)
    ON DELETE CASCADE
);

CREATE INDEX idx_api_sessions_user_id ON public.api_sessions(user_id);
CREATE INDEX idx_api_sessions_active_group_id ON public.api_sessions(active_group_id);
CREATE INDEX idx_api_sessions_expires_at ON public.api_sessions(expires_at);
CREATE INDEX idx_api_sessions_unrevoked_expires
  ON public.api_sessions(expires_at) WHERE revoked_at IS NULL;
