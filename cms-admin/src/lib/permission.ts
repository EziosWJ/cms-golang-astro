import { useAuthStore } from "@/store/auth-store";

function getCurrentPermissionCodes(): Set<string> {
  const codes = new Set<string>();
  const visit = (menus: ReturnType<typeof useAuthStore.getState>["menus"]) => {
    menus.forEach((menu) => {
      if (menu.permissionCode) codes.add(menu.permissionCode);
      if (menu.children.length) visit(menu.children);
    });
  };

  visit(useAuthStore.getState().menus);
  return codes;
}

export function hasPermission(permissionCode: string): boolean {
  return getCurrentPermissionCodes().has(permissionCode);
}

export function hasAnyPermission(permissionCodes: string[]): boolean {
  return permissionCodes.some((code) => hasPermission(code));
}

export function hasAllPermissions(permissionCodes: string[]): boolean {
  return permissionCodes.every((code) => hasPermission(code));
}
