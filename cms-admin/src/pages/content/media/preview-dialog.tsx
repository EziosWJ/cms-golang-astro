import { Download, File as FileIcon, Loader2, X } from "lucide-react";
import { getFileViewUrl } from "@/api/file";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogHeader,
  DialogOverlay,
  DialogTitle,
} from "@/components/ui/dialog";
import { useAuthenticatedFileUrl } from "@/hooks/use-authenticated-file-url";
import type { MediaRecord } from "@/api/media";

export function MediaPreviewDialog({
  record,
  onClose,
  onDownload,
}: {
  record: MediaRecord | null;
  onClose: () => void;
  onDownload: (record: MediaRecord) => void;
}) {
  const isImage = Boolean(record?.image);
  const previewPath = record && isImage ? getFileViewUrl(record.id) : null;
  const { url, loading, error } = useAuthenticatedFileUrl(previewPath);

  return (
    <Dialog
      open={record !== null}
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
      trapFocus
      restoreFocus
      lockScroll
    >
      <DialogOverlay className="bg-slate-950/50" />
      <DialogContent className="flex max-h-[calc(100vh-48px)] max-w-[960px] flex-col">
        <DialogHeader className="items-center px-5 py-3">
          <DialogTitle className="min-w-0 flex-1 truncate">
            {record?.originalName ?? "媒体预览"}
          </DialogTitle>
          <div className="flex items-center gap-1">
            {record && (
              <Button size="sm" variant="secondary" onClick={() => onDownload(record)}>
                <Download className="h-4 w-4" aria-hidden />
                下载
              </Button>
            )}
            <DialogClose aria-label="关闭预览">
              <X className="h-4 w-4" aria-hidden />
            </DialogClose>
          </div>
        </DialogHeader>
        <div className="flex min-h-[280px] flex-1 items-center justify-center overflow-auto bg-neutral-background p-space-5">
          {!record && null}
          {record && isImage && loading && (
            <div className="flex items-center gap-space-2 text-sm text-text-tertiary" role="status">
              <Loader2 className="h-4 w-4 animate-spin" aria-hidden />
              正在加载预览…
            </div>
          )}
          {record && isImage && error && <p className="text-sm text-error" role="alert">{error}</p>}
          {record && isImage && !loading && !error && url && (
            <img src={url} alt={record.originalName} className="max-h-[65vh] max-w-full object-contain" />
          )}
          {record && !isImage && (
            <div className="py-12 text-center text-sm text-text-tertiary">
              <FileIcon className="mx-auto h-10 w-10" aria-hidden />
              <p className="mt-space-3">此文件类型不支持在线预览</p>
              <p className="mt-space-1">点击右上角“下载”后查看文件内容。</p>
            </div>
          )}
        </div>
      </DialogContent>
    </Dialog>
  );
}
