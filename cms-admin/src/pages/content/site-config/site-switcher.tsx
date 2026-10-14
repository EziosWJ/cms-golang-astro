import type { SiteSummary } from "@/api/site-config";
import { StatusTag } from "@/components/common/status-tag";
import { cn } from "@/lib/utils";

type SiteSwitcherProps = {
  sites: SiteSummary[];
  selectedKey: string;
  disabled?: boolean;
  onSelect: (siteKey: string) => void;
};

/**
 * 站点切换控件。
 *
 * 复用 `src/pages/content/taxonomy/index.tsx` 的页签模式（`role="tablist"` +
 * `aria-selected`）：站点是同一页面内并列的一组实体，与「分类 / 标签」同形，
 * 切换即切换下半部分的配置面板。每个站点同时带出角色与启停用标识，
 * `role="local"` 一定有「本地站点」标签。
 */
export function SiteSwitcher({ sites, selectedKey, disabled, onSelect }: SiteSwitcherProps) {
  return (
    <div role="tablist" aria-label="站点列表" className="flex flex-wrap gap-space-2">
      {sites.map((site) => {
        const selected = site.siteKey === selectedKey;
        return (
          <button
            key={site.siteKey}
            type="button"
            role="tab"
            aria-selected={selected}
            disabled={disabled}
            onClick={() => onSelect(site.siteKey)}
            className={cn(
              "flex min-w-[10rem] flex-col items-start gap-space-2 rounded-admin border px-space-3 py-space-2 text-left transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary disabled:cursor-not-allowed disabled:opacity-60",
              selected
                ? "border-primary bg-primary/5"
                : "border-border bg-surface hover:border-primary/60",
            )}
          >
            <span className={cn("truncate text-sm font-medium", selected ? "text-primary" : "text-text-primary")}>
              {site.siteName?.trim() || site.siteKey}
            </span>
            <span className="flex flex-wrap items-center gap-space-1">
              {site.role === "local" ? (
                <StatusTag tone="info">本地站点</StatusTag>
              ) : (
                <StatusTag tone="neutral">远端站点</StatusTag>
              )}
              {site.status === 0 && <StatusTag tone="warning">已停用</StatusTag>}
            </span>
          </button>
        );
      })}
    </div>
  );
}
