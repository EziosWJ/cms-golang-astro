import { useAuthStore } from "@/store/auth-store";
import { recoveryKey, temporaryIdentity, readRecovery, type RecoveryCopy } from "./recovery";
import { DraftComparison } from "./comparison";
import { useDeferredValue, useEffect, useRef, useState, type ClipboardEvent, type FormEvent } from "react";
import { useBlocker, useLocation, useNavigate, useParams } from "react-router-dom";
import MDEditor from "@uiw/react-md-editor";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import { createArticle, getArticle, saveArticleDraft, restoreArticleRevision, setArticleLifecycle } from "@/api/content";
import { PermissionGuard } from "@/components/auth/permission-guard";
import { PageHeader } from "@/components/common/page-header";
import { FormSection } from "@/components/common/form-section";
import { Field } from "@/components/common/field";
import { ContentCard } from "@/components/common/content-card";
import { EmptyState } from "@/components/common/empty-state";
import { ConfirmDialog } from "@/components/common/confirm-dialog";
import { Dialog, DialogOverlay, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { getEditorContext } from "@/api/site-config";
import { getMedia, uploadMedia } from "@/api/media";
import { hasPermission } from "@/lib/permission";
import { formatContentDate, parseContentDate } from "@/lib/content-date";
import { ApiError, getErrorMessage } from "@/lib/api-error";
import { createUUID } from "@/lib/uuid";
import { MediaSelector } from "../media/selector";
import { MarkdownMediaImage, MarkdownMediaLink } from "../media/preview";
import { TaxonomyPicker } from "../taxonomy/picker";
import { getPublications, submitPublication, type PublishKind, type PublishInput } from "@/api/publishing";
import { PublicationTask } from "../publications/task";
import { RevisionHistory } from "./revisions";
import { ARTICLE_EDIT_PERMISSION, type ArticleDetail, type ArticleDraftInput } from "@/types/content";

type FormValues = Omit<ArticleDraftInput, "displayDate"> & { displayDate: string };
const emptyValues: FormValues = { coverMediaId: null, categoryIds: [], tagIds: [], title: "", slug: "", markdown: "", summary: "", displayDate: "" };

function formFrom(detail: ArticleDetail, timezone: string): FormValues {
  return { coverMediaId: detail.draft.coverMediaId, categoryIds: (detail.draft.taxonomy ?? []).filter((term) => term.kind === "category").map((term) => term.id), tagIds: (detail.draft.taxonomy ?? []).filter((term) => term.kind === "tag").map((term) => term.id), title: detail.draft.title, slug: detail.slug, markdown: detail.draft.markdown, summary: detail.draft.summary, displayDate: formatContentDate(detail.draft.displayDate,timezone) };
}
function slugSuggestion(title: string) {
  return title.normalize("NFC").trim().toLowerCase().replace(/[^\p{L}\p{N}_-]+/gu, "-").replace(/^-+|-+$/g, "").slice(0, 200);
}

export function ArticleEditorPage() {
  const { id } = useParams();
  const route=useRef<{ id?:string; key:string }>({id,key:id ?? "new"});
 if(route.current.id !== id) { const migrating=route.current.id===undefined && !!id; route.current={id,key:migrating ? route.current.key : id ?? createUUID()}; }
 return <ArticleEditor key={route.current.key} id={id} />;
}

function ArticleEditor({ id }: { id?: string }) {
  const navigate = useNavigate();
 const location=useLocation(); const listReturn=new URLSearchParams(location.search).get("list"); const listPath=listReturn && /^\/content\/articles(?:\?[^#]*)?$/.test(listReturn) ? listReturn : "/content/articles";
  const account = useAuthStore((state) => state.user?.id);
 const [temporary] = useState(() => temporaryIdentity(account ?? 0));
 const [recovery, setRecovery] = useState<RecoveryCopy | null>(null);
 const [storageError, setStorageError] = useState("");
 const [settingsOpen,setSettingsOpen]=useState(false);
 const [narrow,setNarrow]=useState(() => window.matchMedia("(max-width: 767px)").matches);
 useEffect(() => { const query=window.matchMedia("(max-width: 767px)"); const changed=() => setNarrow(query.matches); query.addEventListener("change",changed); return () => query.removeEventListener("change",changed); },[]);
 const [editorMode, setEditorMode] = useState<"edit" | "preview" | "split">("edit");
 const [composing, setComposing] = useState(false);
 const recoveryPending = useRef(false);
 const recoveryReadKey = useRef<string | null>(null);
 const createRequest = useRef<RecoveryCopy["createRequest"]>(undefined);
 const activeID = useRef<number | null>(id ? Number(id) : null);
 const isNew = activeID.current === null;
 const [previewSnapshot, setPreviewSnapshot] = useState("");
  const [taskId, setTaskId] = useState<number | null>(null);
const [confirmation, setConfirmation] = useState<"article" | "unpublish" | "archive" | "activate" | null>(null);
const publishRequest = useRef<{ input: PublishInput; key: string } | null>(null);
const [publishError, setPublishError] = useState("");
const [detail, setDetail] = useState<ArticleDetail | null>(null);
  const [values, setValues] = useState<FormValues>(emptyValues);
  const [baseline, setBaseline] = useState<FormValues>(emptyValues);
  const [publicUrl, setPublicUrl] = useState("");
 const [hasConfig,setHasConfig] = useState(false);
  const [timezone, setTimezone] = useState("Asia/Shanghai");
const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [uploadingMedia,setUploadingMedia]=useState(false);
  const [uploadingImage, setUploadingImage] = useState(false);
  const [imageError, setImageError] = useState("");
  const imageUploadPending = useRef(false);
  const [autosaving, setAutosaving] = useState(false);
  const detailRef = useRef<ArticleDetail | null>(null);
  const queue = useRef<Promise<unknown>>(Promise.resolve());
  const autoSaveRef = useRef<() => Promise<void>>(async () => {});
  const conflictRef = useRef(false);
  const [loadError, setLoadError] = useState("");
  const [saveError, setSaveError] = useState("");
  const [conflict, setConflict] = useState(false);
  const [latest, setLatest] = useState<ArticleDetail | null>(null);
  const [latestLoading, setLatestLoading] = useState(false);
  const [slugEdited, setSlugEdited] = useState(false);
  const [savedMessage, setSavedMessage] = useState("");
  const submitting = useRef(false);
 const valuesRef = useRef(values); valuesRef.current = values;
  const dirty = uploadingImage || uploadingMedia || JSON.stringify(values) !== JSON.stringify(baseline);
  const blocker = useBlocker(({ currentLocation, nextLocation }) => dirty && !submitting.current && nextLocation.pathname !== `/content/articles/${activeID.current}` && currentLocation.pathname !== nextLocation.pathname);
  const preview = useDeferredValue(values.markdown);
  const readOnly = loading || saving || uploadingImage || uploadingMedia || confirmation !== null || (detail !== null && detail.lifecycle !== "active");

  useEffect(() => {
    if (detailRef.current && detailRef.current.id === activeID.current && (!id || Number(id) === activeID.current)) return;
    const controller = new AbortController();
if (isNew) { getEditorContext(controller.signal).then((context)=> { if (!controller.signal.aborted) { setTimezone(context.timezone); setHasConfig(!!context.published); setPublicUrl(context.published?.data.publicUrl ?? ""); } }).catch((error:unknown)=>{if(!controller.signal.aborted)setLoadError(getErrorMessage(error,"加载时区失败"));}).finally(()=>{if(!controller.signal.aborted)setLoading(false);});return ()=>controller.abort(); }
    if (!/^\d+$/.test(id ?? "") || Number(id) < 1) {
      setLoadError("文章不存在"); setLoading(false); return;
    }
    Promise.all([getArticle(Number(id), controller.signal),getEditorContext(controller.signal)]).then(([result, configuration]) => {
      if (controller.signal.aborted) return;
      setTimezone(configuration.timezone); setHasConfig(!!configuration.published); setPublicUrl(configuration.published?.data.publicUrl ?? "");
      const next = formFrom(result,configuration.timezone);
      void getPublications({page:1,pageSize:100}).then((tasks) => { if (!controller.signal.aborted) { const task=tasks.records.find((t) => t.articleId===result.id && ["queued","running","failed","interrupted"].includes(t.status)); if(task)setTaskId(task.id); } }).catch(() => undefined);
      activeID.current=result.id; detailRef.current = result; setDetail(result); setValues(next); setBaseline(next); setSlugEdited(true);
    }).catch((error: unknown) => {
      if (!controller.signal.aborted) setLoadError(getErrorMessage(error, "加载文章失败"));
    }).finally(() => { if (!controller.signal.aborted) setLoading(false); });
    return () => controller.abort();
  }, [id, isNew]);

  const copyKey = account ? recoveryKey(account, id ?? `new-${temporary}`) : null;
 useEffect(() => {
 if (loading || !copyKey || recoveryReadKey.current===copyKey) return;
 recoveryReadKey.current=copyKey;
 try { const copy=readRecovery(copyKey); if (copy && (JSON.stringify(copy.values) !== JSON.stringify(baseline) || copy.createRequest)) { recoveryPending.current=true; setRecovery(copy); } }
 catch (error) { setStorageError(getErrorMessage(error, "此浏览器无法读取恢复副本。")); }
 // A recovery decision is made once after loading each identity.
 // eslint-disable-next-line react-hooks/exhaustive-deps
 }, [loading, copyKey]);
 useEffect(() => {
 if (loading || !copyKey || recovery || recoveryPending.current) return;
 try {
 if (dirty || createRequest.current) localStorage.setItem(copyKey, JSON.stringify({ values, baseline, version: detail?.draft.version ?? null, createRequest: createRequest.current, savedAt:new Date().toISOString() } satisfies RecoveryCopy));
 else localStorage.removeItem(copyKey);
 } catch { setStorageError("浏览器临时保护不可用或空间不足；请保持页面打开并重试服务器保存。"); }
 }, [loading, copyKey, recovery, dirty, values, baseline, detail?.draft.version]);
 function clearCopy() { if (!copyKey) return; try { localStorage.removeItem(copyKey); recoveryPending.current=false; setRecovery(null); } catch { setStorageError("无法清除浏览器副本。"); } }
 function recoverCopy() {
 if (!recovery) return;
 setValues(recovery.values); createRequest.current=recovery.createRequest;
 if (detail && recovery.version !== detail.draft.version) { conflictRef.current=true; setConflict(true); setLatest(detail); }
 recoveryPending.current=false; setRecovery(null);
 }
  useEffect(() => {
    const beforeUnload = (event: BeforeUnloadEvent) => {
      if (dirty) { event.preventDefault(); event.returnValue = ""; }
    };
    window.addEventListener("beforeunload", beforeUnload);
    return () => window.removeEventListener("beforeunload", beforeUnload);
  }, [dirty]);

  function change<K extends keyof FormValues>(key: K, value: FormValues[K]) {
    const normalized = key === "displayDate" && typeof value === "string" && value.length === 16 ? `${value}:00` : value;
    setValues((current) => ({ ...current, [key]: normalized, ...(key === "title" && isNew && !slugEdited ? { slug: slugSuggestion(String(value)) } : {}) }));
    setSavedMessage("");
  }

  async function pasteImages(event: ClipboardEvent<HTMLTextAreaElement>) {
    const images = Array.from(event.clipboardData.items)
      .filter((item) => item.kind === "file" && item.type.startsWith("image/"))
      .map((item) => item.getAsFile()).filter((file): file is File => file !== null);
    if (!images.length) return;
    event.preventDefault();
    if (readOnly || imageUploadPending.current || uploadingMedia) return;
    if (!hasPermission("content:media:edit")) {
      setImageError("没有上传媒体的权限，请联系管理员。");
      return;
    }
    const textarea = event.currentTarget;
    const start = textarea.selectionStart;
    const end = textarea.selectionEnd;
    const markdown = textarea.value;
    imageUploadPending.current = true;
    setUploadingImage(true); setImageError("");
    const inserted: string[] = [];
    try {
      for (const image of images) {
        const file = await uploadMedia(image, { businessModule: "attachment" });
        const media = await getMedia(file.id);
        if (!media.image) throw new Error("该文件不是站点支持的图片格式，请使用 PNG、JPEG 或 GIF。");
        const alt = media.originalName.replace(/[[\]\\\r\n]/g, "");
        inserted.push(`![${alt}](${media.stablePath})`);
      }
    } catch (error) {
      setImageError(getErrorMessage(error, "图片上传失败，请重新粘贴；已上传的图片可从媒体库插入。"));
    } finally {
      if (inserted.length) {
        const text = `\n${inserted.join("\n")}\n`;
        change("markdown", markdown.slice(0, start) + text + markdown.slice(end));
        window.requestAnimationFrame(() => {
          if (!textarea.isConnected) return;
          textarea.focus(); textarea.setSelectionRange(start + text.length, start + text.length);
        });
      }
      imageUploadPending.current = false;
      setUploadingImage(false);
    }
  }

  async function persist(mode: "manual" | "autosave") {
    if (submitting.current || readOnly || conflictRef.current || recovery !== null || composing) return;
    if (mode === "manual") { submitting.current = true; setSaving(true); }
    else setAutosaving(true);
    setSaveError(""); setSavedMessage("");
    const snapshot = { ...values };
    const operation = async () => {
      if (conflictRef.current) return;
      const input: ArticleDraftInput = { ...snapshot, displayDate: parseContentDate(snapshot.displayDate,timezone) };
      let result: ArticleDetail;
 if (activeID.current === null) {
 createRequest.current ??= { ...input, requestKey:createUUID(), createMode:mode };
 if (copyKey) { try { localStorage.setItem(copyKey, JSON.stringify({ values:snapshot, baseline, version:null, createRequest:createRequest.current, savedAt:new Date().toISOString() })); } catch { setStorageError("无法保存恢复副本；请求重试仍使用同一身份。"); } }
 const creationMode=createRequest.current.createMode;
 result = await createArticle(createRequest.current);
 if(result.draft.version !== 1) { conflictRef.current=true; setConflict(true); setLatest(result); setSaveError("首次请求已保存，但服务器随后已有修改。请比较内容后明确选择。"); }
 activeID.current=result.id;
 createRequest.current=undefined;
 detailRef.current=result; setDetail(result); setBaseline(formFrom(result,timezone));
 if (account) recoveryReadKey.current=recoveryKey(account,String(result.id));
 // Copy unconfirmed later input before changing the temporary URL identity.
 if (copyKey && account) { try { localStorage.setItem(recoveryKey(account,String(result.id)), JSON.stringify({ values:valuesRef.current, baseline:formFrom(result,timezone), version:result.draft.version, savedAt:new Date().toISOString() })); localStorage.removeItem(copyKey); } catch { setStorageError("恢复副本迁移失败，请保持页面打开直到保存完成。"); } }
 if(!id) navigate(`/content/articles/${result.id}${listReturn ? `?list=${encodeURIComponent(listPath)}` : ""}`, {replace:true});
 if(mode === "manual" && creationMode === "autosave" && !conflictRef.current) result=await saveArticleDraft(result.id,result.draft.version,input,"manual");
 } else result = await saveArticleDraft(activeID.current, detailRef.current!.draft.version, input, mode);
      detailRef.current = result;
      setDetail(result); setBaseline(formFrom(result,timezone));
      setSavedMessage(mode === "manual" ? "已保存版本，尚未发布" : "已保存");
      if (!id) navigate(`/content/articles/${result.id}${listReturn ? `?list=${encodeURIComponent(listPath)}` : ""}`, { replace: true });
    };
    const request = queue.current.then(operation);
    queue.current = request.catch(() => undefined);
    try { await request; }
    catch (error) {
      if (error instanceof ApiError && error.status === 409 && typeof error.data === "object" && error.data !== null && "reason" in error.data && error.data.reason === "version_conflict" && activeID.current !== null) { conflictRef.current = true; setConflict(true); }
      setSaveError(getErrorMessage(error, "保存失败，本地内容已保留"));
    } finally {
      if (mode === "manual") { submitting.current = false; setSaving(false); }
      else setAutosaving(false);
    }
  }
  autoSaveRef.current = () => persist("autosave");
  useEffect(() => {
    if (loading || composing || recovery || confirmation !== null || saving || uploadingImage || autosaving || conflict || !dirty || saveError || (detail && detail.lifecycle !== "active") || (isNew && !values.title.trim() && !values.markdown.trim())) return;
    const timer = window.setTimeout(() => { void autoSaveRef.current(); }, 2000);
    return () => window.clearTimeout(timer);
  }, [isNew, loading, composing, recovery, confirmation, saving, uploadingImage, autosaving, conflict, dirty, values, saveError, detail]);

  async function save(event: FormEvent) { event.preventDefault(); await persist("manual"); }
  async function restore(revisionID: number) {
    if (submitting.current || conflictRef.current) return;
    submitting.current = true; setSaving(true); setSaveError("");
    const operation = async () => {
      if (conflictRef.current) return;
      const result = await restoreArticleRevision(activeID.current!, revisionID, detailRef.current!.draft.version);
      detailRef.current = result; setDetail(result);
      const restored = formFrom(result,timezone); setValues(restored); setBaseline(restored);
      setSavedMessage("已恢复到工作稿，尚未发布");
    };
    const request = queue.current.then(operation); queue.current = request.catch(() => undefined);
    try { await request; }
    catch (error) {
      if (error instanceof ApiError && error.status === 409 && typeof error.data === "object" && error.data !== null && "reason" in error.data && error.data.reason === "version_conflict") { conflictRef.current = true; setConflict(true); }
      setSaveError(getErrorMessage(error, "恢复失败，本地内容已保留"));
    } finally { submitting.current = false; setSaving(false); }
  }


  async function publish(kind: PublishKind) {
    if (isNew || submitting.current || conflictRef.current || imageUploadPending.current || uploadingMedia) return;
    if (!hasConfig) { setConfirmation(null); setPublishError("首次发布前，请先补齐并发布站点与作者资料。配置发布成功后返回本文，再确认发布。"); return; }
 if (kind !== "unpublish" && (!values.title.trim() || !values.markdown.trim() || !values.slug || /[\s/\\%?#]/u.test(values.slug))) { setConfirmation(null); setPublishError("请填写标题、非空正文和合法文章地址，再发布或预览。"); if(!values.slug) setSettingsOpen(true); window.requestAnimationFrame(() => { const field=document.getElementById(!values.title.trim() ? "article-title" : !values.markdown.trim() ? "article-markdown" : "article-slug"); const section=field?.closest("details"); if(section)section.open=true; field?.focus(); }); return; }
    submitting.current = true; setSaving(true); setPublishError("");
    try {
      await queue.current;
      if (conflictRef.current) return;
      const input: PublishInput = { kind, articleId: activeID.current!, ...(kind !== "unpublish" ? { save: { ...values, displayDate: parseContentDate(values.displayDate,timezone), expectedVersion: detailRef.current!.draft.version, mode: "manual" as const } } : {}) };
      publishRequest.current ??= { input, key: createUUID() };
      const request = publishRequest.current;
      const task = await submitPublication(request.input, request.key);
      publishRequest.current = null; setTaskId(task.id);
      const saved = await getArticle(activeID.current!); detailRef.current = saved; setDetail(saved); const confirmed=formFrom(saved,timezone); setBaseline(confirmed); setValues((current) => JSON.stringify(current)===JSON.stringify(values) ? confirmed : current);
 if (kind === "preview") setPreviewSnapshot(JSON.stringify(formFrom(saved,timezone))); else setPreviewSnapshot("");
    } catch (error) { if (error instanceof ApiError && error.status===409 && typeof error.data === "object" && error.data!==null && "reason" in error.data && error.data.reason === "version_conflict") { conflictRef.current=true; setConflict(true); } if (error instanceof ApiError && error.type !== "network") publishRequest.current = null; setPublishError(getErrorMessage(error, "提交失败，可用相同请求重试")); }
    finally { submitting.current = false; setSaving(false); setConfirmation(null); }
  }
  async function lifecycle(next: "active" | "archived") {
    if (submitting.current || conflictRef.current) return;
    submitting.current = true; setSaving(true);
    try { await queue.current; if (conflictRef.current) return; const result = await setArticleLifecycle(activeID.current!, detailRef.current!.draft.version, next); detailRef.current = result;setDetail(result);setValues(formFrom(result,timezone));setBaseline(formFrom(result,timezone)); }
    catch (error) { setSaveError(getErrorMessage(error, "生命周期操作失败")); }
    finally { submitting.current = false;setSaving(false);setConfirmation(null); }
  }

  async function loadLatest() {
    setLatestLoading(true);
    try { setLatest(await getArticle(activeID.current!)); }
    catch (error) { setSaveError(getErrorMessage(error, "读取最新工作稿失败")); }
    finally { setLatestLoading(false); }
  }

  function useLatestVersion() {
    if (!latest) return;
    if (!window.confirm("保留本地全部内容，以最新服务器版本继续保存？服务器内容将被替换；再次并发修改仍会检测冲突。")) return;
    // Keep the local form. The user explicitly chooses the version to replace.
    detailRef.current = latest; conflictRef.current = false; setDetail(latest); setBaseline(formFrom(latest,timezone)); setConflict(false); setLatest(null); setSaveError("");
  }

  const settingsContent = (<div className="mt-space-4 space-y-space-4">          <FormSection columns={1} title="文章设置">
 {detail && <Button onClick={() => { setSettingsOpen(false); window.requestAnimationFrame(() => document.getElementById("article-history")?.scrollIntoView({block:"start"})); }}>历史版本</Button>}

            <Field label="文章地址" htmlFor="article-slug" help="中文和数字可用；首次成功发布后固定。"><Input id="article-slug" value={values.slug} maxLength={200} disabled={readOnly || !!detail?.slugLockedAt} onChange={(event) => { setSlugEdited(true); change("slug", event.target.value); }} /></Field>
            {publicUrl && values.slug && <p className="break-all text-sm text-text-secondary">公开地址：{publicUrl.replace(/\/$/, "")}/archives/{encodeURIComponent(values.slug)}/</p>}
            <Field label="展示日期" htmlFor="article-date" help="可留空，发布时确定默认日期。"><Input id="article-date" type="datetime-local" step="1" value={values.displayDate} disabled={readOnly} onChange={(event) => change("displayDate", event.target.value)} /></Field>
            <div className="col-span-1"><Field label="摘要" htmlFor="article-summary"><Textarea id="article-summary" value={values.summary} maxLength={5000} disabled={readOnly} onChange={(event) => change("summary", event.target.value)} /></Field></div>
          </FormSection>
          <TaxonomyPicker categoryIds={values.categoryIds} tagIds={values.tagIds} disabled={readOnly} onChange={(kind, ids) => change(kind === "category" ? "categoryIds" : "tagIds", ids)} />
          <ContentCard title="媒体与封面"><MediaSelector onUploading={setUploadingMedia} disabled={readOnly} onSelect={(file) => { const textarea=document.getElementById("article-markdown") as HTMLTextAreaElement | null; const start=textarea?.selectionStart ?? values.markdown.length; const end=textarea?.selectionEnd ?? start; const text=`\n${file.image ? "!" : ""}[${file.originalName.replace(/[[\]\\\r\n]/g, "")}](${file.stablePath})\n`; change("markdown", valuesRef.current.markdown.slice(0,start)+text+valuesRef.current.markdown.slice(end)); }} onCover={(file) => change("coverMediaId", file?.id ?? null)} coverMediaId={values.coverMediaId} /></ContentCard>
</div>);
  return (
    <PermissionGuard permissionCode={ARTICLE_EDIT_PERMISSION} fallback={<EmptyState title="无访问权限" description="请联系管理员授予内容编辑权限。" />}>
      <PageHeader title={isNew ? "新建文章" : "编辑文章"} description="保存工作稿不会改变网站。发布更新才会对读者生效。" actions={<Button onClick={() => navigate(listPath)}>返回列表</Button>} />
      {detail && <p className="mb-space-3 text-sm text-text-secondary" role="status">{detail.lifecycle === "archived" ? "已归档" : detail.published ? `已上线${detail.unpublishedChanges || dirty ? " · 仍有未发布修改" : " · 工作稿与线上内容一致"}` : "草稿 · 尚未上线"}</p>}
      {!hasConfig && !loading && <ContentCard title="首次设置" description="可以先写文章。首次发布前，需要单独发布站点名称、公开地址和作者资料。"><Button onClick={() => navigate(`/content/site-config?returnTo=${encodeURIComponent(id ? `/content/articles/${id}` : "/content/articles/new")}`)}>设置站点与作者</Button></ContentCard>}
      {storageError && <p role="alert" className="mb-space-4 text-warning">{storageError}</p>}
 {recovery && <ContentCard title="发现浏览器临时恢复副本" description="仅此浏览器可用。恢复前请比较输入；数据库仍是内容主数据源。"><DraftComparison local={recovery.values} remote={baseline} /><Button onClick={recoverCopy}>恢复本地输入</Button><Button onClick={() => { if (window.confirm("清除这份未确认保存的浏览器副本？")) clearCopy(); }}>放弃并清除副本</Button></ContentCard>}
 {dirty && !recovery && <p className="mb-space-3 text-sm text-text-secondary">{storageError ? "当前输入尚未保存到服务器，浏览器保护受限。" : "临时副本仅保留在当前浏览器。"}<Button size="sm" onClick={() => { if (window.confirm("清除本地恢复副本？当前编辑输入仍保留，后续修改会重新保护。")) clearCopy(); }}>清除副本</Button></p>}
      {loadError ? <EmptyState title="加载失败" description={loadError} /> : (
        <form className="max-w-[1200px] space-y-space-6" onSubmit={(event) => void save(event)}>
          <div className="grid min-w-0 items-start gap-space-4 xl:grid-cols-[minmax(0,1fr)_320px]"><section className="min-w-0 space-y-space-4">
          <Field label="标题" htmlFor="article-title" help="可以保存尚未完成的标题。"><Input id="article-title" onCompositionStart={() => setComposing(true)} onCompositionEnd={() => setComposing(false)} value={values.title} maxLength={500} disabled={readOnly} onChange={(event) => change("title", event.target.value)} /></Field>
          <ContentCard title="Markdown 正文" description="源码编辑与即时预览；支持表格、列表及围栏代码。截图后在输入框按 Ctrl+V（Mac 使用 ⌘V）上传图片。">
            {uploadingImage && <p role="status" className="mb-space-3 text-sm text-text-secondary">正在上传图片，请稍候…</p>}
            {imageError && <p role="alert" className="mb-space-3 text-sm text-error">{imageError}</p>}
            <div className="mb-space-3 flex flex-wrap gap-space-2">{([ ["edit","源码"],["preview","正文预览"],["split","分屏"] ] as const).map(([mode,label]) => <Button key={mode} size="sm" aria-pressed={editorMode===mode} onClick={() => setEditorMode(mode)}>{label}</Button>)}</div>
 <div className={`grid min-w-0 gap-space-4 ${editorMode === "split" ? "lg:grid-cols-2" : ""}`}>
              <div className={editorMode === "preview" ? "hidden" : "min-w-0"} data-color-mode="light"><MDEditor value={values.markdown} onChange={(value) => { if (!readOnly) change("markdown", value ?? ""); }} height={480} preview="edit" extraCommands={[]} defaultTabEnable textareaProps={{ id: "article-markdown", "aria-label": "Markdown 正文", disabled: readOnly, spellCheck: false, onCompositionStart: () => setComposing(true), onCompositionEnd: () => setComposing(false), onPaste: (event) => { void pasteImages(event); } }} /></div>
              <section hidden={editorMode === "edit"} className="article-markdown-preview min-w-0 overflow-auto rounded-control border border-border p-space-4" aria-label="正文即时预览">
                <ReactMarkdown remarkPlugins={[remarkGfm]} components={{ img: MarkdownMediaImage, a: MarkdownMediaLink }}>{preview}</ReactMarkdown>
              </section>
            </div>
          </ContentCard>
          </section><aside className="min-w-0">
          {narrow ? <><Button onClick={() => setSettingsOpen(true)}>文章设置、插图与封面</Button><Dialog open={settingsOpen} onOpenChange={setSettingsOpen} trapFocus restoreFocus lockScroll><DialogOverlay /><DialogContent className="max-h-[85dvh] overflow-y-auto"><DialogHeader><DialogTitle>文章设置、插图与封面</DialogTitle></DialogHeader>{settingsContent}<Button onClick={() => setSettingsOpen(false)}>完成设置</Button></DialogContent></Dialog></> : <details className="rounded-admin border border-border bg-surface p-space-4"><summary className="cursor-pointer font-medium">文章设置、插图与封面</summary>{settingsContent}</details>}
          </aside></div>
          {conflict && <ContentCard title="工作稿保存冲突" description="本地内容已保留。读取最新版本后，确认是否用本地内容替换它。"><Button disabled={latestLoading} onClick={() => void loadLatest()}>{latestLoading ? "读取中…" : "读取最新版本"}</Button>{latest && <div className="mt-space-4 space-y-space-3"><p>服务器版本 {latest.draft.version} · {latest.draft.title || "未命名文章"}</p><DraftComparison local={values} remote={formFrom(latest,timezone)} /><Button onClick={() => { if (!latest || !window.confirm("放弃本地修改并采用服务器全部内容？")) return; const next=formFrom(latest,timezone); setValues(next); setBaseline(next); detailRef.current=latest; setDetail(latest); conflictRef.current=false; setConflict(false); setLatest(null); setSaveError(""); }}>采用服务器内容</Button><Button onClick={useLatestVersion}>保留本地内容，以此版本继续编辑</Button></div>}</ContentCard>}
          <div className="sticky bottom-0 z-10 flex flex-wrap items-center gap-space-3 rounded-control border border-border bg-surface p-space-3">
            <Button variant="primary" type="submit" disabled={readOnly || conflict || !!publishRequest.current}>{saving ? "保存中…" : "保存版本"}</Button>
            {!isNew && detail?.lifecycle === "active" && <><Button disabled={readOnly || conflict || !!publishRequest.current} onClick={() => void publish("preview")}>网站效果预览</Button><PermissionGuard permissionCode="content:publish"><Button disabled={readOnly || conflict || !!publishRequest.current} onClick={() => setConfirmation("article")}>{detail.published ? "发布更新" : "发布文章"}</Button>{detail.published && <details><summary className="cursor-pointer text-sm">更多操作</summary><Button disabled={readOnly || conflict || !!publishRequest.current} onClick={() => setConfirmation("unpublish")}>下线文章</Button></details>}</PermissionGuard>{!detail.published && <details><summary className="cursor-pointer text-sm">更多操作</summary><Button disabled={readOnly || conflict || dirty || !!publishRequest.current} onClick={() => setConfirmation("archive")}>归档</Button></details>}</>}
            {!isNew && detail?.lifecycle === "archived" && <Button disabled={saving || conflict} onClick={() => setConfirmation("activate")}>恢复编辑</Button>}
            <span className="text-sm text-text-secondary" role="status">{conflict ? "保存冲突" : saveError ? "保存失败" : saving || autosaving ? "正在保存" : loading ? "加载中…" : dirty ? "尚未保存" : savedMessage || (detail ? "已保存" : "尚未保存")}</span>
            {saveError && !conflict && <Button onClick={() => void persist("manual")}>重试保存</Button>}
            {saveError && <p className="w-full text-sm text-error" role="alert">{saveError}</p>}
          </div>
          {publishError && <ContentCard title="提交失败"><p role="alert" className="text-error">{publishError}</p>{publishRequest.current && <Button disabled={saving} onClick={() => void publish(publishRequest.current!.input.kind)}>重试同一请求</Button>}</ContentCard>}
          {previewSnapshot && previewSnapshot !== JSON.stringify(values) && <p role="status">文章已有新修改，此网站预览尚未包含。点击网站效果预览可更新。</p>}
          {taskId && <PublicationTask key={taskId} id={taskId} publicUrl={publicUrl && detail?.slug ? `${publicUrl.replace(/\/$/, "")}/archives/${encodeURIComponent(detail.slug)}/` : undefined} onSuccess={() => { void getArticle(activeID.current!).then((result) => { detailRef.current = result;setDetail(result); }); }} />}
          {!isNew && detail && <section id="article-history" className="scroll-mt-20"><RevisionHistory current={values} timezone={timezone} articleId={detail.id} version={detail.draft.version} disabled={readOnly || conflict} dirty={dirty} onRestore={restore} /></section>}
        </form>
      )}
      <ConfirmDialog open={confirmation !== null} title={confirmation === "article" ? "发布当前文章？" : confirmation === "unpublish" ? "下线文章？" : confirmation === "archive" ? "归档文章？" : "恢复文章编辑？"} description={confirmation === "article" ? "提交时固定当前工作稿。其他文章和站点配置保持线上版本。" : confirmation === "unpublish" ? "完成后公开站点移除此文章及无文章的聚合入口。" : confirmation === "archive" ? "已下线且无在途任务的文章可以归档。" : "恢复工作稿编辑，不自动上线。"} loading={saving} onCancel={() => setConfirmation(null)} onConfirm={() => { if (confirmation === "article" || confirmation === "unpublish") void publish(confirmation); else void lifecycle(confirmation === "archive" ? "archived" : "active"); }} />
      <ConfirmDialog open={blocker.state === "blocked"} title="离开文章编辑？" description="尚未保存的本地修改将丢失。" confirmText="离开" cancelText="继续编辑" onConfirm={() => blocker.state === "blocked" && blocker.proceed()} onCancel={() => blocker.state === "blocked" && blocker.reset()} />
    </PermissionGuard>
  );
}
