import { ArrowUpRight } from "lucide-react";
import { Link } from "react-router-dom";
import { PageHeader } from "@/components/common/page-header";
import { convertUserMenusToNavItems, type NavItem } from "@/config/navigation";
import { useAuthStore } from "@/store/auth-store";

function getAuthorizedEntries(items: NavItem[]): NavItem[] {
  return items.flatMap((item) => {
    if (item.children?.length) return getAuthorizedEntries(item.children);
    return item.path ? [item] : [];
  });
}

export function DashboardPage() {
  const user = useAuthStore((state) => state.user);
  const menus = useAuthStore((state) => state.menus);
  const entries = getAuthorizedEntries(convertUserMenusToNavItems(menus));
  const displayName = user?.nickname || user?.username || "用户";

  return (
    <>
      <PageHeader
        title={`欢迎回来，${displayName}`}
        description="当前账号已登录。以下入口来自已授权菜单。"
      />

      {entries.length > 0 ? (
        <nav aria-label="已授权系统入口" className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
          {entries.map((entry) => {
            const Icon = entry.icon;
            const content = (
              <>
                <span className="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-primary/10 text-primary">
                  <Icon className="h-5 w-5" aria-hidden />
                </span>
                <span className="min-w-0 flex-1">
                  <span className="block truncate font-medium text-text-primary">
                    {entry.label}
                  </span>
                  <span className="mt-1 block truncate text-sm text-text-tertiary">
                    {entry.path}
                  </span>
                </span>
                <ArrowUpRight className="h-4 w-4 shrink-0 text-text-tertiary" aria-hidden />
              </>
            );

            const className = "flex min-h-20 items-center gap-3 rounded-admin border border-border bg-surface p-4 shadow-admin transition-colors hover:border-primary/40 hover:bg-neutral-background focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary";

            return entry.externalUrl ? (
              <a
                key={`${entry.externalUrl}:${entry.label}`}
                href={entry.externalUrl}
                target="_blank"
                rel="noreferrer"
                className={className}
              >
                {content}
              </a>
            ) : (
              <Link key={entry.path} to={entry.path} className={className}>
                {content}
              </Link>
            );
          })}
        </nav>
      ) : (
        <section className="rounded-admin border border-border bg-surface px-5 py-8 text-sm text-text-tertiary shadow-admin">
          暂无已授权菜单入口。
        </section>
      )}
    </>
  );
}
