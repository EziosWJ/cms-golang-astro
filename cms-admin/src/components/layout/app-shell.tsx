import { useState } from "react";
import { Link, Outlet } from "react-router-dom";
import { Dialog, DialogOverlay, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { convertUserMenusToNavItems, defaultNavItems, mergeNavItems, type NavItem } from "@/config/navigation";
import { useAuthStore } from "@/store/auth-store";
import { AppHeader } from "@/components/layout/app-header";
import { AppSidebar } from "@/components/layout/app-sidebar";
import { RouteLoadingIndicator } from "@/components/layout/route-loading-indicator";
import { useRouteNavigationLoading } from "@/components/layout/use-route-navigation-loading";
import { cn } from "@/lib/utils";

export function AppShell() {
  const [mobileOpen,setMobileOpen]=useState(false);
 const menus=useAuthStore((state) => state.menus);
 const mobileLinks=(items:NavItem[]):NavItem[] => items.flatMap((item) => item.children ? mobileLinks(item.children) : [item]);
  const [collapsed, setCollapsed] = useState(false);
  const { navigationIntent, handleNavigationClickCapture } = useRouteNavigationLoading();

  return (
    <div className="min-h-screen bg-background" onClickCapture={handleNavigationClickCapture}>
      <RouteLoadingIndicator visible={navigationIntent} />
      <a
        href="#main-content"
        className="sr-only focus:not-sr-only focus:fixed focus:left-4 focus:top-4 focus:z-50 focus:rounded-lg focus:bg-primary focus:px-4 focus:py-2 focus:text-white"
      >
        跳转到主内容
      </a>
      <AppSidebar collapsed={collapsed} />
 <Dialog open={mobileOpen} onOpenChange={setMobileOpen} trapFocus restoreFocus lockScroll><DialogOverlay /><DialogContent><DialogHeader><DialogTitle>CMS 内容管理</DialogTitle></DialogHeader><nav aria-label="移动端主导航" className="grid gap-space-3">{mobileLinks(mergeNavItems(defaultNavItems,convertUserMenusToNavItems(menus))).map((item) => item.externalUrl ? <a key={item.path} href={item.externalUrl}>{item.label}</a> : <Link key={item.path} to={item.path} onClick={() => setMobileOpen(false)}>{item.label}</Link>)}</nav></DialogContent></Dialog>
      <div
        className={cn(
          "min-h-screen transition-[padding-left]",
          collapsed ? "md:pl-16" : "md:pl-60",
        )}
      >
        <AppHeader onToggleSidebar={() => { if(window.matchMedia("(max-width: 767px)").matches) setMobileOpen(true); else setCollapsed((value) => !value); }} />
        <main id="main-content" className="p-4 md:p-6">
          <Outlet />
        </main>
      </div>
    </div>
  );
}
