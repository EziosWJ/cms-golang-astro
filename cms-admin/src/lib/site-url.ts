import type { WorkingSiteConfig } from "@/api/site-config";

/**
 * 去掉地址末尾的斜杠。
 *
 * 站点配置的 `publicUrl` 按后端校验是裸 origin（`siteconfig.Validate` 拒绝任何路径），
 * 但作者可能手写末尾斜杠，拼接前统一收敛，避免出现 `https://a.com//archives/`。
 */
export function trimTrailingSlash(url: string): string {
  return url.replace(/\/+$/, "");
}

/**
 * 站点上的公开文章**绝对**地址：`{origin}/archives/{slug}/`。
 *
 * 这是全站唯一的拼接规则，与 canonical 语义一致：文章编辑器的「公开地址」
 * 与发布结果卡片都调用它，不再各自复制一份 `replace(/\/$/,"") + "/archives/" + slug`。
 */
export function publicArticleUrl(baseUrl: string, slug: string): string {
  return `${trimTrailingSlash(baseUrl)}/archives/${encodeURIComponent(slug)}/`;
}

/**
 * 一个站点当前对外可见的公开地址——「公开地址」在界面上的唯一口径。
 *
 * 本地站点：以「已上线配置」为准（那才是此刻线上站点所在地址），尚未发布过时退回
 * 已保存配置；远端站点：没有线上配置概念（后端 `published` 恒为 null），取其保存的
 * 公开地址。
 *
 * `data` 可选是为了兼容编辑器上下文（`GET /site-config/editor-context` 只返回
 * `published`，因为编辑器只面向本地站点）。除此之外的调用方都直接传工作副本。
 *
 * 取数时不要再写 `published.data.publicUrl` 与 `data.publicUrl` 两套写法。
 */
export function sitePublicUrl(
  working: Pick<WorkingSiteConfig, "published"> & { data?: Pick<WorkingSiteConfig["data"], "publicUrl"> } | null,
): string {
  if (!working) return "";
  return working.published?.data.publicUrl || working.data?.publicUrl || "";
}

/** 公开地址不合规时的提示，与后端 `siteconfig.Validate` 的字段错误同义。 */
export const PUBLIC_URL_ERROR = "公开地址须为 HTTP(S) 站点根地址，不含账号、查询或片段";

/**
 * 公开地址的即时校验。
 *
 * 与后端同规则：协议必须是 http/https、必须有主机名、不能带账号、查询或片段，
 * 且路径只能是空或 `/`（`siteconfig.Validate` 明确拒绝任何非空 path），因此每个
 * 站点的公开地址只能是**裸 origin**。表单用它挡住会被后端拒绝的输入，而不是等提交
 * 之后才报错。空值返回空字符串，交给必填规则处理。
 */
export function publicUrlError(value: string): string {
  const raw = value.trim();
  if (!raw) return "";
  let parsed: URL;
  try {
    parsed = new URL(raw);
  } catch {
    return PUBLIC_URL_ERROR;
  }
  const schemeOk = parsed.protocol === "http:" || parsed.protocol === "https:";
  const hasCredentials = Boolean(parsed.username || parsed.password);
  const hasPath = parsed.pathname !== "" && parsed.pathname !== "/";
  if (!schemeOk || !parsed.hostname || hasCredentials || parsed.search || parsed.hash || hasPath) {
    return PUBLIC_URL_ERROR;
  }
  return "";
}
