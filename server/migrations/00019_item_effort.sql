-- +goose Up
ALTER TABLE items ADD COLUMN effort_minutes integer CHECK (effort_minutes >= 0);

ALTER TABLE item_assignees
    ADD COLUMN allocation_unit text CHECK (allocation_unit IN ('percent', 'minutes_per_day')),
    ADD COLUMN allocation_value integer CHECK (allocation_value >= 0),
    ADD CONSTRAINT item_assignees_allocation_pair
        CHECK ((allocation_unit IS NULL) = (allocation_value IS NULL));
