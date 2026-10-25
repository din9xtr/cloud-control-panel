import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryHistory, createRootRoute, createRoute, createRouter, Outlet, RouterProvider } from "@tanstack/react-router";
import { render } from "@testing-library/react";
import type { ReactNode } from "react";
import { UploadsProvider } from "@/features/uploads/UploadsProvider";
import type { UploadFactory } from "@/features/uploads/UploadQueue";
import { ToastProvider } from "@/ui/Toast";

const idleFactory: UploadFactory = () => ({ start: () => undefined, pause: () => undefined, cancel: () => undefined });

export function renderWithApp(ui: ReactNode, { path = "/", factory = idleFactory }: { path?: string; factory?: UploadFactory } = {}) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
  const rootRoute = createRootRoute({ component: Outlet });
  const pageRoute = createRoute({ getParentRoute: () => rootRoute, path: "$", component: () => <>{ui}</> });
  const indexRoute = createRoute({ getParentRoute: () => rootRoute, path: "/", component: () => <>{ui}</> });
  const router = createRouter({
    routeTree: rootRoute.addChildren([indexRoute, pageRoute]),
    history: createMemoryHistory({ initialEntries: [path] }),
  });
  const result = render(
    <QueryClientProvider client={queryClient}>
      <ToastProvider>
        <UploadsProvider factory={factory}>
          <RouterProvider router={router} />
        </UploadsProvider>
      </ToastProvider>
    </QueryClientProvider>,
  );
  return { ...result, queryClient, router };
}
