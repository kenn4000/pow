CREATE TABLE social_conversation_deletions (
    owner_id text NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    other_id text NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    deleted_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (owner_id, other_id),
    CHECK (owner_id <> other_id)
);

ALTER TABLE online_battle_invites
    ADD COLUMN wager_marks bigint NOT NULL DEFAULT 0 CHECK (wager_marks BETWEEN 0 AND 1000);
ALTER TABLE puzzle_battle_invites
    ADD COLUMN wager_marks bigint NOT NULL DEFAULT 0 CHECK (wager_marks BETWEEN 0 AND 1000);
ALTER TABLE online_battle_matches
    ADD COLUMN wager_marks bigint NOT NULL DEFAULT 0 CHECK (wager_marks BETWEEN 0 AND 1000);
ALTER TABLE puzzle_battle_matches
    ADD COLUMN wager_marks bigint NOT NULL DEFAULT 0 CHECK (wager_marks BETWEEN 0 AND 1000);

CREATE TABLE friend_wagers (
    match_id text PRIMARY KEY,
    game_type text NOT NULL CHECK (game_type IN ('online', 'puzzle')),
    player_a_id text NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    player_b_id text NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    stake_marks bigint NOT NULL CHECK (stake_marks BETWEEN 1 AND 1000),
    winner_player smallint CHECK (winner_player IN (0, 1)),
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'settled')),
    created_at timestamptz NOT NULL DEFAULT now(),
    settled_at timestamptz,
    CHECK (player_a_id <> player_b_id)
);

CREATE INDEX friend_wagers_player_recent
    ON friend_wagers (player_a_id, created_at DESC);
CREATE INDEX friend_wagers_opponent_recent
    ON friend_wagers (player_b_id, created_at DESC);
