ALTER TABLE online_battle_queue
    DROP CONSTRAINT online_battle_queue_variant_check,
    ADD CONSTRAINT online_battle_queue_variant_check
        CHECK (variant IN ('main', 'fog', 'rummy', 'ladder', 'poker-duel', 'tug-war', 'abandon-duel'));

ALTER TABLE online_battle_invites
    DROP CONSTRAINT online_battle_invites_variant_check,
    ADD CONSTRAINT online_battle_invites_variant_check
        CHECK (variant IN ('main', 'fog', 'rummy', 'ladder', 'poker-duel', 'tug-war', 'abandon-duel'));

ALTER TABLE online_battle_matches
    DROP CONSTRAINT online_battle_matches_variant_check,
    ADD CONSTRAINT online_battle_matches_variant_check
        CHECK (variant IN ('main', 'fog', 'rummy', 'ladder', 'poker-duel', 'tug-war', 'abandon-duel'));
