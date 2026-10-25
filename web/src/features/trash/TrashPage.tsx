import { useQuery } from "@tanstack/react-query";
import { RotateCcw, Trash2 } from "lucide-react";
import { useState } from "react";
import { messageOf, type TrashItem } from "@/api/client";
import { settingsQuery, trashQuery, useDeleteForever, useEmptyTrash, useRestore } from "@/api/queries";
import { EntryIcon } from "@/features/files/entryKind";
import { PageBar, PrimaryAction, Page } from "@/layout/PageBar";
import { formatBytes, formatDate, formatRemaining } from "@/lib/format";
import { stagger } from "@/lib/motion";
import { displayPath, parentPath } from "@/lib/paths";
import { Button } from "@/ui/Button";
import { Dialog } from "@/ui/Dialog";
import { Badge, EmptyState, ErrorState, Loading, PageHeader } from "@/ui/Feedback";
import { useToast } from "@/ui/Toast";

type Confirm = { kind: "none" } | { kind: "empty" } | { kind: "item"; item: TrashItem };

export function TrashPage() {
  const { data, isPending, isError, error, refetch } = useQuery(trashQuery);
  const { data: settings } = useQuery(settingsQuery);
  const [confirm, setConfirm] = useState<Confirm>({ kind: "none" });
  const empty = useEmptyTrash();
  const remove = useDeleteForever();
  const toast = useToast();
  const total = data?.reduce((sum, i) => sum + i.size, 0) ?? 0;
  const hasItems = !!data && data.length > 0;

  const close = () => setConfirm({ kind: "none" });
  const confirmDelete = () => {
    if (confirm.kind === "empty") {
      empty.mutate(undefined, { onSuccess: () => toast.notify("Trash emptied"), onError: (e) => toast.fail(messageOf(e)), onSettled: close });
    } else if (confirm.kind === "item") {
      remove.mutate(confirm.item.id, { onSuccess: () => toast.notify(`Deleted ${confirm.item.name}`), onError: (e) => toast.fail(messageOf(e)), onSettled: close });
    }
  };

  return (
    <>
      <PageBar
        prefix="04"
        title="Trash"
        primary={hasItems ? <PrimaryAction tone="danger" onClick={() => setConfirm({ kind: "empty" })}>Empty Trash</PrimaryAction> : undefined}
      />
      <Page>
        <PageHeader eyebrow="04 / Trash" title={hasItems ? <>Trash <span className="text-fg-subtle">· {formatBytes(total)}</span></> : "Trash"}>
          {settings ? `Items stay here for ${settings.trashRetentionDays} days, then they are deleted automatically.` : "Items stay here until they are deleted automatically."}
        </PageHeader>
        {isPending && <Loading />}
        {isError && <ErrorState message={messageOf(error)} onRetry={() => void refetch()} />}
        {data && data.length === 0 && (
          <EmptyState icon={<Trash2 size={24} />} title="Trash is empty">
            Items you move to Trash stay here until they are deleted automatically.
          </EmptyState>
        )}
        {hasItems && (
          <ul className="m-0 flex animate-reveal list-none flex-col overflow-hidden rounded-card border border-line bg-surface p-0 [animation-delay:80ms]">
            {data.map((item, index) => (
              <TrashRow key={item.id} item={item} index={index} onDelete={() => setConfirm({ kind: "item", item })} />
            ))}
          </ul>
        )}
      </Page>
      <Dialog
        open={confirm.kind !== "none"}
        onClose={close}
        size="sm"
        eyebrow="Delete permanently"
        title={confirm.kind === "item" ? confirm.item.name : `Empty Trash (${data?.length ?? 0} items)`}
        footer={
          <>
            <Button onClick={close}>Cancel</Button>
            <Button variant="danger" onClick={confirmDelete} busy={empty.isPending || remove.isPending}>
              Delete forever
            </Button>
          </>
        }
      >
        <p className="type-body m-0 text-fg-muted">This cannot be undone. The space is freed immediately.</p>
      </Dialog>
    </>
  );
}

function TrashRow({ item, index, onDelete }: { item: TrashItem; index: number; onDelete: () => void }) {
  const restore = useRestore();
  const toast = useToast();
  return (
    <li style={stagger(index, 160)} className="flex animate-fade-in flex-wrap items-center gap-x-5 gap-y-3 border-b border-line px-6 py-5 last:border-b-0 max-sm:px-5">
      <EntryIcon entry={{ kind: item.kind, contentType: item.kind === "dir" ? undefined : guessType(item.name) }} size={22} />
      <div className="flex min-w-0 flex-[1_1_220px] flex-col gap-1">
        <span className="type-body truncate font-semibold">{item.name}</span>
        <span className="type-small truncate text-fg-subtle">
          From {displayPath(parentPath(item.originalPath))} · {formatBytes(item.size)}
        </span>
      </div>
      <span title={`Deleted ${formatDate(item.trashedAt)}`}>
        <Badge>Deletes in {formatRemaining(item.purgeAt)}</Badge>
      </span>
      <div className="flex gap-2">
        <Button
          icon={<RotateCcw size={14} aria-hidden />}
          busy={restore.isPending}
          onClick={() => restore.mutate(item.id, { onSuccess: (path) => toast.notify(`Restored to ${displayPath(path)}`), onError: (e) => toast.fail(messageOf(e)) })}
        >
          Restore
        </Button>
        <Button variant="danger" onClick={onDelete}>
          Delete
        </Button>
      </div>
    </li>
  );
}

function guessType(name: string): string {
  const ext = name.split(".").pop()?.toLowerCase() ?? "";
  if (["jpg", "jpeg", "png", "gif", "webp", "heic", "avif"].includes(ext)) return "image/*";
  if (["mp4", "mov", "mkv", "webm"].includes(ext)) return "video/*";
  if (["mp3", "m4a", "flac", "wav", "ogg"].includes(ext)) return "audio/*";
  if (["zip", "7z", "tar", "gz", "rar", "zst"].includes(ext)) return "application/zip";
  if (["pdf", "txt", "md", "epub", "doc", "docx"].includes(ext)) return "text/plain";
  return "";
}
