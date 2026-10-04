export type ThemeManifest = {
  id: string;
  name: string;
  version: string;
  description: string;
  author: string;
  preview: string;
  compatibility: { cmsThemeApi: string };
  capabilities: string[];
};

export type ThemeSite = {
  name: string;
  description: string;
  language: string;
  publicUrl: string;
  authorName: string;
  authorBio: string;
  avatarUrl: string;
  homeUrl: string;
  archiveUrl: string;
  categoriesUrl: string;
  tagsUrl: string;
};

export type ThemeTerm = { kind: 'category' | 'tag'; name: string; slug: string; url: string; count: number };
export type ThemeArticle = { title: string; slug: string; url: string; summary: string; dateLabel: string; updatedLabel: string; coverUrl: string; categories: ThemeTerm[]; tags: ThemeTerm[] };

export type BaseLayoutProps = { site: ThemeSite; title?: string; description?: string };
export type ArticleListProps = { site: ThemeSite; articles: ThemeArticle[] };
export type ArticleProps = { site: ThemeSite; article: ThemeArticle; html: string };
export type TaxonomyWallProps = { site: ThemeSite; title: string; terms: ThemeTerm[] };
export type TaxonomyDetailProps = { site: ThemeSite; term: ThemeTerm; articles: ThemeArticle[] };
export type NotFoundProps = { site: ThemeSite };

export type ThemeModule = {
  themeManifest: ThemeManifest;
  shikiConfig: unknown;
  BaseLayout: unknown;
  HomeView: unknown;
  ArchiveView: unknown;
  PostArticleView: unknown;
  CategoryWallView: unknown;
  CategoryDetailView: unknown;
  TagWallView: unknown;
  TagDetailView: unknown;
  NotFoundView: unknown;
};
