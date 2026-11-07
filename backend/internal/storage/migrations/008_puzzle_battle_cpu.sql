CREATE TABLE puzzle_battle_cpu_matches (
    id text PRIMARY KEY,
    player_id text NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    hero_id text NOT NULL,
    status text NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'completed')),
    state jsonb NOT NULL,
    result jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    completed_at timestamptz
);

CREATE INDEX puzzle_battle_cpu_matches_player
    ON puzzle_battle_cpu_matches (player_id, updated_at DESC);
CREATE UNIQUE INDEX puzzle_battle_cpu_matches_active_player
    ON puzzle_battle_cpu_matches (player_id)
    WHERE status = 'active';
