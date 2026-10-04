import { useEffect, useState } from "react";
import { getArticleRevision, getArticleRevisions, type ArticleRevision } from "@/api/content";
import { ContentCard } from "@/components/common/content-card";
import { Pagination } from "@/components/common/pagination";
import { ConfirmDialog } from "@/components/common/confirm-dialog";
import { Button } from "@/components/ui/button";
import { getErrorMessage } from "@/lib/api-error";

export function RevisionHistory({ articleId, version, disabled, dirty, onRestore }: { articleId: number; version: number; disabled: boolean; dirty: boolean; onRestore: (id: number) => Promise<void> }) {
  const [page, setPage] = useState(1);
  const [total, setTotal] = useState(0);
  const [records, setRecords] = useState<Pick<ArticleRevision, "id" | "title" | "version" | "createdAt" | "createdBy">[]>([]);
  const [selected, setSelected] = useState<ArticleRevision | null>(null);
  const [error, setError] = useState("");
  const [confirm, setConfirm] = useState(false);
  useEffect(() => {
    let active = true;
    getArticleRevisions(articleId, page).then((result) => { if (active) { setRecords(result.records); setTotal(result.total); setError(""); } }).catch((err: unknown) => { if (active) setError(getErrorMessage(err, "加载修订失败")); });
    return () => { active = false; };
  }, [articleId, page, version]);
  async function select(id: number) {
    try { setSelected(await getArticleRevision(articleId, id)); setError(""); }
    catch (err) { setError(getErrorMessage(err, "读取修订失败")); }
  }
  return <ContentCard title="手动修订历史" description="自动保存不会新增修订。恢复只改变工作稿。">
    {error && <p role="alert" className="text-error">{error}</p>}
    <div className="flex flex-wrap gap-space-2">{records.map((record) => <Button key={record.id} size="sm" onClick={() => void select(record.id)}>#{record.id} · 版本 {record.version} · {new Date(record.createdAt).toLocaleString("zh-CN")}</Button>)}</div>
    <Pagination page={page} pageSize={20} total={total} onPageChange={setPage} />
    {selected && <div className="mt-space-4 space-y-space-3"><p>{selected.title || "未命名文章"} · Slug：{selected.slug}</p><p>摘要：{selected.summary || "无"} · 展示日期：{selected.displayDate || "未设置"}</p><pre className="max-h-80 overflow-auto whitespace-pre-wrap rounded-control bg-neutral-background p-space-3">{selected.markdown}</pre><Button disabled={disabled} onClick={() => setConfirm(true)}>恢复到工作稿</Button></div>}
    <ConfirmDialog open={confirm} title="恢复所选修订？" description={dirty ? "当前未保存内容会被所选修订替换，线上版本保持不变。" : "所选修订会写入工作稿，线上版本保持不变。"} onCancel={() => setConfirm(false)} onConfirm={() => { setConfirm(false); if (selected) void onRestore(selected.id); }} />
  </ContentCard>;
}
