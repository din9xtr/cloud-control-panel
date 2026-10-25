import { useQuery } from "@tanstack/react-query";
import { Link as RouterLink } from "@tanstack/react-router";
import { Check, Copy, Link2 } from "lucide-react";
import { useState } from "react";
import { type Link, messageOf } from "@/api/client";
import { linksQuery, useRevokeLink } from "@/api/queries";
import { formatDate, formatRemaining } from "@/lib/format";
import { displayPath, filesHref, parentPath } from "@/lib/paths";
import { Button, IconButton } from "@/ui/Button";
import { EmptyState, ErrorState, Loading, PageHeader } from "@/ui/Feedback";
import { useToast } from "@/ui/Toast";
import { absoluteUrl } from "./ShareDialog";

export function LinksPage() {
  const { data, isPending, isError, error, refetch } = useQuery(linksQuery);
  return (
    <div className="flex w-full max-w-[960px] flex-col gap-6 px-4 pt-[max(env(safe-area-inset-top),24px)] pb-10 sm:px-7 sm:pt-8">
      <PageHeader eyebrow="02 / Links" title="Temporary links" />
      <p className="m-0 max-w-xl text-[13px] text-fg-subtle">
        Links allow downloading files and folders or uploading into a folder until they expire. Revoke a link to stop access immediately.
      </p>
      {isPending && <Loading />}
      {isError && <ErrorState message={messageOf(error)} onRetry={() => void refetch()} />}
      {data && data.length === 0 && (
        <EmptyState icon={<Link2 size={20} />} title="No active links">
          Open a file or folder’s actions and choose Temporary link.
        </EmptyState>
      )}
      {data && data.length > 0 && (
        <ul className="m-0 flex list-none flex-col overflow-hidden rounded-xl border border-line bg-sunken p-0">
          {data.map((link) => (
            <LinkRow key={link.id} link={link} />
          ))}
        </ul>
      )}
    </div>
  );
}

function LinkRow({ link }: { link: Link }) {
  const revoke = useRevokeLink();
  const toast = useToast();
  const [copied, setCopied] = useState(false);
  const copy = async () => {
    try {
      await navigator.clipboard.writeText(absoluteUrl(link));
      setCopied(true);
      window.setTimeout(() => setCopied(false), 1600);
    } catch {
      toast.fail("Copy failed.");
    }
  };
  return (
    <li className="flex flex-wrap items-center gap-x-4 gap-y-2 border-b border-line px-4 py-3.5 last:border-b-0 sm:px-5">
      <div className="flex min-w-0 flex-[1_1_260px] flex-col gap-1">
        <span className="truncate text-sm">{link.name}</span>
        <RouterLink to={filesHref(parentPath(link.path))} className="truncate font-mono text-xs text-fg-subtle no-underline hover:text-fg">
          {displayPath(parentPath(link.path))}
        </RouterLink>
      </div>
      <div className="flex flex-col gap-1 font-mono text-xs text-fg-muted">
        <span>{formatRemaining(link.expiresAt)} left</span>
        <span className="text-fg-subtle">until {formatDate(link.expiresAt)}</span>
      </div>
      <span className="w-24 font-mono text-xs text-fg-subtle">
        {link.mode === "upload" ? "Upload only" : `${link.downloads} ${link.downloads === 1 ? "download" : "downloads"}`}
      </span>
      <div className="flex items-center gap-1">
        <IconButton label={copied ? "Copied" : `Copy link to ${link.name}`} onClick={copy}>
          {copied ? <Check size={16} aria-hidden /> : <Copy size={16} aria-hidden />}
        </IconButton>
        <Button
          variant="danger"
          size="sm"
          busy={revoke.isPending}
          onClick={() => revoke.mutate(link.id, { onSuccess: () => toast.notify("Link revoked"), onError: (e) => toast.fail(messageOf(e)) })}
        >
          Revoke
        </Button>
      </div>
    </li>
  );
}
