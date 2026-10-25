import { describe, expect, it, vi } from "vitest";
import type { UploadOptions } from "tus-js-client";
import { createTusFactory } from "./tusFactory";

const mocks = vi.hoisted(() => ({
  previous: [] as { metadata: Record<string, string>; uploadUrl: string }[],
  uploads: [] as { options: UploadOptions; start: ReturnType<typeof vi.fn>; abort: ReturnType<typeof vi.fn>; resumeFromPreviousUpload: ReturnType<typeof vi.fn> }[],
}));

vi.mock("tus-js-client", () => ({
  DetailedError: class extends Error {},
  Upload: class {
    start = vi.fn();
    abort = vi.fn().mockResolvedValue(undefined);
    constructor(_file: File, readonly options: UploadOptions) { mocks.uploads.push(this); }
    findPreviousUploads() { return Promise.resolve(mocks.previous); }
    resumeFromPreviousUpload = vi.fn();
  },
}));

const callbacks = () => ({ onProgress: vi.fn(), onSuccess: vi.fn(), onError: vi.fn() });

describe("tus upload factory", () => {
  it("resumes only uploads targeting the same folder, including older stored records", async () => {
    const correct = { metadata: { folder: "garden", filename: "seeds.txt" }, uploadUrl: "/api/v1/uploads/correct" };
    const wrong = { metadata: { folder: "greenhouse", filename: "seeds.txt" }, uploadUrl: "/api/v1/uploads/wrong" };
    mocks.previous = [correct, wrong];
    const handle = createTusFactory("/api/v1/uploads")(new File(["seeds"], "seeds.txt"), "garden", callbacks());
    handle.start();
    await Promise.resolve();
    expect(mocks.uploads.at(-1)?.resumeFromPreviousUpload).toHaveBeenCalledWith(correct);
    mocks.previous = [];
  });

  it("does not start a paused upload after asynchronous resume discovery", async () => {
    const handle = createTusFactory("/api/v1/uploads")(new File([], "seeds.txt"), "garden", callbacks());
    const upload = mocks.uploads.at(-1);
    handle.start();
    handle.pause();
    await Promise.resolve();
    expect(upload?.start).not.toHaveBeenCalled();
    handle.start();
    expect(upload?.start).toHaveBeenCalledOnce();
  });
});
