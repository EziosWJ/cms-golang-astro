import { useAuthStore } from "@/store/auth-store";
import { useEffect, useState } from "react";
import { detectGitConnection, disconnectGitConnection, getGitConnection, getGitRefs, saveGitConnection, type GitConnection, type GitConnectionInput, type GitRepository } from "@/api/git-export";
import { PermissionGuard } from "@/components/auth/permission-guard";
import { ContentCard } from "@/components/common/content-card";
import { EmptyState } from "@/components/common/empty-state";
import { Field } from "@/components/common/field";
import { FormSection } from "@/components/common/form-section";
import { PageHeader } from "@/components/common/page-header";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Select } from "@/components/ui/select";
import { hasPermission } from "@/lib/permission";

const initial: GitConnectionInput = { token: "", repository: "", branch: "main", sourceRepository: "EziosWJ/cms-golang-astro", sourceRef: "main" };
export function GitConnectionPage() {
  useAuthStore((state) => state.menus);
  const [values, setValues] = useState(initial);
  const [connection, setConnection] = useState<GitConnection | null>(null);
  const [repos, setRepos] = useState<GitRepository[]>([]);
  const [branches, setBranches] = useState<string[]>([]);
  const [sourceRefs, setSourceRefs] = useState<string[]>([]);
  const [busy, setBusy] = useState(false);
  const [loading, setLoading] = useState(true);
  const [message, setMessage] = useState("");
  const allowed = hasPermission("integration:git:manage");
  useEffect(() => {
    if (!allowed) return;
    let active = true;
    getGitConnection().then((v) => { if (active) { setConnection(v); setValues({ ...initial, repository: v.repository || "", branch: v.branch || "main", sourceRepository: v.sourceRepository || initial.sourceRepository, sourceRef: v.sourceRef || "main" }); } }).catch((e: unknown) => { if (active) setMessage(e instanceof Error ? e.message : "连接读取失败"); }).finally(() => { if (active) setLoading(false); });
    return () => { active = false; };
  }, [allowed]);
  const change = (key: keyof GitConnectionInput, value: string) => { setValues((v) => ({ ...v, [key]: value })); if (key === "repository") setBranches([]); if (key === "sourceRepository") setSourceRefs([]); };
  const run = async (action: () => Promise<void>) => { if (busy) return; setBusy(true); setMessage(""); try { await action(); } catch (e) { setMessage(e instanceof Error ? e.message : "Git 操作失败"); } finally { setBusy(false); } };
  return <PermissionGuard permissionCode="integration:git:manage" fallback={<EmptyState title="无访问权限" />}>
    <PageHeader title="GitHub 连接" description="在页面配置手动 Git 推送。现有本地发布继续独立运行。" />
    <form className="max-w-5xl space-y-space-4" onSubmit={(e) => { e.preventDefault(); void run(async () => { const saved = await saveGitConnection(values); setConnection(saved); setValues((v) => ({ ...v, token: "" })); setMessage("连接已保存，Token 不会回显。"); }); }}>
      <fieldset disabled={busy || loading} className="space-y-space-4">
        <FormSection title="授权与输入仓库" description="先在 GitHub 创建公开仓库。fine-grained PAT 限定该仓库，授予 Contents 读写权限。" columns={1}>
          <Field label="GitHub Token" htmlFor="git-token" required={!connection?.hasCredential} help={connection?.hasCredential ? "留空保留已有凭据；检测通过只证明连接和仓库访问，实际推送仍受 Token 权限与分支保护限制。" : "Token 仅用于服务器访问 GitHub，不会保存在浏览器存储。"}><Input id="git-token" type="password" autoComplete="off" value={values.token} onChange={(e) => change("token", e.target.value)} required={!connection?.hasCredential} maxLength={4096} /></Field>
          {connection?.credentialError && <p role="alert" className="text-error">凭据密钥不可用，请先断开连接，再填写新 Token 重新连接。</p>}
          <Button type="button" onClick={() => void run(async () => { const detected = await detectGitConnection(values.token); setRepos(detected.repositories); setMessage(`已连接 ${detected.login}，发现 ${detected.repositories.length} 个可选公开仓库。`); })}>检测连接并读取仓库</Button>
          <Field label="公开输入仓库" htmlFor="git-repo" required help="仅推送已发布内容及其附件。空仓库使用默认分支完成首次推送。"><Select id="git-repo" value={values.repository} required onChange={(e) => { const repo = repos.find((r) => r.full_name === e.target.value); setValues((v) => ({ ...v, repository: e.target.value, branch: repo?.default_branch || "main" })); setBranches([]); }}><option value="">请检测连接后选择仓库</option>{values.repository && !repos.some((r) => r.full_name === values.repository) && <option value={values.repository}>{values.repository}</option>}{repos.map((r) => <option key={r.full_name} value={r.full_name}>{r.full_name}</option>)}</Select></Field>
          <div className="flex flex-wrap gap-space-3"><Button type="button" disabled={!values.repository} onClick={() => void run(async () => { const refs = await getGitRefs(values.repository, "branches", values.token); setBranches(refs.map((r) => r.name)); setMessage(refs.length ? "目标分支已读取。" : "仓库为空，将创建填写的默认分支。"); })}>读取目标分支</Button></div>
          <Field label="目标分支" htmlFor="git-branch" required><Input id="git-branch" list="git-branches" value={values.branch} onChange={(e) => change("branch", e.target.value)} required /><datalist id="git-branches">{branches.map((b) => <option key={b} value={b} />)}</datalist></Field>
        </FormSection>
        <FormSection title="Astro 源码版本" description="每次推送将分支或标签解析为固定提交 SHA；未提交的本地模板改动不会进入 Actions。" columns={1}>
          <Field label="公开源码仓库" htmlFor="git-source" required><Input id="git-source" value={values.sourceRepository} onChange={(e) => change("sourceRepository", e.target.value)} required /></Field>
          <Button type="button" disabled={!values.sourceRepository} onClick={() => void run(async () => { const branches = await getGitRefs(values.sourceRepository, "branches", values.token, true); const tags = await getGitRefs(values.sourceRepository, "tags", values.token, true); setSourceRefs([...new Set([...branches, ...tags].map((r) => r.name))]); setMessage("源码分支和标签已读取。"); })}>读取源码分支与标签</Button>
          <Field label="源码分支或标签" htmlFor="git-source-ref" required><Input id="git-source-ref" list="git-source-refs" value={values.sourceRef} onChange={(e) => change("sourceRef", e.target.value)} required /><datalist id="git-source-refs">{sourceRefs.map((r) => <option key={r} value={r} />)}</datalist></Field>
        </FormSection>
        <div className="flex flex-wrap gap-space-3 border-t border-border pt-space-4"><Button variant="primary" type="submit">{busy ? "处理中…" : "保存连接"}</Button><Button type="button" disabled={!connection?.hasCredential} onClick={() => void run(async () => { if (!window.confirm("断开 GitHub 连接并移除保存的凭据？")) return; await disconnectGitConnection(); setConnection(null); setValues(initial); setRepos([]); setBranches([]); setMessage("已断开连接。"); })}>断开连接</Button></div>
      </fieldset>
      <p role="status" className="text-sm text-text-secondary">{loading ? "加载连接…" : message || (connection?.hasCredential ? `已保存 ${connection.login} 的连接。` : "尚未配置连接。")}</p>
    </form>
    <ContentCard title="Actions 构建准备" className="mt-space-4 max-w-5xl"><p className="text-sm text-text-secondary">工作流需要由你安装到输入仓库，并监听所选目标分支。CMS 无需 Workflows 写权限。Git 推送成功后，构建结果在 GitHub 查看。</p><a className="mt-space-3 block text-sm text-primary" href={`https://github.com/${values.sourceRepository}/blob/${encodeURIComponent(values.sourceRef)}/templates/github-actions/build-site.yml`} target="_blank" rel="noreferrer">查看并复制 Actions 构建模板</a></ContentCard>
  </PermissionGuard>;
}
