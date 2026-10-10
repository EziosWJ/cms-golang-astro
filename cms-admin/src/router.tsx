import { createBrowserRouter, Navigate } from "react-router-dom";
import { lazy, Suspense } from "react";
import { RequireAuth } from "@/components/auth/require-auth";
import { AppShell } from "@/components/layout/app-shell";
import { AccountProfilePage } from "@/pages/account-profile";
import { ChangePasswordPage } from "@/pages/change-password";
import { DashboardPage } from "@/pages/dashboard";
import { LoginPage } from "@/pages/login";
import { NotFoundPage } from "@/pages/not-found";
import { GitConnectionPage } from "@/pages/settings/git";
import { SettingsPage } from "@/pages/settings";
import { SystemConfigsPage } from "@/pages/system/configs";
import { SystemDeptsPage } from "@/pages/system/depts";
import { SystemDictsPage } from "@/pages/system/dicts";
import { SystemFilesPage } from "@/pages/system/files";
import { SystemLoginLogsPage } from "@/pages/system/logs/login-logs";
import { SystemOperLogsPage } from "@/pages/system/logs/oper-logs";
import { SystemMenusPage } from "@/pages/system/menus";
import { SystemRolesPage } from "@/pages/system/roles";
import { UsersPage } from "@/pages/system/users";
import { NotificationsPage } from "@/pages/notifications";
import { NotificationManagePage } from "@/pages/system/notifications";
import { ArticlesPage } from "@/pages/content/articles";
import { TaxonomyPage } from "@/pages/content/taxonomy";
import { MediaPage } from "@/pages/content/media";
import { PublicationsPage } from "@/pages/content/publications";
import { SiteConfigPage } from "@/pages/content/site-config";

const ArticleEditorPage = lazy(() => import("@/pages/content/articles/editor").then((module) => ({ default: module.ArticleEditorPage })));

export const router = createBrowserRouter([
  {
    path: "/login",
    element: <LoginPage />,
  },
  {
    path: "/",
    element: (
      <RequireAuth>
        <AppShell />
      </RequireAuth>
    ),
    children: [
      { path: "settings/git", element: <GitConnectionPage /> },
      { path: "content/publications", element: <PublicationsPage /> },
      { path: "content/site-config", element: <SiteConfigPage /> },
      { path: "content/media", element: <MediaPage /> },
      { path: "content/taxonomy", element: <TaxonomyPage /> },
      { path: "content/articles", element: <ArticlesPage /> },
      { path: "content/articles/new", element: <Suspense fallback={<p role="status">加载编辑器…</p>}><ArticleEditorPage /></Suspense> },
      { path: "content/articles/:id", element: <Suspense fallback={<p role="status">加载编辑器…</p>}><ArticleEditorPage /></Suspense> },
      {
        index: true,
        element: <Navigate to="/content/articles" replace />,
      },
      {
        path: "dashboard",
        element: <DashboardPage />,
      },
      {
        path: "system/user",
        element: <UsersPage />,
      },
      {
        path: "settings",
        element: <SettingsPage />,
      },
      {
        path: "system",
        element: <Navigate to="/system/user" replace />,
      },
      {
        path: "system/dept",
        element: <SystemDeptsPage />,
      },
      {
        path: "system/dict",
        element: <SystemDictsPage />,
      },
      {
        path: "system/config",
        element: <SystemConfigsPage />,
      },
      {
        path: "system/role",
        element: <SystemRolesPage />,
      },
      {
        path: "system/menu",
        element: <SystemMenusPage />,
      },
      {
        path: "system/login-log",
        element: <SystemLoginLogsPage />,
      },
      {
        path: "system/oper-log",
        element: <SystemOperLogsPage />,
      },
      {
        path: "system/file",
        element: <SystemFilesPage />,
      },
      {
        path: "notifications",
        element: <NotificationsPage />,
      },
      {
        path: "notifications/:id",
        element: <NotificationsPage />,
      },
      {
        path: "system/notification",
        element: <NotificationManagePage />,
      },
      {
        path: "account/profile",
        element: <AccountProfilePage />,
      },
      {
        path: "account/change-password",
        element: <ChangePasswordPage />,
      },
      {
        path: "*",
        element: <NotFoundPage />,
      },
    ],
  },
]);
