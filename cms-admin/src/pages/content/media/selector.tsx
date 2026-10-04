import { useEffect, useState } from "react";
import { getMedia, getMediaPage, uploadMedia, type MediaRecord } from "@/api/media";
import { PermissionGuard } from "@/components/auth/permission-guard";
import { FileUpload } from "@/components/common/file-upload";
import { Pagination } from "@/components/common/pagination";
import { Button } from "@/components/ui/button";
import { getErrorMessage } from "@/lib/api-error";
export function MediaSelector({ onSelect, onCover, coverMediaId, disabled = false, label = "封面" }: { onSelect?: (file: MediaRecord) => void; onCover?: (file: MediaRecord | null) => void; coverMediaId?: number | null; disabled?: boolean; label?: string }) {
  const [records, setRecords] = useState<MediaRecord[]>([]); const [page, setPage] = useState(1); const [total, setTotal] = useState(0); const [refresh, setRefresh] = useState(0); const [error, setError] = useState("");
  useEffect(() => { let active = true; getMediaPage(page).then((result) => { if (active) { setRecords(result.records); setTotal(result.total); setError(""); } }).catch((err: unknown) => { if (active) setError(getErrorMessage(err, "加载媒体失败")); }); return () => { active = false; }; }, [page, refresh]);
  return <div className="space-y-space-3">
    <PermissionGuard permissionCode="content:media:edit"><FileUpload uploader={uploadMedia} businessModule="attachment" disabled={disabled} buttonText="上传新媒体" onUploaded={(file) => { setRefresh((value) => value + 1); void getMedia(file.id).then((resource) => onSelect?.(resource)).catch((err: unknown) => setError(getErrorMessage(err, "读取上传资源失败"))); }} /></PermissionGuard>
    {error && <p role="alert" className="text-error">{error}</p>}
    {onCover && <p>当前{label}：{coverMediaId ? `媒体 #${coverMediaId}` : "无"} <Button size="sm" disabled={disabled || !coverMediaId} onClick={() => onCover(null)}>清空{label}</Button></p>}
    <div className="max-h-64 space-y-space-2 overflow-auto">{records.map((file) => <div key={file.id} className="flex flex-wrap items-center justify-between gap-space-2 border-b border-border py-space-2"><span className="min-w-0 truncate">#{file.id} · {file.originalName}</span><div className="flex gap-space-2">{onSelect && <Button size="sm" disabled={disabled} onClick={() => onSelect(file)}>插入正文</Button>}{onCover && file.image && <Button size="sm" disabled={disabled} onClick={() => onCover(file)}>设为{label}</Button>}</div></div>)}</div>
    <Pagination page={page} pageSize={20} total={total} onPageChange={setPage} disabled={disabled} />
  </div>;
}
