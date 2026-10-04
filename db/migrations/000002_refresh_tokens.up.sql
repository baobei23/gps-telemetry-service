CREATE TABLE IF NOT EXISTS refresh_tokens (
    jti UUID PRIMARY KEY,
    user_id UUID NOT NULL references users(id) ON DELETE CASCADE,
    expires_at timestamptz NOT NULL,
    created_at timestamptz DEFAULT now()
);
