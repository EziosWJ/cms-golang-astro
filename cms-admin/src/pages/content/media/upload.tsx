import { File as FileIcon, Loader2, RotateCcw, UploadCloud, X } from "lucide-react";
import { useRef, useState, type DragEvent } from "react";
import { uploadMedia, getMedia, type MediaRecord } from "@/api/media";
import { Button } from "@/components/ui/button";
import { getErrorMessage } from "@/lib/api-error";

type Entry = {
  id: string;
  file: File;
  status: "waiting" | "uploading" | "succeeded" | "failed";
  error: string;
};

function formatFileSize(size: number) {
  if (size < 1024) return `${size} B`;
  if (size < 1024 * 1024) return `${(size / 1024).toFixed(1)} KB`;
  return `${(size / (1024 * 1024)).toFixed(1)} MB`;
}

export function MediaUpload({
  disabled = false,
  onUploaded,
  onBusy,
}: {
  disabled?: boolean;
  onBusy?: (busy: boolean) => void;
  onUploaded: (file: MediaRecord) => void;
}) {
  const inputRef = useRef<HTMLInputElement>(null);
  const pending = useRef(0);
  const [entries, setEntries] = useState<Entry[]>([]);
  const [dragging, setDragging] = useState(false);

  async function upload(entry: Entry) {
    pending.current += 1;
    onBusy?.(true);
    setEntries((items) =>
      items.map((item) =>
        item.id === entry.id
          ? { ...item, status: "uploading", error: "" }
          : item,
      ),
    );

    try {
      const file = await uploadMedia(entry.file, { businessModule: "attachment" });
      const media = await getMedia(file.id);
      setEntries((items) =>
        items.map((item) =>
          item.id === entry.id
            ? { ...item, status: "succeeded", error: "" }
            : item,
        ),
      );
      onUploaded(media);
    } catch (error) {
      setEntries((items) =>
        items.map((item) =>
          item.id === entry.id
            ? {
                ...item,
                status: "failed",
                error: getErrorMessage(error, "上传失败，请重试"),
              }
            : item,
        ),
      );
    } finally {
      pending.current -= 1;
      onBusy?.(pending.current > 0);
    }
  }

  function addFiles(files: File[]) {
    if (!files.length || disabled) return;
    const added = files.map(
      (file): Entry => ({
        id: crypto.randomUUID(),
        file,
        status: "waiting",
        error: "",
      }),
    );
    setEntries((old) => [...old, ...added]);
    added.forEach((entry) => void upload(entry));
  }

  function handleDrop(event: DragEvent<HTMLDivElement>) {
    event.preventDefault();
    setDragging(false);
    addFiles(Array.from(event.dataTransfer.files));
  }

  const finishedCount = entries.filter(
    (entry) => entry.status === "succeeded",
  ).length;

  return (
    <div className="space-y-space-4">
      <div
        className={`rounded-admin border border-dashed px-space-5 py-space-6 text-center transition-colors ${
          dragging
            ? "border-primary bg-primary/5"
            : "border-border bg-neutral-background"
        }`}
        onDragEnter={(event) => {
          event.preventDefault();
          setDragging(true);
        }}
        onDragOver={(event) => event.preventDefault()}
        onDragLeave={(event) => {
          if (event.currentTarget === event.target) setDragging(false);
        }}
        onDrop={handleDrop}
      >
        <UploadCloud className="mx-auto h-8 w-8 text-primary" aria-hidden />
        <p className="mt-space-2 font-medium text-text-primary">
          将文件拖到这里，或选择文件
        </p>
        <p className="mt-space-1 text-sm text-text-tertiary">
          支持图片和附件，可一次选择多个文件；格式和大小由服务器校验
        </p>
        <input
          ref={inputRef}
          className="sr-only"
          type="file"
          multiple
          disabled={disabled}
          onChange={(event) => {
            addFiles(Array.from(event.target.files ?? []));
            event.target.value = "";
          }}
          aria-label="选择要上传的文件"
        />
        <Button
          className="mt-space-4"
          variant="primary"
          disabled={disabled}
          onClick={() => inputRef.current?.click()}
        >
          选择文件
        </Button>
      </div>

      {entries.length > 0 && (
        <div className="rounded-admin border border-border bg-surface">
          <div className="flex flex-wrap items-center justify-between gap-space-2 border-b border-border px-space-4 py-space-3">
            <div>
              <h3 className="text-sm font-medium text-text-primary">上传记录</h3>
              <p className="mt-0.5 text-xs text-text-tertiary" aria-live="polite">
                {pending.current > 0 ? "正在上传，请保持页面打开" : `已完成 ${finishedCount} 个文件`}
              </p>
            </div>
            {finishedCount > 0 && (
              <Button
                size="sm"
                variant="ghost"
                onClick={() =>
                  setEntries((items) =>
                    items.filter((item) => item.status !== "succeeded"),
                  )
                }
              >
                清除已完成
              </Button>
            )}
          </div>
          <ul className="max-h-64 divide-y divide-border overflow-auto" aria-live="polite">
            {entries.map((entry) => (
              <li
                key={entry.id}
                className="flex flex-wrap items-center gap-space-3 px-space-4 py-space-3 text-sm"
              >
                <FileIcon className="h-4 w-4 shrink-0 text-text-tertiary" aria-hidden />
                <div className="min-w-0 flex-1">
                  <p className="truncate font-medium text-text-primary" title={entry.file.name}>
                    {entry.file.name}
                  </p>
                  <p className="text-xs text-text-tertiary">
                    {formatFileSize(entry.file.size)}
                  </p>
                </div>
                {entry.status === "waiting" && <span className="text-text-tertiary">等待上传</span>}
                {entry.status === "uploading" && (
                  <span className="inline-flex items-center gap-1 text-primary" role="status">
                    <Loader2 className="h-4 w-4 animate-spin" aria-hidden />
                    上传中
                  </span>
                )}
                {entry.status === "succeeded" && <span className="text-success">上传成功</span>}
                {entry.status === "failed" && (
                  <div className="flex flex-wrap items-center gap-space-2">
                    <span className="text-error" role="alert">{entry.error}</span>
                    <Button size="sm" variant="secondary" disabled={disabled} onClick={() => void upload(entry)}>
                      <RotateCcw className="h-3.5 w-3.5" aria-hidden />
                      重试
                    </Button>
                  </div>
                )}
                {entry.status !== "uploading" && entry.status !== "waiting" && (
                  <Button
                    size="icon"
                    variant="ghost"
                    className="h-7 w-7"
                    aria-label={`移除 ${entry.file.name}`}
                    onClick={() => setEntries((items) => items.filter((item) => item.id !== entry.id))}
                  >
                    <X className="h-4 w-4" aria-hidden />
                  </Button>
                )}
              </li>
            ))}
          </ul>
        </div>
      )}
    </div>
  );
}
