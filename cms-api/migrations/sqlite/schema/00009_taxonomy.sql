-- +goose Up
CREATE TABLE cms_taxonomy (
 id INTEGER PRIMARY KEY AUTOINCREMENT, kind VARCHAR(20) NOT NULL CHECK (kind IN ('category','tag')),
 name VARCHAR(200) NOT NULL CHECK (length(trim(name)) > 0), url VARCHAR(200) NOT NULL CHECK (length(url) > 0),
 version INTEGER NOT NULL DEFAULT 1, locked_at DATETIME,
 CONSTRAINT uk_cms_taxonomy_name UNIQUE(kind,name), CONSTRAINT uk_cms_taxonomy_url UNIQUE(kind,url)
);
CREATE TABLE cms_taxonomy_ref (
 owner_type VARCHAR(20) NOT NULL, owner_id INTEGER NOT NULL,
 term_id INTEGER NOT NULL REFERENCES cms_taxonomy(id) ON DELETE RESTRICT,
 PRIMARY KEY(owner_type,owner_id,term_id)
);
CREATE INDEX idx_cms_taxonomy_ref_term ON cms_taxonomy_ref(term_id);
ALTER TABLE cms_working_draft ADD COLUMN taxonomy TEXT NOT NULL DEFAULT '[]';
ALTER TABLE cms_article_revision ADD COLUMN taxonomy TEXT NOT NULL DEFAULT '[]';
-- +goose Down
ALTER TABLE cms_article_revision DROP COLUMN taxonomy;
ALTER TABLE cms_working_draft DROP COLUMN taxonomy;
DROP TABLE cms_taxonomy_ref;
DROP TABLE cms_taxonomy;
