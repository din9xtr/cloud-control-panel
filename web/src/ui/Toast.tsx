import { CircleAlert, Check, X } from "lucide-react";
import { createContext, type ReactNode, useCallback, useContext, useMemo, useRef, useState } from "react";
import { cn } from "@/lib/cn";

type Tone = "info" | "error";
type Toast = { id: number; message: string; tone: Tone };
type ToastApi = { notify: (message: string) => void; fail: (message: string) => void };

const ToastContext = createContext<ToastApi | null>(null);
const lifetime = 4200;

export function ToastProvider({ children }: { children: ReactNode }) {
  const [toasts, setToasts] = useState<Toast[]>([]);
  const next = useRef(0);

  const dismiss = useCallback((id: number) => setToasts((list) => list.filter((t) => t.id !== id)), []);
  const push = useCallback(
    (message: string, tone: Tone) => {
      const id = ++next.current;
      setToasts((list) => [...list.slice(-2), { id, message, tone }]);
      window.setTimeout(() => dismiss(id), tone === "error" ? lifetime * 1.5 : lifetime);
    },
    [dismiss],
  );
  const value = useMemo<ToastApi>(() => ({ notify: (m) => push(m, "info"), fail: (m) => push(m, "error") }), [push]);

  return (
    <ToastContext.Provider value={value}>
      {children}
      <div aria-live="polite" className="pointer-events-none fixed inset-x-0 bottom-0 z-50 flex flex-col items-center gap-2 p-4 pb-[calc(env(safe-area-inset-bottom)+88px)] sm:items-end sm:pb-6">
        {toasts.map((t) => (
          <div
            key={t.id}
            role={t.tone === "error" ? "alert" : "status"}
            className="pointer-events-auto flex max-w-sm animate-rise items-center gap-3 rounded-xl border border-line-strong bg-surface/95 py-2.5 pr-2 pl-3.5 text-[13px] shadow-float backdrop-blur-xl"
          >
            {t.tone === "error" ? <CircleAlert size={16} className="text-danger" aria-hidden /> : <Check size={16} className="text-fg-muted" aria-hidden />}
            <span className={cn("flex-1", t.tone === "error" && "text-fg")}>{t.message}</span>
            <button type="button" aria-label="Dismiss" onClick={() => dismiss(t.id)} className="inline-flex size-7 cursor-pointer items-center justify-center rounded-md text-fg-subtle hover:bg-raised hover:text-fg">
              <X size={14} aria-hidden />
            </button>
          </div>
        ))}
      </div>
    </ToastContext.Provider>
  );
}

export function useToast(): ToastApi {
  const ctx = useContext(ToastContext);
  if (!ctx) throw new Error("useToast outside ToastProvider");
  return ctx;
}
