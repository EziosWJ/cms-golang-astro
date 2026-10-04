import { readFile } from 'node:fs/promises';
import { createMarkdownProcessor } from '@astrojs/markdown-remark';

export type Taxonomy = { id: number; kind: 'category' | 'tag'; name: string; url: string };
export type Revision = { articleId: number; title: string; slug: string; markdown: string; summary: string; displayDate: string | null; coverMediaId: number | null; taxonomy: Taxonomy[] };
export type Article = { revision: Revision; firstPublishedAt: string; updatedAt: string };
export type Config = { siteName: string; description: string; publicUrl: string; language: string; timezone: string; authorName: string; authorBio: string; avatarMediaId: number | null };
export type Snapshot = { config: { data: Config }; articles: Article[]; media: { file: { id: number; originalName: string; mimeType: string }; path: string }[] };
const defaultSnapshot: Snapshot = { config: { data: { siteName: 'CMS 博客', description: '', publicUrl: 'http://localhost:8080', language: 'zh-CN', timezone: 'Asia/Shanghai', authorName: '作者', authorBio: '', avatarMediaId: null } }, articles: [], media: [] };
const inputPath = process.env.CMS_INPUT_PATH;
export const snapshot: Snapshot = inputPath ? JSON.parse(await readFile(inputPath, 'utf8')) : defaultSnapshot;
export const config = snapshot.config.data;
export const articles = [...snapshot.articles].sort((a, b) => new Date(b.revision.displayDate ?? b.firstPublishedAt).getTime() - new Date(a.revision.displayDate ?? a.firstPublishedAt).getTime());
export function withBase(path: string) { return `${import.meta.env.BASE_URL.replace(/\/$/, '')}/${path.replace(/^\//, '')}`; }
export function articleURL(article: Article) { return withBase(`/archives/${encodeURIComponent(article.revision.slug)}/`); }
export function taxonomyURL(term: Taxonomy) { return withBase(`/${term.kind === 'category' ? 'categories' : 'tags'}/${encodeURIComponent(term.url)}/`); }
export function mediaURL(id: number | null) { if (!id) return ''; const resource = snapshot.media.find((item) => item.file.id === id); if (!resource) throw new Error(`Manifest is missing media ${id}`); return withBase(resource.path); }
export function terms(kind: Taxonomy['kind']) { const map = new Map<number, Taxonomy>(); for (const article of articles) for (const term of article.revision.taxonomy ?? []) if (term.kind === kind) map.set(term.id, term); return [...map.values()].sort((a, b) => a.name.localeCompare(b.name)); }
export function formatDate(value: string | null) { return value ? new Intl.DateTimeFormat(config.language, { timeZone: config.timezone, dateStyle: 'medium' }).format(new Date(value)) : ''; }
// Escape HTML nodes before rehype processing; raw HTML and editor directives stay text.
function escapeRawHtml() { return (tree: { type: string; children?: typeof tree[] }) => { function walk(node: typeof tree) { if (node.type === 'html') node.type = 'text'; node.children?.forEach(walk); } walk(tree); }; }
const processor = createMarkdownProcessor({ gfm: true, smartypants: false, syntaxHighlight: 'shiki', shikiConfig: { transformers: [{ name: 'cms-language-label', pre(node) { node.properties['data-language'] = this.options.lang ?? 'text'; } }] }, remarkPlugins: [escapeRawHtml] });
export async function renderMarkdown(markdown: string) { const result = await (await processor).render(markdown); return result.code.replace(/(href|src)="(\/[^\"]*)"/g, (_match, attr, path) => `${attr}="${withBase(path)}"`); }
