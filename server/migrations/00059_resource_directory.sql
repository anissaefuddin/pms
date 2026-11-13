-- +goose Up
CREATE TABLE resource_roles (id uuid PRIMARY KEY DEFAULT gen_random_uuid(), name text NOT NULL UNIQUE);
CREATE TABLE resource_skills (id uuid PRIMARY KEY DEFAULT gen_random_uuid(), name text NOT NULL UNIQUE);
CREATE TABLE resource_user_roles (user_id uuid REFERENCES users(id) ON DELETE CASCADE, role_id uuid REFERENCES resource_roles(id) ON DELETE CASCADE, PRIMARY KEY(user_id,role_id));
CREATE TABLE resource_user_skills (user_id uuid REFERENCES users(id) ON DELETE CASCADE, skill_id uuid REFERENCES resource_skills(id) ON DELETE CASCADE, PRIMARY KEY(user_id,skill_id));
CREATE TABLE resource_bookings (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), project_id uuid NOT NULL REFERENCES items(id) ON DELETE CASCADE,
 user_id uuid REFERENCES users(id) ON DELETE CASCADE, role_id uuid REFERENCES resource_roles(id) ON DELETE RESTRICT,
 start_date date NOT NULL, end_date date NOT NULL, minutes_per_day integer NOT NULL CHECK(minutes_per_day BETWEEN 1 AND 1440),
 CHECK (num_nonnulls(user_id,role_id)=1), CHECK(start_date<=end_date)
);
CREATE INDEX resource_bookings_dates_idx ON resource_bookings(end_date,start_date);

-- +goose Down
DROP TABLE resource_bookings,resource_user_skills,resource_user_roles,resource_skills,resource_roles;
