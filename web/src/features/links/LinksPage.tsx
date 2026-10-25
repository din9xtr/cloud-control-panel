import { useQuery } from "@tanstack/react-query";
import { Link as RouterLink } from "@tanstack/react-router";
import { Check, Copy, Link2 } from "lucide-react";
import { useState } from "react";
import { type Link, messageOf } from "@/api/client";
import { linksQuery, useRevokeLink } from "@/api/queries";
import { PageBar, Page } from "@/layout/PageBar";
import { formatDate, formatRemaining } from "@/lib/format";
import { stagger } from "@/lib/motion";
import { displayPath, filesHref, parentPath } from "@/lib/paths";
import { Button, IconButton } from "@/ui/Button";
import { Badge, EmptyState, ErrorState, Loading, PageHeader } from "@/ui/Feedback";
import { useToast } from "@/ui/Toast";
import { absoluteUrl } from "./ShareDialog";

export function LinksPage() {
  const { data, isPending, isError, error, refetch } = useQuery(linksQuery);
  return (
    <>
      <PageBar prefix="02" title="Links" />
      <Page>
        <PageHeader eyebrow="02 / Links" title="Temporary links">
          Links allow downloading files and folders or uploading into a folder until they expire. Revoke a link to stop access immediately.
        </PageHeader>
        {isPending && <Loading />}
        {isError && <ErrorState message={messageOf(error)} onRetry={() => void refetch()} />}
        {data && data.length === 0 && (
          <EmptyState icon={<Link2 size={24} />} title="No active links">
            Open a file or folder’s actions and choose Temporary link.
          </EmptyState>
        )}
        {data && data.length > 0 && (
          <ul className="m-0 flex animate-reveal list-none flex-col overflow-hidden rounded-card border border-line bg-surface p-0 [animation-delay:80ms]">
            {data.map((link, index) => (
              <LinkRow key={link.id} link={link} index={index} />
            ))}
          </ul>
        )}
      </Page>
    </>
  );
}

function LinkRow({ link, index }: { link: Link; index: number }) {
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
    <li style={stagger(index, 160)} className="flex animate-fade-in flex-wrap items-center gap-x-6 gap-y-3 border-b border-line px-6 py-5 last:border-b-0 max-sm:px-5">
      <div className="flex min-w-0 flex-[1_1_260px] flex-col gap-1">
        <span className="type-body truncate font-semibold">{link.name}</span>
        <RouterLink to={filesHref(parentPath(link.path))} className="type-small truncate text-fg-subtle no-underline transition-colors hover:text-link">
          {displayPath(parentPath(link.path))}
        </RouterLink>
      </div>
      <div className="flex flex-col items-start gap-1.5">
        <Badge tone="info">{formatRemaining(link.expiresAt)} left</Badge>
        <span className="type-micro text-fg-subtle">until {formatDate(link.expiresAt)}</span>
      </div>
      <span className="type-small w-28 text-fg-subtle">
        {link.mode === "upload" ? "Upload only" : `${link.downloads} ${link.downloads === 1 ? "download" : "downloads"}`}
      </span>
      <div className="flex items-center gap-2">
        <IconButton label={copied ? "Copied" : `Copy link to ${link.name}`} onClick={copy} className="border border-line-strong">
          {copied ? <Check size={16} strokeWidth={2.25} className="text-success" aria-hidden /> : <Copy size={16} aria-hidden />}
        </IconButton>
        <Button
          variant="danger"
          size="md"
          busy={revoke.isPending}
          onClick={() => revoke.mutate(link.id, { onSuccess: () => toast.notify("Link revoked"), onError: (e) => toast.fail(messageOf(e)) })}
        >
          Revoke
        </Button>
      </div>
    </li>
  );
}
