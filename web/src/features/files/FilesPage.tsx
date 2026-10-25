import { useQuery } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import { ArrowRightLeft, ChevronDown, ChevronLeft, CircleCheck, Download, FolderPlus, Image, Plus, Search, Trash2, Upload, X } from "lucide-react";
import { type DragEvent, type ReactNode, useDeferredValue, useEffect, useRef, useState } from "react";
import { type Entry, messageOf } from "@/api/client";
import { folderQuery, searchQuery, useTrash } from "@/api/queries";
import { ShareDialog } from "@/features/links/ShareDialog";
import { droppedFiles, enqueueSelection, selectedFiles } from "@/features/uploads/folderUpload";
import { useUploadQueue } from "@/features/uploads/UploadsProvider";
import { cn } from "@/lib/cn";
import { formatBytes } from "@/lib/format";
import { archiveUrl, baseName, contentUrl, crumbs, filesHref, parentPath } from "@/lib/paths";
import { Button, IconButton } from "@/ui/Button";
import { EmptyState, ErrorState, Loading } from "@/ui/Feedback";
import { Menu } from "@/ui/Menu";
import { useToast } from "@/ui/Toast";
import { MoveDialog, NewFolderDialog, RenameDialog } from "./dialogs";
import { previewKind } from "./entryKind";
import { download, type EntryActions, FileList } from "./FileList";
import { PreviewDialog } from "./PreviewDialog";

type Modal =
  | { kind: "none" }
  | { kind: "newFolder" }
  | { kind: "rename"; path: string }
  | { kind: "move"; paths: string[] }
  | { kind: "share"; entry: Entry }
  | { kind: "preview"; entry: Entry };

export function FilesPage({ path }: { path: string }) {
  const [query, setQuery] = useState("");
  const deferredQuery = useDeferredValue(query.trim());
  const searching = deferredQuery.length > 0;
  const folder = useQuery(folderQuery(path));
  const search = useQuery(searchQuery(path, deferredQuery));
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [selectMode, setSelectMode] = useState(false);
  const [modal, setModal] = useState<Modal>({ kind: "none" });
  const [preparing, setPreparing] = useState(false);
  const [dragging, setDragging] = useState(false);
  const searchRef = useRef<HTMLInputElement>(null);
  const uploads = useUploadQueue();
  const trash = useTrash();
  const toast = useToast();

  const entries = (searching ? search.data : folder.data?.entries) ?? [];
  const selectedEntries = entries.filter((e) => selected.has(e.path));
  const selectedBytes = selectedEntries.reduce((sum, e) => sum + e.size, 0);

  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      const target = event.target as HTMLElement;
      const typing = target.closest("input, textarea, select, [contenteditable]") || document.querySelector("dialog[open]");
      if (event.key === "/" && !typing) {
        event.preventDefault();
        searchRef.current?.focus();
      }
      if (event.key === "Escape" && !typing) setSelected(new Set());
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, []);

  const toggle = (p: string) =>
    setSelected((current) => {
      const next = new Set(current);
      if (next.has(p)) next.delete(p);
      else next.add(p);
      return next;
    });
  const toggleAll = () => setSelected((current) => (entries.every((e) => current.has(e.path)) ? new Set() : new Set(entries.map((e) => e.path))));
  const clearSelection = () => {
    setSelected(new Set());
    setSelectMode(false);
  };

  const moveToTrash = (paths: string[]) =>
    trash.mutate(paths, {
      onSuccess: () => {
        toast.notify(paths.length === 1 ? `Moved ${baseName(paths[0] ?? "")} to Trash` : `Moved ${paths.length} items to Trash`);
        clearSelection();
      },
      onError: (e) => toast.fail(messageOf(e)),
    });

  const actions: EntryActions = {
    open: (entry) => (previewKind(entry) ? setModal({ kind: "preview", entry }) : download(contentUrl(entry.path))),
    share: (entry) => setModal({ kind: "share", entry }),
    rename: (entry) => setModal({ kind: "rename", path: entry.path }),
    move: (paths) => setModal({ kind: "move", paths }),
    trash: moveToTrash,
  };

  const enqueue = async (selection: ReturnType<typeof selectedFiles>) => {
    setPreparing(true);
    try {
      await enqueueSelection(selection, path, uploads);
      await folder.refetch();
    } catch (error) { toast.fail(messageOf(error)); }
    finally { setPreparing(false); }
  };

  const pickFiles = (accept?: string, directory = false) => {
    const input = document.createElement("input");
    input.type = "file";
    input.multiple = true;
    input.webkitdirectory = directory;
    if (accept) input.accept = accept;
    input.onchange = () => { if (input.files) void enqueue(selectedFiles(Array.from(input.files))); };
    input.click();
  };

  const onDrop = async (event: DragEvent) => {
    event.preventDefault();
    setDragging(false);
    try {
      const selection = await droppedFiles(event.dataTransfer.items, event.dataTransfer.files);
      await enqueue(selection);
    } catch (error) { toast.fail(messageOf(error)); }
  };

  const title = baseName(path) || "Files";
  const listLabel = searching ? `Search results for ${deferredQuery}` : `Files in ${title}`;

  return (
    <div
      className="relative flex min-h-full flex-1 flex-col"
      onDragOver={(e) => {
        if (!e.dataTransfer.types.includes("Files")) return;
        e.preventDefault();
        setDragging(true);
      }}
      onDragLeave={(e) => {
        if (!e.currentTarget.contains(e.relatedTarget as Node)) setDragging(false);
      }}
      onDrop={onDrop}
    >
      <header className="sticky top-0 z-10 flex flex-col gap-3 border-b border-line bg-canvas/90 px-4 pt-[max(env(safe-area-inset-top),16px)] pb-3 backdrop-blur-xl sm:grid sm:grid-cols-[minmax(0,1fr)_auto] sm:items-center sm:px-7 sm:py-4">
        <MobileTitle path={path} title={title} selectMode={selectMode} onToggleSelect={() => (selectMode ? clearSelection() : setSelectMode(true))} />
        <Breadcrumbs path={path} />
        <label className="flex h-10 items-center gap-2 rounded-[10px] bg-surface px-3 text-fg-subtle sm:h-9 sm:min-w-0 sm:rounded-lg sm:border sm:border-line-strong sm:bg-canvas">
          <Search size={16} strokeWidth={1.75} aria-hidden />
          <span className="sr-only">Search in {title}</span>
          <input
            ref={searchRef}
            type="search"
            placeholder="Search"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            className="min-w-0 flex-1 border-0 bg-transparent text-base text-fg outline-none placeholder:text-fg-subtle sm:text-[13px]"
          />
          <kbd className="hidden rounded border border-line-strong px-1.5 font-mono text-[11px] text-fg-subtle sm:inline">/</kbd>
        </label>
        <div className="hidden gap-2 sm:flex">
          <Button icon={<FolderPlus size={16} strokeWidth={1.75} aria-hidden />} onClick={() => setModal({ kind: "newFolder" })}>
            New folder
          </Button>
          <Menu
            label="Upload"
            trigger={<><Upload size={16} aria-hidden /><span>Upload</span><ChevronDown size={14} aria-hidden /></>}
            triggerClassName="!w-auto gap-2 border border-fg bg-fg px-3.5 !text-canvas hover:!bg-white"
            items={[
              { label: "Upload files", icon: <Upload size={16} />, onSelect: () => pickFiles() },
              { label: "Upload folder", icon: <FolderPlus size={16} />, onSelect: () => pickFiles(undefined, true) },
            ]}
          />
        </div>
      </header>

      {selected.size > 0 && (
        <SelectionBar
          count={selected.size}
          bytes={selectedBytes}
          onDownload={() => download(selected.size === 1 && selectedEntries[0]?.kind === "file" ? contentUrl(selectedEntries[0].path) : archiveUrl([...selected]))}
          onMove={() => setModal({ kind: "move", paths: [...selected] })}
          onTrash={() => moveToTrash([...selected])}
          onClear={clearSelection}
          busy={trash.isPending}
        />
      )}

      {preparing && <p role="status" className="px-4 text-sm text-fg-muted">Preparing folders…</p>}
      <section className="flex-1 sm:px-7 sm:pt-3" aria-busy={folder.isFetching || search.isFetching}>
        <Listing
          query={searching ? search : folder}
          entries={entries}
          searching={searching}
          empty={
            searching ? (
              <EmptyState icon={<Search size={20} />} title="No matches">
                Nothing in this folder is named like “{deferredQuery}”.
              </EmptyState>
            ) : (
              <EmptyState icon={<Upload size={20} />} title="This folder is empty">
                Drop files here or use Upload. Large files resume after interruptions.
              </EmptyState>
            )
          }
        >
          <FileList
            entries={entries}
            selected={selected}
            onToggle={toggle}
            onToggleAll={toggleAll}
            actions={actions}
            showLocation={searching}
            selectMode={selectMode}
            label={listLabel}
          />
        </Listing>
      </section>

      <div className="fixed right-4 bottom-[calc(env(safe-area-inset-bottom)+80px)] z-20 sm:hidden">
        <Menu
          label="Add to this folder"
          title={`Add to ${title}`}
          triggerClassName="!size-14 !rounded-2xl !bg-fg !text-canvas shadow-float"
          trigger={<Plus size={24} strokeWidth={2} aria-hidden />}
          items={[
            { label: "Upload folder", icon: <Upload size={18} />, onSelect: () => pickFiles(undefined, true) },
            { label: "Upload files", icon: <Upload size={18} />, onSelect: () => pickFiles() },
            { label: "Photos and videos", icon: <Image size={18} />, onSelect: () => pickFiles("image/*,video/*") },
            { label: "New folder", icon: <FolderPlus size={18} />, onSelect: () => setModal({ kind: "newFolder" }) },
          ]}
        />
      </div>

      {dragging && (
        <div aria-hidden className="pointer-events-none absolute inset-3 z-20 flex animate-fade-in items-center justify-center rounded-2xl border-2 border-dashed border-fg-muted bg-canvas/80 backdrop-blur-sm">
          <span className="flex items-center gap-3 text-[15px] text-fg">
            <Upload size={20} aria-hidden />
            Drop to upload to {title}
          </span>
        </div>
      )}

      <NewFolderDialog open={modal.kind === "newFolder"} onClose={() => setModal({ kind: "none" })} dir={path} />
      {modal.kind === "rename" && <RenameDialog key={modal.path} open onClose={() => setModal({ kind: "none" })} path={modal.path} />}
      {modal.kind === "move" && <MoveDialog open onClose={() => setModal({ kind: "none" })} paths={modal.paths} start={path} />}
      {modal.kind === "share" && <ShareDialog entry={modal.entry} onClose={() => setModal({ kind: "none" })} />}
      {modal.kind === "preview" && <PreviewDialog entry={modal.entry} onClose={() => setModal({ kind: "none" })} onShare={(entry) => setModal({ kind: "share", entry })} />}
    </div>
  );
}

function Listing({
  query,
  entries,
  searching,
  empty,
  children,
}: {
  query: { isPending: boolean; isError: boolean; error: unknown; refetch: () => unknown };
  entries: Entry[];
  searching: boolean;
  empty: ReactNode;
  children: ReactNode;
}) {
  if (query.isPending && (!searching || entries.length === 0)) return <Loading />;
  if (query.isError) return <ErrorState message={messageOf(query.error)} onRetry={() => void query.refetch()} />;
  if (entries.length === 0) return <>{empty}</>;
  return <>{children}</>;
}

function Breadcrumbs({ path }: { path: string }) {
  const parts = crumbs(path);
  return (
    <nav aria-label="Breadcrumb" className="hidden min-w-0 flex-1 sm:col-span-2 items-center gap-1.5 font-mono text-sm text-fg-subtle sm:flex">
      <Link to="/files" className="shrink-0 text-fg-subtle no-underline hover:text-fg" aria-current={parts.length === 0 ? "page" : undefined}>
        ~
      </Link>
      {parts.map((crumb, i) => (
        <span key={crumb.path} className="flex min-w-0 items-center gap-1.5">
          <span aria-hidden>/</span>
          {i === parts.length - 1 ? (
            <span aria-current="page" className="truncate text-fg">
              {crumb.name}
            </span>
          ) : (
            <Link to={filesHref(crumb.path)} className="truncate text-fg-subtle no-underline hover:text-fg">
              {crumb.name}
            </Link>
          )}
        </span>
      ))}
    </nav>
  );
}

function MobileTitle({ path, title, selectMode, onToggleSelect }: { path: string; title: string; selectMode: boolean; onToggleSelect: () => void }) {
  const parent = parentPath(path);
  return (
    <div className="flex flex-col gap-1 sm:hidden">
      <div className="flex min-h-11 items-center justify-between">
        {path ? (
          <Link to={filesHref(parent)} className="-ml-1 inline-flex min-h-11 items-center gap-1 font-mono text-sm text-fg-muted no-underline">
            <ChevronLeft size={18} aria-hidden />
            {baseName(parent) || "Files"}
          </Link>
        ) : (
          <samp className="font-mono text-xs text-fg-subtle">cloud:~$</samp>
        )}
        <IconButton label={selectMode ? "Done selecting" : "Select items"} size="touch" onClick={onToggleSelect} aria-pressed={selectMode}>
          {selectMode ? <X size={20} aria-hidden /> : <CircleCheck size={20} strokeWidth={1.75} aria-hidden />}
        </IconButton>
      </div>
      <h1 className="m-0 truncate text-[28px] leading-tight font-semibold tracking-[-0.01em]">{title}</h1>
    </div>
  );
}

function SelectionBar({ count, bytes, onDownload, onMove, onTrash, onClear, busy }: { count: number; bytes: number; onDownload: () => void; onMove: () => void; onTrash: () => void; onClear: () => void; busy: boolean }) {
  return (
    <div
      role="toolbar"
      aria-label="Selection"
      className={cn(
        "z-20 flex animate-rise items-center gap-1 border border-line-strong bg-surface/95 py-1.5 pr-1.5 pl-3.5 backdrop-blur-xl",
        "fixed inset-x-2 bottom-[calc(env(safe-area-inset-bottom)+68px)] rounded-2xl shadow-float",
        "sm:sticky sm:inset-x-auto sm:top-[101px] sm:bottom-auto sm:mx-7 sm:mt-4 sm:rounded-[10px] sm:shadow-none",
      )}
    >
      <span className="text-[13px] whitespace-nowrap">{count} selected</span>
      <span className="hidden font-mono text-xs text-fg-subtle sm:inline">· {formatBytes(bytes)}</span>
      <span className="flex-1" />
      <Button variant="ghost" size="sm" onClick={onDownload} icon={<Download size={16} aria-hidden />} className="max-sm:size-11 max-sm:px-0">
        <span className="max-sm:sr-only">Download</span>
      </Button>
      <Button variant="ghost" size="sm" onClick={onMove} icon={<ArrowRightLeft size={16} aria-hidden />} className="max-sm:size-11 max-sm:px-0">
        <span className="max-sm:sr-only">Move</span>
      </Button>
      <Button variant="ghost" size="sm" onClick={onTrash} busy={busy} icon={<Trash2 size={16} aria-hidden />} className="text-danger hover:text-danger max-sm:size-11 max-sm:px-0">
        <span className="max-sm:sr-only">Move to Trash</span>
      </Button>
      <IconButton label="Clear selection" size="sm" onClick={onClear} className="max-sm:size-11">
        <X size={16} aria-hidden />
      </IconButton>
    </div>
  );
}
