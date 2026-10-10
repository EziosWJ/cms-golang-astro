-- +goose Up
INSERT INTO sys_menu(parent_id,menu_name,menu_type,path,component,icon,permission_code,sort_order,visible,status,is_builtin)
VALUES(0,'GitHub 连接','MENU','/settings/git','settings/git','Github','integration:git:manage',10,1,1,1),
(0,'Git 推送授权','MENU','','','Upload','integration:git:push',11,1,1,1),
(0,'Git 推送记录授权','MENU','','','History','integration:git:view',12,1,1,1);
INSERT INTO sys_role_menu(role_id,menu_id) SELECT r.id,m.id FROM sys_role r CROSS JOIN sys_menu m WHERE r.role_code='ADMIN' AND r.deleted=0 AND m.permission_code IN('integration:git:manage','integration:git:push','integration:git:view');
-- +goose Down
DELETE FROM sys_role_menu WHERE menu_id IN(SELECT id FROM sys_menu WHERE permission_code IN('integration:git:manage','integration:git:push','integration:git:view') AND is_builtin=1);
DELETE FROM sys_menu WHERE permission_code IN('integration:git:manage','integration:git:push','integration:git:view') AND is_builtin=1;
