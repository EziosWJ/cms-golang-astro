import { articleURL, articles, config, formatDate, mediaURL, taxonomyURL, terms, withBase, type Article, type Taxonomy } from './content';
import type { ThemeArticle, ThemeSite, ThemeTerm } from '../themes/types';

export const themeSite: ThemeSite = {
  name: config.siteName,
  description: config.description,
  language: config.language,
  publicUrl: config.publicUrl,
  authorName: config.authorName,
  authorBio: config.authorBio,
  avatarUrl: config.avatarMediaId ? mediaURL(config.avatarMediaId) : '',
  homeUrl: withBase('/'),
  archiveUrl: withBase('/archives/'),
  categoriesUrl: withBase('/categories/'),
  tagsUrl: withBase('/tags/'),
};

export function toThemeArticle(article: Article): ThemeArticle {
  const taxonomy = article.revision.taxonomy ?? [];
  return {
    title: article.revision.title,
    slug: article.revision.slug,
    url: articleURL(article),
    summary: article.revision.summary,
    dateLabel: formatDate(article.revision.displayDate ?? article.firstPublishedAt),
    updatedLabel: formatDate(article.updatedAt),
    coverUrl: article.revision.coverMediaId ? mediaURL(article.revision.coverMediaId) : '',
    categories: taxonomy.filter((term) => term.kind === 'category').map(toThemeTerm),
    tags: taxonomy.filter((term) => term.kind === 'tag').map(toThemeTerm),
  };
}

export function toThemeTerm(term: Taxonomy): ThemeTerm {
  return {
    kind: term.kind,
    name: term.name,
    slug: term.url,
    url: taxonomyURL(term),
    count: articles.filter((article) => (article.revision.taxonomy ?? []).some((value) => value.id === term.id)).length,
  };
}

export const themeArticles = articles.map(toThemeArticle);
export function themeTerms(kind: Taxonomy['kind']) { return terms(kind).map(toThemeTerm); }
export function themeArticlesForTerm(term: Taxonomy) { return articles.filter((article) => (article.revision.taxonomy ?? []).some((value) => value.id === term.id)).map(toThemeArticle); }
