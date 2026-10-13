import { http } from "@/lib/http";
import type { ApiPageResult } from "@/types/api";
import type { FileRecord, FileUploadOptions, FileUploadBatchResult } from "@/types/file";
export type MediaKind = "all" | "image" | "attachment";
export type MediaSort = "newest" | "name" | "size";
export type MediaRecord = { id: number; originalName: string; mimeType: string; fileSize: number; status: number; stablePath: string; image: boolean; createTime?: string };
export function getMediaPage(page = 1, name = "", kind: MediaKind = "all", sort: MediaSort = "newest") {
  return http.get<ApiPageResult<MediaRecord>>("/api/v1/media", {
    query: { page, pageSize: 20, name, kind: kind === "all" ? undefined : kind, sort },
  });
}
export function getMedia(id: number) { return http.get<MediaRecord>(`/api/v1/media/${id}`); }
const uploadIdentities = new WeakMap<File,string>();
export function uploadMedia(file: File, options?: FileUploadOptions) { const body = new FormData(); body.append("file", file); if (options?.businessModule) body.append("businessModule", options.businessModule); let key=uploadIdentities.get(file); if(!key) { key=crypto.randomUUID(); uploadIdentities.set(file,key); } return http.post<FileRecord>("/api/v1/media/upload", body, { headers:{"Idempotency-Key":key} }); }
export function uploadMediaBatch(files: File[]) { const body = new FormData(); files.forEach((file) => body.append("files", file)); body.append("businessModule", "attachment"); return http.post<FileUploadBatchResult>("/api/v1/media/upload-batch", body); }
export function deleteMedia(id: number) { return http.delete<void>(`/api/v1/media/${id}`); }

export type MediaReference = { ownerType:string; ownerId:number; articleId:number | null; title:string };
export function getMediaReferences(id:number) { return http.get<MediaReference[]>(`/api/v1/media/${id}/references`); }
