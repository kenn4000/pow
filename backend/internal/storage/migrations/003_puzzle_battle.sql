CREATE TABLE puzzle_battle_queue (
    user_id text PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    hero_id text NOT NULL,
    queued_at timestamptz NOT NULL DEFAULT now(),
    last_seen timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX puzzle_battle_queue_order
    ON puzzle_battle_queue (queued_at, user_id);

CREATE TABLE puzzle_battle_invites (
    id text PRIMARY KEY,
    sender_id text NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    recipient_id text NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    sender_hero_id text NOT NULL,
    recipient_hero_id text,
    status text NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'accepted', 'declined', 'expired')),
    created_at timestamptz NOT NULL DEFAULT now(),
    responded_at timestamptz,
    CHECK (sender_id <> recipient_id)
);

CREATE UNIQUE INDEX puzzle_battle_invites_pending_pair
    ON puzzle_battle_invites (LEAST(sender_id, recipient_id), GREATEST(sender_id, recipient_id))
    WHERE status = 'pending';
CREATE INDEX puzzle_battle_invites_recipient
    ON puzzle_battle_invites (recipient_id, created_at DESC)
    WHERE status = 'pending';

CREATE TABLE puzzle_battle_matches (
    id text PRIMARY KEY,
    player_a_id text NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    player_b_id text NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    player_a_hero_id text NOT NULL,
    player_b_hero_id text NOT NULL,
    status text NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'completed', 'abandoned')),
    state jsonb NOT NULL,
    result jsonb,
    player_a_last_seen timestamptz NOT NULL DEFAULT now(),
    player_b_last_seen timestamptz NOT NULL DEFAULT now(),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    completed_at timestamptz,
    CHECK (player_a_id <> player_b_id)
);

CREATE INDEX puzzle_battle_matches_player_a
    ON puzzle_battle_matches (player_a_id, updated_at DESC);
CREATE INDEX puzzle_battle_matches_player_b
    ON puzzle_battle_matches (player_b_id, updated_at DESC);
CREATE INDEX puzzle_battle_matches_active_a
    ON puzzle_battle_matches (player_a_id)
    WHERE status = 'active';
CREATE INDEX puzzle_battle_matches_active_b
    ON puzzle_battle_matches (player_b_id)
    WHERE status = 'active';
