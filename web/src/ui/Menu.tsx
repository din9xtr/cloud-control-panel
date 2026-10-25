import { type KeyboardEvent, type ReactNode, useEffect, useId, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { X } from "lucide-react";
import { cn } from "@/lib/cn";

export type MenuItem = {
  label: string;
  icon: ReactNode;
  onSelect: () => void;
  danger?: boolean;
  hidden?: boolean;
};

type MenuProps = {
  label: string;
  title?: string;
  trigger: ReactNode;
  items: MenuItem[];
  triggerClassName?: string;
};

export function Menu({ label, title, trigger, items, triggerClassName }: MenuProps) {
  const [open, setOpen] = useState(false);
  const [mobile, setMobile] = useState(() => window.matchMedia?.("(max-width: 639px)").matches ?? false);
  const panel = useRef<HTMLDivElement>(null);
  const root = useRef<HTMLDivElement>(null);
  const button = useRef<HTMLButtonElement>(null);
  const menuId = useId();
  const visible = items.filter((item) => !item.hidden);

  useEffect(() => {
    const media = window.matchMedia?.("(max-width: 639px)");
    if (!media) return;
    const change = () => { setMobile(media.matches); setOpen(false); };
    media.addEventListener("change", change);
    return () => media.removeEventListener("change", change);
  }, []);

  useEffect(() => {
    if (!open || !mobile) return;
    const locked = document.body.classList.contains("overflow-hidden");
    document.body.classList.add("overflow-hidden");
    return () => { if (!locked) document.body.classList.remove("overflow-hidden"); };
  }, [open, mobile]);

  useEffect(() => {
    if (!open) return;
    panel.current?.querySelector<HTMLButtonElement>('[role="menuitem"]')?.focus();
    const onPointer = (event: PointerEvent) => {
      if (!mobile && !root.current?.contains(event.target as Node)) setOpen(false);
    };
    document.addEventListener("pointerdown", onPointer);
    return () => document.removeEventListener("pointerdown", onPointer);
  }, [open, mobile]);

  const close = (refocus = true) => {
    setOpen(false);
    if (refocus) button.current?.focus();
  };

  const onKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
    const entries = Array.from(panel.current?.querySelectorAll<HTMLButtonElement>('[role="menuitem"]') ?? []);
    const index = entries.indexOf(document.activeElement as HTMLButtonElement);
    if (event.key === "Escape") {
      event.stopPropagation();
      close();
    } else if (event.key === "ArrowDown" || event.key === "ArrowUp") {
      event.preventDefault();
      const step = event.key === "ArrowDown" ? 1 : -1;
      entries[(index + step + entries.length) % entries.length]?.focus();
    } else if (event.key === "Tab") {
      close(false);
    }
  };

  const content = (
    <div
      ref={panel}
      id={menuId}
      role="menu"
      aria-label={label}
      onKeyDown={mobile ? undefined : onKeyDown}
      className={cn(
        "relative z-50 flex animate-rise flex-col border border-line-strong bg-surface text-left shadow-float",
        mobile
          ? "max-h-[85dvh] w-full overflow-y-auto rounded-t-3xl border-b-0 px-2 pt-2 pb-[max(env(safe-area-inset-bottom),12px)]"
          : "absolute top-full right-0 mt-1 min-w-52 rounded-xl py-1.5",
      )}
    >
      {mobile && (
        <header className="flex items-center gap-3 border-b border-line px-3 pt-1 pb-3">
          <span className="min-w-0 flex-1 truncate text-[15px] font-semibold">{title ?? label}</span>
          <button type="button" aria-label="Close menu" onClick={() => close()} className="flex size-11 shrink-0 items-center justify-center rounded-xl text-fg-muted hover:bg-raised"><X size={20} aria-hidden /></button>
        </header>
      )}
      {visible.map((item) => (
        <button
          key={item.label}
          type="button"
          role="menuitem"
          onClick={() => { close(false); item.onSelect(); }}
          className={cn(
            "flex cursor-pointer items-center gap-3 rounded-xl text-left outline-none hover:bg-raised focus-visible:bg-raised",
            mobile ? "min-h-13 px-3 text-[15px]" : "mx-1.5 min-h-9 rounded-md px-2.5 text-[13px]",
            item.danger ? "text-danger" : "text-fg",
          )}
        >
          <span aria-hidden className={cn("flex shrink-0", item.danger ? "text-danger" : "text-fg-muted")}>{item.icon}</span>
          {item.label}
        </button>
      ))}
    </div>
  );

  return (
    <div ref={root} className="relative">
      <button
        ref={button}
        type="button"
        aria-label={label}
        title={label}
        aria-haspopup="menu"
        aria-expanded={open}
        aria-controls={open ? menuId : undefined}
        onClick={() => setOpen((v) => !v)}
        className={cn(
          "inline-flex size-9 cursor-pointer items-center justify-center rounded-lg text-fg-muted transition-colors hover:bg-raised hover:text-fg max-sm:size-11",
          open && "bg-raised text-fg",
          triggerClassName,
        )}
      >
        {trigger}
      </button>
      {open && (mobile ? createPortal(
        <div className="fixed inset-0 z-50 flex items-end" onKeyDown={onKeyDown}>
          <button type="button" tabIndex={-1} aria-label="Dismiss menu" onClick={() => close()} className="absolute inset-0 animate-fade-in bg-[rgb(1_4_9/0.6)]" />
          {content}
        </div>, document.body,
      ) : content)}
    </div>
  );
}
