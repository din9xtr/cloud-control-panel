import { Link } from "@tanstack/react-router";
import { ArrowRightLeft, Clock, Download, Eye, FolderOpen, Link2, MoreHorizontal, Pencil, Trash2 } from "lucide-react";
import type { Entry } from "@/api/client";
import { cn } from "@/lib/cn";
import { formatBytes, formatDate, formatRemaining } from "@/lib/format";
import { stagger } from "@/lib/motion";
import { archiveUrl, contentUrl, displayPath, filesHref, parentPath } from "@/lib/paths";
import { Badge } from "@/ui/Feedback";
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
    <div role="table" aria-label={label} className="flex animate-reveal flex-col overflow-hidden rounded-card border border-line bg-surface [animation-delay:80ms] max-sm:-mx-4 max-sm:rounded-none max-sm:border-x-0 max-sm:bg-transparent">
      <div role="row" className="label-caps hidden h-12 grid-cols-[44px_minmax(0,1fr)_120px_170px_52px] items-center border-b border-line-strong px-5 sm:grid">
        <span role="columnheader" className="flex">
          <input type="checkbox" aria-label="Select all" checked={allSelected} onChange={onToggleAll} className="check" />
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
      {entries.map((entry, index) => (
        <Row key={entry.path} index={index} entry={entry} selected={selected.has(entry.path)} onToggle={onToggle} actions={actions} showLocation={showLocation} selectMode={selectMode} />
      ))}
    </div>
  );
}

type RowProps = { index: number; entry: Entry; selected: boolean; onToggle: (path: string) => void; actions: EntryActions; showLocation?: boolean; selectMode: boolean };

function Row({ index, entry, selected, onToggle, actions, showLocation, selectMode }: RowProps) {
  const isDir = entry.kind === "dir";
  const meta = isDir ? formatDate(entry.modifiedAt) : `${formatBytes(entry.size)} · ${formatDate(entry.modifiedAt)}`;
  return (
    <div
      role="row"
      aria-selected={selected}
      style={index < 24 ? stagger(index, 120) : undefined}
      className={cn(
        "group relative grid min-h-16 animate-fade-in items-center gap-x-1 pr-1 pl-4 transition-colors duration-200 ease-swift sm:min-h-14 sm:grid-cols-[44px_minmax(0,1fr)_120px_170px_52px] sm:gap-x-0 sm:px-5",
        "after:absolute after:right-0 after:bottom-0 after:left-[52px] after:h-px after:bg-line last:after:hidden sm:after:left-[64px]",
        selectMode ? "grid-cols-[40px_minmax(0,1fr)_44px]" : "grid-cols-[minmax(0,1fr)_44px]",
        selected ? "bg-selected" : "hover:bg-raised/60",
      )}
    >
      <span role="cell" className={cn(selectMode ? "flex" : "hidden", "sm:flex")}>
        <input type="checkbox" aria-label={`Select ${entry.name}`} checked={selected} onChange={() => onToggle(entry.path)} className="check" />
      </span>
      <span role="cell" className="flex min-w-0 items-center gap-3.5 sm:gap-3">
        <span className="flex shrink-0 sm:hidden">
          <EntryIcon entry={entry} size={24} />
        </span>
        <span className="hidden shrink-0 sm:flex">
          <EntryIcon entry={entry} size={22} />
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
  const className = "type-body min-w-0 cursor-pointer truncate text-left text-fg no-underline decoration-fg-faint underline-offset-4 transition-colors duration-200 hover:text-link";
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
    <span title="Temporary link active" className="hidden sm:inline-flex">
      <Badge tone="info">
        <Clock size={11} strokeWidth={2.5} aria-hidden />
        link · {formatRemaining(expiresAt)}
      </Badge>
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
