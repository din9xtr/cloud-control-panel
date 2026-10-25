import { describe, expect, it, vi } from "vitest";
import { type UploadCallbacks, type UploadFactory, UploadQueue } from "./UploadQueue";

type Fake = { file: File; callbacks: UploadCallbacks; started: number; paused: number; canceled: number };

function setup(concurrency = 2) {
  const fakes: Fake[] = [];
  const factory: UploadFactory = (file, _folder, callbacks) => {
    const fake: Fake = { file, callbacks, started: 0, paused: 0, canceled: 0 };
    fakes.push(fake);
    return {
      start: () => fake.started++,
      pause: () => fake.paused++,
      cancel: () => fake.canceled++,
    };
  };
  const finished = vi.fn();
  let clock = 0;
  const queue = new UploadQueue(factory, finished, concurrency, () => clock);
  const tick = (ms: number) => (clock += ms);
  return { queue, fakes, finished, tick };
}

const file = (name: string, size = 100) => new File([new Uint8Array(size)], name);

describe("UploadQueue", () => {
  it("limits concurrent uploads and starts the next one on success", () => {
    const { queue, fakes, finished } = setup(2);
    queue.add([file("a"), file("b"), file("c")], "docs");

    expect(fakes).toHaveLength(2);
    expect(queue.getSnapshot().map((i) => i.status)).toEqual(["uploading", "uploading", "queued"]);

    fakes[0]!.callbacks.onSuccess();
    expect(finished).toHaveBeenCalledWith("docs");
    expect(fakes).toHaveLength(3);
    expect(queue.getSnapshot().map((i) => i.status)).toEqual(["done", "uploading", "uploading"]);
  });

  it("tracks progress and smooths speed", () => {
    const { queue, fakes, tick } = setup();
    queue.add([file("a", 10_000)], "");
    tick(1000);
    fakes[0]!.callbacks.onProgress(4000);
    const [item] = queue.getSnapshot();
    expect(item!.uploaded).toBe(4000);
    expect(item!.speed).toBe(4000);
  });

  it("pauses, resumes with the same handle and retries after errors", () => {
    const { queue, fakes } = setup();
    queue.add([file("a")], "");
    const id = queue.getSnapshot()[0]!.id;

    queue.pause(id);
    expect(fakes[0]!.paused).toBe(1);
    expect(queue.getSnapshot()[0]!.status).toBe("paused");

    queue.resume(id);
    expect(fakes).toHaveLength(1);
    expect(fakes[0]!.started).toBe(2);

    fakes[0]!.callbacks.onError("Storage quota exceeded.");
    expect(queue.getSnapshot()[0]).toMatchObject({ status: "error", error: "Storage quota exceeded." });
    queue.resume(id);
    expect(queue.getSnapshot()[0]!.status).toBe("uploading");
  });

  it("cancels and clears finished items", () => {
    const { queue, fakes } = setup();
    queue.add([file("a"), file("b")], "");
    const [a, b] = queue.getSnapshot();
    queue.cancel(a!.id);
    expect(fakes[0]!.canceled).toBe(1);
    fakes[1]!.callbacks.onSuccess();
    queue.clearFinished();
    expect(queue.getSnapshot().find((i) => i.id === b!.id)).toBeUndefined();
    expect(queue.getSnapshot()).toHaveLength(0);
  });

  it("notifies subscribers with a new snapshot", () => {
    const { queue } = setup();
    const listener = vi.fn();
    queue.subscribe(listener);
    const before = queue.getSnapshot();
    queue.add([file("a")], "");
    expect(listener).toHaveBeenCalled();
    expect(queue.getSnapshot()).not.toBe(before);
  });
});
