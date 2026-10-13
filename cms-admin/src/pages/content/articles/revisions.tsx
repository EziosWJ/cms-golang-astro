import type { ArticleDraftInput } from "@/types/content";
import { DraftComparison } from "./comparison";
import { useEffect, useRef, useState } from "react";
import { useLocation, useNavigate, useParams } from "react-router-dom";
import { ArrowLeft, Pencil } from "lucide-react";
import { getArticle, getArticleRevision, getArticleRevisions, restoreArticleRevision, type ArticleRevision } from "@/api/content";
import { getEditorContext } from "@/api/site-config";
import { PermissionGuard } from "@/components/auth/permission-guard";
import { ContentCard } from "@/components/common/content-card";
import { EmptyState } from "@/components/common/empty-state";
import { Pagination } from "@/components/common/pagination";
import { PageHeader } from "@/components/common/page-header";
import { ConfirmDialog } from "@/components/common/confirm-dialog";
import { Button } from "@/components/ui/button";
import { formatContentDate } from "@/lib/content-date";
import { ApiError, getErrorMessage } from "@/lib/api-error";
import { ARTICLE_EDIT_PERMISSION, type ArticleDetail } from "@/types/content";

export function RevisionHistory({ current, timezone, articleId, version, disabled, dirty, onRestore }: { current: ArticleDraftInput; timezone:string; articleId: number; version: number; disabled: boolean; dirty: boolean; onRestore: (id: number) => Promise<void> }) {
  const [page, setPage] = useState(1);
  const [total, setTotal] = useState(0);
  const [records, setRecords] = useState<Pick<ArticleRevision, "id" | "title" | "version" | "createdAt" | "createdBy" | "source">[]>([]);
  const [selected, setSelected] = useState<ArticleRevision | null>(null);
  const [error, setError] = useState("");
  const selectionRequest=useRef(0);
  const [confirm, setConfirm] = useState(false);
  useEffect(() => {
    let active = true;
    getArticleRevisions(articleId, page).then((result) => { if (active) { setRecords(result.records); setTotal(result.total); setError(""); } }).catch((err: unknown) => { if (active) setError(getErrorMessage(err, "加载修订失败")); });
    return () => { active = false; };
  }, [articleId, page, version]);
  async function select(id: number) {
    const sequence=++selectionRequest.current;
    try { const result=await getArticleRevision(articleId,id); if(sequence===selectionRequest.current) { setSelected(result); setError(""); } }
    catch (err) { if(sequence===selectionRequest.current)setError(getErrorMessage(err, "读取修订失败")); }
  }
  return <ContentCard title="历史版本" description="自动保存不会新增修订。恢复只改变工作稿。">
    {error && <p role="alert" className="text-error">{error}</p>}
    <div className="flex flex-wrap gap-space-2">{records.map((record) => <Button className="max-w-full" title={record.title} key={record.id} size="sm" onClick={() => void select(record.id)}><span className="min-w-0 truncate">{new Date(record.createdAt).toLocaleString("zh-CN")} · {record.source === "publish" ? "发布" : "保存版本"} · {record.title || "未命名文章"}</span></Button>)}</div>
    <Pagination page={page} pageSize={20} total={total} onPageChange={setPage} />
    {selected && <div className="mt-space-4 space-y-space-3"><p>{selected.title || "未命名文章"} · Slug：{selected.slug}</p><p>摘要：{selected.summary || "无"} · 展示日期：{selected.displayDate || "未设置"}</p><DraftComparison local={current} remote={{ title:selected.title, markdown:selected.markdown, summary:selected.summary, slug:selected.slug, displayDate:formatContentDate(selected.displayDate,timezone), coverMediaId:selected.coverMediaId, categoryIds:(selected.taxonomy ?? []).filter((t) => t.kind === "category").map((t) => t.id), tagIds:(selected.taxonomy ?? []).filter((t) => t.kind === "tag").map((t) => t.id) }} /><Button disabled={disabled} onClick={() => setConfirm(true)}>恢复到工作稿</Button></div>}
    <ConfirmDialog open={confirm} title="恢复所选修订？" description={dirty ? "当前未保存内容会被所选修订替换，线上版本保持不变。" : "所选修订会写入工作稿，线上版本保持不变。"} onCancel={() => setConfirm(false)} onConfirm={() => { setConfirm(false); if (selected) void onRestore(selected.id); }} />
  </ContentCard>;
}

function draftInputFrom(detail: ArticleDetail, timezone: string): ArticleDraftInput {
  return {
    coverMediaId: detail.draft.coverMediaId,
    categoryIds: (detail.draft.taxonomy ?? []).filter((term) => term.kind === "category").map((term) => term.id),
    tagIds: (detail.draft.taxonomy ?? []).filter((term) => term.kind === "tag").map((term) => term.id),
    title: detail.draft.title,
    slug: detail.slug,
    markdown: detail.draft.markdown,
    summary: detail.draft.summary,
    displayDate: formatContentDate(detail.draft.displayDate, timezone),
  };
}

export function ArticleRevisionHistoryPage() {
  const { id } = useParams();
  const location = useLocation();
  const navigate = useNavigate();
  const listReturn = new URLSearchParams(location.search).get("list");
  const listPath = listReturn && /^\/content\/articles(?:\?[^#]*)?$/.test(listReturn) ? listReturn : "/content/articles";
  const [detail, setDetail] = useState<ArticleDetail | null>(null);
  const [timezone, setTimezone] = useState("Asia/Shanghai");
  const [loading, setLoading] = useState(true);
  const [restoring, setRestoring] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    const controller = new AbortController();
    if (!/^\d+$/.test(id ?? "") || Number(id) < 1) {
      setError("文章不存在");
      setLoading(false);
      return () => controller.abort();
    }
    Promise.all([getArticle(Number(id), controller.signal), getEditorContext(controller.signal)]).then(([article, context]) => {
      if (controller.signal.aborted) return;
      setDetail(article);
      setTimezone(context.timezone);
      setError("");
    }).catch((reason: unknown) => {
      if (!controller.signal.aborted) setError(getErrorMessage(reason, "加载文章历史失败"));
    }).finally(() => {
      if (!controller.signal.aborted) setLoading(false);
    });
    return () => controller.abort();
  }, [id]);

  async function restore(revisionID: number) {
    if (!detail || restoring) return;
    setRestoring(true);
    setError("");
    try {
      await restoreArticleRevision(detail.id, revisionID, detail.draft.version);
      navigate(`/content/articles/${detail.id}?list=${encodeURIComponent(listPath)}`);
    } catch (reason: unknown) {
      if (reason instanceof ApiError && reason.status === 409 && typeof reason.data === "object" && reason.data !== null && "reason" in reason.data && reason.data.reason === "version_conflict") {
        setError("工作稿已被其他操作修改，请返回文章后重新读取历史版本。");
      } else {
        setError(getErrorMessage(reason, "恢复失败，工作稿未改变"));
      }
    } finally {
      setRestoring(false);
    }
  }

  return (
    <PermissionGuard permissionCode={ARTICLE_EDIT_PERMISSION} fallback={<EmptyState title="无访问权限" description="请联系管理员授予内容编辑权限。" />}>
      <PageHeader
        title="历史版本"
        description={detail ? `${detail.draft.title || "未命名文章"} · 对比历史修订，恢复后返回编辑器。` : "查看文章的保存版本和发布版本。"}
        actions={<><Button size="sm" onClick={() => navigate(listPath)}><ArrowLeft className="h-4 w-4" aria-hidden />返回列表</Button>{detail && <Button size="sm" onClick={() => navigate(`/content/articles/${detail.id}?list=${encodeURIComponent(listPath)}`)}><Pencil className="h-4 w-4" aria-hidden />编辑文章</Button>}</>}
      />
      {loading && <p role="status" className="text-text-secondary">正在加载文章历史…</p>}
      {!loading && error && !detail && <EmptyState title="加载失败" description={error} />}
      {!loading && detail && <div className="space-y-space-4">
        {error && <p role="alert" className="text-error">{error}</p>}
        <RevisionHistory current={draftInputFrom(detail, timezone)} timezone={timezone} articleId={detail.id} version={detail.draft.version} disabled={restoring} dirty={false} onRestore={restore} />
      </div>}
    </PermissionGuard>
  );
}
