-- +goose Up
-- 站点管理授权：content:site:manage 是「新增站点」「停用/启用站点」两个入口的闸门。
--
-- 必须 visible=1：前端只从 GET /api/auth/menus 取权限码，而
-- internal/auth/repository.go 的 FindVisibleMenusByUserID 只返回 visible=1 的行；
-- 若种成 visible=0，该权限码永远不会下发到前端，任何依赖菜单树的前端闸门都会
-- 恒为 false（这正是本票初版踩过的坑）。
--
-- 不占菜单位：与 seed/00008_publication_menu.sql 的 content:config:publish 同形——
-- path/component 指向它所属的既有页面（站点设置），并由前端
-- cms-admin/src/config/navigation.ts 的 toNavItem 显式排除，因此侧边栏不会多出条目。
-- 与 seed/00009_import_permission.sql 的 visible=0 写法不同，那一条没有前端闸门消费。
INSERT INTO sys_menu(parent_id,menu_name,menu_type,path,component,icon,permission_code,sort_order,visible,status,is_builtin)
VALUES(0,'站点管理授权','MENU','/content/site-config','content/site-config/index','Settings','content:site:manage',11,1,1,1);
INSERT INTO sys_role_menu(role_id,menu_id) SELECT r.id,m.id FROM sys_role r CROSS JOIN sys_menu m WHERE r.role_code='ADMIN' AND r.deleted=0 AND m.permission_code='content:site:manage';
-- +goose Down
DELETE FROM sys_role_menu WHERE menu_id IN(SELECT id FROM sys_menu WHERE permission_code='content:site:manage' AND is_builtin=1);
DELETE FROM sys_menu WHERE permission_code='content:site:manage' AND is_builtin=1;
