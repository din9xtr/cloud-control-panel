import type { Entry, Link, Settings } from "@/api/client";

export const settings: Settings = { quotaBytes: 0, trashRetentionDays: 30, defaultLinkTtlSeconds: 86400 };

export const entries: Entry[] = [
  { name: "seedlings", path: "garden/seedlings", kind: "dir", size: 0, modifiedAt: "2026-10-21T14:02:00Z" },
  { name: "seed-catalog-2026.pdf", path: "garden/seed-catalog-2026.pdf", kind: "file", size: 25_270_000, modifiedAt: "2026-10-18T09:41:00Z", contentType: "application/pdf" },
  { name: "watering-schedule.md", path: "garden/watering-schedule.md", kind: "file", size: 12_288, modifiedAt: "2026-09-30T16:24:00Z", contentType: "text/markdown; charset=utf-8" },
];

export function link(overrides: Partial<Link> = {}): Link {
  return {
    id: "l1",
    mode: "download",
    kind: "file",
    path: "garden/watering-schedule.md",
    name: "watering-schedule.md",
    url: "/s/q7Xv2mK9pLwR4tZaBcDeFg",
    createdAt: "2026-10-25T12:00:00Z",
    expiresAt: new Date(Date.now() + 86_400_000).toISOString(),
    downloads: 0,
    ...overrides,
  };
}

export function apiError(code: string, message: string) {
  return { error: { code, message } };
}
