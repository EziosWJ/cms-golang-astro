import { http } from "@/lib/http";
import type { ApiPageResult } from "@/types/api";
import type { ArticleDetail, ArticleDraftInput, ArticleListRecord } from "@/types/content";

const BASE_PATH = "/api/v1/articles";

export function getArticlePage(query: { page: number; pageSize: number; title: string; lifecycle?: string; status?: string }) {
  return http.get<ApiPageResult<ArticleListRecord>>(BASE_PATH, { query });
}

export function getArticle(id: number, signal?: AbortSignal) {
  return http.get<ArticleDetail>(`${BASE_PATH}/${id}`, { signal });
}

export function createArticle(input: ArticleDraftInput & { requestKey?: string; createMode?: "manual" | "autosave" }) {
  return http.post<ArticleDetail>(BASE_PATH, input);
}

export function saveArticleDraft(id: number, expectedVersion: number, input: ArticleDraftInput, mode: "manual" | "autosave" = "manual") {
  return http.put<ArticleDetail>(`${BASE_PATH}/${id}/draft`, { ...input, expectedVersion, mode });
}

export type ArticleRevision = Omit<ArticleDetail["draft"], "savedAt" | "savedBy"> & { id: number; source: string; slug: string; createdAt: string; createdBy: number };
export function getArticleRevisions(id: number, page: number) { return http.get<ApiPageResult<Pick<ArticleRevision, "id" | "articleId" | "version" | "title" | "createdAt" | "createdBy" | "source">>>(`${BASE_PATH}/${id}/revisions`, { query: { page, pageSize: 20 } }); }
export function getArticleRevision(id: number, revision: number) { return http.get<ArticleRevision>(`${BASE_PATH}/${id}/revisions/${revision}`); }
export function restoreArticleRevision(id: number, revision: number, expectedVersion: number) { return http.post<ArticleDetail>(`${BASE_PATH}/${id}/revisions/${revision}/restore`, { expectedVersion }); }

export function setArticleLifecycle(id: number, expectedVersion: number, lifecycle: "active" | "archived") { return http.post<ArticleDetail>(`${BASE_PATH}/${id}/lifecycle`, { expectedVersion, lifecycle }); }
