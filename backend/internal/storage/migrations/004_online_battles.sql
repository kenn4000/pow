CREATE TABLE online_battle_queue (
    user_id text PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    variant text NOT NULL CHECK (variant IN ('main', 'fog', 'rummy', 'ladder', 'poker-duel')),
    hero_id text NOT NULL,
    queued_at timestamptz NOT NULL DEFAULT now(),
    last_seen timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX online_battle_queue_order
    ON online_battle_queue (variant, queued_at, user_id);

CREATE TABLE online_battle_invites (
    id text PRIMARY KEY,
    variant text NOT NULL CHECK (variant IN ('main', 'fog', 'rummy', 'ladder', 'poker-duel')),
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

CREATE UNIQUE INDEX online_battle_invites_pending_pair
    ON online_battle_invites
        (variant, LEAST(sender_id, recipient_id), GREATEST(sender_id, recipient_id))
    WHERE status = 'pending';
CREATE INDEX online_battle_invites_recipient
    ON online_battle_invites (recipient_id, created_at DESC)
    WHERE status = 'pending';

CREATE TABLE online_battle_matches (
    id text PRIMARY KEY,
    variant text NOT NULL CHECK (variant IN ('main', 'fog', 'rummy', 'ladder', 'poker-duel')),
    player_a_id text NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    player_b_id text NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    player_a_hero_id text NOT NULL,
    player_b_hero_id text NOT NULL,
    status text NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'completed', 'abandoned')),
    state jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    completed_at timestamptz,
    CHECK (player_a_id <> player_b_id)
);

CREATE INDEX online_battle_matches_player_a
    ON online_battle_matches (player_a_id, updated_at DESC);
CREATE INDEX online_battle_matches_player_b
    ON online_battle_matches (player_b_id, updated_at DESC);

CREATE TABLE online_battle_active_users (
    user_id text PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    match_id text NOT NULL REFERENCES online_battle_matches(id) ON DELETE CASCADE
);
