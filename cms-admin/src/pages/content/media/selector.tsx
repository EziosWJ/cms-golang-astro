import { useEffect, useState } from "react";
import { getMedia, getMediaPage, type MediaRecord } from "@/api/media";
import { PermissionGuard } from "@/components/auth/permission-guard";
import { MediaUpload } from "./upload";
import { MarkdownMediaImage } from "./preview";
import { Input } from "@/components/ui/input";
import { Pagination } from "@/components/common/pagination";
import { Button } from "@/components/ui/button";
import { Drawer, DrawerBody, DrawerContent, DrawerHeader, DrawerOverlay, DrawerTitle } from "@/components/ui/drawer";
import { getErrorMessage } from "@/lib/api-error";

type MediaSelectorProps = {
  onSelect?: (file: MediaRecord) => void;
  onCover?: (file: MediaRecord | null) => void;
  coverMediaId?: number | null;
  disabled?: boolean;
  onUploading?: (busy: boolean) => void;
  label?: string;
  coverOnly?: boolean;
};

export function MediaSelector({ onSelect, onCover, coverMediaId, disabled = false, onUploading, label = "封面", coverOnly = false }: MediaSelectorProps) {
  const [name, setName] = useState("");
  const [cover, setCover] = useState<MediaRecord | null>(null);
  const [records, setRecords] = useState<MediaRecord[]>([]);
  const [page, setPage] = useState(1);
  const [total, setTotal] = useState(0);
  const [refresh, setRefresh] = useState(0);
  const [error, setError] = useState("");
  const [libraryOpen, setLibraryOpen] = useState(false);

  useEffect(() => {
    let active = true;
    if (coverMediaId) void getMedia(coverMediaId).then((value) => { if (active) setCover(value); }).catch(() => undefined);
    else setCover(null);
    return () => { active = false; };
  }, [coverMediaId]);

  useEffect(() => {
    let active = true;
    getMediaPage(page, name).then((result) => {
      if (active) { setRecords(result.records); setTotal(result.total); setError(""); }
    }).catch((err: unknown) => { if (active) setError(getErrorMessage(err, "加载媒体失败")); });
    return () => { active = false; };
  }, [page, refresh, name]);

  function handleUploaded(file: MediaRecord) {
    setRefresh((value) => value + 1);
    if (coverOnly && file.image) {
      setCover(file);
      onCover?.(file);
      setLibraryOpen(false);
      return;
    }
    if (coverOnly) {
      setError("封面只能使用图片文件。");
      return;
    }
    onSelect?.(file);
  }

  function selectCover(file: MediaRecord) {
    setCover(file);
    onCover?.(file);
    setLibraryOpen(false);
  }

  const imageRecords = records.filter((file) => file.image);

  if (coverOnly) return <>
    <div className="media-cover-setting">
      <div className="media-cover-preview">{cover ? <MarkdownMediaImage src={cover.stablePath} alt={cover.originalName} className="h-full w-full object-cover" /> : <span>未设置{label}</span>}</div>
      <div className="media-cover-setting__body"><div><strong>{cover?.originalName || `${label}图片`}</strong><p>{cover ? (label === "头像" ? "用于网站作者区域展示。" : "读者将在文章卡片和详情页看到它。") : (label === "头像" ? "选择一张图片作为网站作者头像。" : "添加一张图片，让文章更容易被识别。")}</p></div><div className="media-cover-setting__actions"><Button size="sm" disabled={disabled} onClick={() => setLibraryOpen(true)}>{cover ? `更换${label}` : `选择${label}`}</Button>{cover && <Button size="sm" variant="ghost" disabled={disabled} onClick={() => { setCover(null); onCover?.(null); }}>移除</Button>}</div></div>
    </div>
    <Drawer open={libraryOpen} onOpenChange={setLibraryOpen} trapFocus restoreFocus lockScroll>
      <DrawerOverlay />
      <DrawerContent size="lg">
        <DrawerHeader><DrawerTitle>选择{label}图片</DrawerTitle><p className="text-body-secondary text-text-tertiary">从媒体库选择，或上传一张新的图片。</p></DrawerHeader>
        <DrawerBody className="media-library-drawer-body">
          <PermissionGuard permissionCode="content:media:edit"><MediaUpload onBusy={onUploading} disabled={disabled} onUploaded={handleUploaded} /></PermissionGuard>
          <Input aria-label="搜索媒体文件名" placeholder="搜索图片文件名" value={name} onChange={(event) => { setName(event.target.value); setPage(1); }} />
          {error && <p role="alert" className="text-error">{error}</p>}
          <div className="media-library-grid">{imageRecords.map((file) => <div key={file.id} className={`media-library-item ${coverMediaId === file.id ? "is-selected" : ""}`}><button type="button" className="media-library-item__main" disabled={disabled} onClick={() => selectCover(file)}><MarkdownMediaImage src={file.stablePath} alt={file.originalName} className="media-library-item__image" /><span className="media-library-item__name">{file.originalName}</span>{coverMediaId === file.id && <span className="media-library-item__status">当前{label}</span>}</button>{onSelect && <Button size="sm" variant="ghost" disabled={disabled} onClick={() => onSelect(file)}>插入正文</Button>}</div>)}{!imageRecords.length && <p className="taxonomy-empty">暂无匹配图片</p>}</div>
          <Pagination page={page} pageSize={20} total={total} onPageChange={setPage} disabled={disabled} />
        </DrawerBody>
      </DrawerContent>
    </Drawer>
  </>;

  return <div className="space-y-space-3">
    <PermissionGuard permissionCode="content:media:edit"><MediaUpload onBusy={onUploading} disabled={disabled} onUploaded={handleUploaded} /></PermissionGuard>
    <Input aria-label="搜索媒体文件名" placeholder="搜索文件名" value={name} onChange={(event) => { setName(event.target.value); setPage(1); }} />
    {error && <p role="alert" className="text-error">{error}</p>}
    {onCover && <p>当前{label}：{coverMediaId ? cover?.originalName || "加载中" : "无"} <Button size="sm" disabled={disabled || !coverMediaId} onClick={() => onCover(null)}>清空{label}</Button></p>}
    {cover && <MarkdownMediaImage src={cover.stablePath} alt={cover.originalName} className="h-24 w-24 object-contain" />}
    <div className="max-h-64 space-y-space-2 overflow-auto">{records.map((file) => <div key={file.id} className="flex flex-wrap items-center justify-between gap-space-2 border-b border-border py-space-2"><span className="flex min-w-0 items-center gap-space-2">{file.image && <MarkdownMediaImage src={file.stablePath} alt={file.originalName} className="h-12 w-12 object-contain" />}<span className="truncate">{file.originalName}</span></span><div className="flex gap-space-2">{onSelect && <Button size="sm" disabled={disabled} onClick={() => onSelect(file)}>插入正文</Button>}{onCover && file.image && <Button size="sm" disabled={disabled} onClick={() => onCover(file)}>设为{label}</Button>}</div></div>)}</div>
    <Pagination page={page} pageSize={20} total={total} onPageChange={setPage} disabled={disabled} />
  </div>;
}
