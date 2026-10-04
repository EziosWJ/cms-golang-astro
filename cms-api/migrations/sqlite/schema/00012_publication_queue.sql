-- +goose Up
CREATE TABLE cms_publish_state(id INTEGER PRIMARY KEY CHECK(id=1),current_release_id INTEGER,version INTEGER NOT NULL DEFAULT 1,blocked_reason TEXT NOT NULL DEFAULT '');
CREATE TABLE cms_publish_task(
 id INTEGER PRIMARY KEY AUTOINCREMENT,kind VARCHAR(20) NOT NULL CHECK(kind IN('config','article','unpublish','preview')),
 article_id INTEGER REFERENCES cms_article(id),revision_id INTEGER REFERENCES cms_article_revision(id),config_revision_id INTEGER REFERENCES cms_config_revision(id),
 target_snapshot TEXT NOT NULL DEFAULT '{}',
 status VARCHAR(20) NOT NULL CHECK(status IN('queued','running','succeeded','failed','interrupted')),error TEXT NOT NULL DEFAULT '',
 created_at DATETIME NOT NULL,created_by INTEGER NOT NULL REFERENCES sys_user(id),updated_at DATETIME NOT NULL
);
CREATE INDEX idx_cms_publish_task_queue ON cms_publish_task(status,id);
CREATE INDEX idx_cms_publish_task_article ON cms_publish_task(article_id,status);
CREATE TABLE cms_publish_attempt(
 id INTEGER PRIMARY KEY AUTOINCREMENT,task_id INTEGER NOT NULL REFERENCES cms_publish_task(id),status VARCHAR(20) NOT NULL,
 baseline_release_id INTEGER,release_key VARCHAR(64) NOT NULL UNIQUE,manifest TEXT NOT NULL DEFAULT '{}',manifest_hash VARCHAR(64) NOT NULL DEFAULT '',
 switch_intent SMALLINT NOT NULL DEFAULT 0 CHECK(switch_intent IN(0,1)),error TEXT NOT NULL DEFAULT '',created_at DATETIME NOT NULL,finished_at DATETIME
);
CREATE INDEX idx_cms_publish_attempt_task ON cms_publish_attempt(task_id,id);
CREATE TABLE cms_release(id INTEGER PRIMARY KEY AUTOINCREMENT,attempt_id INTEGER NOT NULL UNIQUE REFERENCES cms_publish_attempt(id),release_key VARCHAR(64) NOT NULL UNIQUE,manifest TEXT NOT NULL,manifest_hash VARCHAR(64) NOT NULL,created_at DATETIME NOT NULL,cleaned_at DATETIME);
CREATE TABLE cms_publish_request(key VARCHAR(200) PRIMARY KEY,request_hash VARCHAR(64) NOT NULL,task_id INTEGER NOT NULL REFERENCES cms_publish_task(id),created_at DATETIME NOT NULL);
CREATE TABLE cms_published_article(article_id INTEGER PRIMARY KEY REFERENCES cms_article(id),revision_id INTEGER NOT NULL REFERENCES cms_article_revision(id),first_published_at DATETIME NOT NULL,updated_at DATETIME NOT NULL);
CREATE TABLE cms_preview_access (
 token_hash TEXT PRIMARY KEY, task_id BIGINT NOT NULL REFERENCES cms_publish_task(id),
 user_id BIGINT NOT NULL REFERENCES sys_user(id), jti TEXT NOT NULL, expires_at TIMESTAMP NOT NULL
);
CREATE INDEX idx_preview_access_expiry ON cms_preview_access(expires_at);

-- +goose Down
DROP TABLE IF EXISTS cms_preview_access;
DROP TABLE cms_published_article;
DROP TABLE cms_publish_request;
DROP TABLE cms_release;
DROP TABLE cms_publish_attempt;
DROP TABLE cms_publish_task;
DROP TABLE cms_publish_state;
