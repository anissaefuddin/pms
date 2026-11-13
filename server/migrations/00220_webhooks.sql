-- +goose Up
-- Webhook subscriptions (API-02). Deliveries are created by the outbox handler, one per (subscription, event), so a re-delivered event never sends twice.
CREATE TABLE webhook_subscriptions (
    id uuid PRIMARY KEY,
    owner_id uuid NOT NULL REFERENCES users(id),
    name text NOT NULL CHECK (length(name) BETWEEN 1 AND 100),
    url text NOT NULL CHECK (length(url) <= 2000),
    -- shortcut: the signing secret is stored as-is (HMAC needs it); encrypt with a server key when one is configured for it.
    secret text NOT NULL,
    event_types text[] NOT NULL CHECK (cardinality(event_types) > 0),
    enabled boolean NOT NULL DEFAULT true,
    consecutive_failures integer NOT NULL DEFAULT 0,
    disabled_reason text,
    created_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz
);
CREATE INDEX webhook_subscriptions_owner_idx ON webhook_subscriptions (owner_id) WHERE deleted_at IS NULL;

CREATE TABLE webhook_deliveries (
    id uuid PRIMARY KEY,
    subscription_id uuid NOT NULL REFERENCES webhook_subscriptions(id) ON DELETE CASCADE,
    event_id uuid NOT NULL,
    event_type text NOT NULL,
    payload jsonb NOT NULL,
    status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','success','failed')),
    attempts integer NOT NULL DEFAULT 0,
    next_attempt_at timestamptz NOT NULL DEFAULT now(),
    response_status integer,
    last_error text,
    created_at timestamptz NOT NULL DEFAULT now(),
    delivered_at timestamptz,
    UNIQUE (subscription_id, event_id)
);
CREATE INDEX webhook_deliveries_due_idx ON webhook_deliveries (next_attempt_at) WHERE status = 'pending';
CREATE INDEX webhook_deliveries_sub_idx ON webhook_deliveries (subscription_id, created_at DESC);

-- +goose Down
DROP TABLE webhook_deliveries;
DROP TABLE webhook_subscriptions;
