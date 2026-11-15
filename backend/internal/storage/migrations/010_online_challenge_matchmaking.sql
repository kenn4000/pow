CREATE TABLE online_challenge_preferences (
    user_id text PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    accepted_challenges text[] NOT NULL DEFAULT '{}',
    last_seen timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE online_challenge_cooldowns (
    user_id text PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    blocked_until timestamptz NOT NULL
);

ALTER TABLE online_battle_queue
    ADD COLUMN challenge_matchmaking boolean NOT NULL DEFAULT false;

ALTER TABLE puzzle_battle_queue
    ADD COLUMN challenge_matchmaking boolean NOT NULL DEFAULT false;

ALTER TABLE online_battle_invites
    ADD COLUMN match_mode text NOT NULL DEFAULT 'friend'
        CHECK (match_mode IN ('friend', 'online', 'ranked'));

ALTER TABLE puzzle_battle_invites
    ADD COLUMN match_mode text NOT NULL DEFAULT 'friend'
        CHECK (match_mode IN ('friend', 'online', 'ranked'));
