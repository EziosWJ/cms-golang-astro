import { useEffect, useState } from "react";
import { ChevronDown, Plus, X } from "lucide-react";
import { getTaxonomy, writeTaxonomy, type TaxonomyTerm } from "@/api/taxonomy";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { hasPermission } from "@/lib/permission";
import { getErrorMessage } from "@/lib/api-error";

type TaxonomyKind = "category" | "tag";
type NewTerm = { kind: TaxonomyKind; name: string; url: string };

function termUrl(name: string) {
  return name.normalize("NFC").trim().toLowerCase().replace(/[^\p{L}\p{N}_-]+/gu, "-").replace(/^-+|-+$/g, "").slice(0, 200);
}

export function TaxonomyPicker({ categoryIds, tagIds, disabled, onChange }: { categoryIds: number[]; tagIds: number[]; disabled: boolean; onChange: (kind: TaxonomyKind, ids: number[]) => void }) {
  const [categories, setCategories] = useState<TaxonomyTerm[]>([]);
  const [tags, setTags] = useState<TaxonomyTerm[]>([]);
  const [openKind, setOpenKind] = useState<TaxonomyKind | null>(null);
  const [searches, setSearches] = useState<Record<TaxonomyKind, string>>({ category: "", tag: "" });
  const [newTerm, setNewTerm] = useState<NewTerm | null>(null);
  const [creating, setCreating] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    let active = true;
    Promise.all([getTaxonomy("category"), getTaxonomy("tag")]).then(([categoryList, tagList]) => {
      if (active) { setCategories(categoryList); setTags(tagList); }
    }).catch((err: unknown) => { if (active) setError(getErrorMessage(err, "加载分类标签失败")); });
    return () => { active = false; };
  }, []);

  function terms(kind: TaxonomyKind) { return kind === "category" ? categories : tags; }
  function ids(kind: TaxonomyKind) { return kind === "category" ? categoryIds : tagIds; }
  function label(kind: TaxonomyKind) { return kind === "category" ? "分类" : "标签"; }
  function selectedTerms(kind: TaxonomyKind) { return terms(kind).filter((term) => ids(kind).includes(term.id)); }

  function toggle(kind: TaxonomyKind, id: number) {
    const selected = ids(kind);
    onChange(kind, selected.includes(id) ? selected.filter((item) => item !== id) : [...selected, id]);
  }

  function remove(kind: TaxonomyKind, id: number) {
    onChange(kind, ids(kind).filter((item) => item !== id));
  }

  async function create(kind: TaxonomyKind, name: string, url: string) {
    const cleanName = name.trim();
    if (!cleanName || creating) return;
    setCreating(true);
    try {
      const term = await writeTaxonomy(kind, { name: cleanName, url: url.trim() || termUrl(cleanName) });
      (kind === "category" ? setCategories : setTags)((current) => [...current, term]);
      onChange(kind, [...ids(kind), term.id]);
      setNewTerm(null);
      setSearches((current) => ({ ...current, [kind]: "" }));
      setOpenKind(null);
      setError("");
    } catch (err: unknown) {
      setError(getErrorMessage(err, "创建失败，文章输入保留"));
    } finally {
      setCreating(false);
    }
  }

  function renderSelector(kind: TaxonomyKind) {
    const query = searches[kind].trim().toLowerCase();
    const list = terms(kind).filter((term) => term.name.toLowerCase().includes(query));
    const selected = selectedTerms(kind);
    const exactMatch = terms(kind).some((term) => term.name.toLowerCase() === query);

    return <div key={kind} className="taxonomy-select-field">
      <div className="taxonomy-select-label-row"><span>{label(kind)}</span><button type="button" className="taxonomy-inline-action" disabled={disabled || !hasPermission("content:taxonomy:edit")} onClick={() => { setNewTerm({ kind, name: searches[kind], url: termUrl(searches[kind]) }); setOpenKind(kind); }}>新建{label(kind)}</button></div>
      <div className={`taxonomy-select-control ${openKind === kind ? "is-open" : ""}`}>
        <div className="taxonomy-selected-values">
          {selected.length ? selected.map((term) => <span key={term.id} className="taxonomy-badge"><span>{term.name}</span><button type="button" aria-label={`移除${label(kind)} ${term.name}`} disabled={disabled} onClick={() => remove(kind, term.id)}><X className="h-3 w-3" aria-hidden /></button></span>) : <span className="taxonomy-placeholder">选择{label(kind)}</span>}
        </div>
        <button type="button" className="taxonomy-select-trigger" aria-label={`打开${label(kind)}选择器`} aria-expanded={openKind === kind} disabled={disabled} onClick={() => setOpenKind(openKind === kind ? null : kind)}><ChevronDown className="h-4 w-4" aria-hidden /></button>
      </div>
      {openKind === kind && <div className="taxonomy-select-menu">
        <Input autoFocus aria-label={`搜索${label(kind)}`} placeholder={`搜索${label(kind)}…`} value={searches[kind]} onChange={(event) => setSearches((current) => ({ ...current, [kind]: event.target.value }))} />
        <div className="taxonomy-options" role="listbox" aria-label={`${label(kind)}选项`}>
          {list.map((term) => <button type="button" role="option" aria-selected={ids(kind).includes(term.id)} className={`taxonomy-option ${ids(kind).includes(term.id) ? "is-selected" : ""}`} key={term.id} onClick={() => toggle(kind, term.id)}><span>{term.name}</span>{ids(kind).includes(term.id) && <span className="taxonomy-option-check">已选</span>}</button>)}
          {!list.length && <p className="taxonomy-empty">没有匹配的{label(kind)}</p>}
        </div>
        {searches[kind].trim() && !exactMatch && hasPermission("content:taxonomy:edit") && <button type="button" className="taxonomy-create-quick" disabled={creating} onClick={() => void create(kind, searches[kind], termUrl(searches[kind]))}><Plus className="h-4 w-4" aria-hidden />创建“{searches[kind].trim()}”</button>}
      </div>}
      {newTerm?.kind === kind && <div className="taxonomy-create-form"><p>新建{label(kind)}</p><Input aria-label={`${label(kind)}名称`} placeholder={`${label(kind)}名称`} value={newTerm.name} disabled={creating} onChange={(event) => setNewTerm({ ...newTerm, name: event.target.value })} /><Input aria-label={`${label(kind)}路径`} placeholder="URL 路径" value={newTerm.url} disabled={creating} onChange={(event) => setNewTerm({ ...newTerm, url: event.target.value })} /><div className="taxonomy-create-actions"><Button size="sm" disabled={creating || disabled || !newTerm.name.trim()} onClick={() => void create(kind, newTerm.name, newTerm.url)}>创建并选中</Button><Button size="sm" disabled={creating} onClick={() => setNewTerm(null)}>取消</Button></div></div>}
    </div>;
  }

  return <div className="taxonomy-picker">
    {renderSelector("category")}
    {renderSelector("tag")}
    {error && <p role="alert" className="text-error">{error}</p>}
  </div>;
}
