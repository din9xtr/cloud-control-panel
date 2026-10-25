import { X } from "lucide-react";
import { type ReactNode, useEffect, useId, useRef } from "react";
import { cn } from "@/lib/cn";

type DialogProps = {
  open: boolean;
  onClose: () => void;
  title: ReactNode;
  eyebrow?: string;
  description?: ReactNode;
  children?: ReactNode;
  footer?: ReactNode;
  size?: "sm" | "md";
};

export function Dialog({ open, onClose, title, eyebrow, description, children, footer, size = "md" }: DialogProps) {
  const ref = useRef<HTMLDialogElement>(null);
  const titleId = useId();

  useEffect(() => {
    const dialog = ref.current;
    if (!dialog) return;
    if (open && !dialog.open) dialog.showModal();
    if (!open && dialog.open) dialog.close();
  }, [open]);

  return (
    <dialog
      ref={ref}
      aria-labelledby={titleId}
      onClose={onClose}
      onCancel={(event) => {
        event.preventDefault();
        onClose();
      }}
      onClick={(event) => {
        if (event.target === ref.current) onClose();
      }}
      className={cn(
        "m-auto w-[calc(100%-32px)] border border-line-strong bg-surface p-0 text-fg shadow-dialog backdrop:animate-fade-in",
        "rounded-sheet open:animate-rise",
        "max-sm:mb-0 max-sm:w-full max-sm:max-w-none max-sm:rounded-b-none max-sm:border-x-0 max-sm:border-b-0 max-sm:open:animate-sheet",
        size === "sm" ? "max-w-[440px]" : "max-w-[540px]",
      )}
    >
      {open && (
        <div className="flex max-h-[88dvh] min-h-0 flex-col">
          <div aria-hidden="true" className="mx-auto mt-2 h-[5px] w-9 rounded-full bg-fg-faint sm:hidden" />
          <header className="relative flex flex-col gap-1.5 border-b border-line px-6 pt-6 pr-16 pb-5 max-sm:px-5 max-sm:pt-4">
            {eyebrow && <span className="label-caps">{eyebrow}</span>}
            <h2 id={titleId} className="type-title m-0 truncate">
              {title}
            </h2>
            {description && <div className="type-small text-fg-subtle">{description}</div>}
            <button
              type="button"
              aria-label="Close"
              title="Close"
              onClick={onClose}
              className="absolute top-4 right-4 inline-flex size-9 cursor-pointer items-center justify-center rounded-control text-fg-muted transition-colors duration-200 ease-swift hover:bg-raised hover:text-fg max-sm:top-3 max-sm:right-3 max-sm:size-11"
            >
              <X size={16} strokeWidth={2.25} aria-hidden />
            </button>
          </header>
          {children && <div className="flex flex-col gap-5 overflow-y-auto px-6 py-6 max-sm:px-5">{children}</div>}
          {footer && <footer className="flex shrink-0 flex-wrap justify-end gap-2.5 border-t border-line bg-sunken/60 px-6 pt-4 pb-[max(env(safe-area-inset-bottom),16px)] max-sm:px-5 max-sm:pb-[max(env(safe-area-inset-bottom),20px)] [&>button]:max-sm:min-h-11 [&>button]:max-sm:flex-1">{footer}</footer>}
        </div>
      )}
    </dialog>
  );
}
