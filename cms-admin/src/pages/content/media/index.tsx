import { Link } from "react-router-dom";
import { hasPermission } from "@/lib/permission";
import { useEffect, useState } from "react";
import { deleteMedia, getMediaPage, getMediaReferences, type MediaRecord, type MediaReference } from "@/api/media";
import { MediaUpload } from "./upload";
import { MarkdownMediaImage } from "./preview";
import { PermissionGuard } from "@/components/auth/permission-guard";
import { PageHeader } from "@/components/common/page-header";
import { ContentCard } from "@/components/common/content-card";
import { DataTable } from "@/components/common/data-table";
import { DataTableCard } from "@/components/common/data-table-card";
import { Pagination } from "@/components/common/pagination";
import { ConfirmDialog } from "@/components/common/confirm-dialog";
import { EmptyState } from "@/components/common/empty-state";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { getErrorMessage } from "@/lib/api-error";
import type { DataTableColumn } from "@/types/table";
const ownerLabels:Record<string,string>={draft:"文章工作稿",revision:"历史版本",config:"工作配置",config_revision:"配置历史",task:"发布任务",preview:"私有预览",release:"保留发布产物", attempt:"构建尝试 / 预览产物"};
export function MediaPage() {
 const [records,setRecords]=useState<MediaRecord[]>([]); const [total,setTotal]=useState(0); const [page,setPage]=useState(1); const [refresh,setRefresh]=useState(0); const [name,setName]=useState(""); const [error,setError]=useState(""); const [loading,setLoading]=useState(true); const [removing,setRemoving]=useState<MediaRecord | null>(null); const [dimensions,setDimensions]=useState(""); const [selected,setSelected]=useState<MediaRecord | null>(null); const [references,setReferences]=useState<MediaReference[] | null>(null);
 useEffect(() => { let active=true; setLoading(true); getMediaPage(page,name).then((result) => { if(active) { setRecords(result.records); setTotal(result.total); setError(""); } }).catch((err:unknown) => { if(active)setError(getErrorMessage(err,"加载失败")); }).finally(() => { if(active)setLoading(false); }); return () => { active=false; }; },[page,refresh,name]);
 async function detail(file:MediaRecord) { setSelected(file); setDimensions(""); setReferences(null); try { setReferences(await getMediaReferences(file.id)); } catch(err) { setError(getErrorMessage(err,"引用查询失败；不能据此判断资源没有引用")); } }
 const columns:DataTableColumn<MediaRecord>[]=[{title:"预览",key:"image",render:(_,file) => file.image ? <MarkdownMediaImage src={file.stablePath} alt={file.originalName} className="h-12 w-12 object-contain" /> : "附件"},{title:"文件名",key:"name",render:(_,file) => <Button variant="ghost" onClick={() => void detail(file)}>{file.originalName}</Button>},{title:"类型",dataIndex:"mimeType"},{title:"大小",key:"size",render:(_,file) => `${(file.fileSize/1024).toFixed(1)} KB`},{title:"操作",key:"delete",render:(_,file) => <PermissionGuard permissionCode="content:media:edit"><Button variant="ghost" onClick={() => setRemoving(file)}>删除</Button></PermissionGuard>}];
 return <PermissionGuard permissionCode="content:media:edit" fallback={<EmptyState title="无访问权限" />}><PageHeader title="媒体库" description="源文件保持私有；工作稿、历史、配置、任务、预览与保留产物的引用由服务器保护。" />
 <ContentCard title="上传媒体"><MediaUpload onUploaded={() => setRefresh((n) => n+1)} /></ContentCard>
 <div className="my-space-4 flex gap-space-3"><Input aria-label="搜索文件名" placeholder="搜索文件名" value={name} onChange={(event) => { setName(event.target.value); setPage(1); }} /><Button onClick={() => setRefresh((n) => n+1)}>刷新 / 重试</Button></div>
 <DataTableCard pagination={<Pagination page={page} pageSize={20} total={total} onPageChange={setPage} />}><DataTable columns={columns} dataSource={records} rowKey="id" loading={loading} error={error} empty={<EmptyState title="暂无媒体" description="上传图片或附件后即可在文章中选择。" />} /></DataTableCard>
 {selected && <ContentCard className="mt-space-4" title={selected.originalName} extra={<Button onClick={() => setSelected(null)}>关闭详情</Button>}><p>{selected.mimeType} · {selected.fileSize} 字节{dimensions && ` · ${dimensions}`}</p>{selected.image && <MarkdownMediaImage onLoad={(event) => setDimensions(`${event.currentTarget.naturalWidth} × ${event.currentTarget.naturalHeight} 像素`)} src={selected.stablePath} alt={selected.originalName} className="max-h-64 max-w-full object-contain" />}<p className="break-all">稳定路径：{selected.stablePath}</p><h3 className="mt-space-3 font-medium">引用来源</h3>{references===null ? <p>正在查询引用；加载失败时请重试。</p> : references.length ? references.map((ref) => <p key={`${ref.ownerType}:${ref.ownerId}`}>{ownerLabels[ref.ownerType] ?? ref.ownerType} · {ref.articleId && hasPermission("content:article:edit") ? <Link to={`/content/articles/${ref.articleId}`} className="text-primary">{ref.title || "未命名文章"}</Link> : `#${ref.ownerId}`}</p>) : <p>当前未记录引用；删除时服务器仍会重新检查。</p>}<Button onClick={() => void detail(selected)}>重试引用查询</Button></ContentCard>}
 <ConfirmDialog open={!!removing} title="删除媒体？" description={`删除 ${removing?.originalName ?? ""} 的私有源文件。任何工作稿、历史版本、配置、在途任务、预览或保留产物引用都会阻止删除。`} danger onCancel={() => setRemoving(null)} onConfirm={() => { if(!removing)return; void deleteMedia(removing.id).then(() => { setRefresh((n) => n+1); setSelected(null); }).catch((err:unknown) => setError(getErrorMessage(err,"删除失败"))).finally(() => setRemoving(null)); }} /></PermissionGuard>;
}
