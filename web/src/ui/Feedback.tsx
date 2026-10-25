import type { ReactNode } from "react";
import { cn } from "@/lib/cn";
import { Spinner } from "./Button";

export function EmptyState({ icon, title, children }: { icon: ReactNode; title: string; children?: ReactNode }) {
  return (
    <div className="flex animate-reveal flex-col items-center justify-center gap-4 px-6 py-24 text-center">
      <span className="flex size-14 items-center justify-center rounded-card border border-line bg-surface text-fg-subtle" aria-hidden>
        {icon}
      </span>
      <p className="type-title m-0">{title}</p>
      {children && <div className="type-body max-w-md text-fg-subtle">{children}</div>}
    </div>
  );
}

export function Loading({ label = "Loading" }: { label?: string }) {
  return (
    <div role="status" className="type-small flex animate-fade-in items-center justify-center gap-2.5 py-24 text-fg-subtle">
      <Spinner className="size-4" />
      {label}
    </div>
  );
}

export function ErrorState({ message, onRetry }: { message: string; onRetry?: () => void }) {
  return (
    <div role="alert" className="type-body flex flex-col items-center gap-3 py-24 text-center text-fg-muted">
      <p className="m-0">{message}</p>
      {onRetry && (
        <button type="button" onClick={onRetry} className="cursor-pointer text-link underline-offset-4 hover:underline">
          Try again
        </button>
      )}
    </div>
  );
}

export function PageHeader({ eyebrow, title, actions, children }: { eyebrow?: string; title: ReactNode; actions?: ReactNode; children?: ReactNode }) {
  return (
    <header className="flex animate-reveal flex-wrap items-end justify-between gap-x-6 gap-y-5">
      <div className="flex min-w-0 flex-col gap-2">
        {eyebrow && <span className="label-caps">{eyebrow}</span>}
        <h1 className="type-page m-0">{title}</h1>
        {children && <div className="type-body mt-1 max-w-[640px] text-fg-muted">{children}</div>}
      </div>
      {actions && <div className="flex flex-wrap gap-3">{actions}</div>}
    </header>
  );
}

export function Badge({ children, tone = "neutral" }: { children: ReactNode; tone?: "neutral" | "info" }) {
  const tones = { neutral: "bg-elevated text-fg-secondary", info: "bg-accent/15 text-link" };
  return <span className={cn("type-micro inline-flex shrink-0 items-center gap-1 rounded-md px-1.5 py-0.5 font-medium", tones[tone])}>{children}</span>;
}
