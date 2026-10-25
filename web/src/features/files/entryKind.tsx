import { File, FileArchive, FileAudio, FileText, FileVideo, Folder, Image } from "lucide-react";
import type { Entry } from "@/api/client";

export type PreviewKind = "image" | "video" | "audio" | "pdf" | "text" | null;

export function previewKind(entry: Pick<Entry, "kind" | "contentType">): PreviewKind {
  const type = entry.contentType ?? "";
  if (entry.kind === "dir") return null;
  if (type.startsWith("image/") && !type.startsWith("image/svg") && !type.startsWith("image/heic")) return "image";
  if (type.startsWith("video/mp4") || type.startsWith("video/webm") || type.startsWith("video/quicktime")) return "video";
  if (type.startsWith("audio/")) return "audio";
  if (type.startsWith("application/pdf")) return "pdf";
  if (type.startsWith("text/plain") || type.startsWith("text/markdown") || type.startsWith("text/csv")) return "text";
  return null;
}

export function EntryIcon({ entry, size = 18 }: { entry: Pick<Entry, "kind" | "contentType">; size?: number }) {
  const props = { size, strokeWidth: 1.6, "aria-hidden": true } as const;
  if (entry.kind === "dir") return <Folder {...props} fill="currentColor" fillOpacity={0.24} className="text-link" />;
  const type = entry.contentType ?? "";
  if (type.startsWith("image/")) return <Image {...props} className="text-orange" />;
  if (type.startsWith("video/")) return <FileVideo {...props} className="text-purple" />;
  if (type.startsWith("audio/")) return <FileAudio {...props} className="text-pink" />;
  if (/zip|x-7z|zstd|gzip|tar|rar/.test(type)) return <FileArchive {...props} className="text-teal" />;
  if (type.startsWith("text/") || type.includes("pdf") || type.includes("epub")) return <FileText {...props} className="text-fg-secondary" />;
  return <File {...props} className="text-fg-subtle" />;
}
