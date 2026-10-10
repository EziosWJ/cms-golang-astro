import type { ArticleDraftInput } from "@/types/content";
import { createUUID } from "@/lib/uuid";
export type EditorValues = Omit<ArticleDraftInput, "displayDate"> & { displayDate: string };
export type RecoveryCopy = { values: EditorValues; baseline: EditorValues; version: number | null; createRequest?: ArticleDraftInput & { requestKey: string; createMode: "manual" | "autosave" }; savedAt: string };
export function recoveryKey(account: number, identity: string) { return `cms-recovery:${location.origin}:${account}:${identity}`; }
export function temporaryIdentity(account: number) {
 const key = recoveryKey(account, "new-tab");
 try { const existing = sessionStorage.getItem(key); if (existing) return existing; const id=createUUID(); sessionStorage.setItem(key,id); return id; } catch { return createUUID(); }
}
export function readRecovery(key: string): RecoveryCopy | null {
 const raw=localStorage.getItem(key); if (!raw) return null;
 const copy=JSON.parse(raw) as RecoveryCopy;
 if (!copy.values || !copy.baseline || typeof copy.values.markdown !== "string" || typeof copy.values.title !== "string" || !Array.isArray(copy.values.categoryIds) || !Array.isArray(copy.values.tagIds)) throw new Error("浏览器恢复副本格式无效，请清除副本后继续。");
 return copy;
}
