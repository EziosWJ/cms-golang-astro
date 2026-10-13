import { useEffect, useMemo, useRef, useState } from "react";
import { Link, useSearchParams } from "react-router-dom";
import { ExternalLink } from "lucide-react";
import {
  getSiteConfig,
  getThemes,
  saveSiteConfig,
  type SiteConfigData,
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
import { Input } from "@/components/ui/input";
import { Select } from "@/components/ui/select";
import { Textarea } from "@/components/ui/textarea";
import { Button } from "@/components/ui/button";
import { ApiError, getErrorMessage } from "@/lib/api-error";
import { PublicationTask } from "../publications/task";
import { ConfirmDialog } from "@/components/common/confirm-dialog";
import { MediaSelector } from "../media/selector";

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

function SetupChecklist({
  values,
  onNavigate,
}: {
  values: SiteConfigData;
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
      title="完成站点设置"
      description="先完成这三项，保存后即可发布站点配置。"
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
  const [taskId, setTaskId] = useState<number | null>(null);
  const [confirmPublish, setConfirmPublish] = useState(false);
  const request = useRef<{ key: string; revision: number } | null>(null);
  const [activeTab, setActiveTab] = useState<ConfigTab>("basic");
  const [working, setWorking] = useState<WorkingSiteConfig | null>(null);
  const [values, setValues] = useState<SiteConfigData | null>(null);
  const [themes, setThemes] = useState<ThemeDefinition[]>([]);
  const [error, setError] = useState("");
  const [fields, setFields] = useState<Record<string, string>>({});
  const [saving, setSaving] = useState(false);
  const [conflict, setConflict] = useState(false);
  const [latest, setLatest] = useState<WorkingSiteConfig | null>(null);
  const [message, setMessage] = useState("");

  useEffect(() => {
    let active = true;
    Promise.all([getSiteConfig(), getThemes()])
      .then(([result, catalog]) => {
        if (active) {
          setWorking(result);
          setValues(result.data);
          setThemes(catalog);
        }
      })
      .catch((err: unknown) => {
        if (active) setError(getErrorMessage(err, "加载失败"));
      });
    return () => {
      active = false;
    };
  }, []);

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
      const result = await saveSiteConfig(working.version, values);
      setWorking(result);
      setValues(result.data);
      setMessage("已保存，尚未发布");
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
    if (!working?.revisionId || saving) return;
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
  const isFirstSetup = Boolean(working && !working.published);
  const statusText = isDirty
    ? "有未保存修改"
    : hasUnpublishedChanges
      ? "已保存，尚未发布"
      : working?.published
        ? "已与线上一致"
        : "尚未保存配置";
  const statusTone = isDirty || hasUnpublishedChanges ? "text-warning" : "text-success";
  const canPublish = Boolean(
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
          description="这些信息会出现在网站首页和页面基础信息中。"
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
            error={fields.publicUrl}
            help="请输入不带路径、查询参数或片段的 HTTP(S) 根地址。"
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
              <option value="zh-CN">简体中文（zh-CN）</option>
              <option value="en-US">English（en-US）</option>
              <option value="ja-JP">日本語（ja-JP）</option>
              {!['zh-CN', 'en-US', 'ja-JP'].includes(values.language) && (
                <option value={values.language}>{values.language}</option>
              )}
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
              <option value="Asia/Shanghai">中国标准时间（Asia/Shanghai）</option>
              <option value="UTC">协调世界时（UTC）</option>
              <option value="Asia/Tokyo">日本标准时间（Asia/Tokyo）</option>
              <option value="America/Los_Angeles">太平洋时间（America/Los_Angeles）</option>
              {!['Asia/Shanghai', 'UTC', 'Asia/Tokyo', 'America/Los_Angeles'].includes(values.timezone) && (
                <option value={values.timezone}>{values.timezone}</option>
              )}
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
          description="这里的作者身份用于网站展示，与登录后台的系统用户相互独立。"
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
        description="主题决定网站的整体布局和文章视觉。选择后保存，再从底部明确发布才会生效。"
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
                  {online && <span className="rounded-full bg-success/10 px-space-2 py-1 text-success">线上主题</span>}
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
        description="管理网站基本信息、作者身份和外观主题；保存后仍需明确发布才会影响线上网站。"
        actions={
          working?.published && (
            <a
              className="inline-flex items-center gap-space-2 text-sm text-primary"
              href={working.published.data.publicUrl}
              target="_blank"
              rel="noreferrer"
            >
              查看线上网站
              <ExternalLink className="h-4 w-4" aria-hidden="true" />
            </a>
          )
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
          <div className="flex flex-wrap items-center justify-between gap-space-3 rounded-control border border-border bg-surface px-card py-space-3">
            <div>
              <p className={`font-medium ${statusTone}`} role="status">{message || statusText}</p>
              <p className="mt-space-1 text-sm text-text-tertiary">
                {isDirty
                  ? "保存后才会生成可发布的配置。"
                  : hasUnpublishedChanges
                    ? "线上网站仍保持原配置。"
                    : "修改站点资料不会自动发布。"}
              </p>
            </div>
            {working.savedAt && (
              <span className="text-sm text-text-tertiary">
                最后保存：{new Date(working.savedAt).toLocaleString("zh-CN")}
              </span>
            )}
          </div>

          {isFirstSetup && <SetupChecklist values={values} onNavigate={setActiveTab} />}

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
                  void getSiteConfig()
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
              {hasUnpublishedChanges && !isDirty && <span className="ml-space-2 text-text-tertiary">发布后才会更新网站</span>}
            </div>
            <div className="flex flex-wrap gap-space-2">
              <Button
                variant="primary"
                type="submit"
                disabled={!canSave}
              >
                {saving ? "保存中…" : "保存修改"}
              </Button>
              <PermissionGuard permissionCode="content:config:publish">
                <Button
                  disabled={!canPublish}
                  onClick={() => setConfirmPublish(true)}
                >
                  发布到网站
                </Button>
              </PermissionGuard>
              {request.current && !saving && (
                <Button onClick={() => void publish()}>重试本次发布</Button>
              )}
            </div>
          </div>

          {taskId && (
            <PublicationTask
              key={taskId}
              id={taskId}
              publicUrl={working.data.publicUrl}
              onSuccess={() => {
                void getSiteConfig()
                  .then((result) => {
                    setWorking(result);
                    setValues(result.data);
                    setMessage("配置已发布，可以查看线上网站。");
                  })
                  .catch((err: unknown) => setError(getErrorMessage(err, "刷新线上配置失败")));
              }}
            />
          )}
        </form>
      ) : (
        !error && <p role="status">加载中…</p>
      )}
      <ConfirmDialog
        open={confirmPublish}
        title="发布站点配置？"
        description={`本次将发布站点资料与 ${selectedTheme ? `${selectedTheme.name} v${selectedTheme.version}` : "当前主题"}。文章内容不会发生变化。`}
        loading={saving}
        confirmText="确认发布"
        onConfirm={() => void publish()}
        onCancel={() => setConfirmPublish(false)}
      />
    </PermissionGuard>
  );
}
