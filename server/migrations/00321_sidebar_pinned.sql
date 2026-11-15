-- +goose Up
ALTER TABLE users ADD COLUMN sidebar_pinned text[] NOT NULL DEFAULT '{request_forms,stream}';
-- Existing users keep the default pins minus anything they already hid; ordinality keeps the default order.
UPDATE users SET sidebar_pinned = ARRAY(
  SELECT k FROM unnest(sidebar_pinned) WITH ORDINALITY AS p(k,n)
  WHERE k <> ALL(sidebar_hidden) ORDER BY n);

-- +goose Down
ALTER TABLE users DROP COLUMN sidebar_pinned;
