-- +goose Up
-- +goose StatementBegin

CREATE TABLE "public"."shorturl" (
  "hash" varchar NOT NULL,
  "url" text NOT NULL,
  "hit" int8 NOT NULL DEFAULT 0,
  "created" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "title" varchar NOT NULL DEFAULT ''::character varying,
  "meta" json NOT NULL DEFAULT '[]'::json
);

CREATE TABLE "public"."shorturl_history" (
  "hash" varchar NOT NULL,
  "agent" json,
  "device" json,
  "created" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE "public"."shorturl_tracking" (
  "ip_addr" varchar NOT NULL,
  "hash" varchar NOT NULL,
  "isp" varchar NOT NULL,
  "country" varchar NOT NULL,
  "proxy" bool NOT NULL,
  "hosting" bool NOT NULL,
  "visited" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "hit" int8 NOT NULL DEFAULT 0
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS "public"."shorturl";
DROP TABLE IF EXISTS "public"."shorturl_history";
DROP TABLE IF EXISTS "public"."shorturl_tracking";
-- +goose StatementEnd
