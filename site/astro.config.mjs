import { defineConfig } from "astro/config";
import { existsSync, readFileSync, readdirSync } from "node:fs";
import path from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";

const siteRoot = path.dirname(fileURLToPath(import.meta.url));
const themesRoot = path.join(siteRoot, "src", "themes");
const DEFAULT_THEME = "comic";

const availableThemes = readdirSync(themesRoot, { withFileTypes: true })
  .filter((entry) => entry.isDirectory())
  .map((entry) => entry.name)
  .filter((name) => existsSync(path.join(themesRoot, name, "index.ts")) && existsSync(path.join(themesRoot, name, "manifest.ts")))
  .sort();

function selectedTheme() {
  if (process.env.CMS_INPUT_PATH) {
    const snapshot = JSON.parse(readFileSync(process.env.CMS_INPUT_PATH, "utf8"));
    const id = snapshot?.config?.data?.theme;
    const version = snapshot?.config?.data?.themeVersion;
    if (!id || !version) throw new Error("Publication Manifest is missing config.data.theme/themeVersion; save and publish site config first");
    return { id, version, formal: true };
  }
  return { id: process.env.BLOG_THEME || DEFAULT_THEME, version: null, formal: false };
}

const selection = selectedTheme();
if (!availableThemes.includes(selection.id)) throw new Error(`Unknown theme "${selection.id}". Available themes: ${availableThemes.join(", ")}`);
const selectedRoot = path.join(themesRoot, selection.id);
const manifestModule = await import(pathToFileURL(path.join(selectedRoot, "manifest.ts")).href);
const manifest = manifestModule.themeManifest;
if (!manifest || manifest.id !== selection.id) throw new Error(`Theme manifest id must match directory "${selection.id}"`);
if (manifest.compatibility?.cmsThemeApi !== "1") throw new Error(`Theme "${selection.id}" is not compatible with CMS Theme API v1`);
if (selection.formal && manifest.version !== selection.version) throw new Error(`Theme version mismatch: release requires ${selection.id}@${selection.version}, site contains ${selection.id}@${manifest.version}`);

export default defineConfig({
  base: process.env.CMS_BASE_PATH || "/",
  trailingSlash: "always",
  vite: { resolve: { alias: { "@theme": path.join(selectedRoot, "index.ts") } } },
});
