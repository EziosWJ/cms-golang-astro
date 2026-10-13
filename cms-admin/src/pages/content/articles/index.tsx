import { Link, useLocation, useNavigate } from "react-router-dom";
import { useState } from "react";
import { History, Pencil, Plus, RefreshCw, RotateCcw, Search, Send } from "lucide-react";
import { getArticle, getArticlePage } from "@/api/content";
import { submitPublication, type PublishInput } from "@/api/publishing";
import { PermissionGuard } from "@/components/auth/permission-guard";
import { ConfirmDialog } from "@/components/common/confirm-dialog";
import { PageHeader } from "@/components/common/page-header";
import { SearchFilterBar } from "@/components/common/search-filter-bar";
import { Field } from "@/components/common/field";
import { DataTableCard } from "@/components/common/data-table-card";
import { DataTable } from "@/components/common/data-table";
import { TableActions } from "@/components/common/table-actions";
import { TableToolbar } from "@/components/common/table-toolbar";
import { Pagination } from "@/components/common/pagination";
import { EmptyState } from "@/components/common/empty-state";
import { StatusTag } from "@/components/common/status-tag";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Select } from "@/components/ui/select";
import { toast } from "@/components/common/toast-store";
import { getErrorMessage } from "@/lib/api-error";
import { useListPage } from "@/hooks/use-list-page";
import { formatDateOnly, formatDateTime } from "@/lib/datetime";
import { ARTICLE_EDIT_PERMISSION, type ArticleDetail, type ArticleListRecord } from "@/types/content";
import type { DataTableColumn } from "@/types/table";

const defaultFilters = { title: "", lifecycle: "active", status: "" };

function ArticleStatus({ record }: { record: ArticleListRecord }) {
  if (record.lifecycle === "archived") {
    return <StatusTag>已归档</StatusTag>;
  }

  if (!record.published) {
    return <StatusTag tone="warning">草稿 · 未上线</StatusTag>;
  }

  return (
    <div className="flex flex-wrap items-center gap-space-1">
      <StatusTag tone="success">已上线</StatusTag>
      {record.unpublishedChanges && <StatusTag tone="info">有未发布修改</StatusTag>}
    </div>
  );
}

export function ArticlesPage() { const location=useLocation(); return <ArticleList key={location.search} />; }
function ArticleList() {
 const location=useLocation(); const params=new URLSearchParams(location.search);
 const applied={title:params.get("title") ?? "",lifecycle:params.get("lifecycle")==="archived" ? "archived" : "active",status:["draft","published","changed"].includes(params.get("status") ?? "") ? params.get("status")! : ""};
 const returnPath=location.pathname+location.search;
 const editorPath=(id:string | number) => `/content/articles/${id}?list=${encodeURIComponent(returnPath)}`;
  const revisionsPath=(id:string | number) => `/content/articles/${id}/revisions?list=${encodeURIComponent(returnPath)}`;
  const navigate = useNavigate();
  const [publishingArticle, setPublishingArticle] = useState<ArticleListRecord | null>(null);
  const [publishingDetail, setPublishingDetail] = useState<ArticleDetail | null>(null);
  const [publishingLoading, setPublishingLoading] = useState(false);
  const [publishingSubmitting, setPublishingSubmitting] = useState(false);
  const list = useListPage({
    fetch: getArticlePage,
    defaultFilters:applied,
 defaultPage: Math.max(1, Math.min(1000000,Number(params.get("page")) || 1)),
 defaultPageSize:[10,20,50,100].includes(Number(params.get("pageSize"))) ? Number(params.get("pageSize")) : 10,
    toQuery: (filters, page, pageSize) => ({ ...filters, page, pageSize }),
  });
  function query(filters:typeof defaultFilters,page=1,pageSize=list.pageSize) { const search=new URLSearchParams({...filters,page:String(page),pageSize:String(pageSize)}); navigate(`/content/articles?${search.toString()}`,{replace:true}); }
  async function openPublish(record: ArticleListRecord) {
    if (publishingLoading || publishingSubmitting) return;
    setPublishingLoading(true);
    try {
      const detail = await getArticle(record.id);
      setPublishingArticle(record);
      setPublishingDetail(detail);
    } catch (error) {
      toast.error({ title: "文章加载失败", description: getErrorMessage(error, "无法读取文章最新内容，请稍后重试") });
    } finally {
      setPublishingLoading(false);
    }
  }
  async function confirmPublish() {
    if (!publishingArticle || !publishingDetail || publishingSubmitting) return;
    const detail = publishingDetail;
    setPublishingSubmitting(true);
    try {
      const taxonomy = detail.draft.taxonomy ?? [];
      const input: PublishInput = {
        kind: "article",
        articleId: detail.id,
        save: {
          coverMediaId: detail.draft.coverMediaId,
          categoryIds: taxonomy.filter((term) => term.kind === "category").map((term) => term.id),
          tagIds: taxonomy.filter((term) => term.kind === "tag").map((term) => term.id),
          title: detail.draft.title,
          slug: detail.slug,
          markdown: detail.draft.markdown,
          summary: detail.draft.summary,
          displayDate: detail.draft.displayDate,
          expectedVersion: detail.draft.version,
          mode: "manual",
        },
      };
      await submitPublication(input, crypto.randomUUID());
      setPublishingArticle(null);
      setPublishingDetail(null);
      toast.success("已提交发布任务");
      list.reload();
    } catch (error) {
      toast.error({ title: "发布提交失败", description: getErrorMessage(error, "请稍后重试") });
    } finally {
      setPublishingSubmitting(false);
    }
  }
  function closePublish() {
    if (publishingSubmitting) return;
    setPublishingArticle(null);
    setPublishingDetail(null);
  }
  const columns: DataTableColumn<ArticleListRecord>[] = [
    { title: "标题", key: "title", render: (_, record) => <Link className="block max-w-sm truncate font-medium text-primary" to={editorPath(record.id)}>{record.title || "未命名文章"}</Link> },
    { title: "分类", key: "taxonomy", render: (_, record) => record.taxonomy?.filter((term) => term.kind === "category").map((term) => term.name).join("、") || "未分类" },
    { title: "状态", key: "status", render: (_, record) => <ArticleStatus record={record} /> },
    { title: "展示日期", key: "displayDate", nowrap: true, render: (_, record) => formatDateOnly(record.displayDate) },
    { title: "最近保存", key: "savedAt", nowrap: true, render: (_, record) => formatDateTime(record.savedAt) },
    { title: "操作", key: "actions", nowrap: true, align: "center", width: 280, render: (_, record) => <TableActions><Button variant="ghost" size="sm" onClick={() => navigate(editorPath(record.id))}><Pencil className="h-4 w-4" aria-hidden />编辑</Button><Button variant="ghost" size="sm" onClick={() => navigate(revisionsPath(record.id))}><History className="h-4 w-4" aria-hidden />历史版本</Button>{record.lifecycle === "active" && <PermissionGuard permissionCode="content:publish"><Button variant="ghost" size="sm" disabled={publishingLoading || publishingSubmitting} onClick={() => void openPublish(record)}><Send className="h-4 w-4" aria-hidden />发布</Button></PermissionGuard>}</TableActions> },
  ];
  const hasFilters = applied.title !== "" || applied.lifecycle !== defaultFilters.lifecycle || applied.status !== "";
  return (
    <PermissionGuard permissionCode={ARTICLE_EDIT_PERMISSION} fallback={<EmptyState title="无访问权限" description="请联系管理员授予内容编辑权限。" />}>
      <PageHeader title="文章管理" description="管理文章工作稿、上线状态和归档内容。" actions={<Button variant="primary" onClick={() => navigate(editorPath("new"))}><Plus className="h-4 w-4" aria-hidden />新建文章</Button>} />
      <SearchFilterBar actions={<><Button variant="primary" onClick={() => query(list.filters)}><Search className="h-4 w-4" aria-hidden />查询</Button><Button onClick={() => query(defaultFilters)}><RotateCcw className="h-4 w-4" aria-hidden />清除筛选</Button></>}>
        <Field label="文章范围" htmlFor="article-lifecycle"><Select id="article-lifecycle" value={list.filters.lifecycle} onChange={(event) => list.setFilter("lifecycle", event.target.value)}><option value="active">当前文章（不含归档）</option><option value="archived">已归档文章</option></Select></Field>
        <Field label="上线状态" htmlFor="article-status"><Select id="article-status" value={list.filters.status} onChange={(event) => list.setFilter("status", event.target.value)}><option value="">全部状态</option><option value="draft">草稿（未上线）</option><option value="published">已上线</option><option value="changed">有未发布修改</option></Select></Field><Field label="标题关键词" htmlFor="article-search"><Input id="article-search" type="search" placeholder="输入标题搜索" value={list.filters.title} onChange={(event) => list.setFilter("title", event.target.value)} onKeyDown={(event) => { if (event.key === "Enter") query(list.filters); }} /></Field>
      </SearchFilterBar>
      <DataTableCard toolbar={<TableToolbar title="文章列表" description={list.loading ? "正在加载…" : `当前筛选 ${list.total} 篇文章`} actions={<Button disabled={list.loading} onClick={list.reload}><RefreshCw className="h-4 w-4" aria-hidden />刷新</Button>} />} pagination={<Pagination page={list.page} pageSize={list.pageSize} total={list.total} disabled={list.loading} onPageChange={(page) => query(applied,page)} onPageSizeChange={(size) => query(applied,1,size)} />}>
        <DataTable columns={columns} dataSource={list.data} rowKey="id" loading={list.loading} error={list.error} empty={<EmptyState title={hasFilters ? "没有找到匹配的文章" : "还没有文章"} description={hasFilters ? "试试更换关键词或放宽筛选条件。" : "新建一篇文章即可开始写作，不完整的内容也可以先保存。"} actionText={hasFilters ? "清除筛选" : undefined} onAction={hasFilters ? () => query(defaultFilters) : undefined} />} />
      </DataTableCard>
      <ConfirmDialog
        open={publishingArticle !== null}
        title={`${publishingArticle?.published ? "发布更新" : "发布文章"}？`}
        description={publishingArticle ? `将“${publishingArticle.title || "未命名文章"}”当前工作稿提交到发布队列。确认后会固定当前版本，发布完成前线上内容保持不变。` : undefined}
        confirmText="确认发布"
        loading={publishingSubmitting}
        onCancel={closePublish}
        onConfirm={() => void confirmPublish()}
      />
    </PermissionGuard>
  );
}
