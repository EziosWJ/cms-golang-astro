import { useNavigate } from "react-router-dom";
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

const defaultFilters = { title: "", lifecycle: "active" };

export function ArticlesPage() {
  const navigate = useNavigate();
  const list = useListPage({
    fetch: getArticlePage,
    defaultFilters,
    toQuery: (filters, page, pageSize) => ({ ...filters, page, pageSize }),
  });
  const columns: DataTableColumn<ArticleListRecord>[] = [
    { title: "标题", key: "title", render: (_, record) => <span className="block max-w-sm truncate" title={record.title}>{record.title || "未命名文章"}</span> },
    { title: "Slug", dataIndex: "slug" },
{ title: "状态", key: "status", render: (_, record) => record.lifecycle === "archived" ? "已归档" : `${record.published ? "已上线" : "未上线"}${record.unpublishedChanges ? " · 有未发布修改" : ""}` },
    { title: "工作稿版本", dataIndex: "version", width: 120 },
    { title: "保存时间", key: "savedAt", render: (_, record) => new Date(record.savedAt).toLocaleString("zh-CN") },
    { title: "操作", key: "actions", width: 100, render: (_, record) => <Button variant="ghost" size="sm" onClick={() => navigate(`/content/articles/${record.id}`)}>编辑</Button> },
  ];
  return (
    <PermissionGuard permissionCode={ARTICLE_EDIT_PERMISSION} fallback={<EmptyState title="无访问权限" description="请联系管理员授予内容编辑权限。" />}>
      <PageHeader title="文章管理" description="管理文章工作稿，保存后可重新打开继续编辑。" actions={<Button variant="primary" onClick={() => navigate("/content/articles/new")}><Plus className="h-4 w-4" aria-hidden />新建文章</Button>} />
      <SearchFilterBar actions={<><Button variant="primary" onClick={list.submitFilters}>查询</Button><Button onClick={list.resetFilters}>重置</Button></>}>
        <Field label="生命周期" htmlFor="article-lifecycle"><select id="article-lifecycle" className="h-control-md rounded-control border border-border px-space-3" value={list.filters.lifecycle} onChange={(event) => list.setFilter("lifecycle", event.target.value)}><option value="active">编辑中</option><option value="archived">已归档</option></select></Field>
        <Field label="标题" htmlFor="article-search"><Input id="article-search" value={list.filters.title} onChange={(event) => list.setFilter("title", event.target.value)} onKeyDown={(event) => { if (event.key === "Enter") list.submitFilters(); }} /></Field>
      </SearchFilterBar>
      <DataTableCard toolbar={<TableToolbar title="文章列表" actions={<Button disabled={list.loading} onClick={list.reload}><RefreshCw className="h-4 w-4" aria-hidden />刷新</Button>} />} pagination={<Pagination page={list.page} pageSize={list.pageSize} total={list.total} disabled={list.loading} onPageChange={list.setPage} onPageSizeChange={list.setPageSize} />}>
        <DataTable columns={columns} dataSource={list.data} rowKey="id" loading={list.loading} error={list.error} empty={<EmptyState title="暂无文章" description="新建文章即可开始写作，不完整的内容也可以保存。" />} />
      </DataTableCard>
    </PermissionGuard>
  );
}
