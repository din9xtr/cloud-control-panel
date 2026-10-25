import type { ButtonHTMLAttributes, ReactNode } from "react";
import { cn } from "@/lib/cn";

type Variant = "primary" | "secondary" | "ghost" | "danger";
type Size = "sm" | "md" | "lg";

const variants: Record<Variant, string> = {
  primary: "bg-fg text-canvas border-fg hover:bg-white disabled:bg-fg-faint disabled:border-fg-faint",
  secondary: "bg-surface text-fg border-line-strong hover:bg-raised hover:border-[#3d444d]",
  ghost: "bg-transparent text-fg-muted border-transparent hover:bg-raised hover:text-fg",
  danger: "bg-transparent text-danger border-line-strong hover:bg-danger/10 hover:border-danger/40",
};

const sizes: Record<Size, string> = {
  sm: "h-8 px-3 text-[13px] gap-1.5 rounded-md",
  md: "h-9 px-3.5 text-[13px] gap-2 rounded-lg",
  lg: "h-11 px-4 text-[15px] gap-2 rounded-[10px]",
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
        "inline-flex shrink-0 cursor-pointer items-center justify-center border font-medium whitespace-nowrap transition-colors select-none disabled:cursor-not-allowed disabled:opacity-60",
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
        "inline-flex shrink-0 cursor-pointer items-center justify-center rounded-lg border border-transparent text-fg-muted transition-colors hover:bg-raised hover:text-fg disabled:opacity-50",
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
      className={cn("inline-block size-3.5 animate-spin rounded-full border-2 border-current border-r-transparent", className)}
    />
  );
}
