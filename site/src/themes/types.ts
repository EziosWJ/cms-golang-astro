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

export type ThemeTerm = {
  kind: 'category' | 'tag';
  name: string;
  slug: string;
  url: string;
  count: number;
};

export type ThemeArticle = {
  title: string;
  slug: string;
  url: string;
  summary: string;
  dateLabel: string;
  updatedLabel: string;
  publishedAt: string;
  year: number;
  coverUrl: string;
  categories: ThemeTerm[];
  tags: ThemeTerm[];
};

export type ThemeHeading = { depth: 2 | 3; text: string; slug: string };
export type ThemeStat = { value: number; label: string };
export type ThemeArchiveGroup = { year: number; articles: ThemeArticle[] };

export type BaseLayoutProps = {
  site: ThemeSite;
  title?: string;
  description?: string;
  type?: 'website' | 'article';
  canonical?: string;
  noindex?: boolean;
};

export type ArticleListProps = { site: ThemeSite; articles: ThemeArticle[] };
export type HomeViewProps = ArticleListProps & {
  latest: ThemeArticle[];
  categories: ThemeTerm[];
  tags: ThemeTerm[];
  hotTags: ThemeTerm[];
  stats: ThemeStat[];
};
export type ArchiveViewProps = ArticleListProps & {
  groups: ThemeArchiveGroup[];
  firstYear: number | null;
};
export type ArticleProps = {
  site: ThemeSite;
  article: ThemeArticle;
  html: string;
  headings: ThemeHeading[];
  newer?: ThemeArticle;
  older?: ThemeArticle;
  related: ThemeArticle[];
  wordCount: number;
  imageCount: number;
};
export type TaxonomyWallProps = {
  site: ThemeSite;
  title: string;
  terms: ThemeTerm[];
  totalArticles: number;
};
export type TaxonomyDetailProps = {
  site: ThemeSite;
  term: ThemeTerm;
  articles: ThemeArticle[];
  others: ThemeTerm[];
  years: number[];
};
export type NotFoundProps = { site: ThemeSite; recent: ThemeArticle[] };

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
