-- +goose Up
-- Transactional outbox (AUT-01, ADR-AUT): every change that automation may react to writes one row in the same transaction as
-- the change itself. A dispatcher claims undispatched rows with SKIP LOCKED, so a crash or rollback can neither lose nor invent an event.
CREATE TABLE domain_events (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    event_id uuid NOT NULL UNIQUE,
    type text NOT NULL,
    item_id uuid,
    actor_id uuid,
    payload jsonb NOT NULL DEFAULT '{}' CHECK (jsonb_typeof(payload) = 'object'),
    -- Loop guard (AUT-03): the run that caused this event, how many automation hops led here, and which rules were on the way.
    caused_by_run uuid,
    depth integer NOT NULL DEFAULT 0,
    chain uuid[] NOT NULL DEFAULT '{}',
    created_at timestamptz NOT NULL DEFAULT now(),
    dispatched_at timestamptz,
    attempts integer NOT NULL DEFAULT 0,
    last_error text
);
CREATE INDEX domain_events_pending_idx ON domain_events (id) WHERE dispatched_at IS NULL;
CREATE INDEX domain_events_item_idx ON domain_events (item_id, id DESC);
