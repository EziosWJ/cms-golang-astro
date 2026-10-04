-- +goose Up
INSERT INTO sys_menu (parent_id,menu_name,menu_type,path,component,icon,permission_code,sort_order,visible,status,is_builtin)
VALUES (0,'分类与标签','MENU','/content/taxonomy','content/taxonomy/index','BookOpen','content:taxonomy:edit',5,1,1,1);
INSERT INTO sys_role_menu(role_id,menu_id) SELECT r.id,m.id FROM sys_role r CROSS JOIN sys_menu m WHERE r.role_code='ADMIN' AND r.deleted=0 AND m.permission_code='content:taxonomy:edit';
-- +goose Down
DELETE FROM sys_role_menu WHERE menu_id IN (SELECT id FROM sys_menu WHERE permission_code='content:taxonomy:edit' AND is_builtin=1);
DELETE FROM sys_menu WHERE permission_code='content:taxonomy:edit' AND is_builtin=1;
