-- email et téléphone deviennent optionnels (mais au moins un des deux)
ALTER TABLE users ALTER COLUMN email DROP NOT NULL;
ALTER TABLE users ALTER COLUMN phone_number DROP NOT NULL;
ALTER TABLE users ADD CONSTRAINT users_contact_ck
    CHECK (email IS NOT NULL OR phone_number IS NOT NULL);

-- table utilisée par CreateUserWithOutbox
CREATE TABLE IF NOT EXISTS outbox_events (
    id           BIGSERIAL PRIMARY KEY,
    aggregate_id BIGINT      NOT NULL,
    event_type   VARCHAR(60) NOT NULL,
    payload      JSONB       NOT NULL,
    status       VARCHAR(20) NOT NULL DEFAULT 'pending'
                 CHECK (status IN ('pending','sent','failed')),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    processed_at TIMESTAMPTZ NULL
);

CREATE INDEX idx_outbox_pending ON outbox_events (id) WHERE status = 'pending';