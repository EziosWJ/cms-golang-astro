import type { ArticleDraftInput, TaxonomySnapshot } from "@/types/content";
import { useCallback, useEffect, useRef, useState } from "react";
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
import { toast } from "@/components/common/toast-store";
import { ARTICLE_EDIT_PERMISSION, type ArticleDetail } from "@/types/content";

type RevisionField = keyof ArticleDraftInput;
type RevisionRecord = Pick<ArticleRevision, "id" | "title" | "version" | "createdAt" | "createdBy" | "source">;

const fieldDefinitions: { key: RevisionField; label: string; group: "基础信息" | "内容" | "元数据" }[] = [
  { key: "title", label: "标题", group: "基础信息" },
  { key: "slug", label: "文章地址", group: "基础信息" },
  { key: "summary", label: "摘要", group: "基础信息" },
  { key: "displayDate", label: "展示日期", group: "基础信息" },
  { key: "markdown", label: "正文", group: "内容" },
  { key: "categoryIds", label: "分类", group: "元数据" },
  { key: "tagIds", label: "标签", group: "元数据" },
  { key: "coverMediaId", label: "封面", group: "元数据" },
];

function sourceLabel(source: string) {
  if (source === "publish") return "发布版本";
  if (source === "autosave") return "自动保存";
  return "手动保存";
}

function sourceClass(source: string) {
  return source === "publish" ? "bg-primary/10 text-primary" : "bg-neutral-background text-text-secondary";
}

function timelineTime(value: string) {
  return new Date(value).toLocaleString("zh-CN", { month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit" });
}

function revisionInputFrom(revision: ArticleRevision, timezone: string): ArticleDraftInput {
  return {
    coverMediaId: revision.coverMediaId,
    categoryIds: (revision.taxonomy ?? []).filter((term) => term.kind === "category").map((term) => term.id),
    tagIds: (revision.taxonomy ?? []).filter((term) => term.kind === "tag").map((term) => term.id),
    title: revision.title,
    slug: revision.slug,
    markdown: revision.markdown,
    summary: revision.summary,
    displayDate: formatContentDate(revision.displayDate, timezone),
  };
}

function normalizedFieldValue(input: ArticleDraftInput, key: RevisionField) {
  const value = input[key];
  return key === "categoryIds" || key === "tagIds" ? [...(value as number[])].sort((a, b) => a - b) : value;
}

function fieldChanged(current: ArticleDraftInput, revision: ArticleDraftInput, key: RevisionField) {
  return JSON.stringify(normalizedFieldValue(current, key)) !== JSON.stringify(normalizedFieldValue(revision, key));
}

function changedFieldLabels(current: ArticleDraftInput, revision: ArticleDraftInput) {
  return fieldDefinitions.filter((field) => fieldChanged(current, revision, field.key)).map((field) => field.label);
}

function taxonomyNames(taxonomy: TaxonomySnapshot[], kind: "category" | "tag") {
  const names = taxonomy.filter((term) => term.kind === kind).map((term) => term.name).filter(Boolean);
  return names.length ? names.join("、") : "未设置";
}

function displayFieldValue(input: ArticleDraftInput, taxonomy: TaxonomySnapshot[], key: RevisionField) {
  if (key === "categoryIds") return taxonomyNames(taxonomy, "category");
  if (key === "tagIds") return taxonomyNames(taxonomy, "tag");
  if (key === "coverMediaId") return input.coverMediaId ? `媒体 #${input.coverMediaId}` : "未设置";
  if (key === "markdown") return input.markdown || "（空）";
  return input[key] || "未设置";
}

type LineDiffKind = "same" | "added" | "removed";
type LineDiffRow = { kind: LineDiffKind; value: string; currentLine: number | null; revisionLine: number | null };

function markdownLines(value: string) {
  return value ? value.replace(/\r\n?/g, "\n").split("\n") : [];
}

function markdownLineDiff(currentValue: string, revisionValue: string): LineDiffRow[] {
  const currentLines = markdownLines(currentValue);
  const revisionLines = markdownLines(revisionValue);
  const trace: Map<number, number>[] = [];
  let frontier = new Map<number, number>([[1, 0]]);

  for (let distance = 0; distance <= currentLines.length + revisionLines.length; distance += 1) {
    const next = new Map<number, number>();
    for (let diagonal = -distance; diagonal <= distance; diagonal += 2) {
      let currentIndex: number;
      if (diagonal === -distance || (diagonal !== distance && (frontier.get(diagonal - 1) ?? -1) < (frontier.get(diagonal + 1) ?? -1))) {
        currentIndex = frontier.get(diagonal + 1) ?? 0;
      } else {
        currentIndex = (frontier.get(diagonal - 1) ?? 0) + 1;
      }
      let revisionIndex = currentIndex - diagonal;
      while (currentIndex < currentLines.length && revisionIndex < revisionLines.length && currentLines[currentIndex] === revisionLines[revisionIndex]) {
        currentIndex += 1;
        revisionIndex += 1;
      }
      next.set(diagonal, currentIndex);
      if (currentIndex >= currentLines.length && revisionIndex >= revisionLines.length) {
        trace.push(next);
        const reversed: Omit<LineDiffRow, "currentLine" | "revisionLine">[] = [];
        let x = currentLines.length;
        let y = revisionLines.length;
        for (let step = trace.length - 1; step > 0; step -= 1) {
          const previous = trace[step - 1];
          const diagonalAtStep = x - y;
          const movedDown = diagonalAtStep === -step || (diagonalAtStep !== step && (previous.get(diagonalAtStep - 1) ?? -1) < (previous.get(diagonalAtStep + 1) ?? -1));
          const previousDiagonal = movedDown ? diagonalAtStep + 1 : diagonalAtStep - 1;
          const previousX = previous.get(previousDiagonal) ?? 0;
          const previousY = previousX - previousDiagonal;
          while (x > previousX && y > previousY) {
            reversed.push({ kind: "same", value: currentLines[x - 1] });
            x -= 1;
            y -= 1;
          }
          if (movedDown) {
            reversed.push({ kind: "removed", value: revisionLines[y - 1] });
            y -= 1;
          } else {
            reversed.push({ kind: "added", value: currentLines[x - 1] });
            x -= 1;
          }
        }
        while (x > 0 && y > 0) {
          reversed.push({ kind: "same", value: currentLines[x - 1] });
          x -= 1;
          y -= 1;
        }
        while (x > 0) {
          reversed.push({ kind: "added", value: currentLines[x - 1] });
          x -= 1;
        }
        while (y > 0) {
          reversed.push({ kind: "removed", value: revisionLines[y - 1] });
          y -= 1;
        }
        let currentLine = 0;
        let revisionLine = 0;
        return reversed.reverse().map((row) => {
          if (row.kind === "added") {
            currentLine += 1;
            return { ...row, currentLine, revisionLine: null };
          }
          if (row.kind === "removed") {
            revisionLine += 1;
            return { ...row, currentLine: null, revisionLine };
          }
          currentLine += 1;
          revisionLine += 1;
          return { ...row, currentLine, revisionLine };
        });
      }
    }
    trace.push(next);
    frontier = next;
  }
  return [];
}

function MarkdownDiff({ current, revision }: { current: string; revision: string }) {
  const rows = markdownLineDiff(current, revision);
  if (!rows.length) return <p className="rounded-control border border-dashed border-border px-space-3 py-space-4 text-sm text-text-tertiary">两边都是空正文。</p>;
  const currentEmpty = markdownLines(current).length === 0;
  const revisionEmpty = markdownLines(revision).length === 0;
  return <div className="overflow-hidden rounded-control border border-border">
    <div className="grid grid-cols-2 border-b border-border bg-neutral-background text-xs font-medium text-text-secondary">
      <div className="border-r border-border px-space-3 py-space-2">
        <span>当前工作稿（当前）</span>
        {currentEmpty && <span className="ml-space-2 font-normal text-text-tertiary">空正文</span>}
      </div>
      <div className="px-space-3 py-space-2">
        <span>所选历史修订（将恢复）</span>
        {revisionEmpty && <span className="ml-space-2 font-normal text-text-tertiary">空正文</span>}
      </div>
    </div>
    <div className="flex flex-wrap items-center gap-x-space-4 gap-y-space-1 border-b border-border px-space-3 py-space-2 text-xs text-text-secondary">
      <span className="inline-flex items-center gap-space-1"><span className="h-2.5 w-2.5 rounded-sm border border-error bg-error-background" aria-hidden="true" />恢复后移除当前内容</span>
      <span className="inline-flex items-center gap-space-1"><span className="h-2.5 w-2.5 rounded-sm border border-success bg-success-background" aria-hidden="true" />恢复后加入历史内容</span>
      <span className="text-text-tertiary">恢复后以右侧历史修订为准</span>
    </div>
    <div className="max-h-[460px] overflow-auto font-mono text-sm leading-6">
      {rows.map((row, index) => {
        const currentStyle = row.kind === "added" ? "border-l-2 border-error bg-error-background text-text-primary" : row.kind === "removed" ? "bg-neutral-background text-text-tertiary" : "text-text-primary";
        const revisionStyle = row.kind === "removed" ? "border-l-2 border-success bg-success-background text-text-primary" : row.kind === "added" ? "bg-neutral-background text-text-tertiary" : "text-text-primary";
        const line = (value: string, number: number | null, style: string, empty: boolean, marker: string) => <div className={`grid min-w-0 grid-cols-[1rem_2rem_minmax(0,1fr)] gap-space-2 border-b border-border/60 px-space-2 last:border-b-0 ${style}`}><span className="select-none text-center text-xs font-semibold text-text-tertiary">{marker}</span><span className="select-none text-right text-xs text-text-tertiary">{number ?? ""}</span><span className={`min-w-0 whitespace-pre-wrap break-words ${empty ? "text-text-tertiary" : ""}`}>{empty ? "—" : value || " "}</span></div>;
        return <div key={`${row.kind}-${index}`} className="grid min-w-[720px] grid-cols-2">
          <div className="border-r border-border">{line(row.kind === "removed" ? "" : row.value, row.currentLine, currentStyle, row.kind === "removed", row.kind === "added" ? "−" : "")}</div>
          <div>{line(row.kind === "added" ? "" : row.value, row.revisionLine, revisionStyle, row.kind === "added", row.kind === "removed" ? "+" : "")}</div>
        </div>;
      })}
    </div>
  </div>;
}

function RevisionDiff({ current, currentTaxonomy, revision, timezone }: { current: ArticleDraftInput; currentTaxonomy: TaxonomySnapshot[]; revision: ArticleRevision; timezone: string }) {
  const revisionInput = revisionInputFrom(revision, timezone);
  const changed = fieldDefinitions.filter((field) => fieldChanged(current, revisionInput, field.key));
  const [showUnchanged, setShowUnchanged] = useState(false);
  const visible = showUnchanged ? fieldDefinitions : changed;
  const groups = ["基础信息", "内容", "元数据"] as const;

  return (
    <div className="space-y-space-5">
      <div className="flex flex-wrap items-center justify-between gap-space-3 rounded-control bg-neutral-background px-space-4 py-space-3">
        <p className="text-sm text-text-secondary">{changed.length ? `当前工作稿相对所选修订有 ${changed.length} 处变化` : "当前工作稿与所选修订一致"}</p>
        {!!(fieldDefinitions.length - changed.length) && <Button size="sm" variant="ghost" onClick={() => setShowUnchanged((value) => !value)}>{showUnchanged ? "仅显示变化字段" : "显示全部字段差异"}</Button>}
      </div>
      {changed.length === 0 && !showUnchanged && <p className="rounded-control border border-dashed border-border px-space-4 py-space-5 text-sm text-text-tertiary">当前工作稿与此历史版本没有差异。</p>}
      {groups.map((group) => {
        const fields = visible.filter((field) => field.group === group);
        if (!fields.length) return null;
        return <section key={group} className="space-y-space-3"><h3 className="text-sm font-semibold text-text-primary">{group}</h3>{fields.map((field) => {
          const isChanged = fieldChanged(current, revisionInput, field.key);
          if (!isChanged) return <div key={field.key} className="flex items-center gap-space-2 rounded-control border border-border px-space-3 py-space-2 text-sm text-text-secondary"><span className="text-success" aria-hidden="true">✓</span><span>{field.label}</span><span className="text-text-tertiary">未变化</span></div>;
          return <div key={field.key} className="space-y-space-2">
            <div className="flex flex-wrap items-center gap-space-2"><h4 className="text-sm font-medium text-text-primary">{field.label}</h4><span className="text-xs text-warning">已修改</span></div>
            {field.key === "markdown" ? <MarkdownDiff current={current.markdown} revision={revisionInput.markdown} /> : <div className="grid min-w-0 gap-space-3 md:grid-cols-2"><div className="min-w-0"><p className="mb-space-1 text-xs text-text-tertiary">当前工作稿（当前）</p><pre className="max-h-64 overflow-auto whitespace-pre-wrap break-words rounded-control border border-primary/15 bg-primary/5 p-space-3 text-sm">{displayFieldValue(current, currentTaxonomy, field.key)}</pre></div><div className="min-w-0"><p className="mb-space-1 text-xs text-text-tertiary">所选历史修订（将恢复）</p><pre className="max-h-64 overflow-auto whitespace-pre-wrap break-words rounded-control border border-warning/20 bg-warning/5 p-space-3 text-sm">{displayFieldValue(revisionInput, revision.taxonomy ?? [], field.key)}</pre></div></div>}
          </div>;
        })}</section>;
      })}
    </div>
  );
}

export function RevisionHistory({ current, currentTaxonomy, currentSavedAt, timezone, articleId, version, disabled, dirty, onRestore }: { current: ArticleDraftInput; currentTaxonomy: TaxonomySnapshot[]; currentSavedAt: string; timezone: string; articleId: number; version: number; disabled: boolean; dirty: boolean; onRestore: (id: number) => Promise<void> }) {
  const [page, setPage] = useState(1);
  const [total, setTotal] = useState(0);
  const [records, setRecords] = useState<RevisionRecord[]>([]);
  const [selected, setSelected] = useState<ArticleRevision | null>(null);
  const [error, setError] = useState("");
  const [selectedLoading, setSelectedLoading] = useState(false);
  const selectionRequest = useRef(0);
  const selectedRef = useRef<number | null>(null);
  const [confirm, setConfirm] = useState(false);
  const select = useCallback(async (id: number) => {
    const sequence = ++selectionRequest.current;
    setSelectedLoading(true);
    try { const result = await getArticleRevision(articleId, id); if (sequence === selectionRequest.current) { selectedRef.current = id; setSelected(result); setError(""); } }
    catch (err) { if (sequence === selectionRequest.current) setError(getErrorMessage(err, "读取修订失败")); }
    finally { if (sequence === selectionRequest.current) setSelectedLoading(false); }
  }, [articleId]);
  useEffect(() => {
    selectedRef.current = null;
    setSelected(null);
    setPage(1);
  }, [articleId]);
  useEffect(() => {
    let active = true;
    getArticleRevisions(articleId, page).then((result) => { if (active) { setRecords(result.records); setTotal(result.total); setError(""); if (selectedRef.current === null && result.records[0]) void select(result.records[0].id); } }).catch((err: unknown) => { if (active) setError(getErrorMessage(err, "加载修订失败")); });
    return () => { active = false; };
  }, [articleId, page, select, version]);
  const selectedChanges = selected ? changedFieldLabels(current, revisionInputFrom(selected, timezone)) : [];
  return <div className="grid min-w-0 gap-space-4 lg:grid-cols-[280px_minmax(0,1fr)]">
    <aside className="min-w-0">
      <ContentCard title="历史修订" description="手动保存和发布的版本可恢复；自动保存只更新当前工作稿。">
        {error && <p role="alert" className="mb-space-3 text-sm text-error">{error}</p>}
        <div className="rounded-control border border-primary/15 bg-primary/5 px-space-3 py-space-3">
          <div className="flex items-center justify-between gap-space-2">
            <span className="font-medium text-text-primary">当前工作稿</span>
            <span className="shrink-0 text-xs text-primary">对比基准</span>
          </div>
          <div className="mt-space-1 flex min-w-0 items-center gap-space-2 text-xs">
            <span className="min-w-0 truncate text-text-secondary">{current.title || "未命名文章"}</span>
            <span className="shrink-0 tabular-nums text-text-tertiary">{timelineTime(currentSavedAt)}</span>
          </div>
        </div>
        <div className="mt-space-4">
          <p className="mb-space-2 text-xs font-medium text-text-tertiary">可恢复版本</p>
          <div className="relative space-y-space-1 pl-space-1 before:absolute before:bottom-3 before:left-[9px] before:top-3 before:w-px before:bg-border">
            {records.length ? records.map((record, index) => <button type="button" key={record.id} title={record.title} onClick={() => void select(record.id)} className={`relative flex w-full min-w-0 items-start gap-space-3 rounded-control px-space-2 py-space-2 text-left transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary ${selected?.id === record.id ? "bg-neutral-background" : "hover:bg-neutral-background"}`}><span className={`relative z-10 mt-1 h-2.5 w-2.5 shrink-0 rounded-full border-2 ${selected?.id === record.id ? "border-primary bg-primary" : "border-border bg-surface"}`} aria-hidden="true" /><span className="min-w-0 flex-1"><span className="flex min-w-0 items-center gap-space-2"><span className={`truncate rounded-full px-space-2 py-0.5 text-xs font-medium ${sourceClass(record.source)}`}>{sourceLabel(record.source)}</span>{index === 0 && <span className="shrink-0 text-xs text-primary">最新</span>}</span><span className="mt-0.5 flex min-w-0 items-center gap-space-2 text-xs"><span className="min-w-0 truncate text-text-secondary">{record.title || "未命名文章"}</span><span className="shrink-0 tabular-nums text-text-tertiary">{timelineTime(record.createdAt)}</span></span></span></button>) : <p className="py-space-4 pl-space-5 text-sm text-text-tertiary">暂无可恢复版本。</p>}
          </div>
        </div>
        <Pagination compact className="px-0 md:flex-col md:items-stretch md:justify-start" page={page} pageSize={20} total={total} onPageChange={setPage} />
      </ContentCard>
    </aside>
    <ContentCard bodyClassName="min-h-[560px]" title={selected ? (selected.title || "未命名文章") : "版本详情"} description={selected ? `${sourceLabel(selected.source)} · ${new Date(selected.createdAt).toLocaleString("zh-CN")} · 所选历史修订` : "从左侧选择一个版本查看变化。"} extra={selected && <Button variant="primary" disabled={disabled || selectedLoading || selectedChanges.length === 0} onClick={() => setConfirm(true)}>{selectedChanges.length ? "将此修订恢复为当前工作稿" : "当前已是此修订"}</Button>}>
      {selected && !selectedLoading && <div className="mb-space-4 space-y-space-2 rounded-control border border-primary/15 bg-primary/5 px-space-3 py-space-3"><div className="flex flex-wrap items-center gap-x-space-2 gap-y-space-1"><span className="text-xs font-medium text-primary">恢复方向</span><span className="text-sm text-text-primary">所选历史修订 → 当前工作稿</span></div><p className="text-sm text-text-secondary">{selectedChanges.length ? `当前工作稿相对所选修订有 ${selectedChanges.length} 处变化，确认后将由右侧内容替换当前工作稿。` : "当前工作稿与所选修订一致，无需恢复。"}</p><p className="text-xs text-text-tertiary">线上已发布内容不受影响。</p><p className="text-xs text-text-tertiary">变化字段：{selectedChanges.length ? selectedChanges.join("、") : "无"}</p></div>}
      {selectedLoading && <p role="status" className="mb-space-4 text-sm text-text-secondary">正在加载版本详情…</p>}
      {selected && !selectedLoading && <RevisionDiff current={current} currentTaxonomy={currentTaxonomy} revision={selected} timezone={timezone} />}
      {!selected && !selectedLoading && <p className="py-space-10 text-center text-sm text-text-tertiary">选择一个历史版本后，这里会显示正文和元信息的变化。</p>}
    </ContentCard>
    <ConfirmDialog open={confirm} title="将此修订恢复为当前工作稿？" description={selected ? `${dirty ? "当前未保存的编辑内容会被替换。" : "当前工作稿会被替换。"}所选历史修订：${new Date(selected.createdAt).toLocaleString("zh-CN")} · ${sourceLabel(selected.source)}。线上已发布内容保持不变。` : "所选历史修订会写入当前工作稿，线上已发布内容保持不变。"} loading={disabled} confirmText="确认恢复此修订" onCancel={() => setConfirm(false)} onConfirm={() => { setConfirm(false); if (selected) void onRestore(selected.id); }} />
  </div>;
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
      toast.success("已恢复到工作稿，尚未发布");
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
        <RevisionHistory current={draftInputFrom(detail, timezone)} currentTaxonomy={detail.draft.taxonomy ?? []} currentSavedAt={detail.draft.savedAt} timezone={timezone} articleId={detail.id} version={detail.draft.version} disabled={restoring} dirty={false} onRestore={restore} />
      </div>}
    </PermissionGuard>
  );
}
