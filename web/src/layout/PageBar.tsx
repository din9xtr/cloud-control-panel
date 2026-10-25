import { useNavigate } from "@tanstack/react-router";
import { MoreHorizontal } from "lucide-react";
import type { ReactNode } from "react";
import { cn } from "@/lib/cn";
import { Button } from "@/ui/Button";
import { Menu } from "@/ui/Menu";

export type PageAction = { label: string; icon?: ReactNode; onSelect?: () => void; to?: string };

type PageBarProps = {
  title: ReactNode;
  prefix?: string;
  actions?: PageAction[];
  primary?: ReactNode;
};

export function PageBar({ title, prefix, actions = [], primary }: PageBarProps) {
  const navigate = useNavigate();
  const run = (action: PageAction) => (action.to ? void navigate({ to: action.to }) : action.onSelect?.());
  return (
    <div className="glass z-30 border-b border-line md:sticky md:top-14">
      <div className="mx-auto flex h-12 max-w-[1120px] items-center gap-3 px-6 max-md:px-4">
        {typeof title === "string" ? (
          <h2 className="type-small m-0 flex min-w-0 flex-1 items-baseline gap-2 font-semibold">
            {prefix && <span className="shrink-0 font-mono text-xs font-normal text-fg-faint">{prefix}</span>}
            <span className="truncate">{title}</span>
          </h2>
        ) : (
          <div className="flex min-w-0 flex-1 items-center">{title}</div>
        )}
        {actions.length > 0 && (
          <>
            <div className="hidden items-center gap-1 md:flex">
              {actions.map((action) => (
                <Button key={action.label} variant="ghost" size="sm" icon={action.icon} onClick={() => run(action)}>
                  {action.label}
                </Button>
              ))}
            </div>
            <div className="md:hidden">
              <Menu
                label="More actions"
                title={typeof title === "string" ? title : undefined}
                trigger={<MoreHorizontal size={18} aria-hidden />}
                items={actions.map((action) => ({ label: action.label, icon: action.icon ?? null, onSelect: () => run(action) }))}
              />
            </div>
          </>
        )}
        {primary}
      </div>
    </div>
  );
}

export function PrimaryAction({ children, onClick, icon, tone = "accent" }: { children: ReactNode; onClick: () => void; icon?: ReactNode; tone?: "accent" | "danger" }) {
  return (
    <Button variant={tone === "accent" ? "primary" : "danger"} size="sm" icon={icon} onClick={onClick} className="shrink-0">
      {children}
    </Button>
  );
}

export function Page({ children, width = "default" }: { children: ReactNode; width?: "default" | "narrow" | "wide" }) {
  const max = { default: "max-w-[1000px]", narrow: "max-w-[720px]", wide: "max-w-[1120px]" }[width];
  return <div className={cn("mx-auto flex w-full flex-col gap-9 px-6 pt-12 pb-10 max-md:gap-7 max-md:px-4 max-md:pt-8", max)}>{children}</div>;
}
