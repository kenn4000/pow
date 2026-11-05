CREATE TABLE user_variant_ratings (
    user_id text NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    variant text NOT NULL CHECK (variant IN (
        'main', 'fog', 'rummy', 'puzzle-battle', 'poker-duel',
        'ladder', 'tug-war', 'abandon-duel'
    )),
    rating integer NOT NULL DEFAULT 1000 CHECK (rating >= 0 AND rating <= 10000),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, variant)
);

INSERT INTO user_variant_ratings (user_id, variant)
SELECT users.id, variants.variant
FROM users
CROSS JOIN unnest(ARRAY[
    'main', 'fog', 'rummy', 'puzzle-battle', 'poker-duel',
    'ladder', 'tug-war', 'abandon-duel'
]::text[]) AS variants(variant);

CREATE FUNCTION seed_user_variant_ratings() RETURNS trigger AS $$
BEGIN
    INSERT INTO user_variant_ratings (user_id, variant)
    SELECT NEW.id, variant
    FROM unnest(ARRAY[
        'main', 'fog', 'rummy', 'puzzle-battle', 'poker-duel',
        'ladder', 'tug-war', 'abandon-duel'
    ]::text[]) AS variants(variant);
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER users_seed_variant_ratings
AFTER INSERT ON users
FOR EACH ROW EXECUTE FUNCTION seed_user_variant_ratings();

ALTER TABLE online_battle_queue
    ADD COLUMN match_mode text NOT NULL DEFAULT 'online'
        CHECK (match_mode IN ('online', 'ranked'));

ALTER TABLE online_battle_matches
    ADD COLUMN match_mode text NOT NULL DEFAULT 'friend'
        CHECK (match_mode IN ('friend', 'online', 'ranked'));

ALTER TABLE puzzle_battle_queue
    ADD COLUMN match_mode text NOT NULL DEFAULT 'online'
        CHECK (match_mode IN ('online', 'ranked'));

ALTER TABLE puzzle_battle_matches
    ADD COLUMN match_mode text NOT NULL DEFAULT 'friend'
        CHECK (match_mode IN ('friend', 'online', 'ranked'));
