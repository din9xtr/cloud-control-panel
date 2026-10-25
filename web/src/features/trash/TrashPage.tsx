import { useQuery } from "@tanstack/react-query";
import { RotateCcw, Trash2 } from "lucide-react";
import { useState } from "react";
import { messageOf, type TrashItem } from "@/api/client";
import { trashQuery, useDeleteForever, useEmptyTrash, useRestore } from "@/api/queries";
import { EntryIcon } from "@/features/files/entryKind";
import { formatBytes, formatDate, formatRemaining } from "@/lib/format";
import { displayPath, parentPath } from "@/lib/paths";
import { Button } from "@/ui/Button";
import { Dialog } from "@/ui/Dialog";
import { EmptyState, ErrorState, Loading, PageHeader } from "@/ui/Feedback";
import { useToast } from "@/ui/Toast";

type Confirm = { kind: "none" } | { kind: "empty" } | { kind: "item"; item: TrashItem };

export function TrashPage() {
  const { data, isPending, isError, error, refetch } = useQuery(trashQuery);
  const [confirm, setConfirm] = useState<Confirm>({ kind: "none" });
  const empty = useEmptyTrash();
  const remove = useDeleteForever();
  const toast = useToast();
  const total = data?.reduce((sum, i) => sum + i.size, 0) ?? 0;

  const close = () => setConfirm({ kind: "none" });
  const confirmDelete = () => {
    if (confirm.kind === "empty") {
      empty.mutate(undefined, { onSuccess: () => toast.notify("Trash emptied"), onError: (e) => toast.fail(messageOf(e)), onSettled: close });
    } else if (confirm.kind === "item") {
      remove.mutate(confirm.item.id, { onSuccess: () => toast.notify(`Deleted ${confirm.item.name}`), onError: (e) => toast.fail(messageOf(e)), onSettled: close });
    }
  };

  return (
    <div className="flex w-full max-w-[960px] flex-col gap-6 px-4 pt-[max(env(safe-area-inset-top),24px)] pb-10 sm:px-7 sm:pt-8">
      <PageHeader
        eyebrow="04 / Trash"
        title={
          <>
            Trash {data && data.length > 0 && <span className="font-normal text-fg-subtle">· {formatBytes(total)}</span>}
          </>
        }
        actions={
          data && data.length > 0 ? (
            <Button variant="danger" onClick={() => setConfirm({ kind: "empty" })}>
              Empty Trash
            </Button>
          ) : null
        }
      />
      {isPending && <Loading />}
      {isError && <ErrorState message={messageOf(error)} onRetry={() => void refetch()} />}
      {data && data.length === 0 && (
        <EmptyState icon={<Trash2 size={20} />} title="Trash is empty">
          Items you move to Trash stay here until they are deleted automatically.
        </EmptyState>
      )}
      {data && data.length > 0 && (
        <ul className="m-0 flex list-none flex-col overflow-hidden rounded-xl border border-line bg-sunken p-0">
          {data.map((item) => (
            <TrashRow key={item.id} item={item} onDelete={() => setConfirm({ kind: "item", item })} />
          ))}
        </ul>
      )}
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
        <p className="m-0 text-sm text-fg-muted">This cannot be undone. The space is freed immediately.</p>
      </Dialog>
    </div>
  );
}

function TrashRow({ item, onDelete }: { item: TrashItem; onDelete: () => void }) {
  const restore = useRestore();
  const toast = useToast();
  return (
    <li className="flex flex-wrap items-center gap-x-4 gap-y-2 border-b border-line px-4 py-3.5 last:border-b-0 sm:px-5">
      <EntryIcon entry={{ kind: item.kind, contentType: item.kind === "dir" ? undefined : guessType(item.name) }} />
      <div className="flex min-w-0 flex-[1_1_220px] flex-col gap-1">
        <span className="truncate text-sm">{item.name}</span>
        <span className="truncate font-mono text-xs text-fg-subtle">
          from {displayPath(parentPath(item.originalPath))} · {formatBytes(item.size)}
        </span>
      </div>
      <span className="font-mono text-xs text-fg-subtle" title={`Deleted ${formatDate(item.trashedAt)}`}>
        deletes in {formatRemaining(item.purgeAt)}
      </span>
      <div className="flex gap-2">
        <Button
          size="sm"
          icon={<RotateCcw size={14} aria-hidden />}
          busy={restore.isPending}
          onClick={() => restore.mutate(item.id, { onSuccess: (path) => toast.notify(`Restored to ${displayPath(path)}`), onError: (e) => toast.fail(messageOf(e)) })}
        >
          Restore
        </Button>
        <Button variant="danger" size="sm" onClick={onDelete}>
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
