import { Check, ChevronDown, CircleAlert, Pause, Play, RotateCw, X } from "lucide-react";
import { useState } from "react";
import { cn } from "@/lib/cn";
import { formatBytes, formatSpeed, percent } from "@/lib/format";
import { IconButton } from "@/ui/Button";
import type { UploadItem } from "./UploadQueue";
import { useUploadQueue, useUploads } from "./UploadsProvider";

export function UploadTray() {
  const items = useUploads();
  const queue = useUploadQueue();
  const [collapsed, setCollapsed] = useState(false);
  if (items.length === 0) return null;

  const done = items.filter((i) => i.status === "done").length;
  const active = items.some((i) => i.status === "uploading" || i.status === "queued");
  const total = items.reduce((sum, i) => sum + i.size, 0);
  const sent = items.reduce((sum, i) => sum + i.uploaded, 0);
  const heading = active ? `Uploading · ${done} of ${items.length}` : `Uploaded · ${done} of ${items.length}`;

  return (
    <section
      aria-label="Uploads"
      className={cn(
        "fixed z-30 animate-rise overflow-hidden rounded-card border border-line-strong bg-raised shadow-float",
        "inset-x-3 bottom-[calc(env(safe-area-inset-bottom)+84px)]",
        "lg:inset-x-auto lg:right-6 lg:bottom-6 lg:w-[380px]",
      )}
    >
      <header className="flex items-center gap-2 border-b border-line-strong py-2 pr-2 pl-4">
        <span className="type-small flex-1 font-semibold">{heading}</span>
        {active ? (
          <button type="button" onClick={() => queue.pauseAll()} className="type-small min-h-9 cursor-pointer rounded-lg px-3 text-link transition-colors hover:bg-elevated">
            Pause all
          </button>
        ) : (
          <button type="button" onClick={() => queue.clearFinished()} className="type-small min-h-9 cursor-pointer rounded-lg px-3 text-link transition-colors hover:bg-elevated">
            Clear
          </button>
        )}
        <IconButton label={collapsed ? "Expand uploads" : "Collapse uploads"} size="sm" onClick={() => setCollapsed((v) => !v)} aria-expanded={!collapsed}>
          <ChevronDown size={16} className={cn("transition-transform duration-280 ease-swift", collapsed && "rotate-180")} aria-hidden />
        </IconButton>
      </header>
      {collapsed ? (
        <div className="px-4 py-3">
          <Progress value={percent(sent, total)} label="Total upload progress" />
        </div>
      ) : (
        <ul className="m-0 max-h-[40dvh] list-none overflow-y-auto p-0">
          {items.map((item) => (
            <UploadRow key={item.id} item={item} />
          ))}
        </ul>
      )}
    </section>
  );
}

function UploadRow({ item }: { item: UploadItem }) {
  const queue = useUploadQueue();
  const value = percent(item.uploaded, item.size);
  return (
    <li className="flex animate-fade-in flex-col gap-2 border-b border-line px-4 py-3 last:border-b-0">
      <div className="flex items-center gap-2">
        <StatusIcon item={item} />
        <span className="type-small min-w-0 flex-1 truncate" title={item.name}>
          {item.name}
        </span>
        <span className="type-micro text-fg-subtle tabular-nums">{summary(item, value)}</span>
        <Controls item={item} onPause={() => queue.pause(item.id)} onResume={() => queue.resume(item.id)} onCancel={() => queue.cancel(item.id)} />
      </div>
      {(item.status === "uploading" || item.status === "paused") && <Progress value={value} label={item.name} />}
      {item.status === "error" && <p className="type-micro m-0 text-danger">{item.error}</p>}
    </li>
  );
}

function summary(item: UploadItem, value: number): string {
  switch (item.status) {
    case "queued":
      return "queued";
    case "paused":
      return `paused · ${value}%`;
    case "done":
      return formatBytes(item.size);
    case "error":
      return `${value}%`;
    default:
      return item.speed > 0 ? `${value}% · ${formatSpeed(item.speed)}` : `${value}%`;
  }
}

function StatusIcon({ item }: { item: UploadItem }) {
  if (item.status === "done") return <Check size={14} strokeWidth={2.5} className="text-success" aria-label="Done" />;
  if (item.status === "error") return <CircleAlert size={14} className="text-danger" aria-label="Failed" />;
  return null;
}

function Controls({ item, onPause, onResume, onCancel }: { item: UploadItem; onPause: () => void; onResume: () => void; onCancel: () => void }) {
  return (
    <div className="flex">
      {(item.status === "uploading" || item.status === "queued") && (
        <IconButton label={`Pause ${item.name}`} size="sm" onClick={onPause}>
          <Pause size={14} aria-hidden />
        </IconButton>
      )}
      {item.status === "paused" && (
        <IconButton label={`Resume ${item.name}`} size="sm" onClick={onResume}>
          <Play size={14} aria-hidden />
        </IconButton>
      )}
      {item.status === "error" && (
        <IconButton label={`Retry ${item.name}`} size="sm" onClick={onResume}>
          <RotateCw size={14} aria-hidden />
        </IconButton>
      )}
      {item.status !== "done" && (
        <IconButton label={`Cancel ${item.name}`} size="sm" onClick={onCancel}>
          <X size={14} aria-hidden />
        </IconButton>
      )}
    </div>
  );
}

function Progress({ value, label }: { value: number; label: string }) {
  return (
    <div role="progressbar" aria-label={label} aria-valuenow={value} aria-valuemin={0} aria-valuemax={100} className="h-1 overflow-hidden rounded-sm bg-elevated">
      <div className="h-full rounded-sm bg-link transition-[width] duration-300 ease-swift" style={{ width: `${value}%` }} />
    </div>
  );
}
