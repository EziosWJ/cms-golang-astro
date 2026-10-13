import { FileImage, Link2 } from "lucide-react";
import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import type { MediaRecord, MediaReference } from "@/api/media";
import { DetailDialog } from "@/components/common/detail-dialog";
import { DetailItem } from "@/components/common/detail-item";
import { Button } from "@/components/ui/button";
import { hasPermission } from "@/lib/permission";
import { formatDateTime } from "@/lib/datetime";
import { MarkdownMediaImage } from "./preview";

const ownerLabels: Record<string, string> = {
  draft: "文章工作稿",
  revision: "历史版本",
  config: "工作配置",
  config_revision: "配置历史",
  task: "发布任务",
  preview: "私有预览",
  release: "保留发布产物",
  attempt: "构建尝试 / 预览产物",
};

type MediaDetailDialogProps = {
  open: boolean;
  record: MediaRecord | null;
  references: MediaReference[] | null;
  referenceError: string;
  onRetryReferences: () => void;
  onCopyPath: () => void;
  onCancel: () => void;
};

function formatFileSize(size: number) {
  if (size < 1024) return `${size} B`;
  if (size < 1024 * 1024) return `${(size / 1024).toFixed(1)} KB`;
  return `${(size / (1024 * 1024)).toFixed(1)} MB`;
}

function typeLabel(file: MediaRecord) {
  if (file.image) return "图片";
  const extension = file.mimeType.split("/")[1]?.toUpperCase();
  return extension || "附件";
}

export function MediaDetailDialog({
  open,
  record,
  references,
  referenceError,
  onRetryReferences,
  onCopyPath,
  onCancel,
}: MediaDetailDialogProps) {
  const [dimensions, setDimensions] = useState("");

  useEffect(() => {
    setDimensions("");
  }, [record?.id]);

  return (
    <DetailDialog
      open={open}
      title="媒体详情"
      description={record?.originalName}
      contentClassName="max-w-[860px]"
      onCancel={onCancel}
    >
      {record && (
        <div className="space-y-space-5">
          <div className="grid gap-space-5 lg:grid-cols-[180px_minmax(0,1fr)]">
            <div className="flex h-40 items-center justify-center rounded-admin border border-border bg-neutral-background p-space-3">
              {record.image ? (
                <MarkdownMediaImage
                  src={record.stablePath}
                  alt={record.originalName}
                  onLoad={(event) => setDimensions(`${event.currentTarget.naturalWidth} × ${event.currentTarget.naturalHeight} 像素`)}
                  className="max-h-full max-w-full object-contain"
                />
              ) : (
                <FileImage className="h-12 w-12 text-text-tertiary" aria-hidden />
              )}
            </div>
            <div className="grid min-w-0 gap-space-4 sm:grid-cols-2">
              <DetailItem label="类型" value={typeLabel(record)} />
              <DetailItem label="MIME 类型" value={record.mimeType} />
              <DetailItem label="文件大小" value={formatFileSize(record.fileSize)} />
              <DetailItem label="图片尺寸" value={dimensions} />
              <DetailItem label="上传时间" value={formatDateTime(record.createTime)} />
              <DetailItem
                label="媒体路径"
                className="sm:col-span-2"
                value={<code className="block break-all rounded-control bg-neutral-background px-space-3 py-space-2 text-xs text-text-secondary">{record.stablePath}</code>}
              />
            </div>
          </div>

          <div className="flex justify-end border-t border-border pt-space-4">
            <Button size="sm" variant="secondary" onClick={onCopyPath}>
              <Link2 className="h-4 w-4" aria-hidden />
              复制媒体路径
            </Button>
          </div>

          <section className="border-t border-border pt-space-4">
            <h3 className="font-medium text-text-primary">引用来源</h3>
            {referenceError ? (
              <div className="mt-space-2">
                <p className="text-sm text-error" role="alert">{referenceError}</p>
                <Button size="sm" variant="ghost" className="mt-space-2" onClick={onRetryReferences}>重试引用查询</Button>
              </div>
            ) : references === null ? (
              <p className="mt-space-2 text-sm text-text-tertiary">正在查询引用…</p>
            ) : references.length ? (
              <ul className="mt-space-2 space-y-space-1 text-sm text-text-secondary">
                {references.map((reference) => (
                  <li key={`${reference.ownerType}:${reference.ownerId}`}>
                    {ownerLabels[reference.ownerType] ?? reference.ownerType} · {reference.articleId && hasPermission("content:article:edit") ? (
                      <Link to={`/content/articles/${reference.articleId}`} className="text-primary hover:underline">
                        {reference.title || "未命名文章"}
                      </Link>
                    ) : `#${reference.ownerId}`}
                  </li>
                ))}
              </ul>
            ) : (
              <p className="mt-space-2 text-sm text-text-tertiary">当前未记录引用，删除时服务器仍会重新检查。</p>
            )}
          </section>
        </div>
      )}
    </DetailDialog>
  );
}
