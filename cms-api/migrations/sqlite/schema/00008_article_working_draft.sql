-- +goose Up
CREATE TABLE cms_article (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    slug VARCHAR(200),
    lifecycle VARCHAR(20) NOT NULL DEFAULT 'active' CHECK (lifecycle IN ('active', 'archived')),
    slug_locked_at DATETIME,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uk_cms_article_slug UNIQUE (slug),
    CONSTRAINT ck_cms_article_slug CHECK (slug IS NULL OR length(slug) BETWEEN 1 AND 200)
);
CREATE TABLE cms_working_draft (
    article_id INTEGER PRIMARY KEY REFERENCES cms_article(id) ON DELETE RESTRICT,
    version INTEGER NOT NULL CHECK (version >= 1),
    title TEXT NOT NULL DEFAULT '',
    markdown TEXT NOT NULL DEFAULT '',
    summary TEXT NOT NULL DEFAULT '',
    display_date DATETIME,
    saved_at DATETIME NOT NULL,
    saved_by INTEGER NOT NULL REFERENCES sys_user(id)
);
CREATE TABLE cms_article_revision (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    article_id INTEGER NOT NULL REFERENCES cms_article(id) ON DELETE RESTRICT,
    version INTEGER NOT NULL CHECK (version >= 1),
    slug VARCHAR(200) NOT NULL DEFAULT '',
    title TEXT NOT NULL,
    markdown TEXT NOT NULL,
    summary TEXT NOT NULL,
    display_date DATETIME,
    created_at DATETIME NOT NULL,
    created_by INTEGER NOT NULL REFERENCES sys_user(id),
    CONSTRAINT uk_cms_article_revision_version UNIQUE(article_id, version)
);
CREATE INDEX idx_cms_article_revision_article ON cms_article_revision(article_id, id);
CREATE INDEX idx_cms_working_draft_saved ON cms_working_draft(saved_at, article_id);
-- +goose StatementBegin
CREATE TRIGGER cms_article_revision_no_update BEFORE UPDATE ON cms_article_revision
BEGIN
    SELECT RAISE(ABORT, 'article revisions are immutable');
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER cms_article_revision_no_delete BEFORE DELETE ON cms_article_revision
BEGIN
    SELECT RAISE(ABORT, 'article revisions are immutable');
END;
-- +goose StatementEnd

-- +goose Down
DROP TABLE IF EXISTS cms_article_revision;
DROP TABLE IF EXISTS cms_working_draft;
DROP TABLE IF EXISTS cms_article;
