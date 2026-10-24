CREATE TABLE users (
    id text PRIMARY KEY,
    email text NOT NULL UNIQUE,
    password_hash bytea NOT NULL,
    role text NOT NULL DEFAULT 'user' CHECK (role IN ('user', 'admin')),
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE auth_sessions (
    token_hash bytea PRIMARY KEY,
    user_id text NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX auth_sessions_user ON auth_sessions(user_id);
CREATE INDEX auth_sessions_expiry ON auth_sessions(expires_at);
CREATE TABLE auth_attempts (
    email text PRIMARY KEY,
    attempts integer NOT NULL,
    reset_at timestamptz NOT NULL
);
CREATE TABLE player_data (
    user_id text PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    profile jsonb NOT NULL,
    state jsonb NOT NULL DEFAULT '{}',
    version bigint NOT NULL DEFAULT 0,
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE games (
    id text PRIMARY KEY,
    user_id text NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    mode text NOT NULL,
    status text NOT NULL CHECK (status IN ('active', 'completed', 'abandoned')),
    state jsonb NOT NULL,
    record jsonb,
    started_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    completed_at timestamptz
);
CREATE INDEX games_user_history ON games(user_id, started_at DESC, id DESC);
CREATE UNIQUE INDEX games_one_active_per_user ON games(user_id) WHERE status = 'active';
CREATE TABLE game_events (
    game_id text NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    sequence bigint NOT NULL,
    action jsonb NOT NULL,
    view jsonb NOT NULL,
    state jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (game_id, sequence)
);
CREATE TABLE role_audit (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    actor_id text NOT NULL REFERENCES users(id),
    user_id text NOT NULL REFERENCES users(id),
    previous_role text NOT NULL,
    new_role text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
