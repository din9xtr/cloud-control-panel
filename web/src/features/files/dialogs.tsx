import { useQuery } from "@tanstack/react-query";
import { ChevronLeft, ChevronRight, Folder } from "lucide-react";
import { type FormEvent, useState } from "react";
import { messageOf } from "@/api/client";
import { folderQuery, useCreateFolder, useMove, useRename } from "@/api/queries";
import { baseName, displayPath, joinPath, parentPath, validateName } from "@/lib/paths";
import { Button } from "@/ui/Button";
import { Dialog } from "@/ui/Dialog";
import { Loading } from "@/ui/Feedback";
import { Field } from "@/ui/Field";
import { useToast } from "@/ui/Toast";

type NameDialogProps = {
  open: boolean;
  onClose: () => void;
};

export function NewFolderDialog({ open, onClose, dir }: NameDialogProps & { dir: string }) {
  const create = useCreateFolder();
  const toast = useToast();
  const [name, setName] = useState("");
  const [touched, setTouched] = useState(false);
  const error = touched ? validateName(name) : null;

  const close = () => {
    setName("");
    setTouched(false);
    create.reset();
    onClose();
  };
  const submit = (event: FormEvent) => {
    event.preventDefault();
    setTouched(true);
    if (validateName(name)) return;
    create.mutate(joinPath(dir, name), {
      onSuccess: () => {
        toast.notify(`Created ${name}`);
        close();
      },
    });
  };

  return (
    <Dialog
      open={open}
      onClose={close}
      size="sm"
      eyebrow="New folder"
      title="Create folder"
      description={`in ${displayPath(dir)}`}
      footer={
        <>
          <Button onClick={close}>Cancel</Button>
          <Button variant="primary" type="submit" form="new-folder" busy={create.isPending}>
            Create
          </Button>
        </>
      }
    >
      <form id="new-folder" onSubmit={submit} noValidate>
        <Field
          label="Name"
          autoFocus
          autoComplete="off"
          value={name}
          onChange={(e) => setName(e.target.value)}
          onBlur={() => setTouched(true)}
          error={error ?? (create.isError ? messageOf(create.error) : null)}
        />
      </form>
    </Dialog>
  );
}

export function RenameDialog({ open, onClose, path }: NameDialogProps & { path: string }) {
  const rename = useRename();
  const toast = useToast();
  const original = baseName(path);
  const [name, setName] = useState(original);
  const error = validateName(name);

  const close = () => {
    rename.reset();
    onClose();
  };
  const submit = (event: FormEvent) => {
    event.preventDefault();
    if (error || name === original) return close();
    rename.mutate(
      { path, name },
      {
        onSuccess: () => {
          toast.notify(`Renamed to ${name}`);
          close();
        },
      },
    );
  };

  return (
    <Dialog
      open={open}
      onClose={close}
      size="sm"
      eyebrow="Rename"
      title={original}
      description={displayPath(parentPath(path))}
      footer={
        <>
          <Button onClick={close}>Cancel</Button>
          <Button variant="primary" type="submit" form="rename" busy={rename.isPending} disabled={!!error}>
            Rename
          </Button>
        </>
      }
    >
      <form id="rename" onSubmit={submit} noValidate>
        <Field
          label="New name"
          autoComplete="off"
          value={name}
          onChange={(e) => setName(e.target.value)}
          onFocus={(e) => {
            const dot = original.lastIndexOf(".");
            e.currentTarget.setSelectionRange(0, dot > 0 ? dot : original.length);
          }}
          autoFocus
          error={name !== original ? (error ?? (rename.isError ? messageOf(rename.error) : null)) : null}
        />
      </form>
    </Dialog>
  );
}

export function MoveDialog({ open, onClose, paths, start }: NameDialogProps & { paths: string[]; start: string }) {
  const [dir, setDir] = useState(start);
  const move = useMove();
  const toast = useToast();
  const { data, isPending } = useQuery({ ...folderQuery(dir), enabled: open });
  const folders = (data?.entries ?? []).filter((e) => e.kind === "dir");
  const blocked = paths.some((p) => dir === p || dir.startsWith(`${p}/`));
  const unchanged = paths.every((p) => parentPath(p) === dir);
  const count = paths.length === 1 ? baseName(paths[0] ?? "") : `${paths.length} items`;

  const close = () => {
    move.reset();
    setDir(start);
    onClose();
  };
  const submit = () =>
    move.mutate(
      { paths, destination: dir },
      {
        onSuccess: () => {
          toast.notify(`Moved ${count} to ${displayPath(dir)}`);
          close();
        },
      },
    );

  return (
    <Dialog
      open={open}
      onClose={close}
      eyebrow="Move"
      title={count}
      description={`to ${displayPath(dir)}`}
      footer={
        <>
          <Button onClick={close}>Cancel</Button>
          <Button variant="primary" onClick={submit} busy={move.isPending} disabled={blocked || unchanged}>
            Move here
          </Button>
        </>
      }
    >
      <div className="flex flex-col overflow-hidden rounded-lg border border-line bg-canvas">
        <div className="flex h-11 items-center gap-1 border-b border-line px-1.5">
          <button
            type="button"
            disabled={!dir}
            onClick={() => setDir(parentPath(dir))}
            className="inline-flex size-9 cursor-pointer items-center justify-center rounded-md text-fg-muted hover:bg-raised disabled:cursor-default disabled:opacity-40"
            aria-label="Up one level"
          >
            <ChevronLeft size={16} aria-hidden />
          </button>
          <span className="truncate font-mono text-[13px] text-fg-muted">{displayPath(dir)}</span>
        </div>
        <ul className="m-0 max-h-64 list-none overflow-y-auto p-1" aria-label="Folders">
          {isPending && <Loading />}
          {!isPending && folders.length === 0 && <li className="px-3 py-6 text-center text-[13px] text-fg-subtle">No folders here</li>}
          {folders.map((f) => {
            const disabled = paths.includes(f.path);
            return (
              <li key={f.path}>
                <button
                  type="button"
                  disabled={disabled}
                  onClick={() => setDir(f.path)}
                  className="flex min-h-10 w-full cursor-pointer items-center gap-3 rounded-md px-2.5 text-left text-sm hover:bg-raised disabled:cursor-not-allowed disabled:opacity-40"
                >
                  <Folder size={16} className="text-fg-muted" aria-hidden />
                  <span className="flex-1 truncate">{f.name}</span>
                  <ChevronRight size={14} className="text-fg-faint" aria-hidden />
                </button>
              </li>
            );
          })}
        </ul>
      </div>
      {blocked && <p className="m-0 text-[13px] text-fg-subtle">A folder cannot be moved into itself.</p>}
      {move.isError && (
        <p role="alert" className="m-0 text-[13px] text-danger">
          {messageOf(move.error)}
        </p>
      )}
    </Dialog>
  );
}
