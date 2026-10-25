import { useQuery } from "@tanstack/react-query";
import { Link, useNavigate } from "@tanstack/react-router";
import { ArrowRightLeft, Download, FolderPlus, FolderUp, Image, Search, Trash2, Upload, X } from "lucide-react";
import { type DragEvent, type ReactNode, useDeferredValue, useEffect, useRef, useState } from "react";
import { type Entry, messageOf } from "@/api/client";
import { folderQuery, searchQuery, useTrash } from "@/api/queries";
import { ShareDialog } from "@/features/links/ShareDialog";
import { droppedFiles, enqueueSelection, selectedFiles } from "@/features/uploads/folderUpload";
import { useUploadQueue } from "@/features/uploads/UploadsProvider";
import { PageBar, PrimaryAction } from "@/layout/PageBar";
import { formatBytes } from "@/lib/format";
import { archiveUrl, baseName, contentUrl, crumbs, filesHref } from "@/lib/paths";
import { Button, IconButton } from "@/ui/Button";
import { EmptyState, ErrorState, Loading } from "@/ui/Feedback";
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
  const navigate = useNavigate();

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
    open: (entry) => {
      if (entry.kind === "dir") void navigate({ to: filesHref(entry.path) });
      else if (previewKind(entry)) setModal({ kind: "preview", entry });
      else download(contentUrl(entry.path));
    },
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
  const folderEntries = folder.data?.entries ?? [];
  const folderBytes = folderEntries.reduce((sum, e) => sum + e.size, 0);
  const summary = folder.data ? `${folderEntries.length} ${folderEntries.length === 1 ? "item" : "items"}${folderBytes > 0 ? ` · ${formatBytes(folderBytes)}` : ""}` : " ";

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
      <PageBar
        title={<Breadcrumbs path={path} />}
        actions={[
          { label: "New folder", icon: <FolderPlus size={15} aria-hidden />, onSelect: () => setModal({ kind: "newFolder" }) },
          { label: "Upload folder", icon: <FolderUp size={15} aria-hidden />, onSelect: () => pickFiles(undefined, true) },
          { label: "Photos and videos", icon: <Image size={15} aria-hidden />, onSelect: () => pickFiles("image/*,video/*") },
        ]}
        primary={<PrimaryAction icon={<Upload size={14} aria-hidden />} onClick={() => pickFiles()}>Upload</PrimaryAction>}
      />

      <div className="mx-auto flex w-full max-w-[1120px] flex-col gap-8 px-6 pt-10 pb-10 max-md:gap-6 max-md:px-4 max-md:pt-6">
        <div className="flex animate-reveal flex-col gap-5">
          <div className="flex items-end justify-between gap-4">
            <div className="flex min-w-0 flex-col gap-2">
              <h1 className="type-page m-0 truncate">{title}</h1>
              <p className="m-0 font-mono text-[13px] text-fg-subtle">{summary}</p>
            </div>
            <button
              type="button"
              aria-pressed={selectMode}
              onClick={() => (selectMode ? clearSelection() : setSelectMode(true))}
              className="type-small shrink-0 cursor-pointer pb-1 font-medium text-link sm:hidden"
            >
              {selectMode ? "Done" : "Select"}
            </button>
          </div>
          <label className="group flex h-11 items-center gap-2.5 rounded-control border border-line-strong bg-sunken px-3.5 text-fg-subtle transition-shadow duration-200 ease-swift focus-within:shadow-[0_0_0_3px_rgb(0_125_250/0.6)] sm:max-w-[420px]">
            <Search size={17} strokeWidth={2} aria-hidden />
            <span className="sr-only">Search in {title}</span>
            <input
              ref={searchRef}
              type="search"
              placeholder={`Search in ${title}`}
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              className="type-body min-w-0 flex-1 border-0 bg-transparent text-fg outline-none placeholder:text-fg-subtle"
            />
            {query ? (
              <button type="button" aria-label="Clear search" onClick={() => setQuery("")} className="inline-flex size-5 cursor-pointer items-center justify-center rounded-md bg-elevated text-fg-muted">
                <X size={12} strokeWidth={3} aria-hidden />
              </button>
            ) : (
              <kbd className="type-micro hidden rounded-md border border-line-strong px-1.5 font-sans text-fg-subtle sm:inline">/</kbd>
            )}
          </label>
        </div>

        {preparing && <p role="status" className="type-small m-0 text-fg-muted">Preparing folders…</p>}
        <section className="flex-1" aria-busy={folder.isFetching || search.isFetching}>
          <Listing
            query={searching ? search : folder}
            entries={entries}
            searching={searching}
            empty={
              searching ? (
                <EmptyState icon={<Search size={24} />} title="No matches">
                  Nothing in this folder is named like “{deferredQuery}”.
                </EmptyState>
              ) : (
                <EmptyState icon={<Upload size={24} />} title="This folder is empty">
                  <span className="flex flex-col items-center gap-6">
                    Drop files here or use Upload. Large files resume after interruptions.
                    <Button variant="primary" icon={<Upload size={15} aria-hidden />} onClick={() => pickFiles()}>Upload files</Button>
                  </span>
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
      </div>

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

      {dragging && (
        <div aria-hidden className="pointer-events-none fixed inset-3 z-40 flex animate-fade-in items-center justify-center rounded-sheet border-2 border-dashed border-link bg-black/70 backdrop-blur-[10px]">
          <span className="type-section flex animate-rise flex-col items-center gap-4 text-center">
            <Upload size={36} aria-hidden className="text-link" />
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
  const link = "truncate text-fg-subtle no-underline transition-colors duration-200 hover:text-link";
  return (
    <nav aria-label="Breadcrumb" className="flex min-w-0 flex-1 items-center gap-1.5 font-mono text-[13px]">
      {parts.length === 0 ? (
        <span aria-current="page" className="text-fg-secondary">~</span>
      ) : (
        <Link to="/files" className={link}>
          ~
        </Link>
      )}
      {parts.map((crumb, i) => (
        <span key={crumb.path} className="flex min-w-0 items-center gap-1.5">
          <span aria-hidden className="text-fg-faint">/</span>
          {i === parts.length - 1 ? (
            <span aria-current="page" className="truncate font-sans text-sm font-semibold text-fg">
              {crumb.name}
            </span>
          ) : (
            <Link to={filesHref(crumb.path)} className={link}>
              {crumb.name}
            </Link>
          )}
        </span>
      ))}
    </nav>
  );
}

function SelectionBar({ count, bytes, onDownload, onMove, onTrash, onClear, busy }: { count: number; bytes: number; onDownload: () => void; onMove: () => void; onTrash: () => void; onClear: () => void; busy: boolean }) {
  return (
    <div className="pointer-events-none fixed inset-x-0 bottom-[calc(env(safe-area-inset-bottom)+16px)] z-30 flex justify-center px-3 sm:bottom-8">
      <div
        role="toolbar"
        aria-label="Selection"
        className="pointer-events-auto flex w-full max-w-[560px] animate-rise items-center gap-1 rounded-card border border-line-strong bg-raised py-1.5 pr-1.5 pl-5 shadow-float"
      >
        <span className="type-small font-semibold whitespace-nowrap">{count} selected</span>
        <span className="type-small hidden text-fg-subtle sm:inline">· {formatBytes(bytes)}</span>
        <span className="flex-1" />
        <Button variant="ghost" size="md" onClick={onDownload} icon={<Download size={16} aria-hidden />} className="max-sm:size-11 max-sm:px-0">
          <span className="max-sm:sr-only">Download</span>
        </Button>
        <Button variant="ghost" size="md" onClick={onMove} icon={<ArrowRightLeft size={16} aria-hidden />} className="max-sm:size-11 max-sm:px-0">
          <span className="max-sm:sr-only">Move</span>
        </Button>
        <Button variant="ghost" size="md" onClick={onTrash} busy={busy} icon={<Trash2 size={16} aria-hidden />} className="!text-danger hover:!bg-danger/12 max-sm:size-11 max-sm:px-0">
          <span className="max-sm:sr-only">Move to Trash</span>
        </Button>
        <IconButton label="Clear selection" onClick={onClear} className="max-sm:size-11">
          <X size={16} aria-hidden />
        </IconButton>
      </div>
    </div>
  );
}
