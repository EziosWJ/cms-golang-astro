-- +goose Up
CREATE TABLE cms_site_config (id INTEGER PRIMARY KEY CHECK(id=1),version INTEGER NOT NULL CHECK(version>=1),data TEXT NOT NULL,saved_at DATETIME NOT NULL,saved_by INTEGER NOT NULL REFERENCES sys_user(id));
CREATE TABLE cms_config_revision (id INTEGER PRIMARY KEY AUTOINCREMENT,version INTEGER NOT NULL UNIQUE,data TEXT NOT NULL,created_at DATETIME NOT NULL,created_by INTEGER NOT NULL REFERENCES sys_user(id));
-- +goose StatementBegin
CREATE TRIGGER cms_config_revision_no_update BEFORE UPDATE ON cms_config_revision BEGIN SELECT RAISE(ABORT,'config revisions are immutable'); END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER cms_config_revision_no_delete BEFORE DELETE ON cms_config_revision BEGIN SELECT RAISE(ABORT,'config revisions are immutable'); END;
-- +goose StatementEnd
-- +goose Down
DROP TABLE cms_config_revision;
DROP TABLE cms_site_config;
