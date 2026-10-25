import { api, ApiError } from "@/api/client";
import type { UploadQueue } from "./UploadQueue";

export type FolderSelection = { files: { file: File; relativePath: string }[]; directories: string[] };

type DroppedEntry = {
  name: string;
  isFile: boolean;
  isDirectory: boolean;
  file: (success: (file: File) => void, failure: (error: DOMException) => void) => void;
  createReader: () => { readEntries: (success: (entries: DroppedEntry[]) => void, failure: (error: DOMException) => void) => void };
};

export function selectedFiles(files: Iterable<File>): FolderSelection {
  return { files: Array.from(files, (file) => ({ file, relativePath: file.webkitRelativePath || file.name })), directories: [] };
}

export async function droppedFiles(items: DataTransferItemList, fallback: FileList): Promise<FolderSelection> {
  const entries = Array.from(items, (item) => (item.webkitGetAsEntry?.() ?? null) as DroppedEntry | null).filter((entry) => entry !== null);
  if (entries.length === 0) return selectedFiles(Array.from(fallback));
  const selection: FolderSelection = { files: [], directories: [] };
  async function visit(entry: DroppedEntry, parent: string): Promise<void> {
    const relativePath = parent ? `${parent}/${entry.name}` : entry.name;
    if (entry.isFile) {
      const file = await new Promise<File>((resolve, reject) => entry.file(resolve, reject));
      selection.files.push({ file, relativePath });
    } else if (entry.isDirectory) {
      selection.directories.push(relativePath);
      const reader = entry.createReader();
      for (;;) {
        const children = await new Promise<DroppedEntry[]>((resolve, reject) => reader.readEntries(resolve, reject));
        if (children.length === 0) break;
        for (const child of children) await visit(child, relativePath);
      }
    }
  }
  for (const entry of entries) await visit(entry, "");
  return selection;
}

function segments(path: string): string[] {
  const parts = path.split("/");
  if (parts.some((part) => !part || part === "." || part === ".." || part.includes("\\"))) throw new Error("Invalid folder path.");
  return parts;
}

export async function enqueueSelection(selection: FolderSelection, destination: string, queue: UploadQueue): Promise<void> {
  const directories = new Set<string>();
  const addParents = (parts: string[]) => {
    for (let depth = 1; depth <= parts.length; depth++) directories.add(parts.slice(0, depth).join("/"));
  };
  for (const path of selection.directories) addParents(segments(path));
  for (const item of selection.files) addParents(segments(item.relativePath).slice(0, -1));
  const absolute = (relative: string) => [destination, relative].filter(Boolean).join("/");
  for (const relative of [...directories].sort((a, b) => a.split("/").length - b.split("/").length)) {
    const path = absolute(relative);
    try {
      await api.POST("/files/folders", { body: { path } });
    } catch (error) {
      if (!(error instanceof ApiError) || error.status !== 409) throw error;
      await api.GET("/files", { params: { query: { path } } });
    }
  }
  const groups = new Map<string, File[]>();
  for (const { file, relativePath } of selection.files) {
    const folder = absolute(segments(relativePath).slice(0, -1).join("/"));
    const files = groups.get(folder) ?? [];
    files.push(file);
    groups.set(folder, files);
  }
  for (const [folder, files] of groups) queue.add(files, folder);
}
