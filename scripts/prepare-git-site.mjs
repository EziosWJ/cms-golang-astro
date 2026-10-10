import { createHash } from 'node:crypto';
import { lstat, readFile, readdir, mkdir, rm, copyFile } from 'node:fs/promises';
import path from 'node:path';

const digest = (bytes) => createHash('sha256').update(bytes).digest('hex');
const safe = (root, relative) => {
  if (!relative || relative.includes('\\') || path.posix.isAbsolute(relative) || relative.split('/').some((p) => !p || p === '..' || p === '.')) throw Error('Unsafe input path');
  const target = path.resolve(root, relative);
  if (!target.startsWith(path.resolve(root) + path.sep)) throw Error('Input path escapes root');
  return target;
};
async function inventory(root, prefix = '') {
  const files = [];
  for (const entry of await readdir(root, { withFileTypes: true })) {
    const relative = prefix + entry.name;
    if (entry.isSymbolicLink()) throw Error('Symlinks are not allowed');
    if (entry.isDirectory()) files.push(...await inventory(path.join(root, entry.name), relative + '/'));
    else if (entry.isFile()) files.push(relative);
    else throw Error('Non-regular input file');
  }
  return files.sort();
}
const [inputArg, siteArg] = process.argv.slice(2);
if (inputArg === '--check-output') {
  if (!siteArg) throw Error('Static output directory required');
  const files = await inventory(path.resolve(siteArg));
  if (!files.includes('index.html') || !files.includes('404.html')) throw Error('Incomplete static output');
  for (const file of files) {
    if (file === 'manifest.json' || file === 'export.json' || file.endsWith('/.release.json') || file === '.release.json' || file.endsWith('.log') || file.endsWith('/master.key') || file === 'master.key' || file.endsWith('.env')) throw Error('Private artifact in static output');
  }
  console.log(`Verified ${files.length} public static files`);
} else {
  if (!inputArg || !siteArg) throw Error('Usage: node prepare-git-site.mjs <cms-input> <site>');
  const input = path.resolve(inputArg);
  const site = path.resolve(siteArg);
  const metadata = JSON.parse(await readFile(path.join(input, 'export.json'), 'utf8'));
  if (metadata.schemaVersion !== 1 || !metadata.files || !/^[a-f0-9]{64}$/.test(metadata.inputHash)) throw Error('Invalid export metadata');
  if (!/^[A-Za-z0-9_.-]+\/[A-Za-z0-9_.-]+$/.test(metadata.source?.repository) || !/^[a-f0-9]{40}$/.test(metadata.source?.sha)) throw Error('Invalid pinned source');
  const files = await inventory(input);
  const expected = [...Object.keys(metadata.files), 'export.json'].sort();
  if (JSON.stringify(files) !== JSON.stringify(expected)) throw Error('Export contains missing or unlisted files');
  // Go encoding/json orders map keys. Reproduce its deterministic identity bytes,
  // including default HTML escaping, while preserving the metadata field order.
  const orderedFiles = Object.fromEntries(Object.entries(metadata.files).sort(([a], [b]) => a < b ? -1 : a > b ? 1 : 0));
  const identity = JSON.stringify({ schemaVersion: 1, source: { repository: metadata.source.repository, sha: metadata.source.sha }, files: orderedFiles }).replace(/[<>&\u2028\u2029]/g, (c) => `\\u${c.charCodeAt(0).toString(16).padStart(4, '0')}`);
  if (digest(identity) !== metadata.inputHash) throw Error('Input identity does not match metadata');
  for (const [relative, checksum] of Object.entries(metadata.files)) {
    if (!/^[a-f0-9]{64}$/.test(checksum)) throw Error('Invalid file checksum');
    const file = safe(input, relative);
    if (!(await lstat(file)).isFile() || digest(await readFile(file)) !== checksum) throw Error('Export file checksum mismatch');
    if (relative !== 'manifest.json' && !/^markdown\/[1-9][0-9]*\.md$/.test(relative) && !relative.startsWith('public/')) throw Error('Unknown export file');
  }
  const snapshot = JSON.parse(await readFile(path.join(input, 'manifest.json'), 'utf8'));
  if (!snapshot.config?.data || !Array.isArray(snapshot.articles) || !Array.isArray(snapshot.media)) throw Error('Invalid public snapshot');
  // Only immutable exported media enter public/, never source fixtures or raw JSON.
  const publicRoot = path.join(site, 'public');
  await rm(publicRoot, { recursive: true, force: true });
  await mkdir(publicRoot, { recursive: true });
  for (const relative of expected.filter((f) => f.startsWith('public/'))) {
    const target = safe(publicRoot, relative.slice('public/'.length));
    await mkdir(path.dirname(target), { recursive: true });
    await copyFile(safe(input, relative), target);
  }
  for (const resource of snapshot.media) {
    if (typeof resource.path !== 'string' || !resource.path.startsWith('/') || !metadata.files['public' + resource.path]) throw Error('Snapshot media is not covered by export');
  }
  console.log(`Prepared ${snapshot.articles.length} published articles and ${snapshot.media.length} media paths`);
}
