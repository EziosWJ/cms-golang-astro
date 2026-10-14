import { http } from "@/lib/http";

/** 本地站点的站点标识。内容权威恒为 local，编辑器链路只服务它。 */
export const LOCAL_SITE_KEY = "local";

/**
 * 站点标识的协议值域。与后端 `siteconfig.Validate` 的 `siteKeyPattern` 保持一致：
 * 1–63 位小写字母、数字或短横线，且以字母或数字开头。
 */
export const SITE_KEY_PATTERN = /^[a-z0-9][a-z0-9-]{0,62}$/;
export const SITE_KEY_ERROR =
  "站点标识须为 1–63 位小写字母、数字或短横线，且以字母或数字开头";

export type ThemeDefinition = { id: string; name: string; version: string; description: string; preview: string; compatibility: { cmsThemeApi: string }; capabilities: string[] };
export type SiteConfigData = { siteName: string; description: string; publicUrl: string; language: string; timezone: string; authorName: string; authorBio: string; avatarMediaId: number | null; theme: string; themeVersion: string };

/** 配置修订。既是读取视图，也是产物契约，字段不得改动。 */
export type ConfigRevision = { id: number; version: number; data: SiteConfigData; createdAt: string; createdBy: number };

/**
 * 一个站点的配置工作副本。
 *
 * - `role`：`local`（内容权威）或 `remote`。
 * - `published`：仅 local 有意义；非 local 恒为 `null`（不存在「已上线配置」）。
 * - `note`：仅非 local 出现，是「保存即生效于下一次交付、不产生发布任务」的官方文案。
 */
export type WorkingSiteConfig = {
  siteKey: string;
  role: string;
  status: number;
  version: number;
  data: SiteConfigData;
  savedAt: string;
  savedBy: number;
  revisionId: number | null;
  published: ConfigRevision | null;
  note?: string;
};

/** 站点集合的列表项。非 local 的 `published` 恒为 false。 */
export type SiteSummary = { siteKey: string; role: string; status: number; siteName: string; publicUrl: string; version: number; published: boolean };

export function getSiteConfig() { return http.get<WorkingSiteConfig>("/api/v1/site-config"); }
export function getThemes() { return http.get<ThemeDefinition[]>("/api/v1/site-config/themes"); }
export function saveSiteConfig(expectedVersion: number, data: SiteConfigData) { return http.put<WorkingSiteConfig>("/api/v1/site-config", { expectedVersion, data }); }

export function getSites() { return http.get<SiteSummary[]>("/api/v1/site-config/sites"); }
export function getSite(siteKey: string) { return http.get<WorkingSiteConfig>(`/api/v1/site-config/sites/${encodeURIComponent(siteKey)}`); }
export function saveSite(siteKey: string, expectedVersion: number, data: SiteConfigData) { return http.put<WorkingSiteConfig>(`/api/v1/site-config/sites/${encodeURIComponent(siteKey)}`, { expectedVersion, data }); }
export function createSite(siteKey: string, data: SiteConfigData) { return http.post<WorkingSiteConfig>("/api/v1/site-config/sites", { siteKey, data }); }
export function setSiteStatus(siteKey: string, status: number) { return http.post<WorkingSiteConfig>(`/api/v1/site-config/sites/${encodeURIComponent(siteKey)}/status`, { status }); }

/**
 * 读取指定站点的工作副本。
 *
 * local 走「默认站点」路由（`/site-config`），其余站点按 siteKey 寻址。保持 local 走
 * 默认路由既符合后端契约（内容权威恒为 local），也让既有回归脚本断言的一次 PUT 成立。
 */
export function loadSiteWorking(siteKey: string) {
  return siteKey === LOCAL_SITE_KEY ? getSiteConfig() : getSite(siteKey);
}

/** 保存指定站点的工作副本；与 {@link loadSiteWorking} 同一套 local/远端分流。 */
export function saveSiteWorking(siteKey: string, expectedVersion: number, data: SiteConfigData) {
  return siteKey === LOCAL_SITE_KEY ? saveSiteConfig(expectedVersion, data) : saveSite(siteKey, expectedVersion, data);
}

export function getEditorTimezone(signal?: AbortSignal) { return http.get<{ timezone: string }>("/api/v1/site-config/editor-timezone", { signal }); }

/** 编辑器上下文只服务 local：内容权威恒为 local，不会给远端站点拼文章链接。 */
export function getEditorContext(signal?: AbortSignal) { return http.get<{ siteName:string; timezone:string; published:ConfigRevision | null }>("/api/v1/site-config/editor-context", { signal }); }
