import { useCallback, useEffect, useState } from "react";
import { deleteTaxonomy, getTaxonomy, writeTaxonomy, type TaxonomyTerm } from "@/api/taxonomy";
import { PermissionGuard } from "@/components/auth/permission-guard";
import { PageHeader } from "@/components/common/page-header";
import { FormSection } from "@/components/common/form-section";
import { Field } from "@/components/common/field";
import { DataTableCard } from "@/components/common/data-table-card";
import { DataTable } from "@/components/common/data-table";
import { TableToolbar } from "@/components/common/table-toolbar";
import { ConfirmDialog } from "@/components/common/confirm-dialog";
import { EmptyState } from "@/components/common/empty-state";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { getErrorMessage } from "@/lib/api-error";
import type { DataTableColumn } from "@/types/table";

export function TaxonomyPage() {
  const [kind, setKind] = useState<"category" | "tag">("category");
  const [records, setRecords] = useState<TaxonomyTerm[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [name, setName] = useState(""); const [url, setURL] = useState("");
  const [editing, setEditing] = useState<TaxonomyTerm | null>(null);
  const [removing, setRemoving] = useState<TaxonomyTerm | null>(null);
  const reload = useCallback(async () => { setLoading(true); try { setRecords(await getTaxonomy(kind)); setError(""); } catch (err) { setError(getErrorMessage(err, "加载失败")); } finally { setLoading(false); } }, [kind]);
  useEffect(() => { void reload(); }, [reload]);
  async function submit() { setLoading(true); try { await writeTaxonomy(kind, { name, url, expectedVersion: editing?.version }, editing?.id); setEditing(null); setName(""); setURL(""); await reload(); } catch (err) { setError(getErrorMessage(err, "保存失败")); } finally { setLoading(false); } }
  const columns: DataTableColumn<TaxonomyTerm>[] = [{ title: "名称", dataIndex: "name" }, { title: "URL", dataIndex: "url" }, { title: "身份", key: "locked", render: (_, row) => row.lockedAt ? "已发布，已锁定" : "尚未发布" }, { title: "操作", key: "actions", render: (_, row) => <><Button variant="ghost" disabled={!!row.lockedAt || loading} onClick={() => { setEditing(row); setName(row.name); setURL(row.url); }}>编辑</Button><Button variant="ghost" disabled={!!row.lockedAt || loading} onClick={() => setRemoving(row)}>删除</Button></> }];
  return <PermissionGuard permissionCode="content:taxonomy:edit" fallback={<EmptyState title="无访问权限" />}>
    <PageHeader title="分类与标签" description="平面组织内容，首次发布后名称与 URL 保持稳定。" />
    <div className="mb-space-4 flex gap-space-2">{(["category", "tag"] as const).map((value) => <Button key={value} variant={kind === value ? "primary" : "secondary"} onClick={() => { setKind(value); setEditing(null); setName(""); setURL(""); }}>{value === "category" ? "分类" : "标签"}</Button>)}</div>
    <div className="mb-space-6"><FormSection title={editing ? "编辑" : "新建"}><Field label="名称" htmlFor="term-name"><Input id="term-name" value={name} maxLength={200} onChange={(event) => { setName(event.target.value); if (!editing) setURL(event.target.value); }} /></Field><Field label="URL 片段" htmlFor="term-url"><Input id="term-url" value={url} maxLength={200} onChange={(event) => setURL(event.target.value)} /></Field><div><Button variant="primary" disabled={loading || !name || !url} onClick={() => void submit()}>保存</Button>{editing && <Button onClick={() => { setEditing(null); setName(""); setURL(""); }}>取消</Button>}</div></FormSection></div>
    {error && <p className="mb-space-4 text-error" role="alert">{error}</p>}
    <DataTableCard toolbar={<TableToolbar title={kind === "category" ? "分类列表" : "标签列表"} actions={<Button disabled={loading} onClick={() => void reload()}>刷新</Button>} />}><DataTable columns={columns} dataSource={records} rowKey="id" loading={loading} /></DataTableCard>
    <ConfirmDialog open={!!removing} title="删除所选项？" description="被工作稿、修订或发布产物引用的项无法删除。" danger onCancel={() => setRemoving(null)} onConfirm={() => { if (!removing) return; void deleteTaxonomy(kind, removing.id).then(reload).catch((err: unknown) => setError(getErrorMessage(err, "删除失败"))).finally(() => setRemoving(null)); }} />
  </PermissionGuard>;
}
