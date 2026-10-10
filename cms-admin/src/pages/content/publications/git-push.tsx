import { useAuthStore } from "@/store/auth-store";
import { useEffect, useRef, useState } from "react";
import { Link } from "react-router-dom";
import { createGitPush, getGitPush, getGitPushes, gitPushStatus, retryGitPush, type GitPushTask } from "@/api/git-export";
import { PermissionGuard } from "@/components/auth/permission-guard";
import { ContentCard } from "@/components/common/content-card";
import { DataTable } from "@/components/common/data-table";
import { Pagination } from "@/components/common/pagination";
import { Button } from "@/components/ui/button";
import { hasAnyPermission, hasPermission } from "@/lib/permission";
import { getErrorMessage, isApiError } from "@/lib/api-error";
import { createUUID } from "@/lib/uuid";
import type { DataTableColumn } from "@/types/table";

export function GitPushSection() {
  useAuthStore((state) => state.menus);
  const [tasks, setTasks] = useState<GitPushTask[]>([]);
  const [detail, setDetail] = useState<GitPushTask | null>(null);
  const [page, setPage] = useState(1);
  const [total, setTotal] = useState(0);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [message, setMessage] = useState("");
  const [refresh, setRefresh] = useState(0);
  const pending = useRef<{ key: string; id?: number } | null>(null);
  const [pendingVisible, setPendingVisible] = useState(false);
  const canView = hasPermission("integration:git:view");
  const allowed = hasAnyPermission(["integration:git:view", "integration:git:push"]);
  useEffect(() => {
    if (!canView) return;
    let active = true;
    const load = async () => { try { const result = await getGitPushes(page); if (active) { setTasks(result.records); setTotal(result.total); } } catch (e) { if (active) setError(getErrorMessage(e, "Git 推送记录读取失败")); } };
    void load(); const timer = window.setInterval(() => void load(), 3000);
    return () => { active = false; window.clearInterval(timer); };
  }, [page, refresh, canView]);
  async function submit(id?: number) {
    if (busy) return;
    if (!pending.current) pending.current = { key: createUUID(), id };
    if (pending.current.id !== id) { setError("请先确认上一请求的结果。"); return; }
    setBusy(true); setError("");
    try {
      const task = id ? await retryGitPush(id, pending.current.key) : await createGitPush(pending.current.key);
      pending.current = null; setPendingVisible(false); setMessage(task.status === "superseded" ? "原输入已被较新推送取代，请推送当前版本。" : `已接受 Git 推送任务 #${task.id}，构建结果请在 GitHub 查看。`);
      if (canView) setDetail(await getGitPush(task.id));
      setRefresh((v) => v + 1);
    } catch (e) { setError(getErrorMessage(e, "推送请求未确认，请重试同一请求。")); if (isApiError(e) && e.status && e.status >= 400 && e.status < 500) { pending.current = null; setPendingVisible(false); } else { setPendingVisible(true); } }
    finally { setBusy(false); }
  }
  const columns: DataTableColumn<GitPushTask>[] = [
    { title: "任务 / 本地版本", key: "id", render: (_, t) => `#${t.id} / Release #${t.releaseId}` },
    { title: "目标", key: "repository", render: (_, t) => `${t.repository} · ${t.branch}` },
    { title: "状态", key: "status", render: (_, t) => `${gitPushStatus[t.status] ?? t.status}${t.unchanged ? "（输入未变化）" : ""}` },
    { title: "源码", key: "sourceSha", render: (_, t) => t.sourceSha.slice(0, 12) },
    { title: "操作", key: "actions", render: (_, t) => <div className="flex flex-wrap gap-space-2"><Button variant="ghost" onClick={() => { void getGitPush(t.id).then(setDetail).catch((e) => setError(getErrorMessage(e, "详情读取失败"))); }}>详情</Button>{t.commitUrl && <a className="text-primary" href={t.commitUrl} target="_blank" rel="noreferrer">提交</a>}<PermissionGuard permissionCode="integration:git:push">{["failed", "interrupted"].includes(t.status) && <Button disabled={busy || pendingVisible} onClick={() => void submit(t.id)}>重试原输入</Button>}</PermissionGuard></div> },
  ];
  if (!allowed) return null;
  return <ContentCard className="mb-space-6" title="手动 Git 推送" description="固定当前成功本地发布的整站输入。Git 推送成功仅表示提交成功，Actions 构建在 GitHub 查看。" extra={<div className="flex flex-wrap gap-space-2"><PermissionGuard permissionCode="integration:git:manage"><Link to="/settings/git" className="text-primary">配置 GitHub</Link></PermissionGuard><PermissionGuard permissionCode="integration:git:push"><Button variant="primary" disabled={busy || pendingVisible} onClick={() => void submit()}>推送当前已发布站点</Button></PermissionGuard></div>}>
    {(error || message) && <p role={error ? "alert" : "status"} className="mb-space-3 text-sm">{error || message}</p>}
    {pendingVisible && <Button disabled={busy} onClick={() => void submit(pending.current?.id)}>重试同一请求</Button>}
    {canView && <><DataTable columns={columns} dataSource={tasks} rowKey="id" /><Pagination page={page} pageSize={10} total={total} onPageChange={setPage} />{detail && <div className="mt-space-4 space-y-space-2 border-t border-border pt-space-4 text-sm"><p>任务 #{detail.id} · {gitPushStatus[detail.status]}</p><p className="break-all">{detail.sourceRepository} @ {detail.sourceSha}</p><p>{detail.error}</p>{detail.commitUrl && <div className="flex flex-wrap gap-space-3"><a href={detail.commitUrl} target="_blank" rel="noreferrer" className="text-primary">查看提交</a><a href={`https://github.com/${detail.repository}/actions`} target="_blank" rel="noreferrer" className="text-primary">查看或重新运行 Actions</a></div>}{detail.attempts?.map((a) => <p key={a.id}>尝试 #{a.id} · {gitPushStatus[a.status]} {a.error}</p>)}<Button variant="ghost" onClick={() => void getGitPush(detail.id).then(setDetail).catch((e) => setError(getErrorMessage(e, "详情读取失败")))}>刷新详情</Button></div>}</>}
  </ContentCard>;
}
