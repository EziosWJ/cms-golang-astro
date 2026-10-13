import { getWorkerStatus, pauseWorker, resumeWorker, workerState, type WorkerStatus } from "@/api/publishing";
import { ContentCard } from "@/components/common/content-card";
import { PermissionGuard } from "@/components/auth/permission-guard";
import { getErrorMessage } from "@/lib/api-error";
import { useEffect, useState } from "react";
import { getPublications, publicationKind, publicationStatus, type PublishTask } from "@/api/publishing";
import { useListPage } from "@/hooks/use-list-page";
import { PageHeader } from "@/components/common/page-header";
import { DataTableCard } from "@/components/common/data-table-card";
import { DataTable } from "@/components/common/data-table";
import { TableToolbar } from "@/components/common/table-toolbar";
import { Pagination } from "@/components/common/pagination";
import { Button } from "@/components/ui/button";
import type { DataTableColumn } from "@/types/table";
import { PublicationTask } from "./task";
import { EmptyState } from "@/components/common/empty-state";
import { StatusTag } from "@/components/common/status-tag";
import { AlertTriangle, CheckCircle2, CircleHelp, RefreshCw, Wrench } from "lucide-react";

const taskStatusTone: Record<string, "success" | "warning" | "error" | "info" | "neutral"> = {
 succeeded: "success",
 queued: "info",
 running: "warning",
 failed: "error",
 interrupted: "warning",
};

function statusLabel(status: string) {
 return publicationStatus[status] ?? status;
}

function WorkerControls() {
 const [status, setStatus] = useState<WorkerStatus | null>(null);
 const [error, setError] = useState("");
 const [busy, setBusy] = useState(false);
 useEffect(() => {
  let active = true;
  const load = async () => { try { const value = await getWorkerStatus(); if (active) { setStatus(value); setError(""); } } catch (e) { if (active) setError(getErrorMessage(e, "执行器操作失败")); } };
  void load(); const timer = window.setInterval(() => void load(), 2000);
  return () => { active = false; window.clearInterval(timer); };
 }, []);
 async function control(action: () => Promise<WorkerStatus>) {
  setBusy(true); try { setStatus(await action()); setError(""); } catch (e) { setError(getErrorMessage(e, "执行器操作失败")); } finally { setBusy(false); }
 }
 const state = status?.state;
 const blocked = state === "blocked";
 const unavailable = !status || ["disabled", "unavailable", "lock_occupied", "stopped"].includes(state ?? "");
 const healthy = state === "running" || state === "starting";
 return <ContentCard className="mb-space-6" title="发布服务" description="发布、下线和预览任务会在这里排队执行。" extra={<div className="flex items-center gap-space-2"><Button size="sm" variant="ghost" aria-label="重新检查发布服务" title="重新检查" disabled={busy} onClick={() => void control(getWorkerStatus)}><RefreshCw className="h-4 w-4" aria-hidden /></Button><details className="relative"><summary className="flex h-control-sm cursor-pointer list-none items-center gap-space-1 rounded-control border border-border px-space-3 text-sm text-text-secondary hover:bg-neutral-background"><Wrench className="h-4 w-4" aria-hidden />维护</summary><div className="absolute right-0 z-10 mt-space-2 w-64 rounded-admin border border-border bg-surface p-space-3 shadow-admin"><p className="mb-space-3 text-xs text-text-tertiary">暂停后，当前任务完成，新的任务会等待恢复。</p><PermissionGuard permissionCode="content:publish"><div className="flex gap-space-2"><Button size="sm" variant="secondary" disabled={busy || !status || ["disabled", "paused", "pausing", "stopping", "stopped", "blocked"].includes(state ?? "")} onClick={() => void control(pauseWorker)}>暂停执行</Button><Button size="sm" disabled={busy || !status || !["paused", "blocked"].includes(state ?? "")} onClick={() => void control(resumeWorker)}>{blocked ? "重新恢复" : "继续执行"}</Button></div></PermissionGuard></div></details></div>}>
  <div className={`flex items-start gap-space-3 rounded-control border px-space-4 py-space-3 ${blocked || unavailable ? "border-error-border bg-error-background" : healthy ? "border-success-border bg-success-background" : "border-warning-border bg-warning-background"}`} role="status">
   {blocked || unavailable ? <AlertTriangle className="mt-0.5 h-5 w-5 shrink-0 text-error" aria-hidden /> : healthy ? <CheckCircle2 className="mt-0.5 h-5 w-5 shrink-0 text-success" aria-hidden /> : <CircleHelp className="mt-0.5 h-5 w-5 shrink-0 text-warning" aria-hidden />}
   <div className="min-w-0"><p className="font-medium text-text-primary">{status ? workerState[state!] ?? state : "正在检查发布服务…"}{status?.taskId ? ` · 当前任务 #${status.taskId}` : ""}</p><p className="mt-1 text-sm text-text-secondary">{blocked ? "发布记录与部署文件不一致。修复部署记录后点击“重新恢复”，新的发布会暂时停止。" : unavailable ? "当前无法确认发布执行器是否可用，请重新检查或联系系统管理员。" : state === "paused" ? "执行器已暂停，排队中的任务会在继续执行后处理。" : "当前可以提交新的发布、下线或预览任务。"}</p></div>
  </div>
  {(error || status?.error) && <p role="alert" className="mt-space-2 break-words text-sm text-error">{error || status?.error}</p>}
 </ContentCard>;
}
export function PublicationsPage() { const [selected, setSelected] = useState<number | null>(null); const list = useListPage({ fetch: getPublications, defaultFilters: {}, toQuery: (_, page, pageSize) => ({ page, pageSize }) });
 useEffect(() => { if (!list.data.some((task) => ["queued", "running"].includes(task.status))) return; const timer = window.setInterval(list.reload, 3000); return () => window.clearInterval(timer); }, [list.data, list.reload]);
 const columns: DataTableColumn<PublishTask>[] = [{title:"内容",key:"title",render:(_,value) => <div><p className="font-medium text-text-primary">{value.title || publicationKind[value.kind]}</p><p className="mt-1 text-xs text-text-tertiary">{publicationKind[value.kind]} · 任务 #{value.id}</p></div>}, { title: "状态", key: "status", render: (_, value) => <StatusTag tone={taskStatusTone[value.status] ?? "neutral"}>{statusLabel(value.status)}</StatusTag> }, { title: "提交时间", key: "createdAt", render: (_, value) => <span className="whitespace-nowrap text-sm text-text-secondary">{new Date(value.createdAt).toLocaleString("zh-CN")}</span> }, { title: "操作", key: "action", align: "right", render: (_, value) => <Button variant="ghost" onClick={() => setSelected(value.id)}>{["failed", "interrupted"].includes(value.status) ? "查看并重试" : "查看详情"}</Button> }];
 return <><PageHeader title="发布与预览任务" description="查看任务进度。失败或中断后，可以重试提交时固定的版本。" /><WorkerControls /><DataTableCard toolbar={<TableToolbar title="任务记录" description="发布、下线和预览任务按提交时间排列。" actions={<Button variant="secondary" onClick={list.reload}><RefreshCw className="mr-space-1 h-4 w-4" aria-hidden />刷新</Button>} />} pagination={list.total > 0 ? <Pagination page={list.page} pageSize={list.pageSize} total={list.total} onPageChange={list.setPage} onPageSizeChange={list.setPageSize} /> : undefined}><DataTable columns={columns} dataSource={list.data} rowKey="id" loading={list.loading} error={list.error} empty={<EmptyState title="暂无发布任务" description="在文章编辑页点击“发布”或“预览”，任务会显示在这里。" actionText="前往文章管理" onAction={() => { window.location.href = "/content/articles"; }} />} /></DataTableCard>{selected && <div className="mt-space-6"><PublicationTask key={selected} id={selected} onSuccess={list.reload} /></div>}</>; }
