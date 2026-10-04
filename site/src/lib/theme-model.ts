import { articleURL, articles, config, formatDate, mediaURL, taxonomyURL, terms, withBase, type Article, type Taxonomy } from './content';
import type { ThemeArchiveGroup, ThemeArticle, ThemeHeading, ThemeSite, ThemeStat, ThemeTerm } from '../themes/types';

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

function publishedAt(article: Article) { return article.revision.displayDate ?? article.firstPublishedAt; }
function articleYear(article: Article) { return new Date(publishedAt(article)).getFullYear(); }

export function toThemeArticle(article: Article): ThemeArticle {
  const taxonomy = article.revision.taxonomy ?? [];
  const date = publishedAt(article);
  return {
    title: article.revision.title,
    slug: article.revision.slug,
    url: articleURL(article),
    summary: article.revision.summary,
    dateLabel: formatDate(date),
    updatedLabel: formatDate(article.updatedAt),
    publishedAt: date,
    year: articleYear(article),
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

function countWords(markdown: string) {
  const text = markdown
    .replace(/```[\s\S]*?```/g, ' ')
    .replace(/`[^`]*`/g, ' ')
    .replace(/!\[[^\]]*\]\([^)]*\)/g, ' ')
    .replace(/\[[^\]]*\]\([^)]*\)/g, ' ')
    .replace(/[#>*_~|-]/g, ' ');
  const han = text.match(/[\p{Script=Han}]/gu)?.length ?? 0;
  const latin = text.replace(/[\p{Script=Han}]/gu, ' ').match(/[\p{Letter}\p{Number}]+(?:['’-][\p{Letter}\p{Number}]+)*/gu)?.length ?? 0;
  return han + latin;
}
function countImages(markdown: string) { return markdown.match(/!\[[^\]]*\]\([^)]*\)/g)?.length ?? 0; }

export const themeHomeModel = (() => {
  const categories = themeTerms('category');
  const tags = themeTerms('tag');
  const stats: ThemeStat[] = [
    { value: themeArticles.length, label: '文章' },
    { value: categories.length, label: '分类' },
    { value: tags.length, label: '标签' },
  ];
  return {
    articles: themeArticles,
    latest: themeArticles.slice(0, 8),
    categories,
    tags,
    hotTags: [...tags].sort((a, b) => b.count - a.count || a.name.localeCompare(b.name)).slice(0, 12),
    stats,
  };
})();

export const themeArchiveModel = (() => {
  const groupsMap = new Map<number, ThemeArticle[]>();
  for (const article of themeArticles) groupsMap.set(article.year, [...(groupsMap.get(article.year) ?? []), article]);
  const groups: ThemeArchiveGroup[] = [...groupsMap.entries()].sort(([a], [b]) => b - a).map(([year, grouped]) => ({ year, articles: grouped }));
  return { articles: themeArticles, groups, firstYear: groups.length ? groups[groups.length - 1].year : null };
})();

function relatedArticles(article: Article) {
  const own = new Set((article.revision.taxonomy ?? []).map((term) => `${term.kind}:${term.url}`));
  return articles
    .filter((candidate) => candidate !== article)
    .map((candidate) => ({ candidate, score: (candidate.revision.taxonomy ?? []).reduce((score, term) => score + (own.has(`${term.kind}:${term.url}`) ? 1 : 0), 0) }))
    .filter(({ score }) => score > 0)
    .sort((a, b) => b.score - a.score || new Date(publishedAt(b.candidate)).getTime() - new Date(publishedAt(a.candidate)).getTime())
    .slice(0, 3)
    .map(({ candidate }) => toThemeArticle(candidate));
}

export function themeArticleContext(article: Article, headings: ThemeHeading[]) {
  const index = articles.indexOf(article);
  return {
    article: toThemeArticle(article),
    headings,
    newer: index > 0 ? toThemeArticle(articles[index - 1]) : undefined,
    older: index >= 0 && index < articles.length - 1 ? toThemeArticle(articles[index + 1]) : undefined,
    related: relatedArticles(article),
    wordCount: countWords(article.revision.markdown),
    imageCount: countImages(article.revision.markdown),
  };
}

export function themeTaxonomyWall(kind: Taxonomy['kind']) {
  return { terms: themeTerms(kind), totalArticles: themeArticles.length };
}

export function themeTaxonomyDetail(term: Taxonomy) {
  const related = themeArticlesForTerm(term);
  const others = themeTerms(term.kind).filter((item) => item.slug !== term.url).sort((a, b) => b.count - a.count).slice(0, 10);
  const years = [...new Set(related.map((article) => article.year))].sort((a, b) => b - a);
  return { term: toThemeTerm(term), articles: related, others, years };
}
