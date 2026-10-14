/**
 * 站点配置表单的共享选项。
 *
 * 「新增站点」弹窗与站点编辑表单都要用同一份语言/时区值域，集中在此避免两份拷贝。
 */

export type SiteOption = { value: string; label: string };

export const languageOptions: SiteOption[] = [
  { value: "zh-CN", label: "简体中文（zh-CN）" },
  { value: "en-US", label: "English（en-US）" },
  { value: "ja-JP", label: "日本語（ja-JP）" },
];

export const timezoneOptions: SiteOption[] = [
  { value: "Asia/Shanghai", label: "中国标准时间（Asia/Shanghai）" },
  { value: "UTC", label: "协调世界时（UTC）" },
  { value: "Asia/Tokyo", label: "日本标准时间（Asia/Tokyo）" },
  { value: "America/Los_Angeles", label: "太平洋时间（America/Los_Angeles）" },
];

/** 已保存的配置可能用了列表外的值，渲染时保留一条同值选项，避免选项被静默改掉。 */
export function optionsWithCurrent(options: SiteOption[], current: string): SiteOption[] {
  if (!current || options.some((option) => option.value === current)) return options;
  return [...options, { value: current, label: current }];
}
