import { http } from "@/lib/http";
import type { ApiPageResult } from "@/types/api";
import type { ArticleDraftInput } from "@/types/content";
export type PublishKind = "config" | "article" | "preview" | "unpublish";
export type PublishAttempt = { id: number; status: string; baselineReleaseId: number | null; releaseKey: string; manifestHash: string; switchIntent: number; error: string; createdAt: string; finishedAt: string | null };
export type PublishTask = { id: number; kind: PublishKind; articleId: number | null; revisionId: number | null; configRevisionId: number | null; status: string; error: string; createdAt: string; updatedAt: string; attempts?: PublishAttempt[] };
export type PublishInput = { kind: PublishKind; articleId?: number; configRevisionId?: number; save?: ArticleDraftInput & { expectedVersion: number; mode: "manual" } };
export function submitPublication(input: PublishInput, key: string) { return http.post<PublishTask>("/api/v1/publications", input, { headers: { "Idempotency-Key": key } }); }
export function getPublications(query: { page: number; pageSize: number }) { return http.get<ApiPageResult<PublishTask>>("/api/v1/publications", { query }); }
export function getPublication(id: number, signal?: AbortSignal) { return http.get<PublishTask>(`/api/v1/publications/${id}`, { signal }); }
export function retryPublication(id: number, key: string) { return http.post<PublishTask>(`/api/v1/publications/${id}/retry`, undefined, { headers: { "Idempotency-Key": key } }); }
export function getPreviewAccess(id: number) { return http.post<{ url: string; expiresAt: string }>(`/api/v1/previews/${id}/access`); }
export const publicationKind = { config: "站点配置", article: "文章发布", preview: "私有预览", unpublish: "文章下线" };
export const publicationStatus: Record<string, string> = { queued: "等待 worker", running: "构建中", succeeded: "已完成", failed: "失败", interrupted: "中断" };

export type WorkerStatus = { state: string; error: string; taskId?: number };
export function getWorkerStatus() { return http.get<WorkerStatus>("/api/v1/publication-worker"); }
export function pauseWorker() { return http.post<WorkerStatus>("/api/v1/publication-worker/pause"); }
export function resumeWorker() { return http.post<WorkerStatus>("/api/v1/publication-worker/resume"); }
export const workerState: Record<string, string> = { disabled: "内置执行器已禁用，使用独立 worker", starting: "启动中", running: "运行中", pausing: "暂停中，等待当前任务结束", paused: "已暂停，可以执行维护；新任务等待恢复", unavailable: "构建环境不可用", lock_occupied: "执行权被其他进程占用", retrying: "故障后等待恢复", blocked: "发布已阻塞，请修复后恢复", stopping: "停止中", stopped: "已停止" };
