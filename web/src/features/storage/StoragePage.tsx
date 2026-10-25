import { useQuery } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import { ChevronRight, Trash2 } from "lucide-react";
import type { ReactNode } from "react";
import { messageOf, type Storage } from "@/api/client";
import { settingsQuery, storageQuery } from "@/api/queries";
import { formatBytes, percent } from "@/lib/format";
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

const shades = ["#e6edf3", "#a6adb8", "#7d8590", "#5b636d", "#484f58", "#30363d"];

export function StoragePage() {
  const { data, isPending, isError, error, refetch } = useQuery(storageQuery);
  const { data: settings } = useQuery(settingsQuery);
  return (
    <div className="flex w-full max-w-[1120px] flex-col gap-6 px-4 pt-[max(env(safe-area-inset-top),24px)] pb-10 sm:px-7 sm:pt-8">
      {isPending && <Loading />}
      {isError && <ErrorState message={messageOf(error)} onRetry={() => void refetch()} />}
      {data && <Overview data={data} trashDays={settings?.trashRetentionDays} />}
    </div>
  );
}

function Overview({ data, trashDays }: { data: Storage; trashDays?: number }) {
  const limit = data.quotaBytes > 0 ? data.quotaBytes : data.disk.totalBytes;
  const used = percent(data.usedBytes, limit);
  const tone = usageTone(used);
  const largestFolder = Math.max(...data.byFolder.map((f) => f.bytes), 1);

  return (
    <>
      <PageHeader
        eyebrow="01 / Storage"
        title={
          <>
            {formatBytes(data.usedBytes)} <span className="font-normal text-fg-subtle">of {formatBytes(limit)} used</span>
          </>
        }
      />
      <div role="meter" aria-label="Space used by category" aria-valuenow={used} aria-valuemin={0} aria-valuemax={100} className="flex h-2.5 gap-0.5 overflow-hidden rounded-full bg-line">
        {data.byCategory.map((c, i) => (
          <div key={c.category} style={{ width: `${(c.bytes / limit) * 100}%`, background: shades[i % shades.length] }} />
        ))}
        {data.trashBytes > 0 && <div style={{ width: `${(data.trashBytes / limit) * 100}%` }} className="bg-[repeating-linear-gradient(45deg,#484f58_0_3px,transparent_3px_6px)]" />}
      </div>
      <ul className="m-0 flex list-none flex-wrap gap-x-5 gap-y-2 p-0 text-[13px] text-fg-muted">
        {data.byCategory.map((c, i) => (
          <li key={c.category} className="flex items-center gap-2">
            <span aria-hidden className="size-2 rounded-[2px]" style={{ background: shades[i % shades.length] }} />
            {categoryLabels[c.category]} <span className="font-mono text-fg-subtle">{formatBytes(c.bytes)}</span>
          </li>
        ))}
        {data.trashBytes > 0 && (
          <li className="flex items-center gap-2">
            <span aria-hidden className="size-2 rounded-[2px] bg-[repeating-linear-gradient(45deg,#7d8590_0_2px,transparent_2px_4px)]" />
            Trash <span className="font-mono text-fg-subtle">{formatBytes(data.trashBytes)}</span>
          </li>
        )}
      </ul>
      {tone !== "default" && (
        <p role="status" className="m-0 rounded-lg border border-warning/40 bg-warning/10 px-3.5 py-2.5 text-[13px] text-fg">
          Storage is {used}% full. Empty the trash or remove large files to keep uploads working.
        </p>
      )}

      <div className="grid grid-cols-2 gap-3 sm:gap-4 lg:grid-cols-4">
        <Stat label="Available" value={formatBytes(data.availableBytes)} hint={data.quotaBytes > 0 ? "within your quota" : "no quota set"} />
        <Stat label="Disk free" value={formatBytes(data.disk.freeBytes)} hint={`of ${formatBytes(data.disk.totalBytes)} on the server`} />
        <Stat label="Files" value={data.fileCount.toLocaleString("en")} hint={data.reservedBytes > 0 ? `${formatBytes(data.reservedBytes)} reserved by uploads` : "stored in total"} />
        <Link to="/trash" className="flex flex-col gap-1.5 rounded-xl border border-line bg-sunken p-4 text-fg no-underline transition-colors hover:border-line-strong sm:p-5">
          <span className="label-caps flex items-center gap-2">
            <Trash2 size={12} aria-hidden />
            Trash
          </span>
          <span className="flex items-center justify-between font-mono text-lg sm:text-[22px]">
            {formatBytes(data.trashBytes)}
            <ChevronRight size={16} className="text-fg-subtle" aria-hidden />
          </span>
          <span className="text-[13px] text-fg-subtle">{trashDays ? `cleared after ${trashDays} days` : "deleted items"}</span>
        </Link>
      </div>

      <div className="grid grid-cols-[repeat(auto-fit,minmax(min(100%,380px),1fr))] gap-4">
        <Card title="02 / By folder">
          {data.byFolder.length === 0 && <p className="m-0 text-[13px] text-fg-subtle">Nothing stored yet.</p>}
          {data.byFolder.slice(0, 8).map((f) => (
            <div key={f.name || "~"} className="grid grid-cols-[minmax(0,140px)_minmax(0,1fr)_80px] items-center gap-3.5 text-sm">
              <Link to={filesHref(f.name)} className="truncate font-mono text-fg no-underline hover:underline">
                {f.name || "~ (root files)"}
              </Link>
              <div className="h-1.5 overflow-hidden rounded-full bg-line">
                <div className="h-full rounded-full bg-fg-muted" style={{ width: `${Math.max(2, (f.bytes / largestFolder) * 100)}%` }} />
              </div>
              <span className="text-right font-mono text-[13px] text-fg-muted">{formatBytes(f.bytes)}</span>
            </div>
          ))}
        </Card>
        <Card title="03 / Largest files">
          {data.largest.length === 0 && <p className="m-0 text-[13px] text-fg-subtle">Nothing stored yet.</p>}
          <ul className="m-0 flex list-none flex-col p-0">
            {data.largest.map((f) => (
              <li key={f.path} className="flex min-h-10 items-center gap-3 border-b border-line text-sm last:border-b-0">
                <span className="min-w-0 flex-1 truncate">{f.name}</span>
                <Link to={filesHref(parentPath(f.path))} className="hidden max-w-[40%] truncate font-mono text-xs text-fg-subtle no-underline hover:text-fg sm:inline">
                  {displayPath(parentPath(f.path))}
                </Link>
                <span className="w-20 text-right font-mono text-[13px] text-fg-muted">{formatBytes(f.size)}</span>
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
    <div className="flex flex-col gap-1.5 rounded-xl border border-line bg-sunken p-4 sm:p-5">
      <span className="label-caps">{label}</span>
      <span className="font-mono text-lg sm:text-[22px]">{value}</span>
      <span className="text-[13px] text-fg-subtle">{hint}</span>
    </div>
  );
}

function Card({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section className="flex flex-col gap-4 rounded-xl border border-line bg-sunken p-5" aria-label={title.replace(/^\d+ \/ /, "")}>
      <h2 className="label-caps m-0 font-normal">{title}</h2>
      {children}
    </section>
  );
}
