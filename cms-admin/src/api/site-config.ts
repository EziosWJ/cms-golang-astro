import { http } from "@/lib/http";
export type ThemeDefinition = { id: string; name: string; version: string; description: string; preview: string; compatibility: { cmsThemeApi: string }; capabilities: string[] };
export type SiteConfigData = { siteName: string; description: string; publicUrl: string; language: string; timezone: string; authorName: string; authorBio: string; avatarMediaId: number | null; theme: string; themeVersion: string };
export type WorkingSiteConfig = { published: { id:number; data:SiteConfigData } | null; version: number; data: SiteConfigData; savedAt: string; savedBy: number; revisionId: number | null };
export function getSiteConfig() { return http.get<WorkingSiteConfig>("/api/v1/site-config"); }
export function getThemes() { return http.get<ThemeDefinition[]>("/api/v1/site-config/themes"); }
export function saveSiteConfig(expectedVersion: number, data: SiteConfigData) { return http.put<WorkingSiteConfig>("/api/v1/site-config", { expectedVersion, data }); }

export function getEditorTimezone(signal?: AbortSignal) { return http.get<{ timezone: string }>("/api/v1/site-config/editor-timezone", { signal }); }

export function getEditorContext(signal?: AbortSignal) { return http.get<{ siteName:string; timezone:string; published:{id:number; data:SiteConfigData} | null }>("/api/v1/site-config/editor-context", { signal }); }
