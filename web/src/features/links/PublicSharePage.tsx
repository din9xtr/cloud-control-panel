import { useQuery } from "@tanstack/react-query";
import { ChevronLeft, Download, FolderOpen, Upload } from "lucide-react";
import { useEffect, useState, useSyncExternalStore } from "react";
import { api, ApiError, messageOf, type Schemas } from "@/api/client";
import { download } from "@/features/files/FileList";
import { createTusFactory } from "@/features/uploads/tusFactory";
import { UploadQueue } from "@/features/uploads/UploadQueue";
import { formatBytes, formatDate } from "@/lib/format";
import { parentPath } from "@/lib/paths";
import { AuthLayout } from "@/features/auth/AuthLayout";
import { EntryIcon } from "@/features/files/entryKind";
import { Wordmark } from "@/layout/Wordmark";
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
  if (shared.isError) return <AuthLayout title="Link unavailable"><ErrorState message={shared.error instanceof ApiError && shared.error.status === 404 ? "This link is no longer available. It may have expired or been revoked." : messageOf(shared.error)} /></AuthLayout>;
  const data = shared.data;
  const endpoint = `/api/v1/public/links/${encodeURIComponent(token)}`;
  const archive = (paths: string[]) => `${endpoint}/archive?${new URLSearchParams(paths.map((item) => ["path", item]))}`;
  const navigate = (next: string) => { setSelected(new Set()); setPath(next); };
  return (
    <div className="flex min-h-dvh flex-col">
      <header className="glass sticky top-0 z-30 h-14 border-b border-line max-md:h-[52px]">
        <div className="mx-auto flex h-full max-w-[1120px] items-center px-6 text-fg max-md:px-4">
          <Wordmark />
        </div>
      </header>
      <main className="mx-auto flex w-full max-w-[1000px] flex-1 flex-col gap-9 px-6 pt-12 pb-16 max-md:gap-7 max-md:px-4 max-md:pt-8">
        <PageHeader eyebrow={data.mode === "upload" ? "File request" : "Shared folder"} title={data.name}>
          Available until {formatDate(data.expiresAt)}
        </PageHeader>
        {data.mode === "upload" ? <FileDrop endpoint={`${endpoint}/uploads`} maxBytes={data.maxUploadBytes} /> : (
          <>
            <div className="flex animate-reveal flex-wrap items-center gap-3 [animation-delay:80ms]">
              {path && <Button onClick={() => navigate(parentPath(path))} icon={<ChevronLeft size={16} />}>Back</Button>}
              <span className="type-small min-w-0 flex-1 truncate text-fg-subtle">{["", ...path.split("/").filter(Boolean)].map((part) => part || data.name).join(" › ")}</span>
              <Button className="max-sm:h-11" onClick={() => download(archive([path]))} icon={<Download size={16} />}>Download folder</Button>
              {selected.size > 0 && <Button variant="primary" className="max-sm:h-11" onClick={() => download(archive([...selected]))}>Download selected ({selected.size})</Button>}
            </div>
            {data.folder?.entries.length === 0 && <EmptyState icon={<FolderOpen size={24} />} title="This folder is empty" />}
            <ul className="m-0 animate-reveal list-none overflow-hidden rounded-card border border-line bg-surface p-0 empty:hidden [animation-delay:160ms]">
              {data.folder?.entries.map((entry) => (
                <li key={entry.path} className="flex min-h-14 items-center gap-4 border-b border-line px-5 py-3 last:border-b-0">
                  <input type="checkbox" className="check" aria-label={`Select ${entry.name}`} checked={selected.has(entry.path)} onChange={() => setSelected((current) => {
                    const next = new Set(current);
                    if (next.has(entry.path)) next.delete(entry.path); else next.add(entry.path);
                    return next;
                  })} />
                  <EntryIcon entry={entry} size={22} />
                  <button className="type-body min-w-0 flex-1 cursor-pointer truncate text-left text-fg transition-colors hover:text-link" onClick={() => entry.kind === "dir" ? navigate(entry.path) : download(`${endpoint}/content?${new URLSearchParams({ path: entry.path })}`)}>
                    {entry.name}
                  </button>
                  <span className="type-small shrink-0 text-fg-subtle tabular-nums">{entry.kind === "dir" ? "Folder" : formatBytes(entry.size)}</span>
                </li>
              ))}
            </ul>
          </>
        )}
      </main>
    </div>
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
    <section className="flex animate-reveal flex-col gap-5 [animation-delay:80ms]">
      <p className="type-body m-0 max-w-xl text-fg-muted">Send files to this folder. Its contents stay private. Existing files are never overwritten.</p>
      <label className="card flex cursor-pointer flex-col items-center gap-4 border-2 border-dashed border-line-strong px-5 py-10 text-center transition-colors duration-200 ease-swift hover:border-link focus-within:border-link sm:py-16" onDragOver={(event) => event.preventDefault()} onDrop={(event) => { event.preventDefault(); add(Array.from(event.dataTransfer.files)); }}>
        <Upload size={36} className="text-link" aria-hidden />
        <span className="type-body inline-flex h-11 items-center justify-center rounded-control bg-accent px-5 font-medium text-on-accent">Choose files</span>
        <span className="type-body text-fg-muted">or drop them here</span>
        <span className="type-micro flex flex-col gap-1 text-fg-subtle"><span>Up to {formatBytes(maxBytes)} per file</span><span>Uploads resume after interruptions</span></span>
        <input type="file" multiple aria-label="Choose files to upload" className="sr-only" onChange={(event) => { if (event.target.files) add(Array.from(event.target.files)); event.target.value = ""; }} />
      </label>
      {error && <p role="alert" className="type-small m-0 text-danger">{error}</p>}
      <ul className="m-0 flex list-none flex-col gap-3 p-0" aria-live="polite">
        {items.map((item) => (
          <li key={item.id} className="card type-small grid animate-rise grid-cols-[minmax(0,1fr)_auto] items-center gap-3 px-5 py-4 sm:flex sm:flex-wrap">
            <span className="col-span-2 min-w-0 flex-1 truncate">{item.name}</span>
            <span className="type-micro min-w-0 break-words text-fg-subtle">{item.status === "done" ? "Uploaded" : item.status === "error" ? item.error : `${Math.round(item.size ? item.uploaded / item.size * 100 : 0)}% · ${item.status}`}</span>
            {item.status === "uploading" && <Button size="sm" onClick={() => queue.pause(item.id)}>Pause</Button>}
            {(item.status === "paused" || item.status === "error") && <Button size="sm" onClick={() => queue.resume(item.id)}>Resume</Button>}
            {item.status !== "done" && <Button size="sm" onClick={() => queue.cancel(item.id)}>Cancel</Button>}
          </li>
        ))}
      </ul>
    </section>
  );
}
