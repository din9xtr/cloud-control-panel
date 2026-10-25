import { useQuery } from "@tanstack/react-query";
import { Check, Clock, Copy, Share } from "lucide-react";
import { useState } from "react";
import { type Entry, type Link, messageOf } from "@/api/client";
import { linksQuery, settingsQuery, useCreateLink, useRevokeLink } from "@/api/queries";
import { cn } from "@/lib/cn";
import { formatBytes, formatDate, formatRemaining } from "@/lib/format";
import { displayPath, parentPath } from "@/lib/paths";
import { Button } from "@/ui/Button";
import { Dialog } from "@/ui/Dialog";
import { control } from "@/ui/Field";
import { useToast } from "@/ui/Toast";

const hour = 3600;
const lifetimes = [
  { seconds: hour, label: "1 hour" },
  { seconds: 24 * hour, label: "24 hours" },
  { seconds: 7 * 24 * hour, label: "7 days" },
];

export function absoluteUrl(link: Pick<Link, "url">): string {
  return new URL(link.url, window.location.origin).toString();
}

export function ShareDialog({ entry, onClose }: { entry: Entry; onClose: () => void }) {
  const { data: settings } = useQuery(settingsQuery);
  const { data: links = [] } = useQuery(linksQuery);
  const [mode, setMode] = useState<Link["mode"]>("download");
  const [ttl, setTtl] = useState<number | null>(null);
  const [created, setCreated] = useState<Link | null>(null);
  const create = useCreateLink();
  const selected = ttl ?? settings?.defaultLinkTtlSeconds ?? 24 * hour;
  const active = links.filter((l) => l.path === entry.path);

  const submit = () => create.mutate({ path: entry.path, ttlSeconds: selected, mode }, { onSuccess: setCreated });

  return (
    <Dialog
      open
      onClose={onClose}
      eyebrow="Temporary link"
      title={entry.name}
      description={`${displayPath(parentPath(entry.path))} · ${entry.kind === "dir" ? "Folder" : formatBytes(entry.size)}`}
      footer={<Button onClick={onClose}>Done</Button>}
    >
      {entry.kind === "dir" && (
        <label className="type-small flex flex-col gap-2 text-fg-muted">
          Link access
          <select aria-label="Link access" value={mode} onChange={(event) => { setMode(event.target.value as Link["mode"]); setCreated(null); }} className={cn(control, "type-body h-12 cursor-pointer appearance-none px-4")}>
            <option value="download">View and download</option>
            <option value="upload">Upload only</option>
          </select>
          <span className="text-fg-subtle">{mode === "upload" ? "Visitors can upload files into this folder. Its contents stay private and existing files are never overwritten." : "Visitors can browse this folder and download files or ZIP archives."}</span>
        </label>
      )}
      <div className="flex flex-col gap-2">
        <span id="ttl-label" className="type-small text-fg-muted">
          Expires after
        </span>
        <div role="radiogroup" aria-labelledby="ttl-label" className="relative grid grid-cols-3 rounded-[10px] bg-sunken p-0.5">
          <span
            aria-hidden
            className="absolute inset-y-0.5 left-0.5 w-[calc((100%-4px)/3)] rounded-lg bg-elevated shadow-[0_3px_8px_rgb(0_0_0/0.24)] transition-transform duration-280 ease-glide"
            style={{ transform: `translateX(${Math.max(0, lifetimes.findIndex((o) => o.seconds === selected)) * 100}%)` }}
          />
          {lifetimes.map((option) => (
            <button
              key={option.seconds}
              type="button"
              role="radio"
              aria-checked={selected === option.seconds}
              onClick={() => {
                setTtl(option.seconds);
                setCreated(null);
              }}
              className={cn(
                "type-small relative h-11 cursor-pointer rounded-lg font-semibold transition-colors duration-200 sm:h-9",
                selected === option.seconds ? "text-fg" : "text-fg-muted hover:text-fg",
              )}
            >
              {option.label}
            </button>
          ))}
        </div>
      </div>

      {created ? (
        <CreatedLink link={created} />
      ) : (
        <Button variant="primary" size="lg" onClick={submit} busy={create.isPending}>
          Create link
        </Button>
      )}
      {create.isError && (
        <p role="alert" className="type-small m-0 text-danger">
          {messageOf(create.error)}
        </p>
      )}

      {active.length > 0 && (
        <section aria-labelledby="active-links" className="flex flex-col gap-3 border-t border-line-strong pt-5">
          <h3 id="active-links" className="label-caps m-0">
            Active links for this item
          </h3>
          <ul className="m-0 flex list-none flex-col gap-1 p-0">
            {active.map((link) => (
              <ActiveLink key={link.id} link={link} />
            ))}
          </ul>
        </section>
      )}
    </Dialog>
  );
}

function CreatedLink({ link }: { link: Link }) {
  const url = absoluteUrl(link);
  const toast = useToast();
  const [copied, setCopied] = useState(false);
  const canShare = typeof navigator.share === "function";

  const copy = async () => {
    try {
      await navigator.clipboard.writeText(url);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 1800);
    } catch {
      toast.fail("Copy failed. Select the link and copy it manually.");
    }
  };

  return (
    <div className="flex flex-col gap-2">
      <label htmlFor="share-url" className="type-small text-fg-muted">
        Link
      </label>
      <div className="flex gap-2">
        <input
          id="share-url"
          readOnly
          value={url}
          onFocus={(e) => e.currentTarget.select()}
          className={cn(control, "h-10 min-w-0 flex-1 px-3.5 font-mono text-[13px]")}
        />
        <Button variant="primary" onClick={copy} icon={copied ? <Check size={15} aria-hidden /> : <Copy size={15} aria-hidden />} className="h-10">
          {copied ? "Copied" : "Copy"}
        </Button>
        {canShare && (
          <Button onClick={() => void navigator.share({ url }).catch(() => undefined)} icon={<Share size={15} aria-hidden />} className="max-sm:h-10 sm:hidden">
            <span className="sr-only">Share</span>
          </Button>
        )}
      </div>
      <p className="type-small m-0 flex items-start gap-2 text-fg-subtle">
        <Clock size={14} className="mt-[3px] shrink-0" aria-hidden />
        <span>Anyone with the link can {link.mode === "upload" ? "upload" : "download"} until <span className="text-fg-secondary">{formatDate(link.expiresAt)}</span></span>
      </p>
    </div>
  );
}

function ActiveLink({ link }: { link: Link }) {
  const revoke = useRevokeLink();
  const toast = useToast();
  const short = link.url.length > 14 ? `${link.url.slice(0, 7)}…${link.url.slice(-3)}` : link.url;
  return (
    <li className="type-small grid grid-cols-[minmax(0,1fr)_auto] items-center gap-x-3 gap-y-1 rounded-control border border-line bg-sunken px-4 py-3 sm:flex">
      <span className="flex-1 truncate font-mono text-[13px] text-fg-muted">{short}</span>
      <span className="type-micro col-start-1 row-start-2 text-fg-subtle">
        {formatRemaining(link.expiresAt)} · {link.mode === "upload" ? "upload only" : `${link.downloads}`} {link.mode === "upload" ? "" : link.downloads === 1 ? "download" : "downloads"}
      </span>
      <Button
        variant="danger"
        size="sm"
        className="col-start-2 row-span-2 row-start-1 max-sm:min-h-11"
        busy={revoke.isPending}
        onClick={() => revoke.mutate(link.id, { onSuccess: () => toast.notify("Link revoked"), onError: (e) => toast.fail(messageOf(e)) })}
      >
        Revoke
      </Button>
    </li>
  );
}
