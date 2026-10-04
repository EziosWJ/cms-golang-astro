export type TaxonomySnapshot = { id: number; kind: "category" | "tag"; name: string; url: string };
export type ArticleDraftInput = {
 coverMediaId: number | null;
 categoryIds: number[];
 tagIds: number[];
  title: string;
  slug: string;
  markdown: string;
  summary: string;
  displayDate: string | null;
};

export type ArticleDetail = {
  published: boolean; unpublishedChanges: boolean; publishedRevisionId: number | null;
  id: number;
  slug: string;
  lifecycle: "active" | "archived";
  slugLockedAt: string | null;
  createdAt: string;
  revisionId?: number;
  draft: Omit<ArticleDraftInput, "slug" | "categoryIds" | "tagIds"> & {
    taxonomy: TaxonomySnapshot[];
    articleId: number;
    version: number;
    savedAt: string;
    savedBy: number;
  };
};

export type ArticleListRecord = {
  published: boolean; unpublishedChanges: boolean;
  id: number;
  slug: string;
  lifecycle: "active" | "archived";
  title: string;
  version: number;
  savedAt: string;
  displayDate: string | null;
};

export const ARTICLE_EDIT_PERMISSION = "content:article:edit";
