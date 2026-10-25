import type { ReactNode } from "react";
import { Spinner } from "./Button";

export function EmptyState({ icon, title, children }: { icon: ReactNode; title: string; children?: ReactNode }) {
  return (
    <div className="flex flex-col items-center justify-center gap-3 px-6 py-20 text-center">
      <span className="flex size-12 items-center justify-center rounded-2xl border border-line bg-sunken text-fg-subtle" aria-hidden>
        {icon}
      </span>
      <p className="m-0 text-[15px] font-medium">{title}</p>
      {children && <div className="max-w-sm text-[13px] text-fg-subtle">{children}</div>}
    </div>
  );
}

export function Loading({ label = "Loading" }: { label?: string }) {
  return (
    <div role="status" className="flex items-center justify-center gap-2 py-20 text-[13px] text-fg-subtle">
      <Spinner />
      {label}
    </div>
  );
}

export function ErrorState({ message, onRetry }: { message: string; onRetry?: () => void }) {
  return (
    <div role="alert" className="flex flex-col items-center gap-3 py-20 text-center text-[13px] text-fg-muted">
      <p className="m-0">{message}</p>
      {onRetry && (
        <button type="button" onClick={onRetry} className="cursor-pointer rounded-md px-3 py-1.5 text-fg underline-offset-4 hover:underline">
          Try again
        </button>
      )}
    </div>
  );
}

export function PageHeader({ eyebrow, title, actions }: { eyebrow: string; title: ReactNode; actions?: ReactNode }) {
  return (
    <header className="flex flex-wrap items-end justify-between gap-4">
      <div className="flex min-w-0 flex-col gap-1.5">
        <span className="label-caps">{eyebrow}</span>
        <h1 className="m-0 text-[26px] leading-tight font-semibold tracking-[-0.01em]">{title}</h1>
      </div>
      {actions && <div className="flex flex-wrap gap-2">{actions}</div>}
    </header>
  );
}
