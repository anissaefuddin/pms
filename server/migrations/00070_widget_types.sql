-- +goose Up
-- Analytics widgets (S11.2): area and pivot visuals, burndown, completion progress, and a short description per widget.
ALTER TABLE widgets DROP CONSTRAINT widgets_type_check;
ALTER TABLE widgets ADD CONSTRAINT widgets_type_check CHECK (type IN ('task_list','indicator','pie','column','bar','line','table','area','pivot','burndown','completion'));
ALTER TABLE widgets DROP CONSTRAINT widgets_config_check;
ALTER TABLE widgets ADD CONSTRAINT widgets_config_check CHECK (
    jsonb_typeof(config) = 'object'
    AND pg_column_size(config) <= 4096
    AND config - ARRAY['show_legend','show_labels','pie_style','list','template','query','description'] = '{}'::jsonb
    AND (NOT config ? 'show_legend' OR (type IN ('pie','column','bar','line','area','burndown') AND jsonb_typeof(config->'show_legend') = 'boolean'))
    AND (NOT config ? 'show_labels' OR (type IN ('pie','column','bar','line','area','burndown') AND jsonb_typeof(config->'show_labels') = 'boolean'))
    AND (NOT config ? 'pie_style' OR (type = 'pie' AND config->>'pie_style' IN ('classic','donut','ring')
        AND jsonb_typeof(config->'pie_style') = 'string'))
    AND (NOT (config ? 'list' OR config ? 'template') OR type = 'task_list')
    AND (NOT config ? 'query' OR (type <> 'task_list' AND jsonb_typeof(config->'query') = 'object'))
    AND (NOT config ? 'description' OR (jsonb_typeof(config->'description') = 'string' AND char_length(config->>'description') <= 500))
);

-- +goose Down
DELETE FROM widgets WHERE type IN ('area','pivot','burndown','completion');
UPDATE widgets SET config = config - 'description';
ALTER TABLE widgets DROP CONSTRAINT widgets_config_check;
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
ALTER TABLE widgets DROP CONSTRAINT widgets_type_check;
ALTER TABLE widgets ADD CONSTRAINT widgets_type_check CHECK (type IN ('task_list','indicator','pie','column','bar','line','table'));
