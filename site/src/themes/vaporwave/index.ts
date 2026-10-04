import BaseLayout from './layouts/BaseLayout.astro';
import HomeView from './views/HomeView.astro';
import ArchiveView from './views/ArchiveView.astro';
import PostArticleView from './views/PostArticleView.astro';
import TaxonomyWallView from './views/TaxonomyWallView.astro';
import TaxonomyDetailView from './views/TaxonomyDetailView.astro';
import NotFoundView from './views/NotFoundView.astro';
import { themeManifest, shikiConfig } from './manifest';
import type { ThemeModule } from '../types';

const theme = { themeManifest, shikiConfig, BaseLayout, HomeView, ArchiveView, PostArticleView, CategoryWallView: TaxonomyWallView, CategoryDetailView: TaxonomyDetailView, TagWallView: TaxonomyWallView, TagDetailView: TaxonomyDetailView, NotFoundView } satisfies ThemeModule;
export default theme;
export { themeManifest, shikiConfig, BaseLayout, HomeView, ArchiveView, PostArticleView, TaxonomyWallView as CategoryWallView, TaxonomyDetailView as CategoryDetailView, TaxonomyWallView as TagWallView, TaxonomyDetailView as TagDetailView, NotFoundView };
