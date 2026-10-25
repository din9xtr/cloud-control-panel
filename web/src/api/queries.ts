import { queryOptions, useMutation, useQueryClient } from "@tanstack/react-query";
import { api, type Settings } from "./client";

function must<T>(data: T | undefined): T {
  if (data === undefined) throw new Error("Empty response");
  return data;
}

export const keys = {
  me: ["me"] as const,
  setup: ["setup"] as const,
  folder: (path: string) => ["folder", path] as const,
  folders: ["folder"] as const,
  search: (path: string, q: string) => ["search", path, q] as const,
  links: ["links"] as const,
  storage: ["storage"] as const,
  trash: ["trash"] as const,
  settings: ["settings"] as const,
  sessions: ["sessions"] as const,
};

export const meQuery = queryOptions({
  queryKey: keys.me,
  queryFn: async () => must((await api.GET("/auth/me")).data).user,
  staleTime: 5 * 60_000,
  retry: false,
});

export const setupQuery = queryOptions({
  queryKey: keys.setup,
  queryFn: async () => must((await api.GET("/setup")).data).required,
  staleTime: Infinity,
  retry: false,
});

export const folderQuery = (path: string) =>
  queryOptions({
    queryKey: keys.folder(path),
    queryFn: async ({ signal }) => must((await api.GET("/files", { params: { query: { path } }, signal })).data),
    staleTime: 10_000,
  });

export const searchQuery = (path: string, q: string) =>
  queryOptions({
    queryKey: keys.search(path, q),
    queryFn: async ({ signal }) => must((await api.GET("/files/search", { params: { query: { path, q } }, signal })).data).entries,
    enabled: q.trim().length > 0,
    staleTime: 10_000,
  });

export const linksQuery = queryOptions({
  queryKey: keys.links,
  queryFn: async () => must((await api.GET("/links")).data).items,
});

export const storageQuery = queryOptions({
  queryKey: keys.storage,
  queryFn: async () => must((await api.GET("/storage")).data),
  staleTime: 30_000,
});

export const trashQuery = queryOptions({
  queryKey: keys.trash,
  queryFn: async () => must((await api.GET("/trash")).data).items,
});

export const settingsQuery = queryOptions({
  queryKey: keys.settings,
  queryFn: async () => must((await api.GET("/settings")).data),
});

export const sessionsQuery = queryOptions({
  queryKey: keys.sessions,
  queryFn: async () => must((await api.GET("/account/sessions")).data).items,
});

function useInvalidate() {
  const client = useQueryClient();
  return (...queryKeys: readonly (readonly unknown[])[]) =>
    Promise.all(queryKeys.map((queryKey) => client.invalidateQueries({ queryKey })));
}

export function useLogin() {
  const client = useQueryClient();
  return useMutation({
    mutationFn: async (body: { username: string; password: string }) => must((await api.POST("/auth/login", { body })).data).user,
    onSuccess: (user) => client.setQueryData(keys.me, user),
  });
}

export function useSetup() {
  const client = useQueryClient();
  return useMutation({
    mutationFn: async (body: { username: string; password: string }) => must((await api.POST("/setup", { body })).data).user,
    onSuccess: (user) => {
      client.setQueryData(keys.me, user);
      client.setQueryData(keys.setup, false);
    },
  });
}

export function useLogout() {
  const client = useQueryClient();
  return useMutation({
    mutationFn: async () => {
      await api.POST("/auth/logout");
    },
    onSettled: () => client.clear(),
  });
}

export function useCreateFolder() {
  const invalidate = useInvalidate();
  return useMutation({
    mutationFn: async (path: string) => must((await api.POST("/files/folders", { body: { path } })).data),
    onSuccess: () => invalidate(keys.folders),
  });
}

export function useRename() {
  const invalidate = useInvalidate();
  return useMutation({
    mutationFn: async (body: { path: string; name: string }) => must((await api.POST("/files/rename", { body })).data),
    onSuccess: () => invalidate(keys.folders, keys.links),
  });
}

export function useMove() {
  const invalidate = useInvalidate();
  return useMutation({
    mutationFn: async (body: { paths: string[]; destination: string }) => {
      await api.POST("/files/move", { body });
    },
    onSuccess: () => invalidate(keys.folders, keys.links, keys.storage),
  });
}

export function useTrash() {
  const invalidate = useInvalidate();
  return useMutation({
    mutationFn: async (paths: string[]) => {
      await api.POST("/files/trash", { body: { paths } });
    },
    onSuccess: () => invalidate(keys.folders, keys.trash, keys.links, keys.storage),
  });
}

export function useRestore() {
  const invalidate = useInvalidate();
  return useMutation({
    mutationFn: async (id: string) => must((await api.POST("/trash/{id}/restore", { params: { path: { id } } })).data).path,
    onSuccess: () => invalidate(keys.folders, keys.trash, keys.storage),
  });
}

export function useDeleteForever() {
  const invalidate = useInvalidate();
  return useMutation({
    mutationFn: async (id: string) => {
      await api.DELETE("/trash/{id}", { params: { path: { id } } });
    },
    onSuccess: () => invalidate(keys.trash, keys.storage),
  });
}

export function useEmptyTrash() {
  const invalidate = useInvalidate();
  return useMutation({
    mutationFn: async () => {
      await api.DELETE("/trash");
    },
    onSuccess: () => invalidate(keys.trash, keys.storage),
  });
}

export function useCreateLink() {
  const invalidate = useInvalidate();
  return useMutation({
    mutationFn: async (body: { path: string; ttlSeconds?: number; mode?: "download" | "upload" }) => must((await api.POST("/links", { body })).data),
    onSuccess: () => invalidate(keys.links, keys.folders),
  });
}

export function useRevokeLink() {
  const invalidate = useInvalidate();
  return useMutation({
    mutationFn: async (id: string) => {
      await api.DELETE("/links/{id}", { params: { path: { id } } });
    },
    onSuccess: () => invalidate(keys.links, keys.folders),
  });
}

export function useUpdateSettings() {
  const client = useQueryClient();
  return useMutation({
    mutationFn: async (body: Settings) => must((await api.PUT("/settings", { body })).data),
    onSuccess: (settings) => {
      client.setQueryData(keys.settings, settings);
      return client.invalidateQueries({ queryKey: keys.storage });
    },
  });
}

export function useChangeCredentials() {
  const client = useQueryClient();
  return useMutation({
    mutationFn: async (body: { currentPassword: string; username?: string; newPassword?: string }) =>
      must((await api.PUT("/account/credentials", { body })).data).user,
    onSuccess: (user) => {
      client.setQueryData(keys.me, user);
      return client.invalidateQueries({ queryKey: keys.sessions });
    },
  });
}

export function useRevokeSession() {
  const invalidate = useInvalidate();
  return useMutation({
    mutationFn: async (id: string) => {
      await api.DELETE("/account/sessions/{id}", { params: { path: { id } } });
    },
    onSuccess: () => invalidate(keys.sessions),
  });
}
