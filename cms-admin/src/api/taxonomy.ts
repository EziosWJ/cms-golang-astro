import { http } from "@/lib/http";
import type { TaxonomySnapshot } from "@/types/content";
export type TaxonomyTerm = TaxonomySnapshot & { version: number; lockedAt: string | null };
function path(kind: "category" | "tag") { return `/api/v1/${kind === "category" ? "categories" : "tags"}`; }
export function getTaxonomy(kind: "category" | "tag") { return http.get<TaxonomyTerm[]>(path(kind)); }
export function writeTaxonomy(kind: "category" | "tag", input: { name: string; url: string; expectedVersion?: number }, id?: number) { return id ? http.put<TaxonomyTerm>(`${path(kind)}/${id}`, input) : http.post<TaxonomyTerm>(path(kind), input); }
export function deleteTaxonomy(kind: "category" | "tag", id: number) { return http.delete<void>(`${path(kind)}/${id}`); }
