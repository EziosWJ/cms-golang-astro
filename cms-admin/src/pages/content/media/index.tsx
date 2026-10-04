import { useEffect, useState } from "react";
import { deleteMedia, getMediaPage, uploadMediaBatch, type MediaRecord } from "@/api/media";
import { MediaSelector } from "./selector";
import { PermissionGuard } from "@/components/auth/permission-guard";
import { PageHeader } from "@/components/common/page-header";
import { ContentCard } from "@/components/common/content-card";
import { DataTable } from "@/components/common/data-table";
import { DataTableCard } from "@/components/common/data-table-card";
import { Pagination } from "@/components/common/pagination";
import { ConfirmDialog } from "@/components/common/confirm-dialog";
import { EmptyState } from "@/components/common/empty-state";
import { Button } from "@/components/ui/button";
import { getErrorMessage } from "@/lib/api-error";
import type { DataTableColumn } from "@/types/table";
export function MediaPage() {
  const [records, setRecords] = useState<MediaRecord[]>([]); const [total, setTotal] = useState(0); const [page, setPage] = useState(1); const [refresh, setRefresh] = useState(0); const [error, setError] = useState(""); const [removing, setRemoving] = useState<MediaRecord | null>(null); const [uploading, setUploading] = useState(false);
  useEffect(() => { let active = true; getMediaPage(page).then((result) => { if (active) { setRecords(result.records); setTotal(result.total); } }).catch((err: unknown) => { if (active) setError(getErrorMessage(err, "加载失败")); }); return () => { active = false; }; }, [page, refresh]);
  const columns: DataTableColumn<MediaRecord>[] = [{ title: "文件名", dataIndex: "originalName" }, { title: "类型", dataIndex: "mimeType" }, { title: "稳定路径", dataIndex: "stablePath" }, { title: "操作", key: "delete", render: (_, file) => <Button variant="ghost" onClick={() => setRemoving(file)}>删除</Button> }];
  return <PermissionGuard permissionCode="content:media:edit" fallback={<EmptyState title="无访问权限" />}><PageHeader title="媒体库" description="上传源文件保持私有。已有引用会保护资源，替换内容请上传新文件。" />
    <ContentCard title="上传"><MediaSelector /><label className="mt-space-4 block">批量上传<input type="file" multiple disabled={uploading} onChange={(event) => { const files = Array.from(event.target.files ?? []); if (!files.length) return; setUploading(true); void uploadMediaBatch(files).then((result) => { setError(result.failed.map((file) => `${file.fileName}: ${file.message}`).join("；")); setRefresh((value) => value + 1); }).catch((err: unknown) => setError(getErrorMessage(err, "批量上传失败"))).finally(() => setUploading(false)); event.target.value = ""; }} /></label></ContentCard>
    {error && <p role="alert" className="my-space-4 text-error">{error}</p>}<div className="mt-space-6"><DataTableCard pagination={<Pagination page={page} pageSize={20} total={total} onPageChange={setPage} />}><DataTable columns={columns} dataSource={records} rowKey="id" /></DataTableCard></div>
    <ConfirmDialog open={!!removing} title="删除媒体？" description="被工作稿、修订或产物引用时会拒绝删除。" danger onCancel={() => setRemoving(null)} onConfirm={() => { if (!removing) return; void deleteMedia(removing.id).then(() => setRefresh((value) => value + 1)).catch((err: unknown) => setError(getErrorMessage(err, "删除失败"))).finally(() => setRemoving(null)); }} />
  </PermissionGuard>;
}
