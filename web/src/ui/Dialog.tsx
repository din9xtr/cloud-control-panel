import { X } from "lucide-react";
import { type ReactNode, useEffect, useId, useRef } from "react";
import { cn } from "@/lib/cn";
import { IconButton } from "./Button";

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
        "max-sm:mb-0 max-sm:w-full max-sm:max-w-none max-sm:rounded-t-[20px] max-sm:rounded-b-none max-sm:border-b-0",
        "rounded-[14px] open:animate-rise",
        size === "sm" ? "max-w-[400px]" : "max-w-[480px]",
      )}
    >
      {open && (
        <div className="flex max-h-[85dvh] min-h-0 flex-col">
          <div aria-hidden="true" className="mx-auto mt-2 h-[5px] w-9 rounded-full bg-fg-faint sm:hidden" />
          <header className="flex items-start gap-3 px-5 pt-5 max-sm:pt-3">
            <div className="flex min-w-0 flex-1 flex-col gap-1.5">
              {eyebrow && <span className="label-caps">{eyebrow}</span>}
              <h2 id={titleId} className="m-0 truncate text-[17px] font-semibold">
                {title}
              </h2>
              {description && <div className="font-mono text-xs text-fg-subtle">{description}</div>}
            </div>
            <IconButton label="Close" size="sm" className="max-sm:-mt-1 max-sm:-mr-2 max-sm:size-11" onClick={onClose}>
              <X size={16} strokeWidth={1.75} />
            </IconButton>
          </header>
          {children && <div className="flex flex-col gap-4 overflow-y-auto px-5 py-5">{children}</div>}
          {footer && <footer className="flex shrink-0 flex-wrap justify-end gap-2 border-t border-line px-5 pt-3.5 pb-[max(env(safe-area-inset-bottom),14px)] [&>button]:max-sm:min-h-11">{footer}</footer>}
        </div>
      )}
    </dialog>
  );
}
