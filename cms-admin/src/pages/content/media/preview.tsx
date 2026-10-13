import type { ComponentProps } from "react";
import { useAuthenticatedFileUrl } from "@/hooks/use-authenticated-file-url";
import { downloadFile } from "@/api/file";
import { getErrorMessage } from "@/lib/api-error";
import { useState } from "react";
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
export function MarkdownMediaLink({ href, children, ...props }: ComponentProps<"a">) {
  const id = mediaID(href); const [error, setError] = useState("");
  if (!id) return <a {...props} href={href}>{children}</a>;
  return <><button type="button" className="text-primary underline" onClick={() => { void downloadFile(id).then((blob) => { const link = document.createElement("a"); const objectURL = URL.createObjectURL(blob); link.href = objectURL; link.download = typeof children === "string" ? children : `attachment-${id}`; link.click(); URL.revokeObjectURL(objectURL); }).catch((err: unknown) => setError(getErrorMessage(err, "附件下载失败"))); }}>{children}</button>{error && <span role="alert">{error}</span>}</>;
}
