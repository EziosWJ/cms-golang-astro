// Parse and transform semantic Markdown nodes; fenced examples are untouched.
import { unified } from 'unified';
import remarkParse from 'remark-parse';
import remarkGfm from 'remark-gfm';
import remarkStringify from 'remark-stringify';
import path from 'node:path';
let input = '';for await (const chunk of process.stdin) input += chunk;
const { markdown, sourcePath, articles } = JSON.parse(input);
const parser = unified().use(remarkParse).use(remarkGfm).use(remarkStringify);
const tree = parser.parse(markdown);const media = new Set();const warnings = [];
function visit(node) {
 if (node.type === 'html') warnings.push('原始 HTML 需要人工确认，默认渲染为文本');
 if ((node.type === 'link' || node.type === 'image' || node.type === 'definition') && node.url) {
  const value = node.url;
  if (/^https?:\/\//i.test(value) || value.startsWith('#')) return;
  if (/^[a-z][a-z0-9+.-]*:/i.test(value) || value.startsWith('//')) { warnings.push(`不支持的链接 ${value}`); return; }
  let decoded;try { decoded = decodeURI(value); } catch { warnings.push(`无效编码 ${value}`); return; }
  if (/^\/(images|upload)\//.test(decoded)) { media.add(decoded.split(/[?#]/)[0]); return; }
  if (!decoded.startsWith('/')) {
   const [address,fragment] = decoded.split('#');const target = path.posix.normalize(path.posix.join(path.posix.dirname(sourcePath), address));
   const slug = articles[target] ?? articles[`${target}.md`];
   if (slug) node.url = `/archives/${encodeURIComponent(slug)}/${fragment ? `#${fragment}` : ''}`;
   else warnings.push(`无法解析相对链接 ${value}`);
  } else if (!/^\/(archives|categories|tags|media)\//.test(decoded)) warnings.push(`无法解析站内链接 ${value}`);
 }
 node.children?.forEach(visit);
}
visit(tree);
process.stdout.write(JSON.stringify({ markdown: parser.stringify(tree), media: [...media], warnings }));
