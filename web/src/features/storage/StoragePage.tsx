import { useQuery } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import { ChevronRight, Trash2 } from "lucide-react";
import type { ReactNode } from "react";
import { messageOf, type Storage } from "@/api/client";
import { settingsQuery, storageQuery } from "@/api/queries";
import { PageBar, Page } from "@/layout/PageBar";
import { cn } from "@/lib/cn";
import { formatBytes, percent } from "@/lib/format";
import { stagger } from "@/lib/motion";
import { displayPath, filesHref, parentPath } from "@/lib/paths";
import { ErrorState, Loading, PageHeader } from "@/ui/Feedback";
import { usageTone } from "@/ui/Field";

const categoryLabels: Record<Storage["byCategory"][number]["category"], string> = {
  video: "Video",
  image: "Images",
  audio: "Audio",
  document: "Documents",
  archive: "Archives",
  other: "Other",
};

const shades = ["bg-link", "bg-orange", "bg-purple", "bg-success", "bg-pink", "bg-teal"];

export function StoragePage() {
  const { data, isPending, isError, error, refetch } = useQuery(storageQuery);
  const { data: settings } = useQuery(settingsQuery);
  return (
    <>
      <PageBar prefix="03" title="Storage" actions={[{ label: "Open Trash", to: "/trash" }]} />
      <Page width="wide">
        {isPending && <Loading />}
        {isError && <ErrorState message={messageOf(error)} onRetry={() => void refetch()} />}
        {data && <Overview data={data} trashDays={settings?.trashRetentionDays} />}
      </Page>
    </>
  );
}

function Overview({ data, trashDays }: { data: Storage; trashDays?: number }) {
  const limit = data.quotaBytes > 0 ? data.quotaBytes : data.disk.totalBytes;
  const used = percent(data.usedBytes, limit);
  const tone = usageTone(used);
  const largestFolder = Math.max(...data.byFolder.map((f) => f.bytes), 1);

  return (
    <>
      <PageHeader eyebrow="03 / Storage" title={<>{formatBytes(data.usedBytes)} <span className="text-fg-subtle">of {formatBytes(limit)} used</span></>} />
      <div className="flex animate-reveal flex-col gap-5 [animation-delay:80ms]">
        <div role="meter" aria-label="Space used by category" aria-valuenow={used} aria-valuemin={0} aria-valuemax={100} className="flex h-2.5 gap-[3px] overflow-hidden rounded-sm bg-elevated">
          {data.byCategory.map((c, i) => (
            <div key={c.category} className={cn("h-full origin-left animate-grow", shades[i % shades.length])} style={{ width: `${(c.bytes / limit) * 100}%`, ...stagger(i, 200) }} />
          ))}
          {data.trashBytes > 0 && <div style={{ width: `${(data.trashBytes / limit) * 100}%` }} className="bg-[repeating-linear-gradient(45deg,var(--color-fg-faint)_0_3px,transparent_3px_6px)]" />}
        </div>
        <ul className="type-small m-0 flex list-none flex-wrap gap-x-6 gap-y-2 p-0 text-fg-muted">
          {data.byCategory.map((c, i) => (
            <li key={c.category} className="flex items-center gap-2">
              <span aria-hidden className={cn("size-2.5 rounded-[3px]", shades[i % shades.length])} />
              {categoryLabels[c.category]} <span className="text-fg-subtle">{formatBytes(c.bytes)}</span>
            </li>
          ))}
          {data.trashBytes > 0 && (
            <li className="flex items-center gap-2">
              <span aria-hidden className="size-2.5 rounded-[3px] bg-[repeating-linear-gradient(45deg,var(--color-fg-subtle)_0_2px,transparent_2px_4px)]" />
              Trash <span className="text-fg-subtle">{formatBytes(data.trashBytes)}</span>
            </li>
          )}
        </ul>
        {tone !== "default" && (
          <p role="status" className="type-small m-0 rounded-control border border-warning/30 bg-warning/10 px-4 py-3 text-fg">
            Storage is {used}% full. Empty the trash or remove large files to keep uploads working.
          </p>
        )}
      </div>

      <div className="grid animate-reveal grid-cols-2 gap-4 [animation-delay:160ms] lg:grid-cols-4">
        <Stat label="Available" value={formatBytes(data.availableBytes)} hint={data.quotaBytes > 0 ? "within your quota" : "no quota set"} />
        <Stat label="Disk free" value={formatBytes(data.disk.freeBytes)} hint={`of ${formatBytes(data.disk.totalBytes)} on the server`} />
        <Stat label="Files" value={data.fileCount.toLocaleString("en")} hint={data.reservedBytes > 0 ? `${formatBytes(data.reservedBytes)} reserved by uploads` : "stored in total"} />
        <Link to="/trash" className="card group flex flex-col gap-2 p-7 text-fg no-underline transition-colors duration-200 ease-swift hover:bg-raised max-sm:p-5">
          <span className="type-small flex items-center gap-2 text-fg-subtle">
            <Trash2 size={14} aria-hidden />
            Trash
          </span>
          <span className="type-section flex items-center justify-between">
            {formatBytes(data.trashBytes)}
            <ChevronRight size={20} className="text-fg-subtle transition-transform duration-280 ease-swift group-hover:translate-x-0.5" aria-hidden />
          </span>
          <span className="type-micro text-fg-subtle">{trashDays ? `cleared after ${trashDays} days` : "deleted items"}</span>
        </Link>
      </div>

      <div className="grid animate-reveal grid-cols-[repeat(auto-fit,minmax(min(100%,400px),1fr))] gap-4 [animation-delay:240ms]">
        <Card title="By folder">
          {data.byFolder.length === 0 && <p className="type-small m-0 text-fg-subtle">Nothing stored yet.</p>}
          {data.byFolder.slice(0, 8).map((f) => (
            <div key={f.name || "~"} className="type-small grid grid-cols-[minmax(0,140px)_minmax(0,1fr)_80px] items-center gap-4">
              <Link to={filesHref(f.name)} className="truncate text-fg no-underline transition-colors hover:text-link">
                {f.name || "Root files"}
              </Link>
              <div className="h-1.5 overflow-hidden rounded-sm bg-elevated">
                <div className="h-full origin-left animate-grow rounded-sm bg-link" style={{ width: `${Math.max(2, (f.bytes / largestFolder) * 100)}%` }} />
              </div>
              <span className="text-right text-fg-subtle tabular-nums">{formatBytes(f.bytes)}</span>
            </div>
          ))}
        </Card>
        <Card title="Largest files">
          {data.largest.length === 0 && <p className="type-small m-0 text-fg-subtle">Nothing stored yet.</p>}
          <ul className="m-0 flex list-none flex-col p-0">
            {data.largest.map((f) => (
              <li key={f.path} className="type-small flex min-h-11 items-center gap-3 border-b border-line last:border-b-0">
                <span className="min-w-0 flex-1 truncate">{f.name}</span>
                <Link to={filesHref(parentPath(f.path))} className="hidden max-w-[40%] truncate text-fg-subtle no-underline transition-colors hover:text-link sm:inline">
                  {displayPath(parentPath(f.path))}
                </Link>
                <span className="w-20 text-right text-fg-subtle tabular-nums">{formatBytes(f.size)}</span>
              </li>
            ))}
          </ul>
        </Card>
      </div>
    </>
  );
}

function Stat({ label, value, hint }: { label: string; value: string; hint: string }) {
  return (
    <div className="card flex flex-col gap-2 p-7 max-sm:p-5">
      <span className="type-small text-fg-subtle">{label}</span>
      <span className="type-section">{value}</span>
      <span className="type-micro text-fg-subtle">{hint}</span>
    </div>
  );
}

function Card({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section className="card flex flex-col gap-5 p-7 max-sm:p-5" aria-label={title}>
      <h2 className="type-title m-0">{title}</h2>
      {children}
    </section>
  );
}
