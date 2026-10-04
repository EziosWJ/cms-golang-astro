-- +goose Up
CREATE TABLE cms_media_ref (
 owner_type VARCHAR(20) NOT NULL, owner_id BIGINT NOT NULL,
 file_id BIGINT NOT NULL REFERENCES sys_file(id) ON DELETE RESTRICT,
 PRIMARY KEY(owner_type,owner_id,file_id)
);
CREATE INDEX idx_cms_media_ref_file ON cms_media_ref(file_id);
ALTER TABLE cms_working_draft ADD COLUMN cover_media_id BIGINT REFERENCES sys_file(id) ON DELETE RESTRICT;
ALTER TABLE cms_article_revision ADD COLUMN cover_media_id BIGINT REFERENCES sys_file(id) ON DELETE RESTRICT;
-- +goose Down
ALTER TABLE cms_article_revision DROP COLUMN cover_media_id;
ALTER TABLE cms_working_draft DROP COLUMN cover_media_id;
DROP TABLE cms_media_ref;
