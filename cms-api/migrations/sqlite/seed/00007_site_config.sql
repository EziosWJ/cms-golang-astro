-- +goose Up
INSERT INTO cms_site_config(id,version,data,saved_at,saved_by) VALUES(1,1,'{"siteName":"CMS 博客","description":"","publicUrl":"http://localhost:8080","language":"zh-CN","timezone":"Asia/Shanghai","authorName":"作者","authorBio":"","avatarMediaId":null}',CURRENT_TIMESTAMP,1);
INSERT INTO sys_menu(parent_id,menu_name,menu_type,path,component,icon,permission_code,sort_order,visible,status,is_builtin)
VALUES(0,'站点配置','MENU','/content/site-config','content/site-config/index','Settings','content:config:edit',7,1,1,1);
INSERT INTO sys_role_menu(role_id,menu_id) SELECT r.id,m.id FROM sys_role r CROSS JOIN sys_menu m WHERE r.role_code='ADMIN' AND r.deleted=0 AND m.permission_code='content:config:edit';
-- +goose Down
DELETE FROM sys_role_menu WHERE menu_id IN(SELECT id FROM sys_menu WHERE permission_code='content:config:edit' AND is_builtin=1);
DELETE FROM sys_menu WHERE permission_code='content:config:edit' AND is_builtin=1;
DELETE FROM cms_site_config WHERE id=1;
