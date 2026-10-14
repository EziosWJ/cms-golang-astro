import { useEffect, useState } from "react";
import {
  SITE_KEY_ERROR,
  SITE_KEY_PATTERN,
  createSite,
  type SiteConfigData,
  type ThemeDefinition,
  type WorkingSiteConfig,
} from "@/api/site-config";
import { Field } from "@/components/common/field";
import { FormDialog } from "@/components/common/form-dialog";
import { Input } from "@/components/ui/input";
import { Select } from "@/components/ui/select";
import { Textarea } from "@/components/ui/textarea";
import { ApiError, getErrorMessage } from "@/lib/api-error";
import { publicUrlError } from "@/lib/site-url";
import { languageOptions, optionsWithCurrent, timezoneOptions } from "./options";

type AddSiteDialogProps = {
  open: boolean;
  themes: ThemeDefinition[];
  /** 当前站点的配置，用作语言/时区/主题的默认值。 */
  template: SiteConfigData | null;
  onCancel: () => void;
  onCreated: (site: WorkingSiteConfig) => void;
};

/**
 * 新增站点弹窗。
 *
 * 站点标识只在这里填写一次：创建之后它不再出现在任何可编辑表单里（编辑表单只改
 * `Data`，不含 siteKey）。校验规则与后端一致，输入过程中即时反馈，不必等提交。
 */
export function AddSiteDialog({ open, themes, template, onCancel, onCreated }: AddSiteDialogProps) {
  const [siteKey, setSiteKey] = useState("");
  const [siteName, setSiteName] = useState("");
  const [publicUrl, setPublicUrl] = useState("");
  const [authorName, setAuthorName] = useState("");
  const [description, setDescription] = useState("");
  const [language, setLanguage] = useState("zh-CN");
  const [timezone, setTimezone] = useState("Asia/Shanghai");
  const [themeId, setThemeId] = useState("");
  const [error, setError] = useState("");
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    if (!open) return;
    setSiteKey("");
    setSiteName("");
    setPublicUrl("");
    setAuthorName("");
    setDescription("");
    setLanguage(template?.language || "zh-CN");
    setTimezone(template?.timezone || "Asia/Shanghai");
    setThemeId(template?.theme || themes[0]?.id || "");
    setError("");
    setFieldErrors({});
    setSubmitting(false);
  }, [open, template, themes]);

  // 站点标识的即时校验：与后端 ^[a-z0-9][a-z0-9-]{0,62}$ 同规则。
  const keyError = siteKey && !SITE_KEY_PATTERN.test(siteKey) ? SITE_KEY_ERROR : "";
  // 公开地址只能是裸 origin：带路径/查询/片段的输入会被后端拒绝，这里提前挡住。
  const urlError = publicUrlError(publicUrl);
  const theme = themes.find((item) => item.id === themeId);
  const canSubmit = Boolean(
    siteKey && !keyError && siteName.trim() && publicUrl.trim() && !urlError && authorName.trim() && themeId,
  );

  async function submit() {
    if (!canSubmit || submitting) return;
    setSubmitting(true);
    setError("");
    setFieldErrors({});
    const data: SiteConfigData = {
      siteName: siteName.trim(),
      description,
      publicUrl: publicUrl.trim(),
      language,
      timezone,
      authorName: authorName.trim(),
      authorBio: "",
      avatarMediaId: null,
      theme: themeId,
      themeVersion: theme?.version ?? "",
    };
    try {
      onCreated(await createSite(siteKey, data));
    } catch (err) {
      if (err instanceof ApiError && err.fieldErrors) setFieldErrors(err.fieldErrors);
      setError(getErrorMessage(err, "新增站点失败"));
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <FormDialog
      open={open}
      title="新增站点"
      description="为另一个站点创建配置。保存只写入 CMS，不产生发布任务；这份配置会在下一次向该站点的交付中生效。"
      loading={submitting}
      submitDisabled={!canSubmit}
      submitText="创建站点"
      loadingText="创建中…"
      onCancel={onCancel}
      onSubmit={submit}
    >
      <div className="grid gap-space-4 md:grid-cols-2">
        <Field
          label="站点标识"
          htmlFor="new-site-key"
          required
          error={fieldErrors.siteKey || keyError}
          help="1–63 位小写字母、数字或短横线，且以字母或数字开头。创建后不可修改。"
        >
          <Input
            id="new-site-key"
            value={siteKey}
            autoComplete="off"
            spellCheck={false}
            placeholder="例如 blog-eu"
            onChange={(event) => setSiteKey(event.target.value.trim())}
          />
        </Field>
        <Field label="站点名称" htmlFor="new-site-name" required error={fieldErrors.siteName}>
          <Input id="new-site-name" value={siteName} onChange={(event) => setSiteName(event.target.value)} />
        </Field>
        <Field
          label="公开地址"
          htmlFor="new-site-url"
          required
          error={fieldErrors.publicUrl || urlError}
          help="HTTP(S) 根地址，不能带路径、查询参数或片段（每个站点一个域名，不支持子路径站点）。"
        >
          <Input
            id="new-site-url"
            type="url"
            placeholder="https://example.com"
            value={publicUrl}
            onChange={(event) => setPublicUrl(event.target.value)}
          />
        </Field>
        <Field label="作者名称" htmlFor="new-site-author" required error={fieldErrors.authorName}>
          <Input id="new-site-author" value={authorName} onChange={(event) => setAuthorName(event.target.value)} />
        </Field>
        <Field label="语言" htmlFor="new-site-language" error={fieldErrors.language}>
          <Select id="new-site-language" value={language} onChange={(event) => setLanguage(event.target.value)}>
            {optionsWithCurrent(languageOptions, language).map((option) => (
              <option key={option.value} value={option.value}>{option.label}</option>
            ))}
          </Select>
        </Field>
        <Field label="时区" htmlFor="new-site-timezone" error={fieldErrors.timezone}>
          <Select id="new-site-timezone" value={timezone} onChange={(event) => setTimezone(event.target.value)}>
            {optionsWithCurrent(timezoneOptions, timezone).map((option) => (
              <option key={option.value} value={option.value}>{option.label}</option>
            ))}
          </Select>
        </Field>
        <Field label="外观主题" htmlFor="new-site-theme" required error={fieldErrors.theme}>
          <Select id="new-site-theme" value={themeId} onChange={(event) => setThemeId(event.target.value)}>
            {themes.map((item) => (
              <option key={item.id} value={item.id}>{item.name}（v{item.version}）</option>
            ))}
          </Select>
        </Field>
        <div className="md:col-span-2">
          <Field label="站点简介" htmlFor="new-site-description" error={fieldErrors.description}>
            <Textarea
              id="new-site-description"
              rows={3}
              value={description}
              onChange={(event) => setDescription(event.target.value)}
            />
          </Field>
        </div>
      </div>
      {error && <p role="alert" className="mt-space-3 text-sm text-error">{error}</p>}
    </FormDialog>
  );
}
