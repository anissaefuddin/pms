-- +goose Up
ALTER TABLE items
    ADD COLUMN progress_method text
        CHECK (progress_method IS NULL OR
            (type = 'project' AND progress_method IN ('task_count', 'weighted_duration', 'average_percent'))),
    ADD COLUMN progress_percent smallint
        CHECK (progress_percent IS NULL OR (type = 'task' AND progress_percent BETWEEN 0 AND 100));
