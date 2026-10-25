import type { ReactNode } from "react";
import { Wordmark } from "@/layout/Wordmark";

export function AuthLayout({ title, prompt, intro, children, footnote }: { title: string; prompt?: string; intro?: ReactNode; children: ReactNode; footnote?: ReactNode }) {
  return (
    <div className="flex min-h-dvh flex-col">
      <main className="flex flex-1 flex-col items-center justify-center gap-10 px-4 pt-[max(64px,env(safe-area-inset-top))] pb-[max(64px,env(safe-area-inset-bottom))]">
        <div className="flex animate-reveal flex-col items-center gap-3 text-center">
          {prompt && <samp className="font-mono text-[13px] text-fg-subtle">{prompt}</samp>}
          <Wordmark size="lg" blink className="mb-3" />
          <h1 className="type-page m-0">{title}</h1>
          {intro && <p className="type-body m-0 max-w-[400px] text-fg-muted">{intro}</p>}
        </div>
        <div className="w-full max-w-[400px] animate-reveal [animation-delay:120ms]">{children}</div>
        {footnote && <samp className="m-0 animate-fade-in font-mono text-xs text-fg-subtle [animation-delay:320ms]">{footnote}</samp>}
      </main>
    </div>
  );
}
