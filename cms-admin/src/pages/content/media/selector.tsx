import { useEffect, useState } from "react";
import { getMedia, getMediaPage, type MediaRecord } from "@/api/media";
import { PermissionGuard } from "@/components/auth/permission-guard";
import { MediaUpload } from "./upload";
import { MarkdownMediaImage } from "./preview";
import { Input } from "@/components/ui/input";
import { Pagination } from "@/components/common/pagination";
import { Button } from "@/components/ui/button";
import { getErrorMessage } from "@/lib/api-error";
export function MediaSelector({ onSelect, onCover, coverMediaId, disabled = false, onUploading, label = "封面" }: { onSelect?: (file: MediaRecord) => void; onCover?: (file: MediaRecord | null) => void; coverMediaId?: number | null; disabled?: boolean; onUploading?:(busy:boolean) => void; label?: string }) {
  const [name,setName]=useState("");
 const [cover,setCover]=useState<MediaRecord | null>(null);
 useEffect(() => { let active=true; if(coverMediaId) void getMedia(coverMediaId).then((v) => { if(active)setCover(v); }).catch(() => undefined); else setCover(null); return () => { active=false; }; }, [coverMediaId]);
  const [records, setRecords] = useState<MediaRecord[]>([]); const [page, setPage] = useState(1); const [total, setTotal] = useState(0); const [refresh, setRefresh] = useState(0); const [error, setError] = useState("");
  useEffect(() => { let active = true; getMediaPage(page,name).then((result) => { if (active) { setRecords(result.records); setTotal(result.total); setError(""); } }).catch((err: unknown) => { if (active) setError(getErrorMessage(err, "加载媒体失败")); }); return () => { active = false; }; }, [page, refresh, name]);
  return <div className="space-y-space-3">
    <PermissionGuard permissionCode="content:media:edit"><MediaUpload onBusy={onUploading} disabled={disabled} onUploaded={(resource) => { setRefresh((value) => value+1); onSelect?.(resource); }} /></PermissionGuard>
    <Input aria-label="搜索媒体文件名" placeholder="搜索文件名" value={name} onChange={(event) => { setName(event.target.value); setPage(1); }} />
    {error && <p role="alert" className="text-error">{error}</p>}
    {onCover && <p>当前{label}：{coverMediaId ? cover?.originalName || "加载中" : "无"} <Button size="sm" disabled={disabled || !coverMediaId} onClick={() => onCover(null)}>清空{label}</Button></p>}
    {cover && <MarkdownMediaImage src={cover.stablePath} alt={cover.originalName} className="h-24 w-24 object-contain" />}
    <div className="max-h-64 space-y-space-2 overflow-auto">{records.map((file) => <div key={file.id} className="flex flex-wrap items-center justify-between gap-space-2 border-b border-border py-space-2"><span className="flex min-w-0 items-center gap-space-2">{file.image && <MarkdownMediaImage src={file.stablePath} alt={file.originalName} className="h-12 w-12 object-contain" />}<span className="truncate">{file.originalName}</span></span><div className="flex gap-space-2">{onSelect && <Button size="sm" disabled={disabled} onClick={() => onSelect(file)}>插入正文</Button>}{onCover && file.image && <Button size="sm" disabled={disabled} onClick={() => onCover(file)}>设为{label}</Button>}</div></div>)}</div>
    <Pagination page={page} pageSize={20} total={total} onPageChange={setPage} disabled={disabled} />
  </div>;
}
