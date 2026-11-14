-- +goose Up
-- SPC-14: item types hidden from a Space's "+" menu. Empty = every active type is offered (today's behaviour).
-- A convenience for the UI only: it never limits what the API accepts (D-99).
ALTER TABLE spaces ADD COLUMN create_menu_hidden text[] NOT NULL DEFAULT '{}';
