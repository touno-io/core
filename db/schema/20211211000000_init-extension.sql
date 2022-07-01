-- +goose Up
-- +goose StatementBegin
CREATE EXTENSION "uuid-ossp";
CREATE EXTENSION "pgcrypto";
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- +goose StatementEnd
