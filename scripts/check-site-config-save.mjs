import { createRequire } from "node:module";

if (!process.env.CMS_WORKFLOW_URL || !process.env.CMS_PLAYWRIGHT_MODULE || !process.env.CMS_CHROMIUM_PATH) {
  throw new Error("请提供 CMS_WORKFLOW_URL、CMS_PLAYWRIGHT_MODULE 和 CMS_CHROMIUM_PATH。");
}

const require = createRequire(process.env.CMS_PLAYWRIGHT_MODULE);
if (require("./package.json").version !== "1.63.0") throw new Error("Playwright version mismatch");
const { chromium } = require("playwright");

const browser = await chromium.launch({
  headless: true,
  executablePath: process.env.CMS_CHROMIUM_PATH,
  args: ["--no-sandbox"],
});
if (browser.version() !== "153.0.8010.12") throw new Error("Chromium version mismatch");

const page = await browser.newPage();
const base = process.env.CMS_WORKFLOW_URL.replace(/\/$/, "");
let saveRequests = 0;
page.on("request", (request) => {
  if (request.method() === "PUT" && request.url().endsWith("/api/v1/site-config")) saveRequests += 1;
});

try {
  await page.goto(`${base}/login`);
  await page.locator("#username").fill(process.env.CMS_WORKFLOW_USER ?? "admin");
  await page.locator("#password").fill(process.env.CMS_WORKFLOW_PASSWORD ?? "admin123");
  await page.getByRole("button", { name: "登录", exact: true }).click();
  await page.waitForURL("**/content/articles");

  await page.goto(`${base}/content/site-config`);
  const siteName = page.locator("#config-siteName");
  const saveButton = page.getByRole("button", { name: "保存修改", exact: true });
  await siteName.waitFor();
  if (await saveButton.isDisabled()) throw new Error("初始配置没有修订时保存按钮不可用");
  await saveButton.click();
  await page.getByText("已保存，尚未发布", { exact: true }).waitFor();
  if (saveRequests !== 1) throw new Error(`首次保存请求次数=${saveRequests}，期望 1`);
  await siteName.fill(`${await siteName.inputValue()}（回归测试）`);
  if (await saveButton.isDisabled()) throw new Error("编辑站点名称后保存按钮不可用");
  await saveButton.click();
  await page.getByText("已保存，尚未发布", { exact: true }).waitFor();
  if (saveRequests !== 2) throw new Error(`修改后保存请求次数=${saveRequests}，期望 2`);
  console.log(JSON.stringify({ ok: true, saveRequests }));
} finally {
  await browser.close();
}
