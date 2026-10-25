import { useQuery } from "@tanstack/react-query";
import { Download, Link2, X } from "lucide-react";
import { useEffect, useId, useRef } from "react";
import type { Entry } from "@/api/client";
import { formatBytes, formatDate } from "@/lib/format";
import { contentUrl, displayPath, parentPath } from "@/lib/paths";
import { decodeText } from "@/lib/text";
import { Button, IconButton } from "@/ui/Button";
import { ErrorState, Loading } from "@/ui/Feedback";
import { previewKind } from "./entryKind";
import { download } from "./FileList";

const maxTextBytes = 1024 * 1024;

export function PreviewDialog({ entry, onClose, onShare }: { entry: Entry; onClose: () => void; onShare: (entry: Entry) => void }) {
  const ref = useRef<HTMLDialogElement>(null);
  const titleId = useId();

  useEffect(() => {
    ref.current?.showModal();
  }, []);

  return (
    <dialog
      ref={ref}
      aria-labelledby={titleId}
      onClose={onClose}
      onCancel={(e) => {
        e.preventDefault();
        onClose();
      }}
      className="m-auto h-dvh max-h-none w-full max-w-none animate-rise border-0 bg-sunken p-0 text-fg shadow-dialog backdrop:animate-fade-in sm:h-[min(88dvh,900px)] sm:w-[min(92vw,1200px)] sm:rounded-sheet"
    >
      <div className="flex h-full flex-col">
        <header className="flex items-center gap-3 py-3 pr-3 pl-6 pt-[max(env(safe-area-inset-top),12px)] max-sm:pl-4">
          <div className="flex min-w-0 flex-1 flex-col">
            <h2 id={titleId} className="type-title m-0 truncate">
              {entry.name}
            </h2>
            <span className="type-micro truncate text-fg-subtle">
              {displayPath(parentPath(entry.path))} · {formatBytes(entry.size)} · {formatDate(entry.modifiedAt)}
            </span>
          </div>
          <Button size="sm" icon={<Link2 size={14} aria-hidden />} onClick={() => onShare(entry)} className="max-sm:hidden">
            Link
          </Button>
          <Button size="sm" icon={<Download size={14} aria-hidden />} onClick={() => download(contentUrl(entry.path))}>
            <span className="max-sm:sr-only">Download</span>
          </Button>
          <IconButton label="Close preview" onClick={onClose} className="bg-elevated">
            <X size={16} strokeWidth={2.25} aria-hidden />
          </IconButton>
        </header>
        <div className="flex min-h-0 flex-1 items-center justify-center overflow-auto px-6 pb-6 max-sm:px-3">
          <PreviewBody entry={entry} />
        </div>
      </div>
    </dialog>
  );
}

function PreviewBody({ entry }: { entry: Entry }) {
  const src = contentUrl(entry.path, true);
  switch (previewKind(entry)) {
    case "image":
      return <img src={src} alt={entry.name} className="max-h-full max-w-full animate-fade-in rounded-card object-contain" />;
    case "video":
      return <video src={src} controls playsInline preload="metadata" className="max-h-full max-w-full rounded-card bg-black" />;
    case "audio":
      return <audio src={src} controls preload="metadata" className="w-full max-w-lg" />;
    case "pdf":
      return <iframe src={src} title={entry.name} className="h-full w-full rounded-card border-0 bg-white" />;
    case "text":
      return <TextPreview entry={entry} src={src} />;
    default:
      return <ErrorState message="No preview for this file type." />;
  }
}

function TextPreview({ entry, src }: { entry: Entry; src: string }) {
  const tooLarge = entry.size > maxTextBytes;
  const { data, isPending, isError } = useQuery({
    queryKey: ["text", entry.path, entry.modifiedAt],
    queryFn: async ({ signal }) => decodeText(await (await fetch(src, { signal, credentials: "same-origin" })).arrayBuffer()),
    enabled: !tooLarge,
  });
  if (tooLarge) return <ErrorState message="This file is too large to preview. Download it instead." />;
  if (isPending) return <Loading />;
  if (isError) return <ErrorState message="Could not load the file." />;
  return <pre className="m-0 h-full w-full max-w-4xl overflow-auto rounded-card border border-line bg-surface p-6 font-mono text-[13px] leading-relaxed whitespace-pre-wrap text-fg-secondary">{data}</pre>;
}
