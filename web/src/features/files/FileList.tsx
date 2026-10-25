import { Link } from "@tanstack/react-router";
import { ArrowRightLeft, Clock, Download, Eye, FolderOpen, Link2, MoreHorizontal, Pencil, Trash2 } from "lucide-react";
import type { Entry } from "@/api/client";
import { cn } from "@/lib/cn";
import { formatBytes, formatDate, formatRemaining } from "@/lib/format";
import { archiveUrl, contentUrl, displayPath, filesHref, parentPath } from "@/lib/paths";
import { Menu, type MenuItem } from "@/ui/Menu";
import { EntryIcon, previewKind } from "./entryKind";

export type EntryActions = {
  open: (entry: Entry) => void;
  share: (entry: Entry) => void;
  rename: (entry: Entry) => void;
  move: (paths: string[]) => void;
  trash: (paths: string[]) => void;
};

type FileListProps = {
  entries: Entry[];
  selected: Set<string>;
  onToggle: (path: string) => void;
  onToggleAll: () => void;
  actions: EntryActions;
  showLocation?: boolean;
  selectMode: boolean;
  label: string;
};

const iconProps = { size: 16, strokeWidth: 1.75 } as const;

export function FileList({ entries, selected, onToggle, onToggleAll, actions, showLocation, selectMode, label }: FileListProps) {
  const allSelected = entries.length > 0 && entries.every((e) => selected.has(e.path));
  return (
    <div role="table" aria-label={label} className="flex flex-col">
      <div role="row" className="label-caps hidden h-9 grid-cols-[40px_minmax(0,1fr)_110px_150px_48px] items-center border-b border-line px-3 sm:grid">
        <span role="columnheader">
          <input type="checkbox" aria-label="Select all" checked={allSelected} onChange={onToggleAll} className="size-4 cursor-pointer" />
        </span>
        <span role="columnheader">Name</span>
        <span role="columnheader" className="text-right">
          Size
        </span>
        <span role="columnheader" className="text-right">
          Modified
        </span>
        <span role="columnheader" className="sr-only">
          Actions
        </span>
      </div>
      {entries.map((entry) => (
        <Row key={entry.path} entry={entry} selected={selected.has(entry.path)} onToggle={onToggle} actions={actions} showLocation={showLocation} selectMode={selectMode} />
      ))}
    </div>
  );
}

type RowProps = { entry: Entry; selected: boolean; onToggle: (path: string) => void; actions: EntryActions; showLocation?: boolean; selectMode: boolean };

function Row({ entry, selected, onToggle, actions, showLocation, selectMode }: RowProps) {
  const isDir = entry.kind === "dir";
  const meta = isDir ? formatDate(entry.modifiedAt) : `${formatBytes(entry.size)} · ${formatDate(entry.modifiedAt)}`;
  return (
    <div
      role="row"
      aria-selected={selected}
      className={cn(
        "group grid min-h-[60px] items-center gap-x-1 border-b border-line pr-1 pl-4 transition-colors sm:min-h-12 sm:grid-cols-[40px_minmax(0,1fr)_110px_150px_48px] sm:gap-x-0 sm:px-3",
        selectMode ? "grid-cols-[36px_minmax(0,1fr)_44px]" : "grid-cols-[minmax(0,1fr)_44px]",
        selected ? "bg-selected" : "hover:bg-sunken",
      )}
    >
      <span role="cell" className={cn(selectMode ? "flex" : "hidden", "sm:flex")}>
        <input type="checkbox" aria-label={`Select ${entry.name}`} checked={selected} onChange={() => onToggle(entry.path)} className="size-5 cursor-pointer sm:size-4" />
      </span>
      <span role="cell" className="flex min-w-0 items-center gap-3.5 sm:gap-3">
        <span className="flex shrink-0 sm:hidden">
          <EntryIcon entry={entry} size={22} />
        </span>
        <span className="hidden shrink-0 sm:flex">
          <EntryIcon entry={entry} />
        </span>
        <span className="flex min-w-0 flex-col gap-0.5 sm:flex-row sm:items-center sm:gap-2">
          <EntryName entry={entry} onOpen={actions.open} />
          <span className="font-mono text-xs text-fg-subtle sm:hidden">
            {showLocation ? displayPath(parentPath(entry.path)) : meta}
            {entry.link && ` · link ${formatRemaining(entry.link.expiresAt)}`}
          </span>
          {showLocation && <span className="hidden truncate font-mono text-xs text-fg-subtle sm:inline">{displayPath(parentPath(entry.path))}</span>}
          {entry.link && <LinkBadge expiresAt={entry.link.expiresAt} />}
        </span>
      </span>
      <span role="cell" className="hidden text-right font-mono text-[13px] text-fg-subtle sm:block">
        {isDir ? "—" : formatBytes(entry.size)}
      </span>
      <span role="cell" className="hidden text-right font-mono text-[13px] text-fg-subtle sm:block">
        {formatDate(entry.modifiedAt)}
      </span>
      <span role="cell" className="flex justify-end">
        <Menu label={`Actions for ${entry.name}`} title={entry.name} trigger={<MoreHorizontal size={18} aria-hidden />} items={menuItems(entry, actions)} />
      </span>
    </div>
  );
}

function EntryName({ entry, onOpen }: { entry: Entry; onOpen: (entry: Entry) => void }) {
  const className = "min-w-0 truncate text-left text-[15px] text-fg no-underline hover:underline underline-offset-4 decoration-fg-faint sm:text-sm cursor-pointer";
  if (entry.kind === "dir") {
    return (
      <Link to={filesHref(entry.path)} className={className}>
        {entry.name}
      </Link>
    );
  }
  return (
    <button type="button" onClick={() => onOpen(entry)} className={cn(className, "border-0 bg-transparent p-0")}>
      {entry.name}
    </button>
  );
}

function LinkBadge({ expiresAt }: { expiresAt: string }) {
  return (
    <span title="Temporary link active" className="hidden shrink-0 items-center gap-1 rounded-full border border-line-strong px-2 py-0.5 font-mono text-[11px] text-fg-muted sm:inline-flex">
      <Clock size={11} strokeWidth={2} aria-hidden />
      link · {formatRemaining(expiresAt)}
    </span>
  );
}

function menuItems(entry: Entry, actions: EntryActions): MenuItem[] {
  const isDir = entry.kind === "dir";
  return [
    { label: isDir ? "Open" : "Preview", icon: isDir ? <FolderOpen {...iconProps} /> : <Eye {...iconProps} />, onSelect: () => actions.open(entry), hidden: !isDir && !previewKind(entry) },
    { label: isDir ? "Download as ZIP" : "Download", icon: <Download {...iconProps} />, onSelect: () => download(isDir ? archiveUrl([entry.path]) : contentUrl(entry.path)) },
    { label: "Temporary link", icon: <Link2 {...iconProps} />, onSelect: () => actions.share(entry) },
    { label: "Rename", icon: <Pencil {...iconProps} />, onSelect: () => actions.rename(entry) },
    { label: "Move", icon: <ArrowRightLeft {...iconProps} />, onSelect: () => actions.move([entry.path]) },
    { label: "Move to Trash", icon: <Trash2 {...iconProps} />, onSelect: () => actions.trash([entry.path]), danger: true },
  ];
}

export function download(url: string): void {
  const anchor = document.createElement("a");
  anchor.href = url;
  anchor.rel = "noopener";
  anchor.download = "";
  document.body.append(anchor);
  anchor.click();
  anchor.remove();
}
