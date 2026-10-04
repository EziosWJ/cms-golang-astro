-- +goose Up
INSERT INTO sys_menu(parent_id,menu_name,menu_type,path,component,icon,permission_code,sort_order,visible,status,is_builtin)
VALUES(0,'旧内容导入授权','MENU','','','FileInput','content:import',10,0,1,1);
INSERT INTO sys_role_menu(role_id,menu_id) SELECT r.id,m.id FROM sys_role r CROSS JOIN sys_menu m WHERE r.role_code='ADMIN' AND r.deleted=0 AND m.permission_code='content:import';
-- +goose Down
DELETE FROM sys_role_menu WHERE menu_id IN(SELECT id FROM sys_menu WHERE permission_code='content:import' AND is_builtin=1);
DELETE FROM sys_menu WHERE permission_code='content:import' AND is_builtin=1;
