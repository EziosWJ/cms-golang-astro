-- +goose Up
INSERT INTO cms_publish_state(id,version)VALUES(1,1);
INSERT INTO sys_menu(parent_id,menu_name,menu_type,path,component,icon,permission_code,sort_order,visible,status,is_builtin)
VALUES(0,'发布任务','MENU','/content/publications','content/publications/index','ListChecks','content:publish',8,1,1,1),
(0,'配置发布授权','MENU','/content/site-config','content/site-config/index','Settings','content:config:publish',9,1,1,1);
INSERT INTO sys_role_menu(role_id,menu_id) SELECT r.id,m.id FROM sys_role r CROSS JOIN sys_menu m WHERE r.role_code='ADMIN' AND r.deleted=0 AND m.permission_code IN('content:publish','content:config:publish');
-- +goose Down
DELETE FROM sys_role_menu WHERE menu_id IN(SELECT id FROM sys_menu WHERE permission_code IN('content:publish','content:config:publish') AND is_builtin=1);
DELETE FROM sys_menu WHERE permission_code IN('content:publish','content:config:publish') AND is_builtin=1;
DELETE FROM cms_publish_state WHERE id=1;
