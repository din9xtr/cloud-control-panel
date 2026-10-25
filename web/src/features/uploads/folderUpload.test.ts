import { http, HttpResponse } from "msw";
import { describe, expect, it } from "vitest";
import { server } from "@/test/server";
import { droppedFiles, enqueueSelection, selectedFiles } from "./folderUpload";
import { UploadQueue } from "./UploadQueue";

function queue() {
  const started: string[] = [];
  const uploads = new UploadQueue((file, folder) => ({ start: () => { started.push(`${folder}/${file.name}`); }, pause: () => undefined, cancel: () => undefined }), () => undefined);
  return { uploads, started };
}

describe("folder uploads", () => {
  it("creates parents before children and preserves duplicate filenames in different folders", async () => {
    const created: string[] = [];
    server.use(http.post("/api/v1/files/folders", async ({ request }) => {
      const { path } = await request.json() as { path: string };
      created.push(path);
      return HttpResponse.json({}, { status: 201 });
    }));
    const { uploads, started } = queue();
    await enqueueSelection({ directories: ["garden/empty"], files: [
      { file: new File(["a"], "seeds.txt"), relativePath: "garden/greenhouse/seeds.txt" },
      { file: new File(["b"], "seeds.txt"), relativePath: "garden/shed/seeds.txt" },
    ] }, "documents", uploads);
    expect(created[0]).toBe("documents/garden");
    expect(created).toHaveLength(4);
    expect(created).toContain("documents/garden/empty");
    expect(started).toEqual(["documents/garden/greenhouse/seeds.txt", "documents/garden/shed/seeds.txt"]);
  });

  it("uses the directory picker relative paths and keeps empty files", () => {
    const file = new File([], "seeds.txt");
    Object.defineProperty(file, "webkitRelativePath", { value: "garden/greenhouse/seeds.txt" });
    expect(selectedFiles([file]).files).toEqual([{ file, relativePath: "garden/greenhouse/seeds.txt" }]);
  });

  it("rejects traversal before creating folders or queuing uploads", async () => {
    const { uploads } = queue();
    await expect(enqueueSelection({ directories: ["garden"], files: [{ file: new File([], "seed.txt"), relativePath: "garden/../seed.txt" }] }, "", uploads)).rejects.toThrow("Invalid folder path");
    expect(uploads.getSnapshot()).toEqual([]);
  });

  it("merges existing directories but stops when a file occupies a directory name", async () => {
    server.use(
      http.post("/api/v1/files/folders", () => HttpResponse.json({ error: { code: "conflict", message: "already exists" } }, { status: 409 })),
      http.get("/api/v1/files", ({ request }) => new URL(request.url).searchParams.get("path") === "garden" ? HttpResponse.json({ path: "garden", entries: [] }) : HttpResponse.json({ error: { code: "invalid_request", message: "not a folder" } }, { status: 400 })),
    );
    const { uploads, started } = queue();
    await enqueueSelection({ directories: [], files: [{ file: new File([], "seed.txt"), relativePath: "garden/seed.txt" }] }, "", uploads);
    expect(started).toEqual(["garden/seed.txt"]);
    await expect(enqueueSelection({ directories: ["garden/occupied"], files: [] }, "", uploads)).rejects.toThrow("not a folder");
  });

  it("reads every directory batch and preserves empty directories", async () => {
    const file = new File([], "seed.txt");
    const child = { name: file.name, isFile: true, isDirectory: false, file: (success: (file: File) => void) => success(file) };
    const empty = { name: "empty", isFile: false, isDirectory: true, createReader: () => ({ readEntries: (success: (entries: unknown[]) => void) => success([]) }) };
    const batches = [[child], [empty], []];
    const root = { name: "garden", isFile: false, isDirectory: true, createReader: () => ({ readEntries: (success: (entries: unknown[]) => void) => success(batches.shift() ?? []) }) };
    const items = [{ webkitGetAsEntry: () => root }] as unknown as DataTransferItemList;
    const selection = await droppedFiles(items, [] as unknown as FileList);
    expect(selection.directories).toEqual(["garden", "garden/empty"]);
    expect(selection.files).toEqual([{ file, relativePath: "garden/seed.txt" }]);
  });

  it("falls back to ordinary files when directory entry APIs are unavailable", async () => {
    const file = new File([], "seed.txt");
    const selection = await droppedFiles([{}] as unknown as DataTransferItemList, [file] as unknown as FileList);
    expect(selection.files).toEqual([{ file, relativePath: "seed.txt" }]);
  });
});
