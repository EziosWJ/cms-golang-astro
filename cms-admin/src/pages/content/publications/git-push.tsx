import { useAuthStore } from "@/store/auth-store";
import { useEffect, useRef, useState } from "react";
import { Link } from "react-router-dom";
import { createGitPush, getGitPush, getGitPushes, gitPushStatus, retryGitPush, type GitPushTask } from "@/api/git-export";
import { PermissionGuard } from "@/components/auth/permission-guard";
import { ContentCard } from "@/components/common/content-card";
import { DataTable } from "@/components/common/data-table";
import { Pagination } from "@/components/common/pagination";
import { Button } from "@/components/ui/button";
import { useListPage } from "@/hooks/use-list-page";
import { hasAnyPermission, hasPermission } from "@/lib/permission";
import { getErrorMessage, isApiError } from "@/lib/api-error";
import { createUUID } from "@/lib/uuid";
import { idlePushSession, nextPushSession, pendingIdentity, type PushOutcome, type PushPending, type PushSession } from "@/lib/git-push-pending";
import type { DataTableColumn } from "@/types/table";

const POLL_INTERVAL = 3000;

export function GitPushSection() {
  useAuthStore((state) => state.menus);
  const { data: tasks, total, loading, error: listFailure, page, pageSize, setPage, setPageSize, reload } = useListPage({ fetch: (query: { page: number; pageSize: number }) => getGitPushes(query.page, query.pageSize), defaultFilters: {}, toQuery: (_filters, page, pageSize) => ({ page, pageSize }) });
  const [detail, setDetail] = useState<GitPushTask | null>(null);
  const session = useRef<PushSession>(idlePushSession);
  const [retryVisible, setRetryVisible] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [message, setMessage] = useState("");
  const canView = hasPermission("integration:git:view");
  const allowed = hasAnyPermission(["integration:git:view", "integration:git:push"]);
  const pageCount = Math.max(Math.ceil(total / pageSize), 1);

  // 后台轮询只刷新数据，不用骨架屏替换已有表格。
  useEffect(() => {
    const timer = window.setInterval(() => reload(), POLL_INTERVAL);
    return () => window.clearInterval(timer);
  }, [reload]);
  // 记录被清理后当前页可能超出范围，回到最后一个有效页。
  useEffect(() => {
    if (page > pageCount) setPage(pageCount);
  }, [page, pageCount, setPage]);

  function apply(outcome: PushOutcome) {
    const next = nextPushSession(session.current, outcome);
    session.current = next;
    setRetryVisible(next.retryVisible);
  }
  function begin(identity: PushPending) {
    session.current = { pending: identity, retryVisible: false };
    setRetryVisible(false);
  }
  async function loadDetail(id: number, failureMessage: string) {
    try { setDetail(await getGitPush(id)); setError(""); }
    catch (e) { setError(getErrorMessage(e, failureMessage)); }
  }
  async function submit(id?: number) {
    if (busy) return;
    const identity = pendingIdentity(session.current, id, createUUID);
    if (!identity) { setError("请先确认上一请求的结果。"); return; }
    begin(identity);
    setBusy(true); setError("");
    let accepted: GitPushTask;
    try {
      accepted = id ? await retryGitPush(id, identity.key) : await createGitPush(identity.key);
    } catch (e) {
      // 请求结果未知或被拒绝：保留的幂等键决定重试是重放同一请求还是重新开始。
      const rejected = isApiError(e) && typeof e.status === "number" && e.status >= 400 && e.status < 500;
      apply(rejected ? "rejected" : "unconfirmed");
      setError(getErrorMessage(e, rejected ? "推送请求被拒绝。" : "推送请求未确认，请重试同一请求。"));
      setBusy(false);
      return;
    }
    apply("accepted");
    setMessage(accepted.status === "superseded" ? "原输入已被较新推送取代，请推送当前版本。" : `已接受 Git 推送任务 #${accepted.id}，构建结果请在 GitHub 查看。`);
    if (page !== 1) setPage(1); else reload();
    try {
      setDetail(await getGitPush(accepted.id));
      apply("detail-loaded");
    } catch (e) {
      // 任务已经建立，只保留原任务与幂等键，重试不会新建当前版本任务。
      apply("detail-failed");
      setError(getErrorMessage(e, `任务 #${accepted.id} 已接受，详情读取失败，请重试同一请求。`));
    }
    setBusy(false);
  }
  const columns: DataTableColumn<GitPushTask>[] = [
    { title: "任务 / 本地版本", key: "id", render: (_, t) => `#${t.id} / Release #${t.releaseId}` },
    { title: "目标", key: "repository", render: (_, t) => `${t.repository} · ${t.branch}` },
    { title: "状态", key: "status", render: (_, t) => `${gitPushStatus[t.status] ?? t.status}${t.unchanged ? "（输入未变化）" : ""}` },
    { title: "源码", key: "sourceSha", render: (_, t) => t.sourceSha.slice(0, 12) },
    { title: "操作", key: "actions", render: (_, t) => <div className="flex flex-wrap gap-space-2"><Button variant="ghost" onClick={() => void loadDetail(t.id, "详情读取失败")}>详情</Button>{t.commitUrl && <a className="text-primary" href={t.commitUrl} target="_blank" rel="noreferrer">提交</a>}<PermissionGuard permissionCode="integration:git:push">{["failed", "interrupted"].includes(t.status) && <Button disabled={busy || retryVisible} onClick={() => void submit(t.id)}>重试原输入</Button>}</PermissionGuard></div> },
  ];
  if (!allowed) return null;
  const listError = tasks.length === 0 ? listFailure : "";
  return <ContentCard className="mb-space-6" title="手动 Git 推送" description="固定当前成功本地发布的整站输入。Git 推送成功仅表示提交成功，Actions 构建在 GitHub 查看。" extra={<div className="flex flex-wrap gap-space-2"><PermissionGuard permissionCode="integration:git:manage"><Link to="/settings/git" className="text-primary">配置 GitHub</Link></PermissionGuard><PermissionGuard permissionCode="integration:git:push"><Button variant="primary" disabled={busy || retryVisible} onClick={() => void submit()}>推送当前已发布站点</Button></PermissionGuard></div>}>
    {(error || listError || message) && <p role={error || listError ? "alert" : "status"} className="mb-space-3 text-sm">{error || listError || message}</p>}
    {retryVisible && <Button disabled={busy} onClick={() => void submit(session.current.pending?.id)}>重试同一请求</Button>}
    {canView && <><DataTable columns={columns} dataSource={tasks} rowKey="id" loading={loading && tasks.length === 0} error={listError || undefined} /><Pagination page={page} pageSize={pageSize} total={total} disabled={busy} onPageChange={setPage} onPageSizeChange={setPageSize} />{detail && <div className="mt-space-4 space-y-space-2 border-t border-border pt-space-4 text-sm"><p>任务 #{detail.id} · {gitPushStatus[detail.status]}</p><p className="break-all">{detail.sourceRepository} @ {detail.sourceSha}</p><p>{detail.error}</p>{detail.commitUrl && <div className="flex flex-wrap gap-space-3"><a href={detail.commitUrl} target="_blank" rel="noreferrer" className="text-primary">查看提交</a><a href={`https://github.com/${detail.repository}/actions`} target="_blank" rel="noreferrer" className="text-primary">查看或重新运行 Actions</a></div>}{detail.attempts?.map((a) => <p key={a.id}>尝试 #{a.id} · {gitPushStatus[a.status]} {a.error}</p>)}<Button variant="ghost" onClick={() => void loadDetail(detail.id, "详情读取失败")}>刷新详情</Button></div>}</>}
  </ContentCard>;
}
