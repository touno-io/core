-- +goose Up
-- +goose StatementBegin
CREATE SCHEMA IF NOT EXISTS "cache";

CREATE TABLE IF NOT EXISTS "cache"."session" (
	s_key  VARCHAR(64) PRIMARY KEY NOT NULL DEFAULT '',
	a_value  BYTEA NOT NULL,
	t_expire  BIGINT NOT NULL DEFAULT '0'
);

CREATE INDEX IF NOT EXISTS "idx_expire" ON "cache"."session" (t_expire);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX "cache"."idx_expire";
DROP TABLE "cache"."session";
DROP SCHEMA "cache";
-- +goose StatementEnd
