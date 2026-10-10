-- +goose Up
CREATE TABLE cms_git_connection (
 id BIGINT PRIMARY KEY CHECK(id=1), login TEXT NOT NULL, repository TEXT NOT NULL, branch TEXT NOT NULL,
 source_repository TEXT NOT NULL, source_ref TEXT NOT NULL, token_cipher TEXT NOT NULL,
 updated_at TIMESTAMP NOT NULL
);
-- +goose Down
DROP TABLE cms_git_connection;
