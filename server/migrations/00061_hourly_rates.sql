-- +goose Up
-- Hourly rates (TASK-14), in minor units (cents) of the workspace currency. Versioned by effective date: a change is a new row,
-- and a time entry copies the rate in force on its date when it is created, so later changes never rewrite history.
CREATE TABLE hourly_rates (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid REFERENCES users(id) ON DELETE CASCADE,
    role_id uuid REFERENCES resource_roles(id) ON DELETE CASCADE,
    rate_minor bigint NOT NULL CHECK (rate_minor BETWEEN 0 AND 100000000),
    effective_from date NOT NULL,
    created_by uuid REFERENCES users(id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CHECK ((user_id IS NULL) <> (role_id IS NULL))
);
CREATE UNIQUE INDEX hourly_rates_user_uq ON hourly_rates (user_id, effective_from) WHERE user_id IS NOT NULL;
CREATE UNIQUE INDEX hourly_rates_role_uq ON hourly_rates (role_id, effective_from) WHERE role_id IS NOT NULL;

ALTER TABLE time_entries ADD COLUMN rate_snapshot bigint CHECK (rate_snapshot IS NULL OR rate_snapshot >= 0);

-- +goose StatementBegin
CREATE FUNCTION time_entry_rate_snapshot() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.rate_snapshot IS NULL THEN
        NEW.rate_snapshot := COALESCE(
            (SELECT r.rate_minor FROM hourly_rates r WHERE r.user_id = NEW.user_id AND r.effective_from <= NEW.entry_date
              ORDER BY r.effective_from DESC LIMIT 1),
            (SELECT max(x.rate_minor) FROM (
                SELECT DISTINCT ON (r.role_id) r.rate_minor FROM hourly_rates r JOIN resource_user_roles ur ON ur.role_id = r.role_id AND ur.user_id = NEW.user_id
                 WHERE r.effective_from <= NEW.entry_date ORDER BY r.role_id, r.effective_from DESC) x));
    END IF;
    RETURN NEW;
END $$;
-- +goose StatementEnd
CREATE TRIGGER time_entries_rate_snapshot BEFORE INSERT ON time_entries FOR EACH ROW EXECUTE FUNCTION time_entry_rate_snapshot();

-- +goose Down
DROP TRIGGER time_entries_rate_snapshot ON time_entries;
DROP FUNCTION time_entry_rate_snapshot();
ALTER TABLE time_entries DROP COLUMN rate_snapshot;
DROP TABLE hourly_rates;
