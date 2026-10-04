import { Link, useSearchParams } from "react-router-dom";
import { DraftComparison } from "../articles/comparison";
import { useEffect, useRef, useState } from "react";
import { getSiteConfig, saveSiteConfig, type SiteConfigData, type WorkingSiteConfig } from "@/api/site-config";
import { PermissionGuard } from "@/components/auth/permission-guard";
import { PageHeader } from "@/components/common/page-header";
import { FormSection } from "@/components/common/form-section";
import { Field } from "@/components/common/field";
import { ContentCard } from "@/components/common/content-card";
import { EmptyState } from "@/components/common/empty-state";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Button } from "@/components/ui/button";
import { ApiError, getErrorMessage } from "@/lib/api-error";
import { submitPublication } from "@/api/publishing";
import { PublicationTask } from "../publications/task";
import { ConfirmDialog } from "@/components/common/confirm-dialog";
import { MediaSelector } from "../media/selector";

export function SiteConfigPage() {
 const [params]=useSearchParams(); const target=params.get("returnTo"); const returnTo=target && /^\/content\/articles(?:\/(?:new|[1-9][0-9]*))?$/.test(target) ? target : null;
  const [taskId, setTaskId] = useState<number | null>(null); const [confirmPublish, setConfirmPublish] = useState(false); const request = useRef<{ key: string; revision: number } | null>(null);
  const [working, setWorking] = useState<WorkingSiteConfig | null>(null); const [values, setValues] = useState<SiteConfigData | null>(null); const [error, setError] = useState(""); const [fields, setFields] = useState<Record<string, string>>({}); const [saving, setSaving] = useState(false); const [conflict, setConflict] = useState(false); const [latest, setLatest] = useState<WorkingSiteConfig | null>(null); const [message, setMessage] = useState("");
  useEffect(() => { let active = true; getSiteConfig().then((result) => { if (active) { setWorking(result); setValues(result.data); } }).catch((err: unknown) => { if (active) setError(getErrorMessage(err, "加载失败")); }); return () => { active = false; }; }, []);
  function change<K extends keyof SiteConfigData>(key: K, value: SiteConfigData[K]) { setValues((current) => current ? { ...current, [key]: value } : current); setMessage(""); }
  async function save() { if (!values || !working || saving || conflict) return; setSaving(true); setError(""); setFields({}); try { const result = await saveSiteConfig(working.version, values); setWorking(result); setMessage("已保存，尚未应用到网站"); } catch (err) { if (err instanceof ApiError) { if (err.status === 409) setConflict(true); if (err.fieldErrors) setFields(err.fieldErrors); } setError(getErrorMessage(err, "保存失败，本地内容已保留")); } finally { setSaving(false); } }

  async function publish() { if (!working?.revisionId || saving) return; setSaving(true); setError("");request.current ??= { key: crypto.randomUUID(), revision: working.revisionId }; try { const task = await submitPublication({ kind: "config", configRevisionId: request.current.revision }, request.current.key);request.current = null;setTaskId(task.id);setConfirmPublish(false); } catch (err) { if (err instanceof ApiError && err.type !== "network") request.current = null;setError(getErrorMessage(err, "发布提交失败，可重试同一请求")); } finally {setSaving(false);} }

  return <PermissionGuard permissionCode="content:config:edit" fallback={<EmptyState title="无访问权限" />}><PageHeader title="站点与作者配置" description="展示作者独立于系统用户。保存工作配置不会改变线上站点。" />
    {error && <p role="alert" className="mb-space-4 text-error">{error}</p>}
    {returnTo && <Link className="mb-space-4 inline-block text-primary" to={returnTo}>返回原文章，继续写作或确认发布</Link>}
    {values && working ? <form className="max-w-[1200px] space-y-space-6" onSubmit={(event) => { event.preventDefault(); void save(); }}>
      <p role="status">{JSON.stringify(values) !== JSON.stringify(working.data) ? "当前输入尚未保存。" : "当前输入已保存。"}{JSON.stringify(working.data) !== JSON.stringify(working.published?.data) ? " 已保存配置尚未应用到网站。" : " 已保存配置与线上一致。"}</p><ContentCard title={working.published ? "站点配置差异" : "首次设置清单"} description="保存与应用到网站分开。先填写站点名称、公开地址、作者名称，并确认语言与时区。"><DraftComparison local={values} remote={working.published?.data ?? {}} /></ContentCard><FormSection title="站点信息">{([{ key: "siteName", label: "站点名称" }, { key: "publicUrl", label: "公开地址" }, { key: "language", label: "语言" }, { key: "timezone", label: "时区" }] as const).map(({ key, label }) => <Field key={key} label={label} htmlFor={`config-${key}`} error={fields[key]} required><Input id={`config-${key}`} value={values[key]} disabled={saving || !!request.current} onChange={(event) => change(key, event.target.value)} /></Field>)}<div className="md:col-span-2"><Field label="站点简介" htmlFor="config-description" error={fields.description}><Textarea id="config-description" value={values.description} disabled={saving || !!request.current} onChange={(event) => change("description", event.target.value)} /></Field></div></FormSection>
      <FormSection title="展示作者"><Field label="作者名称" htmlFor="config-authorName" error={fields.authorName} required><Input id="config-authorName" value={values.authorName} disabled={saving || !!request.current} onChange={(event) => change("authorName", event.target.value)} /></Field><div className="md:col-span-2"><Field label="作者简介" htmlFor="config-authorBio" error={fields.authorBio}><Textarea id="config-authorBio" value={values.authorBio} disabled={saving || !!request.current} onChange={(event) => change("authorBio", event.target.value)} /></Field></div><div className="md:col-span-2"><MediaSelector label="头像" disabled={saving || !!request.current} coverMediaId={values.avatarMediaId} onCover={(file) => change("avatarMediaId", file?.id ?? null)} /></div></FormSection>
      {conflict && <ContentCard title="配置保存冲突" description="本地输入已保留。请读取服务器最新配置并明确选择继续编辑的基线。"><Button onClick={() => { void getSiteConfig().then(setLatest).catch((err: unknown) => setError(getErrorMessage(err, "读取失败"))); }}>读取最新版本</Button>{latest && <div className="mt-space-3"><pre className="overflow-auto whitespace-pre-wrap">{JSON.stringify(latest.data, null, 2)}</pre><Button onClick={() => { setWorking(latest); setLatest(null); setConflict(false); setError(""); }}>保留本地输入，以此版本继续</Button></div>}</ContentCard>}
      <div className="flex flex-wrap items-center gap-space-3 border-t border-border pt-space-4"><Button variant="primary" type="submit" disabled={saving || conflict || !!request.current}>{saving ? "保存中…" : "保存工作配置"}</Button><PermissionGuard permissionCode="content:config:publish"><Button disabled={saving || conflict || !!request.current || !working.revisionId || JSON.stringify(values) !== JSON.stringify(working.data)} onClick={() => setConfirmPublish(true)}>发布已保存配置</Button></PermissionGuard>{request.current && !saving && <Button onClick={() => void publish()}>重试配置修订 #{request.current.revision} 的同一发布请求</Button>}<span role="status">{message || `工作配置版本 ${working.version}`}</span></div>
      {taskId && <PublicationTask key={taskId} id={taskId} publicUrl={working.data.publicUrl} onSuccess={() => { void getSiteConfig().then((result) => { setWorking(result); setMessage("配置已应用到网站，可以返回文章确认发布。"); }).catch((err:unknown) => setError(getErrorMessage(err,"刷新上线配置失败"))); }} />}
    </form> : !error && <p role="status">加载中…</p>}
    <ConfirmDialog open={confirmPublish} title="发布站点配置？" description="固定已保存配置修订，文章保持线上版本。首次发布建立空文章站点。" loading={saving} onConfirm={() => void publish()} onCancel={() => setConfirmPublish(false)} />
  </PermissionGuard>;
}
