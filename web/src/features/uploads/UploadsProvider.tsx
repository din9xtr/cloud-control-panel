import { useQueryClient } from "@tanstack/react-query";
import { createContext, type ReactNode, useContext, useEffect, useState, useSyncExternalStore } from "react";
import { keys } from "@/api/queries";
import { tusFactory } from "./tusFactory";
import { type UploadFactory, type UploadItem, UploadQueue } from "./UploadQueue";

const QueueContext = createContext<UploadQueue | null>(null);

export function UploadsProvider({ children, factory = tusFactory }: { children: ReactNode; factory?: UploadFactory }) {
  const client = useQueryClient();
  const [queue] = useState(
    () =>
      new UploadQueue(factory, (folder) => {
        void client.invalidateQueries({ queryKey: keys.folder(folder) });
        void client.invalidateQueries({ queryKey: keys.storage });
      }),
  );
  const items = useSyncExternalStore(queue.subscribe, queue.getSnapshot);
  const busy = items.some((i) => i.status === "uploading" || i.status === "queued");

  useEffect(() => {
    if (!busy) return;
    const warn = (event: BeforeUnloadEvent) => event.preventDefault();
    window.addEventListener("beforeunload", warn);
    return () => window.removeEventListener("beforeunload", warn);
  }, [busy]);

  return <QueueContext.Provider value={queue}>{children}</QueueContext.Provider>;
}

export function useUploadQueue(): UploadQueue {
  const queue = useContext(QueueContext);
  if (!queue) throw new Error("useUploadQueue outside UploadsProvider");
  return queue;
}

export function useUploads(): UploadItem[] {
  const queue = useUploadQueue();
  return useSyncExternalStore(queue.subscribe, queue.getSnapshot);
}
