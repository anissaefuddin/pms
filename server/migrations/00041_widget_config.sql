-- +goose Up
-- Widgets now carry what they show: a task list preset (list or template) or a validated analytics query.
ALTER TABLE widgets DROP CONSTRAINT widgets_check;
ALTER TABLE widgets ADD CONSTRAINT widgets_config_check CHECK (
    jsonb_typeof(config) = 'object'
    AND pg_column_size(config) <= 4096
    AND config - ARRAY['show_legend','show_labels','pie_style','list','template','query'] = '{}'::jsonb
    AND (NOT config ? 'show_legend' OR (type IN ('pie','column','bar','line') AND jsonb_typeof(config->'show_legend') = 'boolean'))
    AND (NOT config ? 'show_labels' OR (type IN ('pie','column','bar','line') AND jsonb_typeof(config->'show_labels') = 'boolean'))
    AND (NOT config ? 'pie_style' OR (type = 'pie' AND config->>'pie_style' IN ('classic','donut','ring')
        AND jsonb_typeof(config->'pie_style') = 'string'))
    AND (NOT (config ? 'list' OR config ? 'template') OR type = 'task_list')
    AND (NOT config ? 'query' OR (type <> 'task_list' AND jsonb_typeof(config->'query') = 'object'))
);
