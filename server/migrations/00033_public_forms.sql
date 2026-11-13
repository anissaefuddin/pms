-- +goose Up
-- A public link is a random token; NULL means no public link. shortcut: the token is stored as is (the Manage forms
-- screen shows the link again), so it is only a capability to submit this one form; hash it if links ever grant more.
ALTER TABLE request_forms ADD COLUMN public_token text UNIQUE CHECK (public_token IS NULL OR char_length(public_token) >= 32);

-- Items created from a public link are owned by this placeholder account. It holds no seat (deactivated), has no
-- password, and cannot sign in. The person's own details live on the submission row.
INSERT INTO users (id, email, first_name, last_name, user_type, status)
VALUES ('00000000-0000-4000-8000-0000000000e1', 'external-requester@system.invalid', 'External', 'requester', 'external', 'deactivated');

ALTER TABLE form_submissions
  ADD COLUMN requester_name text CHECK (requester_name IS NULL OR char_length(requester_name) <= 120),
  ADD COLUMN requester_email citext CHECK (requester_email IS NULL OR char_length(requester_email) <= 254);
