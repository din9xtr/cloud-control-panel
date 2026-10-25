import { describe, expect, it, vi } from "vitest";
import { DetailedError, type UploadOptions } from "tus-js-client";
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
  it("keeps folder retries separate from older rename-mode sessions", async () => {
    const old = { metadata: { folder: "garden", filename: "seeds.txt" }, uploadUrl: "/api/v1/uploads/old" };
    const safe = { metadata: { folder: "garden", filename: "seeds.txt", conflict: "skip-identical" }, uploadUrl: "/api/v1/uploads/safe" };
    mocks.previous = [safe, old];
    const handle = createTusFactory("/api/v1/uploads")(new File(["seeds"], "seeds.txt"), "garden", callbacks(), true);
    handle.start();
    await Promise.resolve();
    const upload = mocks.uploads.at(-1);
    expect(upload?.options.metadata?.conflict).toBe("skip-identical");
    expect(upload?.resumeFromPreviousUpload).toHaveBeenCalledWith(safe);
    mocks.previous = [];
  });

  it("retries offset races but reports content conflicts without retrying", () => {
    createTusFactory("/api/v1/uploads")(new File(["seeds"], "seeds.txt"), "garden", callbacks(), true);
    const retry = mocks.uploads.at(-1)?.options.onShouldRetry;
    const error = Object.assign(Object.create(DetailedError.prototype) as DetailedError, {
      originalRequest: { getMethod: (): string => "PATCH" },
      originalResponse: { getStatus: () => 409, getBody: () => JSON.stringify({ error: { message: "already exists" } }) },
    });
    expect(retry?.(error, 0, {})).toBe(false);
    error.originalResponse.getBody = () => JSON.stringify({ error: { message: "upload offset mismatch" } });
    expect(retry?.(error, 0, {})).toBe(true);
    error.originalRequest.getMethod = () => "HEAD";
    error.originalResponse.getBody = () => "";
    expect(retry?.(error, 0, {})).toBe(false);
  });

});
