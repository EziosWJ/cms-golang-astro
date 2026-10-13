import { Pencil, Plus, RefreshCw, Search, Trash2 } from "lucide-react";
import { useCallback, useEffect, useMemo, useState } from "react";
import { deleteTaxonomy, getTaxonomy, writeTaxonomy, type TaxonomyTerm } from "@/api/taxonomy";
import { PermissionGuard } from "@/components/auth/permission-guard";
import { ConfirmDialog } from "@/components/common/confirm-dialog";
import { DataTableCard } from "@/components/common/data-table-card";
import { DataTable } from "@/components/common/data-table";
import { EmptyState } from "@/components/common/empty-state";
import { Field } from "@/components/common/field";
import { FormDialog } from "@/components/common/form-dialog";
import { PageHeader } from "@/components/common/page-header";
import { SearchFilterBar } from "@/components/common/search-filter-bar";
import { StatusTag } from "@/components/common/status-tag";
import { TableActions } from "@/components/common/table-actions";
import { TableToolbar } from "@/components/common/table-toolbar";
import { toast } from "@/components/common/toast-store";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { getErrorMessage } from "@/lib/api-error";
import type { DataTableColumn } from "@/types/table";

type TaxonomyKind = "category" | "tag";
type RecordsByKind = Record<TaxonomyKind, TaxonomyTerm[]>;

const labels: Record<TaxonomyKind, string> = {
  category: "分类",
  tag: "标签",
};

function emptyRecords(): RecordsByKind {
  return { category: [], tag: [] };
}

export function TaxonomyPage() {
  const [kind, setKind] = useState<TaxonomyKind>("category");
  const [recordsByKind, setRecordsByKind] = useState<RecordsByKind>(emptyRecords);
  const [search, setSearch] = useState("");
  const [loading, setLoading] = useState(false);
  const [actionLoading, setActionLoading] = useState(false);
  const [error, setError] = useState("");
  const [name, setName] = useState("");
  const [url, setURL] = useState("");
  const [urlTouched, setURLTouched] = useState(false);
  const [formOpen, setFormOpen] = useState(false);
  const [editing, setEditing] = useState<TaxonomyTerm | null>(null);
  const [removing, setRemoving] = useState<TaxonomyTerm | null>(null);

  const label = labels[kind];
  const records = recordsByKind[kind];

  const reload = useCallback(async () => {
    setLoading(true);

    try {
      const [categories, tags] = await Promise.all([
        getTaxonomy("category"),
        getTaxonomy("tag"),
      ]);
      setRecordsByKind({ category: categories, tag: tags });
      setError("");
    } catch (loadError) {
      setError(getErrorMessage(loadError, "分类和标签加载失败"));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void reload();
  }, [reload]);

  const filteredRecords = useMemo(() => {
    const keyword = search.trim().toLocaleLowerCase();
    if (!keyword) return records;

    return records.filter(
      (record) =>
        record.name.toLocaleLowerCase().includes(keyword) ||
        record.url.toLocaleLowerCase().includes(keyword),
    );
  }, [records, search]);

  const resetForm = useCallback(() => {
    setFormOpen(false);
    setEditing(null);
    setName("");
    setURL("");
    setURLTouched(false);
  }, []);

  const openCreateForm = () => {
    setEditing(null);
    setName("");
    setURL("");
    setURLTouched(false);
    setFormOpen(true);
  };

  const openEditForm = (record: TaxonomyTerm) => {
    if (record.lockedAt) return;
    setEditing(record);
    setName(record.name);
    setURL(record.url);
    setURLTouched(true);
    setFormOpen(true);
  };

  const changeKind = (nextKind: TaxonomyKind) => {
    if (nextKind === kind) return;
    setKind(nextKind);
    setSearch("");
    resetForm();
    setError("");
  };

  const submit = async () => {
    const normalizedName = name.trim();
    const normalizedURL = url.trim();
    if (!normalizedName || !normalizedURL) return;

    const currentEditing = editing;
    setActionLoading(true);
    setError("");

    try {
      await writeTaxonomy(
        kind,
        {
          name: normalizedName,
          url: normalizedURL,
          expectedVersion: currentEditing?.version,
        },
        currentEditing?.id,
      );
      resetForm();
      toast.success(currentEditing ? `${label}已更新` : `${label}已创建`);
      await reload();
    } catch (submitError) {
      toast.error({
        title: currentEditing ? `${label}更新失败` : `${label}创建失败`,
        description: getErrorMessage(submitError, "请检查表单后重试"),
      });
    } finally {
      setActionLoading(false);
    }
  };

  const confirmRemove = async () => {
    if (!removing) return;

    const currentRemoving = removing;
    setActionLoading(true);

    try {
      await deleteTaxonomy(kind, currentRemoving.id);
      setRemoving(null);
      toast.success(`${label}已删除`);
      await reload();
    } catch (removeError) {
      toast.error({
        title: `${label}删除失败`,
        description: getErrorMessage(removeError, "请稍后重试"),
      });
    } finally {
      setActionLoading(false);
    }
  };

  const columns: DataTableColumn<TaxonomyTerm>[] = [
    { title: "名称", dataIndex: "name" },
    { title: "URL 片段", dataIndex: "url" },
    {
      title: "引用数",
      key: "referenceCount",
      align: "right",
      render: (_, row) => (
        <span className="tabular-nums">{row.referenceCount ?? 0}</span>
      ),
    },
    {
      title: "状态",
      key: "status",
      render: (_, row) =>
        row.lockedAt ? (
          <span title="首次发布后名称和 URL 保持稳定">
            <StatusTag tone="neutral">已锁定</StatusTag>
          </span>
        ) : (
          <StatusTag tone="success">可编辑</StatusTag>
        ),
    },
    {
      title: "操作",
      key: "actions",
      nowrap: true,
      align: "center",
      render: (_, row) => {
        const locked = Boolean(row.lockedAt);
        const hasReferences = (row.referenceCount ?? 0) > 0;
        const disabled = loading || actionLoading;

        return (
          <TableActions>
            <Button
              size="sm"
              variant="ghost"
              disabled={disabled || locked}
              title={locked ? "首次发布后名称和 URL 保持稳定" : undefined}
              onClick={() => openEditForm(row)}
            >
              <Pencil className="h-4 w-4" aria-hidden />
              编辑
            </Button>
            <Button
              size="sm"
              variant="ghost"
              disabled={disabled || locked || hasReferences}
              title={
                locked
                  ? "首次发布后名称和 URL 保持稳定"
                  : hasReferences
                    ? "已有引用，无法删除"
                    : undefined
              }
              onClick={() => setRemoving(row)}
            >
              <Trash2 className="h-4 w-4" aria-hidden />
              删除
            </Button>
          </TableActions>
        );
      },
    },
  ];

  const empty = search.trim() ? (
    <EmptyState
      title={`没有匹配的${label}`}
      description="请尝试搜索其他名称或 URL 片段。"
      actionText="清除搜索"
      onAction={() => setSearch("")}
    />
  ) : (
    <EmptyState
      title={`暂无${label}`}
      description={`创建${label}后，就可以在文章编辑器中选择。`}
      actionText={`新建${label}`}
      onAction={openCreateForm}
    />
  );

  return (
    <PermissionGuard
      permissionCode="content:taxonomy:edit"
      fallback={<EmptyState title="无访问权限" />}
    >
      <PageHeader
        title="分类与标签"
        description="分类和标签按平面管理；首次发布后名称和 URL 保持稳定。"
        actions={
          <Button
            variant="primary"
            disabled={loading || actionLoading}
            onClick={openCreateForm}
          >
            <Plus className="h-4 w-4" aria-hidden />
            新建{label}
          </Button>
        }
      />

      <div
        className="mb-space-4 flex flex-wrap gap-space-2"
        role="tablist"
        aria-label="分类与标签类型"
      >
        {(["category", "tag"] as const).map((value) => (
          <Button
            key={value}
            role="tab"
            aria-selected={kind === value}
            variant={kind === value ? "primary" : "secondary"}
            onClick={() => changeKind(value)}
          >
            {labels[value]}（{recordsByKind[value].length}）
          </Button>
        ))}
      </div>

      <SearchFilterBar
        actions={
          search && (
            <Button variant="ghost" onClick={() => setSearch("")}>
              清除搜索
            </Button>
          )
        }
      >
        <div className="relative">
          <Search
            className="pointer-events-none absolute left-space-3 top-1/2 h-4 w-4 -translate-y-1/2 text-text-tertiary"
            aria-hidden
          />
          <Input
            className="pl-9"
            aria-label={`搜索${label}`}
            placeholder={`搜索${label}名称或 URL 片段`}
            value={search}
            onChange={(event) => setSearch(event.target.value)}
          />
        </div>
      </SearchFilterBar>

      <DataTableCard
        toolbar={
          <TableToolbar
            title={`${label}列表`}
            description={`共 ${records.length} 项${search.trim() ? `，筛选出 ${filteredRecords.length} 项` : ""}。首次发布后将自动锁定。`}
            actions={
              <Button
                disabled={loading || actionLoading}
                onClick={() => void reload()}
              >
                <RefreshCw className="h-4 w-4" aria-hidden />
                刷新
              </Button>
            }
          />
        }
      >
        <DataTable
          columns={columns}
          dataSource={filteredRecords}
          rowKey="id"
          loading={loading}
          error={error}
          empty={empty}
        />
      </DataTableCard>

      <FormDialog
        open={formOpen}
        title={`${editing ? "编辑" : "新建"}${label}`}
        description={`${label}用于平面组织内容。首次发布后名称和 URL 将保持稳定。`}
        loading={actionLoading}
        submitDisabled={!name.trim() || !url.trim()}
        onCancel={resetForm}
        onSubmit={submit}
      >
        <div className="space-y-space-4">
          <Field
            label="名称"
            htmlFor="taxonomy-name"
            required
            help="用于后台和文章页面展示。"
          >
            <Input
              id="taxonomy-name"
              value={name}
              maxLength={200}
              autoFocus
              onChange={(event) => {
                const nextName = event.target.value;
                setName(nextName);
                if (!editing && !urlTouched) setURL(nextName);
              }}
            />
          </Field>
          <Field
            label="URL 片段"
            htmlFor="taxonomy-url"
            required
            help="只填写路径片段，不要包含斜杠；首次发布后不可修改。"
          >
            <Input
              id="taxonomy-url"
              value={url}
              maxLength={200}
              onChange={(event) => {
                setURLTouched(true);
                setURL(event.target.value);
              }}
            />
          </Field>
        </div>
      </FormDialog>

      <ConfirmDialog
        open={!!removing}
        title={`删除${label}？`}
        description={
          removing
            ? `确定删除“${removing.name}”吗？该项当前没有引用，删除后不可恢复。`
            : undefined
        }
        confirmText="删除"
        danger
        loading={actionLoading}
        onCancel={() => setRemoving(null)}
        onConfirm={() => void confirmRemove()}
      />
    </PermissionGuard>
  );
}
