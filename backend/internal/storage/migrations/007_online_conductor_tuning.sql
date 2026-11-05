ALTER TABLE online_battle_queue
    ADD COLUMN conductor_modifier text NOT NULL DEFAULT '',
    ADD COLUMN conductor_amount integer NOT NULL DEFAULT 0,
    ADD CONSTRAINT online_battle_queue_conductor_tuning_check CHECK (
        (conductor_modifier = '' AND conductor_amount = 0)
        OR (conductor_modifier IN ('bounty', 'shift', 'favored')
            AND conductor_amount BETWEEN -2 AND 2
            AND conductor_amount <> 0)
    );

ALTER TABLE online_battle_invites
    ADD COLUMN sender_conductor_modifier text NOT NULL DEFAULT '',
    ADD COLUMN sender_conductor_amount integer NOT NULL DEFAULT 0,
    ADD CONSTRAINT online_battle_invites_sender_conductor_tuning_check CHECK (
        (sender_conductor_modifier = '' AND sender_conductor_amount = 0)
        OR (sender_conductor_modifier IN ('bounty', 'shift', 'favored')
            AND sender_conductor_amount BETWEEN -2 AND 2
            AND sender_conductor_amount <> 0)
    );
