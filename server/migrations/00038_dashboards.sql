-- +goose Up
CREATE TABLE dashboards (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id uuid NOT NULL REFERENCES users(id),
    space_id uuid REFERENCES spaces(id) ON DELETE CASCADE,
    name text NOT NULL CHECK (char_length(btrim(name)) BETWEEN 1 AND 120),
    version integer NOT NULL DEFAULT 1 CHECK (version > 0)
);

CREATE INDEX dashboards_personal_idx ON dashboards(owner_id, id) WHERE space_id IS NULL;
CREATE INDEX dashboards_space_idx ON dashboards(space_id, id) WHERE space_id IS NOT NULL;

CREATE TABLE widgets (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    dashboard_id uuid NOT NULL REFERENCES dashboards(id) ON DELETE CASCADE,
    name text NOT NULL CHECK (char_length(btrim(name)) BETWEEN 1 AND 120),
    type text NOT NULL CHECK (type IN ('task_list','indicator','pie','column','bar','line','table')),
    position integer NOT NULL DEFAULT 0 CHECK (position BETWEEN 0 AND 9999),
    config jsonb NOT NULL DEFAULT '{}',
    CHECK (
        jsonb_typeof(config) = 'object'
        AND config - ARRAY['show_legend','show_labels','pie_style'] = '{}'::jsonb
        AND (NOT config ? 'show_legend' OR (type IN ('pie','column','bar','line') AND jsonb_typeof(config->'show_legend') = 'boolean'))
        AND (NOT config ? 'show_labels' OR (type IN ('pie','column','bar','line') AND jsonb_typeof(config->'show_labels') = 'boolean'))
        AND (NOT config ? 'pie_style' OR (type = 'pie' AND config->>'pie_style' IN ('classic','donut','ring')
            AND jsonb_typeof(config->'pie_style') = 'string'))
    )
);

CREATE INDEX widgets_dashboard_idx ON widgets(dashboard_id, position, id);
