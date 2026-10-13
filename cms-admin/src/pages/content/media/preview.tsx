import { Eye } from "lucide-react";
import type { ComponentProps } from "react";
import { PhotoView } from "react-photo-view";
import { getFileViewUrl } from "@/api/file";
import { Button } from "@/components/ui/button";
import { useAuthenticatedFileUrl } from "@/hooks/use-authenticated-file-url";
import { downloadFile } from "@/api/file";
import { getErrorMessage } from "@/lib/api-error";
import { useState } from "react";
import type { MediaRecord } from "@/api/media";

function mediaID(path?: string) { const match = path?.match(/^\/media\/(?:images\/(\d+)\.(?:png|jpg|gif|webp)|attachments\/(\d+)\/download)$/); return match ? Number(match[1] || match[2]) : null; }

export function MarkdownMediaImage({ src, alt, ...props }: ComponentProps<"img">) {
  const source = typeof src === "string" && src.trim() ? src : null;
  const id = mediaID(source ?? undefined);
  const resource = useAuthenticatedFileUrl(id ? `/api/system/file/${id}/view` : source);
  if (!source) return null;
  if (resource.error) return <span role="alert">{resource.error}</span>;
  if (resource.loading || !resource.url) return <span>图片加载中…</span>;
  return <img {...props} src={resource.url} alt={alt ?? ""} />;
}

export function MediaPhoto({ file, large = false }: { file: MediaRecord; large?: boolean }) {
  const resource = useAuthenticatedFileUrl(getFileViewUrl(file.id));

  if (resource.error) return <span role="alert">{resource.error}</span>;
  if (resource.loading || !resource.url) {
    return <span className="text-sm text-text-tertiary">图片加载中…</span>;
  }

  return <img src={resource.url} alt={file.originalName} className={`${large ? "h-44 w-full" : "h-14 w-14"} object-contain`} />;
}

export function MediaPreviewButton({
  file,
  size = "sm",
}: {
  file: MediaRecord;
  size?: "sm" | "icon";
}) {
  const resource = useAuthenticatedFileUrl(getFileViewUrl(file.id));

  if (!file.image) return null;
  if (resource.error) return <span className="text-xs text-error" role="alert">预览失败</span>;
  if (resource.loading || !resource.url) {
    return <Button size={size} variant="ghost" disabled aria-label={`正在加载 ${file.originalName} 的预览`} title="预览加载中"><Eye className="h-4 w-4" aria-hidden />{size === "sm" && "预览"}</Button>;
  }

  return (
    <PhotoView src={resource.url}>
      <Button
        size={size}
        variant="ghost"
        aria-label={`预览 ${file.originalName}`}
        title="预览"
      >
        <Eye className="h-4 w-4" aria-hidden />
        {size === "sm" && "预览"}
      </Button>
    </PhotoView>
  );
}

export function MarkdownMediaLink({ href, children, ...props }: ComponentProps<"a">) {
  const id = mediaID(href); const [error, setError] = useState("");
  if (!id) return <a {...props} href={href}>{children}</a>;
  return <><button type="button" className="text-primary underline" onClick={() => { void downloadFile(id).then((blob) => { const link = document.createElement("a"); const objectURL = URL.createObjectURL(blob); link.href = objectURL; link.download = typeof children === "string" ? children : `attachment-${id}`; link.click(); URL.revokeObjectURL(objectURL); }).catch((err: unknown) => setError(getErrorMessage(err, "附件下载失败"))); }}>{children}</button>{error && <span role="alert">{error}</span>}</>;
}
