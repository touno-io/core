-- +goose Up
-- +goose StatementBegin
CREATE TYPE "opt_level" AS ENUM (
  'OWNER',
  'CONTRIBUTOR',
  'SUPPORTER',
  'VISITOR',
  'BANED'
);

CREATE TYPE "opt_role" AS ENUM (
  'COURIER',
  'SYSTEM',
  'USER'
);

CREATE TABLE "user_permission" (
  "id" serial,
  "s_name" varchar(25) NOT NULL,
  "o_scope" jsonb NOT NULL DEFAULT '[]'::jsonb,
  PRIMARY KEY ("id")
);

CREATE TABLE "user_role" (
  "id" serial,
  "user_permission_id" int4 NOT NULL,
  "s_name" varchar(25) NOT NULL,
  "n_role" int4 NOT NULL,
  "e_role" opt_role NOT NULL DEFAULT 'USER',
  PRIMARY KEY ("id"),
  FOREIGN KEY ("user_permission_id") REFERENCES "user_permission" ("id")
);

CREATE TABLE "user_account" (
  "id" serial,
  "s_display_name" varchar(100) NOT NULL,
  "s_email" varchar(50) UNIQUE NOT NULL,
  "s_pwd" text,
  "n_object" uuid NOT NULL DEFAULT uuid_generate_v4(),
  "n_level" opt_level NOT NULL DEFAULT 'BANED',
  "t_created" timestamp WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY ("id")
);

CREATE TABLE "user_policy" (
  "user_id" int4 NOT NULL,
  "user_role_id" int4 NOT NULL,
  "s_hash" varchar(5) UNIQUE NOT NULL DEFAULT SUBSTRING(uuid_generate_v4()::text,1,5),
  FOREIGN KEY ("user_id") REFERENCES "user_account" ("id"),
  FOREIGN KEY ("user_role_id") REFERENCES "user_role" ("id")
);

CREATE TABLE "user_session" (
  "user_id" int4 NOT NULL,
  "n_session" uuid NOT NULL DEFAULT uuid_generate_v4(),
  "o_permission" jsonb NOT NULL,
  "o_policy" jsonb NOT NULL,
  "t_created" timestamp WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE "user_token" (
  "user_id" int4 NOT NULL,
  "user_role_id" int4 NOT NULL,
  "s_token" varchar(32) NOT NULL DEFAULT REPLACE(uuid_generate_v4()::text, '-', '' )
);

INSERT INTO "user_account" (id, s_display_name, s_email, n_level)
VALUES (1, 'Kananek Thongkam', 'info.dvgamer@gmail.com', 'OWNER');

SELECT SETVAL((SELECT pg_get_serial_sequence('user_account', 'id')), (SELECT COALESCE(MAX(id), 1) FROM user_account));
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE "user_token";
DROP TABLE "user_policy";
DROP TABLE "user_role";
DROP TABLE "user_permission";
DROP TABLE "user_session";
DROP TABLE "user_account";
DROP TYPE "opt_role";
DROP TYPE "opt_level";
-- +goose StatementEnd
