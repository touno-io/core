-- +goose Up
-- +goose StatementBegin
CREATE TYPE "opt_notify" AS ENUM (
  'telegram',
  'slack',
  'msteam',
  'line',
  'line-notify',
  'workplace',
  'email',
  'webhook',
  'native'
);

CREATE TABLE "notice_provider" (
  "id" serial PRIMARY KEY,
  "user_id" int4 NOT NULL,
  "e_type" opt_notify NOT NULL,
  "o_param" jsonb NOT NULL,
  "b_deleted" boolean DEFAULT false,
  "t_created" timestamp WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE "notice_room" (
  "id" serial PRIMARY KEY,
  "notice_provider_id" int4 NOT NULL,
  "o_param" jsonb NOT NULL,
  "b_deleted" boolean DEFAULT false,
  "t_created" timestamp WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);


CREATE TABLE "notice_section" (
  "id" serial PRIMARY KEY,
  "user_id" int4 NOT NULL,
  "s_name" varchar(20) NOT NULL,
  "n_uuid" uuid NOT NULL,
  "t_deleted" timestamp WITH TIME ZONE DEFAULT NULL,
  "t_created" timestamp WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);


CREATE TABLE "notice_subscriber" (
  "notice_section_id" int4 NOT NULL,
  "notice_room_id" int4 NOT NULL,
  "t_deleted" timestamp WITH TIME ZONE DEFAULT NULL,
  "t_created" timestamp WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE "notice_history" (
  "notice_room_id" int4 NOT NULL,
  "o_sender" jsonb NOT NULL,
  "b_sended" boolean DEFAULT false,
  "t_created" timestamp WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

ALTER TABLE "notice_provider" ADD FOREIGN KEY ("user_id") REFERENCES "user_account" ("id");
ALTER TABLE "notice_room" ADD FOREIGN KEY ("notice_provider_id") REFERENCES "notice_provider" ("id");
ALTER TABLE "notice_subscriber" ADD FOREIGN KEY ("notice_section_id") REFERENCES "notice_section" ("id");
ALTER TABLE "notice_subscriber" ADD FOREIGN KEY ("notice_room_id") REFERENCES "notice_room" ("id");
ALTER TABLE "notice_history" ADD FOREIGN KEY ("notice_room_id") REFERENCES "notice_room" ("id");
ALTER TABLE "notice_section" ADD FOREIGN KEY ("user_id") REFERENCES "user_account" ("id");

ALTER TABLE "notice_section" ADD CONSTRAINT uq_notice_section UNIQUE ("user_id", "s_name");

CREATE INDEX "idx_notice_section__deleted" ON "notice_section" USING BTREE ("t_deleted");
CREATE INDEX "idx_notice_provider__deleted" ON "notice_provider" USING BTREE ("b_deleted");

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE "notice_subscriber";
DROP TABLE "notice_history";
DROP TABLE "notice_section";
DROP TABLE "notice_room";
DROP TABLE "notice_provider";
DROP TYPE "opt_notify";
-- +goose StatementEnd
