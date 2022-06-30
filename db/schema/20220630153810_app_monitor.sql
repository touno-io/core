-- +goose Up
-- +goose StatementBegin
CREATE TYPE "opt_monitor" AS ENUM (
  'HTTP',
  'PORT',
  'PING'
);

CREATE TABLE "monitor" (
  "id" serial,
	"s_name" varchar(50) NOT NULL,
	"n_heartbeat" int NOT NULL DEFAULT 15,
	"b_mobile" boolean NOT NULL DEFAULT false,
	"b_email" boolean NOT NULL DEFAULT false,
	PRIMARY KEY ("id")
);

CREATE TABLE "monitor_protocal" (
  "id" serial,
  "monitor_id" int4  NOT NULL,
	"v_host" varchar(150) NOT NULL,
	"n_port" int NOT NULL,
	"e_type" "opt_monitor" NOT NULL DEFAULT 'HTTP',
	"o_param" jsonb,
	"n_interval" int NOT NULL DEFAULT 0,
	"n_timeout" int NOT NULL DEFAULT 0,
	"t_created" timestamp with time zone NOT NULL DEFAULT CURRENT_TIMESTAMP,
	PRIMARY KEY ("id"),
	FOREIGN KEY ("monitor_id") REFERENCES "monitor" ("id")
);

CREATE TABLE "monitor_heartbeat" (
  "monitor_protocal_id" int4  NOT NULL,
	"o_latency" jsonb NOT NULL,
	"t_created" timestamp with time zone NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY ("monitor_protocal_id") REFERENCES "monitor_protocal" ("id")
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE "monitor_heartbeat";
DROP TABLE "monitor_protocal";
DROP TABLE "monitor";
DROP TYPE "opt_monitor";
-- +goose StatementEnd
