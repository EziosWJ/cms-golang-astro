import {
  Download,
  File as FileIcon,
  FileText,
  Grid2X2,
  Link2,
  List,
  RefreshCw,
  Search,
  Trash2,
  UploadCloud,
} from "lucide-react";
import { useCallback, useEffect, useMemo, useState } from "react";
import {
  deleteMedia,
  getMediaPage,
  getMediaReferences,
  type MediaKind,
  type MediaRecord,
  type MediaReference,
  type MediaSort,
} from "@/api/media";
import { downloadFile } from "@/api/file";
import { getErrorMessage } from "@/lib/api-error";
import { formatDateTime } from "@/lib/datetime";
import { PermissionGuard } from "@/components/auth/permission-guard";
import { PageHeader } from "@/components/common/page-header";
import { ContentCard } from "@/components/common/content-card";
import { DataTable } from "@/components/common/data-table";
import { DataTableCard } from "@/components/common/data-table-card";
import { EmptyState } from "@/components/common/empty-state";
import { Pagination } from "@/components/common/pagination";
import { SearchFilterBar } from "@/components/common/search-filter-bar";
import { TableActions } from "@/components/common/table-actions";
import { ConfirmDialog } from "@/components/common/confirm-dialog";
import { toast } from "@/components/common/toast-store";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Select } from "@/components/ui/select";
import type { DataTableColumn } from "@/types/table";
import { MediaUpload } from "./upload";
import { MarkdownMediaImage, MediaPhoto, MediaPreviewButton } from "./preview";
import { MediaDetailDialog } from "./media-detail-dialog";
import { PhotoProvider } from "react-photo-view";

const PAGE_SIZE = 20;

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

function downloadBlob(blob: Blob, name: string) {
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = name;
  link.click();
  URL.revokeObjectURL(url);
}

function MediaThumb({ file, large = false }: { file: MediaRecord; large?: boolean }) {
  return file.image ? (
    <MarkdownMediaImage
      src={file.stablePath}
      alt={file.originalName}
      className={`${large ? "h-full w-full" : "h-14 w-14"} object-contain`}
    />
  ) : (
    <FileIcon
      className={large ? "h-12 w-12 text-text-tertiary" : "h-7 w-7 text-text-tertiary"}
      aria-hidden
    />
  );
}

export function MediaPage() {
  const [records, setRecords] = useState<MediaRecord[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [refresh, setRefresh] = useState(0);
  const [searchInput, setSearchInput] = useState("");
  const [name, setName] = useState("");
  const [kind, setKind] = useState<MediaKind>("all");
  const [sort, setSort] = useState<MediaSort>("newest");
  const [viewMode, setViewMode] = useState<"grid" | "list">("grid");
  const [uploadOpen, setUploadOpen] = useState(true);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);
  const [removing, setRemoving] = useState<MediaRecord | null>(null);
  const [removingLoading, setRemovingLoading] = useState(false);
  const [selected, setSelected] = useState<MediaRecord | null>(null);
  const [references, setReferences] = useState<MediaReference[] | null>(null);
  const [referenceError, setReferenceError] = useState("");

  useEffect(() => {
    let active = true;
    setLoading(true);
    getMediaPage(page, name, kind, sort)
      .then((result) => {
        if (!active) return;
        setRecords(result.records);
        setTotal(result.total);
        setError("");
      })
      .catch((err: unknown) => {
        if (active) setError(getErrorMessage(err, "媒体列表加载失败"));
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
    };
  }, [page, name, kind, sort, refresh]);

  const loadReferences = useCallback(async (fileId: number) => {
    setReferenceError("");
    try {
      setReferences(await getMediaReferences(fileId));
    } catch (err) {
      setReferenceError(getErrorMessage(err, "引用查询失败；不能据此判断资源没有引用"));
    }
  }, []);

  const openDetail = useCallback(async (file: MediaRecord) => {
    setSelected(file);
    setReferences(null);
    setReferenceError("");
    await loadReferences(file.id);
  }, [loadReferences]);

  async function handleDownload(file: MediaRecord) {
    try {
      downloadBlob(await downloadFile(file.id), file.originalName);
      toast.success("文件已开始下载");
    } catch (err) {
      toast.error({
        title: "下载失败",
        description: getErrorMessage(err, "文件下载失败，请重试"),
      });
    }
  }

  async function handleCopyPath(file: MediaRecord) {
    try {
      await navigator.clipboard.writeText(file.stablePath);
      toast.success("媒体路径已复制");
    } catch (err) {
      toast.error({
        title: "复制失败",
        description: getErrorMessage(err, "浏览器未授权访问剪贴板"),
      });
    }
  }

  function resetFilters() {
    setSearchInput("");
    setName("");
    setKind("all");
    setSort("newest");
    setPage(1);
  }

  const columns = useMemo<DataTableColumn<MediaRecord>[]>(
    () => [
      {
        title: "预览",
        key: "preview",
        width: 92,
        render: (_, file) => (
          <div className="flex h-14 w-14 items-center justify-center rounded-control border border-border bg-neutral-background">
            {file.image ? <MediaPhoto file={file} /> : <MediaThumb file={file} />}
          </div>
        ),
      },
      {
        title: "文件名",
        key: "name",
        render: (_, file) => (
          <button
            type="button"
            className="max-w-[320px] truncate text-left font-medium text-primary hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary"
            title={file.originalName}
            onClick={() => void openDetail(file)}
          >
            {file.originalName}
          </button>
        ),
      },
      { title: "类型", key: "type", render: (_, file) => typeLabel(file) },
      { title: "大小", key: "size", render: (_, file) => formatFileSize(file.fileSize) },
      {
        title: "上传时间",
        key: "createTime",
        nowrap: true,
        render: (_, file) => formatDateTime(file.createTime),
      },
      {
        title: "操作",
        key: "actions",
        nowrap: true,
        render: (_, file) => (
          <TableActions>
            <MediaPreviewButton file={file} size="icon" />
            <Button size="icon" variant="ghost" aria-label={`查看 ${file.originalName} 的详情`} title="详情" onClick={() => void openDetail(file)}>
              <FileText className="h-4 w-4" aria-hidden />
            </Button>
            <Button size="icon" variant="ghost" aria-label={`下载 ${file.originalName}`} title="下载" onClick={() => void handleDownload(file)}>
              <Download className="h-4 w-4" aria-hidden />
            </Button>
            <Button size="icon" variant="ghost" aria-label={`复制 ${file.originalName} 的路径`} title="复制路径" onClick={() => void handleCopyPath(file)}>
              <Link2 className="h-4 w-4" aria-hidden />
            </Button>
            <PermissionGuard permissionCode="content:media:edit">
              <Button size="icon" variant="ghost" className="text-error hover:text-error" aria-label={`删除 ${file.originalName}`} title="删除" onClick={() => setRemoving(file)}>
                <Trash2 className="h-4 w-4" aria-hidden />
              </Button>
            </PermissionGuard>
          </TableActions>
        ),
      },
    ],
    [openDetail],
  );

  function renderGrid() {
    if (loading) {
      return (
        <div className="grid gap-space-4 p-card sm:grid-cols-2 xl:grid-cols-4" aria-busy="true" aria-live="polite">
          {Array.from({ length: 8 }).map((_, index) => (
            <div key={index} className="h-52 animate-pulse rounded-admin bg-neutral-background" />
          ))}
        </div>
      );
    }
    if (error) {
      return <div className="p-card"><EmptyState title="加载失败" description={error} actionText="重试" onAction={() => setRefresh((value) => value + 1)} /></div>;
    }
    if (!records.length) {
      return <div className="p-card"><EmptyState title={name || kind !== "all" ? "没有找到匹配文件" : "还没有媒体文件"} description={name || kind !== "all" ? "尝试清除搜索或筛选条件。" : "上传第一个文件后，它会出现在这里。"} actionText={name || kind !== "all" ? "清除筛选" : "上传媒体"} onAction={() => { if (name || kind !== "all") resetFilters(); else setUploadOpen(true); }} /></div>;
    }
    return (
      <div className="grid gap-space-4 p-card sm:grid-cols-2 xl:grid-cols-4">
        {records.map((file) => (
          <article key={file.id} className="group min-w-0 overflow-hidden rounded-admin border border-border bg-surface transition-colors hover:border-primary">
            <div
              className="flex h-44 w-full items-center justify-center bg-neutral-background p-space-4 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-primary"
            >
              {file.image ? <MediaPhoto file={file} large /> : <MediaThumb file={file} large />}
            </div>
            <div className="space-y-space-2 p-space-3">
              <button type="button" className="block w-full truncate text-left text-sm font-medium text-text-primary hover:text-primary hover:underline" title={file.originalName} onClick={() => void openDetail(file)}>
                {file.originalName}
              </button>
              <p className="text-xs text-text-tertiary">
                {typeLabel(file)} · {formatFileSize(file.fileSize)} · {formatDateTime(file.createTime)}
              </p>
              <div className="flex items-center gap-1 border-t border-border pt-space-2">
                <MediaPreviewButton file={file} />
                <Button size="sm" variant="ghost" onClick={() => void openDetail(file)}><FileText className="h-4 w-4" aria-hidden />详情</Button>
                <Button size="sm" variant="ghost" onClick={() => void handleDownload(file)}><Download className="h-3.5 w-3.5" aria-hidden />下载</Button>
                <Button size="icon" variant="ghost" className="ml-auto h-7 w-7" aria-label="复制路径" title="复制路径" onClick={() => void handleCopyPath(file)}><Link2 className="h-4 w-4" aria-hidden /></Button>
                <PermissionGuard permissionCode="content:media:edit">
                  <Button size="icon" variant="ghost" className="h-7 w-7 text-error hover:text-error" aria-label={`删除 ${file.originalName}`} title="删除" onClick={() => setRemoving(file)}><Trash2 className="h-4 w-4" aria-hidden /></Button>
                </PermissionGuard>
              </div>
            </div>
          </article>
        ))}
      </div>
    );
  }

  return (
    <PermissionGuard permissionCode="content:media:edit" fallback={<EmptyState title="无访问权限" />}>
      <PhotoProvider loop={false} maskOpacity={0.92} photoClosable={false}>
      <PageHeader
        title="媒体库"
        description="管理图片和附件，上传后可在文章、页面和配置中重复使用。"
        actions={
          <Button variant="primary" onClick={() => setUploadOpen((open) => !open)}>
            <UploadCloud className="h-4 w-4" aria-hidden />
            {uploadOpen ? "收起上传" : "上传媒体"}
          </Button>
        }
      />

      {uploadOpen && (
        <ContentCard title="上传媒体" description="拖拽文件到上传区域，选择后会立即开始上传。">
          <MediaUpload onUploaded={() => setRefresh((value) => value + 1)} />
        </ContentCard>
      )}

      <SearchFilterBar className={uploadOpen ? "mt-space-4" : undefined} actions={
        <>
          <Button variant="secondary" onClick={resetFilters}>重置</Button>
          <Button variant="primary" type="submit" form="media-search-form"><Search className="h-4 w-4" aria-hidden />搜索</Button>
          <Button size="icon" variant="secondary" aria-label="刷新媒体列表" title="刷新列表" onClick={() => setRefresh((value) => value + 1)}><RefreshCw className="h-4 w-4" aria-hidden /></Button>
        </>
      }>
        <form id="media-search-form" className="contents" onSubmit={(event) => { event.preventDefault(); setName(searchInput.trim()); setPage(1); }}>
          <Input aria-label="搜索文件名" placeholder="搜索文件名" value={searchInput} onChange={(event) => setSearchInput(event.target.value)} />
        </form>
        <Select aria-label="媒体类型" value={kind} onChange={(event) => { setKind(event.target.value as MediaKind); setPage(1); }}>
          <option value="all">所有类型</option>
          <option value="image">图片</option>
          <option value="attachment">附件</option>
        </Select>
        <Select aria-label="排序方式" value={sort} onChange={(event) => { setSort(event.target.value as MediaSort); setPage(1); }}>
          <option value="newest">最新上传</option>
          <option value="name">文件名</option>
          <option value="size">文件大小</option>
        </Select>
        <div className="flex items-center gap-1" role="group" aria-label="视图模式">
          <Button size="icon" variant={viewMode === "grid" ? "primary" : "secondary"} aria-label="网格视图" aria-pressed={viewMode === "grid"} title="网格视图" onClick={() => setViewMode("grid")}><Grid2X2 className="h-4 w-4" aria-hidden /></Button>
          <Button size="icon" variant={viewMode === "list" ? "primary" : "secondary"} aria-label="列表视图" aria-pressed={viewMode === "list"} title="列表视图" onClick={() => setViewMode("list")}><List className="h-4 w-4" aria-hidden /></Button>
        </div>
      </SearchFilterBar>

      <DataTableCard pagination={total > PAGE_SIZE ? <Pagination page={page} pageSize={PAGE_SIZE} total={total} onPageChange={setPage} /> : null}>
        {viewMode === "grid" ? renderGrid() : (
          <DataTable
            columns={columns}
            dataSource={records}
            rowKey="id"
            loading={loading}
            error={error ? <EmptyState title="加载失败" description={error} actionText="重试" onAction={() => setRefresh((value) => value + 1)} /> : undefined}
            empty={<EmptyState title={name || kind !== "all" ? "没有找到匹配文件" : "还没有媒体文件"} description={name || kind !== "all" ? "尝试清除搜索或筛选条件。" : "上传图片或附件后即可在文章、页面和配置中重复使用。"} actionText={name || kind !== "all" ? "清除筛选" : "上传媒体"} onAction={() => { if (name || kind !== "all") resetFilters(); else setUploadOpen(true); }} />}
          />
        )}
      </DataTableCard>

      <MediaDetailDialog
        open={selected !== null}
        record={selected}
        references={references}
        referenceError={referenceError}
        onRetryReferences={() => {
          if (selected) void loadReferences(selected.id);
        }}
        onCopyPath={() => {
          if (selected) void handleCopyPath(selected);
        }}
        onCancel={() => setSelected(null)}
      />

      <ConfirmDialog
        open={removing !== null}
        title="删除媒体？"
        description={`删除“${removing?.originalName ?? ""}”的源文件。正在被工作稿、历史版本、配置、发布任务、预览或保留产物引用的文件无法删除。`}
        confirmText="删除"
        danger
        loading={removingLoading}
        onCancel={() => setRemoving(null)}
        onConfirm={() => {
          if (!removing) return;
          setRemovingLoading(true);
          void deleteMedia(removing.id)
            .then(() => {
              toast.success("媒体已删除");
              setRefresh((value) => value + 1);
              if (selected?.id === removing.id) setSelected(null);
              setRemoving(null);
            })
            .catch((err: unknown) => toast.error({ title: "删除失败", description: getErrorMessage(err, "删除失败，请重试") }))
            .finally(() => setRemovingLoading(false));
        }}
      />
      </PhotoProvider>
    </PermissionGuard>
  );
}
