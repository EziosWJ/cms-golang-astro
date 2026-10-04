import { http } from "@/lib/http";
export type SiteConfigData = { siteName: string; description: string; publicUrl: string; language: string; timezone: string; authorName: string; authorBio: string; avatarMediaId: number | null };
export type WorkingSiteConfig = { version: number; data: SiteConfigData; savedAt: string; savedBy: number; revisionId: number | null };
export function getSiteConfig() { return http.get<WorkingSiteConfig>("/api/v1/site-config"); }
export function saveSiteConfig(expectedVersion: number, data: SiteConfigData) { return http.put<WorkingSiteConfig>("/api/v1/site-config", { expectedVersion, data }); }

export function getEditorTimezone(signal?: AbortSignal) { return http.get<{ timezone: string }>("/api/v1/site-config/editor-timezone", { signal }); }
