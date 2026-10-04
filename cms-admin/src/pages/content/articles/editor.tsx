import { useDeferredValue, useEffect, useRef, useState, type ClipboardEvent, type FormEvent } from "react";
import { useBlocker, useNavigate, useParams } from "react-router-dom";
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
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { getEditorTimezone } from "@/api/site-config";
import { getMedia, uploadMedia } from "@/api/media";
import { hasPermission } from "@/lib/permission";
import { formatContentDate, parseContentDate } from "@/lib/content-date";
import { ApiError, getErrorMessage } from "@/lib/api-error";
import { MediaSelector } from "../media/selector";
import { MarkdownMediaImage, MarkdownMediaLink } from "../media/preview";
import { TaxonomyPicker } from "../taxonomy/picker";
import { submitPublication, type PublishKind, type PublishInput } from "@/api/publishing";
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
  // Remounting per identity keeps old article state out of a new route.
  return <ArticleEditor key={id ?? "new"} id={id} />;
}

function ArticleEditor({ id }: { id?: string }) {
  const navigate = useNavigate();
  const isNew = !id;
  const [taskId, setTaskId] = useState<number | null>(null);
const [confirmation, setConfirmation] = useState<"article" | "unpublish" | "archive" | "activate" | null>(null);
const publishRequest = useRef<{ input: PublishInput; key: string } | null>(null);
const [publishError, setPublishError] = useState("");
const [detail, setDetail] = useState<ArticleDetail | null>(null);
  const [values, setValues] = useState<FormValues>(emptyValues);
  const [baseline, setBaseline] = useState<FormValues>(emptyValues);
  const [timezone, setTimezone] = useState("Asia/Shanghai");
const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
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
  const dirty = uploadingImage || JSON.stringify(values) !== JSON.stringify(baseline);
  const blocker = useBlocker(({ currentLocation, nextLocation }) => dirty && !submitting.current && currentLocation.pathname !== nextLocation.pathname);
  const preview = useDeferredValue(values.markdown);
  const readOnly = loading || saving || uploadingImage || (detail !== null && detail.lifecycle !== "active");

  useEffect(() => {
    const controller = new AbortController();
if (isNew) { getEditorTimezone(controller.signal).then(({timezone})=> { if (!controller.signal.aborted) setTimezone(timezone); }).catch((error:unknown)=>{if(!controller.signal.aborted)setLoadError(getErrorMessage(error,"加载时区失败"));}).finally(()=>{if(!controller.signal.aborted)setLoading(false);});return ()=>controller.abort(); }
    if (!/^\d+$/.test(id ?? "") || Number(id) < 1) {
      setLoadError("文章不存在"); setLoading(false); return;
    }
    Promise.all([getArticle(Number(id), controller.signal),getEditorTimezone(controller.signal)]).then(([result, configuration]) => {
      if (controller.signal.aborted) return;
      setTimezone(configuration.timezone);
      const next = formFrom(result,configuration.timezone);
      detailRef.current = result; setDetail(result); setValues(next); setBaseline(next); setSlugEdited(true);
    }).catch((error: unknown) => {
      if (!controller.signal.aborted) setLoadError(getErrorMessage(error, "加载文章失败"));
    }).finally(() => { if (!controller.signal.aborted) setLoading(false); });
    return () => controller.abort();
  }, [id, isNew]);

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
    if (readOnly || imageUploadPending.current) return;
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
    if (submitting.current || readOnly || conflictRef.current || (mode === "autosave" && isNew)) return;
    if (mode === "manual") { submitting.current = true; setSaving(true); }
    else setAutosaving(true);
    setSaveError(""); setSavedMessage("");
    const snapshot = { ...values };
    const operation = async () => {
      if (conflictRef.current) return;
      const input: ArticleDraftInput = { ...snapshot, displayDate: parseContentDate(snapshot.displayDate,timezone) };
      const result = isNew ? await createArticle(input) : await saveArticleDraft(Number(id), detailRef.current!.draft.version, input, mode);
      detailRef.current = result;
      setDetail(result); setBaseline(formFrom(result,timezone));
      setSavedMessage(mode === "manual" ? `已保存工作稿版本 ${result.draft.version}，修订 #${result.revisionId}` : `自动保存完成 · 工作稿版本 ${result.draft.version}`);
      if (isNew) navigate(`/content/articles/${result.id}`, { replace: true });
    };
    const request = queue.current.then(operation);
    queue.current = request.catch(() => undefined);
    try { await request; }
    catch (error) {
      if (error instanceof ApiError && error.status === 409 && typeof error.data === "object" && error.data !== null && "reason" in error.data && error.data.reason === "version_conflict" && !isNew) { conflictRef.current = true; setConflict(true); }
      setSaveError(getErrorMessage(error, "保存失败，本地内容已保留"));
    } finally {
      if (mode === "manual") { submitting.current = false; setSaving(false); }
      else setAutosaving(false);
    }
  }
  autoSaveRef.current = () => persist("autosave");
  useEffect(() => {
    if (isNew || loading || saving || uploadingImage || autosaving || conflict || !dirty || saveError || detail?.lifecycle !== "active") return;
    const timer = window.setTimeout(() => { void autoSaveRef.current(); }, 1500);
    return () => window.clearTimeout(timer);
  }, [isNew, loading, saving, uploadingImage, autosaving, conflict, dirty, values, saveError, detail?.lifecycle]);

  async function save(event: FormEvent) { event.preventDefault(); await persist("manual"); }
  async function restore(revisionID: number) {
    if (submitting.current || conflictRef.current) return;
    submitting.current = true; setSaving(true); setSaveError("");
    const operation = async () => {
      if (conflictRef.current) return;
      const result = await restoreArticleRevision(Number(id), revisionID, detailRef.current!.draft.version);
      detailRef.current = result; setDetail(result);
      const restored = formFrom(result,timezone); setValues(restored); setBaseline(restored);
      setSavedMessage(`已恢复修订 #${revisionID} 到工作稿版本 ${result.draft.version}`);
    };
    const request = queue.current.then(operation); queue.current = request.catch(() => undefined);
    try { await request; }
    catch (error) {
      if (error instanceof ApiError && error.status === 409) { conflictRef.current = true; setConflict(true); }
      setSaveError(getErrorMessage(error, "恢复失败，本地内容已保留"));
    } finally { submitting.current = false; setSaving(false); }
  }


  async function publish(kind: PublishKind) {
    if (isNew || submitting.current || conflictRef.current) return;
    submitting.current = true; setSaving(true); setPublishError("");
    try {
      await queue.current;
      if (conflictRef.current) return;
      const input: PublishInput = { kind, articleId: Number(id), ...(kind !== "unpublish" ? { save: { ...values, displayDate: parseContentDate(values.displayDate,timezone), expectedVersion: detailRef.current!.draft.version, mode: "manual" as const } } : {}) };
      publishRequest.current ??= { input, key: crypto.randomUUID() };
      const request = publishRequest.current;
      const task = await submitPublication(request.input, request.key);
      publishRequest.current = null; setTaskId(task.id);
      const saved = await getArticle(Number(id)); detailRef.current = saved; setDetail(saved);setValues(formFrom(saved,timezone));setBaseline(formFrom(saved,timezone));
    } catch (error) { if (error instanceof ApiError && error.type !== "network") publishRequest.current = null; setPublishError(getErrorMessage(error, "提交失败，可用相同请求重试")); }
    finally { submitting.current = false; setSaving(false); setConfirmation(null); }
  }
  async function lifecycle(next: "active" | "archived") {
    if (submitting.current || conflictRef.current) return;
    submitting.current = true; setSaving(true);
    try { await queue.current; if (conflictRef.current) return; const result = await setArticleLifecycle(Number(id), detailRef.current!.draft.version, next); detailRef.current = result;setDetail(result);setValues(formFrom(result,timezone));setBaseline(formFrom(result,timezone)); }
    catch (error) { setSaveError(getErrorMessage(error, "生命周期操作失败")); }
    finally { submitting.current = false;setSaving(false);setConfirmation(null); }
  }

  async function loadLatest() {
    setLatestLoading(true);
    try { setLatest(await getArticle(Number(id))); }
    catch (error) { setSaveError(getErrorMessage(error, "读取最新工作稿失败")); }
    finally { setLatestLoading(false); }
  }

  function useLatestVersion() {
    if (!latest) return;
    // Keep the local form. The user explicitly chooses the version to replace.
    detailRef.current = latest; conflictRef.current = false; setDetail(latest); setBaseline(formFrom(latest,timezone)); setConflict(false); setLatest(null); setSaveError("");
  }

  return (
    <PermissionGuard permissionCode={ARTICLE_EDIT_PERMISSION} fallback={<EmptyState title="无访问权限" description="请联系管理员授予内容编辑权限。" />}>
      <PageHeader title={isNew ? "新建文章" : "编辑文章"} description={`自动保存工作稿；手动保存保留修订。展示日期使用 ${timezone}。`} actions={<Button onClick={() => navigate("/content/articles")}>返回列表</Button>} />
      {loadError ? <EmptyState title="加载失败" description={loadError} /> : (
        <form className="max-w-[1200px] space-y-space-6" onSubmit={(event) => void save(event)}>
          <FormSection title="文章信息">
            <Field label="标题" htmlFor="article-title" help="可以保存尚未完成的标题。"><Input id="article-title" value={values.title} maxLength={500} disabled={readOnly} onChange={(event) => change("title", event.target.value)} /></Field>
            <Field label="Slug" htmlFor="article-slug" help="中文和数字可用；首次成功发布后固定。"><Input id="article-slug" value={values.slug} maxLength={200} disabled={readOnly || !!detail?.slugLockedAt} onChange={(event) => { setSlugEdited(true); change("slug", event.target.value); }} /></Field>
            <Field label="展示日期" htmlFor="article-date" help="可留空，发布时确定默认日期。"><Input id="article-date" type="datetime-local" step="1" value={values.displayDate} disabled={readOnly} onChange={(event) => change("displayDate", event.target.value)} /></Field>
            <div className="md:col-span-2"><Field label="摘要" htmlFor="article-summary"><Textarea id="article-summary" value={values.summary} maxLength={5000} disabled={readOnly} onChange={(event) => change("summary", event.target.value)} /></Field></div>
          </FormSection>
          <TaxonomyPicker categoryIds={values.categoryIds} tagIds={values.tagIds} disabled={readOnly} onChange={(kind, ids) => change(kind === "category" ? "categoryIds" : "tagIds", ids)} />
          <ContentCard title="媒体与封面"><MediaSelector disabled={readOnly} onSelect={(file) => change("markdown", `${values.markdown}\n${file.image ? "!" : ""}[${file.originalName.split("[").join("").split("]").join("").split("\\").join("")}](${file.stablePath})\n`)} onCover={(file) => change("coverMediaId", file?.id ?? null)} coverMediaId={values.coverMediaId} /></ContentCard>
          <ContentCard title="Markdown 正文" description="源码编辑与即时预览；支持表格、列表及围栏代码。截图后在输入框按 Ctrl+V（Mac 使用 ⌘V）上传图片。">
            {uploadingImage && <p role="status" className="mb-space-3 text-sm text-text-secondary">正在上传图片，请稍候…</p>}
            {imageError && <p role="alert" className="mb-space-3 text-sm text-error">{imageError}</p>}
            <div className="grid min-w-0 gap-space-4 lg:grid-cols-2">
              <div className="min-w-0" data-color-mode="light"><MDEditor value={values.markdown} onChange={(value) => { if (!readOnly) change("markdown", value ?? ""); }} height={480} preview="edit" extraCommands={[]} defaultTabEnable textareaProps={{ id: "article-markdown", "aria-label": "Markdown 正文", disabled: readOnly, spellCheck: false, onPaste: (event) => { void pasteImages(event); } }} /></div>
              <section className="article-markdown-preview min-w-0 overflow-auto rounded-control border border-border p-space-4" aria-label="正文即时预览">
                <ReactMarkdown remarkPlugins={[remarkGfm]} components={{ img: MarkdownMediaImage, a: MarkdownMediaLink }}>{preview}</ReactMarkdown>
              </section>
            </div>
          </ContentCard>
          {conflict && <ContentCard title="工作稿保存冲突" description="本地内容已保留。读取最新版本后，确认是否用本地内容替换它。"><Button disabled={latestLoading} onClick={() => void loadLatest()}>{latestLoading ? "读取中…" : "读取最新版本"}</Button>{latest && <div className="mt-space-4 space-y-space-3"><p>服务器版本 {latest.draft.version} · {latest.draft.title || "未命名文章"}</p><pre className="max-h-64 overflow-auto whitespace-pre-wrap rounded-control bg-neutral-background p-space-3">{latest.draft.markdown}</pre><Button onClick={useLatestVersion}>保留本地内容，以此版本继续编辑</Button></div>}</ContentCard>}
          <div className="flex flex-wrap items-center gap-space-3 border-t border-border pt-space-4">
            <Button variant="primary" type="submit" disabled={readOnly || conflict || !!publishRequest.current}>{saving ? "保存中…" : "手动保存"}</Button>
            {!isNew && detail?.lifecycle === "active" && <><Button disabled={readOnly || conflict || !!publishRequest.current} onClick={() => void publish("preview")}>Astro 预览</Button><PermissionGuard permissionCode="content:publish"><Button disabled={readOnly || conflict || !!publishRequest.current} onClick={() => setConfirmation("article")}>发布文章</Button>{detail.published && <Button disabled={readOnly || conflict || !!publishRequest.current} onClick={() => setConfirmation("unpublish")}>下线文章</Button>}</PermissionGuard>{!detail.published && <Button disabled={readOnly || conflict || dirty || !!publishRequest.current} onClick={() => setConfirmation("archive")}>归档</Button>}</>}
            {!isNew && detail?.lifecycle === "archived" && <Button disabled={saving || conflict} onClick={() => setConfirmation("activate")}>恢复编辑</Button>}
            <span className="text-sm text-text-secondary" role="status">{autosaving ? "自动保存中…" : loading ? "加载中…" : dirty ? "有未保存修改" : savedMessage || (detail ? `工作稿版本 ${detail.draft.version}` : "尚未保存")}</span>
            {saveError && !conflict && <Button onClick={() => void persist("manual")}>重试保存</Button>}
            {saveError && <p className="w-full text-sm text-error" role="alert">{saveError}</p>}
          </div>
          {publishError && <ContentCard title="提交失败"><p role="alert" className="text-error">{publishError}</p>{publishRequest.current && <Button disabled={saving} onClick={() => void publish(publishRequest.current!.input.kind)}>重试同一请求</Button>}</ContentCard>}
          {taskId && <PublicationTask key={taskId} id={taskId} onSuccess={() => { void getArticle(Number(id)).then((result) => { detailRef.current = result;setDetail(result); }); }} />}
          {!isNew && detail && <RevisionHistory articleId={detail.id} version={detail.draft.version} disabled={readOnly || conflict} dirty={dirty} onRestore={restore} />}
        </form>
      )}
      <ConfirmDialog open={confirmation !== null} title={confirmation === "article" ? "发布当前文章？" : confirmation === "unpublish" ? "下线文章？" : confirmation === "archive" ? "归档文章？" : "恢复文章编辑？"} description={confirmation === "article" ? "提交时固定当前工作稿。其他文章和站点配置保持线上版本。" : confirmation === "unpublish" ? "完成后公开站点移除此文章及无文章的聚合入口。" : confirmation === "archive" ? "已下线且无在途任务的文章可以归档。" : "恢复工作稿编辑，不自动上线。"} loading={saving} onCancel={() => setConfirmation(null)} onConfirm={() => { if (confirmation === "article" || confirmation === "unpublish") void publish(confirmation); else void lifecycle(confirmation === "archive" ? "archived" : "active"); }} />
      <ConfirmDialog open={blocker.state === "blocked"} title="离开文章编辑？" description="尚未保存的本地修改将丢失。" confirmText="离开" cancelText="继续编辑" onConfirm={() => blocker.state === "blocked" && blocker.proceed()} onCancel={() => blocker.state === "blocked" && blocker.reset()} />
    </PermissionGuard>
  );
}
