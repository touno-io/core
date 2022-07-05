-- +goose Up
-- +goose StatementBegin
CREATE EXTENSION "uuid-ossp";
CREATE EXTENSION "pgcrypto";
CREATE EXTENSION "pg_buffercache";
CREATE EXTENSION "pg_prewarm";
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- +goose StatementEnd
