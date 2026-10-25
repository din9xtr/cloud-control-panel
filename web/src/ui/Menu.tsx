import { type CSSProperties, type KeyboardEvent, type ReactNode, useEffect, useId, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { X } from "lucide-react";
import { cn } from "@/lib/cn";
import { stagger } from "@/lib/motion";

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

const itemHeight = 36;
const gap = 8;

function placement(trigger: HTMLElement, count: number): CSSProperties {
  const rect = trigger.getBoundingClientRect();
  const height = count * itemHeight + 14;
  const right = Math.max(gap, window.innerWidth - rect.right);
  const below = window.innerHeight - rect.bottom - gap;
  if (below < height && rect.top > below) return { right, bottom: window.innerHeight - rect.top + gap, transformOrigin: "bottom right" };
  return { right, top: rect.bottom + gap, transformOrigin: "top right" };
}

export function Menu({ label, title, trigger, items, triggerClassName }: MenuProps) {
  const [open, setOpen] = useState(false);
  const [position, setPosition] = useState<CSSProperties>({});
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
    panel.current?.querySelector<HTMLButtonElement>('[role="menuitem"]')?.focus({ preventScroll: true });
    if (mobile) return;
    const onPointer = (event: PointerEvent) => {
      const target = event.target as Node;
      if (!root.current?.contains(target) && !panel.current?.contains(target)) setOpen(false);
    };
    const dismiss = () => setOpen(false);
    document.addEventListener("pointerdown", onPointer);
    window.addEventListener("resize", dismiss);
    window.addEventListener("scroll", dismiss, { capture: true, passive: true });
    return () => {
      document.removeEventListener("pointerdown", onPointer);
      window.removeEventListener("resize", dismiss);
      window.removeEventListener("scroll", dismiss, { capture: true });
    };
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
      style={mobile ? undefined : position}
      className={cn(
        "z-50 flex flex-col text-left",
        mobile
          ? "relative max-h-[85dvh] w-full animate-sheet overflow-y-auto rounded-t-sheet border-t border-line-strong bg-surface px-3 pt-2 pb-[max(env(safe-area-inset-bottom),16px)]"
          : "fixed min-w-56 animate-drop rounded-card border border-line-strong bg-raised p-1.5 shadow-float",
      )}
    >
      {mobile && (
        <>
          <div aria-hidden="true" className="mx-auto mb-1 h-[5px] w-9 rounded-full bg-fg-faint" />
          <header className="flex items-center gap-3 px-2 pt-1 pb-2">
            <span className="type-title min-w-0 flex-1 truncate">{title ?? label}</span>
            <button type="button" aria-label="Close menu" onClick={() => close()} className="flex size-11 shrink-0 cursor-pointer items-center justify-center rounded-control text-fg-muted transition-colors hover:bg-raised hover:text-fg"><X size={18} strokeWidth={2.25} aria-hidden /></button>
          </header>
        </>
      )}
      {visible.map((item, index) => (
        <button
          key={item.label}
          type="button"
          role="menuitem"
          style={stagger(index, mobile ? 100 : 30)}
          onClick={() => { close(false); item.onSelect(); }}
          className={cn(
            "flex animate-enter cursor-pointer items-center gap-3 text-left outline-none transition-colors duration-200 ease-swift",
            mobile ? "min-h-13 rounded-control px-3 type-body hover:bg-raised focus-visible:bg-raised" : "min-h-9 rounded-lg px-2.5 type-small hover:bg-elevated focus-visible:bg-elevated",
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
        onClick={() => {
          if (!open && button.current) setPosition(placement(button.current, visible.length));
          setOpen((v) => !v);
        }}
        className={cn(
          "inline-flex size-9 cursor-pointer items-center justify-center rounded-control text-fg-muted transition-colors duration-200 ease-swift hover:bg-raised hover:text-fg max-sm:size-11",
          open && "bg-raised text-fg",
          triggerClassName,
        )}
      >
        {trigger}
      </button>
      {open && createPortal(mobile ? (
        <div className="fixed inset-0 z-50 flex items-end" onKeyDown={onKeyDown}>
          <button type="button" tabIndex={-1} aria-label="Dismiss menu" onClick={() => close()} className="absolute inset-0 animate-fade-in bg-canvas/70 backdrop-blur-[12px]" />
          {content}
        </div>
      ) : content, document.body)}
    </div>
  );
}
