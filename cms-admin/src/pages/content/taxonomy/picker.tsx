import { useEffect, useState } from "react";
import { getTaxonomy, writeTaxonomy, type TaxonomyTerm } from "@/api/taxonomy";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { hasPermission } from "@/lib/permission";
import { FormSection } from "@/components/common/form-section";
import { getErrorMessage } from "@/lib/api-error";
export function TaxonomyPicker({ categoryIds, tagIds, disabled, onChange }: { categoryIds: number[]; tagIds: number[]; disabled: boolean; onChange: (kind: "category" | "tag", ids: number[]) => void }) {
  const [categories, setCategories] = useState<TaxonomyTerm[]>([]);
  const [tags, setTags] = useState<TaxonomyTerm[]>([]);
  const [search, setSearch] = useState("");
 const [newTerm, setNewTerm] = useState<{ kind:"category" | "tag"; name:string; url:string } | null>(null);
 const [creating, setCreating] = useState(false);
  const [error, setError] = useState("");
  useEffect(() => {
    let active = true;
    Promise.all([getTaxonomy("category"), getTaxonomy("tag")]).then(([a, b]) => { if (active) { setCategories(a); setTags(b); } }).catch((err: unknown) => { if (active) setError(getErrorMessage(err, "加载分类标签失败")); });
    return () => { active = false; };
  }, []);
  return <FormSection columns={1} title="分类与标签" description="可多选或全部清空。">
    <Input aria-label="搜索分类标签" placeholder="搜索分类或标签" value={search} onChange={(event) => setSearch(event.target.value)} />
 {newTerm && <div className="space-y-space-2 col-span-1"><Input aria-label="新分类标签名称" placeholder="名称" value={newTerm.name} onChange={(event) => setNewTerm({ ...newTerm, name:event.target.value })} /><Input aria-label="新分类标签地址" placeholder="地址" value={newTerm.url} onChange={(event) => setNewTerm({ ...newTerm, url:event.target.value })} /><Button disabled={creating || disabled} onClick={() => { setCreating(true); void writeTaxonomy(newTerm.kind, { name:newTerm.name, url:newTerm.url }).then((term) => { (newTerm.kind === "category" ? setCategories : setTags)((old) => [...old,term]); onChange(newTerm.kind, [...(newTerm.kind === "category" ? categoryIds : tagIds),term.id]); setNewTerm(null); setError(""); }).catch((err:unknown) => setError(getErrorMessage(err,"创建失败，文章输入保留"))).finally(() => setCreating(false)); }}>创建并选中</Button><Button disabled={creating} onClick={() => setNewTerm(null)}>取消</Button></div>}
    {error && <p role="alert" className="text-error col-span-1">{error}</p>}
    {([{ kind: "category", label: "分类", terms: categories, ids: categoryIds }, { kind: "tag", label: "标签", terms: tags, ids: tagIds }] as const).map(({ kind, label, terms, ids }) => <fieldset key={kind} disabled={disabled} className="space-y-space-2"><legend className="mb-space-2 font-medium">{label}</legend>{terms.length === 0 && <p className="text-text-tertiary">暂无{label}</p>}<div className="flex flex-wrap gap-space-3">{terms.filter((term) => term.name.includes(search)).map((term) => <label key={term.id} className="flex items-center gap-space-2"><input type="checkbox" checked={ids.includes(term.id)} onChange={(event) => onChange(kind, event.target.checked ? [...ids, term.id] : ids.filter((id) => id !== term.id))} />{term.name}</label>)}</div><Button size="sm" disabled={disabled} onClick={() => onChange(kind, [])}>清空选择</Button>{hasPermission("content:taxonomy:edit") && <Button size="sm" disabled={disabled} onClick={() => setNewTerm({ kind, name:"", url:"" })}>新建{label}</Button>}</fieldset>)}
  </FormSection>;
}
