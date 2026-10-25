export type UploadStatus = "queued" | "uploading" | "paused" | "done" | "error";

export type UploadItem = {
  id: string;
  name: string;
  folder: string;
  size: number;
  uploaded: number;
  speed: number;
  status: UploadStatus;
  error?: string;
};

export type UploadCallbacks = {
  onProgress: (uploaded: number) => void;
  onSuccess: () => void;
  onError: (message: string) => void;
};

export type UploadHandle = {
  start: () => void;
  pause: () => void;
  cancel: () => void;
};

export type UploadFactory = (file: File, folder: string, callbacks: UploadCallbacks) => UploadHandle;

type Entry = { item: UploadItem; file: File; handle?: UploadHandle; lastSample?: { at: number; bytes: number } };

const speedSmoothing = 0.3;

export class UploadQueue {
  private readonly entries = new Map<string, Entry>();
  private readonly listeners = new Set<() => void>();
  private snapshot: UploadItem[] = [];
  private sequence = 0;

  constructor(
    private readonly factory: UploadFactory,
    private readonly onFinished: (folder: string) => void,
    private readonly concurrency = 3,
    private readonly now: () => number = () => performance.now(),
  ) {}

  subscribe = (listener: () => void): (() => void) => {
    this.listeners.add(listener);
    return () => this.listeners.delete(listener);
  };

  getSnapshot = (): UploadItem[] => this.snapshot;

  add(files: Iterable<File>, folder: string): void {
    for (const file of files) {
      const id = `u${++this.sequence}`;
      this.entries.set(id, {
        file,
        item: { id, name: file.name, folder, size: file.size, uploaded: 0, speed: 0, status: "queued" },
      });
    }
    this.pump();
  }

  pause(id: string): void {
    const entry = this.entries.get(id);
    if (!entry || (entry.item.status !== "uploading" && entry.item.status !== "queued")) return;
    entry.handle?.pause();
    this.update(entry, { status: "paused", speed: 0 });
    this.pump();
  }

  resume(id: string): void {
    const entry = this.entries.get(id);
    if (!entry || (entry.item.status !== "paused" && entry.item.status !== "error")) return;
    this.update(entry, { status: "queued", error: undefined });
    this.pump();
  }

  cancel(id: string): void {
    const entry = this.entries.get(id);
    if (!entry) return;
    if (entry.item.status !== "done") entry.handle?.cancel();
    this.entries.delete(id);
    this.publish();
    this.pump();
  }

  pauseAll(): void {
    for (const entry of this.entries.values()) this.pause(entry.item.id);
  }

  resumeAll(): void {
    for (const entry of this.entries.values()) this.resume(entry.item.id);
  }

  clearFinished(): void {
    for (const [id, entry] of this.entries) {
      if (entry.item.status === "done") this.entries.delete(id);
    }
    this.publish();
  }

  private pump(): void {
    let active = this.count("uploading");
    for (const entry of this.entries.values()) {
      if (active >= this.concurrency) break;
      if (entry.item.status !== "queued") continue;
      active++;
      this.begin(entry);
    }
  }

  private begin(entry: Entry): void {
    this.update(entry, { status: "uploading" });
    entry.lastSample = { at: this.now(), bytes: entry.item.uploaded };
    entry.handle ??= this.factory(entry.file, entry.item.folder, {
      onProgress: (uploaded) => this.progress(entry, uploaded),
      onSuccess: () => {
        this.update(entry, { status: "done", uploaded: entry.item.size, speed: 0 });
        this.onFinished(entry.item.folder);
        this.pump();
      },
      onError: (message) => {
        this.update(entry, { status: "error", error: message, speed: 0 });
        this.pump();
      },
    });
    entry.handle.start();
  }

  private progress(entry: Entry, uploaded: number): void {
    if (entry.item.status !== "uploading") return;
    const at = this.now();
    const last = entry.lastSample ?? { at, bytes: uploaded };
    const elapsed = (at - last.at) / 1000;
    let speed = entry.item.speed;
    if (elapsed >= 0.25) {
      const instant = (uploaded - last.bytes) / elapsed;
      speed = speed === 0 ? instant : speed + speedSmoothing * (instant - speed);
      entry.lastSample = { at, bytes: uploaded };
    }
    this.update(entry, { uploaded, speed: Math.max(0, speed) });
  }

  private count(status: UploadStatus): number {
    let n = 0;
    for (const entry of this.entries.values()) if (entry.item.status === status) n++;
    return n;
  }

  private update(entry: Entry, patch: Partial<UploadItem>): void {
    entry.item = { ...entry.item, ...patch };
    this.publish();
  }

  private publish(): void {
    this.snapshot = Array.from(this.entries.values(), (e) => e.item);
    this.listeners.forEach((listener) => listener());
  }
}
