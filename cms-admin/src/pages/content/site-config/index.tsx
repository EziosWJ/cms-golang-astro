import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Link, useSearchParams } from "react-router-dom";
import { Plus } from "lucide-react";
import {
  LOCAL_SITE_KEY,
  getSites,
  getThemes,
  loadSiteWorking,
  saveSiteWorking,
  setSiteStatus,
  type SiteConfigData,
  type SiteSummary,
  type ThemeDefinition,
  type WorkingSiteConfig,
} from "@/api/site-config";
import { submitPublication } from "@/api/publishing";
import { PermissionGuard } from "@/components/auth/permission-guard";
import { PageHeader } from "@/components/common/page-header";
import { FormSection } from "@/components/common/form-section";
import { Field } from "@/components/common/field";
import { ContentCard } from "@/components/common/content-card";
import { EmptyState } from "@/components/common/empty-state";
import { StatusTag } from "@/components/common/status-tag";
import { Input } from "@/components/ui/input";
import { Select } from "@/components/ui/select";
import { Textarea } from "@/components/ui/textarea";
import { Button } from "@/components/ui/button";
import { ApiError, getErrorMessage } from "@/lib/api-error";
import { sitePublicUrl, publicUrlError } from "@/lib/site-url";
import { hasPermission } from "@/lib/permission";
import { PublicationTask } from "../publications/task";
import { ConfirmDialog } from "@/components/common/confirm-dialog";
import { MediaSelector } from "../media/selector";
import { AddSiteDialog } from "./add-site-dialog";
import { SiteSwitcher } from "./site-switcher";
import { languageOptions, optionsWithCurrent, timezoneOptions } from "./options";

type ConfigTab = "basic" | "author" | "theme";

const tabs: Array<{ key: ConfigTab; label: string }> = [
  { key: "basic", label: "基本信息" },
  { key: "author", label: "作者资料" },
  { key: "theme", label: "外观主题" },
];

const configFields: Array<{ key: keyof SiteConfigData; label: string }> = [
  { key: "siteName", label: "站点名称" },
  { key: "publicUrl", label: "公开地址" },
  { key: "language", label: "语言" },
  { key: "timezone", label: "时区" },
  { key: "description", label: "站点简介" },
  { key: "authorName", label: "作者名称" },
  { key: "authorBio", label: "作者简介" },
  { key: "avatarMediaId", label: "头像" },
  { key: "theme", label: "主题" },
];

function ThemePreview({ theme }: { theme: ThemeDefinition }) {
  const vaporwave = theme.id === "vaporwave";

  return (
    <div
      className="flex h-28 items-end rounded-md border border-border p-space-3"
      style={
        vaporwave
          ? {
              background:
                "linear-gradient(145deg,#160b2e,#301060 55%,#071827)",
              color: "#67e8f9",
            }
          : {
              background: "linear-gradient(145deg,#fff8d6,#ffd84d)",
              color: "#18181b",
            }
      }
    >
      <div>
        <div className="text-xs font-semibold opacity-70">
          {vaporwave ? "NEON / GRID" : "INK / PAPER"}
        </div>
        <div className="text-xl font-bold">{theme.name}</div>
      </div>
    </div>
  );
}

function displayConfigValue(
  key: keyof SiteConfigData,
  value: unknown,
  themes: ThemeDefinition[],
) {
  if (key === "avatarMediaId") return value ? "已设置" : "未设置";
  if (key === "theme") {
    return themes.find((theme) => theme.id === value)?.name || String(value || "未选择");
  }
  if (value === null || value === undefined || value === "") return "（空）";
  return String(value);
}

function ChangedFields({
  current,
  baseline,
  themes,
  currentLabel,
  baselineLabel,
}: {
  current: SiteConfigData;
  baseline: SiteConfigData;
  themes: ThemeDefinition[];
  currentLabel: string;
  baselineLabel: string;
}) {
  const changed = configFields.filter(
    ({ key }) => JSON.stringify(current[key]) !== JSON.stringify(baseline[key]),
  );

  if (!changed.length) {
    return <p className="text-sm text-text-tertiary">没有待发布的字段变化。</p>;
  }

  return (
    <div className="space-y-space-3">
      {changed.map(({ key, label }) => (
        <div key={key} className="grid gap-space-2 md:grid-cols-[140px_1fr_1fr]">
          <div className="text-sm font-medium text-text-primary">{label}</div>
          <div className="min-w-0 rounded-control bg-neutral-background px-space-3 py-space-2 text-sm text-text-primary">
            <div className="mb-space-1 text-xs text-text-tertiary">{currentLabel}</div>
            <div className="break-words">{displayConfigValue(key, current[key], themes)}</div>
          </div>
          <div className="min-w-0 rounded-control bg-neutral-background px-space-3 py-space-2 text-sm text-text-secondary">
            <div className="mb-space-1 text-xs text-text-tertiary">{baselineLabel}</div>
            <div className="break-words">{displayConfigValue(key, baseline[key], themes)}</div>
          </div>
        </div>
      ))}
    </div>
  );
}

/**
 * 「完成站点设置」清单。
 *
 * 按**站点**判断：它描述的是当前选中站点的配置是否填全，与「系统只有一份配置」
 * 无关，也与本地是否已发布无关（远端站点永不存在已发布配置）。填全即不再出现。
 */
function SetupChecklist({
  values,
  isLocal,
  onNavigate,
}: {
  values: SiteConfigData;
  isLocal: boolean;
  onNavigate: (tab: ConfigTab) => void;
}) {
  const items: Array<{ label: string; complete: boolean; tab: ConfigTab }> = [
    {
      label: "站点基本信息",
      complete: [values.siteName, values.publicUrl, values.language, values.timezone].every(Boolean),
      tab: "basic",
    },
    { label: "作者资料", complete: Boolean(values.authorName), tab: "author" },
    { label: "选择外观主题", complete: Boolean(values.theme), tab: "theme" },
  ];

  return (
    <ContentCard
      title="完成该站点的设置"
      description={
        isLocal
          ? "先完成这三项，保存后即可发布本地站点配置。"
          : "先完成这三项，保存后会在下一次向该站点的交付中生效。"
      }
      className="border-primary/30"
    >
      <div className="grid gap-space-3 md:grid-cols-3">
        {items.map((item) => (
          <button
            key={item.label}
            type="button"
            className="flex items-center justify-between gap-space-3 rounded-control border border-border px-space-3 py-space-3 text-left hover:border-primary/60 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary"
            onClick={() => onNavigate(item.tab)}
          >
            <span className="text-sm font-medium text-text-primary">{item.label}</span>
            <span className={item.complete ? "text-success" : "text-text-tertiary"}>
              {item.complete ? "已完成" : "去填写"}
            </span>
          </button>
        ))}
      </div>
    </ContentCard>
  );
}

export function SiteConfigPage() {
  const [params] = useSearchParams();
  const target = params.get("returnTo");
  const returnTo =
    target && /^\/content\/articles(?:\/(?:new|[1-9][0-9]*))?$/.test(target)
      ? target
      : null;

  const [sites, setSites] = useState<SiteSummary[]>([]);
  const [selectedKey, setSelectedKey] = useState<string>(LOCAL_SITE_KEY);
  const [themes, setThemes] = useState<ThemeDefinition[]>([]);
  const [working, setWorking] = useState<WorkingSiteConfig | null>(null);
  const [values, setValues] = useState<SiteConfigData | null>(null);
  const [loading, setLoading] = useState(true);
  const initialised = useRef(false);

  const [activeTab, setActiveTab] = useState<ConfigTab>("basic");
  const [error, setError] = useState("");
  const [fields, setFields] = useState<Record<string, string>>({});
  const [message, setMessage] = useState("");
  const [saving, setSaving] = useState(false);
  const [statusBusy, setStatusBusy] = useState(false);
  const [confirmStatus, setConfirmStatus] = useState<number | null>(null);
  const [addOpen, setAddOpen] = useState(false);
  const [conflict, setConflict] = useState(false);
  const [latest, setLatest] = useState<WorkingSiteConfig | null>(null);

  const [taskId, setTaskId] = useState<number | null>(null);
  const [confirmPublish, setConfirmPublish] = useState(false);
  const request = useRef<{ key: string; revision: number } | null>(null);

  // 首次进入：一次取回站点列表、主题目录，以及默认（local）站点的工作副本。
  useEffect(() => {
    let active = true;
    setLoading(true);
    Promise.all([getSites(), getThemes(), loadSiteWorking(LOCAL_SITE_KEY)])
      .then(([siteList, catalog, result]) => {
        if (!active) return;
        setSites(siteList);
        setThemes(catalog);
        setWorking(result);
        setValues(result.data);
        setSelectedKey(result.siteKey);
      })
      .catch((err: unknown) => {
        if (active) setError(getErrorMessage(err, "加载失败"));
      })
      .finally(() => {
        if (!active) return;
        initialised.current = true;
        setLoading(false);
      });
    return () => {
      active = false;
    };
  }, []);

  // 切换站点：重新拉取该站点的配置，并清掉上一个站点的临时状态。
  useEffect(() => {
    if (!initialised.current) return;
    let active = true;
    setLoading(true);
    setError("");
    setMessage("");
    setFields({});
    setConflict(false);
    setLatest(null);
    setTaskId(null);
    setWorking(null);
    setValues(null);
    request.current = null;
    loadSiteWorking(selectedKey)
      .then((result) => {
        if (!active) return;
        setWorking(result);
        setValues(result.data);
      })
      .catch((err: unknown) => {
        if (active) setError(getErrorMessage(err, "加载失败"));
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
    };
  }, [selectedKey]);

  const isLocal = selectedKey === LOCAL_SITE_KEY;

  // 「新增站点 / 停用配置」的闸门是 content:site:manage（后端在 POST /sites 与
  // POST /sites/:siteKey/status 上强校验，缺权限返回 403）。
  //
  // 权限码来自 GET /api/auth/menus：该接口只下发 visible=1 的菜单，所以
  // seed/00010_site_manage_permission.sql 把这条纯授权记录种成 visible=1、path 指向
  // 站点设置页，再由 cms-admin/src/config/navigation.ts 的 toNavItem 显式排除，
  // 因此它既能下发权限码，又不会在侧边栏多出条目。
  const canManageSites = hasPermission("content:site:manage");

  const reloadSites = useCallback(async () => {
    try {
      setSites(await getSites());
    } catch {
      // 列表刷新失败不影响当前正在编辑的配置。
    }
  }, []);

  function selectSite(siteKey: string) {
    if (siteKey === selectedKey) return;
    if (
      isDirty &&
      !window.confirm("当前站点有未保存的修改，切换站点会丢失这些修改。仍要切换吗？")
    ) {
      return;
    }
    setSelectedKey(siteKey);
  }

  function change<K extends keyof SiteConfigData>(key: K, value: SiteConfigData[K]) {
    setValues((current) => (current ? { ...current, [key]: value } : current));
    setMessage("");
  }

  function selectTheme(theme: ThemeDefinition) {
    setValues((current) =>
      current
        ? { ...current, theme: theme.id, themeVersion: theme.version }
        : current,
    );
    setMessage("");
    setFields((current) => ({ ...current, theme: "" }));
  }

  async function save() {
    if (!values || !working || saving || conflict) return;
    setSaving(true);
    setError("");
    setFields({});
    try {
      const result = await saveSiteWorking(working.siteKey, working.version, values);
      setWorking(result);
      setValues(result.data);
      setMessage(
        isLocal
          ? "已保存，尚未发布"
          : result.note || "已保存，下次交付生效，不产生发布任务",
      );
      await reloadSites();
    } catch (err) {
      if (err instanceof ApiError) {
        if (err.status === 409) setConflict(true);
        if (err.fieldErrors) setFields(err.fieldErrors);
      }
      setError(getErrorMessage(err, "保存失败，本地内容已保留"));
    } finally {
      setSaving(false);
    }
  }

  async function publish() {
    if (!isLocal || !working?.revisionId || saving) return;
    setSaving(true);
    setError("");
    request.current ??= {
      key: crypto.randomUUID(),
      revision: working.revisionId,
    };
    try {
      const task = await submitPublication(
        { kind: "config", configRevisionId: request.current.revision },
        request.current.key,
      );
      request.current = null;
      setTaskId(task.id);
      setConfirmPublish(false);
    } catch (err) {
      if (err instanceof ApiError && err.type !== "network") request.current = null;
      setError(getErrorMessage(err, "发布提交失败，可重试同一请求"));
    } finally {
      setSaving(false);
    }
  }

  async function changeStatus(next: number) {
    if (!working || isLocal || statusBusy) return;
    setStatusBusy(true);
    setError("");
    try {
      const result = await setSiteStatus(working.siteKey, next);
      setWorking(result);
      setMessage(
        next === 1
          ? "已启用该站点配置。"
          : "已停用该站点配置；远端站点的线上服务不受影响。",
      );
      await reloadSites();
    } catch (err) {
      setError(getErrorMessage(err, "状态更新失败"));
    } finally {
      setStatusBusy(false);
      setConfirmStatus(null);
    }
  }

  async function handleCreated(created: WorkingSiteConfig) {
    setAddOpen(false);
    await reloadSites();
    setSelectedKey(created.siteKey);
  }

  const selectedTheme = themes.find((theme) => theme.id === values?.theme);
  const isDirty = Boolean(
    values && working && JSON.stringify(values) !== JSON.stringify(working.data),
  );
  const hasSavedRevision = working?.revisionId !== null && working?.revisionId !== undefined;
  const hasUnpublishedChanges = Boolean(
    working &&
      hasSavedRevision &&
      (!working.published || JSON.stringify(working.data) !== JSON.stringify(working.published.data)),
  );
  const configComplete = Boolean(
    values &&
      values.siteName &&
      values.publicUrl &&
      values.language &&
      values.timezone &&
      values.authorName &&
      values.theme,
  );
  const statusText = isLocal
    ? isDirty
      ? "有未保存修改"
      : hasUnpublishedChanges
        ? "已保存，尚未发布"
        : working?.published
          ? "已与线上一致"
          : "尚未保存配置"
    : isDirty
      ? "有未保存修改"
      : hasSavedRevision
        ? "已保存，下次交付生效"
        : "尚未保存配置";
  const statusTone = isDirty || (isLocal && hasUnpublishedChanges) ? "text-warning" : "text-success";
  const statusHint = isDirty
    ? isLocal
      ? "保存后才会生成可发布的配置。"
      : "保存后写入 CMS，在下一次向该站点的交付中生效，不产生发布任务。"
    : isLocal
      ? hasUnpublishedChanges
        ? "线上网站仍保持原配置。"
        : "修改站点资料不会自动发布。"
      : "这里的修改不产生发布任务，也不会停止远端站点服务。";
  const canPublish = Boolean(
    isLocal &&
      working &&
      hasSavedRevision &&
      hasUnpublishedChanges &&
      !isDirty &&
      !saving &&
      !conflict &&
      !request.current,
  );
  const canSave = Boolean(
    values &&
      working &&
      !saving &&
      !conflict &&
      !request.current &&
      !publicUrlError(values.publicUrl) &&
      (isDirty || !hasSavedRevision),
  );
  const changedCount = useMemo(() => {
    if (!working?.published) return 0;
    return configFields.filter(
      ({ key }) =>
        JSON.stringify(working.data[key]) !==
        JSON.stringify(working.published?.data[key]),
    ).length;
  }, [working]);

  function renderActiveTab() {
    if (!values) return null;

    if (activeTab === "basic") {
      return (
        <FormSection
          title="基本信息"
          description="这些信息属于当前选中的站点，会出现在该站点网站的基础信息中。"
        >
          <Field label="站点名称" htmlFor="config-siteName" error={fields.siteName} required>
            <Input
              id="config-siteName"
              value={values.siteName}
              disabled={saving || !!request.current}
              onChange={(event) => change("siteName", event.target.value)}
            />
          </Field>
          <Field
            label="公开地址"
            htmlFor="config-publicUrl"
            error={fields.publicUrl || publicUrlError(values.publicUrl)}
            help="本站点的公开根地址：不带路径、查询参数或片段的 HTTP(S) 地址（每个站点一个域名，不支持子路径站点）。"
            required
          >
            <Input
              id="config-publicUrl"
              type="url"
              placeholder="https://example.com"
              value={values.publicUrl}
              disabled={saving || !!request.current}
              onChange={(event) => change("publicUrl", event.target.value)}
            />
          </Field>
          <Field label="语言" htmlFor="config-language" error={fields.language} required>
            <Select
              id="config-language"
              value={values.language}
              disabled={saving || !!request.current}
              onChange={(event) => change("language", event.target.value)}
            >
              {optionsWithCurrent(languageOptions, values.language).map((option) => (
                <option key={option.value} value={option.value}>{option.label}</option>
              ))}
            </Select>
          </Field>
          <Field
            label="时区"
            htmlFor="config-timezone"
            error={fields.timezone}
            required
          >
            <Select
              id="config-timezone"
              value={values.timezone}
              disabled={saving || !!request.current}
              onChange={(event) => change("timezone", event.target.value)}
            >
              {optionsWithCurrent(timezoneOptions, values.timezone).map((option) => (
                <option key={option.value} value={option.value}>{option.label}</option>
              ))}
            </Select>
          </Field>
          <div className="md:col-span-2">
            <Field
              label="站点简介"
              htmlFor="config-description"
              error={fields.description}
              help={`${values.description.length}/5000 字符`}
            >
              <Textarea
                id="config-description"
                rows={5}
                value={values.description}
                disabled={saving || !!request.current}
                onChange={(event) => change("description", event.target.value)}
              />
            </Field>
          </div>
        </FormSection>
      );
    }

    if (activeTab === "author") {
      return (
        <FormSection
          title="作者资料"
          description="当前站点的作者身份，与该站点网站展示一致，与登录后台的系统用户相互独立。"
          columns={1}
        >
          <Field label="作者名称" htmlFor="config-authorName" error={fields.authorName} required>
            <Input
              id="config-authorName"
              value={values.authorName}
              disabled={saving || !!request.current}
              onChange={(event) => change("authorName", event.target.value)}
            />
          </Field>
          <Field
            label="作者简介"
            htmlFor="config-authorBio"
            error={fields.authorBio}
            help={`${values.authorBio.length}/5000 字符`}
          >
            <Textarea
              id="config-authorBio"
              rows={5}
              value={values.authorBio}
              disabled={saving || !!request.current}
              onChange={(event) => change("authorBio", event.target.value)}
            />
          </Field>
          <MediaSelector
            label="头像"
            coverOnly
            disabled={saving || !!request.current}
            coverMediaId={values.avatarMediaId}
            onCover={(file) => change("avatarMediaId", file?.id ?? null)}
          />
        </FormSection>
      );
    }

    return (
      <FormSection
        title="外观主题"
        description={
          isLocal
            ? "主题决定本地站点网站的整体布局和文章视觉。每个站点独立选择；保存后，再从底部明确发布才会生效。"
            : "主题决定该站点网站的整体布局和文章视觉。每个站点独立选择；保存后会在下一次向该站点的交付中生效。"
        }
        columns={1}
      >
        <div className="grid gap-space-4 md:grid-cols-2">
          {themes.map((theme) => {
            const selected = values.theme === theme.id;
            const online = working?.published?.data.theme === theme.id;
            return (
              <button
                key={theme.id}
                type="button"
                disabled={saving || !!request.current}
                aria-pressed={selected}
                onClick={() => selectTheme(theme)}
                className={`rounded-lg border p-space-3 text-left transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary ${selected ? "border-primary ring-2 ring-primary/20" : "border-border hover:border-primary/60"}`}
              >
                <ThemePreview theme={theme} />
                <div className="mt-space-3 flex items-start justify-between gap-space-3">
                  <div>
                    <strong className="text-text-primary">{theme.name}</strong>
                    <p className="mt-space-1 text-sm text-text-tertiary">{theme.description}</p>
                  </div>
                  <span className="shrink-0 text-xs text-text-tertiary">v{theme.version}</span>
                </div>
                <div className="mt-space-3 flex flex-wrap gap-space-2 text-xs">
                  {selected && <span className="rounded-full bg-primary/10 px-space-2 py-1 text-primary">当前选择</span>}
                  {online && <span className="rounded-full bg-success/10 px-space-2 py-1 text-success">本地站点线上主题</span>}
                </div>
              </button>
            );
          })}
        </div>
        {fields.theme && <p role="alert" className="text-sm text-error">{fields.theme}</p>}
      </FormSection>
    );
  }

  return (
    <PermissionGuard
      permissionCode="content:config:edit"
      fallback={<EmptyState title="无访问权限" />}
    >
      <PageHeader
        title="站点设置"
        description="管理各站点的公开地址、作者资料与外观主题。本地站点保存后需发布才对外生效；其他站点保存即在下一次交付时生效，不产生发布任务。"
        actions={
          canManageSites ? (
            <Button variant="primary" onClick={() => setAddOpen(true)}>
              <Plus className="h-4 w-4" aria-hidden="true" />
              新增站点
            </Button>
          ) : null
        }
      />
      {error && <p role="alert" className="mb-space-4 text-error">{error}</p>}
      {returnTo && (
        <Link className="mb-space-4 inline-block text-primary" to={returnTo}>
          返回原文章，继续写作或确认发布
        </Link>
      )}
      {values && working ? (
        <form
          className="max-w-[1200px] space-y-space-6 pb-space-4"
          onSubmit={(event) => {
            event.preventDefault();
            void save();
          }}
        >
          <ContentCard
            title="站点"
            description="选择要配置的站点。文章内容始终属于本地站点；其他站点的配置会在下一次向该站点的交付中生效。"
          >
            <SiteSwitcher
              sites={sites}
              selectedKey={selectedKey}
              disabled={saving || statusBusy}
              onSelect={selectSite}
            />
            <div className="mt-space-4 rounded-control border border-border bg-neutral-background px-space-4 py-space-3">
              <div className="flex flex-wrap items-center justify-between gap-space-3">
                <div className="flex flex-wrap items-center gap-space-2">
                  <span className="text-sm font-medium text-text-primary">
                    {working.data.siteName?.trim() || working.siteKey}
                  </span>
                  {isLocal ? (
                    <StatusTag tone="info">本地站点</StatusTag>
                  ) : (
                    <StatusTag tone="neutral">远端站点</StatusTag>
                  )}
                  {working.status === 0 && <StatusTag tone="warning">已停用</StatusTag>}
                </div>
                {!isLocal && canManageSites && (
                  <Button
                    size="sm"
                    disabled={statusBusy || saving}
                    onClick={() => setConfirmStatus(working.status === 0 ? 1 : 0)}
                  >
                    {working.status === 0 ? "启用配置" : "停用配置"}
                  </Button>
                )}
              </div>
              <dl className="mt-space-2 grid gap-space-1 text-sm text-text-secondary md:grid-cols-2">
                <div>
                  站点标识：
                  <code className="rounded bg-surface px-space-1 text-text-primary">{working.siteKey}</code>
                  <span className="ml-space-1 text-text-tertiary">（创建后不可修改）</span>
                </div>
                <div>公开地址：{working.data.publicUrl || "（未设置）"}</div>
                {isLocal && working.published && (
                  <div className="md:col-span-2">
                    <a
                      className="text-primary"
                      href={sitePublicUrl(working)}
                      target="_blank"
                      rel="noreferrer"
                    >
                      查看线上网站（本地站点）
                    </a>
                  </div>
                )}
              </dl>
              {working.note && (
                <p
                  role="note"
                  className="mt-space-3 rounded-control border border-info-border bg-info-background px-space-3 py-space-2 text-sm text-info"
                >
                  {working.note}
                </p>
              )}
            </div>
          </ContentCard>

          <div className="flex flex-wrap items-center justify-between gap-space-3 rounded-control border border-border bg-surface px-card py-space-3">
            <div>
              <p className={`font-medium ${statusTone}`} role="status">{message || statusText}</p>
              <p className="mt-space-1 text-sm text-text-tertiary">{statusHint}</p>
            </div>
            {working.savedAt && (
              <span className="text-sm text-text-tertiary">
                最后保存：{new Date(working.savedAt).toLocaleString("zh-CN")}
              </span>
            )}
          </div>

          {!configComplete && (
            <SetupChecklist values={values} isLocal={isLocal} onNavigate={setActiveTab} />
          )}

          {working.published && hasUnpublishedChanges && (
            <details className="rounded-admin border border-border bg-surface shadow-admin">
              <summary className="cursor-pointer list-none px-card py-space-4 font-medium text-text-primary">
                待发布修改（{changedCount} 项）
                <span className="ml-space-2 text-sm font-normal text-text-tertiary">
                  展开查看
                </span>
              </summary>
              <div className="border-t border-border p-card">
                <ChangedFields
                  current={working.data}
                  baseline={working.published.data}
                  themes={themes}
                  currentLabel="已保存配置"
                  baselineLabel="线上版本"
                />
              </div>
            </details>
          )}

          <div role="tablist" aria-label="站点设置分类" className="flex overflow-x-auto border-b border-border">
            {tabs.map((tab) => (
              <button
                key={tab.key}
                type="button"
                role="tab"
                aria-selected={activeTab === tab.key}
                onClick={() => setActiveTab(tab.key)}
                className={`border-b-2 px-space-4 py-space-3 text-sm font-medium focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary focus-visible:ring-inset ${activeTab === tab.key ? "border-primary text-primary" : "border-transparent text-text-secondary hover:text-text-primary"}`}
              >
                {tab.label}
              </button>
            ))}
          </div>

          <div role="tabpanel" aria-label={tabs.find((tab) => tab.key === activeTab)?.label}>
            {renderActiveTab()}
          </div>

          {conflict && latest && (
            <ContentCard
              title="配置发生变化"
              description="其他页面已经保存了新版本。本地修改仍然保留，请选择继续使用哪一份配置。"
            >
              <ChangedFields
                current={values}
                baseline={latest.data}
                themes={themes}
                currentLabel="当前修改"
                baselineLabel="服务器最新版本"
              />
              <div className="mt-space-4 flex flex-wrap gap-space-2">
                <Button
                  onClick={() => {
                    setValues(latest.data);
                    setWorking(latest);
                    setLatest(null);
                    setConflict(false);
                    setError("");
                  }}
                >
                  采用服务器版本
                </Button>
                <Button
                  variant="primary"
                  onClick={() => {
                    setWorking(latest);
                    setLatest(null);
                    setConflict(false);
                    setError("");
                  }}
                >
                  保留当前修改，继续保存
                </Button>
              </div>
            </ContentCard>
          )}

          {conflict && !latest && (
            <ContentCard
              title="配置发生变化"
              description="本地修改已保留。读取服务器最新版本后，可以比较并选择继续方式。"
            >
              <Button
                onClick={() => {
                  void loadSiteWorking(selectedKey)
                    .then(setLatest)
                    .catch((err: unknown) => setError(getErrorMessage(err, "读取失败")));
                }}
              >
                读取最新版本
              </Button>
            </ContentCard>
          )}

          <div className="sticky bottom-0 z-10 -mx-space-4 flex flex-wrap items-center justify-between gap-space-3 border-t border-border bg-surface/95 px-space-4 py-space-3 shadow-admin backdrop-blur md:-mx-space-6 md:px-space-6">
            <div className="min-w-0 text-sm text-text-secondary">
              {message || statusText}
              {isLocal && hasUnpublishedChanges && !isDirty && <span className="ml-space-2 text-text-tertiary">发布后才会更新网站</span>}
              {!isLocal && <span className="ml-space-2 text-text-tertiary">不产生发布任务</span>}
            </div>
            <div className="flex flex-wrap gap-space-2">
              <Button
                variant="primary"
                type="submit"
                disabled={!canSave}
              >
                {saving ? "保存中…" : "保存修改"}
              </Button>
              {isLocal && (
                <PermissionGuard permissionCode="content:config:publish">
                  <Button
                    disabled={!canPublish}
                    onClick={() => setConfirmPublish(true)}
                  >
                    发布到网站
                  </Button>
                </PermissionGuard>
              )}
              {isLocal && request.current && !saving && (
                <Button onClick={() => void publish()}>重试本次发布</Button>
              )}
            </div>
          </div>

          {isLocal && taskId && (
            <PublicationTask
              key={taskId}
              id={taskId}
              publicUrl={sitePublicUrl(working)}
              onSuccess={() => {
                void loadSiteWorking(LOCAL_SITE_KEY)
                  .then((result) => {
                    setWorking(result);
                    setValues(result.data);
                    setMessage("配置已发布，可以查看线上网站。");
                    void reloadSites();
                  })
                  .catch((err: unknown) => setError(getErrorMessage(err, "刷新线上配置失败")));
              }}
            />
          )}
        </form>
      ) : (
        !error && <p role="status">{loading ? "加载中…" : "选择一个站点开始配置。"}</p>
      )}

      <AddSiteDialog
        open={addOpen}
        themes={themes}
        template={values}
        onCancel={() => setAddOpen(false)}
        onCreated={(created) => void handleCreated(created)}
      />

      <ConfirmDialog
        open={confirmPublish}
        title="发布站点配置？"
        description={`本次将发布本地站点的资料与 ${selectedTheme ? `${selectedTheme.name} v${selectedTheme.version}` : "当前主题"}。文章内容不会发生变化。其他站点的配置不参与发布。`}
        loading={saving}
        confirmText="确认发布"
        onConfirm={() => void publish()}
        onCancel={() => setConfirmPublish(false)}
      />

      <ConfirmDialog
        open={confirmStatus !== null}
        title={confirmStatus === 0 ? "停用该站点配置？" : "启用该站点配置？"}
        description={
          confirmStatus === 0
            ? "停用只改变这份配置在后台的状态，不会停止该远端站点的线上服务，也不会删除任何内容。它与「下线站点」不同：已上线的远端网站会继续照常访问。随时可以重新启用。"
            : "启用后该站点配置恢复正常状态，可在下一次交付中继续使用。这不会触发立即交付，也不会产生发布任务。"
        }
        confirmText={confirmStatus === 0 ? "停用配置" : "启用配置"}
        loading={statusBusy}
        onConfirm={() => {
          if (confirmStatus !== null) void changeStatus(confirmStatus);
        }}
        onCancel={() => setConfirmStatus(null)}
      />
    </PermissionGuard>
  );
}
