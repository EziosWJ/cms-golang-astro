-- +goose Up
CREATE TABLE cms_article_create_request (
 actor_id BIGINT NOT NULL REFERENCES sys_user(id),
 request_key VARCHAR(100) NOT NULL,
 fingerprint VARCHAR(64) NOT NULL,
 article_id BIGINT NOT NULL REFERENCES cms_article(id),
 PRIMARY KEY (actor_id, request_key)
);
CREATE TABLE cms_media_upload_request (
 actor_id BIGINT NOT NULL REFERENCES sys_user(id), request_key VARCHAR(100) NOT NULL,
 fingerprint TEXT NOT NULL, file_id BIGINT NOT NULL REFERENCES sys_file(id), PRIMARY KEY(actor_id,request_key)
);
ALTER TABLE cms_article_revision ADD COLUMN source VARCHAR(20) NOT NULL DEFAULT 'manual';
-- +goose Down
ALTER TABLE cms_article_revision DROP COLUMN source;
DROP TABLE cms_media_upload_request;
DROP TABLE cms_article_create_request;
