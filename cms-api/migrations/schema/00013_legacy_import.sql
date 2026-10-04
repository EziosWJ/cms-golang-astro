-- +goose Up
ALTER TABLE cms_article ADD COLUMN imported_updated_at TIMESTAMPTZ;
CREATE TABLE cms_media_alias(path TEXT PRIMARY KEY,file_id BIGINT NOT NULL REFERENCES sys_file(id));
CREATE TABLE cms_import_article(source_key TEXT PRIMARY KEY,fingerprint VARCHAR(64) NOT NULL,article_id BIGINT NOT NULL UNIQUE REFERENCES cms_article(id),imported_at TIMESTAMPTZ NOT NULL);
CREATE TABLE cms_import_media(source_key TEXT PRIMARY KEY,fingerprint VARCHAR(64) NOT NULL,file_id BIGINT NOT NULL REFERENCES sys_file(id),imported_at TIMESTAMPTZ NOT NULL);
-- +goose Down
DROP TABLE cms_import_media;
DROP TABLE cms_import_article;
DROP TABLE cms_media_alias;
ALTER TABLE cms_article DROP COLUMN imported_updated_at;
