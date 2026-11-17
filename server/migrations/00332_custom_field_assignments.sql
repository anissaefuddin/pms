-- +goose Up
-- Field assignments (CF-D02, DR-07, DR-08). A Workspace field applies in a Space only when the Space is assigned to it.
-- Space fields apply in their own Space. Project and folder assignments narrow a field to the subtrees they name.
-- Existing Workspace fields are assigned to every existing Space, so today's values stay visible.
CREATE TABLE custom_field_assignments (
    field_id uuid NOT NULL REFERENCES custom_fields(id) ON DELETE CASCADE,
    space_id uuid NOT NULL REFERENCES spaces(id) ON DELETE CASCADE,
    scope_kind text NOT NULL CHECK (scope_kind IN ('space', 'project', 'folder')),
    scope_id uuid NOT NULL,
    created_by uuid REFERENCES users(id),
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (field_id, space_id, scope_kind, scope_id),
    CHECK (scope_kind <> 'space' OR scope_id = space_id)
);
CREATE INDEX custom_field_assignments_space_idx ON custom_field_assignments (space_id, scope_kind, scope_id);

INSERT INTO custom_field_assignments (field_id, space_id, scope_kind, scope_id)
SELECT f.id, s.id, 'space', s.id FROM custom_fields f CROSS JOIN spaces s WHERE f.space_id IS NULL;

-- The one applicability rule. p_space is a Space where the item has a location.
-- shortcut: recursive walk of parents per call, fine at current tree depths; add a materialised ancestor table if trees grow deep.
-- +goose StatementBegin
CREATE FUNCTION custom_field_applies(p_item uuid, p_field uuid, p_space uuid) RETURNS boolean
LANGUAGE sql STABLE AS $$
WITH RECURSIVE up(id) AS (
    SELECT l.parent_item_id FROM item_locations l
    WHERE l.item_id = p_item AND l.space_id = p_space AND l.deleted_root_id IS NULL AND l.parent_item_id IS NOT NULL
  UNION
    SELECT l.parent_item_id FROM up JOIN item_locations l ON l.item_id = up.id AND l.space_id = p_space
    WHERE l.deleted_root_id IS NULL AND l.parent_item_id IS NOT NULL
), chain(id) AS (SELECT p_item UNION SELECT id FROM up)
SELECT EXISTS (
    SELECT 1 FROM custom_fields f
    WHERE f.id = p_field
      AND (f.space_id = p_space OR (f.space_id IS NULL AND EXISTS (SELECT 1 FROM custom_field_assignments a
            WHERE a.field_id = f.id AND a.space_id = p_space AND a.scope_kind = 'space' AND a.scope_id = p_space)))
      AND (NOT EXISTS (SELECT 1 FROM custom_field_assignments a
            WHERE a.field_id = f.id AND a.space_id = p_space AND a.scope_kind IN ('project', 'folder'))
        OR EXISTS (SELECT 1 FROM custom_field_assignments a JOIN chain c ON c.id = a.scope_id
            WHERE a.field_id = f.id AND a.space_id = p_space AND a.scope_kind IN ('project', 'folder')))
)
$$;
-- +goose StatementEnd

-- +goose Down
DROP FUNCTION custom_field_applies(uuid, uuid, uuid);
DROP TABLE custom_field_assignments;
