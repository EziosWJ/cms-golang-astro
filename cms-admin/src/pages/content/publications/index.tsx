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
 return <ContentCard className="mb-space-6" title="发布执行器" description="暂停完成后才可运行维护命令；暂停期间提交的任务等待恢复。" extra={<PermissionGuard permissionCode="content:publish"><div className="flex gap-space-2"><Button variant="secondary" disabled={busy || !status || ["disabled", "paused", "pausing", "stopping", "stopped"].includes(status.state)} onClick={() => void control(pauseWorker)}>暂停</Button><Button disabled={busy || !status || !["paused", "blocked"].includes(status.state)} onClick={() => void control(resumeWorker)}>恢复</Button></div></PermissionGuard>}>
  <p role="status">{status ? workerState[status.state] ?? status.state : "加载中"}{status?.taskId ? ` · 当前任务 #${status.taskId}` : ""}</p>
  {(error || status?.error) && <p role="alert" className="mt-space-2 text-destructive">{error || status?.error}</p>}
 </ContentCard>;
}
export function PublicationsPage() { const [selected, setSelected] = useState<number | null>(null); const list = useListPage({ fetch: getPublications, defaultFilters: {}, toQuery: (_, page, pageSize) => ({ page, pageSize }) }); const columns: DataTableColumn<PublishTask>[] = [{title:"内容",key:"title",render:(_,value) => value.title || publicationKind[value.kind]}, { title: "操作", key: "kind", render: (_, value) => publicationKind[value.kind] }, { title: "状态", key: "status", render: (_, value) => publicationStatus[value.status] }, { title: "创建时间", key: "createdAt", render: (_, value) => new Date(value.createdAt).toLocaleString("zh-CN") }, { title: "查看", key: "action", render: (_, value) => <Button variant="ghost" onClick={() => setSelected(value.id)}>详情</Button> }]; return <><PageHeader title="发布与预览任务" description="任务固定提交时的内容；失败和中断后可明确重试原目标。" /><details className="mb-space-4"><summary className="cursor-pointer">执行器状态与维护</summary><WorkerControls /></details><DataTableCard toolbar={<TableToolbar title="持久化任务" actions={<Button onClick={list.reload}>刷新</Button>} />} pagination={<Pagination page={list.page} pageSize={list.pageSize} total={list.total} onPageChange={list.setPage} onPageSizeChange={list.setPageSize} />}><DataTable columns={columns} dataSource={list.data} rowKey="id" loading={list.loading} error={list.error} /></DataTableCard>{selected && <div className="mt-space-6"><PublicationTask key={selected} id={selected} onSuccess={list.reload} /></div>}</>; }
