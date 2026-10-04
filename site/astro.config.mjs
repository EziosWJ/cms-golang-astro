import { defineConfig } from "astro/config";
export default defineConfig({ base: process.env.CMS_BASE_PATH || "/", trailingSlash: "always" });
