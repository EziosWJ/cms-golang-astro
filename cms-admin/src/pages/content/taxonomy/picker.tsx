import { useEffect, useState } from "react";
import { getTaxonomy, type TaxonomyTerm } from "@/api/taxonomy";
import { FormSection } from "@/components/common/form-section";
import { getErrorMessage } from "@/lib/api-error";
export function TaxonomyPicker({ categoryIds, tagIds, disabled, onChange }: { categoryIds: number[]; tagIds: number[]; disabled: boolean; onChange: (kind: "category" | "tag", ids: number[]) => void }) {
  const [categories, setCategories] = useState<TaxonomyTerm[]>([]);
  const [tags, setTags] = useState<TaxonomyTerm[]>([]);
  const [error, setError] = useState("");
  useEffect(() => {
    let active = true;
    Promise.all([getTaxonomy("category"), getTaxonomy("tag")]).then(([a, b]) => { if (active) { setCategories(a); setTags(b); } }).catch((err: unknown) => { if (active) setError(getErrorMessage(err, "加载分类标签失败")); });
    return () => { active = false; };
  }, []);
  return <FormSection title="分类与标签" description="可多选或全部清空。">
    {error && <p role="alert" className="text-error md:col-span-2">{error}</p>}
    {([{ kind: "category", label: "分类", terms: categories, ids: categoryIds }, { kind: "tag", label: "标签", terms: tags, ids: tagIds }] as const).map(({ kind, label, terms, ids }) => <fieldset key={kind} disabled={disabled} className="space-y-space-2"><legend className="mb-space-2 font-medium">{label}</legend>{terms.length === 0 && <p className="text-text-tertiary">暂无{label}</p>}<div className="flex flex-wrap gap-space-3">{terms.map((term) => <label key={term.id} className="flex items-center gap-space-2"><input type="checkbox" checked={ids.includes(term.id)} onChange={(event) => onChange(kind, event.target.checked ? [...ids, term.id] : ids.filter((id) => id !== term.id))} />{term.name}</label>)}</div></fieldset>)}
  </FormSection>;
}
