import type { ApiBuiltinFlag, ApiPageRequest, ApiStatus } from "./api";

export type RoleStatus = "enabled" | "disabled";

export type RoleListRecord = {
  id: number;
  roleName: string;
  roleCode: string;
  status: ApiStatus;
  sortOrder: number;
  isBuiltin: ApiBuiltinFlag;
  remark?: string | null;
  createTime?: string | null;
};

export type RoleDetailRecord = RoleListRecord & {
  menuIds: number[];
};

export type RoleListQuery = Partial<ApiPageRequest> & {
  roleName?: string;
  roleCode?: string;
  status?: ApiStatus;
};

export type RoleCreateRequest = {
  roleName: string;
  roleCode: string;
  status: ApiStatus;
  sortOrder: number;
  remark?: string;
};

export type RoleUpdateRequest = RoleCreateRequest;

export type RoleStatusRequest = {
  status: ApiStatus;
};

export type RoleBatchDeleteRequest = {
  ids: number[];
};

export type RoleAssignMenusRequest = {
  menuIds: number[];
};

export type SystemMenuType = "DIR" | "MENU" | "LINK";

export type SystemMenuRecord = {
  id: number;
  parentId: number;
  menuName: string;
  menuType: SystemMenuType;
  path?: string | null;
  component?: string | null;
  icon?: string | null;
  permissionCode?: string | null;
  sortOrder: number;
  visible: ApiStatus;
  status: ApiStatus;
  externalUrl?: string | null;
  isBuiltin?: ApiBuiltinFlag;
  remark?: string | null;
  createTime?: string | null;
  updateTime?: string | null;
  children?: SystemMenuRecord[];
};

export type SystemMenuTreeRecord = SystemMenuRecord & {
  children?: SystemMenuTreeRecord[];
};

export type SystemMenuListQuery = Partial<ApiPageRequest> & {
  menuName?: string;
  menuType?: SystemMenuType;
  status?: ApiStatus;
  visible?: ApiStatus;
};

export type SystemMenuCreateRequest = {
  parentId: number;
  menuName: string;
  menuType: SystemMenuType;
  path?: string;
  component?: string;
  icon?: string;
  permissionCode?: string;
  sortOrder: number;
  visible: ApiStatus;
  status: ApiStatus;
  externalUrl?: string;
  remark?: string;
};

export type SystemMenuUpdateRequest = SystemMenuCreateRequest;

export type SystemMenuStatusRequest = {
  status: ApiStatus;
};

export type SystemMenuBatchDeleteRequest = {
  ids: number[];
};
