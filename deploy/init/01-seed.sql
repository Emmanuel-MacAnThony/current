-- Demo fixture for the self-contained Current stack. This is NOT Current's own
-- state — Current persists nothing. It's a stand-in for "the one existing
-- database Current watches," seeded so the demo has something live to show.
-- Runs once, on first container init (docker-entrypoint-initdb.d).

CREATE TABLE payments (
    id         TEXT PRIMARY KEY,
    status     TEXT NOT NULL CHECK (status IN ('pending', 'succeeded', 'failed')),
    amount     BIGINT NOT NULL,          -- minor units (cents)
    currency   TEXT NOT NULL,
    user_id    TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO payments (id, status, amount, currency, user_id) VALUES
    ('pay_001', 'succeeded', 1200, 'USD', 'u_alice'),
    ('pay_002', 'succeeded', 4999, 'USD', 'u_bob'),
    ('pay_003', 'pending',   2500, 'EUR', 'u_carol'),
    ('pay_004', 'succeeded',  750, 'USD', 'u_dave'),
    ('pay_005', 'failed',    3200, 'GBP', 'u_erin'),
    ('pay_006', 'succeeded', 1899, 'USD', 'u_frank');
