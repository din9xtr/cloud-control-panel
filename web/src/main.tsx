import "./styles/index.css";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { RouterProvider } from "@tanstack/react-router";
import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { ApiError, onUnauthorized } from "@/api/client";
import { keys } from "@/api/queries";
import { createAppRouter } from "@/app/router";
import { UploadsProvider } from "@/features/uploads/UploadsProvider";
import { ToastProvider } from "@/ui/Toast";

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: (failures, error) => !(error instanceof ApiError && error.status < 500) && failures < 2,
      refetchOnWindowFocus: true,
    },
  },
});
const router = createAppRouter(queryClient);

onUnauthorized(() => {
  queryClient.removeQueries({ queryKey: keys.me });
  if (router.state.location.pathname !== "/login") {
    void router.navigate({ to: "/login", search: { redirect: router.state.location.href } });
  }
});

const root = document.getElementById("root");
if (!root) throw new Error("missing #root");

createRoot(root).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <ToastProvider>
        <UploadsProvider>
          <RouterProvider router={router} />
        </UploadsProvider>
      </ToastProvider>
    </QueryClientProvider>
  </StrictMode>,
);
