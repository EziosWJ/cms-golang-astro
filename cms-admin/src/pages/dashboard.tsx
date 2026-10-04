import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { getArticlePage } from "@/api/content";
import { getPublications, publicationKind, publicationStatus, type PublishTask } from "@/api/publishing";
import { getSiteConfig, type WorkingSiteConfig } from "@/api/site-config";
import { PageHeader } from "@/components/common/page-header";
import { ContentCard } from "@/components/common/content-card";
import { Button } from "@/components/ui/button";
import { hasPermission } from "@/lib/permission";
import { getErrorMessage } from "@/lib/api-error";
import { useAuthStore } from "@/store/auth-store";
import type { ArticleListRecord } from "@/types/content";
export function DashboardPage() {
 const user=useAuthStore((state) => state.user);
 const menus=useAuthStore((state) => state.menus);
 const [articles,setArticles]=useState<ArticleListRecord[]>([]); const [changed,setChanged]=useState<ArticleListRecord[]>([]); const [tasks,setTasks]=useState<PublishTask[]>([]); const [config,setConfig]=useState<WorkingSiteConfig | null>(null); const [error,setError]=useState(""); const [refresh,setRefresh]=useState(0); const [loading,setLoading]=useState(true);
 useEffect(() => { let active=true; setLoading(true); setError("");
 const work: Promise<void>[]=[];
 if (hasPermission("content:article:edit")) { work.push(getArticlePage({page:1,pageSize:5,title:""}).then((v) => { if(active)setArticles(v.records); })); work.push(getArticlePage({page:1,pageSize:5,title:"",status:"changed"}).then((v) => { if(active)setChanged(v.records); })); }
 if (hasPermission("content:publish") || hasPermission("content:config:publish")) work.push(getPublications({page:1,pageSize:5}).then((v) => { if(active)setTasks(v.records); }));
 if(hasPermission("content:config:edit")) work.push(getSiteConfig().then((v) => { if(active)setConfig(v); }));
 void Promise.allSettled(work).then((results) => { if(!active)return; setError(results.filter((r) => r.status==="rejected").map((r) => getErrorMessage((r as PromiseRejectedResult).reason,"加载失败")).join("；")); setLoading(false); }); return () => { active=false; }; },[user,menus,refresh]);
 const articleList=(records:ArticleListRecord[]) => records.length ? records.map((a) => <p key={a.id} className="flex flex-wrap justify-between gap-space-2 border-b border-border py-space-2"><Link className="text-primary" to={`/content/articles/${a.id}`}>{a.title || "未命名文章"}</Link><span className="text-sm text-text-secondary">{new Date(a.savedAt).toLocaleString("zh-CN")}</span></p>) : <p>暂无文章。可以先开始写作。</p>;
 return <><PageHeader title={`欢迎回来，${user?.nickname || user?.username || "作者"}`} description="保存与上线分开，继续写作或查看最近发布结果。" actions={<Button onClick={() => setRefresh((n) => n+1)}>刷新</Button>} />
 {error && <p role="alert" className="text-error">{error}</p>}{loading && <p role="status">加载中…</p>}
 <div className="mb-space-4 flex flex-wrap gap-space-3">{hasPermission("content:article:edit") && <Link className="text-primary" to="/content/articles/new">新建文章</Link>}{config && <Link className="text-primary" to="/content/site-config">首次设置：{[config.data.siteName,config.data.publicUrl,config.data.authorName].every(Boolean) ? "资料已填写，检查上线配置" : "补齐站点名称、公开地址和作者资料"}</Link>}{config?.published && <a href={config.published.data.publicUrl} className="text-primary" target="_blank" rel="noreferrer">查看网站</a>}</div>
 <div className="grid gap-space-6 lg:grid-cols-2">{hasPermission("content:article:edit") && <><ContentCard title="最近编辑">{articleList(articles)}</ContentCard><ContentCard title="有未发布修改">{articleList(changed)}</ContentCard></>}{(hasPermission("content:publish") || hasPermission("content:config:publish")) && <ContentCard title="最近发布结果">{tasks.length ? tasks.map((task) => <p key={task.id} className="border-b border-border py-space-2"><Link className="text-primary" to="/content/publications">{task.title || publicationKind[task.kind]} · {publicationStatus[task.status]}</Link> · {new Date(task.createdAt).toLocaleString("zh-CN")}</p>) : <p>暂无发布记录。</p>}</ContentCard>}</div></>;
}
