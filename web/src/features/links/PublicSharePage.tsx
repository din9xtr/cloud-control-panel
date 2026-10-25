import { useQuery } from "@tanstack/react-query";
import { ChevronLeft, Download, FolderOpen, Upload } from "lucide-react";
import { useEffect, useState, useSyncExternalStore } from "react";
import { api, ApiError, messageOf, type Schemas } from "@/api/client";
import { download } from "@/features/files/FileList";
import { createTusFactory } from "@/features/uploads/tusFactory";
import { UploadQueue } from "@/features/uploads/UploadQueue";
import { formatBytes, formatDate } from "@/lib/format";
import { parentPath } from "@/lib/paths";
import { Button } from "@/ui/Button";
import { EmptyState, ErrorState, Loading, PageHeader } from "@/ui/Feedback";

export function PublicSharePage({ token }: { token: string }) {
  const [path, setPath] = useState("");
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const shared = useQuery({
    queryKey: ["public-link", token, path],
    queryFn: async () => {
      const { data } = await api.GET("/public/links/{token}", { params: { path: { token }, query: { path } } });
      if (!data) throw new Error("Could not load the shared folder.");
      return data;
    },
    retry: false,
  });
  if (shared.isPending) return <Loading />;
  if (shared.isError) return <main className="mx-auto max-w-xl px-4"><ErrorState message={shared.error instanceof ApiError && shared.error.status === 404 ? "This link is no longer available. It may have expired or been revoked." : messageOf(shared.error)} /></main>;
  const data = shared.data;
  const endpoint = `/api/v1/public/links/${encodeURIComponent(token)}`;
  const archive = (paths: string[]) => `${endpoint}/archive?${new URLSearchParams(paths.map((item) => ["path", item]))}`;
  const navigate = (next: string) => { setSelected(new Set()); setPath(next); };
  return (
    <main className="mx-auto flex min-h-screen max-w-3xl flex-col gap-6 px-4 py-6 sm:gap-8 sm:px-7 sm:py-12">
      <div className="flex flex-col gap-2">
        <PageHeader eyebrow={data.mode === "upload" ? "File request" : "Shared folder"} title={data.name} />
        <p className="m-0 text-[13px] text-fg-subtle">Available until {formatDate(data.expiresAt)}</p>
      </div>
      {data.mode === "upload" ? <FileDrop endpoint={`${endpoint}/uploads`} maxBytes={data.maxUploadBytes} /> : (
        <>
          <div className="grid grid-cols-2 items-center gap-3 sm:flex sm:flex-wrap">
            {path && <Button onClick={() => navigate(parentPath(path))} icon={<ChevronLeft size={16} />}>Back</Button>}
            <span className="col-span-2 row-start-1 min-w-0 flex-1 truncate font-mono text-sm text-fg-muted">/{path}</span>
            <Button className="max-sm:h-11" onClick={() => download(archive([path]))} icon={<Download size={16} />}>Download folder</Button>
            {selected.size > 0 && <Button variant="primary" className="max-sm:col-span-2 max-sm:h-11" onClick={() => download(archive([...selected]))}>Download selected ({selected.size})</Button>}
          </div>
          {data.folder?.entries.length === 0 && <EmptyState icon={<FolderOpen size={20} />} title="This folder is empty" />}
          <ul className="m-0 list-none overflow-hidden rounded-xl border border-line p-0">
            {data.folder?.entries.map((entry) => (
              <li key={entry.path} className="flex items-center gap-3 border-b border-line p-4 last:border-b-0">
                <input type="checkbox" aria-label={`Select ${entry.name}`} checked={selected.has(entry.path)} onChange={() => setSelected((current) => {
                  const next = new Set(current);
                  if (next.has(entry.path)) next.delete(entry.path); else next.add(entry.path);
                  return next;
                })} />
                <button className="min-w-0 flex-1 cursor-pointer truncate text-left text-sm text-fg hover:underline" onClick={() => entry.kind === "dir" ? navigate(entry.path) : download(`${endpoint}/content?${new URLSearchParams({ path: entry.path })}`)}>
                  {entry.kind === "dir" && <FolderOpen size={16} className="mr-2 inline" aria-hidden />}{entry.name}
                </button>
                <span className="shrink-0 font-mono text-xs text-fg-subtle">{entry.kind === "dir" ? "Folder" : formatBytes(entry.size)}</span>
              </li>
            ))}
          </ul>
        </>
      )}
    </main>
  );
}

function FileDrop({ endpoint, maxBytes }: { endpoint: string; maxBytes: Schemas["PublicLink"]["maxUploadBytes"] }) {
  const [queue] = useState(() => new UploadQueue(createTusFactory(endpoint), () => undefined));
  const items = useSyncExternalStore(queue.subscribe, queue.getSnapshot);
  const [error, setError] = useState("");
  const busy = items.some((item) => item.status === "uploading" || item.status === "queued");
  useEffect(() => {
    if (!busy) return;
    const warn = (event: BeforeUnloadEvent) => event.preventDefault();
    window.addEventListener("beforeunload", warn);
    return () => window.removeEventListener("beforeunload", warn);
  }, [busy]);
  useEffect(() => () => queue.pauseAll(), [queue]);
  const add = (files: File[]) => {
    if (files.some((file) => file.size > maxBytes)) { setError(`Each file must be ${formatBytes(maxBytes)} or smaller.`); return; }
    setError("");
    queue.add(files, "");
  };
  return (
    <section className="flex flex-col gap-4">
      <p className="m-0 max-w-xl text-sm leading-relaxed text-fg-muted">Send files to this folder. Its contents stay private. Existing files are never overwritten.</p>
      <label className="flex cursor-pointer flex-col items-center gap-4 rounded-2xl border border-dashed border-line-strong bg-sunken px-5 py-8 text-center transition-colors hover:border-fg-muted focus-within:border-fg-muted sm:py-12" onDragOver={(event) => event.preventDefault()} onDrop={(event) => { event.preventDefault(); add(Array.from(event.dataTransfer.files)); }}>
        <Upload size={28} aria-hidden />
        <span className="inline-flex min-h-11 items-center justify-center rounded-lg bg-fg px-5 text-sm font-medium text-canvas">Choose files</span>
        <span className="text-sm text-fg-muted">or drop them here</span>
        <span className="flex flex-col gap-1 text-xs leading-relaxed text-fg-subtle"><span>Up to {formatBytes(maxBytes)} per file</span><span>Uploads resume after interruptions</span></span>
        <input type="file" multiple aria-label="Choose files to upload" className="sr-only" onChange={(event) => { if (event.target.files) add(Array.from(event.target.files)); event.target.value = ""; }} />
      </label>
      {error && <p role="alert" className="text-sm text-danger">{error}</p>}
      <ul className="m-0 flex list-none flex-col gap-3 p-0" aria-live="polite">
        {items.map((item) => (
          <li key={item.id} className="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-3 rounded-xl border border-line p-4 text-sm sm:flex sm:flex-wrap">
            <span className="col-span-2 min-w-0 flex-1 truncate">{item.name}</span>
            <span className="min-w-0 break-words text-xs text-fg-muted">{item.status === "done" ? "Uploaded" : item.status === "error" ? item.error : `${Math.round(item.size ? item.uploaded / item.size * 100 : 0)}% · ${item.status}`}</span>
            {item.status === "uploading" && <Button size="sm" onClick={() => queue.pause(item.id)}>Pause</Button>}
            {(item.status === "paused" || item.status === "error") && <Button size="sm" onClick={() => queue.resume(item.id)}>Resume</Button>}
            {item.status !== "done" && <Button size="sm" onClick={() => queue.cancel(item.id)}>Cancel</Button>}
          </li>
        ))}
      </ul>
    </section>
  );
}
