import type { ButtonHTMLAttributes, ReactNode } from "react";
import { cn } from "@/lib/cn";

type Variant = "primary" | "secondary" | "elevated" | "ghost" | "danger";
type Size = "sm" | "md" | "lg";

const variants: Record<Variant, string> = {
  primary: "bg-accent text-on-accent hover:bg-accent-hover active:bg-accent-active disabled:bg-accent/40",
  secondary: "border-line-strong bg-raised text-fg hover:border-fg-faint hover:bg-elevated active:bg-raised",
  elevated: "bg-fg text-canvas hover:bg-white active:bg-fg-secondary",
  ghost: "bg-transparent text-fg-muted hover:bg-raised hover:text-fg",
  danger: "border-danger/30 bg-transparent text-danger hover:bg-danger/10 active:bg-danger/16",
};

const sizes: Record<Size, string> = {
  sm: "h-8 px-3 type-micro gap-1.5 font-medium",
  md: "h-9 px-3.5 type-small gap-2 font-medium",
  lg: "h-11 px-5 type-body gap-2 font-medium",
};

type ButtonProps = ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: Variant;
  size?: Size;
  icon?: ReactNode;
  busy?: boolean;
};

export function Button({ variant = "secondary", size = "md", icon, busy, className, children, disabled, type = "button", ...rest }: ButtonProps) {
  return (
    <button
      type={type}
      disabled={disabled || busy}
      aria-busy={busy || undefined}
      className={cn(
        "inline-flex shrink-0 cursor-pointer items-center justify-center rounded-control border border-transparent whitespace-nowrap transition-colors duration-200 ease-swift select-none disabled:cursor-not-allowed disabled:opacity-50",
        variants[variant],
        sizes[size],
        className,
      )}
      {...rest}
    >
      {busy ? <Spinner /> : icon}
      {children}
    </button>
  );
}

type IconButtonProps = ButtonHTMLAttributes<HTMLButtonElement> & {
  label: string;
  size?: "sm" | "md" | "touch";
};

const iconSizes = { sm: "size-8", md: "size-9", touch: "size-11" };

export function IconButton({ label, size = "md", className, children, type = "button", ...rest }: IconButtonProps) {
  return (
    <button
      type={type}
      aria-label={label}
      title={label}
      className={cn(
        "inline-flex shrink-0 cursor-pointer items-center justify-center rounded-control text-fg-muted transition-colors duration-200 ease-swift hover:bg-raised hover:text-fg disabled:opacity-50",
        iconSizes[size],
        className,
      )}
      {...rest}
    >
      {children}
    </button>
  );
}

export function Spinner({ className }: { className?: string }) {
  return (
    <span
      aria-hidden="true"
      className={cn("inline-block size-3.5 animate-spin rounded-full border-[1.5px] border-current border-r-transparent", className)}
    />
  );
}
