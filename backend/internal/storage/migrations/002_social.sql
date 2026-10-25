ALTER TABLE users
    ADD COLUMN public_handle text,
    ADD COLUMN elo_rating integer NOT NULL DEFAULT 1000
        CHECK (elo_rating >= 0 AND elo_rating <= 10000);

UPDATE users
SET public_handle = 'player_' || substr(id, 1, 12)
WHERE public_handle IS NULL;

ALTER TABLE users
    ALTER COLUMN public_handle SET NOT NULL;

CREATE UNIQUE INDEX users_public_handle_lower
    ON users (lower(public_handle));

CREATE TABLE friend_requests (
    id text PRIMARY KEY,
    sender_id text NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    recipient_id text NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    message varchar(140) NOT NULL DEFAULT '',
    status text NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'accepted', 'rejected')),
    created_at timestamptz NOT NULL DEFAULT now(),
    responded_at timestamptz,
    CHECK (sender_id <> recipient_id)
);

CREATE UNIQUE INDEX friend_requests_pending_pair
    ON friend_requests (LEAST(sender_id, recipient_id), GREATEST(sender_id, recipient_id))
    WHERE status = 'pending';
CREATE INDEX friend_requests_recipient_pending
    ON friend_requests (recipient_id, created_at DESC)
    WHERE status = 'pending';

CREATE TABLE friendships (
    user_a text NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    user_b text NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_a, user_b),
    CHECK (user_a < user_b)
);

CREATE TABLE player_blocks (
    blocker_id text NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    blocked_id text NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (blocker_id, blocked_id),
    CHECK (blocker_id <> blocked_id)
);

CREATE TABLE social_messages (
    id text PRIMARY KEY,
    sender_id text NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    recipient_id text NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    body text NOT NULL CHECK (length(body) BETWEEN 1 AND 1000),
    created_at timestamptz NOT NULL DEFAULT now(),
    CHECK (sender_id <> recipient_id)
);

CREATE INDEX social_messages_inbox
    ON social_messages (recipient_id, created_at DESC);
CREATE INDEX social_messages_conversation
    ON social_messages (sender_id, recipient_id, created_at DESC);
