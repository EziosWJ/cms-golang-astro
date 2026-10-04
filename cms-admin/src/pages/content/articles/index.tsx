import { Link, useLocation, useNavigate } from "react-router-dom";
import { Plus, RefreshCw } from "lucide-react";
import { getArticlePage } from "@/api/content";
import { PermissionGuard } from "@/components/auth/permission-guard";
import { PageHeader } from "@/components/common/page-header";
import { SearchFilterBar } from "@/components/common/search-filter-bar";
import { Field } from "@/components/common/field";
import { DataTableCard } from "@/components/common/data-table-card";
import { DataTable } from "@/components/common/data-table";
import { TableToolbar } from "@/components/common/table-toolbar";
import { Pagination } from "@/components/common/pagination";
import { EmptyState } from "@/components/common/empty-state";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { useListPage } from "@/hooks/use-list-page";
import { ARTICLE_EDIT_PERMISSION, type ArticleListRecord } from "@/types/content";
import type { DataTableColumn } from "@/types/table";

const defaultFilters = { title: "", lifecycle: "active", status: "" };

export function ArticlesPage() { const location=useLocation(); return <ArticleList key={location.search} />; }
function ArticleList() {
 const location=useLocation(); const params=new URLSearchParams(location.search);
 const applied={title:params.get("title") ?? "",lifecycle:params.get("lifecycle")==="archived" ? "archived" : "active",status:["draft","published","changed"].includes(params.get("status") ?? "") ? params.get("status")! : ""};
 const returnPath=location.pathname+location.search;
 const editorPath=(id:string | number) => `/content/articles/${id}?list=${encodeURIComponent(returnPath)}`;
  const navigate = useNavigate();
  const list = useListPage({
    fetch: getArticlePage,
    defaultFilters:applied,
 defaultPage: Math.max(1, Math.min(1000000,Number(params.get("page")) || 1)),
 defaultPageSize:[10,20,50,100].includes(Number(params.get("pageSize"))) ? Number(params.get("pageSize")) : 10,
    toQuery: (filters, page, pageSize) => ({ ...filters, page, pageSize }),
  });
  function query(filters:typeof defaultFilters,page=1,pageSize=list.pageSize) { const search=new URLSearchParams({...filters,page:String(page),pageSize:String(pageSize)}); navigate(`/content/articles?${search.toString()}`,{replace:true}); }
  const columns: DataTableColumn<ArticleListRecord>[] = [
    { title: "标题", key: "title", render: (_, record) => <Link className="block max-w-sm truncate text-primary" to={editorPath(record.id)}>{record.title || "未命名文章"}</Link> },
    { title: "分类", key: "taxonomy", render: (_, record) => record.taxonomy?.filter((term) => term.kind === "category").map((term) => term.name).join("、") || "—" },
{ title: "状态", key: "status", render: (_, record) => record.lifecycle === "archived" ? "已归档" : `${record.published ? "已上线" : "未上线"}${record.published && record.unpublishedChanges ? " · 有未发布修改" : ""}` },
    { title: "保存时间", key: "savedAt", render: (_, record) => new Date(record.savedAt).toLocaleString("zh-CN") },
    { title: "操作", key: "actions", width: 100, render: (_, record) => <Button variant="ghost" size="sm" onClick={() => navigate(editorPath(record.id))}>编辑</Button> },
  ];
  return (
    <PermissionGuard permissionCode={ARTICLE_EDIT_PERMISSION} fallback={<EmptyState title="无访问权限" description="请联系管理员授予内容编辑权限。" />}>
      <PageHeader title="文章管理" description="管理文章工作稿，保存后可重新打开继续编辑。" actions={<Button variant="primary" onClick={() => navigate(editorPath("new"))}><Plus className="h-4 w-4" aria-hidden />新建文章</Button>} />
      <SearchFilterBar actions={<><Button variant="primary" onClick={() => query(list.filters)}>查询</Button><Button onClick={() => query(defaultFilters)}>重置</Button></>}>
        <Field label="生命周期" htmlFor="article-lifecycle"><select id="article-lifecycle" className="h-control-md rounded-control border border-border px-space-3" value={list.filters.lifecycle} onChange={(event) => list.setFilter("lifecycle", event.target.value)}><option value="active">全部未归档</option><option value="archived">已归档</option></select></Field>
        <Field label="内容状态" htmlFor="article-status"><select id="article-status" className="h-control-md rounded-control border border-border px-space-3" value={list.filters.status} onChange={(event) => list.setFilter("status", event.target.value)}><option value="">全部</option><option value="draft">草稿</option><option value="published">已发布</option><option value="changed">有未发布修改</option></select></Field><Field label="标题" htmlFor="article-search"><Input id="article-search" value={list.filters.title} onChange={(event) => list.setFilter("title", event.target.value)} onKeyDown={(event) => { if (event.key === "Enter") query(list.filters); }} /></Field>
      </SearchFilterBar>
      <DataTableCard toolbar={<TableToolbar title="文章列表" actions={<Button disabled={list.loading} onClick={list.reload}><RefreshCw className="h-4 w-4" aria-hidden />刷新</Button>} />} pagination={<Pagination page={list.page} pageSize={list.pageSize} total={list.total} disabled={list.loading} onPageChange={(page) => query(applied,page)} onPageSizeChange={(size) => query(applied,1,size)} />}>
        <DataTable columns={columns} dataSource={list.data} rowKey="id" loading={list.loading} error={list.error} empty={<EmptyState title="暂无文章" description="新建文章即可开始写作，不完整的内容也可以保存。" />} />
      </DataTableCard>
    </PermissionGuard>
  );
}
