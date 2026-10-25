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
        <label className="flex flex-col gap-2 text-[13px] text-fg-muted">
          Link access
          <select aria-label="Link access" value={mode} onChange={(event) => { setMode(event.target.value as Link["mode"]); setCreated(null); }} className="h-11 rounded-lg border border-line-strong bg-canvas px-3 text-base text-fg sm:h-10 sm:text-sm">
            <option value="download">View and download</option>
            <option value="upload">Upload only</option>
          </select>
          <span>{mode === "upload" ? "Visitors can upload files into this folder. Its contents stay private and existing files are never overwritten." : "Visitors can browse this folder and download files or ZIP archives."}</span>
        </label>
      )}
      <div className="flex flex-col gap-2">
        <span id="ttl-label" className="text-[13px] text-fg-muted">
          Expires after
        </span>
        <div role="radiogroup" aria-labelledby="ttl-label" className="flex gap-0.5 rounded-lg border border-line bg-canvas p-0.5">
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
                "h-11 flex-1 cursor-pointer rounded-md text-[13px] font-medium transition-colors sm:h-8",
                selected === option.seconds ? "bg-line-strong text-fg" : "text-fg-muted hover:text-fg",
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
        <p role="alert" className="m-0 text-[13px] text-danger">
          {messageOf(create.error)}
        </p>
      )}

      {active.length > 0 && (
        <section aria-labelledby="active-links" className="flex flex-col gap-2.5 border-t border-line pt-3.5">
          <h3 id="active-links" className="label-caps m-0 font-normal">
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
      <label htmlFor="share-url" className="text-[13px] text-fg-muted">
        Link
      </label>
      <div className="flex gap-2">
        <input
          id="share-url"
          readOnly
          value={url}
          onFocus={(e) => e.currentTarget.select()}
          className="h-10 min-w-0 flex-1 rounded-lg border border-line-strong bg-canvas px-3 font-mono text-xs text-fg outline-none focus:border-fg-muted sm:h-9"
        />
        <Button variant="primary" onClick={copy} icon={copied ? <Check size={15} aria-hidden /> : <Copy size={15} aria-hidden />} className="max-sm:h-10">
          {copied ? "Copied" : "Copy"}
        </Button>
        {canShare && (
          <Button onClick={() => void navigator.share({ url }).catch(() => undefined)} icon={<Share size={15} aria-hidden />} className="max-sm:h-10 sm:hidden">
            <span className="sr-only">Share</span>
          </Button>
        )}
      </div>
      <p className="m-0 flex items-start gap-2 text-[13px] leading-relaxed text-fg-subtle">
        <Clock size={14} className="mt-1 shrink-0" aria-hidden />
        <span>Anyone with the link can {link.mode === "upload" ? "upload" : "download"} until <span className="font-mono text-fg-muted">{formatDate(link.expiresAt)}</span></span>
      </p>
    </div>
  );
}

function ActiveLink({ link }: { link: Link }) {
  const revoke = useRevokeLink();
  const toast = useToast();
  const short = link.url.length > 14 ? `${link.url.slice(0, 7)}…${link.url.slice(-3)}` : link.url;
  return (
    <li className="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-x-3 gap-y-1 rounded-lg bg-canvas px-3 py-2.5 text-[13px] sm:flex">
      <span className="flex-1 truncate font-mono text-fg-muted">{short}</span>
      <span className="col-start-1 row-start-2 font-mono text-xs text-fg-subtle">
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
