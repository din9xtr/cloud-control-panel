import { QueryClient } from "@tanstack/react-query";
import { createRootRouteWithContext, createRoute, createRouter, lazyRouteComponent, Outlet, redirect, useParams } from "@tanstack/react-router";
import { meQuery, setupQuery } from "@/api/queries";
import { LoginPage } from "@/features/auth/LoginPage";
import { SetupPage } from "@/features/auth/SetupPage";
import { PublicSharePage } from "@/features/links/PublicSharePage";
import { FilesPage } from "@/features/files/FilesPage";
import { AppShell } from "@/layout/AppShell";
import { EmptyState } from "@/ui/Feedback";
import { FileQuestion } from "lucide-react";

type RouterContext = { queryClient: QueryClient };

const rootRoute = createRootRouteWithContext<RouterContext>()({
  component: Outlet,
  notFoundComponent: () => (
    <EmptyState icon={<FileQuestion size={20} />} title="Page not found">
      The address does not match any page.
    </EmptyState>
  ),
});

const loginRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/login",
  validateSearch: (search: Record<string, unknown>): { redirect?: string } =>
    typeof search.redirect === "string" ? { redirect: search.redirect } : {},
  beforeLoad: async ({ context }) => {
    if (await context.queryClient.ensureQueryData(setupQuery).catch(() => false)) throw redirect({ to: "/setup" });
    const me = await context.queryClient.ensureQueryData(meQuery).catch(() => null);
    if (me) throw redirect({ to: "/files" });
  },
  component: function Login() {
    const { redirect: target } = loginRoute.useSearch();
    return <LoginPage redirect={target} />;
  },
});

const setupRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/setup",
  beforeLoad: async ({ context }) => {
    const required = await context.queryClient.ensureQueryData(setupQuery).catch(() => false);
    if (!required) throw redirect({ to: "/login" });
  },
  component: SetupPage,
});

const appRoute = createRoute({
  getParentRoute: () => rootRoute,
  id: "app",
  beforeLoad: async ({ context, location }) => {
    try {
      await context.queryClient.ensureQueryData(meQuery);
    } catch {
      if (await context.queryClient.ensureQueryData(setupQuery).catch(() => false)) throw redirect({ to: "/setup" });
      throw redirect({ to: "/login", search: { redirect: location.href } });
    }
  },
  component: AppShell,
});

const indexRoute = createRoute({
  getParentRoute: () => appRoute,
  path: "/",
  beforeLoad: () => {
    throw redirect({ to: "/files" });
  },
});

const filesRoute = createRoute({
  getParentRoute: () => appRoute,
  path: "/files",
  component: () => <FilesPage key="" path="" />,
});

const folderRoute = createRoute({
  getParentRoute: () => appRoute,
  path: "/files/$",
  component: function Folder() {
    const { _splat } = useParams({ from: "/app/files/$" });
    const path = (_splat ?? "").split("/").filter(Boolean).join("/");
    return <FilesPage key={path} path={path} />;
  },
});

const linksRoute = createRoute({
  getParentRoute: () => appRoute,
  path: "/links",
  component: lazyRouteComponent(() => import("@/features/links/LinksPage"), "LinksPage"),
});
const storageRoute = createRoute({
  getParentRoute: () => appRoute,
  path: "/storage",
  component: lazyRouteComponent(() => import("@/features/storage/StoragePage"), "StoragePage"),
});
const trashRoute = createRoute({
  getParentRoute: () => appRoute,
  path: "/trash",
  component: lazyRouteComponent(() => import("@/features/trash/TrashPage"), "TrashPage"),
});
const settingsRoute = createRoute({
  getParentRoute: () => appRoute,
  path: "/settings",
  component: lazyRouteComponent(() => import("@/features/settings/SettingsPage"), "SettingsPage"),
});

const publicRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/share/$token",
  component: function SharedFolder() {
    const { token } = publicRoute.useParams();
    return <PublicSharePage key={token} token={token} />;
  },
});

const routeTree = rootRoute.addChildren([
  publicRoute,
  loginRoute,
  setupRoute,
  appRoute.addChildren([indexRoute, filesRoute, folderRoute, linksRoute, storageRoute, trashRoute, settingsRoute]),
]);

export function createAppRouter(queryClient: QueryClient) {
  return createRouter({
    routeTree,
    context: { queryClient },
    defaultPreload: "intent",
    defaultPreloadStaleTime: 0,
    scrollRestoration: true,
  });
}

declare module "@tanstack/react-router" {
  interface Register {
    router: ReturnType<typeof createAppRouter>;
  }
}
